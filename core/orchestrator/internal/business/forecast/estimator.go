package forecast

import (
	"math"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// percentile returns the empirical percentile (0..100) of the sorted
// samples using nearest-rank. Empty input returns decimal.Zero.
func percentile(sortedSamples []decimal.Decimal, p float64) decimal.Decimal {
	n := len(sortedSamples)
	if n == 0 {
		return decimal.Zero
	}
	if p <= 0 {
		return sortedSamples[0]
	}
	if p >= 100 {
		return sortedSamples[n-1]
	}
	// Nearest-rank: idx = ceil(p/100 * n) - 1, clamped.
	idx := int(math.Ceil(p/100.0*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sortedSamples[idx]
}

// computePercentiles sorts samples (a copy is taken so callers can
// keep the input slice ordered however they like) and returns the
// p25 / p50 / p75 / p95 cost band.
func computePercentiles(samples []decimal.Decimal) (p25, p50, p75, p95 decimal.Decimal) {
	if len(samples) == 0 {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero
	}
	sorted := make([]decimal.Decimal, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LessThan(sorted[j]) })
	return percentile(sorted, 25),
		percentile(sorted, 50),
		percentile(sorted, 75),
		percentile(sorted, 95)
}

// confidenceFor maps a sample count to a 0..1 confidence using a
// linear saturation: 0 samples = 0, satAt samples = 1.
func confidenceFor(samples, satAt int) float64 {
	if samples <= 0 {
		return 0
	}
	if satAt <= 0 {
		return 1
	}
	c := float64(samples) / float64(satAt)
	if c > 1 {
		c = 1
	}
	return c
}

// baselineMedianUSD computes the synthetic point estimate used when
// no historical samples are available. It walks the requested
// capabilities, picks the highest $/min rate that applies, and
// multiplies by the duration. This deliberately favours the more
// expensive capability so we never under-promise.
func baselineMedianUSD(caps []string, durationSec int, cfg Config) decimal.Decimal {
	rate := cfg.DefaultRatePerMin
	for _, c := range caps {
		switch strings.ToLower(c) {
		case "reasoning", "thinking", "quality":
			if cfg.ReasoningRatePerMin > rate {
				rate = cfg.ReasoningRatePerMin
			}
		case "code":
			if cfg.CodeRatePerMin > rate {
				rate = cfg.CodeRatePerMin
			}
		case "cheap", "fast", "inline_completion":
			if cfg.CheapRatePerMin > rate {
				rate = cfg.CheapRatePerMin
			}
		}
	}
	mins := float64(durationSec) / 60.0
	return decimal.NewFromFloat(rate * mins)
}

// bandFromMedian widens a synthetic point estimate into a band so the
// UI still has a meaningful low/median/high/p95 to render even when
// the estimator has no history. The multipliers are intentionally
// asymmetric — most uncertainty is on the upside.
func bandFromMedian(median decimal.Decimal) (p25, p50, p75, p95 decimal.Decimal) {
	p50 = median
	p25 = median.Mul(decimal.NewFromFloat(0.60))
	p75 = median.Mul(decimal.NewFromFloat(1.40))
	p95 = median.Mul(decimal.NewFromFloat(2.00))
	return
}

// breakdownFor splits the point estimate into per-component dollar
// figures using the ratios on Config. Returned values sum (within
// decimal rounding) to median.
func breakdownFor(median decimal.Decimal, cfg Config) map[string]decimal.Decimal {
	mul := func(r float64) decimal.Decimal {
		return median.Mul(decimal.NewFromFloat(r)).Round(4)
	}
	return map[string]decimal.Decimal{
		"provider": mul(cfg.BreakdownProvider),
		"sandbox":  mul(cfg.BreakdownSandbox),
		"storage":  mul(cfg.BreakdownStorage),
		"deploy":   mul(cfg.BreakdownDeploy),
	}
}

// caveatFor synthesises the cautionary text the UI renders beneath
// the dollar band. Empty when the estimator is confident.
func caveatFor(in EstimateInput, samples int, confidence float64, cfg Config) string {
	caps := map[string]bool{}
	for _, c := range in.Capabilities {
		caps[strings.ToLower(c)] = true
	}
	if samples == 0 {
		if caps["reasoning"] && caps["thinking"] && caps["quality"] {
			return "Synthetic baseline. Reasoning + thinking + quality stacked — real runs frequently exceed the high band."
		}
		return "No historical runs match this profile yet; the band is a synthetic baseline. Expect wider variance on the first few real executions."
	}
	if confidence < cfg.LowConfidenceThreshold {
		return "Low-confidence estimate based on a small history sample. Treat the high band as a soft ceiling, not a guarantee."
	}
	if caps["reasoning"] && caps["thinking"] && caps["quality"] {
		return "Reasoning + thinking + quality stacked together can blow past the p95 on hard prompts."
	}
	return ""
}

// estimateFromSamples is the path taken when the backend produced a
// non-empty cost sample. The percentiles drive the band; the median
// drives the breakdown; the sample count drives the confidence and
// the caveat.
func estimateFromSamples(in EstimateInput, samples []decimal.Decimal, cfg Config) Estimate {
	p25, p50, p75, p95 := computePercentiles(samples)
	conf := confidenceFor(len(samples), cfg.SamplesForFullConfidence)
	return Estimate{
		LowUSD:      p25,
		MedianUSD:   p50,
		HighUSD:     p75,
		P95USD:      p95,
		Breakdown:   breakdownFor(p50, cfg),
		Confidence:  conf,
		BasedOnRuns: len(samples),
		Caveat:      caveatFor(in, len(samples), conf, cfg),
	}
}

// estimateBaseline is the path taken when the backend has no
// historical samples — either the BlueprintID was unknown, the
// tenant + global windows were empty, or the caller did not supply a
// BlueprintID at all. The band is synthesised from the requested
// capabilities and the duration budget.
func estimateBaseline(in EstimateInput, cfg Config) Estimate {
	dur := in.EstimatedDurationSec
	if dur <= 0 {
		dur = cfg.DefaultDurationSec
	}
	median := baselineMedianUSD(in.Capabilities, dur, cfg)
	p25, p50, p75, p95 := bandFromMedian(median)
	return Estimate{
		LowUSD:      p25.Round(4),
		MedianUSD:   p50.Round(4),
		HighUSD:     p75.Round(4),
		P95USD:      p95.Round(4),
		Breakdown:   breakdownFor(p50, cfg),
		Confidence:  0,
		BasedOnRuns: 0,
		Caveat:      caveatFor(in, 0, 0, cfg),
	}
}
