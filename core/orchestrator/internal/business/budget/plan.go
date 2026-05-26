// Package budget is the financial self-management layer of Ironflyer.
//
//   Subscription (gross revenue)
//     − provider cost (sum of ledger entries)
//     = margin (sent to Vault as company commission)
//
// Plans cap the per-period spend; the Optimizer picks the cheapest provider
// that meets the requested capability tags; the Enforcer blocks calls when
// the user is out of budget (or downgrades to a cheaper model when near it).
package budget

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// PlanTier is a marketing-level identifier (Free/Pro/Team/Enterprise).
type PlanTier string

const (
	TierFree       PlanTier = "free"
	TierPro        PlanTier = "pro"
	TierTeam       PlanTier = "team"
	TierEnterprise PlanTier = "enterprise"
)

// Plan describes what a subscription buys: gross revenue per month and the
// maximum *cost* the user is allowed to incur from providers (the difference
// is our gross margin before infra overhead).
//
// MeteredPriceID is the Stripe Price (recurring, usage_type=metered) that
// charges per-call overage once the user passes CostCapUSD. Free tier never
// overages — HardStop=true blocks the call instead. The price is loaded from
// env (STRIPE_METERED_PRICE_PRO / STRIPE_METERED_PRICE_TEAM) and stitched
// onto the Plan at startup in cmd/orchestrator/main.go. Empty MeteredPriceID
// disables metering for that tier (self-hosted / dev).
type Plan struct {
	Tier           PlanTier        `json:"tier"`
	Name           string          `json:"name"`
	MonthlyPrice   decimal.Decimal `json:"monthlyPrice"` // gross revenue
	CostCapUSD     decimal.Decimal `json:"costCapUSD"`   // max provider cost we will absorb
	StripeID       string          `json:"stripeId,omitempty"`
	MeteredPriceID string          `json:"meteredPriceId,omitempty"` // Stripe usage_type=metered Price ID
	AllowList      []string        `json:"allowList,omitempty"`      // provider names (empty = all)
	BlockList      []string        `json:"blockList,omitempty"`
	HardStop       bool            `json:"hardStop"` // true = block, false = downgrade
}

// MarginEstimate returns the gross margin if the user consumes exactly the
// cost cap.
func (p Plan) MarginEstimate() decimal.Decimal {
	return p.MonthlyPrice.Sub(p.CostCapUSD)
}

// UsagePercent returns the user's % of plan cap used this period
// (0..100+, can exceed 100 when over-cap on soft-stop plans). The web
// dashboard reads this to drive upgrade banners — 80% yellow, 95% inline
// CTA, 100%+ red. The ledger is supplied by the caller so this helper
// stays a pure value method.
func (p Plan) UsagePercent(ctx context.Context, ledger LedgerStore, userID string) (float64, error) {
	if ledger == nil {
		return 0, nil
	}
	spent, err := ledger.SpentByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	if p.CostCapUSD.LessThanOrEqual(decimal.Zero) {
		// Cap of 0 means "no spend is allowed at all". Any positive spend
		// counts as 100%+; otherwise treat as 0% so the UI doesn't divide
		// by zero.
		if spent.GreaterThan(decimal.Zero) {
			return 100, nil
		}
		return 0, nil
	}
	pct := spent.Div(p.CostCapUSD).Mul(decimal.NewFromInt(100))
	f, _ := pct.Float64()
	return f, nil
}

// PeriodResetAt returns the first day of next UTC month — when the user's
// per-period spend ledger rolls over. The dashboard uses it in the
// "wait until <date>" copy on the exhausted banner.
func PeriodResetAt(now time.Time) time.Time {
	t := now.UTC()
	return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

// DefaultPlans is the seed catalogue. Real billing config will load from DB.
//
// Pricing is aligned with the broader vibe-coding market — Base44 enters at
// $20/mo and ladders to $40+ for teams; we match so positioning sits next to
// theirs without arbitrage. Per-tier CostCapUSD is the provider spend we
// absorb before the enforcer downgrades or blocks; subscription − cap is
// the gross margin floor.
func DefaultPlans() []Plan {
	d := decimal.NewFromFloat
	return []Plan{
		{Tier: TierFree, Name: "Free",
			MonthlyPrice: d(0), CostCapUSD: d(0.50), HardStop: true,
			AllowList: []string{"mock", "anthropic"}},
		{Tier: TierPro, Name: "Pro",
			MonthlyPrice: d(20), CostCapUSD: d(5.50), HardStop: false},
		{Tier: TierTeam, Name: "Team",
			MonthlyPrice: d(40), CostCapUSD: d(12.00), HardStop: false},
		{Tier: TierEnterprise, Name: "Enterprise",
			MonthlyPrice: d(0), CostCapUSD: d(80.00), HardStop: false},
	}
}
