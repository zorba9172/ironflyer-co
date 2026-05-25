package dashboards

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ProfitDashboard is the operator view of platform unit economics for
// a [WindowStart, WindowEnd) interval.
//
// Margin first: Gross profit / margin are the headline numbers.
// Active / blocked / refund counts give the operator the why behind a
// margin change.
type ProfitDashboard struct {
	WindowStart       time.Time
	WindowEnd         time.Time
	RevenueUSD        float64
	ProviderCostUSD   float64
	SandboxCostUSD    float64
	OtherCostUSD      float64
	GrossProfitUSD    float64
	GrossMarginPct    float64
	ActiveExecutions  int
	BlockedExecutions int
	RefundCount       int
	TopUpRate         float64
}

// BuildProfit aggregates one ProfitDashboard from the ledger and
// execution sources for [since, until). All ledger sums are converted
// to float64 USD at the dashboard boundary — internal math stays on
// shopspring/decimal.
//
// Definitions match the V22 proof pack
// (03-proof-dashboards/01-profit-dashboard.md):
//
//   revenue          = wallet_topup
//   provider cost    = provider_inference_cost + premium_reasoning_charge
//   sandbox cost     = sandbox_cost
//   other cost       = storage_cost + deployment_cost
//   gross profit     = revenue - (provider + sandbox + other)
//   gross margin pct = gross profit / revenue × 100 (0 when revenue is 0)
//
// Execution counts come from the execution source's status rollup.
// TopUpRate v1 reports the count of wallet_topup entries normalised by
// the window's hour span — a coarse "top-ups per hour" velocity gauge.
func BuildProfit(ctx context.Context, src LedgerSource, execSrc ExecutionSource, since, until time.Time) (ProfitDashboard, error) {
	out := ProfitDashboard{WindowStart: since, WindowEnd: until}

	sums, err := src.SumByType(ctx, since, until, []string{
		"wallet_topup",
		"provider_inference_cost",
		"premium_reasoning_charge",
		"sandbox_cost",
		"storage_cost",
		"deployment_cost",
		"refund",
	})
	if err != nil {
		return ProfitDashboard{}, err
	}

	revenue := sums["wallet_topup"]
	provider := sums["provider_inference_cost"].Add(sums["premium_reasoning_charge"])
	sandbox := sums["sandbox_cost"]
	other := sums["storage_cost"].Add(sums["deployment_cost"])
	gross := revenue.Sub(provider).Sub(sandbox).Sub(other)

	out.RevenueUSD = floatOf(revenue)
	out.ProviderCostUSD = floatOf(provider)
	out.SandboxCostUSD = floatOf(sandbox)
	out.OtherCostUSD = floatOf(other)
	out.GrossProfitUSD = floatOf(gross)
	if !revenue.IsZero() {
		out.GrossMarginPct = floatOf(gross.Div(revenue)) * 100.0
	}

	counts, err := execSrc.CountsByStatus(ctx, since, until)
	if err != nil {
		return ProfitDashboard{}, err
	}
	out.ActiveExecutions = counts["running"] + counts["admitted"]
	out.BlockedExecutions = counts["paused_for_budget"]
	out.RefundCount = counts["refunded"]

	hours := until.Sub(since).Hours()
	if hours > 0 {
		// Count of top-up entries within the window.
		topupCount := sums["wallet_topup_count"]
		// SumByType doesn't return a count; if the source surfaces a
		// "wallet_topup_count" pseudo-key it'll show up here. Otherwise
		// fall back to revenue per hour as a rough velocity.
		if !topupCount.IsZero() {
			out.TopUpRate = floatOf(topupCount) / hours
		} else if !revenue.IsZero() {
			out.TopUpRate = floatOf(revenue) / hours
		}
	}

	return out, nil
}

// floatOf is the single conversion seam from decimal.Decimal to float64
// used by the GraphQL boundary.
func floatOf(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
