package integrations

import (
	"context"
	"sync"
	"time"
)

// MemoryTokenStore is for dev / tests. Tokens lost on restart.
type MemoryTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]Token // key = userID + "|" + kind
}

func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{tokens: make(map[string]Token)}
}

func key(userID string, kind Kind) string { return userID + "|" + string(kind) }

func (m *MemoryTokenStore) Put(_ context.Context, t Token) (Token, error) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	t.UpdatedAt = time.Now().UTC()
	m.mu.Lock()
	m.tokens[key(t.UserID, t.Kind)] = t
	m.mu.Unlock()
	return t, nil
}

func (m *MemoryTokenStore) Get(_ context.Context, userID string, kind Kind) (Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tokens[key(userID, kind)]
	if !ok {
		return Token{}, ErrNotFound
	}
	return t, nil
}

func (m *MemoryTokenStore) Delete(_ context.Context, userID string, kind Kind) error {
	m.mu.Lock()
	delete(m.tokens, key(userID, kind))
	m.mu.Unlock()
	return nil
}

func (m *MemoryTokenStore) ListByUser(_ context.Context, userID string) ([]Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Token
	for k, t := range m.tokens {
		if t.UserID == userID {
			out = append(out, t)
		}
		_ = k
	}
	return out, nil
}

func (m *MemoryTokenStore) FindByExternal(_ context.Context, kind Kind, externalID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tokens {
		if t.Kind == kind && t.ExternalID == externalID && externalID != "" {
			return t.UserID, nil
		}
	}
	return "", ErrNotFound
}

var _ TokenStore = (*MemoryTokenStore)(nil)
