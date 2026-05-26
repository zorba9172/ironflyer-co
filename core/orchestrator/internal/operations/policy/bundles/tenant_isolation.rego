# Tenant isolation — the first invariant every other bundle relies on.
#
# This bundle contributes ONLY deny rules. Allow has to come from an
# action-scoped bundle (deploy_approval, runtime_command,
# provider_dispatch, secret_release, graphql_op). The default bundle
# composes `decision.allow := any_allow AND NOT any_deny` so a deny
# here vetoes everything else.
#
# Cross-tenant access is denied unless the principal is a
# platform_operator AND a break-glass approval ID is present in the
# request context.
package ironflyer

# Resources with an empty tenant_id are treated as global (health
# probes, schema introspection); they are NOT auto-allowed by this
# bundle — they just don't trigger a tenant-mismatch deny.

deny[reason] {
    input.resource.tenant_id != ""
    input.principal.tenant_id != ""
    input.principal.tenant_id != input.resource.tenant_id
    input.principal.kind != "platform_operator"
    reason := "cross_tenant_access"
}

# Platform operators crossing tenants must attach a break-glass
# approval; otherwise deny.
deny[reason] {
    input.principal.kind == "platform_operator"
    input.resource.tenant_id != ""
    input.principal.tenant_id != input.resource.tenant_id
    not input.context.break_glass_approval_id
    reason := "operator_break_glass_required"
}

# Anonymous principals (no tenant) cannot touch tenant-scoped resources.
deny[reason] {
    input.resource.tenant_id != ""
    input.principal.tenant_id == ""
    input.principal.kind != "platform_operator"
    reason := "anonymous_principal_blocked"
}

# Platform-operator break-glass is itself high-risk and audited.
risk_for_allow["high"] {
    input.principal.kind == "platform_operator"
    input.context.break_glass_approval_id != ""
}

obligations_for_allow[o] {
    input.principal.kind == "platform_operator"
    input.context.break_glass_approval_id != ""
    o := {"kind": "audit.high_risk_allow", "params": {"reason": "operator_break_glass"}}
}
