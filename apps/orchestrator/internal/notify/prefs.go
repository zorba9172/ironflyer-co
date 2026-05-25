package notify

import (
	"container/list"
	"context"
	"os"
	"strconv"
	"sync"
)

// defaultPrefsLRUCap is the fall-back capacity used when NOTIFY_PREFS_LRU_CAP
// is unset, empty, or unparseable. 10_000 distinct users worth of cached
// rules at ~hundreds of bytes apiece keeps the prefs cache under a couple of
// megabytes — well below any realistic process budget — while still serving
// the hot path without round-tripping a persistent store.
const defaultPrefsLRUCap = 10_000

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

// prefsEntry is the per-key payload stored in the LRU list. The list element
// points back at this struct so eviction can recover the userID without an
// auxiliary reverse-map.
type prefsEntry struct {
	userID string
	rule   NotificationRule
}

// MemoryPrefsStore is a thread-safe in-memory PrefsStore with a bounded LRU
// to prevent unbounded growth across a long-lived process. Capacity is
// configurable via NOTIFY_PREFS_LRU_CAP (positive integer); invalid or
// missing values fall back to defaultPrefsLRUCap.
//
// The LRU is implemented inline rather than via a dependency: a doubly-
// linked list (container/list) plus a map keyed by userID gives O(1) reads,
// writes, and evictions, and keeps the dependency surface flat. Each Get
// promotes the entry; each Set inserts or refreshes and evicts the LRU tail
// when capacity is exceeded.
type MemoryPrefsStore struct {
	mu    sync.Mutex
	cap   int
	order *list.List               // front = most recently used
	rules map[string]*list.Element // userID → element holding *prefsEntry
}

// NewMemoryPrefsStore returns an empty bounded LRU store using the cap from
// NOTIFY_PREFS_LRU_CAP (or defaultPrefsLRUCap when unset/invalid).
func NewMemoryPrefsStore() *MemoryPrefsStore {
	return NewMemoryPrefsStoreWithCap(prefsLRUCapFromEnv())
}

// NewMemoryPrefsStoreWithCap constructs a store with an explicit capacity.
// Non-positive caps are silently rounded up to defaultPrefsLRUCap so the
// store is never effectively disabled.
func NewMemoryPrefsStoreWithCap(cap int) *MemoryPrefsStore {
	if cap <= 0 {
		cap = defaultPrefsLRUCap
	}
	return &MemoryPrefsStore{
		cap:   cap,
		order: list.New(),
		rules: make(map[string]*list.Element),
	}
}

// prefsLRUCapFromEnv parses NOTIFY_PREFS_LRU_CAP. Anything that isn't a
// positive integer falls back to the default cap — operators can still tune
// it without risking an instant misconfiguration.
func prefsLRUCapFromEnv() int {
	raw := os.Getenv("NOTIFY_PREFS_LRU_CAP")
	if raw == "" {
		return defaultPrefsLRUCap
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultPrefsLRUCap
	}
	return n
}

// Get returns the user's rule, or a synthesised DefaultRule if none exists.
// This makes the GET preferences handler total: every authenticated user
// gets a non-empty rule back even on first visit. Cached entries are
// promoted to MRU; misses do not allocate cache entries (Set is the only
// path that inserts).
func (m *MemoryPrefsStore) Get(_ context.Context, userID string) (NotificationRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if el, ok := m.rules[userID]; ok {
		m.order.MoveToFront(el)
		return el.Value.(*prefsEntry).rule, nil
	}
	return DefaultRule(userID, ""), nil
}

// Set upserts the rule keyed by UserID and promotes it to MRU. When the
// store is at capacity the LRU tail is evicted under the same mutex so the
// caller never observes a transient over-cap state.
func (m *MemoryPrefsStore) Set(_ context.Context, rule NotificationRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if el, ok := m.rules[rule.UserID]; ok {
		el.Value.(*prefsEntry).rule = rule
		m.order.MoveToFront(el)
		return nil
	}
	el := m.order.PushFront(&prefsEntry{userID: rule.UserID, rule: rule})
	m.rules[rule.UserID] = el
	// Evict from the tail until we're back within cap. A single Set can only
	// add one entry, so this loop runs at most once in practice; the loop
	// guards against future code paths that batch inserts.
	for m.order.Len() > m.cap {
		tail := m.order.Back()
		if tail == nil {
			break
		}
		m.order.Remove(tail)
		delete(m.rules, tail.Value.(*prefsEntry).userID)
	}
	return nil
}

// ListAll returns every cached rule. The order is unspecified — callers must
// not rely on LRU ordering leaking through this method.
func (m *MemoryPrefsStore) ListAll(_ context.Context) ([]NotificationRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]NotificationRule, 0, len(m.rules))
	for _, el := range m.rules {
		out = append(out, el.Value.(*prefsEntry).rule)
	}
	return out, nil
}
