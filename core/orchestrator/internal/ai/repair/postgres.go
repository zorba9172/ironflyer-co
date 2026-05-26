package repair

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// PostgresGenome is the Postgres-backed Genome implementation. It
// reads/writes the repair_recipes table from migration 00028.
type PostgresGenome struct {
	pool *pgxpool.Pool
}

// NewPostgresGenome wires the genome to an existing pgxpool.
func NewPostgresGenome(pool *pgxpool.Pool) *PostgresGenome {
	return &PostgresGenome{pool: pool}
}

// Record upserts the (signature, category, fix) tuple. The UNIQUE
// constraint on failure_signature makes the operation idempotent.
func (g *PostgresGenome) Record(ctx context.Context, sig, category string, fix map[string]any) (Recipe, error) {
	fixJSON, err := json.Marshal(fix)
	if err != nil {
		return Recipe{}, fmt.Errorf("repair: marshal fix: %w", err)
	}
	row := g.pool.QueryRow(ctx, `
        INSERT INTO repair_recipes(failure_signature, category, fix_json)
        VALUES ($1, $2, $3::jsonb)
        ON CONFLICT (failure_signature) DO UPDATE
           SET category = EXCLUDED.category
        RETURNING id, failure_signature, category, fix_json, hits, successes,
                  COALESCE(last_hit_at, 'epoch'::timestamptz), created_at`,
		sig, category, string(fixJSON))
	return scanRecipe(row)
}

// Lookup returns the recipe and increments Hits + last_hit_at on a
// match. We do the increment inline so the read side can stay a
// single round-trip.
func (g *PostgresGenome) Lookup(ctx context.Context, sig string) (Recipe, bool, error) {
	row := g.pool.QueryRow(ctx, `
        UPDATE repair_recipes
           SET hits = hits + 1,
               last_hit_at = now()
         WHERE failure_signature = $1
        RETURNING id, failure_signature, category, fix_json, hits, successes,
                  COALESCE(last_hit_at, 'epoch'::timestamptz), created_at`, sig)
	r, err := scanRecipe(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Recipe{}, false, nil
		}
		return Recipe{}, false, err
	}
	// Feedback Brain: a recipe hit means we reused a learned fix —
	// publish so the miner can grow our reuse-rate metric.
	learning.Publish(ctx, learning.OutcomeEvent{
		Kind: learning.KindRepairTriggered,
		Attributes: map[string]any{
			"signature": sig,
			"category":  r.Category,
			"hits":      r.Hits,
			"reused":    true,
		},
		Success: learning.BoolPtr(true),
	})
	return r, true, nil
}

// MarkSuccess increments the Successes counter for the signature.
func (g *PostgresGenome) MarkSuccess(ctx context.Context, sig string) error {
	_, err := g.pool.Exec(ctx, `
        UPDATE repair_recipes
           SET successes = successes + 1
         WHERE failure_signature = $1`, sig)
	if err != nil {
		return fmt.Errorf("repair: mark success: %w", err)
	}
	return nil
}

// AttemptsByExecution returns per-execution recovery attempts. The
// repair_recipes table is keyed by failure_signature, not
// execution_id, so today this returns an empty slice — the wow-loop
// adapter reads the authoritative per-execution view from
// execution_events via
// execution.Service.RecoveryAttemptsByExecution.
//
// TODO(wave-3): if/when we add a recovery_attempts side table that
// records each (executionID, signature, applied, success, ts)
// tuple, query it here.
func (g *PostgresGenome) AttemptsByExecution(_ context.Context, executionID string) ([]Attempt, error) {
	if executionID == "" {
		return nil, nil
	}
	return []Attempt{}, nil
}

// Top returns the most-used recipes by Hits.
func (g *PostgresGenome) Top(ctx context.Context, limit int) ([]Recipe, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := g.pool.Query(ctx, `
        SELECT id, failure_signature, category, fix_json, hits, successes,
               COALESCE(last_hit_at, 'epoch'::timestamptz), created_at
        FROM repair_recipes
        ORDER BY hits DESC, last_hit_at DESC NULLS LAST
        LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("repair: top: %w", err)
	}
	defer rows.Close()
	out := make([]Recipe, 0, limit)
	for rows.Next() {
		r, err := scanRecipe(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repair: top iter: %w", err)
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecipe(row rowScanner) (Recipe, error) {
	var (
		r       Recipe
		fixJSON []byte
	)
	if err := row.Scan(
		&r.ID, &r.FailureSignature, &r.Category, &fixJSON,
		&r.Hits, &r.Successes, &r.LastHitAt, &r.CreatedAt,
	); err != nil {
		return Recipe{}, err
	}
	if len(fixJSON) > 0 {
		if err := json.Unmarshal(fixJSON, &r.Fix); err != nil {
			return Recipe{}, fmt.Errorf("repair: unmarshal fix: %w", err)
		}
	}
	// Normalise the sentinel epoch into a zero time for callers that
	// rely on time.IsZero().
	if r.LastHitAt.Equal(time.Unix(0, 0).UTC()) {
		r.LastHitAt = time.Time{}
	}
	return r, nil
}

// PostgresPatchStore is the Postgres-backed Memory implementation.
type PostgresPatchStore struct {
	pool *pgxpool.Pool
}

// NewPostgresPatchStore wires the patch store to an existing pgxpool.
func NewPostgresPatchStore(pool *pgxpool.Pool) *PostgresPatchStore {
	return &PostgresPatchStore{pool: pool}
}

// Record inserts a new PatchEntry for the intent.
func (m *PostgresPatchStore) Record(ctx context.Context, intent string, patch map[string]any, paths []string, cost decimal.Decimal) (PatchEntry, error) {
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return PatchEntry{}, fmt.Errorf("repair: marshal patch: %w", err)
	}
	if paths == nil {
		paths = []string{}
	}
	row := m.pool.QueryRow(ctx, `
        INSERT INTO patch_memory(intent_signature, patch_json, affected_paths, cost_usd)
        VALUES ($1, $2::jsonb, $3, $4)
        RETURNING id, intent_signature, patch_json, affected_paths,
                  cost_usd::text, applied_count, success_count, created_at,
                  COALESCE(last_applied_at, 'epoch'::timestamptz)`,
		intent, string(patchJSON), paths, cost.String())
	return scanPatchEntry(row)
}

// Find returns every PatchEntry matching the intent signature.
func (m *PostgresPatchStore) Find(ctx context.Context, intent string) ([]PatchEntry, error) {
	rows, err := m.pool.Query(ctx, `
        SELECT id, intent_signature, patch_json, affected_paths,
               cost_usd::text, applied_count, success_count, created_at,
               COALESCE(last_applied_at, 'epoch'::timestamptz)
        FROM patch_memory
        WHERE intent_signature = $1
        ORDER BY created_at DESC`, intent)
	if err != nil {
		return nil, fmt.Errorf("repair: find patches: %w", err)
	}
	defer rows.Close()
	var out []PatchEntry
	for rows.Next() {
		e, err := scanPatchEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repair: find patches iter: %w", err)
	}
	return out, nil
}

// MarkApplied bumps AppliedCount and (on success) SuccessCount.
func (m *PostgresPatchStore) MarkApplied(ctx context.Context, id uuid.UUID, success bool) error {
	succInc := 0
	if success {
		succInc = 1
	}
	_, err := m.pool.Exec(ctx, `
        UPDATE patch_memory
           SET applied_count   = applied_count + 1,
               success_count   = success_count + $2,
               last_applied_at = now()
         WHERE id = $1`, id, succInc)
	if err != nil {
		return fmt.Errorf("repair: mark applied: %w", err)
	}
	return nil
}

func scanPatchEntry(row rowScanner) (PatchEntry, error) {
	var (
		e         PatchEntry
		patchJSON []byte
		costStr   string
	)
	if err := row.Scan(
		&e.ID, &e.IntentSignature, &patchJSON, &e.AffectedPaths,
		&costStr, &e.AppliedCount, &e.SuccessCount, &e.CreatedAt,
		&e.LastAppliedAt,
	); err != nil {
		return PatchEntry{}, err
	}
	if len(patchJSON) > 0 {
		if err := json.Unmarshal(patchJSON, &e.Patch); err != nil {
			return PatchEntry{}, fmt.Errorf("repair: unmarshal patch: %w", err)
		}
	}
	cost, err := decimal.NewFromString(costStr)
	if err != nil {
		return PatchEntry{}, fmt.Errorf("repair: parse cost: %w", err)
	}
	e.CostUSD = cost
	if e.LastAppliedAt.Equal(time.Unix(0, 0).UTC()) {
		e.LastAppliedAt = time.Time{}
	}
	return e, nil
}
