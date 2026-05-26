// Package finisher — ComplianceHIPAAGate.
//
// Enforces HIPAA Security Rule §164.312 technical safeguards against
// the project workspace. Only fires when Project.Spec.Compliance
// contains "hipaa".
//
// Controls covered:
//
//   - 164.312(a)(1)    Access control + RBAC declaration.
//   - 164.312(a)(2)(i) Unique user IDs (warn on shared accounts).
//   - 164.312(a)(2)(iv) Encryption of PHI at rest.
//   - 164.312(b)       Audit controls (audit_log / activity table /
//     opentelemetry export).
//   - 164.312(c)(1)    Integrity controls (HMAC / SHA-256 on PHI).
//   - 164.312(d)       Person / entity authentication (email
//     verification or MFA).
//   - 164.312(e)(1)    Transmission security (HTTPS / TLS).
//   - BAA              Business Associate Agreement artefact.
//   - PHI tagging      Structured `// PHI:` comments on PHI fields.
//
// Repair agent is the Security agent.
package finisher

import (
	"context"
	"regexp"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
)

type ComplianceHIPAAGate struct{}

func (ComplianceHIPAAGate) Name() domain.GateName    { return domain.GateComplianceHIPAA }
func (ComplianceHIPAAGate) RepairAgent() agents.Role { return agents.RoleSecurity }

// hipaaPHITableRe matches migration table declarations whose name
// suggests Protected Health Information. The HIPAA Security Rule
// treats encryption as "addressable" rather than mandatory, but every
// modern audit treats unencrypted PHI as a finding.
var hipaaPHITableRe = regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:public\.)?["` + "`" + `]?(patients?|health_records?|medical_records?|medical_[a-z_]+|phi_[a-z_]+|patient_[a-z_]+|clinical_[a-z_]+|prescriptions?)\b`)

// hipaaPHITagRe matches an inline `// PHI:` comment on a struct field
// or schema column. Operators declare PHI columns explicitly so the
// inventory is human-readable.
var hipaaPHITagRe = regexp.MustCompile(`(?i)//\s*PHI:?\b|#\s*PHI:?\b`)

// hipaaSharedAccountRe matches the obvious "this is a shared account"
// strings. Heuristic — the goal is to surface a warning, not a hard
// fail, since some apps legitimately have a "system" service account.
var hipaaSharedAccountRe = regexp.MustCompile(`(?i)\b(shared_account|guest_user|generic_user|service_account|sharedaccount)\b`)

func (ComplianceHIPAAGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	if !p.Spec.HasCompliance("hipaa") {
		return nil
	}
	var issues []domain.Issue

	// 164.312(a)(1) Access control — auth stack + RBAC.
	if strings.TrimSpace(p.Spec.Stack.Auth) == "" {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityError,
			Message: "164.312(a)(1): no Auth stack declared",
			Hint:    "HIPAA access control requires an authentication system — set Spec.Stack.Auth",
		})
	}
	if !projectMentions(p, "role", "roles", "rbac", "permissions", "policy_check", "policycheck", "abilities") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityError,
			Message: "164.312(a)(1): no RBAC / role-based access surface detected",
			Hint:    "add a roles or permissions concept (table, enum, middleware) — HIPAA wants per-actor access scoping",
		})
	}

	// 164.312(a)(2)(i) Unique user ID — warn on shared / generic accounts.
	for _, f := range p.Files {
		if hipaaSharedAccountRe.MatchString(f.Content) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityWarning,
				Message: "164.312(a)(2)(i): shared/group account marker found",
				Path:    f.Path,
				Hint:    "HIPAA requires unique user IDs — remove shared_account / guest_user / service_account abstractions or scope them tightly",
			})
		}
	}

	// 164.312(a)(2)(iv) Encryption at rest for PHI.
	hasEncryptionImport := projectImports(p, "crypto/aes", "crypto/cipher", "bcrypt", "argon2", "pgcrypto", "kms", "vault")
	phiTables := findPHITables(p)
	if len(phiTables) > 0 && !hasEncryptionImport {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityError,
			Message: "164.312(a)(2)(iv): PHI tables declared but no encryption primitive imported",
			Hint:    "encrypt at rest — use pgcrypto + KMS-managed keys, or AES-GCM with envelope encryption. Tables flagged: " + strings.Join(phiTables, ", "),
		})
	}

	// 164.312(b) Audit controls.
	hasAuditLog := projectMentions(p,
		"audit_log", "auditlog", "audit_logs", "audit_events",
		"activity_log", "activitylog",
		"opentelemetry", "otel_exporter", "otlp_endpoint",
	)
	if !hasAuditLog {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityError,
			Message: "164.312(b): no audit-control surface (audit_log table / Activity model / opentelemetry export)",
			Hint:    "HIPAA REQUIRES auditing of access to PHI — wire an audit_log table or an OTel exporter that records who saw what when",
		})
	}

	// 164.312(c)(1) Integrity controls.
	if len(phiTables) > 0 && !projectImports(p, "crypto/hmac", "crypto/sha256", "crypto/sha512") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityWarning,
			Message: "164.312(c)(1): no HMAC / SHA-256 / SHA-512 import on a project storing PHI",
			Hint:    "stamp an integrity hash (HMAC) on PHI rows so tamper detection is possible",
		})
	}

	// 164.312(d) Person/entity authentication — email verification or MFA.
	if !projectMentions(p,
		"verifyemail", "verify_email", "email_verified", "emailverified",
		"mfa_enabled", "mfa.enabled", "two_factor",
		"webauthn", "passkey", "totp",
	) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityWarning,
			Message: "164.312(d): no email-verification or MFA marker found",
			Hint:    "wire verifyEmail() / email_verified column OR add MFA (TOTP, WebAuthn) so identity is provable",
		})
	}

	// 164.312(e)(1) Transmission security — same surface as SOC2 CC6.6.
	if !hasTLSMarker(p) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityError,
			Message: "164.312(e)(1): no TLS marker found across Dockerfile / nginx / docker-compose — PHI MUST be HTTPS-only",
			Hint:    "terminate TLS at the ingress; HIPAA does not allow PHI on plaintext channels",
		})
	}

	// BAA artefact.
	if !hasSecurityArtefact(p, "baa.md", "docs/baa", "docs/business_associate") &&
		!projectMentions(p, "business associate agreement", "business_associate_agreement") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityInfo,
			Message: "no BAA.md or 'Business Associate Agreement' mention in docs",
			Hint:    "publish a BAA.md naming subprocessors that touch PHI and the BAA status with each",
		})
	}

	// PHI tagging — structured comments.
	if !anyContentMatches(p, hipaaPHITagRe) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceHIPAA, Severity: domain.SeverityWarning,
			Message: "no // PHI: tags found across the project on a HIPAA-declared workspace",
			Hint:    "annotate each PHI-bearing field with `// PHI: <description>` so the data inventory is greppable",
		})
	}

	return issues
}

// findPHITables scans every *.sql / migration file for CREATE TABLE
// statements whose name implies PHI. Returns the de-duplicated set of
// table names so the gate Hint can name them directly.
func findPHITables(p *domain.Project) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		if !strings.HasSuffix(low, ".sql") &&
			!strings.Contains(low, "/migrations/") &&
			!strings.HasPrefix(low, "migrations/") {
			continue
		}
		for _, m := range hipaaPHITableRe.FindAllStringSubmatch(f.Content, -1) {
			if len(m) >= 2 {
				name := strings.ToLower(m[1])
				if _, ok := seen[name]; ok {
					continue
				}
				seen[name] = struct{}{}
				out = append(out, name)
			}
		}
	}
	return out
}

// anyContentMatches returns true when any project file's content
// matches the regexp. Used for project-wide presence checks.
func anyContentMatches(p *domain.Project, re *regexp.Regexp) bool {
	for _, f := range p.Files {
		if re.MatchString(f.Content) {
			return true
		}
	}
	return false
}
