package securityreport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// StandardBuilder is the in-process Builder the GraphQL resolver
// uses. It fans out to ExecutionSource + FindingSource + PolicySource
// and folds everything into a deterministic Report.
//
// The implementation is intentionally pure-go and dependency-light so
// it can be swapped into a Temporal activity, a CLI exporter, or the
// audit-export pipeline without dragging finisher state into the
// caller.
type StandardBuilder struct {
	Exec     ExecutionSource
	Findings FindingSource
	Policy   PolicySource
	// Now is injected for deterministic timestamps in callers that
	// freeze time; defaults to time.Now.UTC.
	Now func() time.Time
}

// NewStandardBuilder is the constructor main.go (via the integration
// agent) uses. Any of the sources may be nil — Build degrades to a
// well-formed empty Report so partial wiring on dev boxes does not
// 500 the resolver.
func NewStandardBuilder(exec ExecutionSource, findings FindingSource, policy PolicySource) *StandardBuilder {
	return &StandardBuilder{
		Exec:     exec,
		Findings: findings,
		Policy:   policy,
		Now:      func() time.Time { return time.Now().UTC() },
	}
}

// Build is the canonical surface. Steps:
//  1. resolve execution meta (tenant, status)
//  2. pull findings from the FindingSource
//  3. apply tenant policy (age window, deploy-block thresholds)
//  4. aggregate counts (SecretsFound, OutdatedDeps, OWASPCoverage)
//  5. derive Status, OverallScore, BlockedDeploy
func (b *StandardBuilder) Build(ctx context.Context, executionID string) (Report, error) {
	now := b.now()
	report := Report{
		ExecutionID:   executionID,
		Status:        "pass",
		OverallScore:  1.0,
		OWASPCoverage: emptyOWASPCoverage(),
		GeneratedAt:   now,
	}

	meta, err := b.resolveExecution(ctx, executionID)
	if err != nil {
		return Report{}, err
	}
	report.TenantID = meta.TenantID

	policy := b.resolvePolicy(ctx, meta.TenantID)

	findings, err := b.resolveFindings(ctx, executionID)
	if err != nil {
		return Report{}, err
	}
	findings = applyAgeFilter(findings, policy.MaxFindingAge, now)
	findings = ensureIDs(findings, executionID)
	report.Findings = findings

	report.SecretsFound = countCategory(findings, "secrets")
	report.OutdatedDeps = countCategory(findings, "deps")
	report.OWASPCoverage = computeOWASPCoverage(findings)
	report.Status = computeStatus(findings, meta.GateStatus)
	report.OverallScore = computeOverallScore(findings)
	report.BlockedDeploy = computeBlockedDeploy(report.Status, findings, policy)

	return report, nil
}

// resolveExecution falls back to a tenant-less stub when the source
// is nil so the resolver still returns a usable empty report.
func (b *StandardBuilder) resolveExecution(ctx context.Context, executionID string) (ExecutionMeta, error) {
	if b.Exec == nil {
		return ExecutionMeta{ID: executionID}, nil
	}
	meta, err := b.Exec.GetExecution(ctx, executionID)
	if err != nil {
		return ExecutionMeta{}, err
	}
	if meta.ID == "" {
		meta.ID = executionID
	}
	return meta, nil
}

func (b *StandardBuilder) resolvePolicy(ctx context.Context, tenantID string) TenantPolicy {
	if b.Policy == nil || tenantID == "" {
		return DefaultPolicy()
	}
	p, err := b.Policy.ForTenant(ctx, tenantID)
	if err != nil {
		return DefaultPolicy()
	}
	return p
}

func (b *StandardBuilder) resolveFindings(ctx context.Context, executionID string) ([]Finding, error) {
	if b.Findings == nil {
		// Empty slice is a successful "nothing to report yet" path —
		// the resolver maps this to a pass verdict with score 1.0.
		return nil, nil
	}
	return b.Findings.ByExecution(ctx, executionID)
}

func (b *StandardBuilder) now() time.Time {
	if b.Now == nil {
		return time.Now().UTC()
	}
	return b.Now().UTC()
}

// computeStatus folds finding severities + gate status into a single
// customer-visible verdict. Rules (per spec):
//
//   - any critical or high  → "fail"
//   - any medium            → "warning"
//   - otherwise             → "pass"
//
// A gate status of "blocked" or "fail" upgrades a "pass" to "fail" so
// a deployment that the finisher already blocked never reads as safe.
func computeStatus(findings []Finding, gateStatus string) string {
	worst := WorstSeverity(findings)
	status := "pass"
	switch worst {
	case SeverityCritical, SeverityHigh:
		status = "fail"
	case SeverityMedium:
		status = "warning"
	}
	switch strings.ToLower(strings.TrimSpace(gateStatus)) {
	case "fail", "blocked":
		if status == "pass" {
			status = "fail"
		}
	case "warning", "warn":
		if status == "pass" {
			status = "warning"
		}
	}
	return status
}

// computeOverallScore returns a 0..1 score where 1.0 means clean.
//
//	score = 1 - clamp((critical*0.5 + high*0.3 + medium*0.15 + low*0.05) / maxNorm)
//
// maxNorm = 4 keeps a small number of critical findings from saturating
// the score immediately — customers want to see a difference between
// "1 critical" and "10 criticals".
func computeOverallScore(findings []Finding) float64 {
	const maxNorm = 4.0
	var weighted float64
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			weighted += 0.50
		case SeverityHigh:
			weighted += 0.30
		case SeverityMedium:
			weighted += 0.15
		case SeverityLow:
			weighted += 0.05
		}
	}
	score := 1.0 - (weighted / maxNorm)
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// computeBlockedDeploy mirrors the V22 finisher policy: any critical
// blocks deploy. Tenant policies may additionally block on fail
// status, on any high severity, or both.
func computeBlockedDeploy(status string, findings []Finding, policy TenantPolicy) bool {
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			return true
		}
	}
	if policy.BlockOnHigh {
		for _, f := range findings {
			if f.Severity == SeverityHigh {
				return true
			}
		}
	}
	if policy.BlockOnFail && status == "fail" {
		return true
	}
	return false
}

func countCategory(findings []Finding, category string) int {
	n := 0
	for _, f := range findings {
		if strings.EqualFold(f.Category, category) {
			n++
		}
	}
	return n
}

// OWASP top-10 (2021) categories. Map keys are stable; values flip
// to true when at least one finding references the category via
// RuleID prefix ("owasp-a01-…") or via the Category field.
func emptyOWASPCoverage() map[string]bool {
	return map[string]bool{
		"A01": false, "A02": false, "A03": false, "A04": false, "A05": false,
		"A06": false, "A07": false, "A08": false, "A09": false, "A10": false,
	}
}

func computeOWASPCoverage(findings []Finding) map[string]bool {
	out := emptyOWASPCoverage()
	for _, f := range findings {
		key := extractOWASPKey(f)
		if key == "" {
			continue
		}
		if _, ok := out[key]; ok {
			out[key] = true
		}
	}
	return out
}

// extractOWASPKey looks for an "A0X" or "Axx" token in the RuleID or
// Category. We accept either a hyphenated or colon-separated layout
// so semgrep / custom rules both project cleanly.
func extractOWASPKey(f Finding) string {
	candidates := []string{strings.ToUpper(f.RuleID), strings.ToUpper(f.Category)}
	for _, c := range candidates {
		if idx := strings.Index(c, "A0"); idx >= 0 && idx+3 <= len(c) {
			key := c[idx : idx+3]
			if key[2] >= '1' && key[2] <= '9' {
				return key
			}
		}
		if idx := strings.Index(c, "A10"); idx >= 0 {
			return "A10"
		}
	}
	return ""
}

func applyAgeFilter(findings []Finding, maxAge time.Duration, now time.Time) []Finding {
	if maxAge <= 0 {
		return findings
	}
	cutoff := now.Add(-maxAge)
	out := findings[:0]
	for _, f := range findings {
		if f.DetectedAt.IsZero() || !f.DetectedAt.Before(cutoff) {
			out = append(out, f)
		}
	}
	return out
}

// ensureIDs fills any missing Finding.ID with a deterministic hash so
// the customer dashboard can dedupe across builder invocations.
func ensureIDs(findings []Finding, executionID string) []Finding {
	for i := range findings {
		if findings[i].ID != "" {
			continue
		}
		findings[i].ID = deriveFindingID(executionID, findings[i])
	}
	return findings
}

func deriveFindingID(executionID string, f Finding) string {
	h := sha256.New()
	h.Write([]byte(executionID))
	h.Write([]byte("|"))
	h.Write([]byte(f.RuleID))
	h.Write([]byte("|"))
	h.Write([]byte(f.Path))
	h.Write([]byte("|"))
	h.Write([]byte(strconv.Itoa(f.Line)))
	return "f_" + hex.EncodeToString(h.Sum(nil))[:16]
}
