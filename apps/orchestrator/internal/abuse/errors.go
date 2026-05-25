package abuse

import "errors"

// ErrUnknownTier signals that a string supplied by an external caller
// (env var, store row, policy bundle) does not map to one of the four
// canonical tier values. Callers that surface this back to clients
// should redact the offending value — never let it land in a client
// error verbatim.
var ErrUnknownTier = errors.New("abuse: unknown tier")

// ErrInvalidWeight is returned when RecordSignal is called with a
// weight outside the sane [-100,100] band. The engine clamps at the
// score boundary anyway, but rejecting outliers at the signal layer
// keeps a single misconfigured caller from skewing the 24h window.
var ErrInvalidWeight = errors.New("abuse: signal weight outside [-100,100]")

// ErrStoreUnavailable wraps lower-level store errors so the caller can
// distinguish a transient infra failure from a malformed input.
var ErrStoreUnavailable = errors.New("abuse: store unavailable")
