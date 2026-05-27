package resolver

// Helpers for the mcp resolver. Lives in a separate file so the next
// gqlgen generate pass doesn't bury them in the auto-generated
// "harms way" comment block at the bottom of mcp.resolver.go.

import (
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/suppliers/mcp_catalog"
)

// mcpSpecToGraphQL maps the supplier catalog struct onto the GraphQL
// model. Note we surface only metadata fields here — Command/Args
// stay server-side so a tenant can never inject their own exec
// invocation through the GraphQL boundary.
func mcpSpecToGraphQL(s mcp_catalog.ServerSpec) model.MCPServerSpec {
	out := model.MCPServerSpec{
		ID:             s.ID,
		Name:           s.Name,
		Description:    s.Description,
		Vendor:         s.Vendor,
		EnvKeys:        s.EnvKeys,
		Category:       s.Category,
		RequiresSecret: s.RequiresSecret,
		Capabilities:   s.Capabilities,
	}
	if s.EnvKeys == nil {
		out.EnvKeys = []string{}
	}
	if s.Capabilities == nil {
		out.Capabilities = []string{}
	}
	if s.IconURL != "" {
		v := s.IconURL
		out.IconURL = &v
	}
	return out
}

// mcpRunningToGraphQL projects the supplier RunningServer onto the
// GraphQL model. The composite id (userID|projectID|serverID) is the
// stable React key the cockpit uses; serverId remains the catalog
// slug so the UI can join back to the spec for the chip label.
func mcpRunningToGraphQL(rs mcp_catalog.RunningServer, projectID, userID string) model.MCPRunningServer {
	return model.MCPRunningServer{
		ID:        userID + "|" + projectID + "|" + rs.ServerID,
		ServerID:  rs.ServerID,
		StartedAt: rs.StartedAt,
	}
}
