package deploy

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
)

// SystemUser is the synthetic UserRef the expiry sweeper presents to
// Service.Decide when it flips a pending approval to its terminal
// "expired" state. UserID is namespaced with the "system:" prefix so
// audit consumers can trivially separate operator-driven decisions
// from background sweep activity.
var SystemUser = UserRef{UserID: "system:sweeper", TenantID: ""}

// defaultSweepInterval is the cadence Sweeper.Run polls every tenant's
// pending approval set at when NewSweeper is constructed with a
// zero-value interval. 60s matches the V22 deploy lifecycle plan's
// "approvals never linger more than a minute past expiry" budget.
const defaultSweepInterval = 60 * time.Second

// Sweeper walks the deploy_approvals table on a fixed cadence and
// transitions pending rows whose ExpiresAt has elapsed to the
// canonical "expired" terminal state via Service.Decide.
//
// The sweeper exists because Service.Decide and Service.Promote both
// already enforce expiry inline on the request path — but operator
// dashboards and the deployFeed subscription need a real
// approval_decided event for expired rows even when no user ever
// returned to the request. The sweep guarantees that signal lands at
// most one interval after the deadline.
//
// Run is the entry point; the integration agent owns spawning a
// goroutine that calls it and propagating ctx cancellation at
// orchestrator shutdown.
type Sweeper struct {
	svc      Service
	log      zerolog.Logger
	interval time.Duration
}

// NewSweeper wires the sweeper to a Service. interval <= 0 falls back
// to defaultSweepInterval (60s); the integration agent overrides via
// env (IRONFLYER_DEPLOY_APPROVAL_SWEEP_INTERVAL_SECONDS).
func NewSweeper(svc Service, log zerolog.Logger, interval time.Duration) *Sweeper {
	if interval <= 0 {
		interval = defaultSweepInterval
	}
	return &Sweeper{
		svc:      svc,
		log:      log.With().Str("subsystem", "deploy.sweeper").Logger(),
		interval: interval,
	}
}

// Run blocks until ctx is cancelled, calling sweep on every tick. A
// single sweep failure does NOT stop the loop — we log and keep going
// so a transient Postgres glitch doesn't silently disable expiry.
func (s *Sweeper) Run(ctx context.Context) error {
	if s == nil || s.svc == nil {
		return errors.New("deploy.Sweeper: nil service")
	}
	t := time.NewTicker(s.interval)
	defer t.Stop()
	s.log.Info().Dur("interval", s.interval).Msg("approval expiry sweeper started")
	// Sweep once at start so freshly-restarted orchestrators flush
	// approvals that expired during downtime without waiting a full
	// interval.
	s.sweep(ctx)
	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("approval expiry sweeper stopped")
			return ctx.Err()
		case <-t.C:
			s.sweep(ctx)
		}
	}
}

// sweep iterates every tenant with pending approvals and flips
// expired rows. Errors are logged and swallowed so one bad tenant or
// one transient DB blip does not abort the whole tick.
func (s *Sweeper) sweep(ctx context.Context) {
	now := time.Now().UTC()
	tenants, err := s.svc.TenantsWithPendingApprovals(ctx)
	if err != nil {
		s.log.Warn().Err(err).Msg("approval sweep: tenants lookup failed")
		return
	}
	if len(tenants) == 0 {
		return
	}
	for _, tenant := range tenants {
		s.sweepTenant(ctx, tenant, now)
	}
}

func (s *Sweeper) sweepTenant(ctx context.Context, tenant string, now time.Time) {
	approvals, err := s.svc.PendingApprovals(ctx, tenant)
	if err != nil {
		s.log.Warn().Err(err).Str("tenant", tenant).Msg("approval sweep: pending lookup failed")
		return
	}
	for _, a := range approvals {
		if a.ExpiresAt.IsZero() || !a.ExpiresAt.Before(now) {
			continue
		}
		// Service.Decide handles the FSM (and writes the
		// approval_expired event when the row is already past
		// expires_at). We surface the sentinel ErrApprovalExpired as
		// the expected path; everything else is a real failure.
		//
		// We pass DecisionReject as the wire verb because Decide only
		// accepts the approve/reject vocabulary — its inline expiry
		// check preempts the verb whenever ExpiresAt has already
		// passed, which is exactly what we want here. The note carries
		// the sweep's intent so audit consumers can distinguish a
		// background expiry from an operator rejection.
		if _, err := s.svc.Decide(ctx, a.ID, SystemUser, DecisionReject, "expiry_sweeper"); err != nil {
			if errors.Is(err, ErrApprovalExpired) {
				continue
			}
			s.log.Warn().
				Err(err).
				Str("tenant", tenant).
				Str("approval_id", a.ID).
				Msg("approval sweep: decide failed")
		}
	}
}
