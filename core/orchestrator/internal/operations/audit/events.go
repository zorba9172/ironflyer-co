package audit

// V22 mandatory audit event constants.
//
// POLICY_SECURITY.md "Audit Chain → Mandatory audit events" enumerates
// the closed list of action classes every consequential side-effect of
// the orchestrator MUST land on the hash chain. The pre-V22 vocabulary
// in audit.go is the original (shorter) set; the constants below extend
// it without renaming so existing callers and the audit dashboard stay
// stable.
//
// Naming convention: "<domain>.<verb>.v1". The trailing `.v1` is the
// schema version of the structured Attrs payload — when the attribute
// shape changes for a class, we bump to `.v2` and keep both alive while
// dashboards / SIEM exporters migrate.
//
// These strings are wire values. Do NOT rename in-place; introduce a
// new constant and let the old one decay.
const (
	EventAuthLifecycle           = "auth.lifecycle.v1"
	EventSessionChange           = "auth.session_change.v1"
	EventGraphQLHighRiskMutation = "graphql.high_risk_mutation.v1"
	EventGraphQLPolicyDeny       = "graphql.policy_deny.v1"
	EventPolicyHighRiskAllow     = "policy.high_risk_allow.v1"
	EventPolicyDeny              = "policy.deny.v1"
	EventProfitGuardDecision     = "profitguard.decision.v1"
	EventProviderDispatch        = "provider.dispatch.v1"
	EventWorkspaceCommandExec    = "workspace.command_exec.v1"
	EventSecretRefWrite          = "secret.ref_write.v1"
	EventSecretRelease           = "secret.release.v1"
	EventSecretRotation          = "secret.rotation.v1"
	EventSecretReleaseDeny       = "secret.release_deny.v1"
	EventPatchProposed           = "patch.proposed.v1"
	EventPatchApproved           = "patch.approved.v1"
	EventPatchApplied            = "patch.applied.v1"
	EventPatchRolledBack         = "patch.rolled_back.v1"
	EventGateVerdict             = "gate.verdict.v1"
	EventGateWaiver              = "gate.waiver.v1"
	EventDeployPlan              = "deploy.plan.v1"
	EventDeployApproval          = "deploy.approval.v1"
	EventDeployProviderAction    = "deploy.provider_action.v1"
	EventDeploySmokeResult       = "deploy.smoke_result.v1"
	EventDeployRollback          = "deploy.rollback.v1"
	EventBreakGlass              = "operator.break_glass.v1"
	EventAbuseEscalation         = "abuse.escalation.v1"
	EventAbuseThrottle           = "abuse.throttle.v1"
	EventAbuseSuspension         = "abuse.suspension.v1"
)

// MandatoryEventNames returns the full closed set of V22 mandatory
// event names in declaration order. The audit verify operator surface
// uses this to assert that every class has at least one entry in a
// production retention window — a missing class is a wiring bug, not
// just an empty section.
func MandatoryEventNames() []string {
	return []string{
		EventAuthLifecycle,
		EventSessionChange,
		EventGraphQLHighRiskMutation,
		EventGraphQLPolicyDeny,
		EventPolicyHighRiskAllow,
		EventPolicyDeny,
		EventProfitGuardDecision,
		EventProviderDispatch,
		EventWorkspaceCommandExec,
		EventSecretRefWrite,
		EventSecretRelease,
		EventSecretRotation,
		EventSecretReleaseDeny,
		EventPatchProposed,
		EventPatchApproved,
		EventPatchApplied,
		EventPatchRolledBack,
		EventGateVerdict,
		EventGateWaiver,
		EventDeployPlan,
		EventDeployApproval,
		EventDeployProviderAction,
		EventDeploySmokeResult,
		EventDeployRollback,
		EventBreakGlass,
		EventAbuseEscalation,
		EventAbuseThrottle,
		EventAbuseSuspension,
	}
}
