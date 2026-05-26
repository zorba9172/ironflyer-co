package notify

import (
	"sync"
	"sync/atomic"
)

// SubscriptionHub is an in-process pub/sub keyed by userID. Each
// Subscribe call returns a buffered channel + an unsubscribe func; the
// Worker calls Publish after every successful in-app delivery. Slow
// subscribers drop on overflow (non-blocking publish) so a wedged
// client cannot stall the worker.
type SubscriptionHub struct {
	mu     sync.RWMutex
	nextID uint64
	subs   map[string]map[uint64]chan Notification
}

// NewSubscriptionHub returns an empty hub.
func NewSubscriptionHub() *SubscriptionHub {
	return &SubscriptionHub{subs: make(map[string]map[uint64]chan Notification)}
}

// Subscribe registers a buffered channel for userID. The caller must
// invoke the returned unsubscribe func on disconnect.
func (h *SubscriptionHub) Subscribe(userID string) (<-chan Notification, func()) {
	ch := make(chan Notification, 16)
	id := atomic.AddUint64(&h.nextID, 1)
	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = make(map[uint64]chan Notification)
	}
	h.subs[userID][id] = ch
	h.mu.Unlock()
	unsub := func() {
		h.mu.Lock()
		if m, ok := h.subs[userID]; ok {
			if c, ok := m[id]; ok {
				delete(m, id)
				close(c)
			}
			if len(m) == 0 {
				delete(h.subs, userID)
			}
		}
		h.mu.Unlock()
	}
	return ch, unsub
}

// Publish fan-outs n to every subscriber for n.UserID. Drops on
// overflow — a wedged client cannot stall the worker.
func (h *SubscriptionHub) Publish(userID string, n Notification) {
	h.mu.RLock()
	subs := h.subs[userID]
	chans := make([]chan Notification, 0, len(subs))
	for _, c := range subs {
		chans = append(chans, c)
	}
	h.mu.RUnlock()
	for _, c := range chans {
		select {
		case c <- n:
		default:
		}
	}
}
