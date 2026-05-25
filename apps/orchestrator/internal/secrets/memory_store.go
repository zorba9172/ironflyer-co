package secrets

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore is the in-process Store implementation. Used in dev and
// in single-binary smoke runs where Postgres is not configured. The
// data shape mirrors migration 00032 exactly so swapping to Postgres
// is a wiring change, not a code change.
type MemoryStore struct {
	mu       sync.RWMutex
	refs     map[string]SecretRef          // id -> ref
	byKey    map[string]string             // tenant|project|name -> id
	releases map[string][]ReleaseRecord    // refID -> rows
	seq      int64
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		refs:     make(map[string]SecretRef),
		byKey:    make(map[string]string),
		releases: make(map[string][]ReleaseRecord),
	}
}

func uniqueKey(tenantID, projectID, name string) string {
	return strings.Join([]string{tenantID, projectID, name}, "|")
}

// CreateRef inserts a SecretRef. Mirrors the (tenant_id, project_id,
// name) UNIQUE constraint from migration 00032.
func (m *MemoryStore) CreateRef(_ context.Context, ref SecretRef) (SecretRef, error) {
	if ref.TenantID == "" || ref.Name == "" {
		return SecretRef{}, fmt.Errorf("secrets: tenant_id and name are required")
	}
	if !validReleaseClass(ref.ReleaseClass) {
		return SecretRef{}, ErrInvalidReleaseClass
	}
	key := uniqueKey(ref.TenantID, ref.ProjectID, ref.Name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byKey[key]; exists {
		return SecretRef{}, fmt.Errorf("secrets: ref already exists for %s/%s/%s", ref.TenantID, ref.ProjectID, ref.Name)
	}
	if ref.ID == "" {
		ref.ID = uuid.NewString()
	}
	if ref.Version <= 0 {
		ref.Version = 1
	}
	if ref.CreatedAt.IsZero() {
		ref.CreatedAt = time.Now().UTC()
	}
	m.refs[ref.ID] = ref
	m.byKey[key] = ref.ID
	return ref, nil
}

// GetRef fetches a SecretRef by id.
func (m *MemoryStore) GetRef(_ context.Context, id string) (SecretRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.refs[id]
	if !ok {
		return SecretRef{}, ErrSecretNotFound
	}
	return r, nil
}

// LookupRef finds a SecretRef by its tenant/project/name tuple.
func (m *MemoryStore) LookupRef(_ context.Context, tenantID, projectID, name string) (SecretRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.byKey[uniqueKey(tenantID, projectID, name)]
	if !ok {
		return SecretRef{}, ErrSecretNotFound
	}
	return m.refs[id], nil
}

// ListRefs returns refs for a tenant, newest-first.
func (m *MemoryStore) ListRefs(_ context.Context, tenantID string) ([]SecretRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SecretRef, 0, len(m.refs))
	for _, r := range m.refs {
		if r.TenantID == tenantID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// UpdateRef applies fn to a copy of the stored ref, then writes the
// result back. Used by Rotate and by Bind.
func (m *MemoryStore) UpdateRef(_ context.Context, id string, fn func(*SecretRef)) (SecretRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.refs[id]
	if !ok {
		return SecretRef{}, ErrSecretNotFound
	}
	fn(&r)
	m.refs[id] = r
	return r, nil
}

// RecordRelease appends a release row. The store enforces non-nil
// fields that the migration's NOT NULL columns enforce in Postgres.
func (m *MemoryStore) RecordRelease(_ context.Context, rel ReleaseRecord) (ReleaseRecord, error) {
	if rel.SecretRefID == "" || rel.TenantID == "" {
		return ReleaseRecord{}, fmt.Errorf("secrets: release requires secret_ref_id and tenant_id")
	}
	if rel.PolicyDecisionID == "" {
		return ReleaseRecord{}, ErrPolicyDecisionRequired
	}
	if !validReleaseTo(rel.ReleasedTo) {
		return ReleaseRecord{}, ErrInvalidReleaseTo
	}
	if rel.ReleasedAt.IsZero() {
		rel.ReleasedAt = time.Now().UTC()
	}
	if rel.RedactionProof == "" {
		rel.RedactionProof = "sha256:redacted"
	}
	m.mu.Lock()
	m.seq++
	rel.ID = m.seq
	m.releases[rel.SecretRefID] = append(m.releases[rel.SecretRefID], rel)
	// Stamp last_released_at on the parent ref so the dashboards can
	// show "this secret was used N seconds ago" without a join.
	if r, ok := m.refs[rel.SecretRefID]; ok {
		t := rel.ReleasedAt
		r.LastReleasedAt = &t
		m.refs[rel.SecretRefID] = r
	}
	m.mu.Unlock()
	return rel, nil
}

// ListReleases returns release rows for a ref, newest-first.
func (m *MemoryStore) ListReleases(_ context.Context, refID string) ([]ReleaseRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rows := m.releases[refID]
	out := make([]ReleaseRecord, len(rows))
	copy(out, rows)
	sort.Slice(out, func(i, j int) bool { return out[i].ReleasedAt.After(out[j].ReleasedAt) })
	return out, nil
}
