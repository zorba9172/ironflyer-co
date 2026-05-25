package abuse

// Tier is the four-band abuse posture the rest of the platform reads
// off the abuse engine. The integer score is the source of truth; the
// tier is a stable label policy and rate-limit code branch on without
// re-deriving thresholds.
type Tier string

const (
	TierNormal     Tier = "normal"     // 0-29
	TierElevated   Tier = "elevated"   // 30-59
	TierRestricted Tier = "restricted" // 60-79
	TierBlocked    Tier = "blocked"    // 80-100
)

// TierFromScore maps a clamped 0..100 score to its band. Numbers
// outside the band are clamped before lookup so callers that bypass
// the engine's clamp still land in a defined tier.
func TierFromScore(score int) Tier {
	switch {
	case score >= 80:
		return TierBlocked
	case score >= 60:
		return TierRestricted
	case score >= 30:
		return TierElevated
	default:
		return TierNormal
	}
}

// ParseTier converts a stored/encoded tier string back to the Tier
// type. Unknown strings collapse to TierNormal + ErrUnknownTier so a
// malformed row never blocks traffic by accident — the safest default
// for trust-plane code is "let the operator know but keep the system
// running".
func ParseTier(s string) (Tier, error) {
	switch Tier(s) {
	case TierNormal, TierElevated, TierRestricted, TierBlocked:
		return Tier(s), nil
	default:
		return TierNormal, ErrUnknownTier
	}
}

// Multiplier is the rate-limit budget scale applied at the gqlhardening
// limiter for a given tier. Normal = 1.0, blocked = 0.0 (full freeze).
// These numbers are also used by ProfitGuard downgrade logic and
// documented in the integration contract — change them together with
// the policy bundle, never in isolation.
func (t Tier) Multiplier() float64 {
	switch t {
	case TierNormal:
		return 1.0
	case TierElevated:
		return 0.5
	case TierRestricted:
		return 0.1
	case TierBlocked:
		return 0.0
	default:
		return 1.0
	}
}

// Score is a clamped 0..100 integer. Helper so call-sites that compose
// signals don't each re-implement the clamp.
type Score int

// ClampScore folds an arbitrary integer into [0,100].
func ClampScore(n int) int {
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
}
