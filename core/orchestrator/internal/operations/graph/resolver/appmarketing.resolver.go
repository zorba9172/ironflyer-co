package resolver

import (
	"context"

	"ironflyer/core/orchestrator/internal/operations/appconsole"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// AppSeoSettings returns the deployed app's SEO / Open Graph metadata.
func (r *queryResolver) AppSeoSettings(ctx context.Context, projectID string) (*model.AppSeoSettings, error) {
	p, err := r.requireOperateProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return appSeoSettingsToModel(r.AppConsole.SeoSettings(projectID, p.Name)), nil
}

// AppSeoAudit returns a live audit derived from the current SEO settings.
func (r *queryResolver) AppSeoAudit(ctx context.Context, projectID string) (*model.AppSeoAudit, error) {
	p, err := r.requireOperateProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return appSeoAuditToModel(r.AppConsole.SeoAudit(projectID, p.Name)), nil
}

// UpdateAppSeoSettings patches the SEO metadata.
func (r *mutationResolver) UpdateAppSeoSettings(ctx context.Context, projectID string, input model.UpdateAppSeoSettingsInput) (*model.AppSeoSettings, error) {
	p, err := r.requireOperateProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	patch := appconsole.SeoPatch{
		Title: input.Title, Description: input.Description, Keywords: input.Keywords,
		OgImageURL: input.OgImageURL, TwitterHandle: input.TwitterHandle,
		CanonicalURL: input.CanonicalURL, Robots: input.Robots, SitemapEnabled: input.SitemapEnabled,
	}
	return appSeoSettingsToModel(r.AppConsole.UpdateSeoSettings(projectID, p.Name, patch)), nil
}
