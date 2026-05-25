package ledger

import "github.com/shopspring/decimal"

// Rollup is the aggregate view of a tenant's ledger over a window.
// It's the data backing both the executive Profit dashboard and the
// per-execution margin breakdown. Every field is expressed in USD
// using shopspring/decimal so margin math never round-trips through
// float64.
//
// Revenue here is the sum of credit entries that represent real money
// in (wallet top-ups). PlatformMargin is the credit booked at
// execution commit. The cost fields are the corresponding debit
// rollups by entry type. GrossMarginPct is computed as
// (revenue - allCosts) / revenue * 100, with a zero-revenue window
// returning a zero percentage instead of a divide-by-zero blow-up.
type Rollup struct {
	RevenueUSD              decimal.Decimal `json:"revenueUSD"`
	ProviderCostUSD         decimal.Decimal `json:"providerCostUSD"`
	SandboxCostUSD          decimal.Decimal `json:"sandboxCostUSD"`
	StorageCostUSD          decimal.Decimal `json:"storageCostUSD"`
	DeploymentCostUSD       decimal.Decimal `json:"deploymentCostUSD"`
	PremiumReasoningCostUSD decimal.Decimal `json:"premiumReasoningCostUSD"`
	RefundsUSD              decimal.Decimal `json:"refundsUSD"`
	PlatformMarginUSD       decimal.Decimal `json:"platformMarginUSD"`
	GrossMarginPct          decimal.Decimal `json:"grossMarginPct"`
}

// Build aggregates a slice of entries into a Rollup. The function is
// pure — it does not touch storage — so the Postgres and Memory
// services can both share it without duplicating the math, and the
// dashboard layer can call it directly on a pre-filtered slice.
//
// Treatment of entry types:
//
//   - wallet_topup           → RevenueUSD (credit)
//   - provider_inference_cost → ProviderCostUSD (debit)
//   - sandbox_cost            → SandboxCostUSD (debit)
//   - storage_cost            → StorageCostUSD (debit)
//   - deployment_cost         → DeploymentCostUSD (debit)
//   - premium_reasoning_charge → PremiumReasoningCostUSD (debit)
//   - refund                  → RefundsUSD (credit)
//   - platform_margin         → PlatformMarginUSD (credit)
//
// credit_reservation and credit_release are accounting holds, not
// revenue or cost — they cancel out at execution commit, so they do
// NOT contribute to the rollup. This keeps GrossMargin honest: only
// real money in vs. real money out.
func Build(entries []Entry) Rollup {
	var r Rollup
	for _, e := range entries {
		switch e.EntryType {
		case EntryWalletTopup:
			r.RevenueUSD = r.RevenueUSD.Add(e.AmountUSD)
		case EntryProviderInferenceCost:
			r.ProviderCostUSD = r.ProviderCostUSD.Add(e.AmountUSD)
		case EntrySandboxCost:
			r.SandboxCostUSD = r.SandboxCostUSD.Add(e.AmountUSD)
		case EntryStorageCost:
			r.StorageCostUSD = r.StorageCostUSD.Add(e.AmountUSD)
		case EntryDeploymentCost:
			r.DeploymentCostUSD = r.DeploymentCostUSD.Add(e.AmountUSD)
		case EntryPremiumReasoningCharge:
			r.PremiumReasoningCostUSD = r.PremiumReasoningCostUSD.Add(e.AmountUSD)
		case EntryRefund:
			r.RefundsUSD = r.RefundsUSD.Add(e.AmountUSD)
		case EntryPlatformMargin:
			r.PlatformMarginUSD = r.PlatformMarginUSD.Add(e.AmountUSD)
		}
	}

	allCosts := decimal.Zero.
		Add(r.ProviderCostUSD).
		Add(r.SandboxCostUSD).
		Add(r.StorageCostUSD).
		Add(r.DeploymentCostUSD).
		Add(r.PremiumReasoningCostUSD).
		Add(r.RefundsUSD)

	if r.RevenueUSD.IsZero() {
		r.GrossMarginPct = decimal.Zero
		return r
	}
	gross := r.RevenueUSD.Sub(allCosts)
	r.GrossMarginPct = gross.Div(r.RevenueUSD).Mul(decimal.NewFromInt(100))
	return r
}
