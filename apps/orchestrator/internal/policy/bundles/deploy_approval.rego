# Deploy approval — production deploy is the most privileged action
# the orchestrator exposes. Consulted when action matches
# deploy.production.start / promote / rollback.
#
# Requirements per docs/ARCHITECTURE_POLICY_SECURITY.md §Deploy:
#   - Security and Deploy gates pass.
#   - ProfitGuard decision attached.
#   - Principal is tenant_admin (or platform_operator with break-glass).
#   - Resource environment is "production".
#   - Same-tenant access (deny override handles cross-tenant).
package ironflyer

allow_votes["deploy_approval"] {
    _is_production_deploy_action
    _has_tenant_admin
    _gates_pass
    input.context.profitguard_decision_id != ""
    input.resource.environment == "production"
}

deny[reason] {
    _is_production_deploy_action
    not _has_tenant_admin
    reason := "deploy_requires_tenant_admin"
}

deny[reason] {
    _is_production_deploy_action
    _has_tenant_admin
    not _gates_pass
    reason := "deploy_gates_not_passing"
}

deny[reason] {
    _is_production_deploy_action
    not input.context.profitguard_decision_id
    reason := "deploy_missing_profitguard_decision"
}

deny[reason] {
    _is_production_deploy_action
    input.resource.environment != "production"
    reason := "deploy_environment_mismatch"
}

risk_for_allow["critical"] {
    _is_production_deploy_action
    allow_votes["deploy_approval"]
}

reason_for_allow["tenant_admin_can_deploy_after_gate_pass"] {
    _is_production_deploy_action
    allow_votes["deploy_approval"]
}

ttl_for_allow[0] {
    _is_production_deploy_action
    allow_votes["deploy_approval"]
}

obligations_for_allow[o] {
    _is_production_deploy_action
    allow_votes["deploy_approval"]
    o := {"kind": "require_deploy_approval_id", "params": {"environment": "production"}}
}

obligations_for_allow[o] {
    _is_production_deploy_action
    allow_votes["deploy_approval"]
    o := {"kind": "audit.high_risk_allow", "params": {"action": input.action}}
}

_is_production_deploy_action {
    input.action == "deploy.production.start"
}

_is_production_deploy_action {
    input.action == "deploy.production.promote"
}

_is_production_deploy_action {
    input.action == "deploy.production.rollback"
}

_has_tenant_admin {
    input.principal.roles[_] == "tenant_admin"
}

_has_tenant_admin {
    input.principal.kind == "platform_operator"
    input.context.break_glass_approval_id != ""
}

_gates_pass {
    input.context.gate_state.security == "pass"
    input.context.gate_state.deploy == "pass"
}
