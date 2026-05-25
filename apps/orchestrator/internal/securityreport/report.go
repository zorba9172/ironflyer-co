package securityreport

import (
	"context"
	"time"
)

// Finding is the canonical per-issue rectangle the report exposes.
// We deliberately keep it provider-agnostic — the upstream may be
// semgrep, govulncheck, gitleaks, trufflehog, npm audit, or the LLM
// tiebreaker; the customer just sees the same shape.
type Finding struct {
	// ID is stable per (execution, rule, path, line). The Builder
	// derives it deterministically so the dashboard can dedupe across
	// repeat runs.
	ID          string
	Severity    Severity
	RuleID      string // e.g. "secret-detected", "owasp-a01-injection"
	Category    string // "secrets" | "deps" | "code" | "config" | "policy"
	Path        string
	Line        int
	Summary     string
	Remediation string
	DetectedAt  time.Time
}

// Report is the per-execution security verdict the customer queries.
// Status / OverallScore / BlockedDeploy are derived from Findings via
// the Builder — never mutate them after construction.
type Report struct {
	ExecutionID   string
	TenantID      string
	Status        string  // "pass" | "fail" | "warning"
	OverallScore  float64 // 0..1; higher is safer
	Findings      []Finding
	SecretsFound  int
	OutdatedDeps  int
	OWASPCoverage map[string]bool
	GeneratedAt   time.Time
	BlockedDeploy bool
}

// Builder turns an execution ID into a Report by reading from the
// upstream FindingSource + ExecutionSource. Implementations must be
// pure: no side effects, no audit writes — those land in the
// finisher gate path that originally produced the findings.
type Builder interface {
	Build(ctx context.Context, executionID string) (Report, error)
}
