package profitguard

// Adaptive thresholds — the Feedback Brain closure for ProfitGuard.
//
// ProfitGuard's policy thresholds are STATIC at boot (DefaultPolicy).
// This file makes the completion-per-dollar floor RUNTIME-MUTABLE in a
// thread-safe way so the learning PolicyAdapter can nudge it from
// observed execution outcomes WITHOUT changing the Decide algorithm or
// its default values.
//
// The contract is deliberately conservative:
//   - When nothing ever calls SetCompletionPerDollarFloor, the override
//     pointer stays nil and Decide reads g.policy.CompletionPerDollarFloor
//     exactly as before — behaviour is byte-identical to today.
//   - The override is a single atomic pointer load on the Decide hot
//     path; no lock, no allocation when unset.
//   - The base/default value is never lost — ResetCompletionPerDollarFloor
//     restores the static policy value, and the adapter clamps every
//     write to a bounded band around it so a bad signal can neither
//     zero-out nor explode the floor.

import (
	"sync/atomic"
)

// PolicyTuner is the runtime-tuning surface a Guard exposes so the
// learning loop can adjust mutable thresholds without importing the
// concrete guard type. A Guard from New / NewWithOptions / the audit
// constructors satisfies this. Implementations MUST be safe for
// concurrent use.
//
// The accessor methods let a tuner clamp against the static default
// before writing (so the floor stays in a sane band) and read the
// currently-effective value for logging.
type PolicyTuner interface {
	// DefaultCompletionPerDollarFloor returns the static policy floor
	// the Guard was constructed with — the anchor the adapter clamps
	// its adjustments around.
	DefaultCompletionPerDollarFloor() float64
	// CompletionPerDollarFloor returns the currently-effective floor
	// (the override when set, else the static default).
	CompletionPerDollarFloor() float64
	// SetCompletionPerDollarFloor installs a runtime override for the
	// completion-per-dollar floor. The value is used by Decide on the
	// next call. Callers are expected to clamp; the Guard does not.
	SetCompletionPerDollarFloor(v float64)
	// ResetCompletionPerDollarFloor drops any override so Decide falls
	// back to the static policy default.
	ResetCompletionPerDollarFloor()
}

// cpdOverride is the atomic override slot for the completion-per-dollar
// floor. A nil pointer means "no override — use the static policy
// value", which keeps the Decide path byte-identical when nothing
// adjusts the threshold.
//
// We embed it as a field on guard (see guard struct) rather than a
// package global so multiple Guards in one process (smoke + live) stay
// independent.

// completionPerDollarFloor resolves the effective floor for one Decide
// pass: the runtime override when present, otherwise the static policy
// value. A single atomic load on the hot path.
func (g *guard) completionPerDollarFloor() float64 {
	if v := g.cpdOverride.Load(); v != nil {
		return *v
	}
	return g.policy.CompletionPerDollarFloor
}

// DefaultCompletionPerDollarFloor satisfies PolicyTuner.
func (g *guard) DefaultCompletionPerDollarFloor() float64 {
	return g.policy.CompletionPerDollarFloor
}

// CompletionPerDollarFloor satisfies PolicyTuner.
func (g *guard) CompletionPerDollarFloor() float64 {
	return g.completionPerDollarFloor()
}

// SetCompletionPerDollarFloor satisfies PolicyTuner. A non-finite or
// non-positive value is ignored (a floor of <= 0 would disable the ROI
// gate entirely, which is never a legitimate learned outcome).
func (g *guard) SetCompletionPerDollarFloor(v float64) {
	if v <= 0 || isNaNOrInf(v) {
		return
	}
	g.cpdOverride.Store(&v)
}

// ResetCompletionPerDollarFloor satisfies PolicyTuner.
func (g *guard) ResetCompletionPerDollarFloor() {
	g.cpdOverride.Store((*float64)(nil))
}

// cpdOverrideSlot is the concrete atomic type used by the guard struct.
// Kept as a named alias so the guard struct field reads cleanly.
type cpdOverrideSlot = atomic.Pointer[float64]

// isNaNOrInf is a tiny local guard so adaptive.go does not pull math
// into the hot Decide path (guard.go already imports math, but the
// override setter lives off the hot path).
func isNaNOrInf(v float64) bool {
	return v != v || v > 1e308 || v < -1e308
}

// AsPolicyTuner returns the PolicyTuner view of a Guard when the
// concrete implementation supports runtime tuning, or (nil, false)
// otherwise. The learning PolicyAdapter uses this to stay decoupled
// from the concrete guard type and to no-op when handed a Guard that
// is not tunable.
func AsPolicyTuner(g Guard) (PolicyTuner, bool) {
	t, ok := g.(PolicyTuner)
	return t, ok
}
