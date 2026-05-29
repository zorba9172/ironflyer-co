package costcascade

import (
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// NewTTLRevenueSource wraps a (possibly expensive) revenue fetch in a
// time-bounded cache so the aggression controller can read revenue on the
// hot path — it is consulted on every billed completion — without hitting
// the underlying store each time. fetch runs at most once per ttl; on error
// or before the first successful fetch it returns the last good value (0
// initially, which keeps the controller neutral). Wire it via
// Cascade.WithRevenueSource. ttl <= 0 falls back to 30s.
func NewTTLRevenueSource(ttl time.Duration, fetch func() (float64, error)) func() float64 {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	var (
		mu       sync.Mutex
		cached   float64
		fetched  time.Time
		hasValue bool
	)
	return func() float64 {
		mu.Lock()
		defer mu.Unlock()
		if hasValue && time.Since(fetched) < ttl {
			return cached
		}
		v, err := fetch()
		fetched = time.Now()
		if err != nil {
			return cached // last good value (0 until the first success)
		}
		cached = v
		hasValue = true
		return cached
	}
}

// Aggression is the self-tuning controller from the cost-optimization
// vision: "If AI cost exceeds 20% of revenue the system automatically
// becomes more aggressive with caching, reuse and routing."
//
// It tracks AI provider spend the cascade has observed and compares it to
// revenue (supplied by an optional injected source — typically the wallet
// / vault aggregate). When the ratio breaches the target ceiling it raises
// a level in [0,1] that the classifier consults to bias toward cheaper
// tiers. The level is published to Prometheus so the breach is visible
// even before any behavioural change lands.
//
// Spend is accumulated process-lifetime; the controller is intentionally
// conservative — it never relaxes below the operator-configured floor and
// only tightens. A Reset() is provided for period rollovers.
type Aggression struct {
	mu sync.Mutex

	// target is the ceiling for AI-cost / revenue (default 0.20). Once the
	// observed ratio exceeds it, level climbs from 0 toward 1 linearly,
	// saturating at target*2 (i.e. 40% spend → fully aggressive).
	target float64

	spentUSD float64

	// revenueFn returns the revenue figure the ratio is measured against
	// (cumulative or period — caller's choice; the controller only divides
	// by it). Nil is allowed: with no revenue signal the ratio is 0 and the
	// controller stays neutral, so an unconfigured deployment never
	// degrades quality on a phantom breach.
	revenueFn func() float64
}

// NewAggression builds the controller. A target <= 0 falls back to 0.20.
func NewAggression(target float64) *Aggression {
	if target <= 0 {
		target = 0.20
	}
	return &Aggression{target: target}
}

// WithRevenueSource injects the revenue figure the cost ratio is measured
// against. Returns the controller so it chains with NewAggression.
func (a *Aggression) WithRevenueSource(fn func() float64) *Aggression {
	if a == nil {
		return a
	}
	a.mu.Lock()
	a.revenueFn = fn
	a.mu.Unlock()
	return a
}

// RecordSpend folds one observed provider charge into the accumulator and
// republishes the ratio + level gauges. Best-effort; zero/negative ignored.
func (a *Aggression) RecordSpend(usd float64) {
	if a == nil || usd <= 0 {
		return
	}
	a.mu.Lock()
	a.spentUSD += usd
	ratio, level := a.computeLocked()
	a.mu.Unlock()
	metrics.SetCascadeCostRatio(ratio)
	metrics.SetCascadeAggression(level)
}

// Ratio returns the live AI-cost / revenue fraction (0 when no revenue
// signal is wired).
func (a *Aggression) Ratio() float64 {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	r, _ := a.computeLocked()
	return r
}

// Level returns the current aggression level in [0,1]. 0 = relaxed (ratio
// at or below target), 1 = maximally aggressive (ratio at or above 2×
// target). The classifier multiplies its downgrade willingness by this.
func (a *Aggression) Level() float64 {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, l := a.computeLocked()
	return l
}

// Reset zeroes the spend accumulator. Call at billing-period rollover so a
// new month starts from a clean ratio.
func (a *Aggression) Reset() {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.spentUSD = 0
	a.mu.Unlock()
	metrics.SetCascadeCostRatio(0)
	metrics.SetCascadeAggression(0)
}

// computeLocked derives (ratio, level). Caller holds a.mu.
func (a *Aggression) computeLocked() (ratio, level float64) {
	revenue := 0.0
	if a.revenueFn != nil {
		revenue = a.revenueFn()
	}
	if revenue <= 0 {
		return 0, 0
	}
	ratio = a.spentUSD / revenue
	if ratio <= a.target {
		return ratio, 0
	}
	// Linear ramp from target → 2×target maps to level 0 → 1.
	level = (ratio - a.target) / a.target
	if level > 1 {
		level = 1
	}
	return ratio, level
}
