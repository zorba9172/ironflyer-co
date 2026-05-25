// Package quota enforces tenant / execution / workspace / node quotas
// before a sandbox is allocated and again during execution, per
// ARCHITECTURE_RUNTIME_SCALE.md "Quotas And Admission". Every failure
// surfaces as a typed reason so callers can route appropriately
// (top-up flow, degrade runtime, capacity wait, etc).
package quota

// Reason is the admission outcome's typed reason string. The values
// match ARCHITECTURE_RUNTIME_SCALE.md exactly so logs / ledger events
// can pivot on them without translation.
type Reason string

const (
	// ReasonPauseForBudget — wallet is empty or insufficient; the
	// orchestrator should pause and request a top-up.
	ReasonPauseForBudget Reason = "pause_for_budget"
	// ReasonQuotaExceeded — a hard tenant/execution/workspace limit
	// would be breached. Retry only after Release().
	ReasonQuotaExceeded Reason = "quota_exceeded"
	// ReasonCapacityWait — node pool or warm slot saturated; retry
	// after the suggested backoff.
	ReasonCapacityWait Reason = "capacity_wait"
	// ReasonDegradeRuntime — admit on a cheaper RuntimeClass to
	// preserve margin (downgrade from kata to gvisor, etc).
	ReasonDegradeRuntime Reason = "degrade_runtime"
	// ReasonStopLoss — ProfitGuard says do not proceed at all.
	ReasonStopLoss Reason = "stop_loss"
)

// Error is the typed admission failure surfaced to callers.
type Error struct {
	Reason  Reason
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Reason)
	}
	return string(e.Reason) + ": " + e.Message
}

// New constructs an admission Error.
func New(r Reason, msg string) *Error { return &Error{Reason: r, Message: msg} }
