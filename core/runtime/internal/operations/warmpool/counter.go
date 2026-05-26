package warmpool

import "math"

// Floor computes the warm-pool floor from a recent paid-arrival rate
// per ARCHITECTURE_RUNTIME_SCALE.md "Maintain a floor from recent
// paid arrival rate, not from total signups". Formula:
//
//	floor = max(MinFloor, ceil(arrivalRatePerMin * TargetSLA / 60))
//	floor = min(floor, MaxFloor)
//
// arrivalRatePerMin must be non-negative; negative inputs are clamped
// to zero so caller bugs do not produce a negative floor.
func Floor(cfg Config, arrivalRatePerMin float64) int {
	if arrivalRatePerMin < 0 {
		arrivalRatePerMin = 0
	}
	min := cfg.MinFloor
	if min < 0 {
		min = 0
	}
	max := cfg.MaxFloor
	if max <= 0 {
		max = 20
	}
	target := cfg.TargetSLAColdStartSeconds
	if target <= 0 {
		target = 5
	}
	// Convert ratePerMin * seconds to expected concurrent warm slots
	// needed to absorb a poisson arrival of that rate within the
	// cold-start window.
	raw := arrivalRatePerMin * float64(target) / 60.0
	desired := int(math.Ceil(raw))
	if desired < min {
		desired = min
	}
	if desired > max {
		desired = max
	}
	return desired
}
