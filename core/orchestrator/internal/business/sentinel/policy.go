package sentinel

import (
	"time"

	"github.com/shopspring/decimal"
)

// Policy is the Sentinel tuning surface. The defaults match the
// industry findings that drove this SKU: a hard cap below the
// configured wallet topup tier defaults so the user runs out of
// runway BEFORE the wallet is fully drained; warning thresholds
// follow the four-color ladder.
type Policy struct {
	// HardCapUSD is the project-level dollar ceiling Sentinel
	// projects against. Zero means "use the wallet available balance
	// as the cap" — Sentinel falls back to wallet headroom.
	HardCapUSD decimal.Decimal

	// MinSamplesForConfidence is the number of similar past
	// completions required before the trajectory projection is
	// reported as "high confidence". Under this floor, Sentinel
	// clamps ProjectionConfidenceFrac to 0.25 so the dashboard
	// renders a "low confidence" badge.
	MinSamplesForConfidence int

	// WindowHoursForBurn is how far back Sentinel looks at the
	// ledger to compute BurnRatePerHourUSD. Default 4h — short
	// enough to capture the current intent, long enough to smooth
	// over a single big call.
	WindowHoursForBurn int

	// PremiumLoadingFactor is the safety margin Sentinel adds on top
	// of the actuarial expected cost when pricing an Insured Ship
	// policy. 1.4 = "charge 40% over the expected cost so the
	// insurance book stays profitable in steady state".
	PremiumLoadingFactor float64
}

// DefaultPolicy returns the production defaults. The values are
// deliberately defensive: the 4h burn window stays sensitive to
// trajectory shifts; the 1.4x loading factor matches the published
// model in docs/V22_PLAN.md.
func DefaultPolicy() Policy {
	return Policy{
		HardCapUSD:              decimal.Zero,
		MinSamplesForConfidence: 5,
		WindowHoursForBurn:      4,
		PremiumLoadingFactor:    1.4,
	}
}

// classify maps a (spent, projected, cap) tuple to a warning level.
// The thresholds match the four-color ladder documented in the
// package doc: 0-60 green, 60-80 yellow, 80-95 orange, 95+ red. We
// compare against the projected total — not the current spent — so
// a flat burn rate does not give a false-green when the trajectory
// is already over.
func classify(projected, cap decimal.Decimal) WarningLevel {
	if !cap.IsPositive() {
		return WarnGreen
	}
	ratio, _ := projected.Div(cap).Float64()
	switch {
	case ratio < 0.6:
		return WarnGreen
	case ratio < 0.8:
		return WarnYellow
	case ratio < 0.95:
		return WarnOrange
	default:
		return WarnRed
	}
}

// effectiveCap returns the cap Sentinel projects against. When the
// policy declares an explicit HardCapUSD that wins; otherwise we
// fall back to whatever the caller passed as walletHeadroom. Zero
// fallback is a misconfiguration — Sentinel treats it as "infinite"
// and returns WarnGreen unconditionally rather than divide by zero.
func (p Policy) effectiveCap(walletHeadroom decimal.Decimal) decimal.Decimal {
	if p.HardCapUSD.IsPositive() {
		return p.HardCapUSD
	}
	return walletHeadroom
}

// nowTruncated returns time.Now in UTC truncated to second
// resolution. Sentinel never needs sub-second precision and the
// truncation makes JSON payloads stable for replay testing.
func nowTruncated() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}
