package resolver

// MCP catalog resolver. Bridges the GraphQL surface defined in
// schema/mcp.graphql to the mcp_catalog Manager. Every mutation goes
// through the project-owner check so a logged-in user can never
// enable an MCP server against a project they don't own.

import (
	"context"
	"errors"
	"fmt"

	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/suppliers/mcp_catalog"
)

// McpCatalog is the resolver for the mcpCatalog field. Returns the
// static catalog. Public — no auth required so the marketing surface
// can list the available integrations without the user having
// signed in yet.
func (r *queryResolver) McpCatalog(ctx context.Context) ([]model.MCPServerSpec, error) {
	specs := mcp_catalog.DefaultCatalog()
	out := make([]model.MCPServerSpec, 0, len(specs))
	for _, s := range specs {
		out = append(out, mcpSpecToGraphQL(s))
	}
	return out, nil
}

// McpEnabled is the resolver for the mcpEnabled field. Returns the
// per-(user, project) set of running servers. Returns an empty list
// when the manager is unwired so the cockpit renders the
// "nothing enabled" empty state cleanly.
func (r *queryResolver) McpEnabled(ctx context.Context, projectID string) ([]model.MCPRunningServer, error) {
	if r.MCPManager == nil {
		return []model.MCPRunningServer{}, nil
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	project, err := r.Resolver.Projects.Get(projectID)
	if err != nil {
		return nil, fmt.Errorf("mcp: load project: %w", err)
	}
	if !project.IsAccessibleBy(user.ID) {
		return nil, errors.New("mcp: project not found")
	}
	running := r.MCPManager.ListEnabled(user.ID, projectID)
	out := make([]model.MCPRunningServer, 0, len(running))
	for _, rs := range running {
		out = append(out, mcpRunningToGraphQL(rs, projectID, user.ID))
	}
	return out, nil
}

// McpEnable is the resolver for the mcpEnable field. Spawns the
// named MCP server against the project; the project's Secrets are
// consulted to populate the child's env.
func (r *mutationResolver) McpEnable(ctx context.Context, projectID string, serverID string) (*model.MCPRunningServer, error) {
	if r.MCPManager == nil {
		return nil, gqlNotConfigured("mcp")
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	project, err := r.Resolver.Projects.Get(projectID)
	if err != nil {
		return nil, fmt.Errorf("mcp: load project: %w", err)
	}
	if !project.IsAccessibleBy(user.ID) {
		return nil, errors.New("mcp: project not found")
	}
	// Defensive copy — Project.Secrets is not serialised but we still
	// avoid handing the manager a live map reference into the store.
	secrets := make(map[string]string, len(project.Secrets))
	for k, v := range project.Secrets {
		secrets[k] = v
	}
	rs, err := r.MCPManager.Enable(ctx, user.ID, projectID, serverID, secrets)
	if err != nil {
		return nil, err
	}
	out := mcpRunningToGraphQL(*rs, projectID, user.ID)
	return &out, nil
}

// McpDisable is the resolver for the mcpDisable field. Tears the
// spawned server down. Idempotent.
func (r *mutationResolver) McpDisable(ctx context.Context, projectID string, serverID string) (bool, error) {
	if r.MCPManager == nil {
		return false, gqlNotConfigured("mcp")
	}
	user, err := currentUser(ctx)
	if err != nil {
		return false, err
	}
	project, err := r.Resolver.Projects.Get(projectID)
	if err != nil {
		return false, fmt.Errorf("mcp: load project: %w", err)
	}
	if !project.IsAccessibleBy(user.ID) {
		return false, errors.New("mcp: project not found")
	}
	if err := r.MCPManager.Disable(ctx, user.ID, projectID, serverID); err != nil {
		return false, err
	}
	return true, nil
}
