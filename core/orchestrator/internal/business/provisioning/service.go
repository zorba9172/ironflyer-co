package provisioning

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Service is the persistence-agnostic contract every storage backend
// must satisfy. Resolvers and connectors hold the interface, never the
// concrete struct, so the in-memory and Postgres backends stay drop-in
// interchangeable.
//
// Owner isolation: every read/write that names a tenant MUST be
// scoped — passing a wrong tenant against an existing ResourceID
// returns ErrForbidden (rendered as ErrResourceNotFound by the
// resolver so the API never leaks existence to the wrong tenant).
type Service interface {
	// Provision records a brand-new ProvisionedResource. The connector
	// has already made the rail-side API call; this is the persistence
	// half — generates ID + timestamps and stores the row in status
	// pending. Idempotent against (tenant, externalID): a repeat call
	// returns the existing row instead of inserting a second one.
	Provision(ctx context.Context, r ProvisionedResource) (ProvisionedResource, error)

	// Get returns one ProvisionedResource. tenant scopes the lookup —
	// rows owned by a different tenant are returned as
	// ErrResourceNotFound.
	Get(ctx context.Context, tenant, id string) (ProvisionedResource, error)

	// List returns every ProvisionedResource for the (tenant, project)
	// pair, newest first. Empty list is normal — used by the
	// `provisionedResources` GraphQL query on the project dashboard.
	List(ctx context.Context, tenant, project string) ([]ProvisionedResource, error)

	// UpdateStatus flips a row's status. Used by Suspend / Reactivate /
	// Close flows. Returns ErrForbidden when tenant does not own id.
	UpdateStatus(ctx context.Context, tenant, id, status string) (ProvisionedResource, error)

	// RecordRevenue inserts one RevenueEvent. Idempotent against
	// (resourceID, externalRef): a duplicate webhook delivery returns
	// ErrDuplicateEvent so the connector can short-circuit cleanly.
	// LedgerEntryID is optional at this layer — wireup writes the
	// platform ledger row separately and links via UpdateLedgerLink.
	RecordRevenue(ctx context.Context, e RevenueEvent) (RevenueEvent, error)

	// ListRevenue returns recent RevenueEvent rows for a resource,
	// newest first. limit <= 0 falls back to 50; max 500 to keep the
	// resolver predictable under a noisy month-end aggregate.
	ListRevenue(ctx context.Context, tenant, resourceID string, limit int) ([]RevenueEvent, error)

	// SumRevenue returns the lifetime Ironflyer-cut total for the
	// resource. Used by the cockpit "lifetime forever revenue" tile —
	// the headline number that says how much margin the rail has
	// produced since it was provisioned.
	SumRevenue(ctx context.Context, tenant, resourceID string) (CutTotals, error)
}

// CutTotals is the aggregate projection for the cockpit headline.
// Split out so SumRevenue can return both numbers without paying for
// a List + reduce on every page render.
type CutTotals struct {
	GrossUSD     decimal.Decimal
	CutUSD       decimal.Decimal
	EventCount   int
	FirstEventAt *time.Time
	LastEventAt  *time.Time
}
