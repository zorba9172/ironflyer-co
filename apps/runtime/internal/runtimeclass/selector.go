package runtimeclass

import "context"

// RiskLow / RiskMedium / RiskHigh are the standard risk labels the
// orchestrator hands the selector. They map onto preferred isolation
// strength so the selector does not have to negotiate that mapping
// inline at every call site.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// Selector picks the RuntimeClass for a (tenant, risk) tuple.
type Selector interface {
	Select(ctx context.Context, tenantID string, risk string) string
}

// StandardSelector intersects the tenant policy with the
// risk-preferred class.
type StandardSelector struct {
	policy *Policy
}

// NewSelector builds a Selector backed by the given Policy.
func NewSelector(policy *Policy) *StandardSelector {
	if policy == nil {
		policy = NewPolicy()
	}
	return &StandardSelector{policy: policy}
}

// preferredFor maps a risk label to a preferred isolation order
// (strongest preferred first).
func preferredFor(risk string) []string {
	switch risk {
	case RiskHigh:
		return []string{ClassFirecracker, ClassKata, ClassGVisor, ClassDocker}
	case RiskMedium:
		return []string{ClassKata, ClassGVisor, ClassDocker}
	default:
		return []string{ClassGVisor, ClassDocker}
	}
}

// Select implements Selector. Returns the first preferred class that
// is also in the tenant's allowlist; falls back to the cheapest
// allowlisted class if no preference matches.
func (s *StandardSelector) Select(_ context.Context, tenantID, risk string) string {
	allow := s.policy.AllowedFor(tenantID)
	if len(allow) == 0 {
		return ClassDocker
	}
	allowSet := make(map[string]struct{}, len(allow))
	for _, c := range allow {
		allowSet[c] = struct{}{}
	}
	for _, c := range preferredFor(risk) {
		if _, ok := allowSet[c]; ok {
			return c
		}
	}
	// Cheapest class still in the allowlist.
	for _, c := range AllClasses() {
		if _, ok := allowSet[c]; ok {
			return c
		}
	}
	return allow[0]
}

var _ Selector = (*StandardSelector)(nil)
