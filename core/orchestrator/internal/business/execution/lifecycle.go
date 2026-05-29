package execution

// Lifecycle helpers that close the V22 money loop. The Settler is
// called once an execution reaches a terminal status (succeeded,
// failed, stopped, killed). It actualises the spend against the
// tenant wallet, releases the unused hold, writes the platform
// margin entry to the append-only ledger, and (when the execution
// was driven from a blueprint) records the run on the blueprint
// stats rollup.
//
// The TickReporter is the per-tick sandbox cost plumbing. v1 wires
// the constructor but no caller — Agent 12 hooks it into the runtime
// once the workspace driver emits real ticks. Putting the type here
// now avoids a wiring churn when the runtime side lands.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/blueprints"
	"ironflyer/core/orchestrator/internal/business/ledger"
	"ironflyer/core/orchestrator/internal/business/outboxhooks"
	"ironflyer/core/orchestrator/internal/business/wallet"
)

// Settler closes the money loop for one execution. Implementations
// must be safe to call from any goroutine that owns the terminal
// transition for the execution id and must be idempotent against
// double-close (the FSM has already moved to a terminal status, so
// the wallet/ledger side is the only thing that can drift).
type Settler interface {
	// Close performs the wallet + ledger settlement for executionID
	// given the finalStatus the caller already drove the FSM into.
	// Close does NOT touch executions.status — the caller is the only
	// component that owns the FSM move. Errors from the wallet, the
	// ledger, or the blueprint-stats writer are returned wrapped, but
	// Close attempts every leg before returning so a partial failure
	// still settles what it can.
	Close(ctx context.Context, executionID string, finalStatus Status) (Settlement, error)
}

// Settlement is the dollar-by-dollar breakdown of one Close call.
// Returned so the caller can log / emit it without re-reading the
// execution row.
type Settlement struct {
	// SpentUSD is the amount actualised against the wallet (Debit).
	SpentUSD decimal.Decimal
	// ReservedUSD is what was held against the wallet at admit time.
	ReservedUSD decimal.Decimal
	// ReleaseUSD is reserved - spent, clamped at zero. Released back
	// to the wallet's available balance.
	ReleaseUSD decimal.Decimal
	// MarginUSD is revenue - spent. Signed: negative values are
	// recorded honestly so the dashboards can flag loss-making runs.
	MarginUSD decimal.Decimal
}

// TickReporter is the per-tick sandbox cost plumbing. The runtime
// side reports the duration of a workspace slice plus the per-hour
// rate; the reporter converts that into a USD amount and books it
// against the execution row + the tenant ledger.
type TickReporter interface {
	// ReportSandboxTick attributes `durationSec` of sandbox time at
	// `ratePerHourUSD` to the execution. costUSD = rate * secs / 3600.
	// Safe to call zero-cost ticks (durationSec == 0): the call
	// short-circuits without writing anything.
	ReportSandboxTick(ctx context.Context, executionID string, durationSec int, ratePerHourUSD decimal.Decimal) error
}

// NewSettler builds the canonical Settler over the execution, wallet,
// ledger, and blueprint-stats services. Any of walletSvc / ledgerSvc /
// blueprintStats may be nil — the corresponding leg of the settlement
// becomes a no-op so test rigs and partial deployments still work.
// execSvc MUST be non-nil; Close has no work to do without it.
func NewSettler(execSvc Service, walletSvc wallet.Service, ledgerSvc ledger.Service, blueprintStats blueprints.StatsService) Settler {
	return &defaultSettler{
		exec:      execSvc,
		wallet:    walletSvc,
		ledger:    ledgerSvc,
		blueprint: blueprintStats,
	}
}

// NewSettlerWithOutbox is NewSettler plus an outbox pool. After the
// wallet + ledger legs commit, Close writes one durable
// execution.settled.v1 event to the outbox so dashboards and audit
// projections see the terminal transition. The settle event lives in
// its own short transaction (eventual-consistency with the wallet
// debit) because the wallet tx is opaque to the settler; the
// ledger billing.ledger.* events still ride the same tx that owns the
// ledger row when the ledger service is built with WithOutbox().
func NewSettlerWithOutbox(execSvc Service, walletSvc wallet.Service, ledgerSvc ledger.Service, blueprintStats blueprints.StatsService, pool *pgxpool.Pool) Settler {
	return &defaultSettler{
		exec:        execSvc,
		wallet:      walletSvc,
		ledger:      ledgerSvc,
		blueprint:   blueprintStats,
		outboxPool:  pool,
	}
}

// NewTickReporter builds the canonical TickReporter. execSvc MUST be
// non-nil; ledgerSvc may be nil (the call still posts cost on the
// execution row).
func NewTickReporter(execSvc Service, ledgerSvc ledger.Service) TickReporter {
	return &defaultTickReporter{exec: execSvc, ledger: ledgerSvc}
}

type defaultSettler struct {
	exec       Service
	wallet     wallet.Service
	ledger     ledger.Service
	blueprint  blueprints.StatsService
	outboxPool *pgxpool.Pool
}

var secondsPerHour = decimal.NewFromInt(3600)

func (s *defaultSettler) Close(ctx context.Context, executionID string, finalStatus Status) (Settlement, error) {
	if s == nil || s.exec == nil || executionID == "" {
		return Settlement{}, nil
	}
	exec, err := s.exec.Get(ctx, executionID)
	if err != nil {
		return Settlement{}, err
	}

	release := exec.ReservedUSD.Sub(exec.SpentUSD)
	if release.IsNegative() {
		release = decimal.Zero
	}
	margin := exec.RevenueUSD.Sub(exec.SpentUSD)

	settlement := Settlement{
		SpentUSD:    exec.SpentUSD,
		ReservedUSD: exec.ReservedUSD,
		ReleaseUSD:  release,
		MarginUSD:   margin,
	}

	// errs accumulates per-leg failures. Every leg is still attempted so
	// a partial failure settles what it can; the joined error is returned
	// to the caller (and the unbalanced money loop is never swallowed).
	var errs []error

	// Wallet leg — actualise the spend, free the unused hold.
	if s.wallet != nil && exec.TenantID != "" {
		if exec.SpentUSD.IsPositive() {
			if err := s.wallet.Debit(ctx, exec.TenantID, exec.SpentUSD); err != nil {
				errs = append(errs, fmt.Errorf("wallet debit %s spent=%s: %w", executionID, exec.SpentUSD, err))
			}
		}
		if release.IsPositive() {
			if err := s.wallet.Release(ctx, exec.TenantID, release); err != nil {
				errs = append(errs, fmt.Errorf("wallet release %s amount=%s: %w", executionID, release, err))
			}
		}
	}

	// Ledger leg — credit_release (if any) + platform_margin (signed).
	if s.ledger != nil {
		execUUID, perr := uuid.Parse(executionID)
		if perr == nil {
			tenantUUID := tenantToUUID(exec.TenantID)
			if release.IsPositive() {
				if _, err := s.ledger.Write(ctx, ledger.Entry{
					TenantID:    tenantUUID,
					ExecutionID: &execUUID,
					EntryType:   ledger.EntryCreditRelease,
					Direction:   ledger.CreditDirection,
					AmountUSD:   release,
					Metadata: map[string]any{
						"final_status": string(finalStatus),
					},
				}); err != nil {
					errs = append(errs, fmt.Errorf("ledger credit_release %s: %w", executionID, err))
				}
			}
			if !margin.IsZero() {
				dir := ledger.CreditDirection
				if margin.IsNegative() {
					dir = ledger.DebitDirection
				}
				signed, _ := margin.Float64()
				if _, err := s.ledger.Write(ctx, ledger.Entry{
					TenantID:       tenantUUID,
					ExecutionID:    &execUUID,
					EntryType:      ledger.EntryPlatformMargin,
					Direction:      dir,
					AmountUSD:      margin.Abs(),
					MarginRelevant: true,
					Metadata: map[string]any{
						"signed_usd":   signed,
						"final_status": string(finalStatus),
					},
				}); err != nil {
					errs = append(errs, fmt.Errorf("ledger platform_margin %s: %w", executionID, err))
				}
			}
		}
	}

	// Blueprint-stats leg — only when the execution actually ran a
	// blueprint. The dashboards treat missing rows as "no blueprint
	// signal", which is the right thing for ad-hoc executions.
	if s.blueprint != nil && exec.BlueprintID != "" {
		previewSuccess := finalStatus == StatusSucceeded
		if err := s.blueprint.RecordRun(ctx, blueprintRunFromExecution(exec, previewSuccess)); err != nil {
			errs = append(errs, fmt.Errorf("blueprint stats %s: %w", executionID, err))
		}
	}

	// Outbox leg — record the terminal transition for downstream
	// dashboards and audit projections. This is intentionally a
	// follow-up tx rather than coupled to the wallet leg because the
	// wallet service owns its own transaction shape; the eventual-
	// consistency fallback documented in ARCHITECTURE_EVENTS.md.
	s.emitSettledEvent(ctx, executionID, exec.TenantID, finalStatus, settlement)

	return settlement, errors.Join(errs...)
}

// emitSettledEvent writes one execution.settled.v1 outbox row in a
// short transaction owned by the settler. Best-effort: a publisher
// failure is logged via the outbox.last_error column and replayed by
// the publisher daemon, not surfaced to the FSM caller.
func (s *defaultSettler) emitSettledEvent(ctx context.Context, executionID, tenantID string, finalStatus Status, st Settlement) {
	if s == nil || s.outboxPool == nil {
		return
	}
	tx, err := s.outboxPool.Begin(ctx)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	spent, _ := st.SpentUSD.Float64()
	reserved, _ := st.ReservedUSD.Float64()
	release, _ := st.ReleaseUSD.Float64()
	margin, _ := st.MarginUSD.Float64()
	evt := outboxhooks.ExecutionLifecycleEvent(executionID, tenantID, "execution.settled.v1", map[string]any{
		"final_status": string(finalStatus),
		"spent_usd":    spent,
		"reserved_usd": reserved,
		"release_usd":  release,
		"margin_usd":   margin,
	})
	if err := outboxhooks.WriteEventInTx(ctx, tx, evt); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}

// blueprintRunFromExecution mints the RunOutcome shape the blueprint
// stats writer needs. Repaired is left false in v1 — the recovery
// loop has not surfaced a per-execution repair counter; Agent 7 owns
// that wiring once the repair genome lands its hooks.
func blueprintRunFromExecution(exec Execution, previewSuccess bool) blueprints.RunOutcome {
	execUUID, _ := uuid.Parse(exec.ID)
	timeToPreview := 0
	if exec.StartedAt != nil && exec.EndedAt != nil {
		secs := int(exec.EndedAt.Sub(*exec.StartedAt).Seconds())
		if secs > 0 {
			timeToPreview = secs
		}
	}
	return blueprints.RunOutcome{
		BlueprintID:          exec.BlueprintID,
		ExecutionID:          execUUID,
		TenantID:             tenantToUUID(exec.TenantID),
		RevenueUSD:           exec.RevenueUSD,
		CostUSD:              exec.SpentUSD,
		CompletionScore:      exec.CompletionScore,
		PreviewSuccess:       previewSuccess,
		Repaired:             false,
		Refunded:             exec.RefundedUSD.IsPositive(),
		TimeToPreviewSeconds: timeToPreview,
	}
}

type defaultTickReporter struct {
	exec   Service
	ledger ledger.Service
}

func (t *defaultTickReporter) ReportSandboxTick(ctx context.Context, executionID string, durationSec int, ratePerHourUSD decimal.Decimal) error {
	if t == nil || t.exec == nil || executionID == "" || durationSec <= 0 {
		return nil
	}
	dur := decimal.NewFromInt(int64(durationSec))
	costUSD := ratePerHourUSD.Mul(dur).Div(secondsPerHour)
	if !costUSD.IsPositive() {
		return nil
	}
	if err := t.exec.AddCost(ctx, executionID, CostSandbox, costUSD, "runtime"); err != nil {
		return err
	}
	if t.ledger != nil {
		execUUID, perr := uuid.Parse(executionID)
		if perr == nil {
			exec, err := t.exec.Get(ctx, executionID)
			if err == nil {
				_, _ = t.ledger.Write(ctx, ledger.Entry{
					TenantID:       tenantToUUID(exec.TenantID),
					ExecutionID:    &execUUID,
					EntryType:      ledger.EntrySandboxCost,
					Direction:      ledger.DebitDirection,
					AmountUSD:      costUSD,
					Provider:       "runtime",
					Billable:       true,
					MarginRelevant: true,
					Metadata: map[string]any{
						"duration_sec":       durationSec,
						"rate_per_hour_usd":  ratePerHourUSD.String(),
					},
				})
			}
		}
	}
	return nil
}

// tenantToUUID returns a UUID for tenant. If tenant is already a UUID
// we parse it; otherwise we deterministically hash it via UUIDv5 so
// the ledger key is stable across calls. Mirrors tenantUUIDFor in the
// graph resolver — keeping the helper local to this package so the
// execution lifecycle has no reverse dependency on graph/resolver.
func tenantToUUID(tenant string) uuid.UUID {
	if tenant == "" {
		return uuid.Nil
	}
	if id, err := uuid.Parse(tenant); err == nil {
		return id
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenant))
}

// jsonRaw is reserved for future event payloads — keeping the
// encoding/json import live in case downstream wiring needs a small
// helper here.
var _ = json.RawMessage(nil)
