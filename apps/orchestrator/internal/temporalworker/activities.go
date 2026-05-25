package temporalworker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.temporal.io/sdk/activity"
)

// ----- Ports -------------------------------------------------------
//
// The activities never import the real service packages
// (internal/wallet, internal/execution, internal/finisher,
// internal/profitguard, internal/ledger). Instead, every collaborator
// is reached through a narrow port interface declared here. The
// integration agent constructs adapter structs in main.go that
// translate between Ironflyer's domain types and these ports.
//
// Keeping the surface small (one method or two per port, all DTO
// inputs/outputs) means the temporalworker package compiles in
// isolation and can be reused by alternate worker binaries later
// (e.g. a split orchestrator-worker deploy) without dragging the
// orchestrator's entire dependency graph along.

// WalletPort is the wallet leg of every activity that moves money
// against the tenant balance. All amounts are decimal USD. opKey is
// the operation/idempotency key minted by idempotency.go — the
// adapter MUST collapse repeated calls on the same key into a single
// economic effect (Postgres unique constraint + ON CONFLICT, in-
// memory dedupe map, etc).
type WalletPort interface {
	Hold(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error
	Release(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error
	Debit(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error
}

// LedgerPort is the append-only ledger leg. Activities call this for
// auxiliary attributions the wallet alone cannot record (provider
// cost ledger rows, sandbox cost ticks, platform margin). v1 of the
// worker only writes through the Settler, but the port is here so
// future activities (sandbox tick, provider charge) can wire in
// without revisiting the Deps shape.
type LedgerPort interface {
	Write(ctx context.Context, entryType string, tenant, executionID string, amount decimal.Decimal, opKey string) error
}

// ExecutionPort is the FSM + state-read leg. Activities call GetState
// to feed ProfitGuard, and the lifecycle setters to advance the FSM.
// The Get/Set distinction matches internal/execution.Service so the
// main.go adapter is mostly a type-and-go wrapper.
type ExecutionPort interface {
	GetState(ctx context.Context, executionID string) (ExecStateSnapshot, error)
	Admit(ctx context.Context, executionID string) error
	Start(ctx context.Context, executionID string) error
	Succeed(ctx context.Context, executionID string) error
	Fail(ctx context.Context, executionID, reason string) error
	Stop(ctx context.Context, executionID, reason string) error
	Kill(ctx context.Context, executionID, reason string) error
}

// ExecStateSnapshot is the minimal projection ProfitGuard needs from
// an execution row. Declared here as a plain DTO so this package does
// not import internal/execution.State. The adapter in main.go fills
// it from execution.Service.GetState.
type ExecStateSnapshot struct {
	ExecutionID             string
	TenantID                string
	ProjectID               string
	Status                  string
	BudgetUSD               decimal.Decimal
	SpentUSD                decimal.Decimal
	ReservedUSD             decimal.Decimal
	StopLossUSD             decimal.Decimal
	CompletionScore         float64
	ExpectedCompletionDelta float64
	RiskScore               float64
}

// ProfitGuardPort is the policy + audit leg. Decide returns the
// action + reason as strings so the workflow can branch without
// importing profitguard. Record persists the decision row.
type ProfitGuardPort interface {
	Decide(ctx context.Context, point string, snapshot ExecStateSnapshot) (action, reason string, err error)
	Record(ctx context.Context, executionID, point, action, reason string) error
}

// EnginePort is the finisher gate runner. RunGate runs one named gate
// against a project's current state and returns the verdict + the
// number of issues + the cost it accrued. The Engine internally
// attributes provider cost on the execution row; CostUSD is reported
// back here for workflow-level dashboarding and progress detection.
type EnginePort interface {
	RunGate(ctx context.Context, projectID, gate string) (passed bool, issuesCount int, costUSD decimal.Decimal, err error)
}

// SettlerPort is the terminal wallet/ledger close-out. The adapter
// wraps execution.Settler.Close and returns the economic summary the
// workflow propagates as WorkflowOutput.
type SettlerPort interface {
	Close(ctx context.Context, executionID, finalStatus string) (SettleOutput, error)
}

// EventEmitterPort is the outbox + Redpanda fan-out. Activities call
// this to surface workflow lifecycle events (execution.admitted,
// execution.completed) onto the GraphQL/Redpanda streams without
// using Temporal history as a pub/sub bus.
type EventEmitterPort interface {
	Emit(ctx context.Context, eventType string, payload map[string]any) error
}

// Deps is the dependency bundle the worker passes to every activity.
// Each field is a port interface, so a nil field disables that leg
// (the activity short-circuits with a no-op result rather than
// panicking). main.go constructs and injects this via SetActivityDeps
// before Worker.Start.
type Deps struct {
	Wallet      WalletPort
	Ledger      LedgerPort
	Execution   ExecutionPort
	ProfitGuard ProfitGuardPort
	Engine      EnginePort
	Settler     SettlerPort
	Events      EventEmitterPort
}

// ----- Package-level dependency injection --------------------------
//
// Temporal activities are registered as plain Go functions, so they
// can't capture Deps via a closure without going through the SDK's
// activity-options machinery. We follow the standard SDK pattern of
// holding Deps behind a guarded package-level pointer that
// SetActivityDeps swaps in at startup. activityDeps() returns a
// guaranteed-non-nil bundle so individual activities can call port
// methods unconditionally and nil-check at the leaf only.

var (
	activityDepsMu sync.RWMutex
	activityDepsP  *Deps
)

// SetActivityDeps installs the dependency bundle the activities
// resolve at runtime. Safe to call from main.go during startup.
// Passing nil clears the bundle (used by tests / shutdown paths).
func SetActivityDeps(d *Deps) {
	activityDepsMu.Lock()
	defer activityDepsMu.Unlock()
	activityDepsP = d
}

// activityDeps returns the currently-installed Deps bundle, or an
// empty Deps if none was installed. Never returns nil; callers
// nil-check individual ports.
func activityDeps() *Deps {
	activityDepsMu.RLock()
	defer activityDepsMu.RUnlock()
	if activityDepsP == nil {
		return &Deps{}
	}
	return activityDepsP
}

// ----- Activities --------------------------------------------------

// AdmitExecutionActivity places the wallet hold and advances the FSM
// from created→admitted. Idempotent against the AdmitKey: a retry
// after worker crash collapses into a no-op once the row is already
// admitted.
func AdmitExecutionActivity(ctx context.Context, input AdmitInput) (AdmitOutput, error) {
	if input.ExecutionID == "" || input.TenantID == "" {
		return AdmitOutput{}, fmt.Errorf("admit: missing identifiers: %w", ErrInvalidArgument)
	}
	deps := activityDeps()

	if deps.Wallet != nil && input.BudgetUSD.IsPositive() {
		if err := deps.Wallet.Hold(ctx, input.TenantID, input.BudgetUSD, AdmitKey(input.ExecutionID)); err != nil {
			// Wallet rejection is a tenant policy violation — do not
			// retry past the first attempt; let the workflow branch
			// into cleanup.
			return AdmitOutput{}, fmt.Errorf("admit: wallet hold: %w: %v", ErrPolicyDeny, err)
		}
	}
	if deps.Execution != nil {
		if err := deps.Execution.Admit(ctx, input.ExecutionID); err != nil {
			// FSM is idempotent against "already admitted" via the
			// adapter; only true illegal-transition errors bubble up.
			return AdmitOutput{}, fmt.Errorf("admit: execution: %w: %v", ErrPolicyDeny, err)
		}
	}
	return AdmitOutput{Admitted: true}, nil
}

// StartExecutionActivity advances the FSM from admitted→running.
// Idempotent against the StartKey via the ExecutionPort adapter.
func StartExecutionActivity(ctx context.Context, input StartInput) (StartOutput, error) {
	if input.ExecutionID == "" {
		return StartOutput{}, fmt.Errorf("start: missing execution id: %w", ErrInvalidArgument)
	}
	deps := activityDeps()
	if deps.Execution == nil {
		return StartOutput{Started: true}, nil
	}
	if err := deps.Execution.Start(ctx, input.ExecutionID); err != nil {
		return StartOutput{}, fmt.Errorf("start: %w: %v", ErrPolicyDeny, err)
	}
	return StartOutput{Started: true}, nil
}

// ProfitGuardBeforeStepActivity calls the ProfitGuard policy with the
// current execution snapshot and translates the verdict into one of
// the four workflow-branch actions (continue, pause, stop, kill).
// The decision is recorded via ProfitGuardPort.Record so the audit
// dashboard reflects it.
func ProfitGuardBeforeStepActivity(ctx context.Context, input PGInput) (PGOutput, error) {
	if input.ExecutionID == "" || input.Point == "" {
		return PGOutput{}, fmt.Errorf("profit_guard: missing args: %w", ErrInvalidArgument)
	}
	deps := activityDeps()
	if deps.ProfitGuard == nil || deps.Execution == nil {
		// No-op default: when the policy is not wired, the loop is
		// allowed to proceed. The hard economic stops (wallet hold,
		// stop-loss) remain enforced by the wallet + the engine.
		return PGOutput{Action: "continue", Reason: "profit_guard_unwired"}, nil
	}

	snapshot, err := deps.Execution.GetState(ctx, input.ExecutionID)
	if err != nil {
		// State read failure: fail-open and let the loop continue —
		// the hard economic stops still apply.
		return PGOutput{Action: "continue", Reason: "snapshot_unavailable"}, nil
	}

	action, reason, derr := deps.ProfitGuard.Decide(ctx, input.Point, snapshot)
	if derr != nil {
		return PGOutput{Action: "continue", Reason: "profit_guard_error"}, nil
	}
	_ = deps.ProfitGuard.Record(ctx, input.ExecutionID, input.Point, action, reason)

	switch action {
	case "stop":
		return PGOutput{Action: "stop", Reason: reason}, nil
	case "kill_branch", "kill":
		return PGOutput{Action: "kill", Reason: reason}, nil
	case "pause_for_budget", "pause":
		return PGOutput{Action: "pause", Reason: reason}, nil
	default:
		// Everything else — continue, degrade, switch_provider,
		// reuse_blueprint, reuse_repair — flows through the loop as
		// "continue". The granular handling is the engine's job;
		// Temporal only owns the four terminal branches.
		return PGOutput{Action: "continue", Reason: reason}, nil
	}
}

// RunGateActivity runs one finisher gate against the project. The
// activity heartbeats every 30s so a long gate (browser-driven
// verification, slow build) does not look hung to Temporal and so a
// retry after worker crash can reattach instead of being killed mid-
// run.
func RunGateActivity(ctx context.Context, input GateInput) (GateOutput, error) {
	if input.ExecutionID == "" || input.ProjectID == "" || input.Gate == "" {
		return GateOutput{}, fmt.Errorf("run_gate: missing args: %w", ErrInvalidArgument)
	}
	deps := activityDeps()
	if deps.Engine == nil {
		// No engine wired — treat as a no-op pass so the workflow
		// can complete. This is the dev/sandbox shortcut.
		return GateOutput{Passed: true, IssuesCount: 0, CostUSD: decimal.Zero}, nil
	}

	// Heartbeat loop: a separate goroutine pings activity.RecordHeartbeat
	// every 30s while the underlying RunGate is in flight. We cancel it
	// the moment RunGate returns to avoid leaking a goroutine after a
	// worker shutdown.
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				// Heartbeat payload carries the iteration so the
				// dashboard can show "iter N gate G still running".
				activity.RecordHeartbeat(hbCtx, input.Iteration, input.Gate)
			}
		}
	}()

	passed, issues, cost, err := deps.Engine.RunGate(ctx, input.ProjectID, input.Gate)
	if err != nil {
		// Engine errors are infrastructure errors (sandbox down,
		// provider 5xx) — Temporal retries normally.
		return GateOutput{}, err
	}
	return GateOutput{Passed: passed, IssuesCount: issues, CostUSD: cost}, nil
}

// SettleExecutionActivity runs the wallet/ledger close-out. Always
// scheduled by the workflow via NewDisconnectedContext so a workflow
// cancellation does not skip settlement — money correctness wins
// over fast cancel.
func SettleExecutionActivity(ctx context.Context, input SettleInput) (SettleOutput, error) {
	if input.ExecutionID == "" || input.FinalStatus == "" {
		return SettleOutput{}, fmt.Errorf("settle: missing args: %w", ErrInvalidArgument)
	}
	deps := activityDeps()
	if deps.Settler == nil {
		// No settler wired — return a zero-valued summary so the
		// workflow output still serialises cleanly.
		return SettleOutput{}, nil
	}
	out, err := deps.Settler.Close(ctx, input.ExecutionID, input.FinalStatus)
	if err != nil {
		// Settlement failures are infrastructure failures by default —
		// Temporal will retry per the worker's settlement retry
		// policy. The only non-retryable case is a malformed
		// execution id, which we already guarded above.
		if errors.Is(err, ErrInvalidArgument) {
			return SettleOutput{}, err
		}
		return SettleOutput{}, fmt.Errorf("settle: %w", err)
	}
	return out, nil
}

// EmitExecutionEventActivity writes an execution-events row through
// the outbox/EventEmitter port. Idempotent against the EventKey
// (eventType + sequence) so a retry collapses into a single row.
func EmitExecutionEventActivity(ctx context.Context, input EmitInput) (EmitOutput, error) {
	if input.ExecutionID == "" || input.EventType == "" {
		return EmitOutput{}, fmt.Errorf("emit_event: missing args: %w", ErrInvalidArgument)
	}
	deps := activityDeps()
	if deps.Events == nil {
		return EmitOutput{Emitted: false}, nil
	}
	// The outbox key is mixed into the payload so the adapter can
	// dedupe at the storage layer without changing the port shape.
	payload := input.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	payload["execution_id"] = input.ExecutionID
	payload["op_key"] = EventKey(input.ExecutionID, input.EventType, input.Sequence)
	if err := deps.Events.Emit(ctx, input.EventType, payload); err != nil {
		return EmitOutput{}, err
	}
	return EmitOutput{Emitted: true}, nil
}
