// Security-report adapter layer — V22 Wave-3 follow-up.
//
// Closes the wired-degraded gap reported by Agent 39: the original
// wireup passed nil for the FindingSource + PolicySource, so the
// resolver always returned an empty "pass" report regardless of what
// the finisher Security gate actually found.
//
// This file owns the two adapter types:
//
//   - secrFindingAdapter projects execution_events whose event_type
//     belongs to the closed security-findings set into the
//     securityreport.Finding shape the Builder consumes.
//   - secrPolicyAdapter resolves a tenant to its TenantPolicy. V1
//     returns DefaultPolicy() for every tenant; the per-tenant
//     override store lands in a later wave.
//
// Both adapters degrade to empty/default rather than erroring so the
// resolver path stays robust on dev boxes that have never run a
// finisher Security gate.
package wireup

import (
	"context"
	"strconv"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/securityreport"
)

// secrFindingAdapter pulls security-finding event payloads off the
// execution.Service and normalises each one into a Finding.
type secrFindingAdapter struct {
	exec execution.Service
}

// ByExecution satisfies securityreport.FindingSource.
//
// Tolerant by contract: an execution with no security events returns
// an empty slice (not an error) so the Builder still emits a clean
// pass-with-empty-findings report. An underlying store error
// propagates — the Builder treats that as a Build-level failure.
func (a *secrFindingAdapter) ByExecution(ctx context.Context, executionID string) ([]securityreport.Finding, error) {
	if a == nil || a.exec == nil {
		return nil, nil
	}
	payloads, err := a.exec.LatestSecurityFindings(ctx, executionID)
	if err != nil {
		return nil, err
	}
	out := make([]securityreport.Finding, 0, len(payloads))
	for _, p := range payloads {
		out = append(out, payloadToFinding(p))
	}
	return out, nil
}

// secrPolicyAdapter resolves a tenant to its security policy. V1
// returns DefaultPolicy() unconditionally; a future agent will swap
// the body for a real tenant-policy store query without touching the
// wireup call site.
type secrPolicyAdapter struct{}

// ForTenant satisfies securityreport.PolicySource.
func (a *secrPolicyAdapter) ForTenant(_ context.Context, _ string) (securityreport.TenantPolicy, error) {
	return securityreport.DefaultPolicy(), nil
}

// payloadToFinding projects a raw event payload (the JSON shape the
// finisher Security gate hook publishes) into the canonical Finding.
// Every field has a sensible default so a partial payload never
// degrades a Build into an error.
//
// Category inference — when the payload omits `category` we look at
// the rule_id prefix; the table is:
//
//	rule_id prefix      → category
//	------------------- → ----------
//	secret-*, gitleaks- → secrets
//	trufflehog-*        → secrets
//	deps-*, npm-*       → deps
//	govuln-*, cve-*     → deps
//	owasp-*, semgrep-*  → code
//	config-*            → config
//	policy-*            → policy
//	(no match)          → code
func payloadToFinding(p map[string]any) securityreport.Finding {
	ruleID := stringAt(p, "rule_id")
	if ruleID == "" {
		ruleID = "unknown"
	}
	severity := securityreport.NormaliseSeverity(stringAt(p, "severity"))
	if severity == securityreport.SeverityInfo {
		// Some upstreams stash the severity under "level" instead.
		severity = securityreport.NormaliseSeverity(stringAt(p, "level"))
	}
	category := stringAt(p, "category")
	if category == "" {
		category = inferCategoryFromRuleID(ruleID)
	}
	return securityreport.Finding{
		ID:          stringAt(p, "id"),
		Severity:    severity,
		RuleID:      ruleID,
		Category:    category,
		Path:        stringAt(p, "path"),
		Line:        intAt(p, "line"),
		Summary:     firstNonEmpty(stringAt(p, "summary"), stringAt(p, "message")),
		Remediation: firstNonEmpty(stringAt(p, "remediation"), stringAt(p, "hint")),
		DetectedAt:  timeAt(p, "detected_at"),
	}
}

// inferCategoryFromRuleID maps the rule_id prefix to a securityreport
// category. The table is documented on payloadToFinding above.
func inferCategoryFromRuleID(ruleID string) string {
	id := strings.ToLower(strings.TrimSpace(ruleID))
	switch {
	case strings.HasPrefix(id, "secret-"),
		strings.HasPrefix(id, "secrets-"),
		strings.HasPrefix(id, "gitleaks-"),
		strings.HasPrefix(id, "trufflehog-"):
		return "secrets"
	case strings.HasPrefix(id, "deps-"),
		strings.HasPrefix(id, "dep-"),
		strings.HasPrefix(id, "npm-"),
		strings.HasPrefix(id, "npm_audit-"),
		strings.HasPrefix(id, "govuln-"),
		strings.HasPrefix(id, "govulncheck-"),
		strings.HasPrefix(id, "cve-"),
		strings.HasPrefix(id, "osv-"):
		return "deps"
	case strings.HasPrefix(id, "config-"),
		strings.HasPrefix(id, "dockerfile-"):
		return "config"
	case strings.HasPrefix(id, "policy-"):
		return "policy"
	case strings.HasPrefix(id, "owasp-"),
		strings.HasPrefix(id, "semgrep-"),
		strings.HasPrefix(id, "sast-"):
		return "code"
	}
	return "code"
}

// stringAt returns p[key] as a string, tolerating absent keys and
// non-string values. Numeric values are stringified so a payload that
// stashes a line as int doesn't blank the field.
func stringAt(p map[string]any, key string) string {
	if v, ok := p[key]; ok {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			// JSON unmarshal of "line": 12 lands as float64; render
			// without the decimal for human-friendly output.
			return strconv.FormatFloat(t, 'f', -1, 64)
		case int:
			return strconv.Itoa(t)
		case int64:
			return strconv.FormatInt(t, 10)
		case bool:
			return strconv.FormatBool(t)
		}
	}
	return ""
}

// intAt returns p[key] as an int, tolerating string + float forms.
// Falls back to 0 on missing/unparseable input.
func intAt(p map[string]any, key string) int {
	if v, ok := p[key]; ok {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case int64:
			return int(t)
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(t))
			if err == nil {
				return n
			}
		}
	}
	return 0
}

// timeAt returns p[key] parsed as RFC 3339 or returns a zero time if
// the field is absent or unparseable. The Builder treats zero
// detected_at as "no age filtering applies".
func timeAt(p map[string]any, key string) time.Time {
	raw := stringAt(p, key)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// firstNonEmpty returns the first non-empty string in `vals`.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
