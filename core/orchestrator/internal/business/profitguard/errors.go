package profitguard

import "errors"

// ErrInvalidState is returned by Decide when the ExecState snapshot
// violates a structural invariant (missing execution id, negative
// monetary amount, NaN ratio, etc.). It is intentionally distinct
// from a normal Stop verdict — a caller that sees ErrInvalidState
// has a bug in the snapshot adapter, not an economic problem.
var ErrInvalidState = errors.New("profitguard: invalid execution state")
