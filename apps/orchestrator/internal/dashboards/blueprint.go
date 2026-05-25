package dashboards

import "context"

// BlueprintDashboard is the per-blueprint stats view per V22 proof
// pack (03-proof-dashboards/04-blueprint-profit-dashboard.md).
type BlueprintDashboard struct {
	Blueprints []BlueprintStats
}

// BuildBlueprint composes one BlueprintDashboard from the source.
func BuildBlueprint(ctx context.Context, src BlueprintSource) (BlueprintDashboard, error) {
	rows, err := src.AllStats(ctx)
	if err != nil {
		return BlueprintDashboard{}, err
	}
	if rows == nil {
		rows = []BlueprintStats{}
	}
	return BlueprintDashboard{Blueprints: rows}, nil
}
