// Package providers — telemetry sink. Every BillingGuard call passes
// through here, so it is the natural chokepoint to capture a structured
// record per agent call: who asked, which provider answered, how many
// tokens at what cost, and how long it took.
//
// Two consumers benefit:
//   1. The operator dashboard — "which agent is the most expensive on
//      Friday afternoons" is a real question and the ledger alone does
//      not carry latency or role context.
//   2. Auto-optimisation — when the planner knows "Sonnet beat Opus on
//      this story type 9 / 10 times last week", the router can pick
//      accordingly. The telemetry feed is the input to that learning.
//
// We default to a memory ring buffer so the orchestrator boots without
// extra infra. Operators can swap in a Postgres sink, a ClickHouse
// shipper, or an OTel exporter via WithTelemetrySink.

package providers

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/operations/bus"
)

// AgentCall is one structured row of telemetry. Times are UTC; durations
// are milliseconds for human readability in JSON dashboards.
type AgentCall struct {
	UserID          string    `json:"userId"`
	ProjectID       string    `json:"projectId,omitempty"`
	Role            string    `json:"role,omitempty"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	Capabilities    []string  `json:"capabilities,omitempty"`
	InputTokens     int       `json:"inputTokens"`
	OutputTokens    int       `json:"outputTokens"`
	CacheReadTokens int       `json:"cacheReadTokens,omitempty"`
	CacheNewTokens  int       `json:"cacheNewTokens,omitempty"`
	CostUSD         float64   `json:"costUSD"`
	DurationMS      int64     `json:"durationMs"`
	StartedAt       time.Time `json:"startedAt"`
	Error           string    `json:"error,omitempty"`
}

// TelemetrySink is the operator-replaceable contract. Record() must not
// block its caller — implementations buffer + flush as needed.
type TelemetrySink interface {
	Record(c AgentCall)
	Recent(limit int) []AgentCall
}

// MemorySink is a bounded ring buffer. Default sink so the orchestrator
// always has a usable feed without external infra.
//
// In addition to the bounded history, the sink fans out every Record() to
// any subscribers registered via Subscribe(). Subscribers receive a copy
// of each AgentCall on a buffered channel so a slow consumer (e.g. an SSE
// client on a flaky connection) is dropped rather than slowing producers.
type MemorySink struct {
	mu    sync.Mutex
	calls []AgentCall
	max   int

	subMu sync.RWMutex
	subs  map[int]chan AgentCall
	next  int

	// Cross-pod fan-out (optional). When set, every Record also
	// republishes the AgentCall on the per-user topic so a cost
	// subscriber on another pod sees it. Per-user topic keeps Redis
	// fan-out scoped to a tenant — a power user with 5 active
	// dashboards doesn't blast every pod with their telemetry.
	bus *bus.Multiplexer

	// busSubs tracks live cross-pod readers keyed by userID so a
	// second Subscribe for the same user re-uses the open bus
	// subscription instead of opening a duplicate one.
	busMu       sync.Mutex
	busReaders  map[string]*sinkBusReader
}

// sinkBusReader holds the bus cancel + a refcount so multiple local
// Subscribes for the same userID share one bus subscription.
type sinkBusReader struct {
	cancel func()
	refs   int
}

func NewMemorySink(max int) *MemorySink {
	if max <= 0 {
		max = 1024
	}
	return &MemorySink{
		max:        max,
		calls:      make([]AgentCall, 0, max),
		subs:       make(map[int]chan AgentCall),
		busReaders: map[string]*sinkBusReader{},
	}
}

// WithBus wires a cross-pod bus so every Record also publishes on
// "cost:<userID>" and so Subscribe also delivers cost ticks emitted
// from other pods. Nil-safe.
func (s *MemorySink) WithBus(b *bus.Multiplexer) *MemorySink {
	s.bus = b
	return s
}

func (s *MemorySink) Record(c AgentCall) {
	if c.StartedAt.IsZero() {
		c.StartedAt = time.Now().UTC()
	}
	s.mu.Lock()
	if len(s.calls) >= s.max {
		// Drop the oldest. Cheaper than a true ring buffer for our N.
		copy(s.calls, s.calls[1:])
		s.calls = s.calls[:len(s.calls)-1]
	}
	s.calls = append(s.calls, c)
	s.mu.Unlock()

	// Fan out to any live subscribers. Non-blocking: a slow SSE client
	// loses cost ticks rather than back-pressuring the BillingGuard.
	s.subMu.RLock()
	for _, ch := range s.subs {
		select {
		case ch <- c:
		default:
		}
	}
	s.subMu.RUnlock()

	// Cross-pod mirror. Per-user topic keeps Redis fan-out tenant-scoped.
	if s.bus != nil && c.UserID != "" {
		payload, err := json.Marshal(c)
		if err == nil {
			_ = s.bus.Publish(context.Background(), "cost:"+c.UserID, payload)
		}
	}
}

// SubscribeUser returns a feed of AgentCalls for a specific userID. It
// includes local Record() calls AND, when the bus is wired, telemetry
// emitted on other pods for the same user. Callers MUST invoke the
// unsubscribe func or the bus reader leaks.
//
// This is the multi-pod-aware sibling of Subscribe; callers that only
// want the legacy global feed can keep using Subscribe.
func (s *MemorySink) SubscribeUser(userID string) (<-chan AgentCall, func()) {
	out := make(chan AgentCall, 32)
	s.subMu.Lock()
	id := s.next
	s.next++
	s.subs[id] = out
	s.subMu.Unlock()

	// Open or refcount-bump a single bus reader per userID.
	var busCancel func()
	if s.bus != nil && userID != "" {
		s.busMu.Lock()
		reader, ok := s.busReaders[userID]
		if !ok {
			ctx := context.Background()
			ch, cancel, err := s.bus.Subscribe(ctx, "cost:"+userID)
			if err == nil {
				reader = &sinkBusReader{cancel: cancel}
				s.busReaders[userID] = reader
				go s.runBusReader(ch)
			}
		}
		if reader != nil {
			reader.refs++
			busCancel = func() {
				s.busMu.Lock()
				defer s.busMu.Unlock()
				r := s.busReaders[userID]
				if r == nil {
					return
				}
				r.refs--
				if r.refs <= 0 {
					r.cancel()
					delete(s.busReaders, userID)
				}
			}
		}
		s.busMu.Unlock()
	}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			s.subMu.Lock()
			delete(s.subs, id)
			s.subMu.Unlock()
			close(out)
			if busCancel != nil {
				busCancel()
			}
		})
	}
	return out, unsubscribe
}

// runBusReader feeds AgentCalls arriving from other pods into the
// local subscriber map by calling Record(). We do NOT skip the publish
// path inside that Record() — but the Multiplexer's pod-id dedup means
// our own re-publish lands on every other pod, not back on us.
//
// One caveat: a remote AgentCall lands in this sink's bounded ring
// buffer too, so the local /telemetry/recent endpoint reflects
// cluster-wide activity. That's the intended behaviour: every pod
// shows the same recent feed.
func (s *MemorySink) runBusReader(ch <-chan []byte) {
	for raw := range ch {
		var c AgentCall
		if err := json.Unmarshal(raw, &c); err != nil {
			continue
		}
		// Avoid re-publishing on the cross-pod hop — this AgentCall
		// already crossed the wire once. Push directly into local
		// subscribers + the ring buffer.
		s.mu.Lock()
		if len(s.calls) >= s.max {
			copy(s.calls, s.calls[1:])
			s.calls = s.calls[:len(s.calls)-1]
		}
		s.calls = append(s.calls, c)
		s.mu.Unlock()
		s.subMu.RLock()
		for _, dst := range s.subs {
			select {
			case dst <- c:
			default:
			}
		}
		s.subMu.RUnlock()
	}
}

// Subscribe registers a live feed of AgentCalls. The returned channel is
// closed by the unsubscribe function — callers must always invoke it,
// typically via defer, to avoid leaking goroutines feeding a dead
// connection.
func (s *MemorySink) Subscribe() (<-chan AgentCall, func()) {
	ch := make(chan AgentCall, 32)
	s.subMu.Lock()
	id := s.next
	s.next++
	s.subs[id] = ch
	s.subMu.Unlock()
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			s.subMu.Lock()
			delete(s.subs, id)
			s.subMu.Unlock()
			close(ch)
		})
	}
	return ch, unsubscribe
}

// Recent returns up to `limit` most-recent records, newest first. Safe
// to call from HTTP handlers; we copy under the lock.
func (s *MemorySink) Recent(limit int) []AgentCall {
	if limit <= 0 {
		limit = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.calls)
	if limit > n {
		limit = n
	}
	out := make([]AgentCall, limit)
	for i := 0; i < limit; i++ {
		out[i] = s.calls[n-1-i]
	}
	return out
}

// WithTelemetry registers a sink on the BillingGuard. The guard emits
// one AgentCall per CompleteStream, fired on DeltaDone (success) or on
// the channel close after a DeltaError (failure with whatever usage was
// reported before the error).
func (g *BillingGuard) WithTelemetry(s TelemetrySink) *BillingGuard {
	g.tel = s
	return g
}
