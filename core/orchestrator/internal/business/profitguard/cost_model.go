package profitguard

// Cost-model hook — additive plumbing so a Guard built via NewWithOptions
// can consult forecast.LearnedCostModel for a per-(tenant, provider,
// capability) cost band before applying its existing static-cost
// algorithm. The Guard interface and the original New / NewWithAuditSink
// constructors are unchanged so the rest of the orchestrator keeps
// compiling. Wireup that wants the learned path replaces New with
// NewWithOptions(...WithCostModel(model), WithCapabilityResolver(fn)).

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/forecast"
)

// CapabilityResolver maps an ExecState + EnforcementPoint into the
// (provider, capability) pair LearnedCostModel keys on. Wireup
// supplies this so the Guard never needs to know about
// providers.Request — the bridge already has it.
//
// Empty provider / capability is legal and signals "no learned
// estimate for this call" — the Guard then keeps its static behaviour.
type CapabilityResolver func(point EnforcementPoint, state ExecState) (provider, capability string)

// Option mutates a guard at construction. NewWithOptions applies each
// in order. The pattern keeps additive plumbing (cost model, audit
// sink, future hooks) decoupled from the base New() signature.
type Option func(*guard)

// WithCostModel attaches a LearnedCostModel that Decide consults
// before the existing static-cost algorithm runs. Predict() falls back
// to the supplied static estimate when the model has fewer than
// MinSamplesForTrust observations, so wiring this is always safe.
func WithCostModel(m *forecast.LearnedCostModel) Option {
	return func(g *guard) { g.costModel = m }
}

// WithCapabilityResolver supplies the (provider, capability) extractor
// the cost model needs. Without it the cost model still works but
// receives an empty capability label — the bridge layer is the only
// place that knows the original providers.Request, so the resolver
// flows in from there.
func WithCapabilityResolver(fn CapabilityResolver) Option {
	return func(g *guard) { g.capResolver = fn }
}

// WithCostModelLogger overrides the logger the cost-model hook uses.
// Defaults to a no-op logger so the additive path doesn't require
// every test boot to wire one.
func WithCostModelLogger(log zerolog.Logger) Option {
	return func(g *guard) { g.costLog = log }
}

// NewWithOptions is the option-style constructor. Equivalent to New
// when no options are supplied.
func NewWithOptions(policy Policy, store DecisionStore, opts ...Option) Guard {
	g := &guard{policy: policy, store: store, costLog: zerolog.Nop()}
	for _, o := range opts {
		if o != nil {
			o(g)
		}
	}
	return g
}

// NewWithAuditSinkAndOptions combines the audit-sink constructor with
// the option list so an operator can wire audit + cost model in one
// call without dropping back to manual struct construction.
func NewWithAuditSinkAndOptions(policy Policy, store DecisionStore, sink AuditSink, opts ...Option) Guard {
	g := &guard{policy: policy, store: store, audit: sink, costLog: zerolog.Nop()}
	for _, o := range opts {
		if o != nil {
			o(g)
		}
	}
	return g
}

// applyCostModel returns the learned estimate + a refusal verdict
// (when the upper bound is unaffordable) for the current call, or
// (state, Decision{}, false) when the model has no useful prediction
// and the existing algorithm should run unchanged.
//
// The Decide path:
//   1. resolves (provider, capability) via the wireup-supplied resolver
//   2. asks the cost model for a prediction, passing the existing
//      static EstimatedNextStepCostUSD as fallback
//   3. when FallbackUsed=false: tightens the state's
//      EstimatedNextStepCostUSD to the learned mean so the downstream
//      margin / supply-cap arithmetic uses the better number
//   4. when the learned upper bound > tenant wallet headroom: returns
//      a Stop verdict immediately so the user is refused before the
//      static algorithm gets to "Continue"
func (g *guard) applyCostModel(_ context.Context, point EnforcementPoint, state ExecState) (ExecState, Decision, bool) {
	if g == nil || g.costModel == nil {
		return state, Decision{}, false
	}
	provider, capability := "", ""
	if g.capResolver != nil {
		provider, capability = g.capResolver(point, state)
	}
	if provider == "" {
		provider = state.CurrentProvider
	}
	if capability == "" {
		capability = "default"
	}
	key := forecast.CostKey{
		TenantID:   state.TenantID,
		Provider:   provider,
		Capability: capability,
	}
	pred := g.costModel.Predict(key, state.EstimatedNextStepCostUSD)
	if pred.FallbackUsed || pred.Confidence < forecast.DefaultConfig().LowConfidenceThreshold {
		// Not enough data — leave state alone so the static algorithm
		// runs with its original estimate.
		return state, Decision{}, false
	}

	// Tighten the snapshot to the learned mean before the algorithm
	// reads it. The arithmetic below (margin, supply cap, completion-
	// per-dollar) now operates on the better number.
	if pred.EstimateUSD.IsPositive() {
		state.EstimatedNextStepCostUSD = pred.EstimateUSD
	}

	// Early refuse — the learned upper bound exceeds the tenant's
	// remaining wallet headroom. Use the same conservative arithmetic
	// the existing stop-loss check uses: spent + reserved + upperBound.
	headroom := state.UserBudgetUSD
	if state.StopLossUSD.IsPositive() && state.StopLossUSD.LessThan(headroom) {
		headroom = state.StopLossUSD
	}
	committedWithUpper := nonNegative(state.SpentUSD).
		Add(nonNegative(state.ReservedUSD)).
		Add(pred.UpperBoundUSD)
	if headroom.IsPositive() && committedWithUpper.GreaterThan(headroom) {
		g.costLog.Info().
			Str("tenant_id", state.TenantID).
			Str("provider", provider).
			Str("capability", capability).
			Str("upper_bound_usd", pred.UpperBoundUSD.String()).
			Str("headroom_usd", headroom.String()).
			Float64("confidence", pred.Confidence).
			Int64("samples", pred.SampleCount).
			Msg("cost_model: learned upper bound exceeds wallet headroom; refusing early")
		return state, Decision{
			Action: PauseForBudget,
			Reason: "learned_cost_upper_bound_exceeds_headroom",
			Metadata: map[string]any{
				"learned_mean_usd":   pred.EstimateUSD.String(),
				"learned_upper_usd":  pred.UpperBoundUSD.String(),
				"learned_confidence": pred.Confidence,
				"sample_count":       pred.SampleCount,
			},
		}, true
	}

	return state, Decision{}, false
}

// Decimal helper used in this file to keep imports honest.
var _ = decimal.Zero
