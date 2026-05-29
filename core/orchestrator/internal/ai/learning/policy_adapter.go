package learning

// PolicyAdapter closes the Feedback-Brain loop into ProfitGuard.
//
// The Pattern Miner already turns recent execution outcomes into
// PatternObservations (gate failure rates, blueprint success, repair
// hits — see miner.go). Until now those observations fed the bandit /
// blueprint / forecast strategies but NEVER reached ProfitGuard's
// economic policy: the completion-per-dollar floor was static at boot
// and could not learn.
//
// PolicyAdapter subscribes to the same in-process PatternObservation
// feed the Miner publishes (Publisher.SetPatternObserver) and nudges
// the ProfitGuard completion-per-dollar floor:
//
//   - When a workload's gate-failure-rate is high (the runs that clear
//     the gate are expensive and the system keeps paying for failed
//     attempts), TIGHTEN the floor so marginal retry / premium /
//     verification branches get killed sooner.
//   - When realized completion-per-dollar comes in healthy / gate
//     failure is low, LOOSEN the floor back toward the static default
//     so a transient bad window doesn't permanently over-gate.
//
// Every adjustment is BOUNDED. The floor is clamped to a band around
// the Guard's static default ([default * minFactor, default * maxFactor])
// so a single noisy signal can neither zero-out the gate (which would
// disable ROI enforcement) nor explode it (which would kill every
// branch). Each tick is a small step toward the target, not a jump,
// so the loop is stable.
//
// The adapter is entirely opt-in: if no PolicyTuner or no Publisher is
// wired, it is a no-op. It never blocks the miner and never panics the
// process (the publisher fan-out already recovers).

import (
	"sync"

	"github.com/rs/zerolog"
)

// PolicyTuner is the slice of the ProfitGuard Guard the adapter
// mutates. It is declared HERE (rather than imported from
// business/profitguard) because profitguard already imports this
// learning package (store.go) — importing it back would create a
// cycle. The concrete *guard in business/profitguard satisfies this
// structurally, and main.go bridges the two via
// profitguard.AsPolicyTuner. nil-safe by convention at the call site.
type PolicyTuner interface {
	// DefaultCompletionPerDollarFloor is the static boot value the
	// adapter clamps its adjustments around.
	DefaultCompletionPerDollarFloor() float64
	// CompletionPerDollarFloor is the currently-effective floor.
	CompletionPerDollarFloor() float64
	// SetCompletionPerDollarFloor installs a new effective floor.
	SetCompletionPerDollarFloor(v float64)
}

// PolicyAdapterConfig holds the tunable bounds. Zero values fall back
// to the conservative defaults below.
type PolicyAdapterConfig struct {
	// MinFactor / MaxFactor bound the floor relative to the static
	// default. Defaults: 0.5x .. 2.0x. The floor can therefore never
	// drop below half nor climb above double the boot value.
	MinFactor float64
	MaxFactor float64
	// Step is the fraction of the gap to the target moved per
	// observation (0 < Step <= 1). Default 0.25 — a quarter step keeps
	// the loop smooth and resistant to a single noisy tick.
	Step float64
	// HighFailureRate is the gate-failure-rate (0..1) at or above which
	// the adapter tightens. Default 0.40.
	HighFailureRate float64
	// LowFailureRate is the gate-failure-rate at or below which the
	// adapter loosens back toward default. Default 0.10.
	LowFailureRate float64
}

func (c PolicyAdapterConfig) withDefaults() PolicyAdapterConfig {
	if c.MinFactor <= 0 {
		c.MinFactor = 0.5
	}
	if c.MaxFactor <= 0 {
		c.MaxFactor = 2.0
	}
	if c.MinFactor > c.MaxFactor {
		c.MinFactor, c.MaxFactor = 0.5, 2.0
	}
	if c.Step <= 0 || c.Step > 1 {
		c.Step = 0.25
	}
	if c.HighFailureRate <= 0 || c.HighFailureRate > 1 {
		c.HighFailureRate = 0.40
	}
	if c.LowFailureRate < 0 || c.LowFailureRate >= c.HighFailureRate {
		c.LowFailureRate = 0.10
	}
	return c
}

// PolicyAdapter nudges ProfitGuard policy thresholds from miner output.
type PolicyAdapter struct {
	tuner PolicyTuner
	cfg   PolicyAdapterConfig
	log   zerolog.Logger

	mu sync.Mutex
}

// NewPolicyAdapter wires the adapter to a PolicyTuner (the live
// ProfitGuard). A nil tuner is legal — every method becomes a no-op so
// callers can construct unconditionally at boot.
func NewPolicyAdapter(tuner PolicyTuner, cfg PolicyAdapterConfig, log zerolog.Logger) *PolicyAdapter {
	return &PolicyAdapter{
		tuner: tuner,
		cfg:   cfg.withDefaults(),
		log:   log,
	}
}

// Subscribe returns a callback suitable for Publisher.SetPatternObserver.
// On a nil adapter / nil tuner it returns a no-op closure so the caller
// never has to branch.
func (a *PolicyAdapter) Subscribe() func(PatternObservation) {
	if a == nil || a.tuner == nil {
		return func(PatternObservation) {}
	}
	return func(obs PatternObservation) { a.Apply(obs) }
}

// Apply routes one observation to the policy floor. Returns true when
// it mutated the floor. Safe for concurrent use; the publisher fan-out
// already invokes this off a recover-guarded goroutine.
func (a *PolicyAdapter) Apply(obs PatternObservation) bool {
	if a == nil || a.tuner == nil {
		return false
	}

	// Determine the desired direction + intensity from the observation.
	// We react to two pattern families:
	//   - "gate_failure_rate": high failure → tighten, low → loosen.
	//   - "completion_per_dollar" (future miner signal): realized cpd
	//     far below estimate → tighten.
	tighten := false
	loosen := false
	switch obs.Pattern {
	case "gate_failure_rate":
		rate, ok := floatEvidence(obs.Evidence, "failure_rate")
		if !ok {
			return false
		}
		if rate >= a.cfg.HighFailureRate {
			tighten = true
		} else if rate <= a.cfg.LowFailureRate {
			loosen = true
		} else {
			return false // dead-band: don't churn on middling signals.
		}
	case "completion_per_dollar":
		// Optional richer signal: realized vs estimated. When realized
		// is materially below estimate the floor is too loose.
		realized, rok := floatEvidence(obs.Evidence, "realized")
		estimated, eok := floatEvidence(obs.Evidence, "estimated")
		if rok && eok && estimated > 0 {
			if realized < estimated*0.5 {
				tighten = true
			} else if realized >= estimated {
				loosen = true
			} else {
				return false
			}
		} else {
			return false
		}
	default:
		// Every other pattern (blueprint_success_rate, repair_recipe_hits,
		// provider_margin, …) is owned by other adapters — ignore.
		return false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	def := a.tuner.DefaultCompletionPerDollarFloor()
	if def <= 0 || isBadFloat(def) {
		return false // guard isn't configured with a sane default.
	}
	lo := def * a.cfg.MinFactor
	hi := def * a.cfg.MaxFactor

	cur := a.tuner.CompletionPerDollarFloor()
	if cur <= 0 || isBadFloat(cur) {
		cur = def
	}

	// Target the band edge in the chosen direction; step part-way so a
	// single observation never jumps the floor.
	var target float64
	switch {
	case tighten:
		target = hi
	case loosen:
		target = def // loosening returns toward the static default, not below it.
	default:
		return false
	}

	// Confidence scales the step — a low-confidence observation moves
	// the floor less. Clamp confidence into [0,1] defensively.
	conf := clamp01(obs.Confidence)
	if conf <= 0 {
		conf = 0.5
	}
	next := cur + (target-cur)*a.cfg.Step*conf

	// Hard clamp to the band — the non-negotiable safety net.
	if next < lo {
		next = lo
	}
	if next > hi {
		next = hi
	}
	if isBadFloat(next) || next <= 0 {
		return false
	}
	// No-op when the move is negligible so we don't spam the log / churn
	// the atomic on every tick.
	if absDelta(next, cur) < def*1e-4 {
		return false
	}

	a.tuner.SetCompletionPerDollarFloor(next)
	a.log.Info().
		Str("pattern", obs.Pattern).
		Str("target", obs.Target).
		Float64("default_floor", def).
		Float64("prev_floor", cur).
		Float64("new_floor", next).
		Bool("tighten", tighten).
		Float64("confidence", conf).
		Msg("profitguard policy adapter: completion-per-dollar floor adjusted")
	return true
}

// floatEvidence reads a float64 from an Evidence map, tolerating the
// JSON-decoded float64 path and an int path (the miner emits native
// float64, but cross-process JSON may surface ints).
func floatEvidence(ev map[string]any, key string) (float64, bool) {
	if ev == nil {
		return 0, false
	}
	switch v := ev[key].(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// isBadFloat reports NaN / Inf without pulling math onto the call site.
func isBadFloat(v float64) bool {
	return v != v || v > 1e308 || v < -1e308
}

func absDelta(a, b float64) float64 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}
