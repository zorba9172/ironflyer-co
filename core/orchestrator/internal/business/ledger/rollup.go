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

	// Mobile-specific cost buckets. They surface separately on the
	// cost-attribution dashboards so an operator can tell an Android
	// build from a Mac-pool iOS run at a glance — instead of having
	// every mobile minute lumped into "sandbox". MobileCostUSD is the
	// aggregate of all five legs for the "Mobile" summary tile.
	//
	// TODO(graphql): the LedgerRollup type in
	//   core/orchestrator/internal/graph/schema/ledger.graphql
	// and its resolver in
	//   core/orchestrator/internal/graph/resolver/ledger.resolver.go
	// must be extended to expose these fields. Codegen is a separate
	// ticket; until then these legs are computed but not wire-visible.
	MobileBuildCostUSD     decimal.Decimal `json:"mobileBuildCostUSD"`
	EmulatorCostUSD        decimal.Decimal `json:"emulatorCostUSD"`
	MacWorkspaceCostUSD    decimal.Decimal `json:"macWorkspaceCostUSD"`
	EASBuildCostUSD        decimal.Decimal `json:"easBuildCostUSD"`
	AppetizeCostUSD        decimal.Decimal `json:"appetizeCostUSD"`
	MobileCostUSD          decimal.Decimal `json:"mobileCostUSD"`
}

// mobileTotal sums the five mobile-cost legs on a Rollup. Exposed as
// a helper so Build and TenantRollup share the math.
func mobileTotal(r Rollup) decimal.Decimal {
	return decimal.Zero.
		Add(r.MobileBuildCostUSD).
		Add(r.EmulatorCostUSD).
		Add(r.MacWorkspaceCostUSD).
		Add(r.EASBuildCostUSD).
		Add(r.AppetizeCostUSD)
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
		case EntryMobileBuildMin:
			r.MobileBuildCostUSD = r.MobileBuildCostUSD.Add(e.AmountUSD)
		case EntryEmulatorMin:
			r.EmulatorCostUSD = r.EmulatorCostUSD.Add(e.AmountUSD)
		case EntryMacWorkspaceMin:
			r.MacWorkspaceCostUSD = r.MacWorkspaceCostUSD.Add(e.AmountUSD)
		case EntryEASBuildCredit:
			r.EASBuildCostUSD = r.EASBuildCostUSD.Add(e.AmountUSD)
		case EntryAppetizeMin:
			r.AppetizeCostUSD = r.AppetizeCostUSD.Add(e.AmountUSD)
		}
	}
	r.MobileCostUSD = mobileTotal(r)

	allCosts := decimal.Zero.
		Add(r.ProviderCostUSD).
		Add(r.SandboxCostUSD).
		Add(r.StorageCostUSD).
		Add(r.DeploymentCostUSD).
		Add(r.PremiumReasoningCostUSD).
		Add(r.RefundsUSD).
		Add(r.MobileCostUSD)

	if r.RevenueUSD.IsZero() {
		r.GrossMarginPct = decimal.Zero
		return r
	}
	gross := r.RevenueUSD.Sub(allCosts)
	r.GrossMarginPct = gross.Div(r.RevenueUSD).Mul(decimal.NewFromInt(100))
	return r
}
