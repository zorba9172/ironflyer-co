package profitguard

import (
	"context"
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

// Guard is the runtime-facing surface of Profit Guard. Every
// enforcement point in the orchestrator calls Decide before doing
// expensive work, and Record after it has acted on the verdict so
// the audit trail captures both the inputs and the outcome.
//
// Decide is a pure function of (Policy, EnforcementPoint, ExecState)
// — it never reads the store, never queries the wallet, and never
// hits a provider. This is what makes the layer cheap enough to run
// on every step and what makes the algorithm reviewable in isolation.
type Guard interface {
	Decide(ctx context.Context, point EnforcementPoint, state ExecState) (Decision, error)
	Record(ctx context.Context, executionID string, point EnforcementPoint, decision Decision, state ExecState) error
}

// New constructs a Guard with the given Policy and DecisionStore.
// Passing a nil store is legal — Record becomes a no-op so Decide
// can be wired in environments (smoke tests, simulators) where
// persistence is not desired.
func New(policy Policy, store DecisionStore) Guard {
	return &guard{policy: policy, store: store}
}

type guard struct {
	policy Policy
	store  DecisionStore
}

// Decide implements the V22 ProfitGuard policy. The algorithm is
// strictly ordered — each step short-circuits on the first hit so the
// hottest-spending failure modes (stop-loss, budget exhaustion) are
// always reached first regardless of the rest of the inputs:
//
//   1. Stop-loss: spent + reserved + estStep > stopLoss → Stop
//   2. Budget:    spent + reserved + estStep > userBudget → PauseForBudget
//   3. Margin:    margin < minimumForWorkload →
//                   a. blueprint available           → ReuseBlueprint
//                   b. repair available + retry/model → ReuseRepair
//                   c. cheaper-but-quality provider  → SwitchProvider
//                   d. degrade legal for point       → Degrade
//                   e. otherwise                     → Stop
//   4. ROI:       completion_per_dollar < floor AND retry/premium/verify
//                                                    → KillBranch
//   5. Risk:      risk > ceiling AND deploy/mobile   → Stop
//   6. otherwise                                     → Continue
//
// Numbered to match the spec verbatim — do not reorder without
// updating the package doc and the audit dashboard.
func (g *guard) Decide(_ context.Context, point EnforcementPoint, state ExecState) (Decision, error) {
	if err := validate(state); err != nil {
		return Decision{}, err
	}

	estStep := nonNegative(state.EstimatedNextStepCostUSD)
	spent := nonNegative(state.SpentUSD)
	reserved := nonNegative(state.ReservedUSD)
	committed := spent.Add(reserved).Add(estStep)

	// (1) Stop-loss circuit breaker. Strictly the highest priority:
	// no economic reason can justify exceeding the per-execution
	// stop-loss once it is set.
	if state.StopLossUSD.IsPositive() && committed.GreaterThan(state.StopLossUSD) {
		return Decision{
			Action: Stop,
			Reason: fmt.Sprintf("stop_loss: committed=%s > stop_loss=%s",
				committed.String(), state.StopLossUSD.String()),
		}, nil
	}

	// (2) Wallet budget exhaustion. Pause rather than Stop so the
	// user can top up and resume — the execution is otherwise
	// healthy.
	if state.UserBudgetUSD.IsPositive() && committed.GreaterThan(state.UserBudgetUSD) {
		return Decision{
			Action: PauseForBudget,
			Reason: fmt.Sprintf("budget_exhausted: committed=%s > user_budget=%s",
				committed.String(), state.UserBudgetUSD.String()),
		}, nil
	}

	// (2.5) Supply-side per-step cap. Defense in depth: even when
	// margin still pencils out, refuse any single step whose
	// projected cost blows past the workload's hard ceiling. Catches
	// runaway Mac pool builds, Opus chains that estimate huge prompt
	// budgets, and unbounded verification loops BEFORE the resource
	// gets allocated — Stop here is cheaper than recovery later.
	if cap := g.policy.maxNextStepFor(workloadFor(point)); cap > 0 {
		if decimalToFloat(estStep) > cap {
			return Decision{
				Action: Stop,
				Reason: fmt.Sprintf("supply_cap: est_step=$%s > workload_cap=$%.2f (%s)",
					estStep.String(), cap, workloadFor(point)),
			}, nil
		}
	}

	// Compute expected margin against the user's budget as estimated
	// revenue. We use UserBudgetUSD because that is what the user
	// has authorised us to bill at execution commit; EstimatedPlatformCostUSD
	// is the projected total platform cost.
	expectedRevenue := decimalToFloat(state.UserBudgetUSD)
	expectedCost := decimalToFloat(state.EstimatedPlatformCostUSD)
	marginPct := computeMarginPct(expectedRevenue, expectedCost)
	minimumMargin := g.policy.minimumMarginFor(state, workloadFor(point))

	// (3) Expected margin below the per-workload floor. Try every
	// rescue path before giving up and Stopping.
	if expectedRevenue > 0 && marginPct < minimumMargin {
		// (3a) Reuse a known-good blueprint.
		if state.SimilarBlueprintAvailable {
			return Decision{
				Action:            ReuseBlueprint,
				Reason:            fmt.Sprintf("margin %.2f%% < floor %.2f%%; blueprint reuse available", marginPct, minimumMargin),
				ExpectedMarginPct: marginPct,
			}, nil
		}
		// (3b) Replay a repair recipe. Only meaningful for retry
		// loops and model calls — a deploy gate does not "retry"
		// in the repair sense.
		if state.SimilarRepairAvailable && (point == BeforeRetryLoop || point == BeforeModelCall) {
			return Decision{
				Action:            ReuseRepair,
				Reason:            fmt.Sprintf("margin %.2f%% < floor %.2f%%; repair recipe available", marginPct, minimumMargin),
				ExpectedMarginPct: marginPct,
			}, nil
		}
		// (3c) Switch to a cheaper provider that still meets quality.
		if cand, ok := pickCheaperProvider(state); ok {
			return Decision{
				Action:              SwitchProvider,
				Reason:              fmt.Sprintf("margin %.2f%% < floor %.2f%%; switching to %s", marginPct, minimumMargin, cand.Name),
				ExpectedMarginPct:   marginPct,
				RecommendedProvider: cand.Name,
			}, nil
		}
		// (3d) Drop a model tier where the runtime supports it.
		if point == BeforePremiumReasoning || point == BeforeModelCall {
			return Decision{
				Action:                   Degrade,
				Reason:                   fmt.Sprintf("margin %.2f%% < floor %.2f%%; degrading model tier", marginPct, minimumMargin),
				ExpectedMarginPct:        marginPct,
				ShouldDowngradeModelTier: true,
			}, nil
		}
		// (3e) Out of rescue options — Stop.
		return Decision{
			Action:            Stop,
			Reason:            fmt.Sprintf("margin %.2f%% < floor %.2f%%; no rescue path available", marginPct, minimumMargin),
			ExpectedMarginPct: marginPct,
		}, nil
	}

	// (4) ROI — completion-per-dollar floor. Only fires on retry,
	// premium, and long-verification branches where we can give up a
	// branch without killing the whole execution.
	if estStep.IsPositive() {
		cpd := state.ExpectedCompletionDelta / decimalToFloat(estStep)
		if cpd < g.policy.CompletionPerDollarFloor &&
			(point == BeforeRetryLoop || point == BeforePremiumReasoning || point == BeforeLongVerification) {
			return Decision{
				Action: KillBranch,
				Reason: fmt.Sprintf("completion_per_dollar %.4f < floor %.4f",
					cpd, g.policy.CompletionPerDollarFloor),
				ExpectedMarginPct: marginPct,
			}, nil
		}
	}

	// (5) Risk ceiling — only deploy / mobile build actions look at it.
	if state.RiskScore > g.policy.RiskCeilingDefault &&
		(point == BeforeVercelDeploy || point == BeforeMobileBuild) {
		return Decision{
			Action: Stop,
			Reason: fmt.Sprintf("risk %.4f > ceiling %.4f for %s",
				state.RiskScore, g.policy.RiskCeilingDefault, point),
			ExpectedMarginPct: marginPct,
		}, nil
	}

	// (6) Default: continue.
	return Decision{
		Action:            Continue,
		Reason:            "ok",
		ExpectedMarginPct: marginPct,
	}, nil
}

// Record persists one decision row to the configured store. A nil
// store is intentionally a no-op so simulators / smoke runs can use
// Decide without wiring a backend. Metrics tick regardless.
func (g *guard) Record(ctx context.Context, executionID string, point EnforcementPoint, decision Decision, state ExecState) error {
	observeDecision(point, decision.Action)
	if g.store == nil {
		return nil
	}
	row := RecordedDecision{
		ExecutionID:             executionID,
		EnforcementPoint:        point,
		Decision:                decision.Action,
		Reason:                  decision.Reason,
		SpentUSD:                state.SpentUSD,
		ReservedUSD:             state.ReservedUSD,
		EstimatedStepCostUSD:    state.EstimatedNextStepCostUSD,
		ExpectedCompletionDelta: state.ExpectedCompletionDelta,
		RecommendedProvider:     decision.RecommendedProvider,
		Metadata:                decision.Metadata,
	}
	if !math.IsNaN(decision.ExpectedMarginPct) {
		m := decision.ExpectedMarginPct
		row.ExpectedMarginPct = &m
	}
	if state.RiskScore > 0 {
		r := state.RiskScore
		row.RiskScore = &r
	}
	return g.store.Record(ctx, row)
}

// validate enforces ExecState invariants that Decide depends on.
// Negative monetary amounts and NaN ratios are bugs in the snapshot
// adapter — refuse them loudly rather than feed garbage into the
// algorithm.
func validate(s ExecState) error {
	if s.UserBudgetUSD.IsNegative() {
		return fmt.Errorf("%w: user_budget_usd is negative", ErrInvalidState)
	}
	if s.SpentUSD.IsNegative() {
		return fmt.Errorf("%w: spent_usd is negative", ErrInvalidState)
	}
	if s.ReservedUSD.IsNegative() {
		return fmt.Errorf("%w: reserved_usd is negative", ErrInvalidState)
	}
	if s.EstimatedNextStepCostUSD.IsNegative() {
		return fmt.Errorf("%w: estimated_next_step_cost_usd is negative", ErrInvalidState)
	}
	if s.EstimatedPlatformCostUSD.IsNegative() {
		return fmt.Errorf("%w: estimated_platform_cost_usd is negative", ErrInvalidState)
	}
	if math.IsNaN(s.ExpectedCompletionDelta) || math.IsInf(s.ExpectedCompletionDelta, 0) {
		return fmt.Errorf("%w: expected_completion_delta is NaN/Inf", ErrInvalidState)
	}
	if math.IsNaN(s.RiskScore) || math.IsInf(s.RiskScore, 0) {
		return fmt.Errorf("%w: risk_score is NaN/Inf", ErrInvalidState)
	}
	if math.IsNaN(s.MinimumMarginPct) || math.IsInf(s.MinimumMarginPct, 0) {
		return fmt.Errorf("%w: minimum_margin_pct is NaN/Inf", ErrInvalidState)
	}
	return nil
}

// nonNegative clamps a decimal at zero. Used in the Decide arithmetic
// so a stale snapshot with a tiny negative drift doesn't shift the
// branch boundary.
func nonNegative(d decimal.Decimal) decimal.Decimal {
	if d.IsNegative() {
		return decimal.Zero
	}
	return d
}

// decimalToFloat is a thin wrapper for readability; the underlying
// call is lossy in the last few cents of precision but is only used
// for ratio arithmetic (margin %, completion-per-dollar) where that
// precision is irrelevant.
func decimalToFloat(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

// computeMarginPct returns ((revenue - cost) / revenue) * 100. When
// revenue is zero the margin is undefined — return 0 so the caller's
// `< minimum` comparison short-circuits to "no rescue needed" (no
// revenue → no expected sale → the higher-priority budget check
// will already have fired if cost > 0).
func computeMarginPct(revenue, cost float64) float64 {
	if revenue <= 0 {
		return 0
	}
	return ((revenue - cost) / revenue) * 100
}

// pickCheaperProvider returns the first provider in the candidate
// list that costs less than the current provider AND clears a
// minimum quality bar (0.7 on a [0, 1] scale). The first-fit choice
// is deliberate: the runtime is expected to pre-sort the candidate
// list by preference (cost vs. latency vs. quality), so Decide just
// honours that ordering.
func pickCheaperProvider(state ExecState) (ProviderQuote, bool) {
	if len(state.AvailableProviders) == 0 {
		return ProviderQuote{}, false
	}
	const minQuality = 0.7
	// Reference cost is the estimated next-step cost on the current
	// provider; we accept any candidate that beats it.
	reference := state.EstimatedNextStepCostUSD
	for _, p := range state.AvailableProviders {
		if p.Name == "" || p.Name == state.CurrentProvider {
			continue
		}
		if p.ExpectedQuality < minQuality {
			continue
		}
		if reference.IsPositive() && p.EstimatedCostUSD.GreaterThanOrEqual(reference) {
			continue
		}
		return p, true
	}
	return ProviderQuote{}, false
}

// workloadFor maps an EnforcementPoint to the workload key the
// margin floor map is indexed by. The mapping is intentionally
// conservative — when in doubt we fall back to StandardWeb so the
// runtime never under-protects margin.
func workloadFor(point EnforcementPoint) string {
	switch point {
	case BeforePremiumReasoning:
		return WorkloadPremiumReasoning
	case BeforeSandboxAllocation:
		return WorkloadSandboxRuntime
	case BeforeVercelDeploy:
		return WorkloadVercelPreview
	case BeforeMobileBuild:
		return WorkloadMobileBuild
	case BeforeArtifactStore:
		return WorkloadStorage
	case BeforeLongVerification:
		return WorkloadSupportHeavy
	default:
		return WorkloadStandardWeb
	}
}
