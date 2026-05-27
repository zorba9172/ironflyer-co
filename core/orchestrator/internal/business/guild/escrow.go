package guild

import (
	"context"

	"github.com/shopspring/decimal"
)

// WalletPort is the narrow subset of wallet.Service the guild package
// reaches into. Re-declared here as an interface so the package does
// not import wallet directly — wireup passes the live wallet.Service
// in, and tests would substitute a fake (if tests existed). Keeping
// the surface tight prevents future "guild reached for TopUp" drift.
type WalletPort interface {
	Hold(ctx context.Context, tenant string, amount decimal.Decimal) error
	Release(ctx context.Context, tenant string, amount decimal.Decimal) error
	Debit(ctx context.Context, tenant string, amount decimal.Decimal) error
}

// Escrow folds wallet mechanics around the guild lifecycle. Every
// money-moving step (open a task, accept a bid, reject / expire a
// task, install a template) routes through here so the wallet hold /
// release / debit pattern lives in one place and stays consistent
// with the wallet package's own contract.
//
// Construction is nil-safe: when wallet is nil (dev boot without a
// wallet service) the escrow no-ops every method — the rest of the
// guild flow still works for testing the resolver / router wiring.
type Escrow struct {
	wallet WalletPort
}

// NewEscrow builds an Escrow bound to the supplied wallet port.
func NewEscrow(w WalletPort) *Escrow { return &Escrow{wallet: w} }

// HoldFloor reserves the task floor against the requestor's wallet.
// Called by Coordinator.CreateTask. ErrInsufficient from the wallet
// layer surfaces as-is so the resolver can translate it into the
// INSUFFICIENT_FUNDS GraphQL error.
func (e *Escrow) HoldFloor(ctx context.Context, tenant string, amount decimal.Decimal) error {
	if e == nil || e.wallet == nil {
		return nil
	}
	return e.wallet.Hold(ctx, tenant, amount)
}

// ReleaseFloor returns the held floor when a task is rejected /
// expired. Idempotent at the wallet layer — releasing more than the
// outstanding hold clamps at zero.
func (e *Escrow) ReleaseFloor(ctx context.Context, tenant string, amount decimal.Decimal) error {
	if e == nil || e.wallet == nil {
		return nil
	}
	return e.wallet.Release(ctx, tenant, amount)
}

// SettleAccepted closes the hold for an accepted bid. The held floor
// is RELEASED first so the wallet's hold counter goes to zero on the
// task, then the FINAL accepted price is DEBITED — this matters when
// the winning bid came in BELOW the floor, the requestor only pays
// what they accepted, and the leftover hold returns to available.
//
// Wallet's own contract requires (held >= debited) inside Debit, so
// we cannot simply Debit the smaller amount against the larger hold;
// instead we walk Release(floor) -> Hold(price) -> Debit(price) so
// the books reconcile correctly even when price < floor.
func (e *Escrow) SettleAccepted(ctx context.Context, tenant string, floor, price decimal.Decimal) error {
	if e == nil || e.wallet == nil {
		return nil
	}
	if err := e.wallet.Release(ctx, tenant, floor); err != nil {
		return err
	}
	if err := e.wallet.Hold(ctx, tenant, price); err != nil {
		return err
	}
	return e.wallet.Debit(ctx, tenant, price)
}

// HoldAndDebit is the one-shot install path used by templates: Hold
// the price, then immediately Debit it once the artifacts land in
// the project workspace. We keep the two calls so the wallet's
// invariant (Debit consumes a prior Hold) stays satisfied without
// a special-case "instant debit" wallet method.
func (e *Escrow) HoldAndDebit(ctx context.Context, tenant string, amount decimal.Decimal) error {
	if e == nil || e.wallet == nil {
		return nil
	}
	if err := e.wallet.Hold(ctx, tenant, amount); err != nil {
		return err
	}
	return e.wallet.Debit(ctx, tenant, amount)
}
