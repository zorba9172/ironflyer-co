# Provider dispatch — gates every outbound LLM provider call.
#
# Concerns:
#   1. Data residency: tenant.data_residency must match provider.region.
#   2. Cross-tenant prompt batching is forbidden — every dispatch maps
#      to exactly one tenant.
#   3. Secrets must already be redacted (context.redaction_proof set).
#
# Action: "provider.dispatch"
# Resource: kind=provider, id=<provider.model>
# Context keys:
#   context.tenant_data_residency  ("us" | "eu" | "any")
#   context.provider_region        ("us" | "eu" | ...)
#   context.batch_tenant_ids       (array of tenant IDs in this batch)
#   context.redaction_proof        (non-empty string when redaction ran)
package ironflyer

allow_votes["provider_dispatch"] {
    input.action == "provider.dispatch"
    _residency_ok
    _single_tenant_batch
    input.context.redaction_proof != ""
}

deny[reason] {
    input.action == "provider.dispatch"
    not _residency_ok
    reason := "provider_data_residency_violation"
}

deny[reason] {
    input.action == "provider.dispatch"
    not _single_tenant_batch
    reason := "provider_cross_tenant_batch"
}

deny[reason] {
    input.action == "provider.dispatch"
    not input.context.redaction_proof
    reason := "provider_missing_redaction_proof"
}

_residency_ok {
    input.context.tenant_data_residency == "any"
}

_residency_ok {
    input.context.tenant_data_residency == input.context.provider_region
}

# A batch is single-tenant when either (a) no batch_tenant_ids was
# supplied (callers that don't batch) or (b) the set has cardinality
# one and equals the principal's tenant.
_single_tenant_batch {
    not input.context.batch_tenant_ids
}

_single_tenant_batch {
    ids := input.context.batch_tenant_ids
    count(ids) == 1
    ids[0] == input.principal.tenant_id
}

risk_for_allow["medium"] {
    input.action == "provider.dispatch"
}

obligations_for_allow[o] {
    input.action == "provider.dispatch"
    o := {"kind": "redact_model_context", "params": {"already_redacted": true}}
}
