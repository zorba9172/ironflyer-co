package dashboards

import (
	"context"
	"time"
)

// CohortDashboard is the per-month cohort retention view per V22 proof
// pack (03-proof-dashboards/03-cohort-dashboard.md).
type CohortDashboard struct {
	Cohorts []Cohort
}

// BuildCohort delegates to the ExecutionSource which knows how to roll
// up users + executions + ledger by calendar month. The dashboard
// surface is just a thin envelope so the GraphQL boundary stays
// trivial.
func BuildCohort(ctx context.Context, src ExecutionSource, sinceMonth time.Time) (CohortDashboard, error) {
	rows, err := src.CountsByCohort(ctx, sinceMonth)
	if err != nil {
		return CohortDashboard{}, err
	}
	if rows == nil {
		rows = []Cohort{}
	}
	return CohortDashboard{Cohorts: rows}, nil
}
