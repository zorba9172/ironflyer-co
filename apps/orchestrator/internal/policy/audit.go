package policy

import (
	"context"
	"fmt"
)

// AuditEmitter is the thin shim between the policy plane and the
// rest of the orchestrator's audit package. We intentionally do NOT
// import internal/audit directly so this package stays a leaf of the
// dependency graph (the integration agent wires the bridge in main).
type AuditEmitter interface {
	// Emit appends a single audit row. kind is a dotted action
	// vocabulary string (e.g. "policy.decision"); attrs is the
	// structured payload that lands in the audit row's Attrs field.
	Emit(ctx context.Context, kind string, attrs map[string]any) error
}

// Auditor bridges PDP Decide calls into the audit hash chain. Every
// allow and every deny lands one row so an operator can replay why
// every consequential side effect was permitted.
type Auditor struct {
	emit AuditEmitter
}

// NewAuditor wraps an AuditEmitter. A nil emitter is allowed (no-op
// auditor) so callers running tests / single-node smoke can skip the
// audit chain wiring.
func NewAuditor(emit AuditEmitter) *Auditor {
	return &Auditor{emit: emit}
}

// Record writes one audit row per PDP Decide call. Both allows AND
// denies are recorded so the audit chain proves what was permitted,
// not just what was blocked. evalErr is included as a structured
// attribute when non-nil.
//
// The kind taxonomy:
//
//	policy.decision.allow  — successful allow
//	policy.decision.deny   — explicit deny from a bundle
//	policy.decision.error  — PDP eval / transport error
func (a *Auditor) Record(ctx context.Context, req DecisionRequest, dec Decision, evalErr error) error {
	if a == nil || a.emit == nil {
		return nil
	}
	kind := "policy.decision.allow"
	if evalErr != nil {
		kind = "policy.decision.error"
	} else if dec.Effect == EffectDeny {
		kind = "policy.decision.deny"
	}

	attrs := map[string]any{
		"decision_id":           dec.DecisionID,
		"policy_bundle_version": dec.PolicyBundleVersion,
		"effect":                string(dec.Effect),
		"risk":                  dec.Risk,
		"reason":                dec.Reason,
		"ttl_seconds":           dec.TTLSeconds,
		"action":                req.Action,
		"principal_kind":        req.Principal.Kind,
		"principal_user_id":     req.Principal.UserID,
		"principal_tenant_id":   req.Principal.TenantID,
		"principal_roles":       req.Principal.Roles,
		"delegation_actor":      req.Delegation.Actor,
		"delegation_agent_role": req.Delegation.AgentRole,
		"delegation_execution":  req.Delegation.ExecutionID,
		"delegation_workspace":  req.Delegation.WorkspaceID,
		"resource_kind":         req.Resource.Kind,
		"resource_id":           req.Resource.ID,
		"resource_tenant_id":    req.Resource.TenantID,
		"resource_environment":  req.Resource.Environment,
	}
	if len(dec.Obligations) > 0 {
		obls := make([]map[string]any, 0, len(dec.Obligations))
		for _, o := range dec.Obligations {
			obls = append(obls, map[string]any{"kind": o.Kind, "params": o.Params})
		}
		attrs["obligations"] = obls
	}
	if evalErr != nil {
		attrs["error"] = evalErr.Error()
	}
	if err := a.emit.Emit(ctx, kind, attrs); err != nil {
		return fmt.Errorf("policy: audit emit: %w", err)
	}
	return nil
}
