package wowloop

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ---------------- Source models ----------------------------------
//
// These are intentionally compact value types specific to the
// wowloop package. They mirror just the fields the bundle needs from
// each upstream package so the wowloop builder can stay decoupled
// from execution / wallet / ledger / deploy / repair types.

// ExecutionSnapshot is the slice of the executions row the bundle
// needs. All money is decimal USD.
type ExecutionSnapshot struct {
	ID                string
	TenantID          string
	Status            string
	RevenueUSD        decimal.Decimal
	ProviderCostUSD   decimal.Decimal
	SandboxCostUSD    decimal.Decimal
	StorageCostUSD    decimal.Decimal
	DeploymentCostUSD decimal.Decimal
	GrossMarginPct    decimal.Decimal
	EndedAt           time.Time

	// WorkspaceID, when set, is the runtime workspace currently
	// allocated to this execution. The builder uses it to ask the
	// RuntimeSource for a live preview URL when the execution has
	// not yet reached a terminal state. Empty = no workspace
	// allocated (or the execution adapter doesn't track it).
	WorkspaceID string
}

// LedgerSnapshot is the per-execution rollup of ledger entries the
// bundle uses to detect "wallet hold released but balance low" — the
// trigger for the top_up next-best-action.
type LedgerSnapshot struct {
	BalanceUSD     decimal.Decimal
	HoldsActive    bool
	LastReleaseAt  time.Time
}

// PatchSnapshot is one applied patch. The bundle aggregates Path
// across every snapshot returned by PatchSource.PatchesFor to derive
// ChangedFiles + PatchCount.
type PatchSnapshot struct {
	ID        string
	Path      string
	AppliedAt time.Time
}

// RepairSnapshot is one repair attempt the finisher engaged. The
// bundle uses repair attempts to flag a gate stage as "repaired"
// rather than "fail".
type RepairSnapshot struct {
	GateName  string
	Succeeded bool
}

// GateSnapshot is one finisher gate verdict.
type GateSnapshot struct {
	Name        string
	Status      string // "pass" | "fail" | "skipped"
	IssuesCount int
}

// DeploySnapshot is the resolved deploy state for the execution. Any
// or all fields may be empty if no deploy ran.
type DeploySnapshot struct {
	PreviewURL    string
	ProductionURL string
}

// SecurityFindingSnapshot mirrors SecurityFinding but lives on the
// source side so the source package can populate it directly without
// importing wowloop.SecurityFinding.
type SecurityFindingSnapshot struct {
	Severity    string
	RuleID      string
	Path        string
	Line        int
	Summary     string
	BlocksDeploy bool
}

// ---------------- Source interfaces ------------------------------
//
// The builder consumes these six interfaces. Each one is the minimum
// surface required to populate one section of the bundle. The V22
// integration agent injects adapters that wrap the real packages
// (execution.Service, ledger.Service, finisher gate audit log,
// patch.Engine history, repair.Genome attempts, deploy.Service).

// ExecutionSource resolves the executions row.
type ExecutionSource interface {
	GetExecution(ctx context.Context, executionID string) (ExecutionSnapshot, error)
}

// LedgerSource exposes the per-execution ledger rollup.
type LedgerSource interface {
	LedgerFor(ctx context.Context, executionID string, tenantID string) (LedgerSnapshot, error)
}

// GateSource lists every gate verdict recorded for the execution.
type GateSource interface {
	GatesFor(ctx context.Context, executionID string) ([]GateSnapshot, error)
	SecurityFindingsFor(ctx context.Context, executionID string) ([]SecurityFindingSnapshot, error)
}

// PatchSource lists every patch applied during the execution.
type PatchSource interface {
	PatchesFor(ctx context.Context, executionID string) ([]PatchSnapshot, error)
}

// RepairSource lists every repair attempt the finisher engaged.
type RepairSource interface {
	RepairsFor(ctx context.Context, executionID string) ([]RepairSnapshot, error)
}

// DeploySource resolves the active preview + production URLs.
type DeploySource interface {
	DeployFor(ctx context.Context, executionID string) (DeploySnapshot, error)
}

// RuntimeSource resolves the live workspace preview URL for an
// in-flight execution. Used by the wow-loop builder so the studio
// iframe shows the running dev server before any deploy has succeeded.
//
// Implementations should return ("", nil) when no preview is
// available yet rather than an error — the builder treats "no
// preview" as "fall back to the deploy URL", not as a hard failure.
//
// nil-tolerant: DefaultBuilder.Runtime may stay nil and the bundle
// will simply use the DeploySource preview URL as today.
type RuntimeSource interface {
	PreviewURL(ctx context.Context, workspaceID string) (string, error)
}
