package sentinel

import (
	"context"

	"github.com/shopspring/decimal"
)

// StaticUnderwriter is the boot-time underwriter implementation.
// Until the Feedback Brain has enough completed-project rows to
// drive a real actuarial model, the underwriter returns a flat
// "expected cost = cap × baseFactor" estimate with a fixed sample
// count so the Insured Ship SKU is sellable on day one. baseFactor
// matches the published model in docs/V22_PLAN.md: ~62% of cap is
// the average actual spend across the Ironflyer dataset on shipped
// projects, so pricing on 0.62 keeps the book honest until the
// learned model takes over.
type StaticUnderwriter struct {
	BaseFactor  float64 // 0.0 - 1.0
	SampleCount int
}

// NewStaticUnderwriter wires the defaults.
func NewStaticUnderwriter() *StaticUnderwriter {
	return &StaticUnderwriter{BaseFactor: 0.62, SampleCount: 50}
}

// ExpectedCost returns the deterministic estimate. The tenant and
// projectID are ignored on the static path — every project is
// priced from the same prior.
func (u *StaticUnderwriter) ExpectedCost(_ context.Context, _ /*tenant*/, _ /*projectID*/ string, capUSD decimal.Decimal) (decimal.Decimal, int, error) {
	factor := u.BaseFactor
	if factor <= 0 {
		factor = 0.5
	}
	return capUSD.Mul(decimal.NewFromFloat(factor)), u.SampleCount, nil
}
