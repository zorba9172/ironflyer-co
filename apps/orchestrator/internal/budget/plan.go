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
type Plan struct {
	Tier         PlanTier         `json:"tier"`
	Name         string           `json:"name"`
	MonthlyPrice decimal.Decimal  `json:"monthlyPrice"` // gross revenue
	CostCapUSD   decimal.Decimal  `json:"costCapUSD"`   // max provider cost we will absorb
	StripeID     string           `json:"stripeId,omitempty"`
	AllowList    []string         `json:"allowList,omitempty"` // provider names (empty = all)
	BlockList    []string         `json:"blockList,omitempty"`
	HardStop     bool             `json:"hardStop"`            // true = block, false = downgrade
}

// MarginEstimate returns the gross margin if the user consumes exactly the
// cost cap.
func (p Plan) MarginEstimate() decimal.Decimal {
	return p.MonthlyPrice.Sub(p.CostCapUSD)
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
