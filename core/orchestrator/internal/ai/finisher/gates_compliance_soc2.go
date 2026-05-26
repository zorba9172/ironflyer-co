// Package finisher — ComplianceSOC2Gate.
//
// Implements the SOC2 Common Criteria (CC) control families relevant
// to a generated SaaS workspace. Today we cover:
//
//   - CC3.4  Risk assessment artefact (SECURITY.md / threat model).
//   - CC6.1  Logical access controls (auth declared; safe password
//     storage; session-secret hygiene).
//   - CC6.6  Encryption in transit (TLS markers).
//   - CC6.7  Encryption at rest (Postgres sslmode).
//   - CC7.2  Monitoring / observability tooling.
//   - CC7.3  Audit logging.
//   - CC8.1  Change-management artefact (README + Dockerfile).
//   - MFA    SOC2 audit-preference for multi-factor support.
//   - Backup Declared backup policy.
//   - Pentest Pen-test or security-audit artefact.
//
// The gate only fires when Project.Spec.Compliance includes "soc2".
// Findings carry the CC code in the Hint so the operator dashboard can
// link to the matching control. Repair agent is the Security agent.
package finisher

import (
	"context"
	"regexp"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
)

type ComplianceSOC2Gate struct{}

func (ComplianceSOC2Gate) Name() domain.GateName    { return domain.GateComplianceSOC2 }
func (ComplianceSOC2Gate) RepairAgent() agents.Role { return agents.RoleSecurity }

// soc2PlaintextPasswordSQLRe matches `password` columns declared as
// VARCHAR / TEXT in SQL migrations. Hashed-password columns nearly
// always carry "hash" / "digest" in the column name, so the false
// positive rate is acceptable for an audit signal.
var soc2PlaintextPasswordSQLRe = regexp.MustCompile(`(?i)\bpassword\s+(?:varchar|text|char)\b`)

// soc2PlaintextPasswordGoRe matches Go struct fields of the form
// `Password string` — the canonical "we forgot to hash it" smell.
var soc2PlaintextPasswordGoRe = regexp.MustCompile(`(?im)^\s*password\s+string\b`)

// soc2HardcodedSecretRe is the JWT-secret literal pattern. The grep
// ignores env.example files; the gate's per-file loop applies that
// filter before evaluating.
var soc2HardcodedSecretRe = regexp.MustCompile(`(?i)JWT_SECRET\s*=\s*["']?[A-Za-z0-9+/=_\-]{8,}`)

// soc2PostgresURLRe matches a Postgres connection URL so we can
// inspect the sslmode parameter.
var soc2PostgresURLRe = regexp.MustCompile(`postgres(?:ql)?://[^"'\s]+`)

func (ComplianceSOC2Gate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	if !p.Spec.HasCompliance("soc2") {
		return nil
	}
	var issues []domain.Issue

	// CC6.1 — auth stack declared.
	if strings.TrimSpace(p.Spec.Stack.Auth) == "" {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityError,
			Message: "CC6.1: project declares SOC2 compliance but no Auth stack is wired",
			Hint:    "SOC2 CC6.1 requires logical access controls — set Spec.Stack.Auth (e.g. jwt+bcrypt, supabase, clerk)",
		})
	}

	// CC6.1 password storage + hashing.
	hasBcryptOrArgon := projectImports(p, "bcrypt", "argon2", "scrypt", "pbkdf2")
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		if strings.HasSuffix(low, ".sql") && soc2PlaintextPasswordSQLRe.MatchString(f.Content) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateComplianceSOC2, Severity: domain.SeverityError,
				Message: "CC6.1: plaintext password column declared in migration",
				Path:    f.Path,
				Hint:    "store a hash (bcrypt / argon2) instead — the column should be VARCHAR holding the digest, not the raw password",
			})
		}
		if strings.HasSuffix(low, ".go") && soc2PlaintextPasswordGoRe.MatchString(f.Content) && !hasBcryptOrArgon {
			issues = append(issues, domain.Issue{
				Gate: domain.GateComplianceSOC2, Severity: domain.SeverityError,
				Message: "CC6.1: Go struct exposes plaintext Password field with no bcrypt/argon2 import in the project",
				Path:    f.Path,
				Hint:    "store a hash (bcrypt.Hash / argon2.IDKey); rename the field to PasswordHash to avoid future regression",
			})
		}
	}

	// CC6.1 hardcoded session secrets. Skip env.example / env.sample
	// files where the placeholder is expected to be replaced.
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		if strings.HasSuffix(low, ".example") || strings.HasSuffix(low, ".sample") || strings.HasSuffix(low, "env.template") {
			continue
		}
		if strings.HasSuffix(low, ".md") {
			continue
		}
		if soc2HardcodedSecretRe.MatchString(f.Content) {
			issues = append(issues, domain.Issue{
				Gate: domain.GateComplianceSOC2, Severity: domain.SeverityError,
				Message: "CC6.1: hardcoded JWT_SECRET found",
				Path:    f.Path,
				Hint:    "move the secret into Project.Secrets / .env (not committed); rotate any value that already shipped",
			})
		}
	}

	// CC6.6 — HTTPS / TLS in transit. We accept presence of any TLS
	// marker across the deploy surface as sufficient signal.
	if !hasTLSMarker(p) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityWarning,
			Message: "CC6.6: no TLS marker found across Dockerfile / nginx / docker-compose",
			Hint:    "terminate TLS at the ingress (Caddy / nginx / cloud LB) and reference the cert path in the deploy artefact",
		})
	}

	// CC6.7 — data at rest: Postgres sslmode hygiene.
	for _, f := range p.Files {
		matches := soc2PostgresURLRe.FindAllString(f.Content, -1)
		for _, m := range matches {
			low := strings.ToLower(m)
			if !strings.Contains(low, "sslmode=require") && !strings.Contains(low, "sslmode=verify") {
				issues = append(issues, domain.Issue{
					Gate: domain.GateComplianceSOC2, Severity: domain.SeverityWarning,
					Message: "CC6.7: Postgres URL without sslmode=require/verify-full",
					Path:    f.Path,
					Hint:    "append ?sslmode=require (or sslmode=verify-full when the CA bundle is wired) to enforce TLS at rest in transit",
				})
				break
			}
		}
	}

	// CC7.2 — observability wired.
	if !projectImports(p, "sentry", "posthog", "datadog", "opentelemetry", "otelhttp", "@opentelemetry", "@sentry") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityWarning,
			Message: "CC7.2: no observability provider detected (sentry / posthog / datadog / opentelemetry)",
			Hint:    "SOC2 CC7.2 expects ongoing monitoring — wire at least one error tracker or APM",
		})
	}

	// CC7.3 — audit logging mention.
	if !projectMentions(p, "audit_log", "auditlog", "auditLogger", "audit.New", "audit.Store", "\"audit\"") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityInfo,
			Message: "CC7.3: no audit-logging surface detected",
			Hint:    "even a minimal audit_log table that records who did what when is enough to satisfy CC7.3",
		})
	}

	// CC8.1 — change management. Deploy gate already enforces these;
	// compliance reasserts them so the audit report is self-contained.
	if !hasFile(p, "README.md") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityWarning,
			Message: "CC8.1: missing README — change management requires a release / change document",
			Hint:    "add README.md describing how to deploy and roll back",
		})
	}
	if !hasAnyDockerfile(p) {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityWarning,
			Message: "CC8.1: no Dockerfile — reproducible builds are part of change management",
			Hint:    "ship a Dockerfile so every release is a pinned, hashed image",
		})
	}

	// CC3.4 — risk assessment artefact.
	if !hasSecurityArtefact(p, "security.md", "docs/threat_model", "docs/risk_assessment") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityInfo,
			Message: "CC3.4: no risk-assessment artefact (SECURITY.md / docs/threat_model.*)",
			Hint:    "publish a SECURITY.md or docs/threat_model.md naming the data, the actors, and the mitigations",
		})
	}

	// Backup policy.
	if !projectMentions(p, "backup", "snapshot", "pg_dump", "barman", "wal-g") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityInfo,
			Message: "no backup policy declared (no 'backup' / 'snapshot' mention in README or scripts/)",
			Hint:    "document the backup cadence (e.g. nightly pg_dump to S3) and the restore drill",
		})
	}

	// MFA preference.
	if !projectMentions(p, "mfa", "totp", "webauthn", "passkey", "two-factor", "twofactor") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityWarning,
			Message: "SOC2 audit prefers MFA support — no mfa/totp/webauthn marker found",
			Hint:    "even an opt-in TOTP enrolment flow is enough to satisfy most SOC2 auditors",
		})
	}

	// Pen-test artefact.
	if !hasSecurityArtefact(p, "docs/pentest", "docs/security-audit", "docs/security_audit") {
		issues = append(issues, domain.Issue{
			Gate: domain.GateComplianceSOC2, Severity: domain.SeverityInfo,
			Message: "no pen-test / security-audit artefact (docs/pentest* or docs/security-audit*)",
			Hint:    "drop the most recent pentest report under docs/pentest-YYYY-MM.md so audits can find it",
		})
	}

	return issues
}

// projectImports returns true when any project file mentions one of the
// given import / package strings. Cheap O(N) substring scan; lower-cased
// once per file.
func projectImports(p *domain.Project, needles ...string) bool {
	for _, f := range p.Files {
		low := strings.ToLower(f.Content)
		for _, n := range needles {
			if strings.Contains(low, strings.ToLower(n)) {
				return true
			}
		}
	}
	return false
}

// projectMentions is the broader cousin of projectImports — looks across
// every file (including markdown / scripts) for any mention of the
// needle words. Used for compliance "did the operator at least name
// this concept" heuristics.
func projectMentions(p *domain.Project, needles ...string) bool {
	for _, f := range p.Files {
		low := strings.ToLower(f.Content)
		for _, n := range needles {
			if strings.Contains(low, strings.ToLower(n)) {
				return true
			}
		}
	}
	return false
}

// hasTLSMarker scans the deploy surface for any sign of TLS termination
// (cert paths, https://, certbot, lets-encrypt, ssl_certificate).
func hasTLSMarker(p *domain.Project) bool {
	markers := []string{
		"ssl_certificate", "ssl_certificate_key", "tls_cert", "tls_key",
		"letsencrypt", "certbot", "acme",
		"caddyfile", "caddy:",
		"https://",
		"servername", // nginx server blocks usually go with TLS in prod
	}
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		if !(strings.Contains(low, "dockerfile") ||
			strings.Contains(low, "docker-compose") ||
			strings.Contains(low, "nginx") ||
			strings.Contains(low, "caddy") ||
			strings.Contains(low, "traefik") ||
			strings.HasSuffix(low, ".tf") ||
			strings.HasSuffix(low, ".yaml") ||
			strings.HasSuffix(low, ".yml")) {
			continue
		}
		body := strings.ToLower(f.Content)
		for _, m := range markers {
			if strings.Contains(body, m) {
				return true
			}
		}
	}
	return false
}

// hasAnyDockerfile reports presence of any *Dockerfile* on the project.
// We match suffix so docker subprojects (Dockerfile.web, Dockerfile.api)
// still count.
func hasAnyDockerfile(p *domain.Project) bool {
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		base := filepathBase(low)
		if base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") {
			return true
		}
	}
	return false
}

// hasSecurityArtefact reports presence of any file whose path matches
// (case-insensitively) the given prefixes. Used to find SECURITY.md /
// threat models / pentest reports without prescribing the exact filename.
func hasSecurityArtefact(p *domain.Project, prefixes ...string) bool {
	for _, f := range p.Files {
		low := strings.ToLower(strings.TrimPrefix(f.Path, "/"))
		for _, pref := range prefixes {
			pl := strings.ToLower(pref)
			if low == pl || strings.HasPrefix(low, pl) || strings.HasSuffix(low, "/"+pl) {
				return true
			}
		}
	}
	return false
}
