package resolver

import (
	"ironflyer/apps/orchestrator/internal/deploy"
	"ironflyer/apps/orchestrator/internal/graph/model"
)

// deployToGraphQL projects a deploy.Deploy row into the model.Deploy
// shape returned by the GraphQL boundary.
func deployToGraphQL(d deploy.Deploy) *model.Deploy {
	out := &model.Deploy{
		ID:          d.ID,
		TenantID:    d.TenantID,
		ProjectID:   d.ProjectID,
		Target:      string(d.Target),
		Environment: string(d.Environment),
		Status:      string(d.Status),
		CostUsd:     floatOfDecimal(d.CostUSD),
		CreatedAt:   d.CreatedAt,
	}
	if d.ExecutionID != "" {
		v := d.ExecutionID
		out.ExecutionID = &v
	}
	if d.BlueprintID != "" {
		v := d.BlueprintID
		out.BlueprintID = &v
	}
	if d.ProviderDeploymentID != "" {
		v := d.ProviderDeploymentID
		out.ProviderDeploymentID = &v
	}
	if d.PreviewURL != "" {
		v := d.PreviewURL
		out.PreviewURL = &v
	}
	if d.ProductionURL != "" {
		v := d.ProductionURL
		out.ProductionURL = &v
	}
	if d.DiffHash != "" {
		v := d.DiffHash
		out.DiffHash = &v
	}
	if d.ArtifactHash != "" {
		v := d.ArtifactHash
		out.ArtifactHash = &v
	}
	if d.PreviewReadyAt != nil {
		t := *d.PreviewReadyAt
		out.PreviewReadyAt = &t
	}
	if d.PromotedAt != nil {
		t := *d.PromotedAt
		out.PromotedAt = &t
	}
	if d.RolledBackAt != nil {
		t := *d.RolledBackAt
		out.RolledBackAt = &t
	}
	out.GateSummary = stringMapToJSON(d.GateSummary)
	return out
}

// approvalToGraphQL projects a deploy.Approval row into the
// model.DeployApproval shape returned by the GraphQL boundary.
func approvalToGraphQL(a deploy.Approval) *model.DeployApproval {
	out := &model.DeployApproval{
		ID:            a.ID,
		DeployID:      a.DeployID,
		TenantID:      a.TenantID,
		Status:        string(a.Status),
		DiffHash:      a.DiffHash,
		ArtifactHash:  a.ArtifactHash,
		CostImpactUsd: floatOfDecimal(a.CostImpactUSD),
		ExpiresAt:     a.ExpiresAt,
		RequestedAt:   a.RequestedAt,
	}
	if a.DecisionNote != "" {
		v := a.DecisionNote
		out.DecisionNote = &v
	}
	if a.DecidedAt != nil {
		t := *a.DecidedAt
		out.DecidedAt = &t
	}
	out.GateSummary = stringMapToJSON(a.GateSummary)
	return out
}

// stringMapToJSON converts the map[string]string gate summary into the
// JSON scalar shape the model package uses.
func stringMapToJSON(in map[string]string) model.JSON {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return model.JSON(out)
}
