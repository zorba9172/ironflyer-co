// Package completion implements the per-execution completion scorer.
//
// The completion score is a 0..1 fraction summarising how close the
// execution is to "shippable". It is the numerator of the runtime
// efficiency metric:
//
//	completion_per_dollar = completion_score_delta / execution_cost_usd
//
// ProfitGuard reads the score (and delta) before every expensive call;
// the Profit / Blueprint dashboards aggregate over the recorded history
// to surface completion-per-dollar in steady state.
//
// The score is computed from gate outcomes using a fixed weight table
// (sums to 1.0). Each Score(...) call appends one event to the history
// and returns the new absolute score plus the delta from the previous
// one. The store is append-only; history is the audit trail.
package completion

import (
	"context"
	"time"
)

// GateOutcome is the input the scorer ingests for one gate run.
//
//   - Gate is the gate name (matches the weight table entries).
//   - Passed indicates whether the gate verdict was "pass".
//   - Issues is the issue count reported by the gate (informational —
//     reserved for future weighting; v1 only uses Passed).
//   - CoverageWeight is the optional fraction of the gate this run
//     covered (0..1). v1 treats it as informational; the weight table
//     drives scoring.
type GateOutcome struct {
	Gate           string
	Passed         bool
	Issues         int
	CoverageWeight float64
}

// ScoreEvent is one append-only entry in the per-execution history.
type ScoreEvent struct {
	Gate       string
	Score      float64
	Delta      float64
	RecordedAt time.Time
}

// Scorer is the read/write contract for the completion scorer. The
// Postgres and in-memory implementations both satisfy this interface.
//
//   - Score appends the gate outcome, recomputes the absolute score,
//     and returns (newScore, delta, err). Delta = newScore - previous.
//   - Get returns the most recently observed absolute score for the
//     execution (0 if no events yet).
//   - History returns every recorded event in chronological order.
type Scorer interface {
	Score(ctx context.Context, executionID string, outcome GateOutcome) (newScore float64, delta float64, err error)
	Get(ctx context.Context, executionID string) (float64, error)
	History(ctx context.Context, executionID string) ([]ScoreEvent, error)
}

// GateWeights is the canonical weight table. The weights sum to 1.0;
// the score for an execution is the weighted sum of "pass" outcomes
// (one outcome per gate, latest wins).
//
// If Agent 8 disagrees with this table, tweak it here — every consumer
// reads through GateWeights so adjustments stay in one place.
var GateWeights = map[string]float64{
	"spec":     0.10,
	"ux":       0.10,
	"arch":     0.10,
	"code":     0.30,
	"test":     0.15,
	"security": 0.15,
	"deploy":   0.10,
}

// computeScore is the pure-function score calculator used by every
// backend. It collapses an execution's gate outcome history into a
// single score by keeping the latest outcome per gate and summing
// weight × (passed ? 1 : 0).
//
// Unknown gate names contribute 0; missing gates contribute 0 (the
// execution is incomplete by construction until every gate has passed).
func computeScore(latestByGate map[string]bool) float64 {
	total := 0.0
	for gate, weight := range GateWeights {
		if passed, ok := latestByGate[gate]; ok && passed {
			total += weight
		}
	}
	if total > 1.0 {
		total = 1.0
	}
	if total < 0 {
		total = 0
	}
	return total
}
