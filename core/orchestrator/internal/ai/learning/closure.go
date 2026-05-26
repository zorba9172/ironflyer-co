package learning

import (
	"context"
	"math"
	"time"

	"github.com/shopspring/decimal"
)

// ExecutionView is the minimal slice of execution state the closure
// calculator needs. We declare a local view instead of importing
// `business/execution` so the learning package can be imported back by
// execution without creating a cycle.
type ExecutionView struct {
	ID              string
	CompletionScore float64
	RevenueUSD      decimal.Decimal
	SpentUSD        decimal.Decimal
	ReservedUSD     decimal.Decimal
}

// ExecutionReader is the bridge interface the closure calculator
// consumes. Implementations live in `business/execution` (a thin
// adapter that maps Execution → ExecutionView).
type ExecutionReader interface {
	GetExecutionView(ctx context.Context, id string) (ExecutionView, error)
}

// GateVerdictReader returns the most recent verdict per gate name for
// an execution. Implementations live in the wow-loop / gates packages;
// the closure compute falls back to a neutral 0.5 when no reader is
// wired.
type GateVerdictReader interface {
	RecentGateVerdicts(ctx context.Context, executionID string) (map[string]string, error)
}

// RuntimeHealthProbe answers "how stable was the runtime over the last
// 5 minutes?" as a [0,1] score. nil-safe.
type RuntimeHealthProbe interface {
	StabilityLast5m(ctx context.Context) (float64, error)
}

// Closure aggregates the four ClosureScore inputs.
type Closure struct {
	exec    ExecutionReader
	gates   GateVerdictReader
	runtime RuntimeHealthProbe
}

// NewClosure constructs the calculator. exec is required; gates and
// runtime are optional — missing readers degrade to neutral inputs.
func NewClosure(exec ExecutionReader, gates GateVerdictReader, runtime RuntimeHealthProbe) *Closure {
	return &Closure{exec: exec, gates: gates, runtime: runtime}
}

// ComputeClosureScore returns the live closure score for executionID.
// Every input is clamped to [0, 1]; Overall is the geometric mean of
// the four so a single zero drags the headline number to zero (which
// is the intent — a run with zero margin health is not shipping even
// if the scope is "complete").
func (c *Closure) ComputeClosureScore(ctx context.Context, executionID string) (ClosureScore, error) {
	if c == nil || c.exec == nil {
		return ClosureScore{ComputedAt: time.Now().UTC()}, nil
	}
	exec, err := c.exec.GetExecutionView(ctx, executionID)
	if err != nil {
		return ClosureScore{ComputedAt: time.Now().UTC()}, err
	}
	scope := clamp01(exec.CompletionScore)
	quality := c.qualityScore(ctx, executionID)
	stability := c.stabilityScore(ctx)
	margin := marginHealth(exec)
	overall := geometricMean(scope, quality, stability, margin)
	return ClosureScore{
		ScopeCompletion:      scope,
		QualityConfidence:    quality,
		IntegrationStability: stability,
		MarginHealth:         margin,
		Overall:              overall,
		ComputedAt:           time.Now().UTC(),
	}, nil
}

func (c *Closure) qualityScore(ctx context.Context, executionID string) float64 {
	if c.gates == nil {
		return 0.5
	}
	verdicts, err := c.gates.RecentGateVerdicts(ctx, executionID)
	if err != nil || len(verdicts) == 0 {
		return 0.5
	}
	// Weighted verdict average — pass=1.0, skip=0.7, block=0.3,
	// fail=0.0. Severity-weighted in spirit (Critical gate failures
	// already dominate via the publisher tagging).
	sum := 0.0
	for _, v := range verdicts {
		switch v {
		case "pass":
			sum += 1.0
		case "skip":
			sum += 0.7
		case "block":
			sum += 0.3
		default:
			// "fail" / unknown
			sum += 0.0
		}
	}
	return clamp01(sum / float64(len(verdicts)))
}

func (c *Closure) stabilityScore(ctx context.Context) float64 {
	if c.runtime == nil {
		return 0.8 // default-optimistic — no probe means no outage signal
	}
	v, err := c.runtime.StabilityLast5m(ctx)
	if err != nil {
		return 0.5
	}
	return clamp01(v)
}

func marginHealth(exec ExecutionView) float64 {
	rev, _ := exec.RevenueUSD.Float64()
	if rev <= 0 {
		// No revenue stamped yet — give the run a benefit-of-the-doubt
		// 0.5 so the geometric mean isn't pinned to zero on a fresh
		// pre-revenue execution.
		return 0.5
	}
	spent, _ := exec.SpentUSD.Float64()
	reserved, _ := exec.ReservedUSD.Float64()
	remaining := math.Max(0, rev-spent-reserved)
	return clamp01(remaining / rev)
}

func geometricMean(vs ...float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	prod := 1.0
	for _, v := range vs {
		if v <= 0 {
			return 0
		}
		prod *= v
	}
	return math.Pow(prod, 1.0/float64(len(vs)))
}
