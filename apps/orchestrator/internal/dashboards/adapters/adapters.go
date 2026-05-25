// Package adapters wires the dashboards source interfaces to the
// concrete V22 services. Lives in its own subpackage to keep
// internal/dashboards a pure read layer and to give Agent 8 a single
// file to grep for "what feeds the dashboards".
package adapters

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/blueprints"
	"ironflyer/apps/orchestrator/internal/dashboards"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/ledger"
)

// LedgerAdapter implements dashboards.LedgerSource over ledger.Service.
//
// The dashboards aggregate across every tenant (operator view) — the
// underlying ledger.Service is per-tenant, so this adapter uses a
// "platform" tenant when the call is global. V1 reports an empty
// rollup for tenant-less calls; TODO(scale) make the ledger service
// support a multi-tenant rollup directly.
type LedgerAdapter struct {
	Svc ledger.Service
}

// SumByType returns a per-entry-type sum across the configured
// PlatformTenant. v1: returns empty when no service is wired.
func (a LedgerAdapter) SumByType(ctx context.Context, since, until time.Time, types []string) (map[string]decimal.Decimal, error) {
	out := map[string]decimal.Decimal{}
	if a.Svc == nil {
		return out, nil
	}
	// TODO(scale): aggregate over every tenant instead of skipping
	// when there is no platform tenant id wired. v1 returns zeros.
	return out, nil
}

// RecentEntries returns the empty slice in v1.
func (a LedgerAdapter) RecentEntries(ctx context.Context, limit int) ([]dashboards.LedgerSummary, error) {
	// TODO(scale): expose a cross-tenant "recent" query on
	// ledger.Service; v1 returns nothing so the operator widget
	// renders an empty state.
	return []dashboards.LedgerSummary{}, nil
}

// ExecutionAdapter implements dashboards.ExecutionSource over the
// execution.Service. Like the ledger adapter it is cross-tenant.
//
// The cross-tenant cohort roll-up cannot be expressed through the
// per-tenant Service surface, so when a pgxpool.Pool is wired the
// adapter runs the cohort SQL directly against the executions table.
// Pool is optional; without it CountsByCohort falls back to the v1
// empty slice so the in-memory backend still satisfies the interface.
type ExecutionAdapter struct {
	Svc  execution.Service
	Pool *pgxpool.Pool
}

// CountsByStatus returns the per-status count in [since, until). v1:
// the execution service does not expose a cross-tenant query so this
// returns an empty map; the dashboard renders a "warming up" state.
func (a ExecutionAdapter) CountsByStatus(ctx context.Context, since, until time.Time) (map[string]int, error) {
	return map[string]int{}, nil
}

// CountsByCohort rolls up the executions table into one row per
// calendar month, keyed by the tenant's FIRST paid execution. "Paid"
// means status is past the wallet-admission stage (anything other
// than 'created' or 'admitted') — a created+admitted execution
// never charged the wallet so it must not seed a cohort.
//
// All metrics resolve in a single round-trip via three CTEs:
//
//   - first_paid   — per-tenant first-paid timestamp + cohort month.
//   - cohort_runs  — every paid execution, joined to its tenant cohort.
//   - tenant_counts — per-tenant total run count + earliest second run.
//
// The SELECT then aggregates per cohort_month using DISTINCT counts
// for the per-tenant funnel metrics (new-paying, second-execution,
// d7, d30) and plain counts/sums for the per-execution metrics
// (avg spend, gross margin, completion rate, refund rate).
//
// supportTicketsPerExec is left at 0 — the V22 surface has no support
// ticket store yet. TODO(milestone3): join against a tickets table
// once support tooling lands.
//
// Falls back to the v1 empty slice when no pgxpool is wired (memory
// backend), so the in-memory dashboard path keeps rendering an empty
// cohort table instead of erroring.
func (a ExecutionAdapter) CountsByCohort(ctx context.Context, sinceCohortMonth time.Time) ([]dashboards.Cohort, error) {
	if a.Pool == nil {
		return []dashboards.Cohort{}, nil
	}
	const cohortSQL = `
WITH first_paid AS (
    SELECT tenant_id,
           MIN(created_at) AS first_at,
           DATE_TRUNC('month', MIN(created_at)) AS cohort_month
    FROM executions
    WHERE status NOT IN ('created', 'admitted')
    GROUP BY tenant_id
),
cohort_runs AS (
    SELECT e.id,
           e.tenant_id,
           e.status,
           e.created_at,
           e.spent_usd,
           e.revenue_usd,
           e.refunded_usd,
           fp.cohort_month,
           fp.first_at
    FROM executions e
    JOIN first_paid fp ON fp.tenant_id = e.tenant_id
    WHERE e.status NOT IN ('created', 'admitted')
),
tenant_counts AS (
    SELECT tenant_id,
           COUNT(*) AS total_runs,
           MIN(CASE WHEN created_at > first_at THEN created_at END) AS second_run_at
    FROM cohort_runs
    GROUP BY tenant_id
)
SELECT
    fp.cohort_month,
    COUNT(DISTINCT fp.tenant_id)                                                                  AS new_paying_users,
    COUNT(DISTINCT CASE WHEN tc.total_runs >= 2 THEN fp.tenant_id END)                            AS second_execution_users,
    COUNT(DISTINCT CASE
        WHEN tc.second_run_at IS NOT NULL
         AND tc.second_run_at - fp.first_at <= INTERVAL '7 days'
        THEN fp.tenant_id
    END)                                                                                           AS d7,
    COUNT(DISTINCT CASE
        WHEN tc.second_run_at IS NOT NULL
         AND tc.second_run_at - fp.first_at <= INTERVAL '30 days'
        THEN fp.tenant_id
    END)                                                                                           AS d30,
    COALESCE(AVG(NULLIF(cr.spent_usd, 0)), 0)::float8                                              AS avg_spend,
    COALESCE(SUM(cr.revenue_usd - cr.spent_usd) / NULLIF(SUM(cr.revenue_usd), 0) * 100, 0)::float8 AS gross_margin_pct,
    COALESCE(COUNT(CASE WHEN cr.status = 'succeeded' THEN 1 END)::float8
             / NULLIF(COUNT(cr.id), 0), 0)::float8                                                 AS completion_rate,
    COALESCE(COUNT(CASE WHEN cr.refunded_usd > 0 THEN 1 END)::float8
             / NULLIF(COUNT(cr.id), 0), 0)::float8                                                 AS refund_rate
FROM first_paid fp
LEFT JOIN tenant_counts tc ON tc.tenant_id = fp.tenant_id
LEFT JOIN cohort_runs    cr ON cr.tenant_id = fp.tenant_id
WHERE fp.cohort_month >= DATE_TRUNC('month', $1::timestamptz)
GROUP BY fp.cohort_month
ORDER BY fp.cohort_month`

	rows, err := a.Pool.Query(ctx, cohortSQL, sinceCohortMonth.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]dashboards.Cohort, 0)
	for rows.Next() {
		var c dashboards.Cohort
		if err := rows.Scan(
			&c.Month,
			&c.NewPayingUsers,
			&c.SecondExecutionUsers,
			&c.Day7RepeatUsers,
			&c.Day30RepeatUsers,
			&c.AvgSpendUSD,
			&c.GrossMarginPct,
			&c.CompletionRate,
			&c.RefundRate,
		); err != nil {
			return nil, err
		}
		// TODO(milestone3): join against a support-tickets store and
		// populate SupportTicketsPerExec. Left at 0 until that exists.
		out = append(out, c)
	}
	return out, rows.Err()
}

// Recent returns the empty slice in v1.
func (a ExecutionAdapter) Recent(ctx context.Context, limit int) ([]dashboards.ExecutionSummary, error) {
	return []dashboards.ExecutionSummary{}, nil
}

// ActiveCount delegates to the underlying execution.Service.
func (a ExecutionAdapter) ActiveCount(ctx context.Context) (int, error) {
	if a.Svc == nil {
		return 0, nil
	}
	return a.Svc.ActiveCount(ctx)
}

// QueuedCount delegates to the underlying execution.Service.
func (a ExecutionAdapter) QueuedCount(ctx context.Context) (int, error) {
	if a.Svc == nil {
		return 0, nil
	}
	return a.Svc.QueuedCount(ctx)
}

// AverageQueueWaitSec delegates to the underlying execution.Service.
func (a ExecutionAdapter) AverageQueueWaitSec(ctx context.Context, since time.Time) (float64, error) {
	if a.Svc == nil {
		return 0, nil
	}
	return a.Svc.AverageQueueWaitSec(ctx, since)
}

// BlueprintAdapter implements dashboards.BlueprintSource over the
// blueprints.StatsService.
type BlueprintAdapter struct {
	Svc blueprints.StatsService
}

// AllStats returns every recorded blueprint's rolled-up stats.
func (a BlueprintAdapter) AllStats(ctx context.Context) ([]dashboards.BlueprintStats, error) {
	if a.Svc == nil {
		return []dashboards.BlueprintStats{}, nil
	}
	rows, err := a.Svc.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dashboards.BlueprintStats, 0, len(rows))
	for _, s := range rows {
		out = append(out, dashboards.BlueprintStats{
			BlueprintID:        s.BlueprintID,
			Executions:         int(s.Executions),
			PreviewSuccess:     int(s.PreviewSuccess),
			Refunds:            int(s.Refunds),
			RepairCount:        int(s.RepairCount),
			AvgRevenueUSD:      floatOf(s.AvgRevenueUSD),
			AvgCostUSD:         floatOf(s.AvgCostUSD),
			GrossMarginPct:     floatOf(s.GrossMarginPct),
			AvgCompletionScore: floatOf(s.AvgCompletionScore),
		})
	}
	return out, nil
}

// ScaleAdapter is the static-side ScaleSource. ActiveExecutions /
// QueueDepth / WorkerUtilizationPct return zero — the ExecutionAdapter
// owns those live numbers and feeds them into BuildScale directly.
// SandboxCapacity honours IRONFLYER_MAX_CONCURRENT_RUNS so a single
// env knob drives both the runslots admission cap (finisher engine)
// and the scale dashboard's denominator.
type ScaleAdapter struct{}

func (ScaleAdapter) ActiveExecutions(_ context.Context) (int, error) { return 0, nil }
func (ScaleAdapter) QueueDepth(_ context.Context) (int, error)       { return 0, nil }
func (ScaleAdapter) WorkerUtilizationPct(_ context.Context) (float64, error) {
	return 0.0, nil
}

// SandboxCapacity returns the operator-configured sandbox ceiling.
// Pulls from IRONFLYER_MAX_CONCURRENT_RUNS (the same env that drives
// runslots) so the dashboard denominator matches what the finisher
// admission control will actually let in. Defaults to 8 — matching
// runslots' fallback — when the env is unset or unparseable.
func (ScaleAdapter) SandboxCapacity(_ context.Context) (int, error) {
	raw := os.Getenv("IRONFLYER_MAX_CONCURRENT_RUNS")
	if raw == "" {
		return 8, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 8, nil
	}
	return n, nil
}

// floatOf is the decimal→float seam at the dashboard boundary.
func floatOf(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

// _ keeps the uuid import referenced; future tenant-aware rollup
// signatures will accept a uuid.UUID.
var _ = uuid.Nil
