package resolver

import (
	"context"

	"ironflyer/core/orchestrator/internal/operations/appconsole"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// Automations lists the deployed app's scheduled / triggered workflows.
func (r *queryResolver) Automations(ctx context.Context, projectID string) ([]model.Automation, error) {
	if _, err := r.requireOperateProject(ctx, projectID); err != nil {
		return nil, err
	}
	rows := r.AppConsole.Automations(projectID)
	out := make([]model.Automation, 0, len(rows))
	for _, a := range rows {
		out = append(out, *automationToModel(a))
	}
	return out, nil
}

// CreateAutomation registers a new automation for a project.
func (r *mutationResolver) CreateAutomation(ctx context.Context, input model.CreateAutomationInput) (*model.Automation, error) {
	if _, err := r.requireOperateProject(ctx, input.ProjectID); err != nil {
		return nil, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	a := r.AppConsole.CreateAutomation(appconsole.Automation{
		ProjectID: input.ProjectID, Name: input.Name, TriggerKind: input.TriggerKind,
		TriggerConfig: input.TriggerConfig, Action: input.Action, Enabled: enabled,
	})
	return automationToModel(a), nil
}

// SetAutomationEnabled toggles an automation on or off.
func (r *mutationResolver) SetAutomationEnabled(ctx context.Context, id string, enabled bool) (*model.Automation, error) {
	if err := r.requireOperateByAutomation(ctx, id); err != nil {
		return nil, err
	}
	a, err := r.AppConsole.SetAutomationEnabled(id, enabled)
	if err != nil {
		return nil, err
	}
	return automationToModel(a), nil
}

// RunAutomation triggers a one-off run and records the outcome.
func (r *mutationResolver) RunAutomation(ctx context.Context, id string) (*model.Automation, error) {
	if err := r.requireOperateByAutomation(ctx, id); err != nil {
		return nil, err
	}
	a, err := r.AppConsole.RunAutomation(id)
	if err != nil {
		return nil, err
	}
	return automationToModel(a), nil
}

// DeleteAutomation removes an automation.
func (r *mutationResolver) DeleteAutomation(ctx context.Context, id string) (*model.OperationResult, error) {
	if err := r.requireOperateByAutomation(ctx, id); err != nil {
		return nil, err
	}
	if err := r.AppConsole.DeleteAutomation(id); err != nil {
		return nil, err
	}
	return &model.OperationResult{Ok: true}, nil
}
