// Package wallet implements the V22 prepaid credit wallet.
//
// Every paid execution must pass through the wallet before any
// expensive call runs (hard law 1: "no execution starts without
// budget"). The wallet exposes four amount-bearing operations:
//
//   - TopUp:   Stripe-funded credit landing on balance_usd
//   - Hold:    reserve part of available balance for an execution
//   - Release: give back an unused hold (execution finished cheaper
//     than reserved, or was cancelled)
//   - Debit:   convert a previously-held amount into actual spend
//     (closes the hold and removes the funds from balance)
//
// All money is decimal.Decimal USD with 6 fractional digits of storage
// precision. The wallet never stores float values.
package wallet

import (
	"time"

	"github.com/shopspring/decimal"
)

// Wallet is the in-memory projection of one tenant's wallet row.
//
// TenantID is whatever string the orchestrator uses to scope per-tenant
// state — today that's User.OrgID when the user belongs to an org, and
// User.ID on personal accounts. The wallet package itself does not
// resolve that mapping; callers pass an already-resolved tenant id.
type Wallet struct {
	TenantID         string
	BalanceUSD       decimal.Decimal
	HoldUSD          decimal.Decimal
	LifetimeTopUpUSD decimal.Decimal
	LifetimeSpendUSD decimal.Decimal
	UpdatedAt        time.Time
	CreatedAt        time.Time
}

// AvailableUSD is balance - hold, clamped at zero. Resolvers use this
// to render the "credit you can spend now" headline number.
func (w Wallet) AvailableUSD() decimal.Decimal {
	a := w.BalanceUSD.Sub(w.HoldUSD)
	if a.IsNegative() {
		return decimal.Zero
	}
	return a
}

// LifetimeStats is the projection the dashboards read. Split out from
// Wallet so the dashboard query doesn't have to fetch balance/hold when
// it only wants the lifetime counters.
type LifetimeStats struct {
	LifetimeTopUpUSD decimal.Decimal
	LifetimeSpendUSD decimal.Decimal
}

// TopUp is one row of the wallet_topups table — a single checkout
// attempt from any payment provider. Status transitions: pending →
// succeeded | failed, or succeeded → refunded. Provider distinguishes
// "stripe" vs. "paddle" so the reconciliation cron can sweep per PSP.
// StripeSessionID is named for legacy reasons but stores any
// provider's transaction reference (Stripe cs_*, Paddle txn_*).
type TopUp struct {
	ID              string
	TenantID        string
	Provider        string
	StripeSessionID string
	AmountUSD       decimal.Decimal
	Status          string
	CreatedAt       time.Time
	CompletedAt     *time.Time
}

// ProviderFromSessionID infers the payment provider from the
// transaction reference id prefix. Stripe checkout sessions are
// prefixed "cs_", Paddle transactions "txn_". Returns "stripe" for
// anything else so legacy dev-seed rows (e.g. "dev-seed-…") classify
// as Stripe — they predate Paddle.
func ProviderFromSessionID(sessionID string) string {
	if len(sessionID) >= 4 && sessionID[:4] == "txn_" {
		return ProviderPaddle
	}
	return ProviderStripe
}
