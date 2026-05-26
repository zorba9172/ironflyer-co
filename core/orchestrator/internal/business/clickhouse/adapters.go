package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/dashboards"
)

// LedgerSource implements dashboards.LedgerSource against
// fact_execution_costs (cost rows) and fact_refunds / fact_wallet_topups
// (revenue and refund rows). The dashboard treats SumByType keys as
// arbitrary strings — we map ClickHouse cost_type values directly so
// the Profit dashboard sees the same vocabulary as before.
type LedgerSource struct {
	ch *Client
}

// NewLedgerSource wires the LedgerSource. Safe to pass a nil client —
// every method short-circuits to the zero value so the dashboards
// still render an empty state when ClickHouse is offline.
func NewLedgerSource(ch *Client) *LedgerSource { return &LedgerSource{ch: ch} }

// SumByType totals fact_execution_costs by cost_type within
// [since, until). When types is nil all known cost types are summed.
// Revenue (wallet topups) and refunds are folded in under the synthetic
// keys "revenue" and "refund" so the profit dashboard can compute
// gross margin in a single source call.
func (s *LedgerSource) SumByType(ctx context.Context, since, until time.Time, types []string) (map[string]decimal.Decimal, error) {
	out := map[string]decimal.Decimal{}
	if s == nil || s.ch == nil {
		return out, nil
	}

	// Cost types live in fact_execution_costs.
	costQuery := `SELECT cost_type, sum(amount_usd) AS amt
	                FROM fact_execution_costs
	               WHERE occurred_at >= ? AND occurred_at < ?
	               GROUP BY cost_type`
	rows, err := s.ch.QueryRows(ctx, costQuery, since.UTC(), until.UTC())
	if err != nil {
		return nil, fmt.Errorf("ledger sum by type: %w", err)
	}
	for rows.Next() {
		var costType string
		var amt decimal.Decimal
		if err := rows.Scan(&costType, &amt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[costType] = amt
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	// Revenue (wallet top-ups).
	rev, err := s.sumOne(ctx, "fact_wallet_topups", since, until)
	if err == nil && !rev.IsZero() {
		out["revenue"] = rev
	}
	// Refunds.
	ref, err := s.sumOne(ctx, "fact_refunds", since, until)
	if err == nil && !ref.IsZero() {
		out["refund"] = ref
	}

	if len(types) > 0 {
		filtered := make(map[string]decimal.Decimal, len(types))
		for _, t := range types {
			if v, ok := out[t]; ok {
				filtered[t] = v
			}
		}
		return filtered, nil
	}
	return out, nil
}

func (s *LedgerSource) sumOne(ctx context.Context, table string, since, until time.Time) (decimal.Decimal, error) {
	q := fmt.Sprintf(`SELECT sum(amount_usd) FROM %s WHERE occurred_at >= ? AND occurred_at < ?`, table)
	rows, err := s.ch.QueryRows(ctx, q, since.UTC(), until.UTC())
	if err != nil {
		return decimal.Zero, err
	}
	defer rows.Close()
	var v decimal.Decimal
	if rows.Next() {
		if err := rows.Scan(&v); err != nil {
			return decimal.Zero, err
		}
	}
	return v, rows.Err()
}

// RecentEntries returns the most recent N rows across the cost and
// refund fact tables. Direction is "out" for cost and refund (money
// leaves the platform) and "in" for top-ups.
func (s *LedgerSource) RecentEntries(ctx context.Context, limit int) ([]dashboards.LedgerSummary, error) {
	if s == nil || s.ch == nil {
		return []dashboards.LedgerSummary{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	// UNION ALL keeps each domain's rows distinct; we LIMIT each leg
	// to `limit` then take the top `limit` of the merged result.
	const q = `
		SELECT id, tenant_id, execution_id, entry_type, direction, amount_usd, occurred_at FROM (
		    SELECT toString(event_id) AS id,
		           tenant_id,
		           execution_id,
		           cost_type AS entry_type,
		           'out' AS direction,
		           amount_usd,
		           occurred_at
		      FROM fact_execution_costs
		     ORDER BY occurred_at DESC
		     LIMIT ?
		    UNION ALL
		    SELECT toString(event_id) AS id,
		           tenant_id,
		           '' AS execution_id,
		           'topup' AS entry_type,
		           'in' AS direction,
		           amount_usd,
		           occurred_at
		      FROM fact_wallet_topups
		     ORDER BY occurred_at DESC
		     LIMIT ?
		    UNION ALL
		    SELECT toString(event_id) AS id,
		           tenant_id,
		           execution_id,
		           'refund' AS entry_type,
		           'out' AS direction,
		           amount_usd,
		           occurred_at
		      FROM fact_refunds
		     ORDER BY occurred_at DESC
		     LIMIT ?
		)
		ORDER BY occurred_at DESC
		LIMIT ?`
	rows, err := s.ch.QueryRows(ctx, q, limit, limit, limit, limit)
	if err != nil {
		return nil, fmt.Errorf("ledger recent: %w", err)
	}
	defer rows.Close()
	out := make([]dashboards.LedgerSummary, 0, limit)
	for rows.Next() {
		var r dashboards.LedgerSummary
		if err := rows.Scan(&r.ID, &r.TenantID, &r.ExecutionID, &r.EntryType, &r.Direction, &r.AmountUSD, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ExecutionSource implements dashboards.ExecutionSource against
// fact_execution_completion and rollup_cohort_monthly.
type ExecutionSource struct {
	ch *Client
}

// NewExecutionSource wires the ExecutionSource. Active/queued counts
// remain Postgres-truth — those numbers must not lag a real-time
// dashboard by a Redpanda hop. The integration agent should hand the
// live execution.Service in via the constructor when wiring; the v1
// adapter returns zero for those calls so a CH-only deploy stays
// compilable.
func NewExecutionSource(ch *Client) *ExecutionSource { return &ExecutionSource{ch: ch} }

func (s *ExecutionSource) CountsByStatus(ctx context.Context, since, until time.Time) (map[string]int, error) {
	out := map[string]int{}
	if s == nil || s.ch == nil {
		return out, nil
	}
	const q = `SELECT status, count() FROM fact_execution_completion
	            WHERE occurred_at >= ? AND occurred_at < ?
	            GROUP BY status`
	rows, err := s.ch.QueryRows(ctx, q, since.UTC(), until.UTC())
	if err != nil {
		return nil, fmt.Errorf("execution counts by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n uint64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = int(n)
	}
	return out, rows.Err()
}

// CountsByCohort aggregates rollup_cohort_monthly into one Cohort row
// per calendar month. Per-tenant fields stored at the (cohort_month,
// tenant) grain — the SELECT collapses them with DISTINCT counts so
// the funnel metrics stay tenant-cardinality based.
func (s *ExecutionSource) CountsByCohort(ctx context.Context, sinceCohortMonth time.Time) ([]dashboards.Cohort, error) {
	out := make([]dashboards.Cohort, 0)
	if s == nil || s.ch == nil {
		return out, nil
	}
	const q = `
		SELECT cohort_month                                                              AS month,
		       countDistinct(tenant_id)                                                  AS new_paying,
		       countDistinctIf(tenant_id, total_runs >= 2)                               AS second_exec,
		       countDistinctIf(tenant_id,
		           toDateTime(second_paid_at) > toDateTime(first_paid_at)
		           AND (toDateTime(second_paid_at) - toDateTime(first_paid_at)) <= 7*86400) AS d7,
		       countDistinctIf(tenant_id,
		           toDateTime(second_paid_at) > toDateTime(first_paid_at)
		           AND (toDateTime(second_paid_at) - toDateTime(first_paid_at)) <= 30*86400) AS d30,
		       toFloat64(sum(spend_usd_sum) / nullIf(sum(total_runs), 0))                AS avg_spend,
		       toFloat64((sum(revenue_usd_sum) - sum(spend_usd_sum)) / nullIf(sum(revenue_usd_sum), 0) * 100) AS gross_margin_pct,
		       toFloat64(sum(completed_count) / nullIf(sum(total_runs), 0))             AS completion_rate,
		       toFloat64(sum(refund_count) / nullIf(sum(total_runs), 0))                AS refund_rate
		  FROM rollup_cohort_monthly
		 WHERE cohort_month >= toDate(?)
		 GROUP BY cohort_month
		 ORDER BY cohort_month`
	rows, err := s.ch.QueryRows(ctx, q, sinceCohortMonth.UTC())
	if err != nil {
		return nil, fmt.Errorf("execution cohort: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c dashboards.Cohort
		var avgSpend, margin, completion, refund *float64
		if err := rows.Scan(
			&c.Month,
			&c.NewPayingUsers,
			&c.SecondExecutionUsers,
			&c.Day7RepeatUsers,
			&c.Day30RepeatUsers,
			&avgSpend,
			&margin,
			&completion,
			&refund,
		); err != nil {
			return nil, err
		}
		if avgSpend != nil {
			c.AvgSpendUSD = *avgSpend
		}
		if margin != nil {
			c.GrossMarginPct = *margin
		}
		if completion != nil {
			c.CompletionRate = *completion
		}
		if refund != nil {
			c.RefundRate = *refund
		}
		// TODO(milestone3): SupportTicketsPerExec stays 0 until the
		// support tickets fact lands in ClickHouse.
		out = append(out, c)
	}
	return out, rows.Err()
}

// Recent returns the latest N completed executions.
func (s *ExecutionSource) Recent(ctx context.Context, limit int) ([]dashboards.ExecutionSummary, error) {
	if s == nil || s.ch == nil {
		return []dashboards.ExecutionSummary{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	const q = `
		SELECT execution_id,
		       tenant_id,
		       blueprint_id,
		       status,
		       toDecimal64(0, 6) AS budget_usd,
		       spent_usd,
		       revenue_usd,
		       completion_score,
		       toFloat64((revenue_usd - spent_usd) / nullIf(revenue_usd, 0) * 100) AS gross_margin_pct,
		       occurred_at AS created_at,
		       occurred_at AS ended_at
		  FROM fact_execution_completion
		 ORDER BY occurred_at DESC
		 LIMIT ?`
	rows, err := s.ch.QueryRows(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("execution recent: %w", err)
	}
	defer rows.Close()
	out := make([]dashboards.ExecutionSummary, 0, limit)
	for rows.Next() {
		var e dashboards.ExecutionSummary
		var margin *float64
		if err := rows.Scan(
			&e.ID,
			&e.TenantID,
			&e.BlueprintID,
			&e.Status,
			&e.BudgetUSD,
			&e.SpentUSD,
			&e.RevenueUSD,
			&e.CompletionScore,
			&margin,
			&e.CreatedAt,
			&e.EndedAt,
		); err != nil {
			return nil, err
		}
		if margin != nil {
			e.GrossMarginPct = *margin
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ActiveCount/QueuedCount/AverageQueueWaitSec — ClickHouse is the
// wrong source for live "executions running right now" numbers
// because it lags behind Postgres by a Redpanda hop. The orchestrator
// must keep using the in-process execution.Service for those reads;
// this adapter returns zero so a CH-only wiring still satisfies the
// interface but does not invent fake live numbers.
func (s *ExecutionSource) ActiveCount(ctx context.Context) (int, error)   { return 0, nil }
func (s *ExecutionSource) QueuedCount(ctx context.Context) (int, error)   { return 0, nil }
func (s *ExecutionSource) AverageQueueWaitSec(ctx context.Context, since time.Time) (float64, error) {
	return 0, nil
}

// BlueprintSource implements dashboards.BlueprintSource against
// rollup_blueprint_daily.
type BlueprintSource struct {
	ch *Client
}

// NewBlueprintSource wires the BlueprintSource.
func NewBlueprintSource(ch *Client) *BlueprintSource { return &BlueprintSource{ch: ch} }

// AllStats rolls rollup_blueprint_daily into one row per blueprint
// (lifetime aggregate). Averages are computed on the fly so a
// schema change in one column doesn't require backfilling the
// rollup.
func (s *BlueprintSource) AllStats(ctx context.Context) ([]dashboards.BlueprintStats, error) {
	if s == nil || s.ch == nil {
		return []dashboards.BlueprintStats{}, nil
	}
	const q = `
		SELECT blueprint_id,
		       sum(executions)                                                       AS executions,
		       toFloat64(sum(revenue_usd_sum) / nullIf(sum(executions), 0))          AS avg_revenue,
		       toFloat64(sum(cost_usd_sum) / nullIf(sum(executions), 0))             AS avg_cost,
		       toFloat64((sum(revenue_usd_sum) - sum(cost_usd_sum))
		                 / nullIf(sum(revenue_usd_sum), 0) * 100)                   AS gross_margin_pct,
		       sum(preview_success_count)                                            AS preview_success,
		       sum(refund_count)                                                     AS refund_count,
		       sum(repair_count_sum)                                                 AS repair_count_sum,
		       toFloat64(sum(completion_score_sum) / nullIf(sum(executions), 0))    AS avg_completion
		  FROM rollup_blueprint_daily
		 GROUP BY blueprint_id
		 ORDER BY executions DESC`
	rows, err := s.ch.QueryRows(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("blueprint all stats: %w", err)
	}
	defer rows.Close()
	out := make([]dashboards.BlueprintStats, 0, 16)
	for rows.Next() {
		var b dashboards.BlueprintStats
		var execs, previewSuccess, refunds, repairs uint64
		var avgRev, avgCost, marginPct, avgCompletion *float64
		if err := rows.Scan(
			&b.BlueprintID,
			&execs,
			&avgRev,
			&avgCost,
			&marginPct,
			&previewSuccess,
			&refunds,
			&repairs,
			&avgCompletion,
		); err != nil {
			return nil, err
		}
		b.Executions = int(execs)
		b.PreviewSuccess = int(previewSuccess)
		b.Refunds = int(refunds)
		b.RepairCount = int(repairs)
		if avgRev != nil {
			b.AvgRevenueUSD = *avgRev
		}
		if avgCost != nil {
			b.AvgCostUSD = *avgCost
		}
		if marginPct != nil {
			b.GrossMarginPct = *marginPct
		}
		if avgCompletion != nil {
			b.AvgCompletionScore = *avgCompletion
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ScaleSource implements dashboards.ScaleSource against
// rollup_runtime_capacity_5m. The dashboard's "right now" numbers
// come from the latest 5-minute bucket — close enough for an
// operational pane while keeping Postgres untouched.
type ScaleSource struct {
	ch *Client
}

// NewScaleSource wires the ScaleSource.
func NewScaleSource(ch *Client) *ScaleSource { return &ScaleSource{ch: ch} }

// ActiveExecutions returns the active_runs from the latest 5-minute
// bucket. When the bucket is missing returns 0.
func (s *ScaleSource) ActiveExecutions(ctx context.Context) (int, error) {
	return s.lastUint(ctx, "active_runs")
}

// QueueDepth returns the queued_runs from the latest 5-minute bucket.
func (s *ScaleSource) QueueDepth(ctx context.Context) (int, error) {
	return s.lastUint(ctx, "queued_runs")
}

// WorkerUtilizationPct returns the utilization_pct of the latest
// 5-minute bucket averaged across pools.
func (s *ScaleSource) WorkerUtilizationPct(ctx context.Context) (float64, error) {
	if s == nil || s.ch == nil {
		return 0, nil
	}
	const q = `SELECT avg(utilization_pct) FROM rollup_runtime_capacity_5m
	            WHERE bucket = (SELECT max(bucket) FROM rollup_runtime_capacity_5m)`
	rows, err := s.ch.QueryRows(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("scale utilization: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var v *float64
		if err := rows.Scan(&v); err != nil {
			return 0, err
		}
		if v != nil {
			return *v, nil
		}
	}
	return 0, rows.Err()
}

// SandboxCapacity returns the latest bucket's summed capacity across
// pools. Falls back to 0 when no bucket exists — the integration
// agent can swap in the env-driven Postgres adapter for the empty case.
func (s *ScaleSource) SandboxCapacity(ctx context.Context) (int, error) {
	return s.lastSum(ctx, "capacity")
}

func (s *ScaleSource) lastUint(ctx context.Context, col string) (int, error) {
	if s == nil || s.ch == nil {
		return 0, nil
	}
	q := fmt.Sprintf(`SELECT sum(%s) FROM rollup_runtime_capacity_5m
	                   WHERE bucket = (SELECT max(bucket) FROM rollup_runtime_capacity_5m)`, col)
	return s.scalarInt(ctx, q)
}

func (s *ScaleSource) lastSum(ctx context.Context, col string) (int, error) {
	return s.lastUint(ctx, col)
}

func (s *ScaleSource) scalarInt(ctx context.Context, query string) (int, error) {
	rows, err := s.ch.QueryRows(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if rows.Next() {
		var v *uint64
		if err := rows.Scan(&v); err != nil {
			return 0, err
		}
		if v != nil {
			return int(*v), nil
		}
	}
	return 0, rows.Err()
}

// Compile-time guards — these will fail to build if the dashboards
// interfaces drift away from the adapters.
var (
	_ dashboards.LedgerSource    = (*LedgerSource)(nil)
	_ dashboards.ExecutionSource = (*ExecutionSource)(nil)
	_ dashboards.BlueprintSource = (*BlueprintSource)(nil)
	_ dashboards.ScaleSource     = (*ScaleSource)(nil)
)

// Suppress unused-import lint when strings ends up only referenced
// inside SQL — kept for future identifier handling.
var _ = strings.TrimSpace
