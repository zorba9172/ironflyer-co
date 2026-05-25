package profitguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/outboxhooks"
)

// RecordedDecision is one persisted row of the
// profit_guard_decisions table. The shape mirrors the migration
// (00029_profitguard_decisions.sql) one-to-one — every
// nullable column on disk is a pointer-or-empty field here.
//
// The audit row is denormalised on purpose: SpentUSD / ReservedUSD /
// EstimatedStepCostUSD are snapshotted at decision time so a later
// dashboard query never has to reconstruct historical wallet state.
type RecordedDecision struct {
	ID                      int64
	ExecutionID             string
	EnforcementPoint        EnforcementPoint
	Decision                Action
	Reason                  string
	SpentUSD                decimal.Decimal
	ReservedUSD             decimal.Decimal
	EstimatedStepCostUSD    decimal.Decimal
	ExpectedCompletionDelta float64
	ExpectedMarginPct       *float64
	RiskScore               *float64
	RecommendedProvider     string
	Metadata                map[string]any
	CreatedAt               time.Time
}

// DecisionStore is the persistence-agnostic contract for the audit
// table. Both the in-memory and Postgres backends implement it. The
// runtime calls Record exactly once per Decide; query methods are
// the read side for the GraphQL ProfitGuardDecisions field and the
// per-execution drill-down.
type DecisionStore interface {
	// Record appends a single decision row. Append-only by contract.
	Record(ctx context.Context, d RecordedDecision) error
	// ListByExecution returns every decision for an execution,
	// oldest-first so the dashboard can render them as a timeline.
	ListByExecution(ctx context.Context, executionID string) ([]RecordedDecision, error)
	// Recent returns the most recent decisions across all executions,
	// newest-first. Used by the operator dashboard.
	Recent(ctx context.Context, limit int) ([]RecordedDecision, error)
}

// MemoryStore is the in-process DecisionStore used in dev
// (`IRONFLYER_DB_DRIVER=memory`) and as a clean substrate before
// Postgres is provisioned. The implementation is a single-mutex
// slice — Profit Guard fires once per gate, not per token, so
// contention is fine.
type MemoryStore struct {
	mu     sync.Mutex
	rows   []RecordedDecision
	nextID int64
}

// NewMemoryStore constructs an empty in-memory audit store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Record appends d, assigning a monotonically-increasing ID and a UTC
// CreatedAt when those fields are zero. The store keeps a deep copy
// of Metadata so a later mutation on the caller's map doesn't bleed
// into the audit trail.
func (s *MemoryStore) Record(_ context.Context, d RecordedDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	if d.ID == 0 {
		d.ID = s.nextID
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	if d.Metadata != nil {
		cp := make(map[string]any, len(d.Metadata))
		for k, v := range d.Metadata {
			cp[k] = v
		}
		d.Metadata = cp
	}
	s.rows = append(s.rows, d)
	return nil
}

// ListByExecution returns a sorted copy of every row whose
// ExecutionID matches.
func (s *MemoryStore) ListByExecution(_ context.Context, executionID string) ([]RecordedDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RecordedDecision, 0, len(s.rows))
	for _, r := range s.rows {
		if r.ExecutionID == executionID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// Recent returns the most recent rows, newest-first.
func (s *MemoryStore) Recent(_ context.Context, limit int) ([]RecordedDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RecordedDecision, len(s.rows))
	copy(out, s.rows)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// PostgresStore is the production DecisionStore backed by the
// profit_guard_decisions table. The migration MUST be applied first
// (the store does not bootstrap schema). All inserts are single-row
// — there is no batching API because Decide fires at most once per
// enforcement point per step.
type PostgresStore struct {
	pool          *pgxpool.Pool
	outboxEnabled bool
}

// NewPostgresStore wires the store to an existing pgxpool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// WithOutbox enables durable event emission: every Decide call writes
// a profitguard.decisions outbox row in the same transaction as the
// audit insert, so dashboards and policy analysers can replay every
// decision offline without re-scanning Postgres.
func (s *PostgresStore) WithOutbox() *PostgresStore {
	if s != nil {
		s.outboxEnabled = true
	}
	return s
}

// Record inserts one row. Numeric columns are passed as text so the
// pgx driver doesn't lossily cast decimal.Decimal through float64.
func (s *PostgresStore) Record(ctx context.Context, d RecordedDecision) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("profitguard: postgres pool is nil")
	}
	metaJSON, err := marshalMetadata(d.Metadata)
	if err != nil {
		return fmt.Errorf("profitguard: marshal metadata: %w", err)
	}
	var marginArg any
	if d.ExpectedMarginPct != nil {
		marginArg = *d.ExpectedMarginPct
	}
	var riskArg any
	if d.RiskScore != nil {
		riskArg = *d.RiskScore
	}
	var providerArg any
	if d.RecommendedProvider != "" {
		providerArg = d.RecommendedProvider
	}
	const insertSQL = `
        INSERT INTO profit_guard_decisions(
            execution_id, enforcement_point, decision, reason,
            spent_usd, reserved_usd, estimated_step_cost_usd,
            expected_completion_delta, expected_margin_pct,
            risk_score, recommended_provider, metadata
        ) VALUES (
            $1, $2, $3, $4,
            $5::numeric, $6::numeric, $7::numeric,
            $8, $9, $10, $11, $12::jsonb
        )`
	args := []any{
		d.ExecutionID,
		string(d.EnforcementPoint),
		string(d.Decision),
		d.Reason,
		d.SpentUSD.String(),
		d.ReservedUSD.String(),
		d.EstimatedStepCostUSD.String(),
		d.ExpectedCompletionDelta,
		marginArg,
		riskArg,
		providerArg,
		metaJSON,
	}
	if !s.outboxEnabled {
		if _, err := s.pool.Exec(ctx, insertSQL, args...); err != nil {
			return fmt.Errorf("profitguard: insert decision: %w", err)
		}
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("profitguard: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, insertSQL, args...); err != nil {
		return fmt.Errorf("profitguard: insert decision: %w", err)
	}
	evt := outboxhooks.ProfitGuardDecisionEvent(
		d.ExecutionID,
		string(d.EnforcementPoint),
		string(d.Decision),
		d.Reason,
	)
	if err := outboxhooks.WriteEventInTx(ctx, tx, evt); err != nil {
		return fmt.Errorf("profitguard: enqueue event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("profitguard: commit: %w", err)
	}
	return nil
}

// ListByExecution returns every row for the execution, oldest-first.
func (s *PostgresStore) ListByExecution(ctx context.Context, executionID string) ([]RecordedDecision, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("profitguard: postgres pool is nil")
	}
	rows, err := s.pool.Query(ctx, `
        SELECT id, execution_id, enforcement_point, decision, reason,
               spent_usd::text, reserved_usd::text,
               estimated_step_cost_usd::text,
               expected_completion_delta, expected_margin_pct,
               risk_score, recommended_provider, metadata, created_at
        FROM profit_guard_decisions
        WHERE execution_id = $1
        ORDER BY created_at ASC, id ASC`, executionID)
	if err != nil {
		return nil, fmt.Errorf("profitguard: query by execution: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// Recent returns the newest rows across all executions.
func (s *PostgresStore) Recent(ctx context.Context, limit int) ([]RecordedDecision, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("profitguard: postgres pool is nil")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
        SELECT id, execution_id, enforcement_point, decision, reason,
               spent_usd::text, reserved_usd::text,
               estimated_step_cost_usd::text,
               expected_completion_delta, expected_margin_pct,
               risk_score, recommended_provider, metadata, created_at
        FROM profit_guard_decisions
        ORDER BY created_at DESC, id DESC
        LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("profitguard: query recent: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// scanRows converts a pgx.Rows cursor into the typed audit slice.
// Numeric columns arrive as text (we cast in the SELECT) so decimal
// round-trips bit-exact.
func scanRows(rows pgx.Rows) ([]RecordedDecision, error) {
	out := make([]RecordedDecision, 0)
	for rows.Next() {
		var (
			r        RecordedDecision
			point    string
			decision string
			spent    string
			reserved string
			step     string
			margin   *float64
			risk     *float64
			provider *string
			meta     []byte
		)
		if err := rows.Scan(
			&r.ID,
			&r.ExecutionID,
			&point,
			&decision,
			&r.Reason,
			&spent,
			&reserved,
			&step,
			&r.ExpectedCompletionDelta,
			&margin,
			&risk,
			&provider,
			&meta,
			&r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("profitguard: scan: %w", err)
		}
		r.EnforcementPoint = EnforcementPoint(point)
		r.Decision = Action(decision)
		var err error
		if r.SpentUSD, err = decimal.NewFromString(spent); err != nil {
			return nil, fmt.Errorf("profitguard: parse spent: %w", err)
		}
		if r.ReservedUSD, err = decimal.NewFromString(reserved); err != nil {
			return nil, fmt.Errorf("profitguard: parse reserved: %w", err)
		}
		if r.EstimatedStepCostUSD, err = decimal.NewFromString(step); err != nil {
			return nil, fmt.Errorf("profitguard: parse step cost: %w", err)
		}
		r.ExpectedMarginPct = margin
		r.RiskScore = risk
		if provider != nil {
			r.RecommendedProvider = *provider
		}
		if len(meta) > 0 {
			m := map[string]any{}
			if err := json.Unmarshal(meta, &m); err != nil {
				return nil, fmt.Errorf("profitguard: parse metadata: %w", err)
			}
			r.Metadata = m
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return out, nil
}

// marshalMetadata encodes the metadata map as JSON, returning the
// empty object literal when m is nil so the column NOT NULL DEFAULT
// '{}' constraint is satisfied without a roundtrip through nil.
func marshalMetadata(m map[string]any) ([]byte, error) {
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}
