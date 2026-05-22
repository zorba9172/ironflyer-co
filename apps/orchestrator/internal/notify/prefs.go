package notify

import (
	"context"
	"sync"
)

// PrefsStore is the persistence contract for per-user NotificationRule. The
// MemoryPrefsStore implementation is process-local; a future Postgres-backed
// store can drop in without changing call sites.
type PrefsStore interface {
	Get(ctx context.Context, userID string) (NotificationRule, error)
	Set(ctx context.Context, rule NotificationRule) error
	// ListAll is used by the Engine on startup to know which users opted into
	// which channels — avoids hitting the store on every event.
	ListAll(ctx context.Context) ([]NotificationRule, error)
}

// MemoryPrefsStore is a thread-safe in-memory PrefsStore.
type MemoryPrefsStore struct {
	mu    sync.RWMutex
	rules map[string]NotificationRule
}

// NewMemoryPrefsStore returns an empty store.
func NewMemoryPrefsStore() *MemoryPrefsStore {
	return &MemoryPrefsStore{rules: make(map[string]NotificationRule)}
}

// Get returns the user's rule, or a synthesised DefaultRule if none exists.
// This makes the GET preferences handler total: every authenticated user
// gets a non-empty rule back even on first visit.
func (m *MemoryPrefsStore) Get(_ context.Context, userID string) (NotificationRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if r, ok := m.rules[userID]; ok {
		return r, nil
	}
	return DefaultRule(userID, ""), nil
}

// Set upserts the rule keyed by UserID.
func (m *MemoryPrefsStore) Set(_ context.Context, rule NotificationRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[rule.UserID] = rule
	return nil
}

// ListAll returns every persisted rule. The order is unspecified.
func (m *MemoryPrefsStore) ListAll(_ context.Context) ([]NotificationRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]NotificationRule, 0, len(m.rules))
	for _, r := range m.rules {
		out = append(out, r)
	}
	return out, nil
}
