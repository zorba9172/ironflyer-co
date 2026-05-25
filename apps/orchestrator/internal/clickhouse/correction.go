package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// CorrectionJob recomputes the trailing window of every daily rollup
// table so late-arriving raw events still influence the dashboards.
//
// The job is best-effort by design: ClickHouse downtime, a transient
// network blip, or a single bad window do not stop the loop. Every
// outcome — success, failure, skip — is mirrored into
// rollup_correction_state so operators have a single table to inspect.
//
// Design choice — INSERT, never DELETE:
//
// ClickHouse mutations (ALTER TABLE … DELETE) are heavy, async, and
// hostile to MergeTree partitions. SummingMergeTree and
// ReplacingMergeTree both collapse rows at merge time when the
// ORDER BY tuple matches. The recompute therefore writes a fresh row
// per window (same ORDER BY key as the previous one) and trusts the
// engine to collapse on merge. SELECTs that need an authoritative
// answer for an in-flight window can use FINAL or sumIf(…) — see
// adapters.go for the read path.
//
// Daily rollups: 14-day correction window, hourly.
// Cohort rollup: current-month recompute, hourly (separate entrypoint).
type CorrectionJob struct {
	client     *Client
	log        zerolog.Logger
	windowDays int
	interval   time.Duration
}

// dailyRollups is the closed list of daily rollups the correction job
// recomputes. Each entry pairs the destination rollup with the SELECT
// shape that re-projects from the underlying fact_* table. The SELECT
// columns MUST match the rollup ORDER BY tuple plus the SummingMergeTree
// numeric columns; otherwise the engine cannot collapse duplicates.
var dailyRollups = []dailyRollup{
	{
		name:        "rollup_profit_daily",
		insertSQL:   profitDailySQL,
		hasTenantID: true,
	},
	{
		name:        "rollup_provider_daily",
		insertSQL:   providerDailySQL,
		hasTenantID: true,
	},
	{
		name:        "rollup_blueprint_daily",
		insertSQL:   blueprintDailySQL,
		hasTenantID: true,
	},
	{
		name:        "rollup_gate_daily",
		insertSQL:   gateDailySQL,
		hasTenantID: true,
	},
	{
		name:        "rollup_abuse_tenant_daily",
		insertSQL:   abuseTenantDailySQL,
		hasTenantID: true,
	},
}

type dailyRollup struct {
	name string
	// insertSQL is a printf template with two `?` parameters:
	// (1) day_start DateTime, (2) day_end DateTime. The job binds them
	// per window so the SELECT scope is exactly one calendar day.
	insertSQL   string
	hasTenantID bool
}

// NewCorrectionJob constructs the recompute scheduler. windowDays<=0
// defaults to 14; interval<=0 defaults to 1 hour. A nil client is
// tolerated — Run/RecomputeCohort then become no-ops so wireup code
// can stay branch-light.
func NewCorrectionJob(client *Client, log zerolog.Logger, windowDays int, interval time.Duration) *CorrectionJob {
	if windowDays <= 0 {
		windowDays = 14
	}
	if interval <= 0 {
		interval = time.Hour
	}
	return &CorrectionJob{
		client:     client,
		log:        log.With().Str("subsystem", "clickhouse.correction").Logger(),
		windowDays: windowDays,
		interval:   interval,
	}
}

// Run loops until ctx is cancelled. On every tick it walks the daily
// rollups and re-projects the trailing windowDays of fact data, then
// recomputes the current-month cohort.
//
// Errors are logged and the loop continues — the goroutine never
// returns until ctx is done, so callers can wire it with a fire-and-
// forget `go correction.Run(ctx)`.
func (j *CorrectionJob) Run(ctx context.Context) error {
	if j == nil || j.client == nil {
		// Analytics plane disabled — quietly do nothing.
		return nil
	}
	j.log.Info().
		Int("window_days", j.windowDays).
		Dur("interval", j.interval).
		Msg("clickhouse correction job started")

	// Run once on startup so the first dashboard render after a
	// deploy reflects the trailing window without waiting an interval.
	j.runOnce(ctx)

	t := time.NewTicker(j.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			j.log.Info().Msg("clickhouse correction job stopping")
			return ctx.Err()
		case <-t.C:
			j.runOnce(ctx)
		}
	}
}

// runOnce executes one full pass: every daily rollup over the
// trailing window, then the current-month cohort. Per-window errors
// are logged and recorded in rollup_correction_state with status
// 'failed' — they never propagate up so a single bad ClickHouse
// shard does not stall the whole pass.
func (j *CorrectionJob) runOnce(ctx context.Context) {
	now := time.Now().UTC()
	for _, r := range dailyRollups {
		j.recomputeRollup(ctx, r, now)
	}
	if err := j.RecomputeCohort(ctx); err != nil {
		j.log.Warn().Err(err).Msg("cohort recompute failed")
	}
}

// recomputeRollup walks the trailing windowDays for one rollup. For
// each day it stamps a 'running' marker, runs the INSERT, then
// finalises the marker with 'idle' (success) or 'failed' (error).
func (j *CorrectionJob) recomputeRollup(ctx context.Context, r dailyRollup, now time.Time) {
	startDay := now.AddDate(0, 0, -j.windowDays).UTC().Truncate(24 * time.Hour)
	for day := startDay; !day.After(now); day = day.AddDate(0, 0, 1) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		j.recomputeDay(ctx, r, day)
	}
}

// recomputeDay runs the INSERT for one (rollup, day) pair and
// records the outcome.
func (j *CorrectionJob) recomputeDay(ctx context.Context, r dailyRollup, day time.Time) {
	dayStart := day
	dayEnd := day.Add(24 * time.Hour)

	// 1) Mark this window as running. ReplacingMergeTree collapses on
	// last_recomputed_at so the previous row for the same window is
	// shadowed at the next merge.
	if err := j.markState(ctx, r.name, dayStart, "running", 0, ""); err != nil {
		j.log.Warn().Err(err).Str("rollup", r.name).Time("day", dayStart).
			Msg("mark running")
		// Continue — the INSERT still has independent value.
	}

	// 2) Run the recompute INSERT. The destination is a
	// SummingMergeTree (or ReplacingMergeTree for cohort); writing
	// the same logical day twice produces the same totals after merge
	// because the keys collide and the engine collapses.
	if err := j.client.Exec(ctx, r.insertSQL, dayStart, dayEnd); err != nil {
		summary := truncErr(err)
		j.log.Warn().Err(err).Str("rollup", r.name).Time("day", dayStart).
			Msg("recompute insert failed")
		if mErr := j.markState(ctx, r.name, dayStart, "failed", 0, summary); mErr != nil {
			j.log.Warn().Err(mErr).Str("rollup", r.name).Msg("mark failed")
		}
		return
	}

	// 3) Count entries we just (re-)wrote. The number is best-effort
	// observability — a count failure does not fail the window.
	var entries uint64
	countQ := fmt.Sprintf(
		"SELECT count() FROM %s WHERE toDate(%s) = toDate(?)",
		r.name, dayColumnFor(r.name),
	)
	if rows, err := j.client.QueryRows(ctx, countQ, dayStart); err == nil {
		if rows.Next() {
			_ = rows.Scan(&entries)
		}
		_ = rows.Close()
	}

	if err := j.markState(ctx, r.name, dayStart, "idle", entries, ""); err != nil {
		j.log.Warn().Err(err).Str("rollup", r.name).Msg("mark idle")
	}
	j.log.Debug().Str("rollup", r.name).Time("day", dayStart).
		Uint64("entries", entries).Msg("correction window recomputed")
}

// RecomputeCohort re-projects rollup_cohort_monthly for the current
// calendar month. Cohort rollups care about month-grain entrants;
// older months are immutable and never need re-projection.
func (j *CorrectionJob) RecomputeCohort(ctx context.Context) error {
	if j == nil || j.client == nil {
		return nil
	}
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	if err := j.markState(ctx, "rollup_cohort_monthly", monthStart, "running", 0, ""); err != nil {
		j.log.Warn().Err(err).Msg("cohort: mark running")
	}
	if err := j.client.Exec(ctx, cohortMonthlySQL, monthStart, monthEnd); err != nil {
		summary := truncErr(err)
		if mErr := j.markState(ctx, "rollup_cohort_monthly", monthStart, "failed", 0, summary); mErr != nil {
			j.log.Warn().Err(mErr).Msg("cohort: mark failed")
		}
		return fmt.Errorf("cohort recompute: %w", err)
	}
	if err := j.markState(ctx, "rollup_cohort_monthly", monthStart, "idle", 0, ""); err != nil {
		j.log.Warn().Err(err).Msg("cohort: mark idle")
	}
	j.log.Debug().Time("month", monthStart).Msg("cohort recompute complete")
	return nil
}

// markState upserts one row into rollup_correction_state. Because the
// table is ReplacingMergeTree(last_recomputed_at) the most recent
// write wins per (rollup_name, window_start).
func (j *CorrectionJob) markState(ctx context.Context, name string, windowStart time.Time, status string, entries uint64, errSummary string) error {
	if j == nil || j.client == nil {
		return nil
	}
	const q = `INSERT INTO rollup_correction_state
	    (rollup_name, window_start, last_recomputed_at, entries_recomputed, status, error_summary)
	    VALUES (?, ?, ?, ?, ?, ?)`
	return j.client.Exec(ctx, q, name, windowStart, time.Now().UTC(), entries, status, errSummary)
}

// dayColumnFor returns the rollup's day-grain column name so the
// post-insert count query targets the right field. Every daily rollup
// in 03_rollups.sql uses `day` except cohort, which is monthly.
func dayColumnFor(rollup string) string {
	switch rollup {
	case "rollup_cohort_monthly":
		return "cohort_month"
	default:
		return "day"
	}
}

// truncErr returns a short error summary safe to store in a
// LowCardinality-adjacent column. ClickHouse error strings can be
// multi-kilobyte; the dashboard only needs the head.
func truncErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 240 {
		s = s[:240] + "..."
	}
	return s
}

// errCorrectionDisabled is returned when callers ask the job to do
// work but the analytics plane is off. Currently used only by tests
// the policy of this repo forbids — kept here so a future check is
// the one-line `if errors.Is(err, errCorrectionDisabled)`.
var errCorrectionDisabled = errors.New("clickhouse correction: client disabled")

// --- INSERT templates ----------------------------------------------------
//
// Each template re-projects one day from the fact_* tables into the
// matching rollup. The bind parameters are always (dayStart, dayEnd)
// — half-open [dayStart, dayEnd). The SELECT must produce every
// column of the rollup in ORDER BY order so SummingMergeTree
// collapses correctly at merge time.

// rollup_profit_daily — revenue + costs + completion counts per day/tenant.
// Source: fact_execution_costs (cost split by cost_type),
//         fact_wallet_topups (revenue),
//         fact_refunds (refunds),
//         fact_execution_completion (completion + profitable counts).
const profitDailySQL = `
INSERT INTO rollup_profit_daily
SELECT
    toDate(occurred_at)                                  AS day,
    tenant_id,
    sumIf(amount_usd, source = 'revenue')                AS revenue_usd,
    sumIf(amount_usd, source = 'provider')               AS provider_cost_usd,
    sumIf(amount_usd, source = 'sandbox')                AS sandbox_cost_usd,
    sumIf(amount_usd, source = 'storage')                AS storage_cost_usd,
    sumIf(amount_usd, source = 'deployment')             AS deployment_cost_usd,
    sumIf(amount_usd, source = 'refund')                 AS refunds_usd,
    sumIf(toUInt64(1), source = 'completion')            AS completed_executions,
    sumIf(toUInt64(1), source = 'profitable_completion') AS profitable_completed_executions
FROM (
    SELECT occurred_at, tenant_id, amount_usd,
           multiIf(cost_type = 'provider',   'provider',
                   cost_type = 'sandbox',    'sandbox',
                   cost_type = 'storage',    'storage',
                   cost_type = 'deployment', 'deployment',
                   'other') AS source
    FROM fact_execution_costs FINAL
    WHERE occurred_at >= ? AND occurred_at < ?
    UNION ALL
    SELECT occurred_at, tenant_id, amount_usd, 'revenue' AS source
    FROM fact_wallet_topups FINAL
    WHERE occurred_at >= ? AND occurred_at < ?
    UNION ALL
    SELECT occurred_at, tenant_id, amount_usd, 'refund' AS source
    FROM fact_refunds FINAL
    WHERE occurred_at >= ? AND occurred_at < ?
    UNION ALL
    SELECT occurred_at, tenant_id, toDecimal64(0, 6) AS amount_usd, 'completion' AS source
    FROM fact_execution_completion FINAL
    WHERE occurred_at >= ? AND occurred_at < ?
      AND status IN ('settled','completed')
    UNION ALL
    SELECT occurred_at, tenant_id, toDecimal64(0, 6) AS amount_usd, 'profitable_completion' AS source
    FROM fact_execution_completion FINAL
    WHERE occurred_at >= ? AND occurred_at < ?
      AND status IN ('settled','completed')
      AND (revenue_usd - spent_usd) > 0
) src
GROUP BY day, tenant_id
`

// rollup_provider_daily — per (provider, model) cost+latency+token totals.
// Source: fact_provider_usage.
const providerDailySQL = `
INSERT INTO rollup_provider_daily
SELECT
    toDate(occurred_at)     AS day,
    tenant_id,
    provider,
    model,
    count()                 AS calls,
    sum(cost_usd)           AS cost_usd,
    sum(input_tokens)       AS input_tokens,
    sum(output_tokens)      AS output_tokens,
    sum(latency_ms)         AS latency_ms_sum
FROM fact_provider_usage FINAL
WHERE occurred_at >= ? AND occurred_at < ?
GROUP BY day, tenant_id, provider, model
`

// rollup_blueprint_daily — per-blueprint daily aggregates.
// Source: fact_blueprint_runs.
const blueprintDailySQL = `
INSERT INTO rollup_blueprint_daily
SELECT
    toDate(occurred_at)                             AS day,
    tenant_id,
    blueprint_id,
    count()                                         AS executions,
    sumIf(toUInt64(1), preview_success = 1)         AS preview_success_count,
    sumIf(toUInt64(1), refunded = 1)                AS refund_count,
    sum(toUInt64(repair_count))                     AS repair_count_sum,
    sum(revenue_usd)                                AS revenue_usd_sum,
    sum(cost_usd)                                   AS cost_usd_sum,
    sum(completion_score)                           AS completion_score_sum
FROM fact_blueprint_runs FINAL
WHERE occurred_at >= ? AND occurred_at < ?
GROUP BY day, tenant_id, blueprint_id
`

// rollup_gate_daily — gate verdict counts + duration sums per day.
// Source: fact_gate_outcomes.
const gateDailySQL = `
INSERT INTO rollup_gate_daily
SELECT
    toDate(occurred_at)     AS day,
    tenant_id,
    gate_name,
    verdict,
    count()                 AS count,
    sum(duration_ms)        AS duration_ms
FROM fact_gate_outcomes FINAL
WHERE occurred_at >= ? AND occurred_at < ?
GROUP BY day, tenant_id, gate_name, verdict
`

// rollup_abuse_tenant_daily — per-tenant abuse scoring inputs.
// Source: fact_execution_completion (failed counts) + fact_refunds
// (refund $) — rate_limit_hits is sourced from the audit topic
// downstream and zeroed here so the rollup remains additive.
const abuseTenantDailySQL = `
INSERT INTO rollup_abuse_tenant_daily
SELECT
    day,
    tenant_id,
    score,
    failed_executions,
    refunds_usd,
    rate_limit_hits
FROM (
    SELECT
        toDate(occurred_at) AS day,
        tenant_id,
        0.0                 AS score,
        sumIf(toUInt64(1), status IN ('failed','cancelled')) AS failed_executions,
        toDecimal64(0, 6)   AS refunds_usd,
        toUInt64(0)         AS rate_limit_hits
    FROM fact_execution_completion FINAL
    WHERE occurred_at >= ? AND occurred_at < ?
    GROUP BY day, tenant_id
    UNION ALL
    SELECT
        toDate(occurred_at) AS day,
        tenant_id,
        0.0                 AS score,
        toUInt64(0)         AS failed_executions,
        sum(amount_usd)     AS refunds_usd,
        toUInt64(0)         AS rate_limit_hits
    FROM fact_refunds FINAL
    WHERE occurred_at >= ? AND occurred_at < ?
    GROUP BY day, tenant_id
)
`

// rollup_cohort_monthly — month cohort funnel.
// Source: fact_execution_completion (first/second paid runs +
// totals). The job recomputes only the current month; older cohorts
// are append-only after their month closes.
const cohortMonthlySQL = `
INSERT INTO rollup_cohort_monthly
SELECT
    toStartOfMonth(min(occurred_at))                AS cohort_month,
    tenant_id,
    minIf(occurred_at, status IN ('settled','completed'))                     AS first_paid_at,
    minIf(occurred_at, status IN ('settled','completed')
                       AND occurred_at > minIf(occurred_at, status IN ('settled','completed'))) AS second_paid_at,
    count()                                         AS total_runs,
    sum(spent_usd)                                  AS spend_usd_sum,
    sum(revenue_usd)                                AS revenue_usd_sum,
    sumIf(toUInt64(1), refunded_usd > 0)            AS refund_count,
    sumIf(toUInt64(1), status IN ('settled','completed')) AS completed_count
FROM fact_execution_completion FINAL
WHERE occurred_at >= ? AND occurred_at < ?
GROUP BY tenant_id
`
