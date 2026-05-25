package temporalworker

import "strconv"

// Idempotency key helpers. Every key follows the contract documented
// in docs/ARCHITECTURE_WORKFLOWS.md so retried activities can be
// deduped at the storage layer (wallet hold, ledger write, execution
// event outbox).
//
// Keys are intentionally compact strings so they fit on a Postgres
// unique index without truncation. They MUST be stable across
// workflow replays — the workflow body computes them deterministically
// from input + iteration counters and passes them to the activity
// payload.

// AdmitKey returns the operation key for the wallet hold + FSM admit
// step. Stable across retries so a re-execution after worker crash
// does not double-hold the wallet.
func AdmitKey(executionID string) string {
	return "execution:" + executionID + ":admit"
}

// StartKey returns the operation key for the FSM start step.
func StartKey(executionID string) string {
	return "execution:" + executionID + ":start"
}

// GateKey returns the operation key for one gate execution within a
// finisher iteration. Includes the iteration index so a retry of the
// same iteration collapses but a follow-up iteration still writes a
// distinct row.
func GateKey(executionID, gate string, iteration int) string {
	return "execution:" + executionID + ":gate:" + gate + ":" + strconv.Itoa(iteration)
}

// SettlementKey returns the operation key for the wallet/ledger
// close-out. Used by SettleExecutionActivity to guarantee that
// settlement-retry does not double-debit or double-write margin.
func SettlementKey(executionID string) string {
	return "execution:" + executionID + ":settlement"
}

// EventKey returns the operation key for one execution event row.
// The (eventType, sequence) suffix lets the same logical event
// (e.g. "execution.completed.v1") be reissued idempotently on
// retry while still distinguishing genuinely new events.
func EventKey(executionID, eventType string, sequence int) string {
	return "execution:" + executionID + ":event:" + eventType + ":" + strconv.Itoa(sequence)
}
