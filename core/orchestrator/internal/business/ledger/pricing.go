package ledger

import "github.com/shopspring/decimal"

// Unit-cost defaults for the mobile cost categories. The values here
// are first-pass estimates calibrated against publicly-listed 2026
// provider rates (Scaleway M2 Mac mini @ $0.18/hr, EAS production
// builds @ ~$1-3, Appetize.io public tier). They feed two call sites:
//
//  1. ProfitGuard reservation maths — multiply estimated minutes by
//     the per-minute rate, sum the legs, and reserve against the
//     wallet before kicking the underlying provider.
//  2. RecordMobile* helpers — multiply the actual observed minutes by
//     the per-minute rate to get the USD amount that lands in the
//     ledger entry.
//
// Calibrate against the actual provider invoice once we have a month
// of traffic; numbers are first-pass estimates.
var (
	// PriceMobileBuildMinUSD — sandbox compute cost plus android-sdk
	// image storage amortisation, charged per build minute.
	PriceMobileBuildMinUSD = decimal.NewFromFloat(0.002)

	// PriceEmulatorMinUSD — KVM-bound vCPU pinned for the emulator's
	// entire lifetime, charged per emulator minute.
	PriceEmulatorMinUSD = decimal.NewFromFloat(0.005)

	// PriceMacWorkspaceMinUSD — Scaleway M2 Mac mini at $0.18/hr =
	// $0.003/min raw cost; we charge 6x to cover the 24h-minimum
	// amortisation and hit a ~50% margin target.
	PriceMacWorkspaceMinUSD = decimal.NewFromFloat(0.018)

	// PriceEASBuildCreditUSD — EAS production builds bill roughly
	// $1-3 per build; we set $1 as the floor and rely on ProfitGuard
	// reservations to clamp upward when the build profile suggests a
	// longer run.
	PriceEASBuildCreditUSD = decimal.NewFromFloat(1.00)

	// PriceAppetizeMinUSD — Appetize.io public-tier per-min converted
	// from their hourly streaming rate, with margin baked in.
	PriceAppetizeMinUSD = decimal.NewFromFloat(0.05)

	// PriceDeviceCloudMinUSD — Pro-tier device-cloud per-minute cost
	// basis (BrowserStack App Live retails at ~$0.20/min; we negotiate
	// closer to $0.05/min at volume). The user-facing list price runs
	// ~$0.25/min so the platform margin is ~$0.20/min; that delta lands
	// in EntryPlatformMargin at execution commit, not here.
	PriceDeviceCloudMinUSD = decimal.NewFromFloat(0.05)
)

// UnitPriceUSD returns the per-unit price for the given mobile
// category. Returns decimal.Zero for non-mobile categories so callers
// can safely look up by EntryType without a separate type-check.
//
// The mobile unit is "minute" for the Min-suffixed categories and
// "credit" for EntryEASBuildCredit. See the package-level Price*
// vars for the calibration notes.
func UnitPriceUSD(c Category) decimal.Decimal {
	switch c {
	case CategoryMobileBuildMin:
		return PriceMobileBuildMinUSD
	case CategoryEmulatorMin:
		return PriceEmulatorMinUSD
	case CategoryMacWorkspaceMin:
		return PriceMacWorkspaceMinUSD
	case CategoryEASBuildCredit:
		return PriceEASBuildCreditUSD
	case CategoryAppetizeMin:
		return PriceAppetizeMinUSD
	case CategoryDeviceCloudMin:
		return PriceDeviceCloudMinUSD
	}
	return decimal.Zero
}
