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
	svc    Service
	logger zerolog.Logger
}

// NewPayouts builds the payouts helper bound to the guild store.
func NewPayouts(svc Service, logger zerolog.Logger) *Payouts {
	return &Payouts{svc: svc, logger: logger}
}

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

// payOutFinisher is the STUB the provisioning agent will replace. It
// MUST eventually:
//
//  1. Look up the finisher's connected Stripe account id from the
//     provisioning vault (acct_*).
//  2. Issue a Stripe Transfer for FinisherCutUSD against the platform
//     balance, scoped to that connected account.
//  3. On success, flip the guild_payouts row from 'pending' to 'paid'
//     and stamp completed_at.
//  4. On terminal failure (bad account, insufficient platform
//     balance, manual review block), flip to 'failed' and surface a
//     notification to the finisher.
//
// Until that wireup lands, this method is a documented no-op. We log
// at info so an operator watching the boot log sees the stub fire.
//
// TODO(provisioning-agent): replace this stub with the real Stripe
// Connect transfer via the ProvisioningVault. The Payout row is the
// hand-off contract — read it from guild_payouts WHERE status='pending'
// and drive the transfer.
func (p *Payouts) payOutFinisher(ctx context.Context, payout Payout) error {
	p.logger.Info().
		Str("payout_id", payout.ID).
		Str("finisher_id", payout.FinisherID).
		Str("amount_usd", payout.FinisherCutUSD.String()).
		Msg("guild: payOutFinisher stub — provisioning agent must wire Stripe Connect transfer")
	return nil
}
