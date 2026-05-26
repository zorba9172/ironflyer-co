// Package temporalworker is the durable command runner for paid
// Ironflyer executions. It hosts the FinisherExecutionWorkflow plus
// the activities that wrap the in-process V22 domain services
// (wallet, ledger, execution, ProfitGuard, finisher engine, settler,
// event emitter).
//
// The package is deliberately decoupled from the real service
// packages: every external collaborator is reached through a port
// interface declared in activities.go, and the integration agent
// wires real adapters in cmd/orchestrator/main.go. This keeps the
// Temporal worker buildable in isolation, lets the embedded
// (non-Temporal) path keep working unchanged, and gives main.go a
// single place to translate between Ironflyer's domain types and the
// activity-friendly DTOs used here.
//
// The workflow body is deterministic — no time.Now, no rand, no
// SQL, no HTTP, no map iteration order dependency. All side effects
// live in activities, and every activity that mutates state is
// idempotent against retry via the keys produced by idempotency.go.
package temporalworker

import (
	"github.com/shopspring/decimal"
)

// WorkflowInput is the durable input contract for
// FinisherExecutionWorkflow. The integration agent constructs this
// from the createPaidExecution flow once wallet admission has been
// approved (or, eventually, lets AdmitExecutionActivity own the
// admission itself).
//
// All fields must be value-typed and JSON-serialisable; Temporal
// persists this payload in workflow history.
type WorkflowInput struct {
	// ExecutionID is the Ironflyer domain id. It doubles as the
	// workflow id (`execution:{ExecutionID}`) and is the prefix for
	// every operation/idempotency key the activities mint.
	ExecutionID string
	// TenantID is the wallet/ledger owner. Required for every
	// wallet-touching activity.
	TenantID string
	// ProjectID is the finisher target. Required by RunGateActivity
	// because the in-process Engine keys per-project state.
	ProjectID string
	// BlueprintID is optional; when set, the settler attributes the
	// run to the blueprint stats rollup.
	BlueprintID string
	// BudgetUSD is the user's authorised reservation for this run.
	// Used as the wallet hold and as the projected revenue for
	// ProfitGuard margin math.
	BudgetUSD decimal.Decimal
	// StopLossUSD is the per-execution stop-loss circuit breaker.
	// Optional — a zero value means "no stop-loss".
	StopLossUSD decimal.Decimal
}

// WorkflowOutput is what the workflow returns to its caller (the
// integration agent's signal/poll surface). The fields mirror the
// Settlement struct from internal/execution so callers can render the
// final economic summary without re-reading the execution row.
type WorkflowOutput struct {
	// FinalStatus is the execution.Status the FSM settled on
	// (succeeded, failed, stopped, killed). String-typed so this
	// package does not depend on internal/execution.
	FinalStatus string
	// SpentUSD is the actualised spend after settlement.
	SpentUSD decimal.Decimal
	// CompletionScore is the final completion score in [0, 1].
	CompletionScore float64
	// GrossMarginPct is the (revenue - cost) / revenue percentage
	// recorded by the settler. Zero when revenue was zero.
	GrossMarginPct decimal.Decimal
}

// AdmitInput is the activity payload for AdmitExecutionActivity.
type AdmitInput struct {
	ExecutionID string
	TenantID    string
	BudgetUSD   decimal.Decimal
}

// AdmitOutput reports whether the wallet hold + FSM admit landed.
type AdmitOutput struct {
	Admitted bool
}

// StartInput is the activity payload for StartExecutionActivity.
type StartInput struct {
	ExecutionID string
}

// StartOutput reports whether the FSM transition to running landed.
type StartOutput struct {
	Started bool
}

// PGInput is the activity payload for ProfitGuardBeforeStepActivity.
// Point is a profitguard.EnforcementPoint serialised as a string so
// this package does not import the profitguard package.
type PGInput struct {
	ExecutionID string
	Point       string
}

// PGOutput is the ProfitGuard verdict translated into the four
// workflow-branch actions the loop understands.
type PGOutput struct {
	// Action is one of: "continue", "pause", "stop", "kill".
	// Continue keeps the loop running; pause/stop/kill terminate the
	// loop with the matching FSM transition.
	Action string
	// Reason is the human-readable explanation recorded with the
	// decision audit row.
	Reason string
}

// GateInput is the activity payload for RunGateActivity.
type GateInput struct {
	ExecutionID string
	ProjectID   string
	Gate        string
	Iteration   int
}

// GateOutput reports the gate verdict + the cost it accrued (the
// Engine attributes provider cost internally; this is reported back
// so the workflow can branch on "any progress was made").
type GateOutput struct {
	Passed      bool
	IssuesCount int
	CostUSD     decimal.Decimal
}

// SettleInput is the activity payload for SettleExecutionActivity.
type SettleInput struct {
	ExecutionID string
	FinalStatus string
}

// SettleOutput is the economic close-out summary.
type SettleOutput struct {
	SpentUSD        decimal.Decimal
	CompletionScore float64
	GrossMarginPct  decimal.Decimal
}

// EmitInput is the activity payload for EmitExecutionEventActivity.
type EmitInput struct {
	ExecutionID string
	EventType   string
	Sequence    int
	Payload     map[string]any
}

// EmitOutput is a stub — the emitter is fire-and-forget but Temporal
// activities must return a typed result.
type EmitOutput struct {
	Emitted bool
}
