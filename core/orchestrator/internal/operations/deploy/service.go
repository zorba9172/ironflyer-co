package deploy

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Service is the persistence-agnostic deploy plane API. Both the
// memory and postgres backends implement it; the GraphQL resolver
// and any future Temporal worker activity talk only to this surface.
//
// The Service owns:
//   - FSM transitions across the deploy_status enum
//   - approval workflow integration (Promote refuses unless an
//     approved approval row exists)
//   - ProfitGuard enforcement at the BeforeVercelDeploy point for
//     production deploys
//   - durable deploy_events emission for the deployFeed subscription
//
// Adapter side effects are performed inline (no Temporal yet) — the
// integration agent layers a Temporal activity wrapper on top when
// the deploy workflow ships.
type Service interface {
	// Plan opens a fresh deploys row in `planned` state, calls
	// Adapter.Plan to capture the provider-side projection, and
	// returns the durable Deploy. Plan does NOT touch the provider
	// in any way that creates resources — it's safe to call as a
	// dry-run.
	Plan(ctx context.Context, in PlanInput) (Deploy, error)

	// BuildPreview transitions a planned deploy to
	// `preview_building`, asks the Adapter to start the build, and
	// flips to `preview_ready` on success (or `failed` on error).
	BuildPreview(ctx context.Context, deployID string) (Deploy, error)

	// RequestApproval opens a deploy_approvals row in `pending`
	// state and flips the parent deploy to `awaiting_approval`. by
	// may be a zero UserRef for AI-driven approval requests; the
	// row records NULL for requested_by_user_id in that case.
	RequestApproval(ctx context.Context, deployID string, by UserRef, expiresIn time.Duration) (Approval, error)

	// Decide flips a pending approval row to `approved` or
	// `rejected`. decision MUST be one of DecisionApprove /
	// DecisionReject (also accepts "approved"/"rejected" verbatim).
	// On rejection the parent deploy moves to `cancelled`.
	Decide(ctx context.Context, approvalID string, by UserRef, decision string, note string) (Approval, error)

	// Promote runs the BeforeVercelDeploy ProfitGuard check (for
	// production deploys), refuses unless an approved approval row
	// exists, then asks the Adapter to promote the preview. Updates
	// production_url + promoted_at on success.
	Promote(ctx context.Context, deployID string) (Deploy, error)

	// Rollback asks the Adapter to revert the provider-side
	// production deploy, then flips the row to `rolled_back`.
	Rollback(ctx context.Context, deployID, reason string) (Deploy, error)

	// Cancel terminates a non-promoted deploy. Refuses to cancel
	// `promoted` rows (use Rollback for those).
	Cancel(ctx context.Context, deployID, reason string) (Deploy, error)

	// Get returns the Deploy or ErrNotFound.
	Get(ctx context.Context, id string) (Deploy, error)

	// GetByExecution returns the most recent Deploy tagged with the
	// given executionID. The boolean is false (with nil error) when
	// no deploy exists for that execution — distinct from a hard
	// not-found, since "execution had no deploy" is a legitimate
	// state for a still-running or never-deployed execution.
	//
	// Used by the wow-loop builder's DeploySource adapter to
	// populate the bundle's preview/production URLs.
	GetByExecution(ctx context.Context, executionID string) (Deploy, bool, error)

	// List returns the most recent deploys for the tenant, newest
	// first. limit defaults to 50, capped at 500.
	List(ctx context.Context, tenant string, limit, offset int) ([]Deploy, error)

	// PendingApprovals returns the tenant's open approval rows.
	PendingApprovals(ctx context.Context, tenant string) ([]Approval, error)

	// TenantsWithPendingApprovals returns the distinct tenant ids that
	// currently have at least one pending approval row. Used by the
	// expiry sweeper (deploy/sweeper.go) so it can walk the work-set
	// without scanning the full approvals table on every tick.
	TenantsWithPendingApprovals(ctx context.Context) ([]string, error)

	// RecordCost bumps deploys.cost_usd by addedUSD and emits a
	// `cost_recorded` event. Used by BillingGuard / runtime cost
	// attribution paths.
	RecordCost(ctx context.Context, deployID string, addedUSD decimal.Decimal) error

	// SubscribeEvents returns a channel that delivers deploy_events
	// for one deploy. Closing the ctx unsubscribes. The memory
	// service backs this with an in-process fan-out; the postgres
	// service backs it with a polling tail (the integration agent
	// may swap that for Redpanda once the outbox publisher is up).
	SubscribeEvents(ctx context.Context, deployID string) (<-chan Event, error)
}
