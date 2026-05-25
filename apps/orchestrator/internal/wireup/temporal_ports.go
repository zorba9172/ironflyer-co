package wireup

import (
	"context"

	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/ledger"
	"ironflyer/apps/orchestrator/internal/temporalworker"
	"ironflyer/apps/orchestrator/internal/wallet"
)

// TemporalWalletAdapter satisfies temporalworker.WalletPort by
// delegating to wallet.Service. The opKey is forwarded to the
// idempotent variants when available; non-idempotent paths fall back
// to plain mutators (the V22 wallet packages always implement the
// idempotent surface, so the fallback only matters for in-test
// doubles).
type TemporalWalletAdapter struct {
	Svc wallet.Service
}

// Hold places the activity's hold against the tenant balance.
func (a TemporalWalletAdapter) Hold(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if a.Svc == nil {
		return nil
	}
	if idem, ok := a.Svc.(wallet.IdempotentService); ok {
		return idem.HoldWithKey(ctx, tenant, amount, opKey)
	}
	return a.Svc.Hold(ctx, tenant, amount)
}

// Release frees a prior hold idempotently.
func (a TemporalWalletAdapter) Release(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if a.Svc == nil {
		return nil
	}
	if idem, ok := a.Svc.(wallet.IdempotentService); ok {
		return idem.ReleaseWithKey(ctx, tenant, amount, opKey)
	}
	return a.Svc.Release(ctx, tenant, amount)
}

// Debit consumes a prior hold idempotently.
func (a TemporalWalletAdapter) Debit(ctx context.Context, tenant string, amount decimal.Decimal, opKey string) error {
	if a.Svc == nil {
		return nil
	}
	if idem, ok := a.Svc.(wallet.IdempotentService); ok {
		return idem.DebitWithKey(ctx, tenant, amount, opKey)
	}
	return a.Svc.Debit(ctx, tenant, amount)
}

// TemporalLedgerAdapter satisfies temporalworker.LedgerPort by writing
// ledger rows that carry an opKey for cross-attempt idempotency. The
// V22 ledger does not currently expose a WriteWithKey; the opKey is
// folded into the entry metadata so duplicate rows are auditable.
type TemporalLedgerAdapter struct {
	Svc ledger.Service
}

// Write appends an entry of the supplied type. tenant and executionID
// are mapped onto the canonical UUID identifiers via tenantUUIDFor.
func (a TemporalLedgerAdapter) Write(ctx context.Context, entryType string, tenant, executionID string, amount decimal.Decimal, opKey string) error {
	if a.Svc == nil {
		return nil
	}
	tenantUUID := temporalTenantUUID(tenant)
	entry := ledger.Entry{
		TenantID:  tenantUUID,
		EntryType: ledger.EntryType(entryType),
		Direction: ledger.DebitDirection,
		AmountUSD: amount,
		Billable:  true,
		Metadata: map[string]any{
			"op_key":           opKey,
			"temporal_source":  true,
			"raw_execution_id": executionID,
		},
	}
	if execUUID, ok := temporalParseExecUUID(executionID); ok {
		entry.ExecutionID = &execUUID
	}
	_, err := a.Svc.Write(ctx, entry)
	return err
}

// Compile-time assertions: keep the temporalworker port surfaces tied
// to these adapters so a future port-shape change forces an update
// here.
var _ temporalworker.WalletPort = TemporalWalletAdapter{}
var _ temporalworker.LedgerPort = TemporalLedgerAdapter{}
