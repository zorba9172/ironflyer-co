package dashboards

import (
	"context"
	"errors"
	"time"
)

// Service is the facade the GraphQL resolvers call. It composes the
// four dashboard builders behind their respective sources.
//
// All four source fields are required for the corresponding method to
// succeed. Agent 8 wires concrete adapters; this package never
// constructs sources itself.
type Service struct {
	Ledger    LedgerSource
	Exec      ExecutionSource
	Blueprint BlueprintSource
	Scale     ScaleSource
}

// ErrTenantRequired is returned by the per-tenant dashboard methods
// (Profit, Cohort, BlueprintDashboard) when the caller did not pass
// an authenticated tenant id. Returning instead of silently running
// cross-tenant SQL closes the privacy hole where a fresh signup saw
// platform-wide aggregates (bug #16).
var ErrTenantRequired = errors.New("dashboards: tenant id required for per-tenant view")

// Profit returns the profit dashboard for [since, until) scoped to
// tenantID. tenantID MUST be the authenticated caller's tenant — the
// resolver derives it from auth.FromContext.
func (s *Service) Profit(ctx context.Context, tenantID string, since, until time.Time) (ProfitDashboard, error) {
	if tenantID == "" {
		return ProfitDashboard{}, ErrTenantRequired
	}
	ctx = WithTenant(ctx, tenantID)
	return BuildProfit(ctx, s.Ledger, s.Exec, since, until)
}

// ScaleDashboard returns the live scale dashboard. Operator-only —
// the resolver enforces operator role before calling. Scale data is
// platform-wide (queue depth, sandbox capacity) and has no per-tenant
// meaning.
func (s *Service) ScaleDashboard(ctx context.Context) (ScaleDashboard, error) {
	return BuildScale(ctx, s.Scale, s.Exec)
}

// Cohort returns the cohort dashboard from sinceMonth onward scoped
// to tenantID. Returns ErrTenantRequired when tenantID is empty.
func (s *Service) Cohort(ctx context.Context, tenantID string, sinceMonth time.Time) (CohortDashboard, error) {
	if tenantID == "" {
		return CohortDashboard{}, ErrTenantRequired
	}
	ctx = WithTenant(ctx, tenantID)
	return BuildCohort(ctx, s.Exec, sinceMonth)
}

// BlueprintDashboard returns the per-blueprint stats dashboard scoped
// to tenantID. Returns ErrTenantRequired when tenantID is empty.
func (s *Service) BlueprintDashboard(ctx context.Context, tenantID string) (BlueprintDashboard, error) {
	if tenantID == "" {
		return BlueprintDashboard{}, ErrTenantRequired
	}
	ctx = WithTenant(ctx, tenantID)
	return BuildBlueprint(ctx, s.Blueprint)
}
