package wallet

import "errors"

// ErrInsufficient is returned when Hold or Debit would drive the
// available balance negative. Resolvers translate this into a 402
// Payment Required GraphQL error carrying a `top_up_url` extension so
// the client can route the user to Stripe Checkout.
var ErrInsufficient = errors.New("wallet: insufficient available balance")

// ErrInvalidAmount is returned for non-positive amounts or amounts the
// wallet rejects (e.g. a top-up tier outside the supported list). The
// wallet refuses to perform zero/negative moves to keep the ledger
// honest — every Hold/Debit/TopUp must carry real economic weight.
var ErrInvalidAmount = errors.New("wallet: invalid amount")

// ErrUnknownSession is returned by the Stripe webhook handler when the
// incoming checkout.session.completed event references a session id we
// have no pending wallet_topups row for. The handler treats this as an
// abnormal (not retryable) error — Stripe should never deliver a
// completion event for a session we never created.
var ErrUnknownSession = errors.New("wallet: unknown stripe session")
