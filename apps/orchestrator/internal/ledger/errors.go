package ledger

import "errors"

// ErrInvalidEntry is returned by Write when an Entry violates one of
// the ledger's structural invariants — unknown entry_type, unknown
// direction, missing tenant, etc. It is intentionally distinct from
// ErrZeroAmount so callers can branch on the failure mode (the latter
// is the most common "I forgot to multiply by tokens" mistake).
var ErrInvalidEntry = errors.New("ledger: invalid entry")

// ErrZeroAmount is returned by Write when AmountUSD <= 0. The
// migration also enforces this at the storage layer via a CHECK
// constraint; this error keeps the failure local and cheap so we
// don't burn a database round-trip on an obviously bad write.
var ErrZeroAmount = errors.New("ledger: amount must be > 0")
