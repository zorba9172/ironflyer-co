package finisher

// Event step names emitted on the SSE bus. These are the *only* names a
// consumer (the web dashboard, the VSCode extension, the SDK) should match
// against — the engine never invents an ad-hoc step string elsewhere.
//
// Payload conventions (carried on domain.Event):
//   - Step      : one of the StepXxx constants below
//   - Gate      : domain.GateName when the step is gate-scoped
//   - Agent     : agents.Role when the step is agent-scoped
//   - Status    : "running" | "done" | "failed" — matches SSE semantics
//   - Message   : short human string (consumers may render verbatim)
//
// Failure events additionally carry a structured ErrorCode in Message of the
// form  "<code>: <human reason>"  so a UI can dispatch on the prefix without
// regex-parsing the free-form tail. Codes are kept stable; see ErrorCode*.
const (
	StepPlanner       = "planner"
	StepArchitect     = "architect"
	StepUXer          = "uxer"
	StepCoder         = "coder"
	StepReviewer      = "reviewer"
	StepGate          = "gate"
	StepPatch         = "patch"
	StepRun           = "run"
	StepLoopIteration = "iteration"
	// StepRecovery is emitted by the auto-recovery engine when a gate has
	// failed after a patch was applied. The recovery loop re-prompts the
	// Coder with the failure context and re-runs the failed gate only.
	// Status semantics on this step:
	//   running : "recovery_started attempt=N gate=<g>"
	//   done    : "recovery_done attempt=N gate=<g>"
	//   failed  : prefixed by an ErrorCode — one of
	//             "recovery_exhausted", "recovery_budget", "patch_invalid",
	//             "runtime_error", "provider_error". The free-form tail
	//             carries the human reason / "recovery_failed"/"recovery_aborted"
	//             marker for finer-grained UI dispatch.
	StepRecovery = "recovery"
)

// Status values used on domain.Event.Status. We piggyback on the existing
// strings already in use by other emit() callers to avoid breaking the wire.
const (
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// ErrorCode is a stable machine code prefix attached to failure Message
// payloads so consumers can dispatch on category (toast vs. modal vs.
// silent log) without parsing English.
type ErrorCode string

const (
	ErrCodeProviderError     ErrorCode = "provider_error"
	ErrCodeBudgetExhausted   ErrorCode = "budget_exhausted"
	ErrCodeGateUnrecoverable ErrorCode = "gate_unrecoverable"
	ErrCodePatchInvalid      ErrorCode = "patch_invalid"
	ErrCodePatchTooLarge     ErrorCode = "patch_too_large"
	ErrCodePlanMalformed     ErrorCode = "plan_malformed"
	ErrCodeRuntimeError      ErrorCode = "runtime_error"
	ErrCodeContextCancelled  ErrorCode = "context_cancelled"
	// ErrRecoveryExhausted is emitted when the recovery engine has used all
	// MaxAttempts iterations and the failed gate still does not pass.
	ErrRecoveryExhausted ErrorCode = "recovery_exhausted"
	// ErrRecoveryBudget is emitted when the per-gate recovery cost cap is
	// crossed mid-loop; the loop stops early without declaring exhaustion.
	ErrRecoveryBudget ErrorCode = "recovery_budget"
)

// fmtErr renders a code+reason into the single Message slot — keeping the
// SSE event schema stable across consumers. We don't change domain.Event
// shape because it ships through the existing /stream contract.
func fmtErr(code ErrorCode, reason string) string {
	return string(code) + ": " + reason
}

// classifyProviderErr inspects an error returned by the LLM router and
// returns a more specific ErrorCode when the underlying cause is something
// the user can act on (budget hit), falling back to provider_error.
func classifyProviderErr(err error) ErrorCode {
	if err == nil {
		return ErrCodeProviderError
	}
	msg := err.Error()
	for i := 0; i+6 <= len(msg); i++ {
		if msg[i:i+6] == "budget" {
			return ErrCodeBudgetExhausted
		}
	}
	return ErrCodeProviderError
}
