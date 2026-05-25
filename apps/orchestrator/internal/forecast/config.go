package forecast

import "time"

// Config tunes the estimator. All thresholds are exported so the V22
// integration agent can override them from main.go without forking the
// package.
type Config struct {
	// PrimaryWindow is the lookback for the tenant-specific
	// percentile pull (defaults to 30 days).
	PrimaryWindow time.Duration
	// FallbackWindow widens the lookback when the primary window
	// produced fewer than MinTenantSamples runs (defaults to 90 days).
	FallbackWindow time.Duration
	// MinTenantSamples is the minimum tenant-specific sample count
	// before the estimator considers the tenant-specific percentiles
	// usable. Below this it falls back to global stats.
	MinTenantSamples int
	// MinGlobalSamples is the minimum global sample count before the
	// estimator trusts global percentiles. Below this it falls back
	// to the capability baseline.
	MinGlobalSamples int
	// SamplesForFullConfidence is the historical sample count at
	// which Confidence saturates at 1.0.
	SamplesForFullConfidence int
	// LowConfidenceThreshold is the Confidence value below which the
	// estimator attaches a "low confidence" caveat.
	LowConfidenceThreshold float64
	// DefaultDurationSec is used when EstimateInput.EstimatedDurationSec
	// is zero or negative.
	DefaultDurationSec int

	// Per-capability $/minute rates used by the baseline estimator
	// when no historical data is available. Operators can tune these
	// independently of the policy floors in profitguard.
	ReasoningRatePerMin float64
	CodeRatePerMin      float64
	CheapRatePerMin     float64
	DefaultRatePerMin   float64

	// BreakdownProvider / Sandbox / Storage / Deploy split the
	// MedianUSD into a per-component breakdown the UI renders. The
	// four ratios must sum to 1.0; the estimator does not enforce
	// the constraint at construction time so an operator can deploy
	// a deliberately skewed split (e.g. 100% sandbox for a sandbox
	// stress test).
	BreakdownProvider float64
	BreakdownSandbox  float64
	BreakdownStorage  float64
	BreakdownDeploy   float64
}

// DefaultConfig returns the V22 launch tuning. The values are
// engineering judgement calls intended to be overridable; they were
// chosen so a healthy run shows a tight band and a sparse / no-history
// run still produces a defensible answer with an honest caveat.
func DefaultConfig() Config {
	return Config{
		PrimaryWindow:            30 * 24 * time.Hour,
		FallbackWindow:           90 * 24 * time.Hour,
		MinTenantSamples:         5,
		MinGlobalSamples:         3,
		SamplesForFullConfidence: 30,
		LowConfidenceThreshold:   0.3,
		DefaultDurationSec:       600,
		ReasoningRatePerMin:      0.50,
		CodeRatePerMin:           0.10,
		CheapRatePerMin:          0.02,
		DefaultRatePerMin:        0.15,
		BreakdownProvider:        0.60,
		BreakdownSandbox:         0.25,
		BreakdownStorage:         0.05,
		BreakdownDeploy:          0.10,
	}
}
