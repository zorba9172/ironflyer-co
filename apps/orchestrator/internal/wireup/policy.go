package wireup

import (
	"context"

	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/policy"
)

// BuildPolicyPEP constructs the policy plane PEP. The Auditor wraps the
// orchestrator's audit.Store so every PDP decision lands a row on the
// hash chain. nil audit store is allowed — the auditor short-circuits.
func BuildPolicyPEP(cfg policy.Config, auditStore audit.Store, log zerolog.Logger) (*policy.PEP, error) {
	auditor := policy.NewAuditor(&policyAuditEmitter{store: auditStore})
	return policy.NewPEP(cfg, auditor, log)
}

// policyAuditEmitter maps policy.Auditor.Emit calls onto
// audit.Store.Record. We deliberately do NOT import audit from the
// policy package — the bridge sits here in wireup so the policy
// package stays a leaf of the dependency graph.
type policyAuditEmitter struct {
	store audit.Store
}

// Emit appends one audit row. action is a dotted vocabulary string
// like "policy.decision.allow"; we record it as the Entry.Action
// verbatim so operators can filter the chain by policy verdict.
func (e *policyAuditEmitter) Emit(ctx context.Context, kind string, attrs map[string]any) error {
	if e == nil || e.store == nil {
		return nil
	}
	outcome := audit.OutcomeSuccess
	switch kind {
	case "policy.decision.deny":
		outcome = audit.OutcomeBlocked
	case "policy.decision.error":
		outcome = audit.OutcomeFailure
	}
	_, err := e.store.Record(ctx, audit.Entry{
		Action:  audit.Action(kind),
		Outcome: outcome,
		Summary: summarizePolicyDecision(attrs),
		Attrs:   attrs,
	})
	return err
}

// summarizePolicyDecision condenses the structured attrs into a
// human-readable summary so an operator scanning the audit chain
// without a JSON viewer can still get the gist.
func summarizePolicyDecision(attrs map[string]any) string {
	if len(attrs) == 0 {
		return "policy decision"
	}
	action, _ := attrs["action"].(string)
	effect, _ := attrs["effect"].(string)
	reason, _ := attrs["reason"].(string)
	if action == "" {
		action = "?"
	}
	if effect == "" {
		effect = "?"
	}
	if reason == "" {
		return action + " -> " + effect
	}
	return action + " -> " + effect + ": " + reason
}
