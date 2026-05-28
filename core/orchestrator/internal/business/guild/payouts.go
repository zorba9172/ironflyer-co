package guild

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// Payouts is the cash-out side of the guild flow. AcceptBid hands the
// accepted task + winning bid here; we compute the platform / finisher
// split using PlatformTaskCutPct, record the Payout row, and DEFER
// the actual Stripe Connect transfer to a TODO the provisioning agent
// is responsible for wiring.
//
// Why deferred: real Stripe Connect transfers require a
// ProvisioningVault that owns the finisher's connected account id
// (acct_*), the platform's application_fee policy, and the per-region
// payout schedule. None of that lives in this package — the
// provisioning team owns the Vault, and we leave a clearly-named
// `payOutFinisher` method here so a single grep finds the wire-up
// point.
type Payouts struct {
	svc        Service
	transferer PayoutTransferer
	logger     zerolog.Logger
}

// PayoutTransferer is the rail-side payout port. The provisioning
// package's Stripe Connect adapter satisfies it; main.go injects the
// adapter when both the guild and the provisioning vault are enabled.
// A nil transferer keeps the queued payout row as the durable record
// and logs a stub line — the row can be settled later once the rail
// is wired.
type PayoutTransferer interface {
	// Transfer pays finisherCutUSD to the finisher's connected payout
	// account and returns the rail-side transfer ref. idempotencyKey
	// dedupes retries (pass the guild payout id). Returns a non-nil
	// error when the finisher has no connected account yet, the rail
	// is disabled, or the transfer is rejected — the caller marks the
	// payout 'failed' and leaves it for manual follow-up.
	Transfer(ctx context.Context, finisherID string, finisherCutUSD decimal.Decimal, idempotencyKey string) (externalRef string, err error)
}

// NewPayouts builds the payouts helper bound to the guild store. The
// transferer is optional — pass nil to keep payouts queued-only.
func NewPayouts(svc Service, transferer PayoutTransferer, logger zerolog.Logger) *Payouts {
	return &Payouts{svc: svc, transferer: transferer, logger: logger}
}

// SetTransferer installs the rail-side payout transferer after
// construction. Used by wireup when the transferer needs the bundle's
// own Service (to resolve finisher profiles) and therefore cannot be
// built before the bundle exists. Safe to call once at boot before
// any payout fires.
func (p *Payouts) SetTransferer(t PayoutTransferer) { p.transferer = t }

// SplitTaskAmount returns (platformCut, finisherCut) for an accepted
// task amount. platformCut = amount * 0.20; finisherCut = amount -
// platformCut so the two pieces sum to amount exactly (no rounding
// loss from computing finisherCut independently).
func SplitTaskAmount(amount decimal.Decimal) (platformCut, finisherCut decimal.Decimal) {
	platformCut = amount.Mul(platformTaskCutPct).Round(6)
	finisherCut = amount.Sub(platformCut)
	return
}

// QueuePayout records a Payout row in 'pending' status and emits a
// learning OutcomeEvent. The actual money transfer to the finisher
// happens later via payOutFinisher (stub).
func (p *Payouts) QueuePayout(ctx context.Context, taskID, finisherID string, amount decimal.Decimal) (Payout, error) {
	platformCut, finisherCut := SplitTaskAmount(amount)
	payout, err := p.svc.RecordPayout(ctx, Payout{
		TaskID:         taskID,
		FinisherID:     finisherID,
		AmountUSD:      amount,
		FinisherCutUSD: finisherCut,
		PlatformCutUSD: platformCut,
		Status:         "pending",
		CreatedAt:      time.Now().UTC(),
	})
	if err != nil {
		return Payout{}, err
	}
	learning.Publish(ctx, learning.OutcomeEvent{
		Kind: learning.OutcomeKind("guild.payout.queued"),
		Attributes: map[string]any{
			"task_id":          taskID,
			"finisher_id":      finisherID,
			"amount_usd":       amount.String(),
			"finisher_cut_usd": finisherCut.String(),
			"platform_cut_usd": platformCut.String(),
		},
		CostUSD:   learning.DecimalPtr(amount),
		MarginUSD: learning.DecimalPtr(platformCut),
		Success:   learning.BoolPtr(true),
	})
	if err := p.payOutFinisher(ctx, payout); err != nil {
		// Best-effort: the queued row is the durable record. The
		// provisioning agent's wireup will retry from there.
		p.logger.Warn().Err(err).
			Str("payout_id", payout.ID).
			Str("finisher_id", finisherID).
			Str("amount_usd", amount.String()).
			Msg("guild: payOutFinisher stub returned non-nil; provisioning agent must wire the real transfer")
	}
	return payout, nil
}

// payOutFinisher drives the rail-side transfer when a transferer is
// wired. The flow:
//
//  1. Issue a Stripe Transfer for FinisherCutUSD to the finisher's
//     connected account, keyed by the payout id (idempotent).
//  2. On success, flip the guild_payouts row to 'paid' and stamp the
//     transfer ref + completed_at.
//  3. On failure, flip to 'failed' (leaving the row for manual follow-
//     up) and return the error so QueuePayout can log it.
//
// When no transferer is wired the method is a documented no-op: the
// queued 'pending' row is the durable hand-off so a later boot with
// the rail enabled can settle it.
func (p *Payouts) payOutFinisher(ctx context.Context, payout Payout) error {
	if p.transferer == nil {
		p.logger.Info().
			Str("payout_id", payout.ID).
			Str("finisher_id", payout.FinisherID).
			Str("amount_usd", payout.FinisherCutUSD.String()).
			Msg("guild: payout queued; no transferer wired (provisioning rail disabled)")
		return nil
	}
	ref, err := p.transferer.Transfer(ctx, payout.FinisherID, payout.FinisherCutUSD, "guild-payout-"+payout.ID)
	if err != nil {
		if _, uerr := p.svc.UpdatePayoutStatus(ctx, payout.ID, "failed", ""); uerr != nil {
			p.logger.Warn().Err(uerr).Str("payout_id", payout.ID).Msg("guild: mark payout failed errored")
		}
		return err
	}
	if _, err := p.svc.UpdatePayoutStatus(ctx, payout.ID, "paid", ref); err != nil {
		return err
	}
	p.logger.Info().
		Str("payout_id", payout.ID).
		Str("finisher_id", payout.FinisherID).
		Str("transfer_ref", ref).
		Str("amount_usd", payout.FinisherCutUSD.String()).
		Msg("guild: finisher payout transferred")
	return nil
}
