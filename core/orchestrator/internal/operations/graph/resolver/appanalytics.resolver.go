package resolver

import (
	"context"

	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// AppAnalytics returns the deployed app's traffic + event analytics.
func (r *queryResolver) AppAnalytics(ctx context.Context, projectID string, days *int) (*model.AppAnalytics, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	d := 30
	if days != nil {
		d = *days
	}
	return appAnalyticsToModel(r.AppConsole.Analytics(projectID, d)), nil
}
