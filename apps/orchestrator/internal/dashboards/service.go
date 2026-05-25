package dashboards

import (
	"context"
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

// Profit returns the profit dashboard for [since, until).
func (s *Service) Profit(ctx context.Context, since, until time.Time) (ProfitDashboard, error) {
	return BuildProfit(ctx, s.Ledger, s.Exec, since, until)
}

// ScaleDashboard returns the live scale dashboard.
func (s *Service) ScaleDashboard(ctx context.Context) (ScaleDashboard, error) {
	return BuildScale(ctx, s.Scale, s.Exec)
}

// Cohort returns the cohort dashboard from sinceMonth onward.
func (s *Service) Cohort(ctx context.Context, sinceMonth time.Time) (CohortDashboard, error) {
	return BuildCohort(ctx, s.Exec, sinceMonth)
}

// BlueprintDashboard returns the per-blueprint stats dashboard.
func (s *Service) BlueprintDashboard(ctx context.Context) (BlueprintDashboard, error) {
	return BuildBlueprint(ctx, s.Blueprint)
}
