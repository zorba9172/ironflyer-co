package execution

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// Service is the persistence-agnostic contract for the execution
// domain. Both the memory and postgres backends implement it; the
// finisher engine, ProfitGuard, and the GraphQL resolvers depend only
// on this interface.
//
// Money parameters are decimal.Decimal USD. Score parameters are
// float64 in [0, 1]. Status-changing operations MUST honour the FSM
// (see fsm.go) and return ErrIllegalTransition when the move is not
// allowed.
type Service interface {
	// Create inserts a new row in `created` status. No wallet hold is
	// performed — call Admit once ProfitGuard has approved the run.
	// Returns the freshly-inserted row (id + timestamps populated).
	Create(ctx context.Context, input CreateInput) (Execution, error)

	// Admit transitions created → admitted. Caller is expected to
	// have placed the matching wallet hold before invoking this.
	Admit(ctx context.Context, id string) error

	// Start transitions admitted → running and stamps started_at.
	Start(ctx context.Context, id string) error

	// Reserve bumps reserved_usd by `amount` (the per-step reservation
	// against the user's remaining budget). The wallet hold is a
	// separate concern owned by the wallet package.
	Reserve(ctx context.Context, id string, amount decimal.Decimal) error

	// AddCost attributes `amount` USD to the matching column for
	// `kind` AND increments spent_usd by the same amount AND
	// recomputes gross_margin_pct if revenue_usd > 0. The `provider`
	// label flows into the event payload (so the dashboards can break
	// down by provider without a JOIN).
	AddCost(ctx context.Context, id string, kind CostKind, amount decimal.Decimal, provider string) error

	// AddRevenue bumps revenue_usd by `amount` and recomputes
	// gross_margin_pct.
	AddRevenue(ctx context.Context, id string, amount decimal.Decimal) error

	// SetCompletionScore overwrites completion_score with `score`.
	// History flows through execution_events (event type
	// "score_updated").
	SetCompletionScore(ctx context.Context, id string, score float64) error

	// SetExpectation records the ProfitGuard view of the next step:
	// expectedDelta is the score gain we expect, risk is the
	// 0..1 estimate that the step fails outright.
	SetExpectation(ctx context.Context, id string, expectedDelta, risk float64) error

	// Pause transitions running → paused_for_budget. `reason` is a
	// short human-readable tag (e.g. "budget_exhausted").
	Pause(ctx context.Context, id, reason string) error

	// Resume transitions paused_for_budget → running.
	Resume(ctx context.Context, id string) error

	// Succeed marks the execution as terminally complete (the artifact
	// was shipped) and stamps ended_at.
	Succeed(ctx context.Context, id string) error

	// Fail marks the execution as terminally failed and records
	// failure_reason.
	Fail(ctx context.Context, id, reason string) error

	// Stop marks the execution as operator-stopped (UI/CLI or
	// ProfitGuard stop verdict). `reason` lands on failure_reason for
	// uniformity with Fail.
	Stop(ctx context.Context, id, reason string) error

	// Kill marks the execution as hard-killed (kill_branch verdict,
	// SIGTERM). `reason` lands on failure_reason.
	Kill(ctx context.Context, id, reason string) error

	// Refund records the wallet credit-back. Legal from any of the
	// terminal statuses (succeeded/failed/stopped/killed).
	Refund(ctx context.Context, id string, amount decimal.Decimal) error

	// Get returns the row for id, or ErrNotFound.
	Get(ctx context.Context, id string) (Execution, error)

	// GetState returns the row plus the derived ProfitGuard
	// projections (BudgetRemaining, CompletionPerDollar). This is the
	// shape ProfitGuard.Decide consumes.
	GetState(ctx context.Context, id string) (State, error)

	// ListByTenant returns the most recent executions for tenant,
	// newest first, paginated by limit + offset.
	ListByTenant(ctx context.Context, tenant string, limit, offset int) ([]Execution, error)

	// ListByTenantAndProject returns the most recent executions for
	// tenant scoped to projectID, newest first, paginated by limit +
	// offset. Used by the Studio when it routes /p/[projectID] to
	// look up the project's running execution without scanning the
	// full tenant window.
	ListByTenantAndProject(ctx context.Context, tenant, projectID string, limit, offset int) ([]Execution, error)

	// RecordEvent appends a row to execution_events without changing
	// the row status. Useful for ProfitGuard decision audit
	// ("profitguard_decision") and for arbitrary observability events.
	RecordEvent(ctx context.Context, id, eventType string, payload json.RawMessage) error

	// SubscribeEvents returns a channel fed by the in-process broker
	// (memory backend) or by Postgres LISTEN/NOTIFY (postgres backend).
	// The channel closes when ctx is done or the subscription is torn
	// down. Buffer overflow drops events — callers MUST treat the
	// channel as best-effort and backfill via Get/RecordEvent queries
	// when they need a guaranteed view.
	SubscribeEvents(ctx context.Context, id string) (<-chan Event, error)

	// ActiveCount returns the number of executions currently in the
	// running status across every tenant. Powers the Scale dashboard's
	// "active executions" live tile.
	ActiveCount(ctx context.Context) (int, error)

	// QueuedCount returns the number of executions waiting to start:
	// status in (created, admitted). Powers the Scale dashboard's
	// "queued paid executions" tile.
	QueuedCount(ctx context.Context) (int, error)

	// AverageQueueWaitSec returns the mean seconds between created_at
	// and admitted_at across executions admitted on or after `since`.
	// Returns 0 (not an error) when no executions admitted in the
	// window — the dashboard renders 0 as "no recent queue pressure".
	AverageQueueWaitSec(ctx context.Context, since time.Time) (float64, error)

	// LatestSecurityFindings returns the recent security-flavoured
	// event payloads for an execution, newest first, capped at 500.
	// Used by the securityreport.FindingSource adapter to project
	// finisher-gate findings into the customer-visible report.
	//
	// Tolerant by contract: returns an empty slice (not an error)
	// when the execution exists but has no matching events. Returns
	// ErrNotFound only when the execution itself is unknown — and
	// even that may be relaxed to empty by callers that want a
	// degraded-but-non-fatal report path.
	LatestSecurityFindings(ctx context.Context, executionID string) ([]map[string]any, error)

	// GateEventsByExecution returns every gate-verdict event recorded
	// against the execution, oldest first. The wow-loop builder
	// (core/orchestrator/internal/wowloop) consumes these to render
	// the gate report panel.
	//
	// Postgres backend reads execution_events filtered by event_type
	// in the gate.* family (gate.verdict.v1, gate.failed.v1,
	// gate.passed.v1, gate.skipped.v1, gate.repaired.v1). Memory
	// backend has no durable event store; it returns whatever it has
	// captured on its per-execution gate-event ring (populated by the
	// emit path when finisher.recordGateOutcome eventually starts
	// shipping gate.verdict.v1 events through RecordEvent).
	//
	// Tolerant by contract: returns an empty slice (not an error)
	// when the execution exists but has no recorded gate verdicts.
	// Returns ErrNotFound only when the execution itself is unknown
	// — and callers MAY treat that as "no data" rather than fatal.
	//
	// TODO(wave-3): finisher.recordGateOutcome should emit
	// gate.verdict.v1 events into execution_events so this path
	// returns real data even on production. Until that lands the
	// wow-loop gate panel renders as "no data yet".
	GateEventsByExecution(ctx context.Context, executionID string) ([]GateEvent, error)

	// PatchAppliedEventsByExecution returns every patch.applied.v1
	// event recorded against the execution, oldest first. Used by
	// the wow-loop builder's PatchSource adapter to populate the
	// "what changed" panel without having to thread executionID
	// through the patch engine's in-memory store (which today has
	// no per-execution indexing).
	//
	// TODO(wave-3): wire finisher.engine.Apply to emit
	// patch.applied.v1 through RecordEvent on the execution so this
	// path lights up.
	PatchAppliedEventsByExecution(ctx context.Context, executionID string) ([]PatchAppliedEvent, error)

	// PendingRefinements returns refinements that have been recorded
	// against the execution via studio.refine.v1 events but not yet
	// consumed by the finisher loop. Oldest first. Returns an empty
	// slice (not an error) when none are pending. Used by the studio
	// surface and the engine to introspect what's still queued.
	PendingRefinements(ctx context.Context, executionID string) ([]Refinement, error)

	// DrainRefinements atomically claims every pending refinement on
	// the execution and stamps a studio.refine.consumed.v1 event for
	// each one so a subsequent call returns an empty slice. Used by
	// the finisher engine at the top of each gate iteration so a
	// user refinement is folded into the next agent prompt exactly
	// once.
	DrainRefinements(ctx context.Context, executionID string) ([]Refinement, error)

	// RecoveryAttemptsByExecution returns every recovery.recipe_hit
	// or recovery.recipe_applied event recorded against the
	// execution, oldest first. Used by the wow-loop builder's
	// RepairSource adapter to flag a gate stage as "repaired" rather
	// than "failed" when a recipe successfully short-circuited the
	// Coder loop.
	//
	// TODO(wave-3): wire finisher/recovery.go to call RecordEvent
	// with recovery.recipe_hit.v1 / recovery.recipe_applied.v1 so
	// the wow-loop repair panel lights up on production.
	RecoveryAttemptsByExecution(ctx context.Context, executionID string) ([]RecoveryAttempt, error)

	// SetWorkspaceID stamps the runtime sandbox workspace bound to the
	// execution onto the row so downstream surfaces (wow-loop builder,
	// GraphQL Execution.workspaceID) can resolve it without proxying
	// through ProjectID. Called by the finisher engine the moment a
	// workspace is resolved/allocated for the active execution.
	//
	// Idempotent — calling twice with the same value is a no-op
	// success. Calling with a different value overwrites (operator
	// choice; covers the rare workspace-rebinding case without an
	// extra surface). Returns ErrNotFound when the execution row does
	// not exist.
	SetWorkspaceID(ctx context.Context, executionID, workspaceID string) error
}

// GateEvent is one finisher gate verdict as projected from the
// execution_events feed. Status is normalised to the wow-loop
// vocabulary: "pass" | "fail" | "repaired" | "skipped".
type GateEvent struct {
	Gate        string
	Status      string
	IssuesCount int
	OccurredAt  time.Time
}

// PatchAppliedEvent is one patch.applied.v1 record projected from
// the execution_events feed.
type PatchAppliedEvent struct {
	PatchID       string
	AffectedPaths []string
	AppliedAt     time.Time
}

// Refinement is one studio.refine.v1 event projected from the
// execution_events feed for the engine to consume. ConsumedAt is
// nil until DrainRefinements stamps the matching
// studio.refine.consumed.v1 marker.
type Refinement struct {
	ID         string
	Message    string
	QueuedAt   time.Time
	ConsumedAt *time.Time
}

// RecoveryAttempt is one recovery.recipe_hit.v1 or
// recovery.recipe_applied.v1 record projected from execution_events.
type RecoveryAttempt struct {
	FailureSignature string
	Gate             string
	Applied          bool
	Success          bool
	OccurredAt       time.Time
}

// Closed sets of event types the wow-loop adapter reads. Exported so
// future emitters (finisher, recovery) can stay in lockstep without
// hard-coding the literals on the publisher side.
const (
	EventGateVerdictV1   = "gate.verdict.v1"
	EventGateFailedV1    = "gate.failed.v1"
	EventGatePassedV1    = "gate.passed.v1"
	EventGateSkippedV1   = "gate.skipped.v1"
	EventGateRepairedV1  = "gate.repaired.v1"
	EventPatchAppliedV1  = "patch.applied.v1"
	EventRecoveryHitV1   = "recovery.recipe_hit.v1"
	EventRecoveryApplyV1 = "recovery.recipe_applied.v1"

	// Agent-stage event vocabulary (A55) — emitted by the finisher
	// engine + recovery loop so the studio chat surface can render
	// what the agent is doing in real time, not just gate verdicts
	// and cost ticks. Payload contract is loose by design: each
	// emitter stamps the fields it knows; the studio UI degrades on
	// missing keys.
	EventAgentStageStartedV1   = "agent.stage.started.v1"
	EventAgentStageActionV1    = "agent.stage.action.v1"
	EventAgentStageResultV1    = "agent.stage.result.v1"
	EventAgentStageCompletedV1 = "agent.stage.completed.v1"
	// Studio refine vocabulary (A55) — refineIdea writes a
	// studio.refine.v1 row; the finisher emits a
	// studio.refine.consumed.v1 row when it picks it up so the
	// in-progress refinements queue stays drainable + idempotent.
	EventStudioRefineV1         = "studio.refine.v1"
	EventStudioRefineConsumedV1 = "studio.refine.consumed.v1"
)

// gateEventTypes is the closed set of event_type values
// GateEventsByExecution treats as gate verdicts.
var gateEventTypes = map[string]struct{}{
	EventGateVerdictV1:  {},
	EventGateFailedV1:   {},
	EventGatePassedV1:   {},
	EventGateSkippedV1:  {},
	EventGateRepairedV1: {},
}

// IsGateEventType reports whether eventType belongs to the closed
// gate-verdict set GateEventsByExecution reads.
func IsGateEventType(eventType string) bool {
	_, ok := gateEventTypes[eventType]
	return ok
}

// IsPatchAppliedEventType reports whether eventType is the
// patch.applied.v1 marker PatchAppliedEventsByExecution reads.
func IsPatchAppliedEventType(eventType string) bool {
	return eventType == EventPatchAppliedV1
}

// IsRecoveryEventType reports whether eventType belongs to the
// closed recovery.* set RecoveryAttemptsByExecution reads.
func IsRecoveryEventType(eventType string) bool {
	return eventType == EventRecoveryHitV1 || eventType == EventRecoveryApplyV1
}
