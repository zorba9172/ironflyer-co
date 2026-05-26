// Package dashboards is the read-only aggregator that powers the four
// V22 proof dashboards (Profit, Scale, Cohort, Blueprint).
//
// The package deliberately depends on local source interfaces rather
// than importing sibling V22 packages (wallet, ledger, execution,
// blueprints, profitguard). That keeps the dashboards a pure read
// layer — Agent 8 wires concrete adapters that satisfy these
// interfaces and the dashboards stay free of cross-package coupling.
package dashboards

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// LedgerSummary is a compact ledger entry for the recent-feed view.
type LedgerSummary struct {
	ID           string
	TenantID     string
	ExecutionID  string
	EntryType    string
	Direction    string
	AmountUSD    decimal.Decimal
	CreatedAt    time.Time
}

// ExecutionSummary is the dashboards' view of an execution.
type ExecutionSummary struct {
	ID                string
	TenantID          string
	BlueprintID       string
	Status            string
	BudgetUSD         decimal.Decimal
	SpentUSD          decimal.Decimal
	RevenueUSD        decimal.Decimal
	CompletionScore   float64
	GrossMarginPct    float64
	CreatedAt         time.Time
	EndedAt           time.Time
}

// Cohort is the cohort dashboard row (one calendar month).
type Cohort struct {
	Month                 time.Time
	NewPayingUsers        int
	SecondExecutionUsers  int
	Day7RepeatUsers       int
	Day30RepeatUsers      int
	AvgSpendUSD           float64
	GrossMarginPct        float64
	CompletionRate        float64
	RefundRate            float64
	SupportTicketsPerExec float64
}

// BlueprintStats is the per-blueprint stats row surfaced by the
// blueprint dashboard. Named with the package's own type identity so
// it stays decoupled from the blueprints package's own type.
type BlueprintStats struct {
	BlueprintID         string
	Executions          int
	AvgRevenueUSD       float64
	AvgCostUSD          float64
	GrossMarginPct      float64
	PreviewSuccess      int
	Refunds             int
	RepairCount         int
	AvgCompletionScore  float64
}

// LedgerSource is the read view the dashboards need on the ledger.
type LedgerSource interface {
	// SumByType returns the summed amount per entry_type within the
	// window. If types is nil all entry types are summed.
	SumByType(ctx context.Context, since, until time.Time, types []string) (map[string]decimal.Decimal, error)
	// RecentEntries returns the latest N ledger entries across all
	// tenants (operator view).
	RecentEntries(ctx context.Context, limit int) ([]LedgerSummary, error)
}

// ExecutionSource is the read view the dashboards need on executions.
type ExecutionSource interface {
	// CountsByStatus returns the count of executions per status string
	// within the window.
	CountsByStatus(ctx context.Context, since, until time.Time) (map[string]int, error)
	// CountsByCohort returns one Cohort row per calendar month at or
	// after sinceCohortMonth.
	CountsByCohort(ctx context.Context, sinceCohortMonth time.Time) ([]Cohort, error)
	// Recent returns the latest N executions.
	Recent(ctx context.Context, limit int) ([]ExecutionSummary, error)
	// ActiveCount returns the live count of in-flight executions.
	ActiveCount(ctx context.Context) (int, error)
	// QueuedCount returns the live count of executions waiting to
	// start (created + admitted).
	QueuedCount(ctx context.Context) (int, error)
	// AverageQueueWaitSec returns the mean wait (created_at →
	// admitted_at) in seconds across executions admitted since `since`.
	AverageQueueWaitSec(ctx context.Context, since time.Time) (float64, error)
}

// BlueprintSource is the read view for blueprint statistics.
type BlueprintSource interface {
	// AllStats returns one BlueprintStats row per known blueprint.
	AllStats(ctx context.Context) ([]BlueprintStats, error)
}

// ScaleSource is the read view for the live-scale metrics the Scale
// dashboard surfaces.
type ScaleSource interface {
	ActiveExecutions(ctx context.Context) (int, error)
	QueueDepth(ctx context.Context) (int, error)
	WorkerUtilizationPct(ctx context.Context) (float64, error)
	SandboxCapacity(ctx context.Context) (int, error)
}

// tenantCtxKey is the unexported key used to carry the caller's
// tenant id through the dashboards read layer without changing the
// existing source interfaces (the ClickHouse + memory adapters live
// in sibling packages and pin the interface shape with compile-time
// guards).
//
// Adapters that can scope by tenant (the Postgres ledger / execution
// / blueprint adapters) MUST honour TenantFromContext for every
// per-tenant dashboard (Profit, Cohort, Blueprint). When the value
// is the empty string the call is operator-wide (Scale dashboard);
// per-tenant Service methods refuse to run with an empty tenant so
// a missing context value never silently leaks cross-tenant rows.
type tenantCtxKeyType struct{}

var tenantCtxKey = tenantCtxKeyType{}

// WithTenant returns a new context carrying tenantID. Service.Profit /
// Cohort / BlueprintDashboard set this before delegating to the
// builders; adapters read it to apply WHERE tenant_id = $1.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantCtxKey, tenantID)
}

// TenantFromContext returns the tenant id previously stored via
// WithTenant, or "" when the call is operator-wide / unscoped.
func TenantFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantCtxKey).(string)
	return v
}
