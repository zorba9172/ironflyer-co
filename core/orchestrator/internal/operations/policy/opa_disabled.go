package policy

import (
	"context"

	"github.com/google/uuid"
)

// DisabledPDP is the no-op PDP used only when Config.Mode == disabled
// AND Config.AllowDisabledMode is true. It always allows. NEVER use
// in production: the constructor in pep.go refuses to instantiate
// this PDP unless the safety latch is explicitly set.
type DisabledPDP struct {
	version string
}

var _ PDP = (*DisabledPDP)(nil)

// NewDisabledPDP returns a PDP that always allows. The version string
// is surfaced in audit so an operator can grep for any decision that
// went through the bypass.
func NewDisabledPDP() *DisabledPDP {
	return &DisabledPDP{version: "pbv_disabled"}
}

// BundleVersion implements PDP.
func (p *DisabledPDP) BundleVersion() string { return p.version }

// Decide implements PDP. Always allow with risk=low and a clear reason.
func (p *DisabledPDP) Decide(_ context.Context, _ DecisionRequest) (Decision, error) {
	return Decision{
		DecisionID:          "pdec_" + uuid.NewString(),
		Effect:              EffectAllow,
		Risk:                RiskLow,
		Reason:              "policy_disabled_bypass",
		TTLSeconds:          0,
		PolicyBundleVersion: p.version,
	}, nil
}
