package bus

import (
	"context"
	"sync"
)

// MemoryBus is the single-pod backend. Subscribers are kept in a
// map[topic][]chan; Publish iterates the slice and non-blocking sends
// each payload. Used when no Redis is configured — the orchestrator
// runs as a single pod and cross-pod delivery is a no-op.
//
// Although the Multiplexer already maintains a local subscriber map in
// front of any backend, MemoryBus exists so the same wiring code works
// both in dev (no Redis) and in production (Redis). When MemoryBus is
// the backend, the Multiplexer effectively delivers everything once
// through its own local map and the MemoryBus is a quiet pass-through —
// we keep it consistent rather than special-casing nil at every layer.
type MemoryBus struct {
	mu     sync.RWMutex
	subs   map[string]map[chan []byte]struct{}
	closed bool
}

// NewMemoryBus builds an empty bus ready for use.
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		subs: map[string]map[chan []byte]struct{}{},
	}
}

// Publish copies payload to every subscriber of topic. Non-blocking per
// subscriber — a slow consumer drops messages rather than stalling the
// producer.
func (m *MemoryBus) Publish(_ context.Context, topic string, payload []byte) error {
	m.mu.RLock()
	bucket := m.subs[topic]
	if len(bucket) == 0 {
		m.mu.RUnlock()
		return nil
	}
	// Snapshot subscribers so we don't hold the lock across sends.
	chans := make([]chan []byte, 0, len(bucket))
	for ch := range bucket {
		chans = append(chans, ch)
	}
	m.mu.RUnlock()
	for _, ch := range chans {
		_ = safeSend(ch, payload)
	}
	return nil
}

// Subscribe attaches a fresh subscriber and returns the receive channel
// plus an idempotent cancel.
func (m *MemoryBus) Subscribe(_ context.Context, topic string) (<-chan []byte, func(), error) {
	ch := make(chan []byte, SubBuffer)
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		close(ch)
		return ch, func() {}, ErrClosed
	}
	bucket, ok := m.subs[topic]
	if !ok {
		bucket = map[chan []byte]struct{}{}
		m.subs[topic] = bucket
	}
	bucket[ch] = struct{}{}
	m.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			b, ok := m.subs[topic]
			if !ok {
				return
			}
			if _, ok := b[ch]; !ok {
				return
			}
			delete(b, ch)
			close(ch)
			if len(b) == 0 {
				delete(m.subs, topic)
			}
		})
	}
	return ch, cancel, nil
}

// Close releases every subscriber. Idempotent.
func (m *MemoryBus) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	for topic, bucket := range m.subs {
		for ch := range bucket {
			close(ch)
		}
		delete(m.subs, topic)
	}
	return nil
}
