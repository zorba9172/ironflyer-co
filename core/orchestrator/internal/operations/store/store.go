// Package store is the project persistence layer. In-memory now; swap to
// Postgres later behind the same interface.
package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	List() []domain.Project
	// ListByOwner returns projects accessible to ownerID (owner-owned
	// plus public seeds) paginated with limit/offset. Order mirrors
	// List(): insertion order for the memory backend, scan order for
	// SurrealDB. A non-positive limit means "no cap"; offset is
	// clamped at 0. Public projects (OwnerID=="") remain visible to
	// every caller so demo content keeps working.
	ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]domain.Project, error)
	Get(id string) (domain.Project, error)
	// GetByIDs batch-loads projects by id. Returned map is keyed by
	// project id and omits unknown ids — the GraphQL dataloader maps
	// the absence onto ErrNotFound per key.
	GetByIDs(ctx context.Context, ids []string) (map[string]domain.Project, error)
	Create(p domain.Project) (domain.Project, error)
	Update(id string, fn func(*domain.Project)) (domain.Project, error)
	Delete(id string) error
}

type MemoryStore struct {
	mu    sync.RWMutex
	byID  map[string]domain.Project
	order []string

	hookMu      sync.RWMutex
	deleteHooks []func(projectID string)
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byID: make(map[string]domain.Project)}
}

// RegisterDeleteHook registers a callback invoked (outside the store mutex)
// after a project is successfully removed. Subsystems that cache per-project
// state — notify subscriptions, webhook fan-out, etc. — use this to release
// resources without each delete site having to know about them.
func (m *MemoryStore) RegisterDeleteHook(fn func(projectID string)) {
	if fn == nil {
		return
	}
	m.hookMu.Lock()
	m.deleteHooks = append(m.deleteHooks, fn)
	m.hookMu.Unlock()
}

// fireDeleteHooks invokes every registered hook for id. Called after the
// store mutex has been released so hooks cannot deadlock against further
// store reads triggered from inside the callback.
func (m *MemoryStore) fireDeleteHooks(id string) {
	m.hookMu.RLock()
	hooks := append([]func(string){}, m.deleteHooks...)
	m.hookMu.RUnlock()
	for _, fn := range hooks {
		fn(id)
	}
}

func (m *MemoryStore) Seed() {
	now := time.Now().UTC()
	p := domain.Project{
		ID:          "demo",
		Name:        "Demo SaaS Workspace",
		Description: "Prompt-to-product execution workspace.",
		Status:      "ready",
		Spec: domain.ProductSpec{
			Idea: "A lightweight invoicing tool for freelancers.",
			Stack: domain.StackDecision{
				Frontend: "Next.js + MUI",
				Backend:  "Go stdlib",
				Storage:  "Postgres",
				Auth:     "JWT",
			},
		},
		Files: []domain.FileNode{
			{Path: "clients/web/app/page.tsx", Type: "file"},
			{Path: "apps/api/main.go", Type: "file"},
			{Path: "README.md", Type: "file"},
		},
		Gates:     emptyGates(now),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, _ = m.Create(p)
}

func emptyGates(t time.Time) map[domain.GateName]domain.GateState {
	out := make(map[domain.GateName]domain.GateState, len(domain.AllGates()))
	for _, g := range domain.AllGates() {
		out[g] = domain.GateState{Name: g, Status: domain.GateStatusPending, UpdatedAt: t}
	}
	return out
}

func (m *MemoryStore) List() []domain.Project {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Project, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.byID[id])
	}
	return out
}

// ListByOwner walks the insertion-ordered project ring and returns
// rows accessible to ownerID (owner-owned plus public seeds),
// paginated by limit/offset. Kept in-place under the read lock for
// consistency with List().
func (m *MemoryStore) ListByOwner(_ context.Context, ownerID string, limit, offset int) ([]domain.Project, error) {
	if offset < 0 {
		offset = 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Project, 0)
	skipped := 0
	for _, id := range m.order {
		p, ok := m.byID[id]
		if !ok {
			continue
		}
		if !p.IsAccessibleBy(ownerID) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, p)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemoryStore) Get(id string) (domain.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.byID[id]
	if !ok {
		return domain.Project{}, ErrNotFound
	}
	return p, nil
}

// GetByIDs batch-loads projects under a single read lock. Missing ids
// are silently skipped so the dataloader's batch function can decide
// how to surface a 404 per key.
func (m *MemoryStore) GetByIDs(_ context.Context, ids []string) (map[string]domain.Project, error) {
	out := make(map[string]domain.Project, len(ids))
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range ids {
		if p, ok := m.byID[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

func (m *MemoryStore) Create(p domain.Project) (domain.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.ID == "" {
		return domain.Project{}, errors.New("project id required")
	}
	if _, exists := m.byID[p.ID]; exists {
		return domain.Project{}, errors.New("project already exists")
	}
	if p.Gates == nil {
		p.Gates = emptyGates(time.Now().UTC())
	}
	m.byID[p.ID] = p
	m.order = append(m.order, p.ID)
	return p, nil
}

func (m *MemoryStore) Update(id string, fn func(*domain.Project)) (domain.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.byID[id]
	if !ok {
		return domain.Project{}, ErrNotFound
	}
	fn(&p)
	p.UpdatedAt = time.Now().UTC()
	m.byID[id] = p
	return p, nil
}

// Delete removes a project from the in-memory store. Idempotent: missing IDs
// return ErrNotFound so callers can decide whether to treat that as fatal.
// After a successful removal any RegisterDeleteHook callbacks fire outside
// the store mutex so subsystems can release per-project caches.
func (m *MemoryStore) Delete(id string) error {
	m.mu.Lock()
	if _, ok := m.byID[id]; !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	delete(m.byID, id)
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.mu.Unlock()
	m.fireDeleteHooks(id)
	return nil
}
