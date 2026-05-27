package guild

import (
	"context"

	"github.com/shopspring/decimal"
)

// Service is the persistence-agnostic contract for guild storage. Both
// the memory and Postgres backends implement it; resolvers, the gate-
// failure router, the escrow helper, and the reconciliation cron all
// talk to this interface so swapping backends is a one-line change.
//
// Every mutating method MUST be safe to call concurrently and MUST
// refuse negative amounts (returning ErrInvalidAmount). Owner-checks
// live ABOVE this interface (in the resolver / escrow layers) so the
// service stays a thin store — the resolver knows who the caller is,
// the service does not.
type Service interface {
	// --- finisher profiles -----------------------------------------
	UpsertFinisherProfile(ctx context.Context, p FinisherProfile) (FinisherProfile, error)
	GetFinisherProfile(ctx context.Context, id string) (FinisherProfile, error)
	GetFinisherProfileByUser(ctx context.Context, userID string) (FinisherProfile, error)

	// --- tasks -----------------------------------------------------
	CreateTask(ctx context.Context, t GuildTask) (GuildTask, error)
	GetTask(ctx context.Context, id string) (GuildTask, error)
	ListTasks(ctx context.Context, filter TaskFilter) ([]GuildTask, error)
	UpdateTaskStatus(ctx context.Context, taskID, status string, assignedTo *string) (GuildTask, error)

	// --- bids ------------------------------------------------------
	PlaceBid(ctx context.Context, b Bid) (Bid, error)
	ListBids(ctx context.Context, taskID string) ([]Bid, error)
	GetBid(ctx context.Context, id string) (Bid, error)
	UpdateBidStatus(ctx context.Context, bidID, status string) (Bid, error)
	CountBidsForTask(ctx context.Context, taskID string) (int, error)

	// --- templates -------------------------------------------------
	UpsertTemplate(ctx context.Context, t Template) (Template, error)
	GetTemplateBySlug(ctx context.Context, slug string) (Template, error)
	ListTemplates(ctx context.Context, verifiedOnly bool) ([]Template, error)
	IncrementTemplateInstallCount(ctx context.Context, templateID string) error

	// --- installs / payouts ----------------------------------------
	RecordInstall(ctx context.Context, i Install) (Install, error)
	RecordPayout(ctx context.Context, p Payout) (Payout, error)

	// --- idempotency ----------------------------------------------
	RecallOp(ctx context.Context, opKey string) (OpOutcome, bool, error)
	RecordOp(ctx context.Context, opKey, opType string, amount decimal.Decimal, status, errorCode string) error

	// --- reconciliation -------------------------------------------
	ListStaleOpenBids(ctx context.Context, olderThanSec int) ([]Bid, error)
	ListAbandonedTasks(ctx context.Context, olderThanSec int) ([]GuildTask, error)
}

// TaskFilter narrows ListTasks. Empty fields mean "no filter on this
// dimension"; Mine=true scopes to tasks where TenantID matches the
// caller's tenant (resolver fills it in before delegating).
type TaskFilter struct {
	Status   string
	TenantID string // empty = all tenants
	Limit    int
}

// OpOutcome mirrors the wallet package's per-op dedupe shape. RecallOp
// returns this when a prior call with the same opKey landed; the
// caller short-circuits on Status=="succeeded" and replays the error
// from ErrorCode on "failed".
type OpOutcome struct {
	Status    string
	ErrorCode string
}
