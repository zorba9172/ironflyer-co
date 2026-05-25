package execution

import "github.com/shopspring/decimal"

// CostKind names the bucket an AddCost call lands in. Each kind maps
// to a dedicated NUMERIC column on the `executions` row so that the
// dashboards can break spend down by category without scanning the
// event feed.
type CostKind string

const (
	// CostProvider is upstream model inference cost (Anthropic, OpenAI,
	// Gemini, …). Lands on provider_cost_usd. The BillingGuard charges
	// the wallet in the same step.
	CostProvider CostKind = "provider"

	// CostSandbox is per-tick workspace runtime cost (CPU/RAM seconds,
	// Docker driver overhead). Lands on sandbox_cost_usd.
	CostSandbox CostKind = "sandbox"

	// CostStorage is object-store / large-artifact write cost (S3, R2,
	// MinIO). Lands on storage_cost_usd.
	CostStorage CostKind = "storage"

	// CostDeployment is the deploy-provider bill (Vercel, mobile build,
	// custom domain) for the artifact shipped by this execution.
	// Lands on deployment_cost_usd.
	CostDeployment CostKind = "deployment"

	// CostPremiumReasoning is a synthetic bucket for premium model
	// usage that should be visible separately on the dashboards. The
	// dollar amount is also added to provider_cost_usd by the caller
	// when the cost is double-counted into the underlying provider.
	CostPremiumReasoning CostKind = "premium_reasoning"
)

// columnForCost returns the executions column name that AddCost should
// increment for the given CostKind. Returns the empty string for
// unknown kinds — callers MUST validate first.
func columnForCost(kind CostKind) string {
	switch kind {
	case CostProvider:
		return "provider_cost_usd"
	case CostSandbox:
		return "sandbox_cost_usd"
	case CostStorage:
		return "storage_cost_usd"
	case CostDeployment:
		return "deployment_cost_usd"
	case CostPremiumReasoning:
		// Premium reasoning has no dedicated DB column — the caller
		// adds it to provider_cost_usd alongside the premium flag in
		// the event payload. We still want a stable key for the events
		// stream, hence the explicit case.
		return "provider_cost_usd"
	}
	return ""
}

// computeGrossMargin returns (revenue - spent) / revenue * 100 when
// revenue is positive; otherwise it returns nil. The dashboards rely
// on the nil-vs-zero distinction to surface "no revenue recorded yet"
// instead of mis-labelling pre-revenue runs as 0% margin.
func computeGrossMargin(revenue, spent decimal.Decimal) *decimal.Decimal {
	if !revenue.IsPositive() {
		return nil
	}
	margin := revenue.Sub(spent).Div(revenue).Mul(decimal.NewFromInt(100))
	return &margin
}

// completionPerDollar = (current_score - initial_score) / spent.
// Returns zero when spent is zero (avoids div-by-zero; ProfitGuard
// treats zero as "no data yet"). The result is unitless score-per-USD
// and can be negative if the score regressed.
func completionPerDollar(current, initial float64, spent decimal.Decimal) decimal.Decimal {
	if !spent.IsPositive() {
		return decimal.Zero
	}
	deltaF := current - initial
	delta := decimal.NewFromFloat(deltaF)
	return delta.Div(spent)
}

// budgetRemaining = max(0, budget - spent - reserved). Clamped so the
// dashboards never render negative remaining credit (the wallet hold
// flow ensures we cannot actually overshoot, but rounding artifacts
// near the boundary would otherwise leak through).
func budgetRemaining(budget, spent, reserved decimal.Decimal) decimal.Decimal {
	rem := budget.Sub(spent).Sub(reserved)
	if rem.IsNegative() {
		return decimal.Zero
	}
	return rem
}
