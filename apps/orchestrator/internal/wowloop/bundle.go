// Package wowloop assembles the V22 "Customer Wow Loop" support
// bundle — one read-only struct that bundles everything a user needs
// to evaluate a finished execution: preview URL, changed files, gate
// report, security report, cost report, and a Next Best Action.
//
// The package keeps its own local source interfaces (sources.go) so
// it never imports the execution / wallet / ledger / deploy / repair
// packages directly. The V22 integration agent wires concrete
// adapters that satisfy these interfaces.
package wowloop

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// SupportBundle is the single-shot read view the
// executionSupportBundle GraphQL query returns. Every field is
// derived; the bundle never mutates state.
type SupportBundle struct {
	ExecutionID string
	TenantID    string
	Status      string

	// PreviewURL is the most recent preview deploy for this
	// execution. Empty when no preview deploy has succeeded yet.
	PreviewURL string
	// ProductionURL is the promoted deploy, when one exists.
	ProductionURL string

	// ChangedFiles is the union of patch.Path across every patch
	// applied during this execution. Sorted, deduped.
	ChangedFiles []string
	// PatchCount is the number of patches that landed.
	PatchCount int

	GateReport     GateReport
	SecurityReport SecurityReport
	CostReport     CostReport
	NextBestAction NextAction

	GeneratedAt time.Time
}

// GateReport summarises every finisher gate stage and a 0..1
// completion score derived from how many stages passed.
type GateReport struct {
	Stages          []GateStage
	CompletionScore float64
}

// GateStage is one finisher gate verdict.
type GateStage struct {
	// Name matches the GateName constant in finisher/domain.
	Name string
	// Status is one of: "pass" | "fail" | "repaired" | "skipped".
	Status string
	// IssuesCount is the number of issues the gate emitted; non-zero
	// even on "repaired" so the UI can surface the original load.
	IssuesCount int
}

// SecurityReport rolls up the security gate output for the bundle.
type SecurityReport struct {
	Findings []SecurityFinding
	// PassRate is (1 - blockingFindings/totalFindings), or 1.0 when
	// there are no findings at all.
	PassRate float64
	// BlockedDeploy is true if any high/critical finding blocked the
	// deploy gate.
	BlockedDeploy bool
}

// SecurityFinding is one issue surfaced by the security gate.
type SecurityFinding struct {
	Severity string
	RuleID   string
	Path     string
	Line     int
	Summary  string
}

// CostReport is the realised economics for this execution. Sourced
// from the executions row + ledger entries that close it out.
type CostReport struct {
	RevenueUSD        decimal.Decimal
	ProviderCostUSD   decimal.Decimal
	SandboxCostUSD    decimal.Decimal
	StorageCostUSD    decimal.Decimal
	DeploymentCostUSD decimal.Decimal
	GrossMarginPct    decimal.Decimal
}

// NextAction is the "what should the user do next?" heuristic the
// bundle promotes to a CTA card.
type NextAction struct {
	// Kind is one of: "deploy" | "review_patch" | "top_up" |
	// "fix_security_finding" | "share_preview".
	Kind string
	// Title is the verb-first one-liner the UI renders on the CTA.
	Title string
	// Reason explains, in one sentence, why this action came up.
	Reason string
	// CTA is the URL or GraphQL operation hint that fulfils the
	// action. Optional — the UI knows the canonical destination from
	// Kind alone, but the bundle can override it.
	CTA string
}

// Builder produces a SupportBundle for one executionID. The
// implementation may fan out to several sources in parallel; the
// returned bundle is fully self-contained.
type Builder interface {
	Build(ctx context.Context, executionID string) (SupportBundle, error)
}
