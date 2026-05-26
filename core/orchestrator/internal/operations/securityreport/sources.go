package securityreport

import (
	"context"
	"time"
)

// ExecutionMeta is the minimal slice of execution state the report
// builder needs. We deliberately keep it small so any execution
// store (execution.Service, store.Store, a Temporal query, …) can
// satisfy it without dragging its full domain object across the
// package boundary.
type ExecutionMeta struct {
	ID         string
	TenantID   string
	Status     string // "running" | "completed" | "failed" | "cancelled"
	GateStatus string // "pass" | "fail" | "warning" | "blocked"
}

// ExecutionSource resolves an execution ID to its tenant + status.
// nil-safe at the Builder: a nil source falls back to defaults so
// dev boxes without execution wiring still return a usable report.
type ExecutionSource interface {
	GetExecution(ctx context.Context, executionID string) (ExecutionMeta, error)
}

// FindingSource emits the raw findings produced by the finisher
// Security gate (or any other security-flavoured gate) for a given
// execution. The Builder takes them as-is — normalisation is the
// adapter's job so the gate's domain.Issue vocabulary stays decoupled
// from the customer-facing Finding shape.
type FindingSource interface {
	ByExecution(ctx context.Context, executionID string) ([]Finding, error)
}

// TenantPolicy describes the deploy-blocking thresholds an enterprise
// tenant configured. A nil policy means "block on any critical, allow
// anything else", which is the V22 baseline customers can opt into a
// stricter posture from.
type TenantPolicy struct {
	BlockOnFail     bool          // "fail" status blocks deploy regardless of severity bag
	BlockOnHigh     bool          // any high severity blocks deploy
	MaxFindingAge   time.Duration // findings older than this are excluded; zero = unlimited
}

// PolicySource resolves a tenant to its policy. nil-safe — Builder
// uses DefaultPolicy() when source or lookup returns nothing.
type PolicySource interface {
	ForTenant(ctx context.Context, tenantID string) (TenantPolicy, error)
}

// DefaultPolicy is the V22 baseline: block deploy only on critical
// findings; do not enforce age windows.
func DefaultPolicy() TenantPolicy {
	return TenantPolicy{BlockOnFail: false, BlockOnHigh: false, MaxFindingAge: 0}
}
