package execution

// Status transition matrix for an Execution.
//
// Legal moves:
//
//	created           → admitted | failed | stopped | killed
//	admitted          → running  | failed | stopped | killed
//	running           → paused_for_budget | succeeded | failed | stopped | killed
//	paused_for_budget → running | stopped | failed | killed
//	succeeded         → refunded
//	failed            → refunded
//	stopped           → refunded
//	killed            → refunded
//	refunded          → (terminal)
//
// Refund is the only post-terminal move because the wallet credit-back
// can land asynchronously (manual goodwill, dispute resolution). All
// other terminals are absorbing.
//
// CanTransition is pure — implementations call it under SELECT … FOR
// UPDATE (Postgres) or while holding the write lock (memory) so that
// concurrent FSM moves cannot interleave.
var allowedTransitions = map[Status]map[Status]bool{
	StatusCreated: {
		StatusAdmitted: true,
		StatusFailed:   true,
		StatusStopped:  true,
		StatusKilled:   true,
	},
	StatusAdmitted: {
		StatusRunning: true,
		StatusFailed:  true,
		StatusStopped: true,
		StatusKilled:  true,
	},
	StatusRunning: {
		StatusPausedForBudget: true,
		StatusSucceeded:       true,
		StatusFailed:          true,
		StatusStopped:         true,
		StatusKilled:          true,
	},
	StatusPausedForBudget: {
		StatusRunning: true,
		StatusStopped: true,
		StatusFailed:  true,
		StatusKilled:  true,
	},
	StatusSucceeded: {StatusRefunded: true},
	StatusFailed:    {StatusRefunded: true},
	StatusStopped:   {StatusRefunded: true},
	StatusKilled:    {StatusRefunded: true},
	StatusRefunded:  {},
}

// CanTransition reports whether moving from `from` to `to` is allowed
// by the execution FSM. An unknown `from` is treated as illegal so the
// caller cannot smuggle in a status the matrix has not been updated to
// recognise.
func CanTransition(from, to Status) bool {
	if from == to {
		// No-op moves are not transitions — callers should avoid them.
		return false
	}
	row, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return row[to]
}

// IsTerminal reports whether the status is one of the absorbing
// terminals (succeeded/failed/stopped/killed/refunded). The cost and
// score mutators check this to return ErrFinalised early instead of
// firing a doomed UPDATE.
func IsTerminal(s Status) bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusStopped, StatusKilled, StatusRefunded:
		return true
	default:
		return false
	}
}
