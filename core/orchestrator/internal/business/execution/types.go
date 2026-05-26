// Package execution implements the V22 Execution entity — every paid
// run of the finisher is one row in `executions`, with full economic
// attribution (revenue, provider/sandbox/storage/deployment cost,
// completion score, gross margin) and an append-only event feed.
//
// ProfitGuard reads Service.GetState(ctx, id) before every expensive
// call; the finisher mutates the row via Reserve / AddCost /
// AddRevenue / SetCompletionScore as work progresses; the wallet sees
// the matching Hold / Debit / Release on the tenant balance.
//
// All money is shopspring/decimal USD. Completion score is float64 in
// [0, 1]. Status transitions are guarded by the FSM in fsm.go and by
// SELECT … FOR UPDATE in the Postgres implementation.
package execution

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// Status is the execution lifecycle state. The string values match the
// `execution_status` ENUM in migration 00026_executions.sql.
type Status string

const (
	// StatusCreated is the initial state right after Service.Create.
	// No wallet hold yet, no work running. The next legal move is
	// Admit (which triggers the wallet hold) or a terminal stop/fail.
	StatusCreated Status = "created"

	// StatusAdmitted means ProfitGuard.Admit succeeded — the wallet
	// hold is in place but the worker has not started yet. From here
	// the FSM moves to Running.
	StatusAdmitted Status = "admitted"

	// StatusRunning is the steady state while the finisher loop is
	// burning the reserved budget. Most cost/score updates land while
	// in Running.
	StatusRunning Status = "running"

	// StatusPausedForBudget is the soft-stop signalled by ProfitGuard
	// when the user budget is exhausted but the execution is otherwise
	// healthy. The worker holds its state; a wallet top-up + Resume
	// moves it back to Running.
	StatusPausedForBudget Status = "paused_for_budget"

	// StatusSucceeded is the happy path terminal — the execution
	// shipped a finished artifact. Refund is still legal after this
	// (manual goodwill / dispute resolution).
	StatusSucceeded Status = "succeeded"

	// StatusFailed is the terminal for an execution that exhausted
	// its budget without producing a usable artifact. failure_reason
	// captures the structured cause.
	StatusFailed Status = "failed"

	// StatusStopped is the operator-initiated terminal (UI button,
	// CLI, ProfitGuard stop verdict). Distinct from Failed because no
	// hard failure occurred.
	StatusStopped Status = "stopped"

	// StatusKilled is the hard terminal — kill_branch from ProfitGuard
	// or an out-of-band SIGTERM. Worker process is gone; partial state
	// may not be recoverable.
	StatusKilled Status = "killed"

	// StatusRefunded is the post-terminal state recorded after the
	// wallet credit-back lands. Only reachable from succeeded /
	// failed / stopped / killed.
	StatusRefunded Status = "refunded"
)

// Execution is the in-memory projection of one `executions` row. All
// money fields are decimal USD; completion fields are float64 in [0,1].
// The struct is intentionally flat so the Postgres + memory backends
// can populate it with a single Scan / map lookup.
type Execution struct {
	ID                      string          `json:"id"`
	TenantID                string          `json:"tenant_id"`
	ProjectID               string          `json:"project_id,omitempty"`
	BlueprintID             string          `json:"blueprint_id,omitempty"`
	// WorkspaceID is the runtime sandbox bound to this execution. The
	// finisher engine stamps it via Service.SetWorkspaceID the moment a
	// workspace is resolved/allocated. Empty for legacy rows and for
	// executions that never allocated a sandbox. Persisted to the
	// `workspace_id` TEXT column (migration 00040).
	WorkspaceID             string          `json:"workspace_id,omitempty"`
	Status                  Status          `json:"status"`
	BudgetUSD               decimal.Decimal `json:"budget_usd"`
	ReservedUSD             decimal.Decimal `json:"reserved_usd"`
	SpentUSD                decimal.Decimal `json:"spent_usd"`
	RefundedUSD             decimal.Decimal `json:"refunded_usd"`
	RevenueUSD              decimal.Decimal `json:"revenue_usd"`
	ProviderCostUSD         decimal.Decimal `json:"provider_cost_usd"`
	SandboxCostUSD          decimal.Decimal `json:"sandbox_cost_usd"`
	StorageCostUSD          decimal.Decimal `json:"storage_cost_usd"`
	DeploymentCostUSD       decimal.Decimal `json:"deployment_cost_usd"`
	CompletionScore         float64         `json:"completion_score"`
	CompletionScoreInitial  float64         `json:"completion_score_initial"`
	GrossMarginPct          *decimal.Decimal `json:"gross_margin_pct,omitempty"`
	ExpectedCompletionDelta *float64        `json:"expected_completion_delta,omitempty"`
	RiskScore               *float64        `json:"risk_score,omitempty"`
	StopLossUSD             *decimal.Decimal `json:"stop_loss_usd,omitempty"`
	PromptSummary           string          `json:"prompt_summary,omitempty"`
	FailureReason           string          `json:"failure_reason,omitempty"`
	Metadata                json.RawMessage `json:"metadata,omitempty"`
	CreatedAt               time.Time       `json:"created_at"`
	AdmittedAt              *time.Time      `json:"admitted_at,omitempty"`
	StartedAt               *time.Time      `json:"started_at,omitempty"`
	EndedAt                 *time.Time      `json:"ended_at,omitempty"`
}

// CreateInput is the payload accepted by Service.Create. Tenant and
// budget are mandatory; everything else is optional. The wallet hold
// is NOT performed inside Create — call Admit once ProfitGuard has
// approved the run.
type CreateInput struct {
	TenantID      string
	ProjectID     string
	BlueprintID   string
	BudgetUSD     decimal.Decimal
	StopLossUSD   *decimal.Decimal
	PromptSummary string
	Metadata      json.RawMessage
}

// State is the snapshot ProfitGuard.Decide consumes. The JSON shape
// matches the input contract in
// docs/.../01-unit-economics/03-profit-guard-policy.md — extra fields
// (BudgetRemaining, CompletionPerDollar) are derived projections that
// the policy reads directly so it doesn't have to recompute them.
//
// BudgetRemaining = max(0, budget_usd - spent_usd - reserved_usd).
// CompletionPerDollar = (completion_score - completion_score_initial) /
//                       max(spent_usd, ε) — measured efficiency to date.
type State struct {
	Execution
	BudgetRemaining     decimal.Decimal `json:"budget_remaining"`
	CompletionPerDollar decimal.Decimal `json:"completion_per_dollar"`
}

// Event is the in-memory projection of one `execution_events` row.
// Subscribers receive these via Service.SubscribeEvents; the broker
// fan-out is keyed by ExecutionID.
type Event struct {
	ExecutionID string          `json:"execution_id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"created_at"`
}
