package resolver

import (
	"ironflyer/core/orchestrator/internal/operations/deploy"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
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

func deployDomainToGraphQL(d deploy.Domain) *model.DeployDomain {
	out := &model.DeployDomain{
		ID:                 d.ID,
		TenantID:           d.TenantID,
		ProjectID:          d.ProjectID,
		Hostname:           d.Hostname,
		Kind:               string(d.Kind),
		Status:             string(d.Status),
		Provider:           d.Provider,
		Primary:            d.Primary,
		DNSRecords:         dnsRecordsToGraphQL(d.DNSRecords),
		VerificationStatus: d.VerificationStatus,
		CertificateStatus:  string(d.CertificateStatus),
		Instructions:       d.Instructions,
		Metadata:           model.JSON(d.Metadata),
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
	if d.DeployID != "" {
		v := d.DeployID
		out.DeployID = &v
	}
	if d.Registrar != "" {
		v := d.Registrar
		out.Registrar = &v
	}
	if d.VerifiedAt != nil {
		t := *d.VerifiedAt
		out.VerifiedAt = &t
	}
	if d.LiveAt != nil {
		t := *d.LiveAt
		out.LiveAt = &t
	}
	if out.Metadata == nil {
		out.Metadata = model.JSON{}
	}
	return out
}

func dnsRecordsToGraphQL(records []deploy.DNSRecord) []model.DNSRecord {
	out := make([]model.DNSRecord, 0, len(records))
	for _, r := range records {
		got := model.DNSRecord{
			Type:  r.Type,
			Name:  r.Name,
			Value: r.Value,
		}
		if r.TTL > 0 {
			ttl := r.TTL
			got.TTL = &ttl
		}
		out = append(out, got)
	}
	return out
}

func domainAvailabilityToGraphQL(a deploy.DomainAvailability) *model.DomainAvailability {
	out := &model.DomainAvailability{
		Domain:       a.Domain,
		Available:    a.Available,
		Registrar:    a.Registrar,
		PriceUsd:     floatOfDecimal(a.PriceUSD),
		Currency:     a.Currency,
		Premium:      a.Premium,
		CanPurchase:  a.CanPurchase,
		CheckedAt:    a.CheckedAt,
		Requirements: a.Requirements,
	}
	if a.Reason != "" {
		v := a.Reason
		out.Reason = &v
	}
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

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func boolDefault(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func intDefault(v *int, fallback int) int {
	if v == nil || *v <= 0 {
		return fallback
	}
	return *v
}

func floatDefault(v *float64, fallback float64) float64 {
	if v == nil {
		return fallback
	}
	return *v
}

func jsonToStringMap(in model.JSON) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}
