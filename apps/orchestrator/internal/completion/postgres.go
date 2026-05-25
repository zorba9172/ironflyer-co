package completion

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresScorer is the Postgres-backed Scorer. Score events land in
// the append-only completion_scores table (migration 00028). On each
// Score(...) call we replay the latest pass/fail per gate for the
// execution, apply the new outcome, recompute, and write a single
// event row.
//
// The replay-on-write strategy is intentional: completion history per
// execution is bounded (one row per gate per gate run, capped by gate
// count and a small number of retries), and it keeps the table
// strictly append-only so the ledger / dashboards can rely on a
// monotone audit log.
type PostgresScorer struct {
	pool *pgxpool.Pool
}

// NewPostgresScorer wires the scorer to an existing pgxpool. The
// migration 00028_repair_genome.sql must have been applied first.
func NewPostgresScorer(pool *pgxpool.Pool) *PostgresScorer {
	return &PostgresScorer{pool: pool}
}

// Score records the gate outcome, recomputes the absolute score, and
// returns (newScore, delta, err).
func (s *PostgresScorer) Score(ctx context.Context, executionID string, outcome GateOutcome) (float64, float64, error) {
	// Replay the most recent outcome per gate so we can recompute the
	// absolute score deterministically. completion_scores is the
	// source of truth — we never carry per-process state.
	rows, err := s.pool.Query(ctx, `
        SELECT DISTINCT ON (gate_name) gate_name, delta
        FROM completion_scores
        WHERE execution_id = $1
        ORDER BY gate_name, recorded_at DESC`, executionID)
	if err != nil {
		return 0, 0, fmt.Errorf("completion: replay: %w", err)
	}
	defer rows.Close()
	latest := map[string]bool{}
	for rows.Next() {
		var gate string
		var delta float64
		if err := rows.Scan(&gate, &delta); err != nil {
			return 0, 0, fmt.Errorf("completion: scan replay: %w", err)
		}
		// A non-negative delta means the gate's most recent outcome
		// contributed (or held) its weight to the running score; a
		// negative delta means the gate just regressed. The exact
		// truth value is recoverable because each event records the
		// signed delta for that specific gate's outcome.
		latest[gate] = delta >= 0
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("completion: replay iter: %w", err)
	}

	// Previous absolute score = most recent event's score (any gate).
	var previous float64
	if err := s.pool.QueryRow(ctx, `
        SELECT COALESCE((
            SELECT score FROM completion_scores
            WHERE execution_id = $1
            ORDER BY recorded_at DESC
            LIMIT 1
        ), 0)`, executionID).Scan(&previous); err != nil {
		return 0, 0, fmt.Errorf("completion: previous: %w", err)
	}

	latest[outcome.Gate] = outcome.Passed
	newScore := computeScore(latest)
	delta := newScore - previous

	if _, err := s.pool.Exec(ctx, `
        INSERT INTO completion_scores(execution_id, gate_name, score, delta)
        VALUES ($1, $2, $3, $4)`, executionID, outcome.Gate, newScore, delta); err != nil {
		return 0, 0, fmt.Errorf("completion: insert: %w", err)
	}
	return newScore, delta, nil
}

// Get returns the most recent absolute score (0 if none).
func (s *PostgresScorer) Get(ctx context.Context, executionID string) (float64, error) {
	var score float64
	err := s.pool.QueryRow(ctx, `
        SELECT COALESCE((
            SELECT score FROM completion_scores
            WHERE execution_id = $1
            ORDER BY recorded_at DESC
            LIMIT 1
        ), 0)`, executionID).Scan(&score)
	if err != nil {
		return 0, fmt.Errorf("completion: get: %w", err)
	}
	return score, nil
}

// History returns the recorded events in chronological order.
func (s *PostgresScorer) History(ctx context.Context, executionID string) ([]ScoreEvent, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT gate_name, score, delta, recorded_at
        FROM completion_scores
        WHERE execution_id = $1
        ORDER BY recorded_at ASC`, executionID)
	if err != nil {
		return nil, fmt.Errorf("completion: history: %w", err)
	}
	defer rows.Close()
	var out []ScoreEvent
	for rows.Next() {
		var ev ScoreEvent
		if err := rows.Scan(&ev.Gate, &ev.Score, &ev.Delta, &ev.RecordedAt); err != nil {
			return nil, fmt.Errorf("completion: scan history: %w", err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("completion: history iter: %w", err)
	}
	return out, nil
}
