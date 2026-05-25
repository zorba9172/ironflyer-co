package execution

import "errors"

// ErrNotFound is returned when no execution row exists for the supplied
// id. Resolvers translate this into a GraphQL NOT_FOUND error so the
// caller can distinguish "wrong id" from "transient failure".
var ErrNotFound = errors.New("execution: not found")

// ErrIllegalTransition is returned by any status-changing operation
// when the requested move is not allowed by the FSM (see fsm.go).
// Callers MUST treat this as a programming error — the orchestrator
// should not attempt the transition in the first place.
var ErrIllegalTransition = errors.New("execution: illegal status transition")

// ErrFinalised is returned when a mutating call (AddCost, Reserve,
// SetCompletionScore, …) lands on an execution that has already
// reached a terminal status (succeeded/failed/stopped/killed/refunded).
// Cost cannot be attributed after the wallet hold has been released.
var ErrFinalised = errors.New("execution: already finalised")

// ErrInvalidAmount is returned for non-positive monetary amounts on
// Reserve/AddCost/AddRevenue/Refund. Matches the wallet contract — no
// zero-value moves so the events stream stays meaningful.
var ErrInvalidAmount = errors.New("execution: invalid amount")

// ErrInvalidScore is returned when a completion score outside [0, 1]
// is supplied. The DB-level CHECK constraint also rejects this, but
// catching it in Go avoids a round-trip and produces a clearer error.
var ErrInvalidScore = errors.New("execution: invalid completion score")
