package abuse

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
)

// Store is the persistence layer for the abuse engine. Two
// implementations ship: MemoryStore (dev / single-node default) and
// PostgresStore (production). Both are interchangeable behind the
// interface, and both treat a zero userID as "tenant-wide" — used by
// operator overrides via SetScore.
type Store interface {
	RecordSignal(ctx context.Context, tenantID, userID string, st SignalType, weight int, signalCtx map[string]any) error
	SumWeights(ctx context.Context, tenantID, userID string, since time.Time) (int, map[SignalType]int, error)
	UpsertScore(ctx context.Context, tenantID, userID string, score int, tier Tier, breakdown map[SignalType]int, reason string) error
	GetScore(ctx context.Context, tenantID, userID string) (score int, tier Tier, found bool, err error)
	Recent(ctx context.Context, tenantID string, limit int) ([]ScoredSignal, error)
}

// MemoryStore is an in-process Store useful for dev mode and tests.
// It keeps signals in a slice (newest last) and scores in a map; both
// are guarded by a single RWMutex. The cardinality model assumes
// per-tenant-per-user → fine for the dev cardinality we care about.
type MemoryStore struct {
	mu      sync.RWMutex
	signals []ScoredSignal
	scores  map[string]storedScore
}

type storedScore struct {
	score     int
	tier      Tier
	breakdown map[SignalType]int
	reason    string
	updated   time.Time
}

// NewMemoryStore builds an empty in-process abuse store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{scores: map[string]storedScore{}}
}

func memKey(tenant, user string) string { return tenant + "/" + user }

// RecordSignal appends to the in-memory ring (capped at 4096 events).
func (m *MemoryStore) RecordSignal(_ context.Context, tenantID, userID string, st SignalType, weight int, signalCtx map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signals = append(m.signals, ScoredSignal{
		TenantID:   tenantID,
		UserID:     userID,
		Type:       st,
		Weight:     weight,
		Context:    signalCtx,
		RecordedAt: time.Now().Unix(),
	})
	if len(m.signals) > 4096 {
		// drop oldest half — keeps memory bounded without thrashing.
		m.signals = append([]ScoredSignal(nil), m.signals[len(m.signals)/2:]...)
	}
	return nil
}

func (m *MemoryStore) SumWeights(_ context.Context, tenantID, userID string, since time.Time) (int, map[SignalType]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := 0
	breakdown := map[SignalType]int{}
	cutoff := since.Unix()
	for _, s := range m.signals {
		if s.TenantID != tenantID || s.UserID != userID {
			continue
		}
		if s.RecordedAt < cutoff {
			continue
		}
		total += s.Weight
		breakdown[s.Type] += s.Weight
	}
	return total, breakdown, nil
}

func (m *MemoryStore) UpsertScore(_ context.Context, tenantID, userID string, score int, tier Tier, breakdown map[SignalType]int, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scores[memKey(tenantID, userID)] = storedScore{
		score:     score,
		tier:      tier,
		breakdown: breakdown,
		reason:    reason,
		updated:   time.Now(),
	}
	return nil
}

func (m *MemoryStore) GetScore(_ context.Context, tenantID, userID string) (int, Tier, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.scores[memKey(tenantID, userID)]
	if !ok {
		return 0, TierNormal, false, nil
	}
	return s.score, s.tier, true, nil
}

func (m *MemoryStore) Recent(_ context.Context, tenantID string, limit int) ([]ScoredSignal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	filtered := make([]ScoredSignal, 0, limit)
	// walk from the end so newest comes first.
	for i := len(m.signals) - 1; i >= 0 && len(filtered) < limit; i-- {
		if m.signals[i].TenantID != tenantID {
			continue
		}
		filtered = append(filtered, m.signals[i])
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].RecordedAt > filtered[j].RecordedAt })
	return filtered, nil
}

// PostgresStore is the production Store backed by pgxpool. The schema
// lives in migrations 00035; this struct holds no state beyond the
// pool pointer.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore wires the abuse engine to an existing pgxpool. The
// pool is assumed to be lifecycle-managed by the orchestrator's main.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (p *PostgresStore) RecordSignal(ctx context.Context, tenantID, userID string, st SignalType, weight int, signalCtx map[string]any) error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("%w: nil pool", ErrStoreUnavailable)
	}
	payload, err := json.Marshal(signalCtx)
	if err != nil {
		payload = []byte("{}")
	}
	// Allow NULL user_id for tenant-wide signals (operator overrides,
	// inbound webhook abuse). pgx treats empty string as a typed
	// parameter, so we have to pass a typed nullable explicitly.
	var userArg any = userID
	if userID == "" {
		userArg = nil
	}
	_, err = p.pool.Exec(ctx,
		`INSERT INTO abuse_signals (tenant_id, user_id, signal_type, weight, context)
		 VALUES ($1, $2, $3, $4, $5::jsonb)`,
		tenantID, userArg, string(st), weight, string(payload),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	return nil
}

func (p *PostgresStore) SumWeights(ctx context.Context, tenantID, userID string, since time.Time) (int, map[SignalType]int, error) {
	if p == nil || p.pool == nil {
		return 0, nil, fmt.Errorf("%w: nil pool", ErrStoreUnavailable)
	}
	var userArg any = userID
	if userID == "" {
		userArg = nil
	}
	rows, err := p.pool.Query(ctx,
		`SELECT signal_type, COALESCE(SUM(weight),0)::int
		   FROM abuse_signals
		  WHERE tenant_id = $1
		    AND ($2::uuid IS NULL OR user_id = $2::uuid)
		    AND recorded_at >= $3
		  GROUP BY signal_type`,
		tenantID, userArg, since,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	defer rows.Close()
	breakdown := map[SignalType]int{}
	total := 0
	for rows.Next() {
		var st string
		var sum int
		if err := rows.Scan(&st, &sum); err != nil {
			return 0, nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
		}
		breakdown[SignalType(st)] = sum
		total += sum
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	return total, breakdown, nil
}

func (p *PostgresStore) UpsertScore(ctx context.Context, tenantID, userID string, score int, tier Tier, breakdown map[SignalType]int, reason string) error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("%w: nil pool", ErrStoreUnavailable)
	}
	payload, err := json.Marshal(breakdown)
	if err != nil {
		payload = []byte("{}")
	}
	var userArg any = userID
	if userID == "" {
		userArg = nil
	}
	_, err = p.pool.Exec(ctx,
		`INSERT INTO abuse_scores (tenant_id, user_id, score, tier, signals, reason, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, now())
		 ON CONFLICT (tenant_id, user_id) DO UPDATE
		   SET score = EXCLUDED.score,
		       tier  = EXCLUDED.tier,
		       signals = EXCLUDED.signals,
		       reason = EXCLUDED.reason,
		       updated_at = now()`,
		tenantID, userArg, score, string(tier), string(payload), nullStr(reason),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	return nil
}

func (p *PostgresStore) GetScore(ctx context.Context, tenantID, userID string) (int, Tier, bool, error) {
	if p == nil || p.pool == nil {
		return 0, TierNormal, false, fmt.Errorf("%w: nil pool", ErrStoreUnavailable)
	}
	var userArg any = userID
	if userID == "" {
		userArg = nil
	}
	var score int
	var tier string
	err := p.pool.QueryRow(ctx,
		`SELECT score, tier FROM abuse_scores
		  WHERE tenant_id = $1
		    AND ($2::uuid IS NULL OR user_id = $2::uuid)
		  LIMIT 1`,
		tenantID, userArg,
	).Scan(&score, &tier)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, TierNormal, false, nil
	}
	if err != nil {
		return 0, TierNormal, false, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	t, _ := ParseTier(tier)
	return score, t, true, nil
}

func (p *PostgresStore) Recent(ctx context.Context, tenantID string, limit int) ([]ScoredSignal, error) {
	if p == nil || p.pool == nil {
		return nil, fmt.Errorf("%w: nil pool", ErrStoreUnavailable)
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx,
		`SELECT tenant_id::text, COALESCE(user_id::text,''), signal_type, weight, context, recorded_at
		   FROM abuse_signals
		  WHERE tenant_id = $1
		  ORDER BY recorded_at DESC
		  LIMIT $2`,
		tenantID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	defer rows.Close()
	out := make([]ScoredSignal, 0, limit)
	for rows.Next() {
		var s ScoredSignal
		var st string
		var raw []byte
		var ts time.Time
		if err := rows.Scan(&s.TenantID, &s.UserID, &st, &s.Weight, &raw, &ts); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
		}
		s.Type = SignalType(st)
		s.RecordedAt = ts.Unix()
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &s.Context)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	return out, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
