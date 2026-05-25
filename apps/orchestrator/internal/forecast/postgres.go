package forecast

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// PostgresForecaster reads historical cost samples from
// blueprint_runs (primary) and falls back to the executions table
// when no blueprint id is supplied. It applies the lookback windows
// declared in Config and degrades to the capability baseline when
// neither query returns enough rows.
//
// Migration 00027_blueprints.sql and 00026_executions.sql MUST have
// been applied first. The forecaster never writes; read-only seam.
type PostgresForecaster struct {
	pool *pgxpool.Pool
	cfg  Config
	now  func() time.Time
}

// NewPostgresForecaster wires the forecaster to an existing pgxpool.
// Pass cfg=DefaultConfig() when in doubt.
func NewPostgresForecaster(pool *pgxpool.Pool, cfg Config) *PostgresForecaster {
	return &PostgresForecaster{
		pool: pool,
		cfg:  cfg,
		now:  time.Now,
	}
}

// Estimate satisfies Forecaster.
//
// Lookup order:
//  1. blueprint_runs filtered by (blueprint_id, tenant_id) within the
//     primary lookback window.
//  2. Same filter widened to the fallback window.
//  3. blueprint_runs filtered by blueprint_id only (global) within
//     the primary window.
//  4. Same global query widened to the fallback window.
//  5. When no BlueprintID is supplied OR all previous queries came
//     up short, executions table filtered by tenant_id within the
//     primary then fallback window.
//  6. Capability baseline.
func (p *PostgresForecaster) Estimate(ctx context.Context, in EstimateInput) (Estimate, error) {
	if in.TenantID == "" {
		return Estimate{}, ErrInvalidInput
	}
	now := p.now()
	primarySince := now.Add(-p.cfg.PrimaryWindow)
	fallbackSince := now.Add(-p.cfg.FallbackWindow)

	if in.BlueprintID != "" {
		// 1. Tenant + blueprint, primary window.
		samples, err := p.blueprintRunSamples(ctx, in.BlueprintID, in.TenantID, primarySince)
		if err != nil {
			return Estimate{}, err
		}
		if len(samples) >= p.cfg.MinTenantSamples {
			return estimateFromSamples(in, samples, p.cfg), nil
		}
		// 2. Widen window.
		samples, err = p.blueprintRunSamples(ctx, in.BlueprintID, in.TenantID, fallbackSince)
		if err != nil {
			return Estimate{}, err
		}
		if len(samples) >= p.cfg.MinTenantSamples {
			return estimateFromSamples(in, samples, p.cfg), nil
		}
		// 3. Global (any tenant), primary window.
		samples, err = p.blueprintRunSamples(ctx, in.BlueprintID, "", primarySince)
		if err != nil {
			return Estimate{}, err
		}
		if len(samples) >= p.cfg.MinGlobalSamples {
			return estimateFromSamples(in, samples, p.cfg), nil
		}
		// 4. Global, widened.
		samples, err = p.blueprintRunSamples(ctx, in.BlueprintID, "", fallbackSince)
		if err != nil {
			return Estimate{}, err
		}
		if len(samples) >= p.cfg.MinGlobalSamples {
			return estimateFromSamples(in, samples, p.cfg), nil
		}
	}

	// 5. No blueprint OR blueprint history was sparse — fall back to
	// the tenant's own execution history.
	samples, err := p.tenantExecutionSamples(ctx, in.TenantID, primarySince)
	if err != nil {
		return Estimate{}, err
	}
	if len(samples) >= p.cfg.MinTenantSamples {
		return estimateFromSamples(in, samples, p.cfg), nil
	}
	samples, err = p.tenantExecutionSamples(ctx, in.TenantID, fallbackSince)
	if err != nil {
		return Estimate{}, err
	}
	if len(samples) >= p.cfg.MinTenantSamples {
		return estimateFromSamples(in, samples, p.cfg), nil
	}

	// 6. Capability baseline.
	return estimateBaseline(in, p.cfg), nil
}

// blueprintRunSamples reads cost_usd from blueprint_runs filtered by
// blueprint_id, optionally tenant_id, since. Pass tenantID="" to drop
// the tenant filter (used by the global fallback). The query reads
// cost_usd as text and decodes via decimal.NewFromString to preserve
// precision across the pgx boundary.
func (p *PostgresForecaster) blueprintRunSamples(ctx context.Context, blueprintID, tenantID string, since time.Time) ([]decimal.Decimal, error) {
	if tenantID != "" {
		rows, err := p.pool.Query(ctx, `
            SELECT cost_usd::text
              FROM blueprint_runs
             WHERE blueprint_id = $1
               AND tenant_id    = $2
               AND created_at  >= $3`,
			blueprintID, tenantID, since)
		if err != nil {
			return nil, fmt.Errorf("forecast: blueprint_runs query: %w", err)
		}
		defer rows.Close()
		return scanCostColumn(rows)
	}
	rows, err := p.pool.Query(ctx, `
        SELECT cost_usd::text
          FROM blueprint_runs
         WHERE blueprint_id = $1
           AND created_at  >= $2`,
		blueprintID, since)
	if err != nil {
		return nil, fmt.Errorf("forecast: blueprint_runs global query: %w", err)
	}
	defer rows.Close()
	return scanCostColumn(rows)
}

// tenantExecutionSamples reads the realised total cost
// (provider + sandbox + storage + deployment) from terminal
// executions in the window. Only succeeded / refunded executions
// inform the band — failed / killed rows would skew the median down.
func (p *PostgresForecaster) tenantExecutionSamples(ctx context.Context, tenantID string, since time.Time) ([]decimal.Decimal, error) {
	rows, err := p.pool.Query(ctx, `
        SELECT (provider_cost_usd + sandbox_cost_usd + storage_cost_usd + deployment_cost_usd)::text
          FROM executions
         WHERE tenant_id  = $1
           AND created_at >= $2
           AND status IN ('succeeded','refunded')`,
		tenantID, since)
	if err != nil {
		return nil, fmt.Errorf("forecast: executions query: %w", err)
	}
	defer rows.Close()
	return scanCostColumn(rows)
}

// pgxRows is the minimum surface we need from pgx.Rows so the cost
// scanner can be exercised by either backend without dragging the
// full driver type into the seam.
type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// scanCostColumn reads a single text column of decimal cost values
// out of a pgx.Rows. Rows that fail to decode are skipped silently —
// the estimator's job is to produce a defensible band, not to police
// the source data.
func scanCostColumn(rows pgxRows) ([]decimal.Decimal, error) {
	var out []decimal.Decimal
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("forecast: scan cost: %w", err)
		}
		d, err := decimal.NewFromString(s)
		if err != nil {
			continue
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("forecast: rows.Err: %w", err)
	}
	return out, nil
}

// Compile-time check.
var _ Forecaster = (*PostgresForecaster)(nil)
