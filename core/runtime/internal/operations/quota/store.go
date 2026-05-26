package quota

import (
	"context"
	"sync"

	"github.com/shopspring/decimal"
)

// Usage is the live per-tenant counter snapshot. Stored by the
// Enforcer's backing store; returned via UsageSnapshot for dashboards
// and admission checks.
type Usage struct {
	TenantID            string
	LiveSandboxes       int
	LiveCPU             int
	LiveMemMB           int
	LiveExecutions      int
	SpendTodayUSD       decimal.Decimal
	OutstandingLeases   map[string]Lease // keyed by leaseID = workspaceID
}

// Lease records a single admitted sandbox so Release() can decrement
// the right counters. Stored alongside the tenant's Usage row.
type Lease struct {
	TenantID     string
	ExecutionID  string
	WorkspaceID  string
	CPU          int
	MemMB        int
	RuntimeClass string
	EstUSD       decimal.Decimal
}

// Store is the persistence boundary for the Enforcer. Memory and
// Postgres implementations live below. The store keeps live counters
// only — historical metering is the orchestrator ledger's job.
type Store interface {
	// Get returns the current Usage for the tenant. Zero-value Usage
	// (with TenantID set) is returned for a tenant that has never
	// been seen.
	Get(ctx context.Context, tenantID string) (Usage, error)
	// Hold atomically charges the lease against the tenant; returns
	// ErrQuotaExceeded-equivalent typed *Error if the addition would
	// breach the supplied TenantQuota.
	Hold(ctx context.Context, tenantID string, q TenantQuota, lease Lease) error
	// Release reverses a Hold by leaseID (workspaceID).
	Release(ctx context.Context, tenantID, executionID, workspaceID string) error
}

// MemoryStore is the in-process Store; used by the mock driver path
// and as a fallback when Postgres is not configured.
type MemoryStore struct {
	mu sync.Mutex
	// per-tenant Usage map. Read paths copy out by value.
	rows map[string]Usage
}

// NewMemoryStore builds an empty in-memory Store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{rows: make(map[string]Usage)}
}

// Get implements Store.
func (s *MemoryStore) Get(_ context.Context, tenantID string) (Usage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.rows[tenantID]; ok {
		// Defensive copy of the lease map.
		out := u
		if u.OutstandingLeases != nil {
			out.OutstandingLeases = make(map[string]Lease, len(u.OutstandingLeases))
			for k, v := range u.OutstandingLeases {
				out.OutstandingLeases[k] = v
			}
		}
		return out, nil
	}
	return Usage{TenantID: tenantID, OutstandingLeases: map[string]Lease{}}, nil
}

// Hold implements Store.
func (s *MemoryStore) Hold(_ context.Context, tenantID string, q TenantQuota, lease Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.rows[tenantID]
	if !ok {
		u = Usage{TenantID: tenantID, OutstandingLeases: map[string]Lease{}}
	}
	if u.OutstandingLeases == nil {
		u.OutstandingLeases = map[string]Lease{}
	}
	if _, dup := u.OutstandingLeases[lease.WorkspaceID]; dup {
		// Idempotent: re-holding the same lease succeeds without
		// double-charging.
		return nil
	}
	if q.MaxConcurrentSandboxes > 0 && u.LiveSandboxes+1 > q.MaxConcurrentSandboxes {
		return New(ReasonQuotaExceeded, "concurrent sandbox limit")
	}
	if q.MaxConcurrentCPU > 0 && u.LiveCPU+lease.CPU > q.MaxConcurrentCPU {
		return New(ReasonQuotaExceeded, "concurrent cpu limit")
	}
	if q.MaxConcurrentMemMB > 0 && u.LiveMemMB+lease.MemMB > q.MaxConcurrentMemMB {
		return New(ReasonQuotaExceeded, "concurrent memory limit")
	}
	if !q.MaxSpendUSDPerDay.IsZero() {
		projected := u.SpendTodayUSD.Add(lease.EstUSD)
		if projected.GreaterThan(q.MaxSpendUSDPerDay) {
			return New(ReasonPauseForBudget, "daily spend ceiling")
		}
	}
	u.LiveSandboxes++
	u.LiveCPU += lease.CPU
	u.LiveMemMB += lease.MemMB
	u.LiveExecutions++
	u.SpendTodayUSD = u.SpendTodayUSD.Add(lease.EstUSD)
	u.OutstandingLeases[lease.WorkspaceID] = lease
	s.rows[tenantID] = u
	return nil
}

// Release implements Store. Missing leases are no-ops so the caller
// can safely retry Release on partial-failure paths.
func (s *MemoryStore) Release(_ context.Context, tenantID, _ /*executionID*/ , workspaceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.rows[tenantID]
	if !ok {
		return nil
	}
	lease, ok := u.OutstandingLeases[workspaceID]
	if !ok {
		return nil
	}
	delete(u.OutstandingLeases, workspaceID)
	if u.LiveSandboxes > 0 {
		u.LiveSandboxes--
	}
	u.LiveCPU -= lease.CPU
	if u.LiveCPU < 0 {
		u.LiveCPU = 0
	}
	u.LiveMemMB -= lease.MemMB
	if u.LiveMemMB < 0 {
		u.LiveMemMB = 0
	}
	if u.LiveExecutions > 0 {
		u.LiveExecutions--
	}
	// SpendTodayUSD is intentionally not reversed — Release() is
	// lifecycle-only; the daily counter resets by calendar day in
	// the future Postgres store.
	s.rows[tenantID] = u
	return nil
}

// _ assertion: MemoryStore satisfies Store.
var _ Store = (*MemoryStore)(nil)

// zeroUSD is a small helper so callers do not have to import the
// decimal package just to construct a zero.
func zeroUSD() decimal.Decimal { return decimal.Decimal{} }
