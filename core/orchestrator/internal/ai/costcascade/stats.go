// Package costcascade is the layered AI-cost-optimization front door.
//
// The cheapest AI call is the one that never happens. Every model call the
// orchestrator makes is routed through progressively more expensive layers,
// and the cheapest layer that can answer wins:
//
//	rules     — deterministic answer, 0 tokens, 0 latency
//	cache      — exact-hash replay of a prior identical call, 0 tokens
//	knowledge — answered from retrieved project knowledge, 0 tokens
//	reflex    — small/cheap model tier (Haiku / 4o-mini / Flash)
//	planning  — medium model tier (Sonnet / gpt-4o / Gemini Pro)
//	reasoning — premium model tier (Opus / o3), used sparingly
//
// The Cascade is a drop-in decorator around providers.BillingGuard: it
// satisfies the same one-method completer interface the agents registry
// consumes, so wiring it changes one line and, on any miss, delegates
// verbatim to the wrapped guard. It NEVER fabricates generated code — the
// only zero-cost answers it returns come from deterministic rules, an
// exact-hash cache of a previously-billed identical call, or an
// operator-supplied knowledge hook. Correctness of shipped patches is
// preserved; the win is in the calls that never need to run.
//
// Observability is first-class: every request is attributed to the layer
// that resolved it so the resolution distribution can be measured against
// the target mix (rules 60% / cache 20% / reflex 10% / planning 8% /
// reasoning 2%), and a self-tuning aggression controller raises caching /
// reuse / routing pressure automatically when AI cost breaches its share
// of revenue.
package costcascade

import "sync/atomic"

// Layer identifies the cheapest layer that resolved a request. The first
// three resolve WITHOUT a model call; the last three are model-call tiers.
type Layer string

const (
	LayerRules     Layer = "rules"
	LayerCache     Layer = "cache"
	LayerKnowledge Layer = "knowledge"
	LayerReflex    Layer = "reflex"
	LayerPlanning  Layer = "planning"
	LayerReasoning Layer = "reasoning"
)

// IsModelCall reports whether the layer incurred a paid provider call.
func (l Layer) IsModelCall() bool {
	switch l {
	case LayerReflex, LayerPlanning, LayerReasoning:
		return true
	default:
		return false
	}
}

// Stats is a lock-free rolling tally of how many requests each layer
// resolved, plus the estimated USD avoided by the zero-cost layers. It is
// a process-lifetime accumulator; the live percentage view is derived in
// Snapshot. Prometheus carries the same signal for dashboards — Stats
// exists so the ops GraphQL surface can read the distribution in one call
// without scraping the metrics endpoint.
type Stats struct {
	rules     atomic.Uint64
	cache     atomic.Uint64
	knowledge atomic.Uint64
	reflex    atomic.Uint64
	planning  atomic.Uint64
	reasoning atomic.Uint64

	// savingsMicroUSD accumulates avoided cost in micro-dollars so the
	// counter stays integer-atomic (float atomics need a CAS loop).
	savingsMicroUSD atomic.Uint64
}

func (s *Stats) record(l Layer) {
	switch l {
	case LayerRules:
		s.rules.Add(1)
	case LayerCache:
		s.cache.Add(1)
	case LayerKnowledge:
		s.knowledge.Add(1)
	case LayerReflex:
		s.reflex.Add(1)
	case LayerPlanning:
		s.planning.Add(1)
	case LayerReasoning:
		s.reasoning.Add(1)
	}
}

func (s *Stats) addSavings(usd float64) {
	if usd <= 0 {
		return
	}
	s.savingsMicroUSD.Add(uint64(usd * 1_000_000))
}

// Snapshot is an immutable view of the cascade's resolution distribution.
type Snapshot struct {
	Rules     uint64 `json:"rules"`
	Cache     uint64 `json:"cache"`
	Knowledge uint64 `json:"knowledge"`
	Reflex    uint64 `json:"reflex"`
	Planning  uint64 `json:"planning"`
	Reasoning uint64 `json:"reasoning"`
	Total     uint64 `json:"total"`
	// AvoidedCalls is rules+cache+knowledge — requests that never reached
	// a model. AvoidedFraction is that count over Total.
	AvoidedCalls    uint64  `json:"avoidedCalls"`
	AvoidedFraction float64 `json:"avoidedFraction"`
	SavingsUSD      float64 `json:"savingsUsd"`
}

// Snapshot reads the current tallies. Cheap and lock-free.
func (s *Stats) Snapshot() Snapshot {
	r := s.rules.Load()
	c := s.cache.Load()
	k := s.knowledge.Load()
	rx := s.reflex.Load()
	pl := s.planning.Load()
	rs := s.reasoning.Load()
	total := r + c + k + rx + pl + rs
	avoided := r + c + k
	frac := 0.0
	if total > 0 {
		frac = float64(avoided) / float64(total)
	}
	return Snapshot{
		Rules: r, Cache: c, Knowledge: k,
		Reflex: rx, Planning: pl, Reasoning: rs,
		Total: total, AvoidedCalls: avoided, AvoidedFraction: frac,
		SavingsUSD: float64(s.savingsMicroUSD.Load()) / 1_000_000,
	}
}
