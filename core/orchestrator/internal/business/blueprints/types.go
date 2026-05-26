// Package blueprints is the V22 data-driven starter registry. It
// replaces the ~30 hand-written scaffolders the proof pack retired
// with a small set of high-leverage starters, each tracked as an
// economic unit. A blueprint carries its expected cost (the "prior")
// and its expected time-to-preview so ProfitGuard can rank them
// before any expensive call runs, and the Blueprint Profit Dashboard
// can decide which to promote vs. retire based on realised margin.
//
// This package is intentionally self-contained: it does not import
// the finisher engine, the wallet, or the ledger. Agent 8 owns the
// cross-wiring — the finisher dispatcher will call Executor.Execute,
// and the execution.Commit flow will call StatsService.RecordRun.
package blueprints

import (
	"io/fs"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Blueprint is the registry-side description of one starter. It is
// loaded once at process start by NewBuiltInRegistry and never
// mutated; callers should treat all fields as read-only.
type Blueprint struct {
	// ID is the stable string key used everywhere — blueprint_stats
	// row PK, ProfitGuard decisions, dashboards, GraphQL queries.
	// Keep these kebab-case and short.
	ID string
	// Name is the human-readable label shown in UI.
	Name string
	// Description is one paragraph of "what you get + what to use it
	// for" — rendered into the Blueprint picker and into the
	// execution timeline event when the blueprint is selected.
	Description string
	// Category is a coarse bucket used by the dashboard and the
	// Recommend filter: "webapp", "api", "static", etc.
	Category string
	// CostPriorUSD is the expected platform cost of an execution
	// that uses this blueprint, in USD. ProfitGuard uses this as the
	// baseline cost-of-action when comparing blueprints before a
	// real run has produced stats. Update it once enough real runs
	// have landed (the stats service computes the realised value).
	CostPriorUSD decimal.Decimal
	// ExpectedTimeToPreviewSec is the expected time from "blueprint
	// selected" to "preview URL available" in seconds. Used to
	// pre-rank blueprints when the user explicitly asks for
	// "fastest preview".
	ExpectedTimeToPreviewSec int
	// SupportedGates names the finisher gates this blueprint is
	// known to satisfy on its own (no additional patches required).
	// Empty means "no implicit guarantees". The finisher consults
	// this list to skip redundant gate runs immediately after
	// scaffolding.
	SupportedGates []string
	// Files is the verbatim file payload written into the workspace
	// when this blueprint executes. Loaded from the embedded
	// templates/ tree by blueprints_data.go.
	Files []TemplateFile
}

// TemplateFile is one file inside a blueprint payload. Path is the
// workspace-relative target (e.g. "app/page.tsx"); Content is the
// exact bytes; Mode is the desired file mode (defaults to 0o644 when
// zero).
type TemplateFile struct {
	Path    string
	Content string
	Mode    fs.FileMode
}

// RunOutcome is the per-execution observation that StatsService.
// RecordRun turns into an immutable blueprint_runs row plus an
// atomic UPSERT against the blueprint_stats rollup. One outcome is
// produced when an execution that used a blueprint commits — the
// integration loop wires this in.
type RunOutcome struct {
	BlueprintID          string
	ExecutionID          uuid.UUID
	TenantID             uuid.UUID
	RevenueUSD           decimal.Decimal
	CostUSD              decimal.Decimal
	CompletionScore      float64
	PreviewSuccess       bool
	Repaired             bool
	Refunded             bool
	TimeToPreviewSeconds int
}

// Stats is the dashboard-ready view of one blueprint's rolled-up
// performance. All averages are derived from the totals stored on
// blueprint_stats so reads stay a single row lookup.
type Stats struct {
	BlueprintID         string
	Executions          int64
	PreviewSuccess      int64
	Refunds             int64
	RepairCount         int64
	AvgRevenueUSD       decimal.Decimal
	AvgCostUSD          decimal.Decimal
	GrossMarginPct      decimal.Decimal // (rev - cost) / rev * 100, 0 when rev = 0
	AvgCompletionScore  decimal.Decimal
	AvgTimeToPreviewSec decimal.Decimal
	UpdatedAt           time.Time
}
