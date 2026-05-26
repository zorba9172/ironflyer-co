package storage

// billing.go — periodic storage cost metering. Reads per-tenant bytes
// from a pluggable UsageSource, prices them at the configured
// per-GB-month rate, and writes typed EntryStorageCost rows to the
// V22 ledger so the platform finally knows what storage is costing it.
//
// Why a separate ticker rather than charging at the object write site:
// most S3-compatible backends bill by capacity-hour, not by request.
// Capturing the cost at write time over-bills (artefacts that get
// deleted minutes later still pay an hour) and under-bills (long-
// lived blobs never get re-charged after the initial write). A 1-hour
// tick that asks "how much does this tenant currently occupy" is the
// honest metering shape that mirrors how the cloud actually bills us.
//
// The Biller is no-op until WIRED to a real UsageSource. Today no
// orchestrator caller stores objects in S3 (see s3client.go header),
// so the default wireup uses NoopUsageSource which simply logs once
// at startup that metering is armed but inert. When the first real
// caller lands (audit export bucket, replay artefacts), drop in a
// MinIOUsageSource / S3UsageSource without touching the Biller
// itself.

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/ledger"
)

// UsageSource exposes per-tenant byte counts at the current moment.
// Implementations talk to the configured object store (MinIO, R2,
// AWS S3, Spaces) and return a snapshot keyed by tenant UUID. A nil
// or empty map is legal and means "no tenants have storage today".
type UsageSource interface {
	PerTenantBytes(ctx context.Context) (map[uuid.UUID]int64, error)
}

// NoopUsageSource is the default until a real bucket scanner lands.
// Returns an empty map so the Biller writes no entries.
type NoopUsageSource struct{}

func (NoopUsageSource) PerTenantBytes(_ context.Context) (map[uuid.UUID]int64, error) {
	return map[uuid.UUID]int64{}, nil
}

// Rate is the configured price per GB-month for the storage backend.
// Defaults to $0.023/GB/month (AWS S3 Standard) and can be lowered
// via env when the backend is R2 ($0.015) or MinIO self-hosted
// (effectively zero, but $0.005 is a sane proxy for the underlying
// host disk + RAID amortisation so dashboards still see *something*
// and the operator doesn't believe storage is free).
type Rate struct {
	USDPerGBMonth decimal.Decimal
}

// DefaultRate resolves the rate per the IRONFLYER_STORAGE_USD_PER_GB_MONTH
// env, falling back to AWS S3 Standard list. The auto-backend mapping
// for R2 / MinIO / Spaces lets the operator just set S3_BACKEND and
// pick up a sensible rate without manually overriding the numeric env.
func DefaultRate() Rate {
	if raw := strings.TrimSpace(os.Getenv("IRONFLYER_STORAGE_USD_PER_GB_MONTH")); raw != "" {
		if f, err := strconv.ParseFloat(raw, 64); err == nil && f > 0 {
			return Rate{USDPerGBMonth: decimal.NewFromFloat(f)}
		}
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("S3_BACKEND"))) {
	case "r2":
		return Rate{USDPerGBMonth: decimal.NewFromFloat(0.015)}
	case "minio":
		return Rate{USDPerGBMonth: decimal.NewFromFloat(0.005)}
	case "spaces":
		return Rate{USDPerGBMonth: decimal.NewFromFloat(0.020)}
	default:
		return Rate{USDPerGBMonth: decimal.NewFromFloat(0.023)}
	}
}

// Biller is the periodic ticker. Construct with NewBiller, then call
// Start. Start spawns one goroutine that runs Tick on the supplied
// cadence until ctx is cancelled. Tick is also exported so an
// operator command can force an out-of-band reconciliation.
type Biller struct {
	source UsageSource
	rate   Rate
	ledger ledger.Service
	logger zerolog.Logger
	tick   time.Duration

	mu       sync.Mutex
	lastRun  time.Time
}

// NewBiller wires the dependencies. tick must be > 0 — passing zero
// falls back to one hour, which matches how AWS/R2 bill capacity. A
// nil ledger or nil source makes the Biller inert (Start returns
// without spawning a goroutine), which is what we want in tests.
func NewBiller(source UsageSource, lg ledger.Service, rate Rate, tick time.Duration, logger zerolog.Logger) *Biller {
	if tick <= 0 {
		tick = time.Hour
	}
	return &Biller{
		source: source,
		rate:   rate,
		ledger: lg,
		logger: logger,
		tick:   tick,
	}
}

// Start runs the ticker until ctx is cancelled. Returns immediately
// when any required dependency is nil so the caller can wire
// optimistically and not pay the goroutine cost in dev.
func (b *Biller) Start(ctx context.Context) {
	if b == nil || b.source == nil || b.ledger == nil {
		return
	}
	go func() {
		t := time.NewTicker(b.tick)
		defer t.Stop()
		// Run one tick immediately on startup so dashboards aren't
		// blank for an hour after a fresh boot.
		b.runTick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				b.runTick(ctx)
			}
		}
	}()
}

// Tick performs one reconciliation pass. Exposed so an operator
// command (or the smoke script) can trigger an out-of-band run
// without waiting for the ticker.
func (b *Biller) Tick(ctx context.Context) error {
	if b == nil || b.source == nil || b.ledger == nil {
		return nil
	}
	b.runTick(ctx)
	return nil
}

// runTick is the actual work. Reads the source snapshot, prorates
// each tenant's bytes onto the elapsed hours since the last tick,
// and writes one EntryStorageCost per tenant. Errors are logged but
// not retried — the next tick will pick the same tenants up at the
// fresh snapshot. We never block startup or other tenants on one
// tenant's failure.
func (b *Biller) runTick(ctx context.Context) {
	b.mu.Lock()
	now := time.Now().UTC()
	elapsed := b.tick
	if !b.lastRun.IsZero() {
		// Honour real wall-clock elapsed so a sleep/wake doesn't
		// over-charge with a single tick worth of cost.
		elapsed = now.Sub(b.lastRun)
		if elapsed <= 0 || elapsed > 24*time.Hour {
			// Pathological clock skew or first-run-after-resume —
			// fall back to one tick's worth.
			elapsed = b.tick
		}
	}
	b.lastRun = now
	b.mu.Unlock()

	usage, err := b.source.PerTenantBytes(ctx)
	if err != nil {
		b.logger.Warn().Err(err).Msg("storage biller: usage source failed")
		return
	}
	if len(usage) == 0 {
		return
	}
	hoursPerMonth := decimal.NewFromInt(730)
	elapsedHours := decimal.NewFromFloat(elapsed.Hours())
	rate := b.rate.USDPerGBMonth
	for tenant, bytes := range usage {
		if bytes <= 0 {
			continue
		}
		gb := decimal.NewFromInt(bytes).Div(decimal.NewFromInt(1_000_000_000))
		// cost = gb * (rate / 730 hours) * elapsed_hours
		cost := gb.Mul(rate).Div(hoursPerMonth).Mul(elapsedHours)
		if !cost.IsPositive() {
			continue
		}
		entry := ledger.Entry{
			TenantID:       tenant,
			EntryType:      ledger.EntryStorageCost,
			Direction:      ledger.DebitDirection,
			AmountUSD:      cost.Round(6),
			Billable:       true,
			MarginRelevant: true,
			Metadata: map[string]any{
				"bytes":              bytes,
				"gb":                 gb.String(),
				"rate_usd_per_gb_mo": rate.String(),
				"elapsed_seconds":    int(elapsed.Seconds()),
				"backend":            strings.ToLower(strings.TrimSpace(os.Getenv("S3_BACKEND"))),
			},
			OpKey: "storage_cost:" + tenant.String() + ":" + now.Format("2006-01-02T15"),
		}
		if _, err := b.ledger.Write(ctx, entry); err != nil {
			b.logger.Warn().
				Err(err).
				Str("tenant", tenant.String()).
				Int64("bytes", bytes).
				Msg("storage biller: ledger write failed")
		}
	}
}
