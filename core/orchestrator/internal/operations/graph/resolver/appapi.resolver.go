package resolver

import (
	"context"

	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// AppAPIKeys lists the deployed app's issued API keys (prefix only).
func (r *queryResolver) AppAPIKeys(ctx context.Context, projectID string) ([]model.AppAPIKey, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	keys := r.AppConsole.APIKeys(projectID)
	out := make([]model.AppAPIKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, *appAPIKeyToModel(k))
	}
	return out, nil
}

// AppEndpoints returns the generated endpoint catalogue.
func (r *queryResolver) AppEndpoints(ctx context.Context, projectID string) ([]model.AppEndpoint, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	eps := r.AppConsole.Endpoints(projectID)
	out := make([]model.AppEndpoint, 0, len(eps))
	for _, e := range eps {
		out = append(out, appEndpointToModel(e))
	}
	return out, nil
}

// AppWebhooks lists the deployed app's outbound webhooks.
func (r *queryResolver) AppWebhooks(ctx context.Context, projectID string) ([]model.AppWebhook, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	hooks := r.AppConsole.Webhooks(projectID)
	out := make([]model.AppWebhook, 0, len(hooks))
	for _, w := range hooks {
		out = append(out, *appWebhookToModel(w))
	}
	return out, nil
}

// CreateAppAPIKey issues a key and returns the one-time plaintext secret.
func (r *mutationResolver) CreateAppAPIKey(ctx context.Context, input model.CreateAppAPIKeyInput) (*model.AppAPIKeyWithSecret, error) {
	if _, err := r.requireOperateProject(ctx, input.ProjectID); err != nil {
		return nil, err
	}
	k, secret := r.AppConsole.CreateAPIKey(input.ProjectID, input.Name, input.Scopes)
	return &model.AppAPIKeyWithSecret{Key: *appAPIKeyToModel(k), Secret: secret}, nil
}

// RevokeAppAPIKey permanently revokes a key.
func (r *mutationResolver) RevokeAppAPIKey(ctx context.Context, id string) (*model.AppAPIKey, error) {
	if r.AppConsole == nil {
		return nil, gqlNotConfigured("operate")
	}
	pid, err := r.AppConsole.APIKeyProject(id)
	if err != nil {
		return nil, err
	}
	if _, err := r.requireOperateProject(ctx, pid); err != nil {
		return nil, err
	}
	k, err := r.AppConsole.RevokeAPIKey(id)
	if err != nil {
		return nil, err
	}
	return appAPIKeyToModel(k), nil
}

// CreateAppWebhook registers an outbound webhook.
func (r *mutationResolver) CreateAppWebhook(ctx context.Context, input model.CreateAppWebhookInput) (*model.AppWebhook, error) {
	if _, err := r.requireOperateProject(ctx, input.ProjectID); err != nil {
		return nil, err
	}
	w := r.AppConsole.CreateWebhook(input.ProjectID, input.URL, input.Events)
	return appWebhookToModel(w), nil
}

// SetAppWebhookEnabled toggles a webhook.
func (r *mutationResolver) SetAppWebhookEnabled(ctx context.Context, id string, enabled bool) (*model.AppWebhook, error) {
	if r.AppConsole == nil {
		return nil, gqlNotConfigured("operate")
	}
	pid, err := r.AppConsole.WebhookProject(id)
	if err != nil {
		return nil, err
	}
	if _, err := r.requireOperateProject(ctx, pid); err != nil {
		return nil, err
	}
	w, err := r.AppConsole.SetWebhookEnabled(id, enabled)
	if err != nil {
		return nil, err
	}
	return appWebhookToModel(w), nil
}

// DeleteAppWebhook removes a webhook.
func (r *mutationResolver) DeleteAppWebhook(ctx context.Context, id string) (*model.OperationResult, error) {
	if r.AppConsole == nil {
		return nil, gqlNotConfigured("operate")
	}
	pid, err := r.AppConsole.WebhookProject(id)
	if err != nil {
		return nil, err
	}
	if _, err := r.requireOperateProject(ctx, pid); err != nil {
		return nil, err
	}
	if err := r.AppConsole.DeleteWebhook(id); err != nil {
		return nil, err
	}
	return &model.OperationResult{Ok: true}, nil
}
