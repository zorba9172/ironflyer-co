package bus

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

// channelPrefix namespaces every Redis pub/sub channel so the bus
// cannot collide with the lock or rate-limit keys also kept in the same
// Redis instance. The full channel name is `ironflyer:bus:<topic>`.
const channelPrefix = "ironflyer:bus:"

// RedisBus uses Redis pub/sub for cross-pod fan-out. The underlying
// client is the same go-redis instance the rest of the orchestrator
// uses for distributed locks + rate limits.
//
// One PubSub object per topic, lazily created on first subscribe and
// torn down when the last subscriber for that topic cancels.
// Subscribers receive `[]byte` payloads exactly as published — the
// Multiplexer layer prepends an 8-byte pod-id so the reader can drop
// echoes of its own publishes.
type RedisBus struct {
	rc *redis.Client

	mu     sync.Mutex
	subs   map[string]*redisTopic
	closed bool
}

type redisTopic struct {
	ps      *redis.PubSub
	refs    int
	chans   []chan []byte
	closeCh chan struct{}
}

// NewRedisBus wraps a go-redis client. Pass the embedded *redis.Client
// from the existing redisbus.Client.
func NewRedisBus(rc *redis.Client) *RedisBus {
	return &RedisBus{
		rc:   rc,
		subs: map[string]*redisTopic{},
	}
}

// Publish writes payload to the Redis channel for topic. Returns nil on
// success; Redis transport errors propagate so the Multiplexer can
// surface them in metrics.
func (r *RedisBus) Publish(ctx context.Context, topic string, payload []byte) error {
	if r == nil || r.rc == nil {
		return nil
	}
	return r.rc.Publish(ctx, channelPrefix+topic, payload).Err()
}

// Subscribe attaches a new consumer to the topic. The first subscriber
// for a topic opens the underlying Redis PubSub; subsequent subscribers
// share it. The returned cancel decrements the refcount and closes the
// PubSub when the last subscriber leaves.
func (r *RedisBus) Subscribe(ctx context.Context, topic string) (<-chan []byte, func(), error) {
	out := make(chan []byte, SubBuffer)
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		close(out)
		return out, func() {}, ErrClosed
	}
	t, ok := r.subs[topic]
	if !ok {
		ps := r.rc.Subscribe(ctx, channelPrefix+topic)
		// Block on the subscription confirmation so the caller can rely
		// on subsequent publishes actually being delivered.
		if _, err := ps.Receive(ctx); err != nil {
			r.mu.Unlock()
			_ = ps.Close()
			close(out)
			return out, func() {}, err
		}
		t = &redisTopic{
			ps:      ps,
			closeCh: make(chan struct{}),
		}
		r.subs[topic] = t
		go r.run(topic, t)
	}
	t.refs++
	t.chans = append(t.chans, out)
	r.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			t, ok := r.subs[topic]
			if !ok {
				return
			}
			// Drop this subscriber's channel from the fan-out slice.
			for i, ch := range t.chans {
				if ch == out {
					t.chans = append(t.chans[:i], t.chans[i+1:]...)
					close(out)
					break
				}
			}
			t.refs--
			if t.refs <= 0 {
				close(t.closeCh)
				_ = t.ps.Close()
				delete(r.subs, topic)
			}
		})
	}
	return out, cancel, nil
}

// run is the per-topic reader goroutine. Fan-out is non-blocking per
// channel — a slow Multiplexer drops messages rather than back-pressure
// the Redis client (which would eventually stall every other topic).
func (r *RedisBus) run(topic string, t *redisTopic) {
	ch := t.ps.Channel()
	for {
		select {
		case <-t.closeCh:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			payload := []byte(msg.Payload)
			r.mu.Lock()
			snapshot := make([]chan []byte, len(t.chans))
			copy(snapshot, t.chans)
			r.mu.Unlock()
			for _, dst := range snapshot {
				select {
				case dst <- payload:
				default:
					// Slow consumer — drop. Metric is bumped at the
					// Multiplexer layer where the topic-kind label is
					// available.
				}
			}
		}
	}
}

// Close releases every active subscription. Safe to call multiple
// times. After Close, Publish/Subscribe return ErrClosed.
func (r *RedisBus) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	for topic, t := range r.subs {
		close(t.closeCh)
		_ = t.ps.Close()
		for _, ch := range t.chans {
			close(ch)
		}
		delete(r.subs, topic)
	}
	return nil
}
