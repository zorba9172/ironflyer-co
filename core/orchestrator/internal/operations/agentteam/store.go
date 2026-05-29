package agentteam

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("agentteam: not found")

// Store persists operator-defined agents and crews, owner-scoped. In-memory
// now; a Postgres backend slots in behind the same interface later.
type Store interface {
	ListAgents(ctx context.Context, ownerID string) ([]CustomAgent, error)
	SaveAgent(ctx context.Context, a CustomAgent) (CustomAgent, error)
	DeleteAgent(ctx context.Context, ownerID, id string) error

	ListCrews(ctx context.Context, ownerID string) ([]Crew, error)
	GetCrew(ctx context.Context, ownerID, id string) (Crew, error)
	SaveCrew(ctx context.Context, c Crew) (Crew, error)
	DeleteCrew(ctx context.Context, ownerID, id string) error
}

// IDFunc generates ids for new records. Injected so the package stays free of a
// direct dependency on a specific id scheme and remains deterministic in tests.
type IDFunc func(prefix string) string

// MemoryStore is the in-process implementation. Safe for concurrent use.
type MemoryStore struct {
	mu     sync.RWMutex
	agents map[string]map[string]CustomAgent // ownerID -> id -> agent
	crews  map[string]map[string]Crew        // ownerID -> id -> crew
	now    func() time.Time
	nextID IDFunc
}

func NewMemoryStore(nextID IDFunc) *MemoryStore {
	if nextID == nil {
		var n int64
		var mu sync.Mutex
		nextID = func(prefix string) string {
			mu.Lock()
			n++
			id := n
			mu.Unlock()
			return prefix + "_" + itoa(id)
		}
	}
	return &MemoryStore{
		agents: map[string]map[string]CustomAgent{},
		crews:  map[string]map[string]Crew{},
		now:    func() time.Time { return time.Now().UTC() },
		nextID: nextID,
	}
}

func (m *MemoryStore) ListAgents(_ context.Context, ownerID string) ([]CustomAgent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]CustomAgent, 0, len(m.agents[ownerID]))
	for _, a := range m.agents[ownerID] {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (m *MemoryStore) SaveAgent(_ context.Context, a CustomAgent) (CustomAgent, error) {
	if a.OwnerID == "" {
		return CustomAgent{}, errors.New("agentteam: ownerID required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	bucket := m.agents[a.OwnerID]
	if bucket == nil {
		bucket = map[string]CustomAgent{}
		m.agents[a.OwnerID] = bucket
	}
	if a.ID == "" {
		a.ID = m.nextID("agent")
		a.CreatedAt = now
	} else if prev, ok := bucket[a.ID]; ok {
		a.CreatedAt = prev.CreatedAt
	} else {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	bucket[a.ID] = a
	return a, nil
}

func (m *MemoryStore) DeleteAgent(_ context.Context, ownerID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	bucket := m.agents[ownerID]
	if bucket == nil {
		return ErrNotFound
	}
	if _, ok := bucket[id]; !ok {
		return ErrNotFound
	}
	delete(bucket, id)
	// Drop the deleted agent from any of this owner's crews.
	for _, c := range m.crews[ownerID] {
		c.MemberIDs = without(c.MemberIDs, id)
		if c.ManagerID == id {
			c.ManagerID = ""
		}
		m.crews[ownerID][c.ID] = c
	}
	return nil
}

func (m *MemoryStore) ListCrews(_ context.Context, ownerID string) ([]Crew, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Crew, 0, len(m.crews[ownerID]))
	for _, c := range m.crews[ownerID] {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (m *MemoryStore) GetCrew(_ context.Context, ownerID, id string) (Crew, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.crews[ownerID][id]; ok {
		return c, nil
	}
	return Crew{}, ErrNotFound
}

func (m *MemoryStore) SaveCrew(_ context.Context, c Crew) (Crew, error) {
	if c.OwnerID == "" {
		return Crew{}, errors.New("agentteam: ownerID required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	bucket := m.crews[c.OwnerID]
	if bucket == nil {
		bucket = map[string]Crew{}
		m.crews[c.OwnerID] = bucket
	}
	if c.ID == "" {
		c.ID = m.nextID("crew")
		c.CreatedAt = now
	} else if prev, ok := bucket[c.ID]; ok {
		c.CreatedAt = prev.CreatedAt
	} else {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	bucket[c.ID] = c
	return c, nil
}

func (m *MemoryStore) DeleteCrew(_ context.Context, ownerID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	bucket := m.crews[ownerID]
	if bucket == nil {
		return ErrNotFound
	}
	if _, ok := bucket[id]; !ok {
		return ErrNotFound
	}
	delete(bucket, id)
	return nil
}

func without(xs []string, v string) []string {
	out := xs[:0:0]
	for _, x := range xs {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
