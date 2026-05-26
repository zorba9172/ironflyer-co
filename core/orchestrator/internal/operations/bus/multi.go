package bus

import (
	"context"
	"crypto/rand"
	"strings"
	"sync"

	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// PodIDSize is the byte length of the per-process identifier prepended
// to every Publish payload. 8 bytes is enough to make two pods colliding
// astronomically unlikely without bloating every message.
const PodIDSize = 8

// Multiplexer fronts a Bus with an in-process subscriber map so
// same-pod delivery never round-trips through Redis, and so a pod
// ignores its own messages when they come back across the wire.
//
// Publish path:
//  1. Synchronously fan out the raw payload to every local subscriber.
//  2. Prepend the 8-byte pod-id and call bus.Publish. The Redis reader
//     loop on the same pod will see this echo and drop it because the
//     prefix matches PodID.
//
// Subscribe path:
//  1. Add the caller's channel to the local subscriber list.
//  2. Ensure (refcounted) a single bus.Subscribe is open for the topic.
//     When messages arrive from the backend, strip the prefix and check
//     it: if it matches PodID, the message is an echo of our own
//     publish — drop. Otherwise, fan out to every local subscriber.
//
// Topics prefixed with LocalPrefix never call bus.Publish/Subscribe
// and stay entirely in-process (used for inline completion where the
// publisher and subscriber share a pod by construction).
type Multiplexer struct {
	bus   Bus
	podID []byte

	mu       sync.Mutex
	subs     map[string]*muxTopic
}

type muxTopic struct {
	chans []chan []byte
	// busCancel tears down the underlying bus subscription when the
	// last local subscriber leaves. nil for local-only topics.
	busCancel func()
	// closeCh signals the reader goroutine to exit on teardown.
	closeCh chan struct{}
}

// NewMultiplexer wires a Multiplexer onto the supplied Bus. PodID is
// minted from crypto/rand so two processes never collide. The bus may
// be a MemoryBus (single-pod) or a RedisBus (multi-pod) — the
// Multiplexer is identical either way.
func NewMultiplexer(b Bus) *Multiplexer {
	if b == nil {
		b = NewMemoryBus()
	}
	id := make([]byte, PodIDSize)
	_, _ = rand.Read(id)
	return &Multiplexer{
		bus:   b,
		podID: id,
		subs:  map[string]*muxTopic{},
	}
}

// PodID returns this multiplexer's identity tag. Exported mostly for
// tests + diagnostic logging; callers shouldn't need to look at it.
func (m *Multiplexer) PodID() []byte {
	out := make([]byte, len(m.podID))
	copy(out, m.podID)
	return out
}

// Publish writes payload to every local subscriber and (unless the
// topic is local-only) republishes to the backing Bus. The Bus copy
// gets the pod-id prefix so this pod can drop the echo when it comes
// back.
func (m *Multiplexer) Publish(ctx context.Context, topic string, payload []byte) error {
	kind := topicKind(topic)
	metrics.ObserveBusPublish(kind)

	// 1) Local fan-out — synchronous so subscribers on the same pod
	// see the event with no Redis round-trip.
	m.fanLocal(topic, payload, "local")

	// 2) Cross-pod hop. Skipped for local-only topics.
	if isLocal(topic) {
		return nil
	}
	stamped := make([]byte, 0, len(m.podID)+len(payload))
	stamped = append(stamped, m.podID...)
	stamped = append(stamped, payload...)
	return m.bus.Publish(ctx, topic, stamped)
}

// Subscribe attaches a new local subscriber and (for non-local topics)
// ensures a single bus.Subscribe is live for the topic, refcounted by
// the count of local subscribers.
func (m *Multiplexer) Subscribe(ctx context.Context, topic string) (<-chan []byte, func(), error) {
	kind := topicKind(topic)
	out := make(chan []byte, SubBuffer)
	m.mu.Lock()
	t, ok := m.subs[topic]
	if !ok {
		t = &muxTopic{closeCh: make(chan struct{})}
		m.subs[topic] = t
		// Only open the backend subscription for non-local topics.
		if !isLocal(topic) {
			busCh, cancel, err := m.bus.Subscribe(ctx, topic)
			if err != nil {
				delete(m.subs, topic)
				m.mu.Unlock()
				close(out)
				return out, func() {}, err
			}
			t.busCancel = cancel
			go m.run(topic, busCh, t)
		}
	}
	t.chans = append(t.chans, out)
	metrics.SetBusActiveSubscribers(kind, m.activeForKindLocked(kind))
	m.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			m.mu.Lock()
			t, ok := m.subs[topic]
			if !ok {
				m.mu.Unlock()
				return
			}
			for i, ch := range t.chans {
				if ch == out {
					t.chans = append(t.chans[:i], t.chans[i+1:]...)
					close(out)
					break
				}
			}
			// Last local subscriber — close the backend hop too.
			if len(t.chans) == 0 {
				close(t.closeCh)
				if t.busCancel != nil {
					t.busCancel()
				}
				delete(m.subs, topic)
			}
			metrics.SetBusActiveSubscribers(kind, m.activeForKindLocked(kind))
			m.mu.Unlock()
		})
	}
	return out, cancel, nil
}

// run is the reader goroutine for a topic's backend subscription. It
// strips the pod-id prefix, drops echoes of our own publishes, and
// fans the remaining bytes out to local subscribers.
func (m *Multiplexer) run(topic string, busCh <-chan []byte, t *muxTopic) {
	kind := topicKind(topic)
	for {
		select {
		case <-t.closeCh:
			return
		case raw, ok := <-busCh:
			if !ok {
				return
			}
			if len(raw) < PodIDSize {
				continue
			}
			prefix := raw[:PodIDSize]
			payload := raw[PodIDSize:]
			// Skip echoes of our own publishes — the local fan-out path
			// already delivered them.
			if equalBytes(prefix, m.podID) {
				continue
			}
			metrics.ObserveBusReceive(kind, "remote")
			m.fanLocal(topic, payload, "remote")
		}
	}
}

// fanLocal dispatches payload to every currently-registered local
// subscriber for topic. Non-blocking per channel: a slow consumer
// drops messages (with metric) rather than stalling the producer.
// source is "local" (Publish called on this pod) or "remote" (message
// arrived via the backend bus).
func (m *Multiplexer) fanLocal(topic string, payload []byte, source string) {
	kind := topicKind(topic)
	m.mu.Lock()
	t, ok := m.subs[topic]
	if !ok {
		m.mu.Unlock()
		if source == "local" {
			metrics.ObserveBusReceive(kind, "local")
		}
		return
	}
	chans := make([]chan []byte, len(t.chans))
	copy(chans, t.chans)
	m.mu.Unlock()
	if source == "local" {
		metrics.ObserveBusReceive(kind, "local")
	}
	for _, ch := range chans {
		select {
		case ch <- payload:
		default:
			metrics.ObserveBusSubscriberDrop(kind)
		}
	}
}

// activeForKindLocked counts how many local subscribers we currently
// hold across every topic with the given kind. Caller must hold m.mu.
func (m *Multiplexer) activeForKindLocked(kind string) int {
	n := 0
	for topic, t := range m.subs {
		if topicKind(topic) == kind {
			n += len(t.chans)
		}
	}
	return n
}

// Close releases every local subscriber and tears down all backend
// subscriptions. Idempotent.
func (m *Multiplexer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for topic, t := range m.subs {
		close(t.closeCh)
		if t.busCancel != nil {
			t.busCancel()
		}
		for _, ch := range t.chans {
			close(ch)
		}
		delete(m.subs, topic)
	}
	return m.bus.Close()
}

// topicKind extracts the first colon-segment of a topic, normalising
// dots into the colon-delimited label space used by metric vectors.
// Bounded cardinality: callers must only construct topics with a small
// set of known kinds (see package doc).
func topicKind(topic string) string {
	// LocalPrefix wraps another kind: "local:inline:abc" → "inline".
	if isLocal(topic) {
		topic = topic[len(LocalPrefix):]
	}
	i := strings.Index(topic, ":")
	if i < 0 {
		return topic
	}
	return strings.ReplaceAll(topic[:i], ".", "_")
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
