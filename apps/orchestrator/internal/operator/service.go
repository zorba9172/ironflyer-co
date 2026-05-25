package operator

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/deploy"
)

// ScaleSnapshot is the live capacity / queue projection the operator
// CLI and GraphQL operatorScaleSnapshot query render. Numbers are
// best-effort: ActiveExecutions + QueuedExecutions come from the
// execution service, SandboxCapacity from the runtime quota config,
// WorkerUtilizationPct is derived from active vs capacity.
type ScaleSnapshot struct {
	ActiveExecutions     int
	QueuedExecutions     int
	SandboxCapacity      int
	WorkerUtilizationPct float64
}

// WalletSnapshot mirrors the tenant-scoped numbers an operator needs
// when an account opens a support ticket about billing — balance,
// hold, lifetime top-up, lifetime spend. We re-flatten the
// wallet.Wallet rather than re-export it so operator-facing payloads
// don't accidentally pick up wallet internals (updated_at etc).
type WalletSnapshot struct {
	TenantID         string
	BalanceUSD       decimal.Decimal
	HoldUSD          decimal.Decimal
	LifetimeTopUpUSD decimal.Decimal
	LifetimeSpendUSD decimal.Decimal
}

// AuditEntry is the operator-facing projection of one audit row. The
// audit.Entry shape carries internal hashes and free-form attrs the
// operator does not need to see by default; this struct exposes the
// minimum needed to identify, sort, and chain-verify a row.
type AuditEntry struct {
	ID        string
	Timestamp time.Time
	Action    string
	Outcome   string
	Hash      string
}

// OperatorService is the read-only operator surface. Every method
// MUST be called only after RequireOperator(ctx) has passed; the
// implementations re-assert that for defense in depth, but callers
// should still gate at the resolver / CLI layer to avoid running the
// underlying service.Get calls speculatively.
type OperatorService interface {
	// PendingApprovals returns open deploy_approvals rows scoped to
	// tenantID. When tenantID is "" the implementation enumerates the
	// distinct tenants with pending approvals and flattens the result
	// — this is what the on-call operator wants when they don't yet
	// know which tenant is stuck.
	PendingApprovals(ctx context.Context, tenantID string) ([]deploy.Approval, error)

	// AbuseScore returns the (score, tier) for the (tenantID, userID)
	// pair. tier is the lowercase string from abuse.Tier so the
	// operator CLI prints stable labels.
	AbuseScore(ctx context.Context, tenantID, userID string) (score int, tier string, err error)

	// ScaleSnapshot returns the live capacity projection.
	ScaleSnapshot(ctx context.Context) (ScaleSnapshot, error)

	// WalletSnapshot returns the wallet projection for tenantID.
	WalletSnapshot(ctx context.Context, tenantID string) (WalletSnapshot, error)

	// AuditCursor returns the audit chain entries created on or after
	// `since`, capped at `limit` (default 100, max 1000). Entries are
	// newest-first so the operator can pipe the result into a less /
	// jq workflow.
	AuditCursor(ctx context.Context, since time.Time, limit int) ([]AuditEntry, error)
}
