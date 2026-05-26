package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Service is the append-only ledger contract. There is no Update and
// no Delete by design — refunds and credit releases are new entries
// with the inverse direction, never mutations of the original row.
// Callers that need to "correct" a posted entry must post a
// compensating entry.
type Service interface {
	// Write appends a single validated entry. Sets ID / CreatedAt if
	// the caller left them zero. Returns the stored entry.
	Write(ctx context.Context, e Entry) (Entry, error)

	// ListByTenant returns entries for a tenant, narrowed by Filter.
	// Order is created_at DESC; pagination is via Limit / Offset.
	ListByTenant(ctx context.Context, tenantID uuid.UUID, f Filter) ([]Entry, error)

	// ListByExecution returns the per-execution ledger trail in the
	// order entries were posted (created_at ASC) — operators reading
	// an execution timeline want top-down chronology, not newest
	// first.
	ListByExecution(ctx context.Context, executionID uuid.UUID) ([]Entry, error)

	// SumByType returns the per-EntryType sum of amount_usd for a
	// tenant over a window. Used by the dashboards to avoid pulling
	// every row across the wire just to sum it. A nil/empty types
	// slice means "all types".
	SumByType(ctx context.Context, tenantID uuid.UUID, types []EntryType, since, until time.Time) (map[EntryType]decimal.Decimal, error)

	// TenantRollup returns the dashboard-ready Rollup for a tenant
	// over a window. Implementations are free to compute this from
	// SumByType or from a more efficient single-query aggregation.
	TenantRollup(ctx context.Context, tenantID uuid.UUID, since, until time.Time) (Rollup, error)
}

// validate is the structural check applied to every Entry before it
// is persisted, by both the Memory and Postgres backends. Keeping the
// rules in one place prevents drift between backends.
func validate(e Entry) error {
	if e.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenantID is required", ErrInvalidEntry)
	}
	if !e.EntryType.IsValid() {
		return fmt.Errorf("%w: unknown entry_type %q", ErrInvalidEntry, e.EntryType)
	}
	if !e.Direction.IsValid() {
		return fmt.Errorf("%w: unknown direction %q", ErrInvalidEntry, e.Direction)
	}
	if e.AmountUSD.Sign() <= 0 {
		return ErrZeroAmount
	}
	return nil
}

// stamp fills in defaults that the storage layer would otherwise have
// to repeat: a fresh UUID if one wasn't supplied, CreatedAt = now,
// and a non-nil Metadata map (Postgres jsonb prefers '{}' over null).
func stamp(e Entry) Entry {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	return e
}
