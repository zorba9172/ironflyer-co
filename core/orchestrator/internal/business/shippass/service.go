package shippass

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// Service is the persistence-agnostic contract for the Ship Pass
// lifecycle. The memory and postgres backends both implement it;
// resolvers, the Settler, and the reconciliation cron all talk only
// to this interface.
//
// All money is decimal USD. Implementations MUST refuse non-positive
// prices and clamp DeadlineAt in the future before persisting.
//
// Concurrency: implementations MUST treat (TenantID, ProjectID,
// Status=active) as a uniqueness constraint — a project may only
// have one active pass at a time. Buying a second pass while the
// first is in-flight is a user error and surfaced as
// ErrPassNotActive.
type Service interface {
	// Quote returns the resolver preview: what would the pass cost,
	// what gates does it require, how far does the wallet fall short
	// (zero when the buyer is fully funded). Walletshortfall is the
	// number rendered as "top up $X to unlock" on the CTA.
	Quote(ctx context.Context, tenant, projectID, tierKey string) (Quote, error)

	// Purchase records a new ShipPass row in `active` and reserves
	// the tier price against the wallet via HoldWithKey. Idempotent
	// against a caller-supplied request id (mapped to the underlying
	// wallet op key) so a Temporal retry of the purchase activity
	// does not stack holds. Returns ErrInsufficientWallet when the
	// wallet available balance is below the tier price; returns
	// ErrInvalidTier on unknown tier keys.
	Purchase(ctx context.Context, tenant, projectID, tierKey, requestID string) (ShipPass, error)

	// Cancel transitions an active pass to `cancelled` and releases
	// the held funds back to the wallet via ReleaseWithKey. Idempotent
	// against the pass id — repeated calls after the first cancel
	// return the cancelled row without touching the wallet again.
	// Returns ErrPassNotActive when the pass is already terminal.
	Cancel(ctx context.Context, tenant, passID string) (ShipPass, error)

	// Get returns the row owned by tenant. Returns ErrPassNotFound
	// when the row is missing OR owned by another tenant — those two
	// cases are deliberately collapsed.
	Get(ctx context.Context, tenant, passID string) (ShipPass, error)

	// ActiveForProject returns the in-flight pass for a project, or
	// (zero, ErrPassNotFound) when nothing is active. Used by the
	// project resolver to surface a "Ship Pass progress" panel on
	// every project page.
	ActiveForProject(ctx context.Context, tenant, projectID string) (ShipPass, error)

	// List returns the most recent passes for the tenant, newest
	// first. Used by the billing dashboard to render the lifetime
	// pass history.
	List(ctx context.Context, tenant string, limit int) ([]ShipPass, error)

	// RecordGateVerdict appends a GateProgress row and, when the
	// observation completes the tier's required gate set, drives the
	// pass through to `shipped` (DebitWithKey + UpdatedAt). The call
	// is idempotent: re-ingesting the same (passID, gate, observedAt)
	// returns the row without double-debiting. Passing a verdict for
	// a gate outside the tier scope is a no-op (recorded for audit,
	// but the pass status does not advance).
	RecordGateVerdict(ctx context.Context, passID string, gate domain.GateName, passed bool, reason string, observedAt time.Time) (ShipPass, error)

	// ProgressFor returns every GateProgress row associated with the
	// pass in observation order. The resolver renders this as the
	// pass timeline.
	ProgressFor(ctx context.Context, tenant, passID string) ([]GateProgress, error)

	// ExpireDue runs as a cron tick. Every active pass whose
	// DeadlineAt is in the past is transitioned to `refunded`, the
	// held funds released via ReleaseWithKey, and an OutcomeEvent
	// emitted. Returns the list of passes that flipped on this tick
	// so the cron caller can log/observe progress.
	ExpireDue(ctx context.Context, now time.Time) ([]ShipPass, error)

	// LifetimeStats returns the headline counters the billing
	// dashboard uses: total passes purchased, total shipped, total
	// refunded, total revenue (USD across `shipped` rows).
	LifetimeStats(ctx context.Context, tenant string) (LifetimeStats, error)
}

// LifetimeStats is the billing-dashboard projection. Pure counters —
// no per-row data — so the dashboard query stays cheap.
type LifetimeStats struct {
	TotalPurchased int
	TotalShipped   int
	TotalRefunded  int
	TotalCancelled int
	RevenueUSD     decimal.Decimal
}
