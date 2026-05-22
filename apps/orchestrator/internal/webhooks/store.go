package webhooks

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned by Store implementations when an ID does not
// resolve, or when a delete is attempted by a non-owner.
var ErrNotFound = errors.New("webhook subscription not found")

// ErrNotImplemented is returned by stub backends (e.g. PostgresStore) that
// were declared for symmetry but are not yet wired. Callers can flag the
// configuration at startup so we never silently drop user signups.
var ErrNotImplemented = errors.New("webhook store backend not yet implemented")

// Store is the persistence contract for webhook subscriptions. Implementations
// must be safe for concurrent use.
type Store interface {
	Create(ctx context.Context, s Subscription) (Subscription, error)
	List(ctx context.Context, userID string) ([]Subscription, error)
	ListMatching(ctx context.Context, userID, projectID string) ([]Subscription, error)
	Get(ctx context.Context, id string) (Subscription, error)
	Delete(ctx context.Context, userID, id string) error
	UpdateStats(ctx context.Context, id string, lastSentAt time.Time, failureCount int, disabled bool) error
}

// MemoryStore keeps subscriptions in process memory. Suitable for dev and
// single-node deployments — swap to PostgresStore for HA / persistence.
type MemoryStore struct {
	mu   sync.RWMutex
	byID map[string]Subscription
}

// NewMemoryStore returns an empty in-memory subscription store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byID: make(map[string]Subscription)}
}

// Create assigns an ID + CreatedAt if missing, then stores.
func (m *MemoryStore) Create(_ context.Context, s Subscription) (Subscription, error) {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[s.ID] = s
	return s, nil
}

// List returns all subscriptions owned by userID.
func (m *MemoryStore) List(_ context.Context, userID string) ([]Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Subscription, 0)
	for _, s := range m.byID {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

// ListMatching returns subscriptions owned by userID that target either the
// given projectID or "all projects" (empty ProjectID).
func (m *MemoryStore) ListMatching(_ context.Context, userID, projectID string) ([]Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Subscription, 0)
	for _, s := range m.byID {
		if s.UserID != userID || s.Disabled {
			continue
		}
		if s.ProjectID != "" && s.ProjectID != projectID {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// Get returns a single subscription by id.
func (m *MemoryStore) Get(_ context.Context, id string) (Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.byID[id]
	if !ok {
		return Subscription{}, ErrNotFound
	}
	return s, nil
}

// Delete removes a subscription; only the owner may delete.
func (m *MemoryStore) Delete(_ context.Context, userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byID[id]
	if !ok || s.UserID != userID {
		return ErrNotFound
	}
	delete(m.byID, id)
	return nil
}

// UpdateStats is called by the dispatcher after each delivery attempt so
// the user can see when a webhook was last fired and whether it is healthy.
func (m *MemoryStore) UpdateStats(_ context.Context, id string, lastSentAt time.Time, failureCount int, disabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	s.LastSentAt = lastSentAt
	s.FailureCount = failureCount
	s.Disabled = disabled
	m.byID[id] = s
	return nil
}

// PostgresStore is a stub backend reserved for the Postgres rollout. Every
// method returns ErrNotImplemented so the configuration layer can detect a
// misconfiguration loudly at startup instead of silently dropping deliveries.
//
// TODO(persistence): implement against the same schema as auth + integrations
// (users → user_id FK, projects → project_id FK, one row per subscription).
type PostgresStore struct{}

// NewPostgresStore returns the stub. Wire-up is deferred until the rest of
// the Postgres backend matures.
func NewPostgresStore() *PostgresStore { return &PostgresStore{} }

func (PostgresStore) Create(context.Context, Subscription) (Subscription, error) {
	return Subscription{}, ErrNotImplemented
}
func (PostgresStore) List(context.Context, string) ([]Subscription, error) {
	return nil, ErrNotImplemented
}
func (PostgresStore) ListMatching(context.Context, string, string) ([]Subscription, error) {
	return nil, ErrNotImplemented
}
func (PostgresStore) Get(context.Context, string) (Subscription, error) {
	return Subscription{}, ErrNotImplemented
}
func (PostgresStore) Delete(context.Context, string, string) error { return ErrNotImplemented }
func (PostgresStore) UpdateStats(context.Context, string, time.Time, int, bool) error {
	return ErrNotImplemented
}
