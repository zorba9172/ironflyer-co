// Package diagnostics is the in-process observability plane the
// orchestrator uses to surface recent errors and warnings without
// requiring an external log aggregator. A bounded ring buffer captures
// every WARN+ log line emitted via the zerolog hook installed in
// main.go, then exposes them through REST (/admin/logs/tail) and
// GraphQL (recentErrors / recentLogs) — both gated to platform
// operators.
//
// The ring buffer is bounded and best-effort: on contention the hook
// drops the entry rather than blocking the log call. The orchestrator
// is the source of truth for durable logs; this plane is strictly for
// "what just broke" investigations.
package diagnostics

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry is one captured log line. Fields mirror the structured fields
// the zerolog hook can recover from the event — the small set kept
// here matches what the orchestrator's logctx package stamps on every
// ctx-aware log call.
type Entry struct {
	Time        time.Time      `json:"time"`
	Level       string         `json:"level"`
	Message     string         `json:"message"`
	RequestID   string         `json:"request_id,omitempty"`
	TenantID    string         `json:"tenant_id,omitempty"`
	ExecutionID string         `json:"execution_id,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	StackTrace  string         `json:"stack_trace,omitempty"`
}

// Ring is a bounded, thread-safe ring buffer of recent Entry values.
// Append is best-effort: on contention with a Snapshot/Aggregate read
// it returns immediately rather than blocking the writer (the zerolog
// hook is on the hot path of every log call).
type Ring struct {
	cap int

	mu      sync.RWMutex
	entries []Entry
	head    int
	full    bool
}

// NewRing returns a Ring with the supplied capacity. A non-positive
// capacity falls back to 1 so the structure stays valid.
func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		capacity = 1
	}
	return &Ring{
		cap:     capacity,
		entries: make([]Entry, capacity),
	}
}

// Append records `e`. If the writer can't acquire the lock immediately
// (a snapshot is in flight) the entry is silently dropped — the hook
// must never block a log call.
func (r *Ring) Append(e Entry) {
	if r == nil {
		return
	}
	if !r.mu.TryLock() {
		// Contention path: drop the entry rather than block the caller.
		return
	}
	defer r.mu.Unlock()
	r.entries[r.head] = e
	r.head = (r.head + 1) % r.cap
	if r.head == 0 {
		r.full = true
	}
}

// Snapshot returns the ring's contents most-recent-first. The slice
// is safe for the caller to retain; entries are value-copied.
func (r *Ring) Snapshot() []Entry {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.snapshotLocked()
}

// snapshotLocked is the workhorse for Snapshot/Aggregate. Caller must
// hold at least the read lock.
func (r *Ring) snapshotLocked() []Entry {
	size := r.head
	if r.full {
		size = r.cap
	}
	out := make([]Entry, 0, size)
	// Walk newest-first.
	for i := 0; i < size; i++ {
		idx := (r.head - 1 - i + r.cap) % r.cap
		out = append(out, r.entries[idx])
	}
	return out
}

// SnapshotSince returns the subset of Snapshot strictly newer than `since`.
// Caps the result at `limit` entries when limit > 0.
func (r *Ring) SnapshotSince(since time.Time, limit int) []Entry {
	all := r.Snapshot()
	if since.IsZero() && limit <= 0 {
		return all
	}
	out := make([]Entry, 0, len(all))
	for _, e := range all {
		if !since.IsZero() && !e.Time.After(since) {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// FilterByLevel keeps entries whose Level is >= minLevel. The level
// comparison uses the orderedLevels helper so "warn" < "error".
func FilterByLevel(in []Entry, minLevel string) []Entry {
	minRank := levelRank(minLevel)
	if minRank == 0 {
		return in
	}
	out := make([]Entry, 0, len(in))
	for _, e := range in {
		if levelRank(e.Level) >= minRank {
			out = append(out, e)
		}
	}
	return out
}

// Aggregate is the error-class summary the recentErrors GraphQL query
// returns. Errors are grouped by their normalized message (see
// NormalizeMessage) so an outage that fires the same line 500 times
// surfaces as one row with Count=500.
type Aggregate struct {
	Class           string    `json:"class"`
	Count           int       `json:"count"`
	LastSeen        time.Time `json:"last_seen"`
	LastMessage     string    `json:"last_message"`
	SampleRequestID string    `json:"sample_request_id,omitempty"`
	LastLevel       string    `json:"last_level"`
}

// Aggregate groups the ring's entries by normalized message class and
// returns the result sorted by Count desc, then LastSeen desc.
func (r *Ring) Aggregate() []Aggregate {
	snap := r.Snapshot()
	classes := make(map[string]*Aggregate, len(snap))
	for _, e := range snap {
		if levelRank(e.Level) < levelRank("warn") {
			continue
		}
		class := NormalizeMessage(e.Message)
		agg, ok := classes[class]
		if !ok {
			agg = &Aggregate{
				Class:           class,
				LastSeen:        e.Time,
				LastMessage:     e.Message,
				SampleRequestID: e.RequestID,
				LastLevel:       e.Level,
			}
			classes[class] = agg
		}
		agg.Count++
		if e.Time.After(agg.LastSeen) {
			agg.LastSeen = e.Time
			agg.LastMessage = e.Message
			agg.LastLevel = e.Level
			if e.RequestID != "" {
				agg.SampleRequestID = e.RequestID
			}
		}
	}
	out := make([]Aggregate, 0, len(classes))
	for _, agg := range classes {
		out = append(out, *agg)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}

// levelRank maps a zerolog level name to an integer so callers can
// filter "warn-or-worse". Returns 0 for unknown values so an unknown
// level always passes.
func levelRank(l string) int {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case "trace":
		return 1
	case "debug":
		return 2
	case "info":
		return 3
	case "warn", "warning":
		return 4
	case "error":
		return 5
	case "fatal":
		return 6
	case "panic":
		return 7
	default:
		return 0
	}
}
