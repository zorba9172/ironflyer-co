// Package adapters wires the dashboards source interfaces to the
// concrete V22 services. Lives in its own subpackage to keep
// internal/dashboards a pure read layer and to give Agent 8 a single
// file to grep for "what feeds the dashboards".
package adapters

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/blueprints"
	"ironflyer/core/orchestrator/internal/business/dashboards"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/ledger"
)

// LedgerAdapter implements dashboards.LedgerSource over ledger.Service.
//
// The dashboards aggregate across every tenant (operator view) — the
// underlying ledger.Service is per-tenant, so this adapter falls back
// to a direct cross-tenant SQL rollup against the ledger_entries table
// when a pgxpool.Pool is wired. When only the in-memory ledger service
// is wired (no Pool) the methods return empty maps so the in-memory
// backend path keeps rendering "warming up" instead of erroring.
type LedgerAdapter struct {
	Svc  ledger.Service
	Pool *pgxpool.Pool
}

// SumByType returns a per-entry-type sum in [since, until). When
// dashboards.TenantFromContext(ctx) is non-empty the rollup is
// strictly scoped by `tenant_id = $1` so a paying customer never sees
// platform-wide revenue. When empty (operator-only Scale path) the
// roll-up still runs cross-tenant. When a pgxpool is wired this runs
// a single grouped query against ledger_entries; without a pool the
// result is empty.
func (a LedgerAdapter) SumByType(ctx context.Context, since, until time.Time, types []string) (map[string]decimal.Decimal, error) {
	out := map[string]decimal.Decimal{}
	if a.Pool == nil {
		return out, nil
	}
	clauses := []string{"1=1"}
	args := []any{}
	if tenantID := dashboards.TenantFromContext(ctx); tenantID != "" {
		tid, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, fmt.Errorf("ledger sum by type: invalid tenant id %q: %w", tenantID, err)
		}
		args = append(args, tid)
		clauses = append(clauses, "tenant_id = $"+strconv.Itoa(len(args)))
	}
	if !since.IsZero() {
		args = append(args, since.UTC())
		clauses = append(clauses, "created_at >= $"+strconv.Itoa(len(args)))
	}
	if !until.IsZero() {
		args = append(args, until.UTC())
		clauses = append(clauses, "created_at < $"+strconv.Itoa(len(args)))
	}
	if len(types) > 0 {
		args = append(args, types)
		clauses = append(clauses, "entry_type = ANY($"+strconv.Itoa(len(args))+")")
	}
	sql := `SELECT entry_type, COALESCE(SUM(amount_usd), 0) FROM ledger_entries WHERE ` +
		joinAnd(clauses) + ` GROUP BY entry_type`
	rows, err := a.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			t   string
			amt decimal.Decimal
		)
		if err := rows.Scan(&t, &amt); err != nil {
			return nil, err
		}
		out[t] = amt
	}
	return out, rows.Err()
}

// RecentEntries returns the latest N ledger entries across every
// tenant. Empty when no pool is wired.
func (a LedgerAdapter) RecentEntries(ctx context.Context, limit int) ([]dashboards.LedgerSummary, error) {
	if a.Pool == nil || limit <= 0 {
		return []dashboards.LedgerSummary{}, nil
	}
	const sql = `
SELECT id, tenant_id, COALESCE(execution_id::text, ''), entry_type, direction, amount_usd, created_at
FROM ledger_entries
ORDER BY created_at DESC
LIMIT $1`
	rows, err := a.Pool.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]dashboards.LedgerSummary, 0, limit)
	for rows.Next() {
		var s dashboards.LedgerSummary
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ExecutionID, &s.EntryType, &s.Direction, &s.AmountUSD, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// joinAnd joins SQL clauses with AND.
func joinAnd(clauses []string) string {
	out := ""
	for i, c := range clauses {
		if i > 0 {
			out += " AND "
		}
		out += c
	}
	return out
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

// CountsByStatus returns the per-status count in [since, until).
// When dashboards.TenantFromContext(ctx) is non-empty the count is
// scoped by `tenant_id = $1`; otherwise it stays cross-tenant for the
// operator-only Scale path. When a pgxpool is wired this is a single
// grouped query against the executions table; without a pool the
// result is empty.
func (a ExecutionAdapter) CountsByStatus(ctx context.Context, since, until time.Time) (map[string]int, error) {
	out := map[string]int{}
	if a.Pool == nil {
		return out, nil
	}
	clauses := []string{"1=1"}
	args := []any{}
	if tenantID := dashboards.TenantFromContext(ctx); tenantID != "" {
		tid, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, fmt.Errorf("execution counts by status: invalid tenant id %q: %w", tenantID, err)
		}
		args = append(args, tid)
		clauses = append(clauses, "tenant_id = $"+strconv.Itoa(len(args)))
	}
	if !since.IsZero() {
		args = append(args, since.UTC())
		clauses = append(clauses, "created_at >= $"+strconv.Itoa(len(args)))
	}
	if !until.IsZero() {
		args = append(args, until.UTC())
		clauses = append(clauses, "created_at < $"+strconv.Itoa(len(args)))
	}
	sql := `SELECT status::text, COUNT(*) FROM executions WHERE ` +
		joinAnd(clauses) + ` GROUP BY status`
	rows, err := a.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			s string
			n int
		)
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
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
	// Per-tenant scoping: every CTE must filter on tenant_id so the
	// cohort row a customer sees is built only from their own runs.
	// When TenantFromContext is empty (operator path) the dashboard
	// keeps its cross-tenant behaviour for the platform view.
	tenantClause := ""
	args := []any{sinceCohortMonth.UTC()}
	if tenantID := dashboards.TenantFromContext(ctx); tenantID != "" {
		tid, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, fmt.Errorf("execution cohort: invalid tenant id %q: %w", tenantID, err)
		}
		args = append(args, tid)
		tenantClause = " AND tenant_id = $2"
	}
	cohortSQL := `
WITH first_paid AS (
    SELECT tenant_id,
           MIN(created_at) AS first_at,
           DATE_TRUNC('month', MIN(created_at)) AS cohort_month
    FROM executions
    WHERE status NOT IN ('created', 'admitted')` + tenantClause + `
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
    WHERE e.status NOT IN ('created', 'admitted')` + strings.Replace(tenantClause, "tenant_id", "e.tenant_id", 1) + `
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

	rows, err := a.Pool.Query(ctx, cohortSQL, args...)
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
// blueprints.StatsService AND, when a per-tenant scope is in flight,
// over the raw blueprint_runs table via the optional pgxpool.
//
// The blueprints.StatsService.All() roll-up is platform-wide (one row
// per blueprint id, summed across every tenant) — surfacing it to a
// signed-in customer would replay bug #16. When TenantFromContext is
// non-empty and Pool is wired, AllStats re-derives per-blueprint
// counts/sums from blueprint_runs WHERE tenant_id = $1; otherwise it
// falls back to the platform-wide rollup (operator path) or to the
// in-memory Svc (when no Pool is wired).
type BlueprintAdapter struct {
	Svc  blueprints.StatsService
	Pool *pgxpool.Pool
}

// AllStats returns the recorded blueprint stats, scoped to the
// caller's tenant when dashboards.TenantFromContext(ctx) is set.
func (a BlueprintAdapter) AllStats(ctx context.Context) ([]dashboards.BlueprintStats, error) {
	if tenantID := dashboards.TenantFromContext(ctx); tenantID != "" {
		if a.Pool == nil {
			// Per-tenant view requires the SQL-backed runs table. With
			// only the in-memory Svc (no Pool) the StatsService rollup
			// would be cross-tenant, which we MUST NOT surface. Return
			// empty so a tenant on the memory backend sees a clean
			// dashboard rather than another tenant's numbers.
			return []dashboards.BlueprintStats{}, nil
		}
		return a.allStatsForTenant(ctx, tenantID)
	}
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

// allStatsForTenant rolls up blueprint_runs for one tenant — the
// per-tenant equivalent of blueprints.StatsService.All(). Repair
// count is approximated as `repaired = true` count; preview success
// is the count of rows with preview_success = true; refunds counts
// rows with refunded = true. Averages divide by the per-tenant
// execution count for that blueprint.
func (a BlueprintAdapter) allStatsForTenant(ctx context.Context, tenantID string) ([]dashboards.BlueprintStats, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("blueprint all stats: invalid tenant id %q: %w", tenantID, err)
	}
	const q = `
SELECT blueprint_id,
       COUNT(*)                                                                                    AS executions,
       COUNT(*) FILTER (WHERE preview_success)                                                     AS preview_success,
       COUNT(*) FILTER (WHERE refunded)                                                            AS refunds,
       COUNT(*) FILTER (WHERE repaired)                                                            AS repair_count,
       COALESCE(SUM(revenue_usd) / NULLIF(COUNT(*), 0), 0)                                         AS avg_revenue,
       COALESCE(SUM(cost_usd)    / NULLIF(COUNT(*), 0), 0)                                         AS avg_cost,
       COALESCE((SUM(revenue_usd) - SUM(cost_usd)) / NULLIF(SUM(revenue_usd), 0) * 100, 0)::float8 AS gross_margin_pct,
       COALESCE(SUM(completion_score) / NULLIF(COUNT(*), 0), 0)::float8                            AS avg_completion
FROM blueprint_runs
WHERE tenant_id = $1
GROUP BY blueprint_id
ORDER BY executions DESC`
	rows, err := a.Pool.Query(ctx, q, tid)
	if err != nil {
		return nil, fmt.Errorf("blueprint all stats (tenant): %w", err)
	}
	defer rows.Close()
	out := make([]dashboards.BlueprintStats, 0)
	for rows.Next() {
		var (
			s              dashboards.BlueprintStats
			execs          int64
			previewSuccess int64
			refunds        int64
			repairs        int64
			avgRev         decimal.Decimal
			avgCost        decimal.Decimal
			marginPct      float64
			avgCompletion  float64
		)
		if err := rows.Scan(&s.BlueprintID, &execs, &previewSuccess, &refunds, &repairs,
			&avgRev, &avgCost, &marginPct, &avgCompletion); err != nil {
			return nil, err
		}
		s.Executions = int(execs)
		s.PreviewSuccess = int(previewSuccess)
		s.Refunds = int(refunds)
		s.RepairCount = int(repairs)
		s.AvgRevenueUSD = floatOf(avgRev)
		s.AvgCostUSD = floatOf(avgCost)
		s.GrossMarginPct = marginPct
		s.AvgCompletionScore = avgCompletion
		out = append(out, s)
	}
	return out, rows.Err()
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

