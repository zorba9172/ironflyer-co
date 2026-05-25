package profitguard

// Workload* are the canonical workload classifiers that map to the
// per-workload minimum-margin floors defined in
//   docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/
//     01-unit-economics/05-margin-thresholds.md
//
// The runtime tags every ExecState with one of these labels before
// calling Decide (the tag lives in ExecState.MinimumMarginPct as a
// pre-resolved float; the workload constants are the canonical keys
// for Policy.MinimumMarginByWorkload).
const (
	WorkloadStandardWeb          = "standard_web"
	WorkloadPremiumReasoning     = "premium_reasoning"
	WorkloadSandboxRuntime       = "sandbox_runtime"
	WorkloadVercelPreview        = "vercel_preview"
	WorkloadMobileBuild          = "mobile_build"
	WorkloadStorage              = "storage"
	WorkloadEnterpriseGovernance = "enterprise_governance"
	WorkloadSupportHeavy         = "support_heavy"
)

// Policy is the tunable surface of Profit Guard. A Guard is
// constructed with a Policy snapshot at boot; hot-reloading is
// intentionally out of scope (an operator change to margin floors is
// a deploy-grade event because it shifts the unit-economics envelope).
//
// MinimumMarginByWorkload — per-workload minimum gross margin in
// percent (45 means 45%). When ExecState.MinimumMarginPct is non-zero,
// it overrides the map lookup; when both are zero, DefaultMinimumMarginPct
// is used.
//
// RiskCeilingDefault — risk score above which deploy/build actions
// are rejected outright. Range [0, 1].
//
// CompletionPerDollarFloor — minimum (ExpectedCompletionDelta /
// EstimatedNextStepCostUSD) ratio for retry / premium / verification
// branches to be allowed to proceed. Falling below the floor maps
// such branches to KillBranch.
type Policy struct {
	MinimumMarginByWorkload  map[string]float64
	DefaultMinimumMarginPct  float64
	RiskCeilingDefault       float64
	CompletionPerDollarFloor float64
}

// DefaultPolicy returns the V22 launch policy.
//
// Margin floors come straight from 05-margin-thresholds.md. The
// CompletionPerDollarFloor and RiskCeilingDefault values are
// engineering judgement calls anchored to the proof-pack worked
// example in 02-completion-per-dollar.md (0.20 completion / $ on the
// reference run) — we set the floor at half of that so a healthy
// run clears it comfortably while a degenerate retry loop trips it.
func DefaultPolicy() Policy {
	return Policy{
		MinimumMarginByWorkload: map[string]float64{
			WorkloadStandardWeb:          45,
			WorkloadPremiumReasoning:     55,
			WorkloadSandboxRuntime:       45,
			WorkloadVercelPreview:        30,
			WorkloadMobileBuild:          35,
			WorkloadStorage:              50,
			WorkloadEnterpriseGovernance: 70,
			WorkloadSupportHeavy:         60,
		},
		// Default margin floor when the workload tag is missing — the
		// most common workload is StandardWeb, so 45% is the safe
		// fall-through.
		DefaultMinimumMarginPct: 45,
		// Risk ceiling — a deploy / mobile build with >70% projected
		// failure probability is rejected. Calibrated to bite only on
		// known-bad inputs (missing secrets, red CI), not on noisy
		// estimators.
		RiskCeilingDefault: 0.70,
		// Completion-per-dollar floor — see comment above.
		CompletionPerDollarFloor: 0.10,
	}
}

// minimumMarginFor resolves the effective margin floor for the
// current decision: explicit per-execution override > workload map >
// policy default. Callers pass the workload key; ExecState carries
// the override.
func (p Policy) minimumMarginFor(state ExecState, workload string) float64 {
	if state.MinimumMarginPct > 0 {
		return state.MinimumMarginPct
	}
	if v, ok := p.MinimumMarginByWorkload[workload]; ok && v > 0 {
		return v
	}
	if p.DefaultMinimumMarginPct > 0 {
		return p.DefaultMinimumMarginPct
	}
	return 45
}
