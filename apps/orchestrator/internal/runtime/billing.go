// Sandbox billing: every workspace allocation that carries an
// execution id ticks N seconds of sandbox time into the ledger via
// the execution.TickReporter Agent 9 built. Hours of compute should
// land as `sandbox_cost` debits attributed to the execution row +
// tenant ledger.
//
// This is the orchestrator side of the closing-the-loop work (V22
// Agent 12, Option A). The runtime service itself stays oblivious to
// money — the orchestrator already knows the workspace lifetime
// because it owns the allocation call, so we wrap that span with a
// ticker and convert wall-clock seconds into a USD cost via the
// per-hour rate.

package runtime

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/execution"
)

// defaultSandboxRateUSDPerHour is the v1 list rate. Override via
// IRONFLYER_SANDBOX_RATE_USD_PER_HOUR for tuning without a redeploy.
// 30 cents/hour matches the runtime-cost line item in the V22 proof
// pack (`01-execution-unit-economics`): assumes a small Docker
// workspace amortised against a 50% packing factor.
const defaultSandboxRateUSDPerHour = 0.30

// defaultSandboxTickInterval is the granularity of the ledger debit.
// 60s is the v1 choice — coarse enough that the ledger doesn't get
// flooded, fine enough that a 5-minute build still lands ~5 debits so
// the dashboards can plot cost-over-time.
const defaultSandboxTickInterval = 60 * time.Second

// SandboxBiller turns workspace lifetime into ledger debits. One
// instance is constructed during boot; Track is called by the
// finisher engine each time a workspace is allocated for an
// execution. Track returns a `stop` func the engine defers — the
// goroutine is bound to that span and emits a final partial-interval
// tick on stop so a 90-second workspace still bills 90 seconds of
// sandbox time (not the floor(90/60)=1 interval the naive
// implementation would produce).
type SandboxBiller struct {
	reporter execution.TickReporter
	rate     decimal.Decimal
	interval time.Duration
	log      zerolog.Logger
}

// NewSandboxBiller constructs the biller. A nil reporter is legal —
// Track becomes a no-op so dev installs without an execution backend
// still boot cleanly. The rate is sourced from
// IRONFLYER_SANDBOX_RATE_USD_PER_HOUR with a 30 cent/hour fallback.
func NewSandboxBiller(reporter execution.TickReporter, log zerolog.Logger) *SandboxBiller {
	return &SandboxBiller{
		reporter: reporter,
		rate:     resolveSandboxRate(),
		interval: defaultSandboxTickInterval,
		log:      log,
	}
}

// WithRate overrides the per-hour rate. Returns the receiver for
// chained config.
func (b *SandboxBiller) WithRate(rate decimal.Decimal) *SandboxBiller {
	if b == nil || !rate.IsPositive() {
		return b
	}
	b.rate = rate
	return b
}

// WithInterval overrides the tick cadence. Returns the receiver for
// chained config. Sub-second intervals are clamped to 1s — finer
// granularity is pointless against the ledger's USD precision.
func (b *SandboxBiller) WithInterval(d time.Duration) *SandboxBiller {
	if b == nil || d <= 0 {
		return b
	}
	if d < time.Second {
		d = time.Second
	}
	b.interval = d
	return b
}

// Rate returns the configured per-hour rate. Surfaced for logging.
func (b *SandboxBiller) Rate() decimal.Decimal {
	if b == nil {
		return decimal.Zero
	}
	return b.rate
}

// Interval returns the configured tick cadence. Surfaced for logging.
func (b *SandboxBiller) Interval() time.Duration {
	if b == nil {
		return 0
	}
	return b.interval
}

// Track starts a goroutine that every `interval` reports one tick of
// sandbox usage for executionID. The returned stop func cancels the
// goroutine and flushes a final partial-interval tick covering the
// time since the last full tick — so the ledger debits match the
// real wall-clock workspace lifetime, not just the floored multiples
// of `interval`.
//
// Calling Track with a nil receiver, empty executionID, nil
// reporter, or non-positive rate returns a no-op stop — the caller
// can defer it unconditionally.
func (b *SandboxBiller) Track(ctx context.Context, executionID, workspaceID string) (stop func()) {
	if b == nil || b.reporter == nil || executionID == "" || !b.rate.IsPositive() {
		return func() {}
	}

	// Decouple the ticker from the request ctx — we want the final
	// flush tick to fire even if the caller's ctx has already been
	// cancelled. The reporter calls below use a short, fresh ctx so
	// the ledger write isn't aborted by the request lifecycle.
	tickerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				b.log.Warn().
					Interface("panic", r).
					Str("execution_id", executionID).
					Str("workspace_id", workspaceID).
					Msg("sandbox biller goroutine panic recovered")
			}
		}()

		ticker := time.NewTicker(b.interval)
		defer ticker.Stop()

		lastTick := time.Now()
		intervalSec := int(b.interval / time.Second)
		if intervalSec <= 0 {
			intervalSec = 1
		}

		for {
			select {
			case <-tickerCtx.Done():
				// Final partial-interval tick — bills the time since the
				// last full tick so a short-lived workspace still lands
				// non-zero on the ledger.
				partial := int(time.Since(lastTick) / time.Second)
				if partial > 0 {
					b.report(executionID, workspaceID, partial)
				}
				return
			case t := <-ticker.C:
				b.report(executionID, workspaceID, intervalSec)
				lastTick = t
			}
		}
	}()

	return func() {
		cancel()
		// Wait briefly for the goroutine to flush the final tick so
		// the ledger debit is durable before the caller returns. The
		// timeout protects against a hung reporter — if the final
		// write is taking longer than a couple of seconds, give up
		// and let the goroutine drain in the background.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

// report posts one sandbox tick to the execution.TickReporter. All
// errors are warned and swallowed — billing must never fail a
// running workspace.
func (b *SandboxBiller) report(executionID, workspaceID string, durationSec int) {
	if durationSec <= 0 {
		return
	}
	// Each ledger write gets its own short ctx so a slow reporter
	// can't stall the goroutine indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			b.log.Warn().
				Interface("panic", r).
				Str("execution_id", executionID).
				Str("workspace_id", workspaceID).
				Msg("sandbox biller report panic recovered")
		}
	}()
	if err := b.reporter.ReportSandboxTick(ctx, executionID, durationSec, b.rate); err != nil {
		b.log.Warn().Err(err).
			Str("execution_id", executionID).
			Str("workspace_id", workspaceID).
			Int("duration_sec", durationSec).
			Str("rate_usd_per_hour", b.rate.String()).
			Msg("sandbox tick report failed")
	}
}

// resolveSandboxRate returns the per-hour USD rate, honouring
// IRONFLYER_SANDBOX_RATE_USD_PER_HOUR when set to a positive float.
func resolveSandboxRate() decimal.Decimal {
	if raw := os.Getenv("IRONFLYER_SANDBOX_RATE_USD_PER_HOUR"); raw != "" {
		if f, err := strconv.ParseFloat(raw, 64); err == nil && f > 0 {
			return decimal.NewFromFloat(f)
		}
	}
	return decimal.NewFromFloat(defaultSandboxRateUSDPerHour)
}
