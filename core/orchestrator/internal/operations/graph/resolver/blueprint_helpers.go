package resolver

import (
	"ironflyer/core/orchestrator/internal/business/blueprints"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// blueprintToGraphQL projects a blueprints.Blueprint into the
// model.Blueprint shape returned by the GraphQL boundary.
func blueprintToGraphQL(b blueprints.Blueprint) *model.Blueprint {
	return &model.Blueprint{
		ID:                       b.ID,
		Name:                     b.Name,
		Description:              b.Description,
		Category:                 b.Category,
		CostPriorUsd:             floatOfDecimal(b.CostPriorUSD),
		ExpectedTimeToPreviewSec: b.ExpectedTimeToPreviewSec,
		SupportedGates:           append([]string(nil), b.SupportedGates...),
		FileCount:                len(b.Files),
	}
}

// blueprintStatsToGraphQL projects per-blueprint runtime stats onto
// the matching GraphQL model.
func blueprintStatsToGraphQL(s blueprints.Stats) *model.BlueprintStats {
	return &model.BlueprintStats{
		BlueprintID:         s.BlueprintID,
		Executions:          int(s.Executions),
		PreviewSuccess:      int(s.PreviewSuccess),
		Refunds:             int(s.Refunds),
		RepairCount:         int(s.RepairCount),
		AvgRevenueUsd:       floatOfDecimal(s.AvgRevenueUSD),
		AvgCostUsd:          floatOfDecimal(s.AvgCostUSD),
		GrossMarginPct:      floatOfDecimal(s.GrossMarginPct),
		AvgCompletionScore:  floatOfDecimal(s.AvgCompletionScore),
		AvgTimeToPreviewSec: floatOfDecimal(s.AvgTimeToPreviewSec),
	}
}
