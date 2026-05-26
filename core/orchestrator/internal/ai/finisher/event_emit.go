package finisher

// event_emit.go is the thin best-effort bridge from the finisher engine /
// recovery loop into the execution-events ring. The wow-loop adapter
// (core/orchestrator/internal/wowloop) reads gate.verdict.v1,
// patch.applied.v1, and recovery.recipe_*.v1 rows back out of
// execution_events to render the customer-facing executionSupportBundle.
//
// Every call here is best-effort by contract: missing execution service,
// missing executionID on context, payload marshal failure, and RecordEvent
// errors all degrade silently — the parent finisher operation MUST never
// abort because telemetry failed to land.

import (
	"context"
	"encoding/json"

	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
)

// emitExecutionEvent records a domain event onto the execution's event
// ring. Nil-safe at every level: nil exec service, missing execID,
// marshal error, and RecordEvent error all return silently.
//
// Callers should NOT branch on the outcome — by design this is fire-and-
// forget observability. If a stronger contract is ever needed, wrap the
// call site rather than changing this helper.
func emitExecutionEvent(ctx context.Context, execSvc execution.Service, eventType string, payload map[string]any) {
	if execSvc == nil || eventType == "" {
		return
	}
	execID, ok := profitguardctx.ExecutionID(ctx)
	if !ok || execID == "" {
		return
	}
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return
		}
		raw = b
	}
	_ = execSvc.RecordEvent(ctx, execID, eventType, raw)
}

// gateStatusFromPassed normalises a gate verdict into the closed
// vocabulary the wow-loop GateSource adapter understands:
//
//	passed=true                -> "passed"
//	passed=false, issues==0    -> "skipped" (gate ran but had nothing to do)
//	passed=false, issues > 0   -> "failed"
//
// The vocabulary mirrors execution.GateEvent.Status downstream, and stays
// in lockstep with the gate.* event-type constants in execution/service.go.
func gateStatusFromPassed(passed bool, issueCount int) string {
	if passed {
		return "passed"
	}
	if issueCount == 0 {
		return "skipped"
	}
	return "failed"
}
