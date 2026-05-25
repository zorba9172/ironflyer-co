package abuse

// SignalType enumerates the inputs the abuse engine knows how to
// aggregate. New signals are added here and nowhere else so the policy
// bundle and the dashboard can rely on a closed enum. Recording an
// unknown signal type is allowed (stored as-is) but it contributes the
// default weight, not the per-type weight from DefaultWeights.
type SignalType string

const (
	// Identity / commercial signals — typically loaded once per
	// session by the auth layer.
	SignalAccountAge        SignalType = "account_age"
	SignalPaymentState      SignalType = "payment_state"
	SignalChargebackHistory SignalType = "chargeback_history"
	SignalEmailReputation   SignalType = "email_reputation"
	SignalMFAOff            SignalType = "mfa_off"

	// GraphQL / API surface signals — wired from the gqlhardening
	// extensions in this same wave.
	SignalGraphQLVelocity    SignalType = "graphql_velocity"
	SignalSubscriptionFanout SignalType = "subscription_fanout"
	SignalFailedAuth         SignalType = "failed_auth"
	SignalPolicyDeny         SignalType = "policy_deny"
	SignalComplexityReject   SignalType = "complexity_reject"

	// Runtime / provider signals — wired from runtime + the provider
	// router. These tend to carry the highest per-event weight because
	// they correlate with real cost or real damage.
	SignalCommandDeny     SignalType = "command_deny"
	SignalProviderRefusal SignalType = "provider_refusal"
	SignalExpensiveRetry  SignalType = "expensive_retry"
	SignalWalletAbuse     SignalType = "wallet_abuse"
)

// DefaultWeights is the per-signal point cost when the caller does not
// specify a weight explicitly. The numbers were chosen so that a single
// signal of any class lands in TierNormal, a small cluster (3-5) lands
// in TierElevated, and a sustained pattern reaches TierRestricted /
// TierBlocked. They are intentionally conservative — escalate via the
// policy bundle, not by editing this map.
var DefaultWeights = map[SignalType]int{
	SignalAccountAge:         5,
	SignalPaymentState:       10,
	SignalChargebackHistory:  25,
	SignalEmailReputation:    10,
	SignalMFAOff:             5,
	SignalGraphQLVelocity:    8,
	SignalSubscriptionFanout: 12,
	SignalFailedAuth:         15,
	SignalPolicyDeny:         20,
	SignalComplexityReject:   12,
	SignalCommandDeny:        18,
	SignalProviderRefusal:    22,
	SignalExpensiveRetry:     15,
	SignalWalletAbuse:        30,
}

// WeightFor returns the default weight for a signal type, or 5 for
// unknown types so unaccounted-for callers still contribute *some*
// signal to the score.
func WeightFor(st SignalType) int {
	if w, ok := DefaultWeights[st]; ok {
		return w
	}
	return 5
}

// ScoredSignal is the read-side projection of a signal event used by
// dashboards and the Engine.Recent feed.
type ScoredSignal struct {
	TenantID   string
	UserID     string
	Type       SignalType
	Weight     int
	Context    map[string]any
	RecordedAt int64 // unix seconds; the store layer fills this in
}
