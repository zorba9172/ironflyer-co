package forecast

// Variance correction loop — the closed-loop half of the learned cost
// model. Subscribes to two OutcomeEvent streams:
//
//   KindProfitGuardDecision — carries the *estimated* cost that
//     ProfitGuard consulted when it made its verdict.
//   KindProviderChosen      — carries the *actual* cost the provider
//     billed (Billing.Charge → learning.Publish).
//
// For each (execution_id, key) pair we join the estimate and the
// actual, compute variance = (actual - estimated) / estimated, feed
// the actual into LearnedCostModel.Record so the running mean
// tightens, and — when |variance| exceeds VarianceFlagThreshold —
// emit a structured warning + bump a flag counter the GraphQL
// dashboard surfaces so the operator can see drift before users do.
//
// The observer is goroutine-safe and additive: register it via
// learning.Publisher.SetObserver (or a thin fan-out wrapper when the
// boot already has an observer wired). It never blocks the publisher
// — the publisher already invokes observers in a goroutine.

import (
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// VarianceFlagThreshold is the |variance| value above which the
// correction loop logs at warn and bumps the flagged counter. 20 %
// matches the spec — anything more and the static heuristic is
// arguably more accurate than the running mean during the warm-up
// window.
const VarianceFlagThreshold = 0.20

// varianceJoinTTL bounds how long an unmatched estimate row stays in
// the join buffer before we evict it. Real executions land both rows
// within seconds; anything older than this is a missing actual (the
// provider call was Stopped / KillBranch'd before it could bill) and
// the estimate is no longer useful for variance attribution.
const varianceJoinTTL = 10 * time.Minute

// pendingEstimate is one in-flight (execution, key) row waiting on the
// matching KindProviderChosen actual to land.
type pendingEstimate struct {
	Key       CostKey
	Estimated decimal.Decimal
	At        time.Time
}

// VarianceTracker owns the join buffer and the running flag counter.
// It is the runtime half of the correction loop; the model it writes
// into is supplied at construction so the wireup layer chooses the
// lifetime.
type VarianceTracker struct {
	model *LearnedCostModel
	log   zerolog.Logger

	mu       sync.Mutex
	pending  map[string]pendingEstimate // execution_id → estimate row
	flagged  int64                      // count of |variance| > threshold
	observed int64                      // count of joined (est, actual) pairs
	sumAbsPE float64                    // running sum of |actual-est|/est
}

// NewVarianceTracker constructs the tracker pointing at model.
func NewVarianceTracker(model *LearnedCostModel, log zerolog.Logger) *VarianceTracker {
	return &VarianceTracker{
		model:   model,
		log:     log.With().Str("component", "cost_variance").Logger(),
		pending: map[string]pendingEstimate{},
	}
}

// Observe is the learning.Publisher observer hook. Fan it in via
// Publisher.SetObserver or a chain when an observer is already wired
// (see VarianceChainObserver below).
func (t *VarianceTracker) Observe(evt learning.OutcomeEvent) {
	if t == nil {
		return
	}
	switch evt.Kind {
	case learning.KindProfitGuardDecision:
		t.recordEstimate(evt)
	case learning.KindProviderChosen:
		t.recordActual(evt)
	}
}

// MeanAbsPercentError is the running MAPE across every joined (est,
// actual) pair. Exposed for the dashboard. Returns 0 when no pairs
// have been observed yet.
func (t *VarianceTracker) MeanAbsPercentError() float64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.observed == 0 {
		return 0
	}
	return t.sumAbsPE / float64(t.observed)
}

// FlaggedCount returns the running count of joined pairs whose
// absolute variance exceeded VarianceFlagThreshold.
func (t *VarianceTracker) FlaggedCount() int64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.flagged
}

// ObservedCount returns the total number of joined pairs the tracker
// has produced.
func (t *VarianceTracker) ObservedCount() int64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.observed
}

func (t *VarianceTracker) recordEstimate(evt learning.OutcomeEvent) {
	if evt.ExecutionID == "" {
		return
	}
	estUSD, ok := extractEstimatedCost(evt)
	if !ok || !estUSD.IsPositive() {
		return
	}
	key := keyFromEvent(evt)
	now := time.Now().UTC()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.evictExpiredLocked(now)
	t.pending[evt.ExecutionID] = pendingEstimate{
		Key:       key,
		Estimated: estUSD,
		At:        now,
	}
}

func (t *VarianceTracker) recordActual(evt learning.OutcomeEvent) {
	if evt.CostUSD == nil || !evt.CostUSD.IsPositive() {
		return
	}
	actualKey := keyFromEvent(evt)
	// Always feed the model — that's the online learning half of the
	// loop and it doesn't need the estimate join.
	t.model.Record(actualKey, *evt.CostUSD)

	if evt.ExecutionID == "" {
		return
	}
	t.mu.Lock()
	pe, ok := t.pending[evt.ExecutionID]
	if ok {
		delete(t.pending, evt.ExecutionID)
	}
	t.mu.Unlock()
	if !ok || !pe.Estimated.IsPositive() {
		return
	}

	actual, _ := evt.CostUSD.Float64()
	estimated, _ := pe.Estimated.Float64()
	if estimated <= 0 {
		return
	}
	pct := (actual - estimated) / estimated
	abs := math.Abs(pct)

	t.mu.Lock()
	t.observed++
	t.sumAbsPE += abs
	flagged := abs > VarianceFlagThreshold
	if flagged {
		t.flagged++
	}
	t.mu.Unlock()

	if flagged {
		t.log.Warn().
			Str("execution_id", evt.ExecutionID).
			Str("tenant_id", evt.TenantID).
			Str("provider", actualKey.Provider).
			Str("capability", actualKey.Capability).
			Float64("estimated_usd", estimated).
			Float64("actual_usd", actual).
			Float64("variance_pct", pct*100).
			Msg("cost_variance: prediction drift exceeds threshold")
	} else {
		t.log.Debug().
			Str("execution_id", evt.ExecutionID).
			Float64("variance_pct", pct*100).
			Msg("cost_variance: within tolerance")
	}
}

func (t *VarianceTracker) evictExpiredLocked(now time.Time) {
	cutoff := now.Add(-varianceJoinTTL)
	for id, pe := range t.pending {
		if pe.At.Before(cutoff) {
			delete(t.pending, id)
		}
	}
}

// extractEstimatedCost pulls the estimated cost USD value out of a
// KindProfitGuardDecision event. The producer side stamps it under
// Attributes["estimated_step_cost_usd"] (Billing's audit projection)
// or — defensively — the top-level CostUSD field.
func extractEstimatedCost(evt learning.OutcomeEvent) (decimal.Decimal, bool) {
	if v, ok := evt.Attributes["estimated_step_cost_usd"]; ok {
		switch val := v.(type) {
		case string:
			d, err := decimal.NewFromString(val)
			if err == nil {
				return d, true
			}
		case float64:
			return decimal.NewFromFloat(val), true
		case decimal.Decimal:
			return val, true
		case *decimal.Decimal:
			if val != nil {
				return *val, true
			}
		}
	}
	if evt.CostUSD != nil {
		return *evt.CostUSD, true
	}
	return decimal.Zero, false
}

// keyFromEvent normalises an OutcomeEvent into a CostKey. Falls back
// to "default" for missing capability so the join still happens (the
// dashboard surfaces the empty-capability row as its own line so the
// operator can see the gap).
func keyFromEvent(evt learning.OutcomeEvent) CostKey {
	provider := stringAttr(evt, "provider")
	capability := stringAttr(evt, "capability")
	if capability == "" {
		capability = "default"
	}
	return CostKey{
		TenantID:   evt.TenantID,
		Provider:   provider,
		Capability: capability,
	}
}

func stringAttr(evt learning.OutcomeEvent, key string) string {
	if v, ok := evt.Attributes[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if v, ok := evt.Tags[key]; ok {
		return v
	}
	return ""
}

// ChainObservers returns an observer that fans evt out to every
// supplied observer in order. Used so the existing memory-store
// observer and the new variance tracker can coexist on a single
// Publisher.SetObserver slot.
func ChainObservers(observers ...func(learning.OutcomeEvent)) func(learning.OutcomeEvent) {
	cleaned := make([]func(learning.OutcomeEvent), 0, len(observers))
	for _, o := range observers {
		if o != nil {
			cleaned = append(cleaned, o)
		}
	}
	if len(cleaned) == 0 {
		return func(learning.OutcomeEvent) {}
	}
	if len(cleaned) == 1 {
		return cleaned[0]
	}
	return func(evt learning.OutcomeEvent) {
		for _, o := range cleaned {
			func(fn func(learning.OutcomeEvent)) {
				defer func() { _ = recover() }()
				fn(evt)
			}(o)
		}
	}
}
