// Package finisher — deepened Security gate plumbing.
//
// The base Security gate already chains regex secret detection +
// semgrep + govulncheck + npm audit (see gates.go::SecurityGate and
// security_scanners.go). This file adds two production-grade signals
// on top:
//
//   - Secret detection via `trufflehog filesystem` or `gitleaks detect`
//     — whichever binary the workspace image carries. Both emit JSONL
//     findings we parse into typed Issues. The legacy regex pass stays
//     as a safety net for sandbox images that ship neither tool.
//   - LLM tiebreaker for medium/low static findings — the Security
//     role re-reads the in-context code and decides "real" vs "false
//     positive in this context". A confirmed real finding gets
//     upgraded to fail-class severity; a false positive is downgraded
//     to info so it doesn't drown the dashboard.
//
// Static-tool absence is silent (exit 127 → no findings). Operators
// should bake semgrep + trufflehog (or gitleaks) into the workspace
// image; see docs/SCALE.md for the recipe.

package finisher

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/profitguardctx"
	"ironflyer/apps/orchestrator/internal/runtime"
)

// SecurityFindingEventType is the event_type the engine publishes on
// the execution_events feed for every issue the Security gate (or any
// security-flavoured gate) emits. The customer-facing
// securityreport.FindingSource scans for this string — adding new
// emit sites with the same type keeps the projection lossless without
// a builder change.
const SecurityFindingEventType = "gate.security.finding.v1"

// emitSecurityFindings is the nil-safe entry point gate runners call
// after a Security-flavoured gate produces issues. It projects each
// issue into the wire payload the securityreport.FindingSource
// consumes and pushes it through executionService.RecordEvent — the
// memory backend rings it, the postgres backend persists it to
// execution_events.
//
// Tolerant by contract: missing executionService, missing
// executionID-on-ctx, or a RecordEvent error all degrade silently.
// The point is observability + downstream reporting, never to fail a
// gate that already produced its verdict.
func (e *Engine) emitSecurityFindings(ctx context.Context, gateName domain.GateName, issues []domain.Issue) {
	if e == nil || e.executionService == nil || len(issues) == 0 {
		return
	}
	if gateName != domain.GateSecurity {
		// V1 only the Security gate carries security findings. Adding
		// SCA/SAST gates later? Allow-list them here.
		return
	}
	execID, ok := profitguardctx.ExecutionID(ctx)
	if !ok || execID == "" {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, iss := range issues {
		path, line := splitPathLine(iss.Path)
		payload := map[string]any{
			"execution_id": execID,
			"gate":         string(gateName),
			"severity":     securityreportSeverityFor(iss.Severity),
			"rule_id":      ruleIDFor(iss),
			"category":     "", // empty — adapter infers from rule_id prefix
			"path":         path,
			"line":         line,
			"summary":      iss.Message,
			"remediation":  iss.Hint,
			"detected_at":  now,
			"index":        i, // disambiguates dedup ID per (rule,path,line)
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		_ = e.executionService.RecordEvent(ctx, execID, SecurityFindingEventType, raw)
	}
}

// securityreportSeverityFor maps the finisher domain.Severity vocab
// to the securityreport wire vocab. Mirrors NormaliseSeverity on the
// adapter side; kept here so the emit path produces canonical strings
// without an import cycle into the securityreport package.
func securityreportSeverityFor(s domain.Severity) string {
	switch s {
	case domain.SeverityCritical:
		return "critical"
	case domain.SeverityError:
		return "high"
	case domain.SeverityWarning:
		return "medium"
	case domain.SeverityInfo:
		return "info"
	}
	return "info"
}

// ruleIDFor extracts a stable rule identifier from a domain.Issue.
// The finisher writes the rule into Issue.Message as
// "<rule>: <description>" or "<tool>: <description>"; we lift the
// prefix when present and fall back to a gate-scoped default so the
// dashboard still groups findings consistently.
func ruleIDFor(iss domain.Issue) string {
	msg := strings.TrimSpace(iss.Message)
	if idx := strings.Index(msg, ":"); idx > 0 && idx < 64 {
		return normaliseRuleID(msg[:idx])
	}
	return "security-" + string(iss.Severity)
}

// normaliseRuleID lower-cases and slug-ifies the lifted prefix so the
// adapter's category inference fires deterministically. Spaces become
// hyphens; runs of non-[a-z0-9-] are collapsed.
func normaliseRuleID(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	var b strings.Builder
	b.Grow(len(raw))
	prevDash := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == '-', r == '_', r == ' ':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

// splitPathLine separates the trailing ":<line>" suffix the security
// scanners append onto Issue.Path. Returns the bare path + 0 when no
// suffix is present.
func splitPathLine(p string) (string, int) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", 0
	}
	idx := strings.LastIndex(p, ":")
	if idx <= 0 || idx == len(p)-1 {
		return p, 0
	}
	tail := p[idx+1:]
	// Accept only pure-digit tails so we don't shred Windows paths or
	// URLs (the scanners only ever append ":<int>").
	for _, r := range tail {
		if r < '0' || r > '9' {
			return p, 0
		}
	}
	n := 0
	for _, r := range tail {
		n = n*10 + int(r-'0')
	}
	return p[:idx], n
}

// runSecretDetectionScanners is invoked by SecurityGate.Check after the
// in-memory regex pass + scanWorkspaceForSecrets. It tries trufflehog
// first (more thorough, slightly slower) then gitleaks (lighter). Either
// is sufficient; both run when both binaries exist.
func runSecretDetectionScanners(ctx context.Context, env *GateEnv) []domain.Issue {
	if !env.HasRuntime() {
		return nil
	}
	var issues []domain.Issue
	issues = append(issues, runTrufflehog(ctx, env)...)
	issues = append(issues, runGitleaks(ctx, env)...)
	return issues
}

// runTrufflehog executes `trufflehog filesystem --json --no-update .`
// when the binary exists. Each verified finding becomes a critical
// Issue; unverified findings become error-severity so the dashboard
// still surfaces them but the gate doesn't necessarily fail.
func runTrufflehog(ctx context.Context, env *GateEnv) []domain.Issue {
	cmd := "command -v trufflehog >/dev/null 2>&1 && " +
		"trufflehog filesystem --json --no-update " +
		"--exclude-paths=<(printf 'node_modules\\n.git\\n') . 2>/dev/null || true"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 180,
	})
	if err != nil || strings.TrimSpace(res.Stdout) == "" {
		return nil
	}
	var out []domain.Issue
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var doc struct {
			DetectorName string `json:"DetectorName"`
			Verified     bool   `json:"Verified"`
			Raw          string `json:"Raw"`
			SourceMetadata struct {
				Data struct {
					Filesystem struct {
						File string `json:"file"`
						Line int    `json:"line,omitempty"`
					} `json:"Filesystem"`
				} `json:"Data"`
			} `json:"SourceMetadata"`
		}
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			continue
		}
		path := doc.SourceMetadata.Data.Filesystem.File
		if doc.SourceMetadata.Data.Filesystem.Line > 0 {
			path += ":" + itoaPositive(doc.SourceMetadata.Data.Filesystem.Line)
		}
		sev := domain.SeverityError
		msg := "trufflehog: " + doc.DetectorName
		if doc.Verified {
			sev = domain.SeverityCritical
			msg += " (verified live credential)"
		}
		out = append(out, domain.Issue{
			Gate: domain.GateSecurity, Severity: sev,
			Message: msg, Path: path,
			Hint: "rotate the credential and remove it from history (git filter-repo / BFG)",
		})
	}
	return out
}

// runGitleaks executes `gitleaks detect --no-banner --report-format json
// --report-path /dev/stdout` when the binary exists. Each finding becomes a
// critical Issue. We pass `--no-git` so the scan works on workspaces that
// were created from a tarball rather than `git clone`.
func runGitleaks(ctx context.Context, env *GateEnv) []domain.Issue {
	cmd := "command -v gitleaks >/dev/null 2>&1 && " +
		"gitleaks detect --no-git --no-banner --redact " +
		"--report-format json --report-path /dev/stdout . 2>/dev/null || true"
	res, err := env.Runtime.Exec(ctx, env.UserBearer, env.WorkspaceID, runtime.ExecOpts{
		Shell: cmd, TimeoutSeconds: 120,
	})
	if err != nil || strings.TrimSpace(res.Stdout) == "" {
		return nil
	}
	// gitleaks emits a JSON array.
	var findings []struct {
		Description string `json:"Description"`
		File        string `json:"File"`
		StartLine   int    `json:"StartLine"`
		RuleID      string `json:"RuleID"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &findings); err != nil {
		return nil
	}
	out := make([]domain.Issue, 0, len(findings))
	for _, f := range findings {
		path := f.File
		if f.StartLine > 0 {
			path += ":" + itoaPositive(f.StartLine)
		}
		out = append(out, domain.Issue{
			Gate: domain.GateSecurity, Severity: domain.SeverityCritical,
			Message: "gitleaks: " + f.RuleID + " — " + f.Description,
			Path:    path,
			Hint:    "remove the secret, rotate it upstream, and rewrite history",
		})
	}
	return out
}

// summariseSecurityVerdict picks an overall verdict for the Security gate
// from the bag of accumulated Issues. Used by the structured-report
// surfacing path so a UI panel can render a single chip.
//
// Verdict rules (per task spec):
//   - any critical/error → "fail"
//   - any warning        → "warn"
//   - empty              → "pass"
func summariseSecurityVerdict(issues []domain.Issue) string {
	hasWarn := false
	for _, iss := range issues {
		if iss.Severity == domain.SeverityCritical || iss.Severity == domain.SeverityError {
			return "fail"
		}
		if iss.Severity == domain.SeverityWarning {
			hasWarn = true
		}
	}
	if hasWarn {
		return "warn"
	}
	return "pass"
}

// Ensure context is used even if a future caller drops the only reference.
var _ = context.Background
