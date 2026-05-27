package guild

import "errors"

// ErrNotFound is the canonical missing-row signal. Mirrors the wallet
// package's pattern: resolvers translate it into a GraphQL 404.
var ErrNotFound = errors.New("guild: not found")

// ErrAlreadyExists fires when a UNIQUE-constrained insert collides
// (template slug, finisher profile per user).
var ErrAlreadyExists = errors.New("guild: already exists")

// ErrInvalidAmount is returned for non-positive USD inputs, bids over
// the task floor, or zero-priced templates.
var ErrInvalidAmount = errors.New("guild: invalid amount")

// ErrInvalidStatus is returned when a state transition would jump the
// allowed status FSM (accepting a bid on an expired task, withdrawing
// a won bid, etc.).
var ErrInvalidStatus = errors.New("guild: invalid status transition")

// ErrForbidden is returned by owner-checks. Surfaced as the same 404
// the projects store uses, NOT 403, so the existence of another
// tenant's task / template stays unleakable.
var ErrForbidden = errors.New("guild: forbidden")

// ErrBidTooHigh is returned by PlaceBid when the bid amount exceeds
// the task's PriceUSDFloor.
var ErrBidTooHigh = errors.New("guild: bid exceeds task floor")

// ErrTaskClosed is returned by PlaceBid when the task is no longer
// accepting bids (any non-open/non-bidding status).
var ErrTaskClosed = errors.New("guild: task not accepting bids")
