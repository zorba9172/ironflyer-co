package resolver

import (
	"context"

	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// AppEndUsers returns a page of the deployed app's end-user roster.
func (r *queryResolver) AppEndUsers(ctx context.Context, projectID string, limit *int, offset *int) ([]model.AppEndUser, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	lim, off := 100, 0
	if limit != nil {
		lim = *limit
	}
	if offset != nil {
		off = *offset
	}
	users := r.AppConsole.EndUsers(projectID, lim, off)
	out := make([]model.AppEndUser, 0, len(users))
	for _, u := range users {
		out = append(out, appEndUserToModel(u))
	}
	return out, nil
}

// AppUserStats returns aggregate counts for the end-user roster.
func (r *queryResolver) AppUserStats(ctx context.Context, projectID string) (*model.AppUserStats, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	return appUserStatsToModel(r.AppConsole.UserStats(projectID)), nil
}

// SetAppUserRole overrides one end-user's role.
func (r *mutationResolver) SetAppUserRole(ctx context.Context, projectID string, userID string, role string) (*model.AppEndUser, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	u, err := r.AppConsole.SetUserRole(projectID, userID, role)
	if err != nil {
		return nil, err
	}
	m := appEndUserToModel(u)
	return &m, nil
}

// SetAppUserSuspended suspends or restores one end-user.
func (r *mutationResolver) SetAppUserSuspended(ctx context.Context, projectID string, userID string, suspended bool) (*model.AppEndUser, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	u, err := r.AppConsole.SetUserSuspended(projectID, userID, suspended)
	if err != nil {
		return nil, err
	}
	m := appEndUserToModel(u)
	return &m, nil
}
