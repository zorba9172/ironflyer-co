package wallet

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// MemoryService is an in-process Service used by dev (`IRONFLYER_DB_DRIVER=
// memory`) and as a clean unit-of-work substrate for resolver wiring
// before Postgres is provisioned. All money math is decimal.Decimal so
// the memory and postgres paths produce bit-identical balances.
//
// Concurrency: a single sync.Mutex covers all wallets — the wallet hot
// path is per-execution, not per-token, so contention is fine and the
// alternative (per-tenant locks) doubles the surface for race bugs.
type MemoryService struct {
	mu      sync.Mutex
	wallets map[string]*Wallet
	topups  map[string][]TopUp // keyed by tenant
	// session index lets the webhook find the pending topup row by
	// stripe session id in O(1).
	bySession map[string]struct {
		tenant string
		index  int
	}
	// opKeys is the per-process dedupe log for opKey-keyed wallet
	// mutations (HoldWithKey / ReleaseWithKey / DebitWithKey /
	// TopUpWithKey). Mirrors the wallet_operations table in the
	// Postgres backend so dev / smoke / single-binary boots see
	// the same idempotency semantics under Temporal retries.
	opKeys map[string]opOutcome
}

// NewMemoryService constructs an empty in-memory wallet store.
func NewMemoryService() *MemoryService {
	return &MemoryService{
		wallets: map[string]*Wallet{},
		topups:  map[string][]TopUp{},
		bySession: map[string]struct {
			tenant string
			index  int
		}{},
		opKeys: map[string]opOutcome{},
	}
}

// opOutcome is the recorded outcome of a prior opKey-keyed mutation.
// Memory backend uses this to mirror the wallet_operations dedupe row
// in the Postgres backend.
type opOutcome struct {
	err error
}

// ensure returns the tenant wallet, creating a zeroed one on first use.
// Caller MUST hold s.mu.
func (s *MemoryService) ensure(tenant string) *Wallet {
	if w, ok := s.wallets[tenant]; ok {
		return w
	}
	now := time.Now().UTC()
	w := &Wallet{
		TenantID:  tenant,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.wallets[tenant] = w
	return w
}

// Get returns a copy so callers cannot mutate the stored wallet.
func (s *MemoryService) Get(_ context.Context, tenant string) (Wallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.ensure(tenant), nil
}

// TopUp credits balance + lifetime_topup and flips the pending row to
// succeeded. Idempotent on stripeSessionID.
func (s *MemoryService) TopUp(_ context.Context, tenant string, amount decimal.Decimal, stripeSessionID string) error {
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Idempotency: if the session already landed as succeeded, no-op.
	if ref, ok := s.bySession[stripeSessionID]; ok && stripeSessionID != "" {
		row := s.topups[ref.tenant][ref.index]
		if row.Status == "succeeded" {
			return nil
		}
	}

	w := s.ensure(tenant)
	w.BalanceUSD = w.BalanceUSD.Add(amount)
	w.LifetimeTopUpUSD = w.LifetimeTopUpUSD.Add(amount)
	w.UpdatedAt = time.Now().UTC()

	// Flip the pending row to succeeded if we have one; else record a
	// synthetic succeeded row so the ledger of top-ups stays complete.
	if ref, ok := s.bySession[stripeSessionID]; ok && stripeSessionID != "" {
		now := time.Now().UTC()
		row := s.topups[ref.tenant][ref.index]
		row.Status = "succeeded"
		row.CompletedAt = &now
		s.topups[ref.tenant][ref.index] = row
		return nil
	}
	now := time.Now().UTC()
	row := TopUp{
		ID:              uuid.NewString(),
		TenantID:        tenant,
		Provider:        ProviderFromSessionID(stripeSessionID),
		StripeSessionID: stripeSessionID,
		AmountUSD:       amount,
		Status:          "succeeded",
		CreatedAt:       now,
		CompletedAt:     &now,
	}
	s.topups[tenant] = append(s.topups[tenant], row)
	if stripeSessionID != "" {
		s.bySession[stripeSessionID] = struct {
			tenant string
			index  int
		}{tenant: tenant, index: len(s.topups[tenant]) - 1}
	}
	return nil
}

// Hold reserves amount against available balance.
func (s *MemoryService) Hold(_ context.Context, tenant string, amount decimal.Decimal) error {
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.ensure(tenant)
	if w.BalanceUSD.Sub(w.HoldUSD).LessThan(amount) {
		return ErrInsufficient
	}
	w.HoldUSD = w.HoldUSD.Add(amount)
	w.UpdatedAt = time.Now().UTC()
	metrics.IncWalletHoldsActive()
	return nil
}

// Release returns an unused hold back to available balance.
func (s *MemoryService) Release(_ context.Context, tenant string, amount decimal.Decimal) error {
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.ensure(tenant)
	if w.HoldUSD.LessThan(amount) {
		// Clamp at zero — releasing more than is held would violate
		// the CHECK (hold_usd >= 0) constraint in Postgres; mirror
		// that here.
		w.HoldUSD = decimal.Zero
	} else {
		w.HoldUSD = w.HoldUSD.Sub(amount)
	}
	w.UpdatedAt = time.Now().UTC()
	metrics.DecWalletHoldsActive()
	return nil
}

// Debit closes a previously-held amount.
func (s *MemoryService) Debit(_ context.Context, tenant string, amount decimal.Decimal) error {
	if amount.IsZero() || amount.IsNegative() {
		return ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.ensure(tenant)
	if w.BalanceUSD.LessThan(amount) || w.HoldUSD.LessThan(amount) {
		return ErrInsufficient
	}
	w.BalanceUSD = w.BalanceUSD.Sub(amount)
	w.HoldUSD = w.HoldUSD.Sub(amount)
	w.LifetimeSpendUSD = w.LifetimeSpendUSD.Add(amount)
	w.UpdatedAt = time.Now().UTC()
	metrics.DecWalletHoldsActive()
	return nil
}

// LifetimeStats returns the dashboard counters.
func (s *MemoryService) LifetimeStats(_ context.Context, tenant string) (LifetimeStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.ensure(tenant)
	return LifetimeStats{
		LifetimeTopUpUSD: w.LifetimeTopUpUSD,
		LifetimeSpendUSD: w.LifetimeSpendUSD,
	}, nil
}

// ListTopUps returns the most recent top-ups for the tenant, newest first.
func (s *MemoryService) ListTopUps(_ context.Context, tenant string, limit int) ([]TopUp, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]TopUp, len(s.topups[tenant]))
	copy(rows, s.topups[tenant])
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

// ListStalePending mirrors PostgresService.ListStalePending for the
// in-memory backend. Returns pending rows older than threshold across
// every tenant.
func (s *MemoryService) ListStalePending(_ context.Context, threshold time.Duration) ([]TopUp, error) {
	cutoff := time.Now().UTC().Add(-threshold)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []TopUp{}
	for _, rows := range s.topups {
		for _, t := range rows {
			if t.Status != "pending" || !t.CreatedAt.Before(cutoff) {
				continue
			}
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// MarkFailed flips a pending row to failed. No-op when the row is
// missing or in a terminal state.
func (s *MemoryService) MarkFailed(_ context.Context, stripeSessionID string) error {
	if stripeSessionID == "" {
		return ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.bySession[stripeSessionID]
	if !ok {
		return nil
	}
	row := s.topups[ref.tenant][ref.index]
	if row.Status != "pending" {
		return nil
	}
	now := time.Now().UTC()
	row.Status = "failed"
	row.CompletedAt = &now
	s.topups[ref.tenant][ref.index] = row
	return nil
}

// --- V22 opKey-aware idempotent variants -----------------------------
//
// Each *WithKey method consults s.opKeys before mutating. A prior
// success returns nil (the activity already landed); a prior failure
// replays the same error so retries surface a stable verdict instead
// of flipping outcomes. Empty opKey falls through to the non-idempotent
// method so existing callers that don't yet thread a key still work.

func (s *MemoryService) recallOp(opKey string) (opOutcome, bool) {
	if opKey == "" {
		return opOutcome{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.opKeys[opKey]
	return v, ok
}

func (s *MemoryService) rememberOp(opKey string, err error) {
	if opKey == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opKeys == nil {
		s.opKeys = map[string]opOutcome{}
	}
	s.opKeys[opKey] = opOutcome{err: err}
}

// HoldWithKey is the idempotent variant of Hold.
func (s *MemoryService) HoldWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if prev, ok := s.recallOp(opKey); ok {
		return prev.err
	}
	err := s.Hold(ctx, tenant, amount)
	s.rememberOp(opKey, err)
	return err
}

// ReleaseWithKey is the idempotent variant of Release.
func (s *MemoryService) ReleaseWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if prev, ok := s.recallOp(opKey); ok {
		return prev.err
	}
	err := s.Release(ctx, tenant, amount)
	s.rememberOp(opKey, err)
	return err
}

// DebitWithKey is the idempotent variant of Debit.
func (s *MemoryService) DebitWithKey(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if prev, ok := s.recallOp(opKey); ok {
		return prev.err
	}
	err := s.Debit(ctx, tenant, amount)
	s.rememberOp(opKey, err)
	return err
}

// TopUpWithKey is the idempotent variant of TopUp. opKey is the
// orchestrator-side dedupe handle; stripeSessionID is still consulted
// by TopUp's internal idempotency on its own.
func (s *MemoryService) TopUpWithKey(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID, opKey string) error {
	if prev, ok := s.recallOp(opKey); ok {
		return prev.err
	}
	err := s.TopUp(ctx, tenant, amount, stripeSessionID)
	s.rememberOp(opKey, err)
	return err
}

// CreatePendingTopUp records a Checkout session before the webhook
// fires.
func (s *MemoryService) CreatePendingTopUp(_ context.Context, tenant string, amount decimal.Decimal, stripeSessionID string) (TopUp, error) {
	if amount.IsZero() || amount.IsNegative() {
		return TopUp{}, ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	row := TopUp{
		ID:              uuid.NewString(),
		TenantID:        tenant,
		Provider:        ProviderFromSessionID(stripeSessionID),
		StripeSessionID: stripeSessionID,
		AmountUSD:       amount,
		Status:          "pending",
		CreatedAt:       time.Now().UTC(),
	}
	s.topups[tenant] = append(s.topups[tenant], row)
	if stripeSessionID != "" {
		s.bySession[stripeSessionID] = struct {
			tenant string
			index  int
		}{tenant: tenant, index: len(s.topups[tenant]) - 1}
	}
	return row, nil
}
