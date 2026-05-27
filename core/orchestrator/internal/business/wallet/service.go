package wallet

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Service is the persistence-agnostic contract for the wallet. Both
// the memory and postgres backends implement it; the Stripe top-up
// machinery talks only to this interface so unit-of-work boundaries
// stay enforceable.
//
// Every amount is decimal USD. Implementations MUST refuse non-positive
// amounts on the money-moving operations and return ErrInvalidAmount.
//
// Hold is atomic: it MUST verify (balance - hold) >= amount under a
// row lock and increment hold in the same transaction; otherwise
// concurrent executions could each see "enough available" and double-
// commit. Debit closes a hold — it decrements both balance AND hold
// by the same amount, so the released portion stays on balance and is
// callable via Release for the leftover.
type Service interface {
	// Get returns the wallet row, creating one with zero balances if
	// the tenant has never had a wallet before. Returning a zero-valued
	// wallet (instead of a not-found error) keeps the resolver flow
	// trivial — new tenants see "$0 balance" and a top-up CTA.
	Get(ctx context.Context, tenant string) (Wallet, error)

	// TopUp credits balance_usd and lifetime_topup_usd, and flips the
	// matching wallet_topups row from pending → succeeded. Idempotent
	// against stripeSessionID — repeated calls for the same Stripe
	// session apply once.
	TopUp(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID string) error

	// Hold reserves amount against the tenant's available balance.
	// Returns ErrInsufficient when balance - hold < amount.
	Hold(ctx context.Context, tenant string, amount decimal.Decimal) error

	// Release returns an unused hold to available balance (decrements
	// hold_usd without touching balance_usd). Used when an execution
	// finishes cheaper than its reservation, or aborts before any
	// spend has materialised.
	Release(ctx context.Context, tenant string, amount decimal.Decimal) error

	// Debit closes a previously-held amount: decrements balance_usd
	// AND hold_usd by amount, and bumps lifetime_spend_usd. The caller
	// is responsible for ensuring the hold existed (otherwise the
	// underlying CHECK constraint will fail and roll back).
	Debit(ctx context.Context, tenant string, amount decimal.Decimal) error

	// LifetimeStats returns the monotonic counters for the profit
	// dashboards without paying the cost of returning the full wallet
	// row.
	LifetimeStats(ctx context.Context, tenant string) (LifetimeStats, error)

	// ListTopUps returns the most recent wallet_topups rows for the
	// tenant, newest first. Used by the GraphQL walletTopUps query
	// rendered on the billing page.
	ListTopUps(ctx context.Context, tenant string, limit int) ([]TopUp, error)

	// CreatePendingTopUp records a new wallet_topups row in 'pending'
	// state. Called by the Stripe Topper after CreateCheckoutSession
	// succeeds, BEFORE we return the URL to the user. The webhook
	// later flips it to succeeded via TopUp.
	CreatePendingTopUp(ctx context.Context, tenant string, amount decimal.Decimal, stripeSessionID string) (TopUp, error)

	// ListStalePending returns wallet_topups rows still in 'pending'
	// state older than threshold. Used by the reconciliation cron to
	// detect missed webhooks: any row older than the vendor's typical
	// settlement window (~10 minutes) almost certainly indicates that
	// a webhook delivery was dropped, the vendor is having an outage,
	// or the user abandoned the checkout. The cron re-queries the
	// vendor via Topper.VerifySession and lands the credit or marks
	// the row failed accordingly.
	ListStalePending(ctx context.Context, threshold time.Duration) ([]TopUp, error)

	// MarkFailed transitions a 'pending' wallet_topups row to 'failed'.
	// Called by the reconciler when the vendor reports the session as
	// expired or terminally failed. Idempotent against repeated calls
	// for the same session id.
	MarkFailed(ctx context.Context, stripeSessionID string) error
}
