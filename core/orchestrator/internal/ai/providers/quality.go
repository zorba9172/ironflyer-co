// Package providers — gate-verdict quality signal feeding the bandit.
//
// The bandit's pre-A30 reward formula was strictly economic:
//
//	r = 1 - costNorm - latNorm
//
// That punishes slow / expensive providers but says nothing about
// whether the provider's OUTPUT was any good. The finisher already
// produces a strong post-hoc signal — gate verdicts. A gate pass after
// the provider's patch means the provider delivered something the
// repair loop accepted; a gate fail means the provider's patch had to
// be repaired or thrown out.
//
// This file exposes the recorder + sink contracts. The bandit folds
// per-provider quality EMA into the reward (see bandit.go reward
// formula). The finisher integration — calling RecordGateOutcome at
// the end of each gate run — is wired by a separate integration agent;
// the contract lives here so the orchestrator's compilation stays
// independent of the wiring order.

package providers

import (
	"sync"
)

// GateOutcome is a strongly-typed quality signal from the finisher's
// gate verdict. The orchestrator calls RecordGateOutcome after each
// gate run; the bandit folds this into provider rewards.
type GateOutcome struct {
	// Provider is the provider name (Provider.Name()) whose patch was
	// just gated. Required.
	Provider string
	// Capability is the request capability tag (matches the request's
	// caps). Optional — left empty when the gate run is provider-wide.
	Capability string
	// Passed is true when the gate verdict was a clean pass.
	Passed bool
	// IssuesCount carries auxiliary detail: 0 = clean; > 0 = repaired
	// or partial. Not used by the bandit today but retained so future
	// scoring (e.g. partial credit) doesn't need a wire change.
	IssuesCount int
}

// QualitySink is implemented by the telemetry/bandit layer. The
// finisher calls RecordGateOutcome after each gate verdict; the
// telemetry sink stores it and exposes per-provider statistics via
// ProviderQuality.
type QualitySink interface {
	RecordGateOutcome(o GateOutcome)
}

// QualityStats is the per-provider rollup used by the bandit reward
// formula. PassRate is an EMA so a provider's recent record matters
// more than its long-tail history.
type QualityStats struct {
	Provider string
	N        int     // total outcomes seen for this provider
	PassRate float64 // EMA in [0, 1]; neutral = 0.5
}

// MinQualitySamples is the floor below which the bandit treats quality
// as neutral. Below this count we have too few observations to trust
// the EMA. 10 outcomes ≈ 2-3 minutes of active finisher use.
const MinQualitySamples = 10

// qualityEMAAlpha is the smoothing constant. 0.1 means each new
// outcome shifts the EMA by 10% of the gap between it and the prior
// EMA — gives ~22-outcome effective lookback (1/alpha · ln(0.1)
// rounded). Tuned to track week-of-Friday drift without thrashing.
const qualityEMAAlpha = 0.1

// QualityStatsProvider is the read surface the bandit pulls. Kept
// separate from QualitySink so a mock sink (recording only) doesn't
// also have to satisfy the read contract.
type QualityStatsProvider interface {
	ProviderQuality(provider string) QualityStats
}

// QualityRegistry is the default in-process QualitySink + provider.
// Concurrent-safe; subscribers / cross-pod fan-out are intentionally
// NOT included — gate verdicts are an internal training signal, not a
// dashboard tile. If we later need per-tenant per-cluster aggregation
// we can layer a bus-backed implementation in front of this one.
type QualityRegistry struct {
	mu    sync.RWMutex
	stats map[string]*QualityStats
}

// NewQualityRegistry returns an empty registry ready to record.
func NewQualityRegistry() *QualityRegistry {
	return &QualityRegistry{stats: map[string]*QualityStats{}}
}

// RecordGateOutcome folds o into the per-provider EMA. Drops empty
// provider names silently — the finisher may emit a provider-less
// verdict during early bring-up (e.g. blueprint-only gates) and we
// don't want those polluting per-provider stats.
func (q *QualityRegistry) RecordGateOutcome(o GateOutcome) {
	if q == nil || o.Provider == "" {
		return
	}
	sample := 0.0
	if o.Passed {
		sample = 1.0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	s, ok := q.stats[o.Provider]
	if !ok {
		// Seed at the sample value so an all-pass / all-fail prefix
		// converges immediately instead of starting at 0.5 and dragging
		// the EMA for the first dozen outcomes.
		s = &QualityStats{Provider: o.Provider, PassRate: sample}
		q.stats[o.Provider] = s
		s.N = 1
		return
	}
	s.N++
	s.PassRate = (1-qualityEMAAlpha)*s.PassRate + qualityEMAAlpha*sample
}

// ProviderQuality returns the rollup for the named provider. Returns
// a neutral (N=0, PassRate=0.5) stats row when the provider is
// unknown so the bandit's qualityBoost formula stays a no-op rather
// than punishing untracked providers.
func (q *QualityRegistry) ProviderQuality(provider string) QualityStats {
	if q == nil || provider == "" {
		return QualityStats{Provider: provider, PassRate: 0.5}
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	s, ok := q.stats[provider]
	if !ok {
		return QualityStats{Provider: provider, PassRate: 0.5}
	}
	// Return a value-copy so callers can't race on our pointer.
	return *s
}

// activeQuality lets the bandit pull a QualityStatsProvider without
// being constructed with one explicitly — keeps existing wiring frozen
// while the finisher integration is in flight. Set via RegisterQuality;
// nil falls back to neutral.
var (
	activeQualityMu sync.RWMutex
	activeQuality   QualityStatsProvider
)

// RegisterQuality installs the live QualityStatsProvider the bandit
// will consult on every Rerank. Safe to call from any goroutine;
// idempotent. Typically called once at startup with the same registry
// instance that the finisher's RecordGateOutcome will write to.
func RegisterQuality(p QualityStatsProvider) {
	activeQualityMu.Lock()
	activeQuality = p
	activeQualityMu.Unlock()
}

// ActiveQuality returns the currently registered provider, or nil
// when nothing has been wired yet.
func ActiveQuality() QualityStatsProvider {
	activeQualityMu.RLock()
	defer activeQualityMu.RUnlock()
	return activeQuality
}
