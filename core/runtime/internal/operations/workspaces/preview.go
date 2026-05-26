// preview.go — durable-ish preview binding state for workspaces.
//
// The runtime keeps the canonical binding inside the sandbox driver
// (so the reverse-proxy can find the container IP without a registry
// round-trip), but the HTTP layer also wants a thin cache it can
// serve from without dialling the driver. This package surfaces a
// Memory-only state for v1; when persistence is needed we can promote
// the same interface to Postgres without touching call sites.
package workspaces

import (
	"sync"
	"time"
)

// PreviewBinding mirrors sandbox.PreviewBinding without pulling the
// sandbox import. Stays narrow to keep the package boundary clean.
type PreviewBinding struct {
	WorkspaceID  string    `json:"workspaceId"`
	InternalPort int       `json:"internalPort"`
	ExternalPort int       `json:"externalPort"`
	URL          string    `json:"url"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// PreviewStore caches per-workspace preview bindings. Memory-only for
// v1; future PostgresPreviewStore would satisfy the same interface.
type PreviewStore interface {
	Put(binding PreviewBinding)
	Get(workspaceID string) (PreviewBinding, bool)
	Drop(workspaceID string)
}

// MemoryPreviewStore is the single-pod in-process implementation.
type MemoryPreviewStore struct {
	mu   sync.RWMutex
	rows map[string]PreviewBinding
}

func NewMemoryPreviewStore() *MemoryPreviewStore {
	return &MemoryPreviewStore{rows: make(map[string]PreviewBinding)}
}

func (m *MemoryPreviewStore) Put(b PreviewBinding) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[b.WorkspaceID] = b
}

func (m *MemoryPreviewStore) Get(id string) (PreviewBinding, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.rows[id]
	if !ok {
		return PreviewBinding{}, false
	}
	if !b.ExpiresAt.IsZero() && time.Now().After(b.ExpiresAt) {
		return PreviewBinding{}, false
	}
	return b, true
}

func (m *MemoryPreviewStore) Drop(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, id)
}

var _ PreviewStore = (*MemoryPreviewStore)(nil)
