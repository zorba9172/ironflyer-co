// Package learning is the Feedback Brain — Ironflyer's self-improving
// loop. Every execution beat (gate, patch, repair, provider call,
// blueprint use, profitguard decision, completion update, terminal
// transition) emits a unified OutcomeEvent that flows through Redpanda
// and into ClickHouse facts. A periodic miner reads the facts, surfaces
// weaknesses and pattern observations, and feeds them back into the
// bandit / blueprint / forecast strategies so the AI gets measurably
// better over time.
//
// The package is deliberately additive — every existing call site can
// add a learning.Publish(ctx, evt) without rewriting its transaction
// boundary. The publisher writes through the existing event_outbox
// (Postgres) or directly to Redpanda when no outbox is wired (dev), and
// it never blocks the producer.
package learning

import (
	"time"

	"github.com/shopspring/decimal"
)

// OutcomeKind is the closed taxonomy of OutcomeEvent shapes. Adding a
// new kind requires updating the ClickHouse fact projection too — keep
// these in sync with schema/05_learning.sql.
type OutcomeKind string

const (
	KindExecutionComplete   OutcomeKind = "execution_complete"
	KindGateOutcome         OutcomeKind = "gate_outcome"
	KindPatchApplied        OutcomeKind = "patch_applied"
	KindRepairTriggered     OutcomeKind = "repair_triggered"
	KindProviderChosen      OutcomeKind = "provider_chosen"
	KindBlueprintUsed       OutcomeKind = "blueprint_used"
	KindProfitGuardDecision OutcomeKind = "profitguard_decision"
	KindCompletionScore     OutcomeKind = "completion_score"
	KindPatternObservation  OutcomeKind = "pattern_observation"
)

// OutcomeEvent is the unified shape every learning beat publishes. It
// is intentionally flat so producers and consumers do not need to know
// each other's internal types — Attributes carries the per-Kind body
// (gate name, provider id, patch path set, …) as a JSON-friendly map.
//
// Success / CostUSD / MarginUSD are pointers so the absence of the
// field on a given Kind is unambiguous (vs. a zero meaning "ran for
// free and failed"). Tags piggybacks Prometheus-style labels for
// dashboard cardinality control without inflating Attributes.
type OutcomeEvent struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	ExecutionID string            `json:"execution_id,omitempty"`
	Kind        OutcomeKind       `json:"kind"`
	Timestamp   time.Time         `json:"timestamp"`
	Attributes  map[string]any    `json:"attributes,omitempty"`
	Success     *bool             `json:"success,omitempty"`
	CostUSD     *decimal.Decimal  `json:"cost_usd,omitempty"`
	MarginUSD   *decimal.Decimal  `json:"margin_usd,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// ClosureScore is the live "is this execution actually finished?"
// metric. The four sub-scores live in [0, 1]; Overall is their
// geometric mean so a single low dimension drags the headline
// number — that asymmetry matches operator intuition (a 0.9 scope
// completion with 0.1 quality is *not* shipping).
type ClosureScore struct {
	ScopeCompletion      float64   `json:"scope_completion"`
	QualityConfidence    float64   `json:"quality_confidence"`
	IntegrationStability float64   `json:"integration_stability"`
	MarginHealth         float64   `json:"margin_health"`
	Overall              float64   `json:"overall"`
	ComputedAt           time.Time `json:"computed_at"`
}

// LearningSnapshot is the dashboard projection — high-level signals
// that say "is the system getting smarter?" Counts default to zero
// when ClickHouse is not wired so the GraphQL resolver returns a
// well-formed payload from boot.
type LearningSnapshot struct {
	OutcomeEventsToday     int                `json:"outcome_events_today"`
	OutcomeEventsAllTime   int                `json:"outcome_events_all_time"`
	ReuseRateLast7d        float64            `json:"reuse_rate_last_7d"`
	RepairRecipeHitsLast7d int                `json:"repair_recipe_hits_last_7d"`
	BanditConfidence       float64            `json:"bandit_confidence"`
	BlueprintSuccessRate   map[string]float64 `json:"blueprint_success_rate,omitempty"`
	GateFailureRateLast7d  map[string]float64 `json:"gate_failure_rate_last_7d,omitempty"`
	AverageCompletionScore float64            `json:"average_completion_score"`
	AverageMarginPctLast7d float64            `json:"average_margin_pct_last_7d"`
	LastIndexedAt          *time.Time         `json:"last_indexed_at,omitempty"`
}

// Weakness is a miner-surfaced gap: a gate that fails too often, a
// provider whose realised margin is under prior, a blueprint that
// keeps falling short of completion. Each Weakness names the
// dimension, gives the operator a one-line description, classifies
// severity, and suggests a concrete remediation. Evidence is the
// caller-visible "show your work" — audit IDs, file:line refs, or
// fact row keys.
type Weakness struct {
	Dimension       string   `json:"dimension"`
	Description     string   `json:"description"`
	Severity        string   `json:"severity"`
	SuggestedAction string   `json:"suggested_action"`
	Evidence        []string `json:"evidence,omitempty"`
}

// PatternObservation is what the miner writes back into the outbox.
// Adapters subscribe to these and rewrite the bandit / blueprint /
// forecast knobs accordingly.
type PatternObservation struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	Pattern    string         `json:"pattern"`
	Target     string         `json:"target"`
	Direction  string         `json:"direction"` // "up" | "down" | "neutral"
	Confidence float64        `json:"confidence"`
	Evidence   map[string]any `json:"evidence,omitempty"`
	ObservedAt time.Time      `json:"observed_at"`
}
