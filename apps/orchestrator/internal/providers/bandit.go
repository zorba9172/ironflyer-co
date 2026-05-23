// Package providers — UCB1-style multi-armed bandit that re-ranks the
// router's candidate chain using historical performance recorded by the
// TelemetrySink.
//
// Why a bandit here:
// The router's base scoring is a static count of capability-tag overlap.
// It can tell that Claude advertises CapCode but not that Claude was
// 3× faster than Gemini on the last 50 code-capability calls in this
// deployment. The bandit closes that gap — it nudges the chain toward
// providers that historically succeeded, were fast, and were cheap on
// the same kind of task — while UCB1's exploration term keeps cold or
// rarely-used providers in the running.
//
// Per arm = (provider, capability-set). An arm's reward on one call is
//
//	r = 1 - min(cost/MaxCostUSD, 1) - min(duration/MaxLatencyMS, 1)
//
// clipped to [0, 1]; errors count as zero reward. UCB1 score is the
// classic mean + ExploreBonus * sqrt(2 * ln(total) / n). Providers with
// zero matched records get a pure-exploration boost so they still get a
// turn at bat.

package providers

import (
	"math"
	"sort"
)

// Bandit re-ranks the router's candidate chain using historical
// performance from the telemetry sink.
type Bandit struct {
	Sink         TelemetrySink
	LookbackN    int     // pull the last N records from Sink.Recent. Default 256.
	MaxCostUSD   float64 // for cost normalisation. Default 0.10 — typical agent call.
	MaxLatencyMS int64   // for latency normalisation. Default 30_000 (30s).
	// ExploreBonus is the UCB1 c-constant. Default sqrt(2) ≈ 1.414.
	ExploreBonus float64
}

// Rerank takes a chain of providers (already filtered + capability-
// scored by Pick/PickChain) plus the request's capability tags and
// returns the same providers re-ordered by descending bandit score.
// Stable when the bandit has no signal — falls back to the input order.
func (b *Bandit) Rerank(chain []Provider, caps []Capability) []Provider {
	if b == nil || b.Sink == nil || len(chain) <= 1 {
		return chain
	}

	lookback := b.LookbackN
	if lookback <= 0 {
		lookback = 256
	}
	maxCost := b.MaxCostUSD
	if maxCost <= 0 {
		maxCost = 0.10
	}
	maxLatency := b.MaxLatencyMS
	if maxLatency <= 0 {
		maxLatency = 30_000
	}
	explore := b.ExploreBonus
	if explore <= 0 {
		explore = math.Sqrt2
	}

	records := b.Sink.Recent(lookback)
	if len(records) == 0 {
		return chain
	}

	// Build a quick lookup of the requested capabilities.
	wantSet := make(map[Capability]struct{}, len(caps))
	for _, c := range caps {
		wantSet[c] = struct{}{}
	}

	// Filter records to those that overlap with the requested caps. If
	// no caps were requested we can't narrow the arm — fall back to all
	// records (still useful: "this provider just succeeded a lot").
	type stat struct {
		n    int
		sum  float64
	}
	stats := make(map[string]*stat)
	total := 0
	for _, rec := range records {
		if len(wantSet) > 0 {
			overlap := false
			for _, rc := range rec.Capabilities {
				if _, ok := wantSet[Capability(rc)]; ok {
					overlap = true
					break
				}
			}
			if !overlap {
				continue
			}
		}
		total++
		st, ok := stats[rec.Provider]
		if !ok {
			st = &stat{}
			stats[rec.Provider] = st
		}
		st.n++
		if rec.Error != "" {
			// Errors count as zero reward.
			continue
		}
		costNorm := rec.CostUSD / maxCost
		if costNorm > 1 {
			costNorm = 1
		}
		if costNorm < 0 {
			costNorm = 0
		}
		latNorm := float64(rec.DurationMS) / float64(maxLatency)
		if latNorm > 1 {
			latNorm = 1
		}
		if latNorm < 0 {
			latNorm = 0
		}
		r := 1 - costNorm - latNorm
		if r < 0 {
			r = 0
		}
		if r > 1 {
			r = 1
		}
		st.sum += r
	}

	if total == 0 {
		return chain
	}

	lnTotal := math.Log(float64(total))
	if lnTotal < 0 {
		lnTotal = 0
	}

	// Score each provider in the chain.
	type ranked struct {
		p   Provider
		idx int     // input position — preserved as a stable tiebreaker
		ucb float64
	}
	out := make([]ranked, len(chain))
	for i, p := range chain {
		st := stats[p.Name()]
		if st == nil || st.n == 0 {
			// Untried in this window: single exploration boost so it
			// still has a shot at the top of the chain.
			out[i] = ranked{p: p, idx: i, ucb: explore * math.Sqrt(2*lnTotal)}
			continue
		}
		mean := st.sum / float64(st.n)
		ucb := mean + explore*math.Sqrt(2*lnTotal/float64(st.n))
		out[i] = ranked{p: p, idx: i, ucb: ucb}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ucb == out[j].ucb {
			return out[i].idx < out[j].idx
		}
		return out[i].ucb > out[j].ucb
	})

	result := make([]Provider, len(out))
	for i, r := range out {
		result[i] = r.p
	}
	return result
}
