package wallet

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// Reconciler closes the rare-but-real gap between vendor settlement
// and our local wallet_topups state. Every Interval it sweeps
// pending rows older than Threshold, asks the originating provider
// for the session's current status, and either credits the wallet
// (paid) or marks the row failed (expired / canceled). The cron is a
// safety net — the primary credit path is still the webhook — but
// it converts the "missed webhook means lost revenue" risk into a
// merely delayed-by-Interval credit.
//
// The reconciler is conservative on the credit side: it only ever
// flips pending → succeeded via Service.TopUp, which is idempotent on
// session id. A late-arriving webhook for the same session is a
// no-op. Failed transitions are also idempotent (MarkFailed only
// touches 'pending' rows).
type Reconciler struct {
	svc       Service
	registry  *TopperRegistry
	logger    zerolog.Logger
	threshold time.Duration
	interval  time.Duration

	mu      sync.Mutex
	running bool
}

// ReconcilerOpts configures the cron cadence. Threshold should be
// larger than the vendor's typical webhook settlement window (~10
// minutes for Stripe; Paddle is similar) so we don't race a webhook
// that's about to land. Interval controls how often the sweep runs;
// 5 minutes is a sensible default — short enough to keep the
// missed-webhook window small, long enough to keep vendor API
// pressure negligible.
type ReconcilerOpts struct {
	Threshold time.Duration
	Interval  time.Duration
	Logger    zerolog.Logger
}

// NewReconciler builds a reconciler. Zero values for Threshold or
// Interval fall back to the production defaults (10 min / 5 min).
func NewReconciler(svc Service, registry *TopperRegistry, opts ReconcilerOpts) *Reconciler {
	if opts.Threshold <= 0 {
		opts.Threshold = 10 * time.Minute
	}
	if opts.Interval <= 0 {
		opts.Interval = 5 * time.Minute
	}
	return &Reconciler{
		svc:       svc,
		registry:  registry,
		logger:    opts.Logger,
		threshold: opts.Threshold,
		interval:  opts.Interval,
	}
}

// Start runs the cron loop until ctx is cancelled. Idempotent — a
// second Start call on the same Reconciler returns immediately so
// wireup that double-invokes (Temporal restart, hot reload) cannot
// spawn duplicate sweepers.
func (r *Reconciler) Start(ctx context.Context) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	if r.svc == nil || r.registry == nil || !r.registry.Enabled() {
		r.logger.Warn().Msg("wallet reconcile: not configured (no service or no enabled topper); cron skipped")
		return
	}
	r.logger.Info().
		Dur("interval", r.interval).
		Dur("threshold", r.threshold).
		Msg("wallet reconcile: started")

	// Run once at startup so a fresh boot catches anything in-flight
	// from the previous instance, then tick on Interval.
	if err := r.RunOnce(ctx); err != nil {
		r.logger.Warn().Err(err).Msg("wallet reconcile: initial sweep failed")
	}
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("wallet reconcile: stopped")
			return
		case <-t.C:
			if err := r.RunOnce(ctx); err != nil {
				r.logger.Warn().Err(err).Msg("wallet reconcile: sweep failed")
			}
		}
	}
}

// RunOnce performs a single sweep. Exposed for ops tooling and the
// admin RunReconciler GraphQL mutation (future). Returns the first
// fatal error; per-row errors are logged and swallowed so one bad
// session never stalls the whole sweep.
func (r *Reconciler) RunOnce(ctx context.Context) error {
	pending, err := r.svc.ListStalePending(ctx, r.threshold)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	r.logger.Info().Int("count", len(pending)).Msg("wallet reconcile: sweeping stale pending rows")
	for _, t := range pending {
		r.reconcileOne(ctx, t)
	}
	return nil
}

// reconcileOne handles a single stale row. Logs only — never returns
// errors — so a vendor outage on row N doesn't block rows N+1..M.
func (r *Reconciler) reconcileOne(ctx context.Context, row TopUp) {
	provider := row.Provider
	if provider == "" {
		provider = ProviderFromSessionID(row.StripeSessionID)
	}
	topper, err := r.registry.ByName(provider)
	if err != nil {
		r.logger.Warn().Err(err).
			Str("provider", provider).
			Str("session_id", row.StripeSessionID).
			Msg("wallet reconcile: provider unavailable; row left pending")
		return
	}
	result, err := topper.VerifySession(ctx, row.StripeSessionID)
	if err != nil {
		r.logger.Warn().Err(err).
			Str("provider", provider).
			Str("session_id", row.StripeSessionID).
			Msg("wallet reconcile: verify failed; will retry next sweep")
		return
	}
	switch result.Status {
	case VerifyPaid:
		amount := row.AmountUSD
		// Trust the vendor-reported amount when it's positive — the
		// pending row amount was set at checkout creation, but the
		// final settled amount may differ (currency conversion, etc.).
		if result.Amount.IsPositive() {
			amount = result.Amount
		}
		if err := r.svc.TopUp(ctx, row.TenantID, amount, row.StripeSessionID); err != nil {
			r.logger.Error().Err(err).
				Str("provider", provider).
				Str("session_id", row.StripeSessionID).
				Str("tenant_id", row.TenantID).
				Str("amount_usd", amount.String()).
				Msg("wallet reconcile: TopUp failed for paid session; manual intervention required")
			metrics.IncWalletReconcileError(provider)
			return
		}
		r.logger.Warn().
			Str("provider", provider).
			Str("session_id", row.StripeSessionID).
			Str("tenant_id", row.TenantID).
			Str("amount_usd", amount.String()).
			Msg("wallet reconcile: credited via reconcile (webhook missed)")
		metrics.IncWalletReconcileRecovered(provider)
	case VerifyExpired, VerifyFailed:
		if err := r.svc.MarkFailed(ctx, row.StripeSessionID); err != nil {
			r.logger.Error().Err(err).
				Str("provider", provider).
				Str("session_id", row.StripeSessionID).
				Msg("wallet reconcile: MarkFailed failed")
			metrics.IncWalletReconcileError(provider)
			return
		}
		r.logger.Info().
			Str("provider", provider).
			Str("session_id", row.StripeSessionID).
			Str("status", string(result.Status)).
			Msg("wallet reconcile: marked failed")
	case VerifyOpen:
		// Still in-flight on the vendor side — leave it for the next
		// sweep. Log at debug so a long-abandoned checkout doesn't
		// spam the warning channel.
		r.logger.Debug().
			Str("provider", provider).
			Str("session_id", row.StripeSessionID).
			Msg("wallet reconcile: still open on vendor; deferred")
	}
}
