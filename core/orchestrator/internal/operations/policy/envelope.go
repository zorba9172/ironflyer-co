package policy

// buildInput converts a DecisionRequest into the canonical map shape
// the Rego bundles consume. The keys MUST match
// docs/ARCHITECTURE_POLICY_SECURITY.md exactly because every bundle
// references them by name.
//
// Stable key contract (do not rename without rev-ing every bundle):
//
//	input.principal.{kind,user_id,tenant_id,session_id,roles,mfa}
//	input.delegation.{actor,agent_role,execution_id,workspace_id}
//	input.action  (dotted, e.g. "deploy.production.start")
//	input.resource.{kind,id,tenant_id,environment}
//	input.context.*  (free-form caller-supplied attributes)
func buildInput(req DecisionRequest) map[string]any {
	roles := req.Principal.Roles
	if roles == nil {
		// Rego prefers empty arrays over nulls when iterating with [_].
		roles = []string{}
	}
	ctxMap := req.Context
	if ctxMap == nil {
		ctxMap = map[string]any{}
	}
	return map[string]any{
		"principal": map[string]any{
			"kind":       req.Principal.Kind,
			"user_id":    req.Principal.UserID,
			"tenant_id":  req.Principal.TenantID,
			"session_id": req.Principal.SessionID,
			"roles":      roles,
			"mfa":        req.Principal.MFA,
		},
		"delegation": map[string]any{
			"actor":        req.Delegation.Actor,
			"agent_role":   req.Delegation.AgentRole,
			"execution_id": req.Delegation.ExecutionID,
			"workspace_id": req.Delegation.WorkspaceID,
		},
		"action": req.Action,
		"resource": map[string]any{
			"kind":        req.Resource.Kind,
			"id":          req.Resource.ID,
			"tenant_id":   req.Resource.TenantID,
			"environment": req.Resource.Environment,
		},
		"context": ctxMap,
	}
}
