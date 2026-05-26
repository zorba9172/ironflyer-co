package ledger

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// MemoryService is the in-process Service implementation. Used by
// dev / local mode and anywhere the orchestrator boots without
// Postgres (smoke runs, single-binary demos). All writes are
// append-only into a slice protected by a single RWMutex; the dataset
// is small enough that linear scans are perfectly fine for the
// in-memory backend's intended use.
type MemoryService struct {
	mu      sync.RWMutex
	entries []Entry
}

// NewMemoryService returns a ready-to-use in-memory ledger.
func NewMemoryService() *MemoryService {
	return &MemoryService{}
}

// Write validates, stamps, appends, and emits the metric. The mutex
// is held across the metric increment so the observed counter and the
// stored row stay consistent under concurrent writers.
//
// V22 idempotency: if e.OpKey is set and a prior entry already exists
// for that op_key, the prior entry is returned without appending a
// duplicate — mirrors the partial-unique-index dedupe the Postgres
// backend gets from migrations/00037.
func (m *MemoryService) Write(_ context.Context, e Entry) (Entry, error) {
	if err := validate(e); err != nil {
		return Entry{}, err
	}
	if e.OpKey != "" {
		m.mu.RLock()
		for _, prior := range m.entries {
			if prior.OpKey == e.OpKey {
				m.mu.RUnlock()
				return prior, nil
			}
		}
		m.mu.RUnlock()
	}
	e = stamp(e)
	m.mu.Lock()
	// Re-check under the write lock so two concurrent writers with the
	// same OpKey don't both observe "not found" and both append.
	if e.OpKey != "" {
		for _, prior := range m.entries {
			if prior.OpKey == e.OpKey {
				m.mu.Unlock()
				return prior, nil
			}
		}
	}
	m.entries = append(m.entries, e)
	m.mu.Unlock()
	observeWrite(e)
	return e, nil
}

// ListByTenant returns matching entries newest-first. The filter is
// applied in-place during the scan so the implementation is O(N) per
// query, which is acceptable for the dev backend.
func (m *MemoryService) ListByTenant(_ context.Context, tenantID uuid.UUID, f Filter) ([]Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Entry, 0, len(m.entries))
	for _, e := range m.entries {
		if e.TenantID != tenantID {
			continue
		}
		if !f.Since.IsZero() && e.CreatedAt.Before(f.Since) {
			continue
		}
		if !f.Until.IsZero() && e.CreatedAt.After(f.Until) {
			continue
		}
		if f.ExecutionID != nil {
			if e.ExecutionID == nil || *e.ExecutionID != *f.ExecutionID {
				continue
			}
		}
		if len(f.EntryTypes) > 0 && !containsType(f.EntryTypes, e.EntryType) {
			continue
		}
		out = append(out, e)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})

	// Offset first, then Limit — matches the SQL semantics in the
	// Postgres backend so both services paginate identically.
	if f.Offset > 0 {
		if f.Offset >= len(out) {
			return []Entry{}, nil
		}
		out = out[f.Offset:]
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// ListByExecution returns the per-execution timeline, oldest-first.
func (m *MemoryService) ListByExecution(_ context.Context, executionID uuid.UUID) ([]Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Entry, 0)
	for _, e := range m.entries {
		if e.ExecutionID != nil && *e.ExecutionID == executionID {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// SumByType walks every entry once. Empty types means "every type".
func (m *MemoryService) SumByType(_ context.Context, tenantID uuid.UUID, types []EntryType, since, until time.Time) (map[EntryType]decimal.Decimal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sums := make(map[EntryType]decimal.Decimal, len(AllEntryTypes))
	for _, e := range m.entries {
		if e.TenantID != tenantID {
			continue
		}
		if !since.IsZero() && e.CreatedAt.Before(since) {
			continue
		}
		if !until.IsZero() && e.CreatedAt.After(until) {
			continue
		}
		if len(types) > 0 && !containsType(types, e.EntryType) {
			continue
		}
		cur, ok := sums[e.EntryType]
		if !ok {
			cur = decimal.Zero
		}
		sums[e.EntryType] = cur.Add(e.AmountUSD)
	}
	return sums, nil
}

// TenantRollup runs the unfiltered scan and feeds it through Build.
// The memory backend can afford the redundant pass; Postgres can be
// smarter if it wants to.
func (m *MemoryService) TenantRollup(ctx context.Context, tenantID uuid.UUID, since, until time.Time) (Rollup, error) {
	entries, err := m.ListByTenant(ctx, tenantID, Filter{Since: since, Until: until})
	if err != nil {
		return Rollup{}, err
	}
	return Build(entries), nil
}

func containsType(types []EntryType, t EntryType) bool {
	for _, k := range types {
		if k == t {
			return true
		}
	}
	return false
}
