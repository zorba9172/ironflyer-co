package temporalworker

import "errors"

// Sentinel errors that activities return when the workflow should
// short-circuit instead of retrying. The activity options registered
// in worker.go list these as non-retryable so a single deterministic
// branch decision can drive the workflow into terminal cleanup.
var (
	// ErrInvalidArgument signals a programmer error in the workflow
	// or the activity payload (missing execution id, malformed
	// inputs). Never retry.
	ErrInvalidArgument = errors.New("temporalworker: invalid argument")

	// ErrProfitGuardStop signals that the ProfitGuard policy returned
	// a Stop / KillBranch verdict. The workflow translates this into
	// the matching FSM transition (stopped or killed) and runs
	// settlement.
	ErrProfitGuardStop = errors.New("temporalworker: profit guard stop")

	// ErrPolicyDeny signals a tenant-level policy violation reported
	// by a downstream service (wallet ErrInsufficient, FSM
	// ErrIllegalTransition, etc). Never retry; surface to the
	// workflow caller as a terminal failure.
	ErrPolicyDeny = errors.New("temporalworker: policy deny")
)

// nonRetryableErrorTypes is the type-name list passed to Temporal's
// RetryPolicy. We rely on Temporal's default error-type encoding (the
// string after the final dot of the error's runtime type) — these
// names match the registered sentinels above when wrapped via
// %w / errors.Is. Activities that need a hard stop should return
// `fmt.Errorf("…: %w", ErrInvalidArgument)` so the worker classifier
// matches.
//
// Kept as a package-level helper so worker.go can attach the same
// list to every activity registration.
func nonRetryableErrorTypes() []string {
	return []string{
		"ErrInvalidArgument",
		"ErrProfitGuardStop",
		"ErrPolicyDeny",
	}
}
