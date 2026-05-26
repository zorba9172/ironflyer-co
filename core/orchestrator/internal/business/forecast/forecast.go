// Package forecast answers the "what would this execution cost?"
// question before any wallet hold is placed. It powers the V22
// pre-flight pricing surface so a user can see a defensible cost band
// (low / median / high / p95) before they pay.
//
// The package is intentionally self-contained: it does not import the
// execution, wallet, ledger, or blueprints packages directly. The
// Postgres backend reads from blueprint_runs / executions only through
// the local pgxRowSource seam; the in-memory backend takes injected
// samples. Resolvers depend on the Forecaster interface; the V22
// integration agent picks the concrete backend based on
// IRONFLYER_DB_DRIVER.
package forecast

import (
	"context"

	"github.com/shopspring/decimal"
)

// Forecaster is the surface every cost forecast backend implements.
// Callers (the estimateExecutionCost GraphQL query, the CLI
// pre-flight, future "what-if" panels) treat the returned Estimate as
// advisory — it never reserves wallet funds.
type Forecaster interface {
	// Estimate produces a cost band for the proposed execution. The
	// implementation is allowed to fall back to a baseline when no
	// historical data is available, but MUST set Confidence
	// proportional to the evidence used.
	Estimate(ctx context.Context, in EstimateInput) (Estimate, error)
}

// EstimateInput is the prospective execution as the caller can
// describe it before any wallet hold lands. All fields are optional
// except TenantID — the estimator gracefully degrades to a baseline
// when nothing else is known.
type EstimateInput struct {
	// TenantID anchors the percentile query to one tenant's history.
	// When the tenant has too few samples the estimator falls back to
	// global stats; see DefaultConfig().MinTenantSamples.
	TenantID string

	// BlueprintID, when non-empty and known to blueprint_stats, lets
	// the estimator key its percentile lookup on the blueprint.
	BlueprintID string

	// Capabilities is the set of provider capability hints the caller
	// expects to invoke ("reasoning", "code", "cheap", "thinking",
	// "quality", ...). They drive the baseline rate when no historical
	// data is available, and the caveat heuristic when present.
	Capabilities []string

	// EstimatedDurationSec is the user-side wall-clock budget in
	// seconds. Defaults to DefaultConfig().DefaultDurationSec when
	// zero or negative.
	EstimatedDurationSec int

	// PromptSummary is a free-text hint about complexity. Reserved for
	// future use; the current heuristic ignores it.
	PromptSummary string
}

// Estimate is the cost band returned by the forecaster. Money is
// decimal USD; Confidence is a float64 in [0, 1].
type Estimate struct {
	// LowUSD is the p25 of past matching runs (or the baseline lower
	// bound when no history is available).
	LowUSD decimal.Decimal
	// MedianUSD is the p50 / point estimate the UI surfaces first.
	MedianUSD decimal.Decimal
	// HighUSD is the p75 — the "expect this much" upper band.
	HighUSD decimal.Decimal
	// P95USD is the worst-case dollar value the UI should display as
	// the "could spend up to" number.
	P95USD decimal.Decimal
	// Breakdown decomposes MedianUSD into per-component slices
	// ("provider", "sandbox", "storage", "deploy").
	Breakdown map[string]decimal.Decimal
	// Confidence in [0,1] — proportional to the historical sample
	// count that informed the estimate. Less than 0.3 means the band
	// is mostly synthetic; the caveat will say so.
	Confidence float64
	// BasedOnRuns is the number of historical executions the
	// estimator considered. Zero means the estimate is fully
	// synthetic.
	BasedOnRuns int
	// Caveat is a human-readable cautionary line for the UI to render
	// underneath the dollar band. Empty when the estimator is
	// confident.
	Caveat string
}
