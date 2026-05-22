// Package store is the project persistence layer. In-memory now; swap to
// Postgres later behind the same interface.
package store

import (
	"errors"
	"sync"
	"time"

	"ironflyer/apps/orchestrator/internal/domain"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	List() []domain.Project
	Get(id string) (domain.Project, error)
	Create(p domain.Project) (domain.Project, error)
	Update(id string, fn func(*domain.Project)) (domain.Project, error)
	Delete(id string) error
}

type MemoryStore struct {
	mu    sync.RWMutex
	byID  map[string]domain.Project
	order []string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byID: make(map[string]domain.Project)}
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
			{Path: "apps/web/app/page.tsx", Type: "file"},
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

func (m *MemoryStore) Get(id string) (domain.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.byID[id]
	if !ok {
		return domain.Project{}, ErrNotFound
	}
	return p, nil
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
func (m *MemoryStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[id]; !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return nil
}
