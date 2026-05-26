package profitguard

import "github.com/shopspring/decimal"

// ExecState is the read-only snapshot the runtime passes into
// Guard.Decide. The field set mirrors the JSON example in
// 03-profit-guard-policy.md, extended with the provider / blueprint /
// repair signals the algorithm needs to choose among Continue /
// Degrade / SwitchProvider / ReuseBlueprint / ReuseRepair.
//
// Money fields are decimal.Decimal so they round-trip cleanly with
// the wallet + ledger packages. Ratios, percentages, and quality
// scores are plain float64 — they are derived numbers, never directly
// debited.
//
// This struct deliberately lives in the profitguard package (not in
// internal/execution) so the Decide() algorithm stays a pure
// function of inputs and policy. Agent 8 will add the
//   func FromExecutionState(execution.State) profitguard.ExecState
// adapter at the integration layer.
type ExecState struct {
	// ExecutionID is the V22 execution this snapshot describes. Only
	// used for audit (Record) — Decide is pure on inputs.
	ExecutionID string
	// TenantID is the wallet-owning identity. Mirrors execution.TenantID.
	TenantID string

	// UserBudgetUSD is the cap the user agreed to spend on this
	// execution (createPaidExecution input).
	UserBudgetUSD decimal.Decimal
	// SpentUSD is the materialised cost so far (sum of debits in the
	// ledger for this execution).
	SpentUSD decimal.Decimal
	// ReservedUSD is the unspent portion of the wallet hold.
	ReservedUSD decimal.Decimal
	// EstimatedNextStepCostUSD is the projected cost of the next
	// action gated by this Decide call.
	EstimatedNextStepCostUSD decimal.Decimal
	// EstimatedPlatformCostUSD is the projected total platform cost
	// for the remainder of the execution; used to compute expected
	// margin against UserBudgetUSD as estimated revenue.
	EstimatedPlatformCostUSD decimal.Decimal

	// MinimumMarginPct is the workload-specific minimum margin (see
	// Policy.MinimumMarginByWorkload). Zero means "fall back to the
	// policy default". Stored on the snapshot so the runtime can
	// override per-execution (e.g. enterprise SLAs).
	MinimumMarginPct float64

	// ExpectedCompletionDelta is the projected gain in the execution's
	// completion score if this step runs to completion. Range [0, 1].
	ExpectedCompletionDelta float64

	// RiskScore is the failure probability for deploy / build /
	// long-verification actions. Range [0, 1].
	RiskScore float64

	// StopLossUSD is the per-execution circuit breaker. If
	// SpentUSD + ReservedUSD + EstimatedNextStepCostUSD would exceed
	// this number, Decide returns Stop regardless of margin.
	StopLossUSD decimal.Decimal

	// CurrentProvider names the provider that would handle the call
	// if no SwitchProvider is recommended. Empty for non-model
	// enforcement points.
	CurrentProvider string

	// AvailableProviders is the candidate set Decide may swap to.
	// Empty means "no alternative" and disables SwitchProvider.
	AvailableProviders []ProviderQuote

	// SimilarBlueprintAvailable signals that internal/blueprints has
	// a high-confidence reusable blueprint for the current intent.
	// Pre-computed by the caller — Decide never queries blueprints.
	SimilarBlueprintAvailable bool

	// SimilarRepairAvailable signals that internal/repair has a
	// matching repair recipe for the current failure signature.
	SimilarRepairAvailable bool
}

// ProviderQuote describes one candidate provider Decide may switch to.
// EstimatedCostUSD is the projected cost for this exact step (NOT a
// per-token rate); LatencyMS is end-to-end median; ExpectedQuality is
// a normalised score in [0, 1] used to gate "still meets quality bar".
type ProviderQuote struct {
	Name             string
	EstimatedCostUSD decimal.Decimal
	ExpectedQuality  float64
	LatencyMS        int
}
