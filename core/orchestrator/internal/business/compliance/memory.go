package compliance

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryBackend is the in-process Backend used for dev
// (`IRONFLYER_DB_DRIVER=memory`) and as a clean substrate for
// resolver wiring before Postgres is provisioned. Parity with
// PostgresBackend: same idempotency contract, same tenant-isolation
// behaviour at the row level (callers MUST still enforce ownership
// before invoking).
type MemoryBackend struct {
	mu sync.Mutex

	enrollments map[string]EnrolledProject // id → row
	byTuple     map[string]string          // tenant|project|framework → enrollment id
	results     map[string][]ControlResult // enrollment id → ordered results
	charges     map[string]Charge          // idempotency key → row
}

// NewMemoryBackend constructs an empty backend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		enrollments: map[string]EnrolledProject{},
		byTuple:     map[string]string{},
		results:     map[string][]ControlResult{},
		charges:     map[string]Charge{},
	}
}

func tupleKey(tenant, projectID, framework string) string {
	return tenant + "|" + projectID + "|" + framework
}

func (m *MemoryBackend) Enroll(_ context.Context, row EnrolledProject) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := tupleKey(row.TenantID, row.ProjectID, row.FrameworkKey)
	if _, ok := m.byTuple[k]; ok {
		return ErrAlreadyEnrolled
	}
	m.enrollments[row.ID] = row
	m.byTuple[k] = row.ID
	return nil
}

func (m *MemoryBackend) GetEnrollment(_ context.Context, id string) (EnrolledProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.enrollments[id]
	if !ok {
		return EnrolledProject{}, ErrNotFound
	}
	return row, nil
}

func (m *MemoryBackend) GetEnrollmentByTuple(_ context.Context, tenant, projectID, framework string) (EnrolledProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byTuple[tupleKey(tenant, projectID, framework)]
	if !ok {
		return EnrolledProject{}, ErrNotFound
	}
	return m.enrollments[id], nil
}

func (m *MemoryBackend) ListEnrollments(_ context.Context, tenant, projectID string) ([]EnrolledProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]EnrolledProject, 0, len(m.enrollments))
	for _, row := range m.enrollments {
		if row.TenantID != tenant {
			continue
		}
		if projectID != "" && row.ProjectID != projectID {
			continue
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EnrolledAt.After(out[j].EnrolledAt) })
	return out, nil
}

func (m *MemoryBackend) ListAllEnrollments(_ context.Context) ([]EnrolledProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]EnrolledProject, 0, len(m.enrollments))
	for _, row := range m.enrollments {
		out = append(out, row)
	}
	return out, nil
}

func (m *MemoryBackend) MarkEvaluated(_ context.Context, id string, at time.Time, verdict VerdictKind) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.enrollments[id]
	if !ok {
		return ErrNotFound
	}
	row.LastEvaluatedAt = &at
	row.LastVerdict = verdict
	m.enrollments[id] = row
	return nil
}

func (m *MemoryBackend) DeleteEnrollment(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.enrollments[id]
	if !ok {
		return nil
	}
	delete(m.enrollments, id)
	delete(m.byTuple, tupleKey(row.TenantID, row.ProjectID, row.FrameworkKey))
	delete(m.results, id)
	return nil
}

func (m *MemoryBackend) SaveResults(_ context.Context, enrollmentID string, results []ControlResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	dup := make([]ControlResult, len(results))
	copy(dup, results)
	m.results[enrollmentID] = dup
	return nil
}

func (m *MemoryBackend) ListResults(_ context.Context, enrollmentID string) ([]ControlResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	src := m.results[enrollmentID]
	out := make([]ControlResult, len(src))
	copy(out, src)
	return out, nil
}

func (m *MemoryBackend) RecordCharge(_ context.Context, charge Charge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.charges[charge.IdempotencyKey]; ok {
		return ErrAlreadyEnrolled
	}
	m.charges[charge.IdempotencyKey] = charge
	return nil
}

func (m *MemoryBackend) HasCharge(_ context.Context, idempotencyKey string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.charges[idempotencyKey]
	return ok, nil
}

var _ Backend = (*MemoryBackend)(nil)
