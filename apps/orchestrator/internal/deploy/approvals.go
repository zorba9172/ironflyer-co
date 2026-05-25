package deploy

import (
	"strings"
	"time"
)

// normalizeDecision accepts the wire vocabulary the resolver layer
// passes through and reduces it to the canonical "approve"/"reject"
// pair the Service consumes. Anything else is rejected by the
// caller.
func normalizeDecision(in string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "approve", "approved":
		return DecisionApprove, true
	case "reject", "rejected", "deny", "denied":
		return DecisionReject, true
	default:
		return "", false
	}
}

// canPromote returns true when the approval row supports a Promote
// call: status == approved AND not yet expired.
func canPromote(a Approval, now time.Time) bool {
	if a.Status != ApprovalApproved {
		return false
	}
	if !a.ExpiresAt.IsZero() && a.ExpiresAt.Before(now) {
		return false
	}
	return true
}

// pickLatestApproval returns the most-recent approval row from the
// supplied slice, or zero-value when the slice is empty. The Service
// uses this when Promote needs to check whether any approval clears
// the canPromote bar.
func pickLatestApproval(rows []Approval) Approval {
	if len(rows) == 0 {
		return Approval{}
	}
	best := rows[0]
	for _, r := range rows[1:] {
		if r.RequestedAt.After(best.RequestedAt) {
			best = r
		}
	}
	return best
}

// defaultIfZero returns d when d > 0, otherwise the supplied default.
func defaultIfZero(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}
