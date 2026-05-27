package provisioning

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// MemoryService is the in-process Service used by dev (no Postgres
// wired) and as the seed substrate for resolver wiring before the
// provisioning migration lands in prod. Concurrency: a single
// sync.Mutex covers both maps — provisioning is per-rail-onboarding
// throughput, not per-token, so contention is irrelevant.
type MemoryService struct {
	mu        sync.Mutex
	resources map[string]ProvisionedResource // id -> resource
	revenue   map[string][]RevenueEvent      // resourceID -> events
	// externalIndex deduplicates Provision idempotency on (tenant, externalID)
	// — a re-run of the Stripe Connect onboarding flow for the same
	// merchant should fold onto the same row instead of inserting a
	// second `pending` shell.
	externalIndex map[string]string // tenant|externalID -> resource id
	// refIndex deduplicates RecordRevenue on (resourceID, externalRef).
	// Stripe webhook redeliveries hit this and short-circuit.
	refIndex map[string]string // resourceID|externalRef -> event id
}

// NewMemoryService constructs an empty in-memory provisioning store.
func NewMemoryService() *MemoryService {
	return &MemoryService{
		resources:     map[string]ProvisionedResource{},
		revenue:       map[string][]RevenueEvent{},
		externalIndex: map[string]string{},
		refIndex:      map[string]string{},
	}
}

// Provision implements Service. Idempotent on (tenant, externalID).
func (s *MemoryService) Provision(ctx context.Context, r ProvisionedResource) (ProvisionedResource, error) {
	if r.TenantID == "" || r.ProjectID == "" || r.Kind == "" {
		return ProvisionedResource{}, ErrUnknownKind
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ExternalID != "" {
		key := r.TenantID + "|" + r.ExternalID
		if existing, ok := s.externalIndex[key]; ok {
			return s.resources[existing], nil
		}
	}
	now := time.Now().UTC()
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.Status == "" {
		r.Status = StatusPending
	}
	r.CreatedAt = now
	r.UpdatedAt = now
	s.resources[r.ID] = r
	if r.ExternalID != "" {
		s.externalIndex[r.TenantID+"|"+r.ExternalID] = r.ID
	}
	publishProvisioned(ctx, r)
	return r, nil
}

// Get implements Service.
func (s *MemoryService) Get(_ context.Context, tenant, id string) (ProvisionedResource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.resources[id]
	if !ok {
		return ProvisionedResource{}, ErrResourceNotFound
	}
	if r.TenantID != tenant {
		return ProvisionedResource{}, ErrResourceNotFound
	}
	return r, nil
}

// List implements Service.
func (s *MemoryService) List(_ context.Context, tenant, project string) ([]ProvisionedResource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ProvisionedResource, 0)
	for _, r := range s.resources {
		if r.TenantID != tenant || r.ProjectID != project {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// UpdateStatus implements Service.
func (s *MemoryService) UpdateStatus(ctx context.Context, tenant, id, status string) (ProvisionedResource, error) {
	s.mu.Lock()
	r, ok := s.resources[id]
	if !ok {
		s.mu.Unlock()
		return ProvisionedResource{}, ErrResourceNotFound
	}
	if r.TenantID != tenant {
		s.mu.Unlock()
		return ProvisionedResource{}, ErrForbidden
	}
	r.Status = status
	r.UpdatedAt = time.Now().UTC()
	s.resources[id] = r
	s.mu.Unlock()
	publishStatusChange(ctx, r)
	return r, nil
}

// RecordRevenue implements Service. Idempotent on (resourceID, externalRef).
func (s *MemoryService) RecordRevenue(ctx context.Context, e RevenueEvent) (RevenueEvent, error) {
	if !e.GrossAmountUSD.IsPositive() {
		return RevenueEvent{}, ErrInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.resources[e.ResourceID]; !exists {
		return RevenueEvent{}, ErrResourceNotFound
	}
	if e.ExternalRef != "" {
		key := e.ResourceID + "|" + e.ExternalRef
		if _, dup := s.refIndex[key]; dup {
			return RevenueEvent{}, ErrDuplicateEvent
		}
	}
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	s.revenue[e.ResourceID] = append(s.revenue[e.ResourceID], e)
	if e.ExternalRef != "" {
		s.refIndex[e.ResourceID+"|"+e.ExternalRef] = e.ID
	}
	resource := s.resources[e.ResourceID]
	publishRevenue(ctx, resource, e)
	return e, nil
}

// ListRevenue implements Service.
func (s *MemoryService) ListRevenue(_ context.Context, tenant, resourceID string, limit int) ([]RevenueEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.resources[resourceID]
	if !ok || r.TenantID != tenant {
		return nil, ErrResourceNotFound
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows := make([]RevenueEvent, len(s.revenue[resourceID]))
	copy(rows, s.revenue[resourceID])
	sort.Slice(rows, func(i, j int) bool { return rows[i].OccurredAt.After(rows[j].OccurredAt) })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

// SumRevenue implements Service.
func (s *MemoryService) SumRevenue(_ context.Context, tenant, resourceID string) (CutTotals, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.resources[resourceID]
	if !ok || r.TenantID != tenant {
		return CutTotals{}, ErrResourceNotFound
	}
	var totals CutTotals
	for _, e := range s.revenue[resourceID] {
		totals.GrossUSD = totals.GrossUSD.Add(e.GrossAmountUSD)
		totals.CutUSD = totals.CutUSD.Add(e.IronflyerCutUSD)
		totals.EventCount++
		t := e.OccurredAt
		if totals.FirstEventAt == nil || t.Before(*totals.FirstEventAt) {
			totals.FirstEventAt = &t
		}
		if totals.LastEventAt == nil || t.After(*totals.LastEventAt) {
			totals.LastEventAt = &t
		}
	}
	return totals, nil
}

// --- OutcomeEvent emissions -----------------------------------------
//
// Every state change publishes through learning.Publish so the Feedback
// Brain can surface "Stripe Connect onboardings dropped" or "domain
// rail margin trending down" without a custom dashboard. The publish
// is fire-and-forget; failures inside the publisher MUST NOT block
// the mutation.

func publishProvisioned(ctx context.Context, r ProvisionedResource) {
	learning.Publish(ctx, learning.OutcomeEvent{
		ID:        r.ID,
		TenantID:  r.TenantID,
		Kind:      learning.KindPatternObservation,
		Timestamp: r.CreatedAt,
		Attributes: map[string]any{
			"event":       "provisioning.resource.created",
			"resource_id": r.ID,
			"project_id":  r.ProjectID,
			"kind":        r.Kind,
			"external_id": r.ExternalID,
			"status":      r.Status,
		},
		Tags: map[string]string{"surface": "provisioning", "kind": r.Kind},
	})
}

func publishStatusChange(ctx context.Context, r ProvisionedResource) {
	learning.Publish(ctx, learning.OutcomeEvent{
		ID:        r.ID,
		TenantID:  r.TenantID,
		Kind:      learning.KindPatternObservation,
		Timestamp: r.UpdatedAt,
		Attributes: map[string]any{
			"event":       "provisioning.resource.status_changed",
			"resource_id": r.ID,
			"project_id":  r.ProjectID,
			"kind":        r.Kind,
			"status":      r.Status,
		},
		Tags: map[string]string{"surface": "provisioning", "kind": r.Kind, "status": r.Status},
	})
}

func publishRevenue(ctx context.Context, r ProvisionedResource, e RevenueEvent) {
	cost := e.IronflyerCutUSD
	margin := e.IronflyerCutUSD
	learning.Publish(ctx, learning.OutcomeEvent{
		ID:        e.ID,
		TenantID:  r.TenantID,
		Kind:      learning.KindPatternObservation,
		Timestamp: e.OccurredAt,
		// Ironflyer cut is pure revenue (the rail bears the underlying
		// processing cost), so cost is recorded as zero economically
		// but the cut itself is the margin number the cockpit graphs.
		CostUSD:   &cost,
		MarginUSD: &margin,
		Attributes: map[string]any{
			"event":             "provisioning.revenue.recorded",
			"resource_id":       r.ID,
			"project_id":        r.ProjectID,
			"kind":              r.Kind,
			"external_ref":      e.ExternalRef,
			"gross_amount_usd":  e.GrossAmountUSD.String(),
			"ironflyer_cut_usd": e.IronflyerCutUSD.String(),
		},
		Tags: map[string]string{"surface": "provisioning", "kind": r.Kind},
	})
}
