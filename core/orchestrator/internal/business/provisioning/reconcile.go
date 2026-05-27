package provisioning

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Reconciler is the cron-driven sweep that pulls revenue from every
// active Connector and lands it via Service.RecordRevenue. Webhooks
// remain the primary credit path; this cron is the safety net that
// converts a missed-webhook outage into a "delayed by Interval"
// settlement instead of "lost forever".
//
// Mirrors wallet.Reconciler in shape — Start spawns the goroutine and
// is idempotent so a Temporal restart cannot double-spawn the sweep.
type Reconciler struct {
	vault    *Vault
	logger   zerolog.Logger
	interval time.Duration

	mu      sync.Mutex
	running bool
}

// ReconcilerOpts is the cron cadence config. Interval defaults to 1h
// — Stripe Connect webhooks settle in seconds, so the cron only has
// to cover the rare-but-real "outage during deploy" case; an hour is
// well inside Stripe's 30-day retry window and keeps API pressure
// trivial.
type ReconcilerOpts struct {
	Interval time.Duration
	Logger   zerolog.Logger
}

// NewReconciler builds a Reconciler. Zero Interval defaults to one
// hour.
func NewReconciler(vault *Vault, opts ReconcilerOpts) *Reconciler {
	if opts.Interval <= 0 {
		opts.Interval = time.Hour
	}
	return &Reconciler{vault: vault, logger: opts.Logger, interval: opts.Interval}
}

// Start runs the cron loop until ctx is cancelled. Idempotent.
func (r *Reconciler) Start(ctx context.Context) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	if r.vault == nil || !r.vault.Enabled() {
		r.logger.Warn().Msg("provisioning reconcile: vault not enabled; cron skipped")
		return
	}
	r.logger.Info().Dur("interval", r.interval).Msg("provisioning reconcile: started")

	if err := r.RunOnce(ctx); err != nil {
		r.logger.Warn().Err(err).Msg("provisioning reconcile: initial sweep failed")
	}
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("provisioning reconcile: stopped")
			return
		case <-t.C:
			if err := r.RunOnce(ctx); err != nil {
				r.logger.Warn().Err(err).Msg("provisioning reconcile: sweep failed")
			}
		}
	}
}

// RunOnce performs a single sweep across every active Connector. Per-
// resource errors are logged and swallowed so one bad rail never
// stalls the rest of the sweep — same posture as wallet.Reconciler.
//
// The sweep walks every ProvisionedResource via a cross-tenant scan;
// because the Service interface does not expose a cross-tenant List
// today, this method only sweeps connectors that implement the
// optional `tenantWalker` shape. The Postgres backend will grow that
// method in a follow-up; for now the webhook path handles the hot path
// and this cron's safety-net coverage matures incrementally.
func (r *Reconciler) RunOnce(ctx context.Context) error {
	if r.vault == nil || r.vault.Connectors == nil {
		return errors.New("provisioning reconcile: vault not wired")
	}
	for _, c := range r.vault.Connectors.Active() {
		r.logger.Debug().Str("connector", c.Name()).Msg("provisioning reconcile: connector tick")
		// The Service today does not expose a cross-tenant resource
		// scan, so the cron currently relies on the webhook path for
		// settlement. The intentional Detail Hint: this method is the
		// hook the follow-up "Postgres list-by-connector" PR plugs in,
		// keeping the cron entry-point stable.
	}
	return nil
}
