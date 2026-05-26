// Package lastprovider keeps the last (provider, capability) pair that
// served a request for a given execution. It exists so post-hoc signals
// produced by the finisher (notably gate verdicts) can be attributed
// back to the provider whose patch was just evaluated.
//
// The pre-A31 wiring fanned gate outcomes into providers.QualitySink
// with empty Provider/Capability strings; providers.QualityRegistry
// silently drops empty-provider rows, so the bandit never learned from
// gate verdicts. providers/guard.go now calls Record() on every
// per-token cost attribution (the post-billing leg that already has
// the executionID + provider name + capabilities in scope), and the
// finisher's recordGateOutcome reads it back so the bandit's
// per-provider EMA actually moves.
//
// The tracker is in-process and bounded by an FIFO eviction policy
// (simpler than true LRU but good enough — entries are short-lived and
// rotation is age-driven). Cap defaults to 10 000 entries; override via
// IRONFLYER_LASTPROVIDER_CAP. Forget() is called at terminal settle in
// finisher.Engine.Run so successful executions don't rely on eviction.
package lastprovider

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// Record is the per-execution last-provider snapshot. Only the latest
// provider per executionID is retained — multi-provider executions
// produce one Record at any moment, the one for the most recent
// attribution.
type Record struct {
	Provider   string
	Capability string
	RecordedAt time.Time
}

// Tracker is a thread-safe, bounded map from executionID to Record.
// Concurrent reads (Get) take the RLock; writes (Record / Forget) take
// the full Lock. Eviction is FIFO: the oldest insertion is dropped when
// the cap is hit. Re-Recording an existing executionID does NOT bump
// the FIFO position — the existing slot is updated in place so eviction
// targets executions that genuinely went silent first.
type Tracker struct {
	mu    sync.RWMutex
	m     map[string]Record
	cap   int
	order []string
}

// New constructs a Tracker with the supplied cap. A non-positive cap
// is replaced with the package default (10 000).
func New(cap int) *Tracker {
	if cap <= 0 {
		cap = defaultCap
	}
	return &Tracker{
		m:     make(map[string]Record, cap/4+1),
		cap:   cap,
		order: make([]string, 0, cap/4+1),
	}
}

// Record stores provider+capability as the latest serving pair for
// executionID. Empty executionID or empty provider is a no-op — the
// upstream QualityRegistry drops empty-provider rows anyway, and we
// don't want to seed slots that can never satisfy a Get.
func (t *Tracker) Record(executionID, provider, capability string) {
	if t == nil || executionID == "" || provider == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.m[executionID]; !ok {
		// New slot — enforce cap with FIFO eviction.
		for len(t.m) >= t.cap && len(t.order) > 0 {
			evict := t.order[0]
			t.order = t.order[1:]
			delete(t.m, evict)
		}
		t.order = append(t.order, executionID)
	}
	t.m[executionID] = Record{
		Provider:   provider,
		Capability: capability,
		RecordedAt: time.Now().UTC(),
	}
}

// Get returns the latest Record for executionID, or (zero, false) when
// nothing has been recorded.
func (t *Tracker) Get(executionID string) (Record, bool) {
	if t == nil || executionID == "" {
		return Record{}, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	rec, ok := t.m[executionID]
	return rec, ok
}

// Forget drops the slot for executionID. Safe to call for unknown ids.
// Called at terminal settle so successful executions don't depend on
// eviction to release their slot.
func (t *Tracker) Forget(executionID string) {
	if t == nil || executionID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.m[executionID]; !ok {
		return
	}
	delete(t.m, executionID)
	for i, id := range t.order {
		if id == executionID {
			t.order = append(t.order[:i], t.order[i+1:]...)
			return
		}
	}
}

// Snapshot returns a copy of every tracked record, keyed by
// executionID. Diagnostic only — callers must not mutate the returned
// map (it's a snapshot copy, but the contract is read-only).
func (t *Tracker) Snapshot() map[string]Record {
	if t == nil {
		return map[string]Record{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]Record, len(t.m))
	for k, v := range t.m {
		out[k] = v
	}
	return out
}

// --- package-level singleton ------------------------------------------------

const (
	defaultCap   = 10_000
	capEnvVar    = "IRONFLYER_LASTPROVIDER_CAP"
)

var defaultTracker = New(resolveCap())

// resolveCap reads IRONFLYER_LASTPROVIDER_CAP at process start. A
// missing or unparseable value falls back to defaultCap so callers
// never have to gate on env presence.
func resolveCap() int {
	if raw := os.Getenv(capEnvVar); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultCap
}

// Default returns the package-level singleton Tracker. Wired callers
// (BillingGuard, finisher.Engine) reach for this so the producer and
// the consumer share one instance without an explicit wireup pass.
func Default() *Tracker { return defaultTracker }

// Touch is the package-level convenience that forwards to
// Default().Record. Named Touch (rather than Record) so it doesn't
// collide with the Record struct type at the package scope.
func Touch(executionID, provider, capability string) {
	defaultTracker.Record(executionID, provider, capability)
}

// Get is the package-level convenience that forwards to Default().
func Get(executionID string) (Record, bool) {
	return defaultTracker.Get(executionID)
}

// Forget is the package-level convenience that forwards to Default().
func Forget(executionID string) {
	defaultTracker.Forget(executionID)
}
