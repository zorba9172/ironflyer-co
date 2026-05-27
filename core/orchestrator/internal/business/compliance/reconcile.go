package compliance

import (
	"context"
	"time"
)

// ReconcilerOpts tunes the background loop. Zero values get sane
// defaults: monthly billing every 24h and nightly evaluation every
// 24h offset by 12h so the two sweeps don't pile onto the same tick.
type ReconcilerOpts struct {
	BillingInterval    time.Duration
	EvaluationInterval time.Duration
}

// Reconciler is the background loop that fires monthly billing and
// nightly evaluation across every enrolment. Construct with
// NewReconciler; the orchestrator calls Start once at boot and the
// loop exits on context cancellation.
type Reconciler struct {
	svc  *Service
	opts ReconcilerOpts
}

// NewReconciler builds the loop. Both intervals default to 24h.
func NewReconciler(svc *Service, opts ReconcilerOpts) *Reconciler {
	if opts.BillingInterval <= 0 {
		opts.BillingInterval = 24 * time.Hour
	}
	if opts.EvaluationInterval <= 0 {
		opts.EvaluationInterval = 24 * time.Hour
	}
	return &Reconciler{svc: svc, opts: opts}
}

// Start kicks the loop. Blocking — callers Go() it.
func (r *Reconciler) Start(ctx context.Context) {
	billTicker := time.NewTicker(r.opts.BillingInterval)
	defer billTicker.Stop()
	evalTicker := time.NewTicker(r.opts.EvaluationInterval)
	defer evalTicker.Stop()

	// Fire each leg once on boot so a fresh deploy doesn't wait a full
	// interval before the first sweep lands.
	r.bill(ctx)
	r.evaluate(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-billTicker.C:
			r.bill(ctx)
		case <-evalTicker.C:
			r.evaluate(ctx)
		}
	}
}

func (r *Reconciler) bill(ctx context.Context) {
	if r == nil || r.svc == nil {
		return
	}
	charged, soft, err := r.svc.RunMonthlyBilling(ctx)
	if err != nil {
		r.svc.logger.Error().Err(err).Msg("compliance reconciler: billing sweep errored")
		return
	}
	r.svc.logger.Info().
		Int("charged", charged).
		Int("soft_failures", soft).
		Msg("compliance reconciler: monthly billing sweep complete")
}

func (r *Reconciler) evaluate(ctx context.Context) {
	if r == nil || r.svc == nil {
		return
	}
	rows, err := r.svc.backend.ListAllEnrollments(ctx)
	if err != nil {
		r.svc.logger.Error().Err(err).Msg("compliance reconciler: list enrollments errored")
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		if _, err := r.svc.EvaluateAll(ctx, row.TenantID, row.ProjectID, row.FrameworkKey); err != nil {
			r.svc.logger.Warn().
				Err(err).
				Str("enrollment_id", row.ID).
				Msg("compliance reconciler: evaluation failed")
		}
	}
}
