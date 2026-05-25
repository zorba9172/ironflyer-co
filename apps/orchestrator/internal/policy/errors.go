package policy

import (
	"errors"
	"fmt"
)

// ErrPolicyDeny is the sentinel returned by PEP.MustAllow when the PDP
// decides EffectDeny. Callers should treat it as a hard failure and
// surface a redacted error to the client (never leak Reason if it
// contains internal policy detail).
var ErrPolicyDeny = errors.New("policy: denied")

// ErrPolicyMisconfigured is returned when the PDP cannot reach its
// backend (bundle load failure, OPA sidecar unreachable) and the
// default-deny safety switch is on.
var ErrPolicyMisconfigured = errors.New("policy: misconfigured")

// DenyError wraps a denied Decision so callers can extract the
// DecisionID / Reason without losing the sentinel for errors.Is.
type DenyError struct {
	Decision Decision
}

// Error formats a redacted message safe to log; for client surfaces
// prefer the Decision's DecisionID alone.
func (e *DenyError) Error() string {
	if e.Decision.Reason == "" {
		return fmt.Sprintf("%s (decision_id=%s)", ErrPolicyDeny.Error(), e.Decision.DecisionID)
	}
	return fmt.Sprintf("%s: %s (decision_id=%s)", ErrPolicyDeny.Error(), e.Decision.Reason, e.Decision.DecisionID)
}

// Unwrap lets errors.Is(err, ErrPolicyDeny) match.
func (e *DenyError) Unwrap() error { return ErrPolicyDeny }
