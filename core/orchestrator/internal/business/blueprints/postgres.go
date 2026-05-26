package blueprints

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// PostgresStatsService is the production-grade StatsService backed
// by Postgres. RecordRun runs both writes (blueprint_runs INSERT +
// blueprint_stats UPSERT) inside a single transaction so a reader
// can never observe a row in one table without the matching counter
// bump in the other.
//
// Migration 00027_blueprints.sql MUST have been applied first; the
// service does not bootstrap schema (goose runs that elsewhere).
type PostgresStatsService struct {
	pool *pgxpool.Pool
}

// NewPostgresStatsService wires the stats service to an existing
// pgxpool.
func NewPostgresStatsService(pool *pgxpool.Pool) *PostgresStatsService {
	return &PostgresStatsService{pool: pool}
}

// RecordRun appends a blueprint_runs row and upserts the rolled-up
// counters in blueprint_stats. Both writes happen in one tx so
// failure leaves both tables untouched.
func (s *PostgresStatsService) RecordRun(ctx context.Context, o RunOutcome) error {
	if err := validateOutcome(o); err != nil {
		return err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("blueprints: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ttp any
	if o.TimeToPreviewSeconds > 0 {
		ttp = o.TimeToPreviewSeconds
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO blueprint_runs(
            blueprint_id, execution_id, tenant_id,
            revenue_usd, cost_usd, completion_score,
            preview_success, repaired, time_to_preview_seconds, refunded
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		o.BlueprintID,
		o.ExecutionID,
		o.TenantID,
		o.RevenueUSD.String(),
		o.CostUSD.String(),
		decimal.NewFromFloat(o.CompletionScore).String(),
		o.PreviewSuccess,
		o.Repaired,
		ttp,
		o.Refunded,
	); err != nil {
		return fmt.Errorf("blueprints: insert run: %w", err)
	}

	previewIncr := 0
	if o.PreviewSuccess {
		previewIncr = 1
	}
	refundIncr := 0
	if o.Refunded {
		refundIncr = 1
	}
	repairIncr := 0
	if o.Repaired {
		repairIncr = 1
	}
	ttpSecondsIncr := 0
	ttpCountIncr := 0
	if o.TimeToPreviewSeconds > 0 {
		ttpSecondsIncr = o.TimeToPreviewSeconds
		ttpCountIncr = 1
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO blueprint_stats(
            blueprint_id, executions, preview_success, refunds,
            total_revenue_usd, total_cost_usd, total_completion_score,
            repair_count, time_to_preview_seconds_sum, time_to_preview_count,
            updated_at
        ) VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, now())
        ON CONFLICT (blueprint_id) DO UPDATE SET
            executions                  = blueprint_stats.executions + 1,
            preview_success             = blueprint_stats.preview_success + EXCLUDED.preview_success,
            refunds                     = blueprint_stats.refunds + EXCLUDED.refunds,
            total_revenue_usd           = blueprint_stats.total_revenue_usd + EXCLUDED.total_revenue_usd,
            total_cost_usd              = blueprint_stats.total_cost_usd + EXCLUDED.total_cost_usd,
            total_completion_score      = blueprint_stats.total_completion_score + EXCLUDED.total_completion_score,
            repair_count                = blueprint_stats.repair_count + EXCLUDED.repair_count,
            time_to_preview_seconds_sum = blueprint_stats.time_to_preview_seconds_sum + EXCLUDED.time_to_preview_seconds_sum,
            time_to_preview_count       = blueprint_stats.time_to_preview_count + EXCLUDED.time_to_preview_count,
            updated_at                  = now()`,
		o.BlueprintID,
		previewIncr,
		refundIncr,
		o.RevenueUSD.String(),
		o.CostUSD.String(),
		decimal.NewFromFloat(o.CompletionScore).String(),
		repairIncr,
		ttpSecondsIncr,
		ttpCountIncr,
	); err != nil {
		return fmt.Errorf("blueprints: upsert stats: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("blueprints: commit: %w", err)
	}
	publishBlueprintOutcome(ctx, o)
	return nil
}

// Get reads one rolled-up row.
func (s *PostgresStatsService) Get(ctx context.Context, blueprintID string) (Stats, error) {
	row := s.pool.QueryRow(ctx, `
        SELECT blueprint_id, executions, preview_success, refunds,
               total_revenue_usd::text, total_cost_usd::text,
               total_completion_score::text, repair_count,
               time_to_preview_seconds_sum, time_to_preview_count,
               updated_at
        FROM blueprint_stats WHERE blueprint_id = $1`, blueprintID)
	stats, err := scanStats(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Stats{}, ErrNoStats
		}
		return Stats{}, fmt.Errorf("blueprints: get stats: %w", err)
	}
	return stats, nil
}

// All returns every rolled-up row ordered by blueprint id.
func (s *PostgresStatsService) All(ctx context.Context) ([]Stats, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT blueprint_id, executions, preview_success, refunds,
               total_revenue_usd::text, total_cost_usd::text,
               total_completion_score::text, repair_count,
               time_to_preview_seconds_sum, time_to_preview_count,
               updated_at
        FROM blueprint_stats ORDER BY blueprint_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("blueprints: list stats: %w", err)
	}
	defer rows.Close()
	out := []Stats{}
	for rows.Next() {
		s, err := scanStats(rows)
		if err != nil {
			return nil, fmt.Errorf("blueprints: scan stats: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Top defers ranking to applyTop so the ordering exactly matches the
// in-memory path. We could push the sort into SQL for some metrics,
// but ranking 3-30 blueprints in Go is free and keeps both backends
// honest.
func (s *PostgresStatsService) Top(ctx context.Context, byMetric string, limit int) ([]Stats, error) {
	all, err := s.All(ctx)
	if err != nil {
		return nil, err
	}
	return applyTop(all, byMetric, limit), nil
}

// scanStats is shared by Get + All. It accepts pgx.Row so a single-
// row QueryRow and a Query's rows.Next loop both feed the same
// parser.
func scanStats(row pgxRow) (Stats, error) {
	var (
		id                   string
		executions           int64
		previewSuccess       int64
		refunds              int64
		totalRevenueStr      string
		totalCostStr         string
		totalCompletionStr   string
		repairCount          int64
		ttpSum               int64
		ttpCount             int64
		updatedAt            time.Time
	)
	if err := row.Scan(
		&id,
		&executions,
		&previewSuccess,
		&refunds,
		&totalRevenueStr,
		&totalCostStr,
		&totalCompletionStr,
		&repairCount,
		&ttpSum,
		&ttpCount,
		&updatedAt,
	); err != nil {
		return Stats{}, err
	}
	totalRev, err := decimal.NewFromString(totalRevenueStr)
	if err != nil {
		return Stats{}, fmt.Errorf("decode total_revenue_usd: %w", err)
	}
	totalCost, err := decimal.NewFromString(totalCostStr)
	if err != nil {
		return Stats{}, fmt.Errorf("decode total_cost_usd: %w", err)
	}
	totalCompletion, err := decimal.NewFromString(totalCompletionStr)
	if err != nil {
		return Stats{}, fmt.Errorf("decode total_completion_score: %w", err)
	}
	stats := Stats{
		BlueprintID:    id,
		Executions:     executions,
		PreviewSuccess: previewSuccess,
		Refunds:        refunds,
		RepairCount:    repairCount,
		UpdatedAt:      updatedAt,
	}
	return computeDerived(stats, totalRev, totalCost, totalCompletion, ttpSum, ttpCount), nil
}

// pgxRow is the minimum surface we need from both pgx.Row and
// pgx.Rows so scanStats can serve both Get and All without
// duplication.
type pgxRow interface {
	Scan(dest ...any) error
}
