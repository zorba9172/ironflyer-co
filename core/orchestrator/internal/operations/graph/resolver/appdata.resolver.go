package resolver

import (
	"context"

	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// AppDataSchema returns the deployed app's database schema (tables + columns).
func (r *queryResolver) AppDataSchema(ctx context.Context, projectID string) ([]model.AppTable, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	tables := r.AppConsole.DataSchema(projectID)
	out := make([]model.AppTable, 0, len(tables))
	for _, t := range tables {
		out = append(out, appTableToModel(t))
	}
	return out, nil
}

// AppTableRows returns a sampled page of rows for one table.
func (r *queryResolver) AppTableRows(ctx context.Context, projectID string, table string, limit *int) (*model.AppTableRows, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	lim := 25
	if limit != nil {
		lim = *limit
	}
	tr, err := r.AppConsole.TableRows(projectID, table, lim)
	if err != nil {
		return nil, err
	}
	return appTableRowsToModel(tr), nil
}
