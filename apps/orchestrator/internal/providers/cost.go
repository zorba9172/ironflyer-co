// Package providers — cost + quote helpers used by ProfitGuard's
// SwitchProvider branch. The Quote API turns the router's live
// provider chain into a [] QuoteEntry the ProfitGuard bridge converts
// to []profitguard.ProviderQuote so the policy can evaluate
// "is there a cheaper provider that still meets the quality bar".
//
// Cost numbers are derived from a small in-package rate table — we
// intentionally do not import internal/budget here to avoid coupling
// the provider router to the wallet / billing surface. The numbers
// below mirror the published list prices in budget.DefaultRateSheet;
// update both when the rate card changes.

package providers

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"
)

// QuoteEntry is one provider's projected cost / quality / latency for
// a single Request. ExpectedQuality is a normalised [0,1] reward EMA
// read from the bandit telemetry when available, otherwise a 0.7
// neutral prior. LatencyMS is the p50 from telemetry, otherwise an
// 800ms default.
type QuoteEntry struct {
	Provider         string
	EstimatedCostUSD decimal.Decimal
	ExpectedQuality  float64
	LatencyMS        int
}

// defaultLatencyMS is the neutral p50 used when no telemetry has been
// observed yet for a provider.
const defaultLatencyMS = 800

// defaultQuality is the neutral [0,1] reward prior used when the
// bandit has no observations for a provider. Matches the warm-start
// reward seed used elsewhere in this package.
const defaultQuality = 0.7

// providerCostRow is the per-provider list-price entry. Prices are
// per 1M tokens, mirroring budget.Rate. We keep a compact rep here so
// the providers package stays self-contained (no budget import).
type providerCostRow struct {
	provider string
	// inputUSDPerMTok and outputUSDPerMTok are USD per 1M tokens. The
	// per-token math runs in decimal so we don't lose precision on
	// small estimates.
	inputUSDPerMTok  float64
	outputUSDPerMTok float64
}

// providerCostTable is the rate sheet keyed by provider Name(). One
// entry per registered provider. Numbers as of early 2026 — keep
// aligned with budget.DefaultRateSheet's most-common model per
// provider.
var providerCostTable = []providerCostRow{
	// Anthropic — Sonnet 4.6 is the default tier.
	{provider: "anthropic", inputUSDPerMTok: 3.00, outputUSDPerMTok: 15.00},
	// OpenAI — gpt-4o is the default tier.
	{provider: "openai", inputUSDPerMTok: 5.00, outputUSDPerMTok: 15.00},
	// Gemini — gemini-2.5-pro is the default tier.
	{provider: "gemini", inputUSDPerMTok: 1.25, outputUSDPerMTok: 5.00},
	// HuggingFace — Llama 3.3 70B reference price.
	{provider: "huggingface", inputUSDPerMTok: 0.90, outputUSDPerMTok: 0.90},
	// DeepSeek — V3 list price.
	{provider: "deepseek", inputUSDPerMTok: 0.27, outputUSDPerMTok: 1.10},
	// Vercel AI Gateway — passthrough; approximate with OpenAI mid tier.
	{provider: "vercel-ai-gateway", inputUSDPerMTok: 5.00, outputUSDPerMTok: 15.00},
	// Mock — costless so dev / smoke runs don't drag the candidate set.
	{provider: "mock", inputUSDPerMTok: 0, outputUSDPerMTok: 0},
}

var providerCostIndex = func() map[string]providerCostRow {
	m := make(map[string]providerCostRow, len(providerCostTable))
	for _, r := range providerCostTable {
		m[strings.ToLower(r.provider)] = r
	}
	return m
}()

// estimateTokenCount returns (in, out) token estimates for a Request.
// Cheap heuristic: charge MaxTokens (or the 4000-token default) for
// both input and output. The Quote consumer (Decide) only uses these
// to rank providers, not to bill — so absolute accuracy is wasted.
func estimateTokenCount(req Request) (in, out int) {
	in = estimateTokens(req.System) + estimateTokens(req.ProjectContext) + estimateTokens(req.Prompt)
	out = req.MaxTokens
	if out <= 0 {
		out = 4000
	}
	if in <= 0 {
		in = out
	}
	return in, out
}

// providerCost returns the USD cost of running `req` on `provider`
// at list rates. Unknown providers return decimal.Zero so they sort
// to the front of a cost-ranked list (Decide will then dismiss them
// because Name == CurrentProvider or quality < bar).
func providerCost(provider string, in, out int) decimal.Decimal {
	row, ok := providerCostIndex[strings.ToLower(provider)]
	if !ok {
		return decimal.Zero
	}
	million := decimal.NewFromInt(1_000_000)
	inCost := decimal.NewFromFloat(row.inputUSDPerMTok).
		Mul(decimal.NewFromInt(int64(in)))
	outCost := decimal.NewFromFloat(row.outputUSDPerMTok).
		Mul(decimal.NewFromInt(int64(out)))
	return inCost.Add(outCost).Div(million)
}

// providerStats aggregates one provider's reward EMA + median latency
// across the last N telemetry rows. Errors clamp reward to 0.
type providerStats struct {
	rewardSum float64
	rewardN   int
	latencies []int64
}

// Quote returns one QuoteEntry per provider eligible for the request's
// capability tags. Cost is computed from the in-package rate sheet at
// list prices, quality from the telemetry-derived reward EMA (or a
// 0.7 prior when there's no signal), and latency from the telemetry
// median (or 800ms when there's no signal).
//
// Order: capability-scored chain order, same as PickChain. The
// ProfitGuard bridge converts the result to []profitguard.ProviderQuote.
func (r *Router) Quote(ctx context.Context, req Request) []QuoteEntry {
	_ = ctx
	chain := r.PickChain(req.Capabilities)
	if len(chain) == 0 {
		return nil
	}

	stats := r.providerStatsFromTelemetry(req.Capabilities)
	inTok, outTok := estimateTokenCount(req)

	out := make([]QuoteEntry, 0, len(chain))
	for _, p := range chain {
		name := p.Name()
		qe := QuoteEntry{
			Provider:         name,
			EstimatedCostUSD: providerCost(name, inTok, outTok),
			ExpectedQuality:  defaultQuality,
			LatencyMS:        defaultLatencyMS,
		}
		if s, ok := stats[name]; ok {
			if s.rewardN > 0 {
				q := s.rewardSum / float64(s.rewardN)
				if q < 0 {
					q = 0
				}
				if q > 1 {
					q = 1
				}
				qe.ExpectedQuality = q
			}
			if len(s.latencies) > 0 {
				qe.LatencyMS = int(medianInt64(s.latencies))
			}
		}
		out = append(out, qe)
	}
	return out
}

// providerStatsFromTelemetry pulls the recent telemetry feed and
// folds it into per-provider reward + latency aggregates. Filters by
// capability overlap when caps is non-empty so the quality signal we
// surface is task-relevant.
func (r *Router) providerStatsFromTelemetry(caps []Capability) map[string]providerStats {
	r.mu.RLock()
	sink := r.tel
	r.mu.RUnlock()
	if sink == nil {
		return nil
	}
	records := sink.Recent(256)
	if len(records) == 0 {
		return nil
	}

	wantSet := make(map[Capability]struct{}, len(caps))
	for _, c := range caps {
		wantSet[c] = struct{}{}
	}

	// Reward normalisation mirrors bandit.Rerank so the quality EMA we
	// surface is on the same scale.
	const (
		maxCostUSD   = 0.10
		maxLatencyMS = 30_000
	)

	out := map[string]providerStats{}
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
		s := out[rec.Provider]
		if rec.Error != "" {
			s.rewardSum += 0
			s.rewardN++
			out[rec.Provider] = s
			continue
		}
		costNorm := rec.CostUSD / maxCostUSD
		if costNorm > 1 {
			costNorm = 1
		}
		if costNorm < 0 {
			costNorm = 0
		}
		latNorm := float64(rec.DurationMS) / float64(maxLatencyMS)
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
		s.rewardSum += r
		s.rewardN++
		if rec.DurationMS > 0 {
			s.latencies = append(s.latencies, rec.DurationMS)
		}
		out[rec.Provider] = s
	}
	return out
}

// medianInt64 returns the median of xs. Mutates the input via a sort.
// Caller passes its own slice so we don't allocate. Empty slice
// returns 0.
func medianInt64(xs []int64) int64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	// Small N — insertion sort is fine and avoids importing sort.
	sortInt64(xs)
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

func sortInt64(xs []int64) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

