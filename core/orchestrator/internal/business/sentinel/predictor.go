package sentinel

import (
	"context"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// CostSample is one ledger reading the predictor consumes. Adapters
// load these from the existing ledger / metered tables and feed them
// through Predict. Sentinel does NOT import the ledger package — the
// adapter shape lets the wireup keep the storage choice loose.
type CostSample struct {
	At     time.Time
	AmtUSD decimal.Decimal
}

// HistoryQuery is the adapter that loads recent ledger samples for a
// project. Returning samples in ascending time order is required.
type HistoryQuery interface {
	// Samples returns every ledger sample for the given project
	// within the time window. Walletheadroom is the wallet's current
	// AvailableUSD — passed in so Sentinel can fall back to it when
	// the policy declares no explicit HardCap.
	Samples(ctx context.Context, tenant, projectID string, since time.Time) (samples []CostSample, walletHeadroom decimal.Decimal, err error)
}

// CompletionEstimate is the adapter that knows the project's
// estimated remaining work. The forecast multiplies BurnRatePerHour
// by RemainingHours to project ExtrapolatedTotal. When the adapter
// has no estimate (greenfield project) it returns (0, 0.0, false)
// and the predictor uses the burn rate alone — the dashboard renders
// "ETA unknown" but still reports the burn level.
type CompletionEstimate interface {
	Remaining(ctx context.Context, tenant, projectID string) (hours float64, confidence float64, ok bool)
}

// Predictor computes a Forecast over the adapters. It holds no
// state of its own — every call is a pure projection over the most
// recent samples. Concurrency safe by construction.
type Predictor struct {
	policy   Policy
	history  HistoryQuery
	estimate CompletionEstimate
}

// NewPredictor wires the predictor. Either adapter may be nil — a
// nil history means "I have no ledger; assume zero burn"; a nil
// estimate means "I have no completion model; clamp confidence to
// 0.25". This makes the predictor usable from boot before the rest
// of the wireup catches up.
func NewPredictor(policy Policy, history HistoryQuery, estimate CompletionEstimate) *Predictor {
	return &Predictor{policy: policy, history: history, estimate: estimate}
}

// Predict returns the forecast for the project. The algorithm is:
//
//  1. Load samples within the burn window from history.
//  2. Compute SpentUSD = sum of all-time samples (caller-provided).
//     Sentinel does not sum every ledger row here — it relies on the
//     caller passing spentAllTime so the projection can be cheap
//     even on long-running projects.
//  3. BurnRatePerHour = sum(window) / windowHours.
//  4. RemainingHours from the estimate adapter (or fall back to
//     extrapolate-to-cap).
//  5. ExtrapolatedTotal = SpentUSD + BurnRate * RemainingHours.
//  6. Level = classify(ExtrapolatedTotal, cap).
//  7. CapBreachAt = now + (cap - SpentUSD) / BurnRate when burn is
//     positive AND projection crosses cap; otherwise nil.
func (p *Predictor) Predict(ctx context.Context, tenant, projectID string, spentAllTime decimal.Decimal) (Forecast, error) {
	now := nowTruncated()
	since := now.Add(-time.Duration(p.policy.WindowHoursForBurn) * time.Hour)

	var (
		samples  []CostSample
		headroom decimal.Decimal
		queryErr error
	)
	if p.history != nil {
		samples, headroom, queryErr = p.history.Samples(ctx, tenant, projectID, since)
		if queryErr != nil {
			return Forecast{}, queryErr
		}
	}

	burnPerHour := computeBurnRate(samples, p.policy.WindowHoursForBurn)
	cap := p.policy.effectiveCap(headroom)

	var (
		remainingHours float64
		conf           float64
		haveEstimate   bool
	)
	if p.estimate != nil {
		remainingHours, conf, haveEstimate = p.estimate.Remaining(ctx, tenant, projectID)
	}
	if !haveEstimate {
		conf = 0.25
		remainingHours = extrapolateRemainingToCap(spentAllTime, burnPerHour, cap)
	}

	burnFloat, _ := burnPerHour.Float64()
	extrapolated := spentAllTime.Add(burnPerHour.Mul(decimal.NewFromFloat(remainingHours)))
	eta := now.Add(time.Duration(remainingHours * float64(time.Hour)))
	headroomRemaining := cap.Sub(spentAllTime)
	if headroomRemaining.IsNegative() {
		headroomRemaining = decimal.Zero
	}

	var breach *time.Time
	if cap.IsPositive() && burnFloat > 0 && extrapolated.GreaterThan(cap) {
		// Solve: spent + burn * t = cap  ⇒  t = (cap - spent) / burn
		runway := cap.Sub(spentAllTime)
		if runway.IsPositive() {
			runwayHours, _ := runway.Div(burnPerHour).Float64()
			b := now.Add(time.Duration(runwayHours * float64(time.Hour)))
			breach = &b
		}
	}

	return Forecast{
		ProjectID:                projectID,
		TenantID:                 tenant,
		SpentUSD:                 spentAllTime,
		HardCapUSD:               cap,
		BurnRatePerHourUSD:       burnPerHour,
		ExtrapolatedTotalUSD:     extrapolated,
		ETACompletionAt:          eta,
		CapBreachAt:              breach,
		Level:                    classify(extrapolated, cap),
		RemainingHeadroomUSD:     headroomRemaining,
		ProjectionConfidenceFrac: clamp01(conf),
		ComputedAt:               now,
	}, nil
}

// computeBurnRate returns the per-hour burn rate over the window.
// The window length is the *policy* window, not the time span of
// the samples, so a quiet project (zero recent calls) correctly
// reports a zero burn rather than divide-by-zero.
func computeBurnRate(samples []CostSample, windowHours int) decimal.Decimal {
	if len(samples) == 0 || windowHours <= 0 {
		return decimal.Zero
	}
	total := decimal.Zero
	for _, s := range samples {
		total = total.Add(s.AmtUSD)
	}
	return total.Div(decimal.NewFromInt(int64(windowHours)))
}

// extrapolateRemainingToCap returns the runway in hours at the
// current burn rate before the project would touch the cap. Used
// when no completion estimate adapter is available. Returns 24h as
// the fallback when burn is zero so the ETA stays bounded.
func extrapolateRemainingToCap(spent, burn, cap decimal.Decimal) float64 {
	if burn.IsZero() || cap.IsZero() {
		return 24.0
	}
	runway := cap.Sub(spent)
	if runway.IsNegative() {
		return 0.0
	}
	hours, _ := runway.Div(burn).Float64()
	return hours
}

// clamp01 keeps the confidence value inside [0, 1] so a misbehaving
// adapter never poisons the dashboard with -3 or 1.7.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// sortSamplesByTime is a defensive helper for adapters that forget
// the contract. Not called on the hot path; kept here for the wireup
// to use when stitching multiple sources.
func sortSamplesByTime(s []CostSample) {
	sort.Slice(s, func(i, j int) bool { return s[i].At.Before(s[j].At) })
}

// ensure unused-helper suppression in case the wireup never calls it.
var _ = sortSamplesByTime
