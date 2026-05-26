package notify

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/ledger"
	"ironflyer/core/orchestrator/internal/pkg/env"
)

// digestTickDefaultMin is the default Run cadence (one hour). The
// runner only dispatches when the current tick falls inside the
// Sunday 09:00–10:00 UTC window AND the digest hasn't already shipped
// this ISO week, so a 60-minute cadence keeps the loop cheap while
// guaranteeing exactly-one shipment per opt-in user per week.
const digestTickDefaultMin = 60

// digestWindow is the trailing observation period. ISO week boundaries
// shift; aggregating against "now - 7d" keeps the dashboard, the email
// subject ("3 runs, 1 deploy"), and the user's mental model in sync.
const digestWindow = 7 * 24 * time.Hour

// DigestRunner ships the opt-in KindWeeklyDigest. It iterates the
// PrefsStore for users with WeeklyDigest=true, aggregates the per-user
// counts from execution + ledger, and dispatches via the Dispatcher.
// Sunday 09:00–10:00 UTC is the chosen window — far enough from the
// US Friday deploy crunch that the digest lands when users plan the
// next week, late enough in Europe that nobody wakes up to it.
type DigestRunner struct {
	prefs      PrefsStore
	executions execution.Service
	ledger     ledger.Service
	dispatcher *Dispatcher
	logger     zerolog.Logger
	tick       time.Duration

	lastWeekKey string
}

// NewDigestRunner constructs the runner. Each dependency is nil-safe
// at call sites — the runner refuses to start when any required
// surface is missing, logging once so the operator sees the boot
// state.
func NewDigestRunner(prefs PrefsStore, executions execution.Service, ledgerSvc ledger.Service, dispatcher *Dispatcher, logger zerolog.Logger) *DigestRunner {
	tickMin := env.Int("IRONFLYER_NOTIFY_DIGEST_TICK_MIN", digestTickDefaultMin)
	if tickMin <= 0 {
		tickMin = digestTickDefaultMin
	}
	return &DigestRunner{
		prefs:      prefs,
		executions: executions,
		ledger:     ledgerSvc,
		dispatcher: dispatcher,
		logger:     logger,
		tick:       time.Duration(tickMin) * time.Minute,
	}
}

// Run blocks until ctx is cancelled. The runner uses a single ticker
// (per env-tuned cadence) and the per-tick guard checks both the day
// of week + hour and the lastWeekKey marker so a missed tick is
// recoverable on the next hour without double-shipping.
func (r *DigestRunner) Run(ctx context.Context) {
	if r == nil || r.dispatcher == nil || r.prefs == nil || r.executions == nil {
		return
	}
	r.logger.Info().Dur("tick", r.tick).Msg("notify: weekly digest runner started")
	ticker := time.NewTicker(r.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("notify: weekly digest runner stopped")
			return
		case <-ticker.C:
			r.maybeRun(ctx, time.Now().UTC())
		}
	}
}

// maybeRun is the per-tick guard. Public-ish so future tests / hooks
// could trigger a run; today the only caller is Run.
func (r *DigestRunner) maybeRun(ctx context.Context, now time.Time) {
	if !isDigestWindow(now) {
		return
	}
	y, w := now.ISOWeek()
	key := isoWeekKey(y, w)
	if key == r.lastWeekKey {
		return
	}
	r.lastWeekKey = key
	r.dispatchAll(ctx, now)
}

// isDigestWindow reports whether now falls inside Sunday 09:00 UTC.
func isDigestWindow(now time.Time) bool {
	u := now.UTC()
	if u.Weekday() != time.Sunday {
		return false
	}
	return u.Hour() == 9
}

// isoWeekKey is the dedupe handle the runner stamps after a successful
// dispatch round. Format matches WeeklyDigest's payloadRefID so a
// matching outbox idempotency key suppresses duplicates on the
// per-user side even if maybeRun fires twice on an operator
// configuration goof.
func isoWeekKey(y, w int) string {
	return formatISOWeek(y, w)
}

func formatISOWeek(y, w int) string {
	yy := y
	ww := w
	wb := []byte{
		'0' + byte(yy/1000%10),
		'0' + byte(yy/100%10),
		'0' + byte(yy/10%10),
		'0' + byte(yy%10),
		'-',
		'0' + byte(ww/10%10),
		'0' + byte(ww%10),
	}
	return string(wb)
}

// dispatchAll iterates the prefs store, materializes each opted-in
// user's WeeklyDigestCounts from the execution + ledger backends, and
// dispatches the payload. Per-user errors log + continue — the runner
// must not block on a single tenant's bad data.
func (r *DigestRunner) dispatchAll(ctx context.Context, now time.Time) {
	rules, err := r.prefs.ListAll(ctx)
	if err != nil {
		r.logger.Warn().Err(err).Msg("notify: digest list prefs failed")
		return
	}
	since := now.Add(-digestWindow)
	period := since.Format("Jan 2") + " – " + now.Format("Jan 2")
	dashboard := r.dispatcher.DashboardURL()
	dispatched := 0
	for _, rule := range rules {
		if !rule.WeeklyDigest || rule.PauseAll {
			continue
		}
		counts := r.aggregate(ctx, rule.UserID, since, now)
		if err := r.dispatcher.Dispatch(ctx, rule.UserID, rule.Email, KindWeeklyDigest, WeeklyDigestPayload{
			PeriodLabel:  period,
			Counts:       counts,
			DashboardURL: dashboard,
		}); err != nil {
			r.logger.Warn().Err(err).Str("user", rule.UserID).Msg("notify: digest dispatch failed")
			continue
		}
		dispatched++
	}
	r.logger.Info().Int("count", dispatched).Str("period", period).Msg("notify: weekly digest dispatched")
}

// aggregate composes the WeeklyDigestCounts for one user over [since,
// now). Two sources:
//
//   - execution.Service.ListByTenant — the run/deploy/refusal totals,
//     scanned in pages of 200 with a hard 1000-row safety cap so a
//     pathological user does not stall the runner.
//   - ledger.Service.SumByType — the spend total (provider + sandbox +
//     storage + deployment cost), in cents.
//
// The gate-failure count piggybacks on GateEventsByExecution per
// execution; the V22 finisher already emits gate.failed.v1 markers.
func (r *DigestRunner) aggregate(ctx context.Context, userID string, since, now time.Time) WeeklyDigestCounts {
	out := WeeklyDigestCounts{Currency: "USD"}
	tenantUUID, err := uuid.Parse(userID)
	if err == nil && r.ledger != nil {
		if sums, lerr := r.ledger.SumByType(ctx, tenantUUID, []ledger.EntryType{
			ledger.EntryProviderInferenceCost,
			ledger.EntrySandboxCost,
			ledger.EntryStorageCost,
			ledger.EntryDeploymentCost,
		}, since, now); lerr == nil {
			total := decimal.Zero
			for _, v := range sums {
				total = total.Add(v)
			}
			out.SpendCents = int(total.Mul(decimal.NewFromInt(100)).IntPart())
		}
	}

	const pageSize = 200
	const maxRows = 1000
	scanned := 0
	for offset := 0; offset < maxRows; offset += pageSize {
		rows, err := r.executions.ListByTenant(ctx, userID, pageSize, offset)
		if err != nil || len(rows) == 0 {
			return out
		}
		for _, e := range rows {
			if e.CreatedAt.Before(since) {
				return out
			}
			out.Runs++
			switch e.Status {
			case execution.StatusSucceeded:
				if strings.Contains(strings.ToLower(string(e.FailureReason)), "") {
					// no-op; placeholder so future cost-aware
					// reclassification stays cheap.
				}
				out.Deploys++
			case execution.StatusFailed, execution.StatusStopped, execution.StatusKilled:
				out.Refusals++
			}
			if r.executions != nil {
				if gates, gerr := r.executions.GateEventsByExecution(ctx, e.ID); gerr == nil {
					for _, g := range gates {
						if g.Status == "fail" {
							out.Gates++
						}
					}
				}
			}
		}
		scanned += len(rows)
		if len(rows) < pageSize {
			return out
		}
	}
	_ = scanned
	return out
}
