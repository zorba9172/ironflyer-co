package shippass

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/learning"
	"ironflyer/core/orchestrator/internal/business/wallet"
)

// MemoryService is the in-process implementation. Used by dev
// (`IRONFLYER_DB_DRIVER=memory`) and as the substrate the smoke runs
// against before postgres is provisioned. Concurrency: a single
// sync.Mutex covers everything because the Ship Pass mutation rate is
// orders of magnitude below the wallet mutation rate, so the simpler
// lock is correct.
type MemoryService struct {
	mu       sync.Mutex
	passes   map[string]*ShipPass
	byTenant map[string][]string // tenant → pass ids, append-only
	progress map[string][]GateProgress

	wallet wallet.IdempotentService
}

// NewMemoryService constructs an empty memory backend. The wallet is
// the live wallet.IdempotentService used by every other paid path —
// Ship Pass deliberately reuses it so holds, debits and releases are
// indistinguishable from any other wallet activity in the ledger.
func NewMemoryService(walletSvc wallet.IdempotentService) *MemoryService {
	return &MemoryService{
		passes:   map[string]*ShipPass{},
		byTenant: map[string][]string{},
		progress: map[string][]GateProgress{},
		wallet:   walletSvc,
	}
}

// Quote previews the buy price without mutating anything. Walletshortfall
// is positive when the wallet falls short — the resolver renders it
// as "top up $X to unlock".
func (s *MemoryService) Quote(ctx context.Context, tenant, _ /*projectID*/, tierKey string) (Quote, error) {
	tier, ok := TierByKey(tierKey)
	if !ok {
		return Quote{}, ErrInvalidTier
	}
	w, err := s.wallet.Get(ctx, tenant)
	if err != nil {
		return Quote{}, err
	}
	shortfall := tier.PriceUSD.Sub(w.AvailableUSD())
	if shortfall.IsNegative() {
		shortfall = decimal.Zero
	}
	return Quote{
		TierKey:         tier.Key,
		PriceUSD:        tier.PriceUSD,
		RequiredGates:   tier.sortedGates(),
		DeadlineDays:    tier.DeadlineDays,
		WalletShortfall: shortfall,
	}, nil
}

// Purchase reserves funds in the wallet and creates the pass row in
// one critical section. Uniqueness on (tenant, project, active) is
// enforced by scanning byTenant — at the dev scale this is O(n) and
// fine; postgres uses a partial unique index for the same guarantee.
func (s *MemoryService) Purchase(ctx context.Context, tenant, projectID, tierKey, requestID string) (ShipPass, error) {
	tier, ok := TierByKey(tierKey)
	if !ok {
		return ShipPass{}, ErrInvalidTier
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	holdKey := "shippass-hold-" + requestID

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pid := range s.byTenant[tenant] {
		row := s.passes[pid]
		if row.ProjectID == projectID && row.Status == StatusActive {
			return *row, ErrPassNotActive
		}
	}

	// Reserve funds outside the lock would be ideal; we hold the lock
	// because the postgres backend MUST do this in one transaction
	// (uniqueness + hold), and we want the memory backend to expose
	// the same observable atomicity for tests-by-running.
	if err := s.wallet.HoldWithKey(ctx, tenant, tier.PriceUSD, holdKey); err != nil {
		return ShipPass{}, err
	}

	now := time.Now().UTC()
	id := uuid.NewString()
	row := &ShipPass{
		ID:         id,
		TenantID:   tenant,
		ProjectID:  projectID,
		TierKey:    tier.Key,
		PriceUSD:   tier.PriceUSD,
		Status:     StatusActive,
		DeadlineAt: tier.Deadline(now),
		CreatedAt:  now,
		UpdatedAt:  now,
		HoldOpKey:  holdKey,
	}
	s.passes[id] = row
	s.byTenant[tenant] = append(s.byTenant[tenant], id)

	publishLifecycle(ctx, *row, "purchased", nil)
	return *row, nil
}

// Cancel releases the hold and flips the row to cancelled.
func (s *MemoryService) Cancel(ctx context.Context, tenant, passID string) (ShipPass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.passes[passID]
	if !ok || row.TenantID != tenant {
		return ShipPass{}, ErrPassNotFound
	}
	if row.Status != StatusActive {
		return *row, ErrPassNotActive
	}

	refundKey := "shippass-cancel-" + passID
	if err := s.wallet.ReleaseWithKey(ctx, tenant, row.PriceUSD, refundKey); err != nil {
		return *row, err
	}
	now := time.Now().UTC()
	row.Status = StatusCancelled
	row.UpdatedAt = now
	row.SettledAt = &now
	row.RefundOpKey = refundKey

	publishLifecycle(ctx, *row, "cancelled", nil)
	return *row, nil
}

// Get returns a copy so callers cannot mutate the stored row.
func (s *MemoryService) Get(_ context.Context, tenant, passID string) (ShipPass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.passes[passID]
	if !ok || row.TenantID != tenant {
		return ShipPass{}, ErrPassNotFound
	}
	return *row, nil
}

// ActiveForProject scans the tenant's pass index. At dev scale this
// is fine; the postgres backend uses a covering index.
func (s *MemoryService) ActiveForProject(_ context.Context, tenant, projectID string) (ShipPass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pid := range s.byTenant[tenant] {
		row := s.passes[pid]
		if row.ProjectID == projectID && row.Status == StatusActive {
			return *row, nil
		}
	}
	return ShipPass{}, ErrPassNotFound
}

// List returns up to limit recent passes for tenant, newest first.
func (s *MemoryService) List(_ context.Context, tenant string, limit int) ([]ShipPass, error) {
	if limit <= 0 {
		limit = 25
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.byTenant[tenant]
	out := make([]ShipPass, 0, len(ids))
	for _, pid := range ids {
		out = append(out, *s.passes[pid])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// RecordGateVerdict appends progress and, when the required set is
// fully covered, settles the pass to `shipped`. The "did the new
// observation complete the set?" check intentionally re-scans every
// observed gate so a flapping gate (pass → fail → pass) correctly
// holds the pass open until the latest observation is positive.
func (s *MemoryService) RecordGateVerdict(ctx context.Context, passID string, gate domain.GateName, passed bool, reason string, observedAt time.Time) (ShipPass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.passes[passID]
	if !ok {
		return ShipPass{}, ErrPassNotFound
	}

	// Always record the verdict for audit, regardless of tier scope.
	rec := GateProgress{
		ID:         uuid.NewString(),
		ShipPassID: passID,
		Gate:       gate,
		Passed:     passed,
		Reason:     reason,
		ObservedAt: observedAt.UTC(),
	}
	s.progress[passID] = append(s.progress[passID], rec)

	if row.Status != StatusActive {
		return *row, nil
	}

	tier, ok := TierByKey(row.TierKey)
	if !ok {
		return *row, nil
	}
	required := tier.requiredGateSet()
	if _, inScope := required[gate]; !inScope {
		return *row, nil
	}

	latest := latestPerGate(s.progress[passID])
	for g := range required {
		state, seen := latest[g]
		if !seen || !state.Passed {
			return *row, nil
		}
	}

	debitKey := "shippass-debit-" + passID
	if err := s.wallet.DebitWithKey(ctx, row.TenantID, row.PriceUSD, debitKey); err != nil {
		return *row, err
	}
	now := time.Now().UTC()
	row.Status = StatusShipped
	row.UpdatedAt = now
	row.SettledAt = &now
	row.DebitOpKey = debitKey

	publishLifecycle(ctx, *row, "shipped", &row.PriceUSD)
	return *row, nil
}

// ProgressFor returns every recorded verdict for the pass in
// observation order. Used by the resolver to render the timeline.
func (s *MemoryService) ProgressFor(_ context.Context, tenant, passID string) ([]GateProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.passes[passID]
	if !ok || row.TenantID != tenant {
		return nil, ErrPassNotFound
	}
	out := make([]GateProgress, len(s.progress[passID]))
	copy(out, s.progress[passID])
	return out, nil
}

// ExpireDue flips every active row whose deadline has passed into
// `refunded` and releases the hold.
func (s *MemoryService) ExpireDue(ctx context.Context, now time.Time) ([]ShipPass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	expired := []ShipPass{}
	for _, row := range s.passes {
		if row.Status != StatusActive {
			continue
		}
		if !now.After(row.DeadlineAt) {
			continue
		}
		refundKey := "shippass-expire-" + row.ID
		if err := s.wallet.ReleaseWithKey(ctx, row.TenantID, row.PriceUSD, refundKey); err != nil {
			// Surface as audit; do not stop the sweep — one tenant's
			// transient wallet error must not block the rest.
			continue
		}
		t := now.UTC()
		row.Status = StatusRefunded
		row.UpdatedAt = t
		row.SettledAt = &t
		row.RefundOpKey = refundKey
		expired = append(expired, *row)
		publishLifecycle(ctx, *row, "refunded", nil)
	}
	return expired, nil
}

// LifetimeStats projects the tenant's history into the four headline
// counters and revenue total. Iterating every row is fine at dev
// scale; the postgres backend uses a materialised query.
func (s *MemoryService) LifetimeStats(_ context.Context, tenant string) (LifetimeStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := LifetimeStats{RevenueUSD: decimal.Zero}
	for _, pid := range s.byTenant[tenant] {
		row := s.passes[pid]
		stats.TotalPurchased++
		switch row.Status {
		case StatusShipped:
			stats.TotalShipped++
			stats.RevenueUSD = stats.RevenueUSD.Add(row.PriceUSD)
		case StatusRefunded:
			stats.TotalRefunded++
		case StatusCancelled:
			stats.TotalCancelled++
		}
	}
	return stats, nil
}

// latestPerGate reduces an observation log to the most recent verdict
// per gate. A pass needs the *latest* observation for every required
// gate to be positive, not just any past observation — otherwise a
// gate that flapped to fail after passing once would still ship.
func latestPerGate(rows []GateProgress) map[domain.GateName]GateProgress {
	latest := map[domain.GateName]GateProgress{}
	for _, r := range rows {
		prev, seen := latest[r.Gate]
		if !seen || r.ObservedAt.After(prev.ObservedAt) {
			latest[r.Gate] = r
		}
	}
	return latest
}

// publishLifecycle emits a learning OutcomeEvent for every state
// transition. Best-effort: a nil publisher (no Feedback Brain wired)
// is a silent no-op so resolvers never break on a missing global.
func publishLifecycle(ctx context.Context, p ShipPass, action string, revenue *decimal.Decimal) {
	success := p.Status == StatusShipped
	attrs := map[string]any{
		"pass_id":    p.ID,
		"project_id": p.ProjectID,
		"tier":       p.TierKey,
		"action":     action,
		"status":     string(p.Status),
		"price_usd":  p.PriceUSD.String(),
	}
	tags := map[string]string{
		"surface": "shippass",
		"tier":    p.TierKey,
	}
	evt := learning.OutcomeEvent{
		ID:         uuid.NewString(),
		TenantID:   p.TenantID,
		Kind:       learning.OutcomeKind("ship_pass_" + action),
		Timestamp:  time.Now().UTC(),
		Attributes: attrs,
		Tags:       tags,
	}
	if p.Status == StatusShipped || p.Status == StatusRefunded || p.Status == StatusCancelled {
		evt.Success = &success
	}
	if revenue != nil {
		evt.MarginUSD = revenue
	}
	learning.Publish(ctx, evt)
}
