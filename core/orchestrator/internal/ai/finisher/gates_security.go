// Package finisher contains the Security gate event bridge. The AppSec core
// produces scanner findings; this file projects the gate's domain.Issue bag
// into execution events so the existing securityreport builder can read a
// stable, provider-agnostic feed.

package finisher

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
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
