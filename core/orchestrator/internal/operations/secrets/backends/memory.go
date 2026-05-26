package backends

import (
	"context"
	"fmt"
	"sync"

	"ironflyer/core/orchestrator/internal/operations/secrets"
)

// Memory is the in-process backend used by dev and by code paths that
// boot without a managed secret store (smoke runs, single-binary
// demos, ephemeral CI). Operator-supplied via IRONFLYER_SECRETS_BACKEND=memory.
//
// The keying matches the env backend: BackendRef takes precedence over
// Name so the two backends are interchangeable during testing.
type Memory struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemory returns an empty Memory backend.
func NewMemory() *Memory { return &Memory{data: make(map[string][]byte)} }

func (m *Memory) Name() secrets.Backend { return secrets.BackendMemory }

// Put registers (or overwrites) a value. The backend keeps its own
// copy so callers can zero their buffer.
func (m *Memory) Put(key string, value []byte) {
	if key == "" {
		return
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	m.mu.Lock()
	m.data[key] = cp
	m.mu.Unlock()
}

// Delete drops a key. Used by Rotate paths in tests.
func (m *Memory) Delete(key string) {
	m.mu.Lock()
	if v, ok := m.data[key]; ok {
		for i := range v {
			v[i] = 0
		}
		delete(m.data, key)
	}
	m.mu.Unlock()
}

func (m *Memory) Load(_ context.Context, ref secrets.SecretRef) ([]byte, error) {
	key := ref.BackendRef
	if key == "" {
		key = ref.Name
	}
	m.mu.RLock()
	v, ok := m.data[key]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: memory %q", secrets.ErrSecretNotFound, key)
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}
