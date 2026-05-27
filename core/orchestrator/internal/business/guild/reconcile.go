package guild

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// Reconciler is the periodic sweep that keeps the guild state honest
// when callers misbehave or the UI never returns. Two responsibilities:
//
//   - Expire abandoned tasks (open / bidding past their SLA) — release
//     the requestor's hold so the wallet is not stuck holding funds
//     forever, and emit OutcomeEvent so the dashboard can render it.
//   - Withdraw stale open bids (no movement past BidStaleAfter) so a
//     finisher's offer list does not balloon with rotting bids.
//
// Cadence default: every 5 minutes. Stale-bid threshold: 24 hours.
// Both tunable via ReconcilerOpts.
type Reconciler struct {
	svc    Service
	escrow *Escrow
	logger zerolog.Logger
	opts   ReconcilerOpts

	mu      sync.Mutex
	running bool
}

// ReconcilerOpts tunes the cron cadence and freshness windows.
type ReconcilerOpts struct {
	Interval      time.Duration
	BidStaleAfter time.Duration
	TaskGracePast time.Duration
	Logger        zerolog.Logger
}

// NewReconciler builds a reconciler with defaults applied for zero
// values.
func NewReconciler(svc Service, escrow *Escrow, opts ReconcilerOpts) *Reconciler {
	if opts.Interval <= 0 {
		opts.Interval = 5 * time.Minute
	}
	if opts.BidStaleAfter <= 0 {
		opts.BidStaleAfter = 24 * time.Hour
	}
	return &Reconciler{svc: svc, escrow: escrow, logger: opts.Logger, opts: opts}
}

// Start kicks the cron loop. Idempotent — a second Start no-ops so a
// hot reload cannot spawn duplicate sweepers.
func (r *Reconciler) Start(ctx context.Context) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()
	r.logger.Info().Dur("interval", r.opts.Interval).Msg("guild reconcile: started")
	if err := r.RunOnce(ctx); err != nil {
		r.logger.Warn().Err(err).Msg("guild reconcile: initial sweep failed")
	}
	t := time.NewTicker(r.opts.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("guild reconcile: stopped")
			return
		case <-t.C:
			if err := r.RunOnce(ctx); err != nil {
				r.logger.Warn().Err(err).Msg("guild reconcile: sweep failed")
			}
		}
	}
}

// RunOnce executes one full sweep. Exposed for ops tooling.
func (r *Reconciler) RunOnce(ctx context.Context) error {
	r.expireTasks(ctx)
	r.withdrawStaleBids(ctx)
	return nil
}

func (r *Reconciler) expireTasks(ctx context.Context) {
	tasks, err := r.svc.ListAbandonedTasks(ctx, 0)
	if err != nil {
		r.logger.Warn().Err(err).Msg("guild reconcile: list abandoned tasks failed")
		return
	}
	for _, t := range tasks {
		opKey := string(OpExpireTask) + ":" + t.ID
		if prior, ok, _ := r.svc.RecallOp(ctx, opKey); ok && prior.Status == "succeeded" {
			continue
		}
		if err := r.escrow.ReleaseFloor(ctx, t.TenantID, t.PriceUSDFloor); err != nil {
			r.logger.Warn().Err(err).Str("task_id", t.ID).Msg("guild reconcile: release hold failed")
			continue
		}
		if _, err := r.svc.UpdateTaskStatus(ctx, t.ID, TaskStatusExpired, nil); err != nil {
			r.logger.Warn().Err(err).Str("task_id", t.ID).Msg("guild reconcile: status update failed")
			continue
		}
		_ = r.svc.RecordOp(ctx, opKey, string(OpExpireTask), t.PriceUSDFloor, "succeeded", "")
		learning.Publish(ctx, learning.OutcomeEvent{
			TenantID: t.TenantID,
			Kind:     learning.OutcomeKind("guild.task.expired"),
			Attributes: map[string]any{
				"task_id":    t.ID,
				"project_id": t.ProjectID,
				"floor_usd":  t.PriceUSDFloor.String(),
			},
			Success: learning.BoolPtr(false),
		})
		r.logger.Info().Str("task_id", t.ID).Msg("guild reconcile: task expired, hold released")
	}
}

func (r *Reconciler) withdrawStaleBids(ctx context.Context) {
	bids, err := r.svc.ListStaleOpenBids(ctx, int(r.opts.BidStaleAfter.Seconds()))
	if err != nil {
		r.logger.Warn().Err(err).Msg("guild reconcile: list stale bids failed")
		return
	}
	for _, b := range bids {
		if _, err := r.svc.UpdateBidStatus(ctx, b.ID, BidStatusWithdrawn); err != nil {
			r.logger.Warn().Err(err).Str("bid_id", b.ID).Msg("guild reconcile: withdraw failed")
			continue
		}
		learning.Publish(ctx, learning.OutcomeEvent{
			Kind: learning.OutcomeKind("guild.bid.withdrawn"),
			Attributes: map[string]any{
				"bid_id":  b.ID,
				"task_id": b.TaskID,
				"reason":  "stale",
			},
			Success: learning.BoolPtr(false),
		})
	}
}
