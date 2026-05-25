# Secret release — every secret crossing the broker boundary asks the
# PDP first. Three release classes are allowed, each with a distinct
# obligation set:
#
#   build_time_reference: name only, visible to AI.
#   runtime_mount:        value flows into workspace env, hidden from AI.
#   operator_break_glass: incident access, two-person approved.
#
# AI agent_role NEVER receives the raw secret value. The runtime PEP
# enforces this by inspecting the obligation list before mounting.
#
# Action: "secret.release"
# Resource: kind=secret, id=<secret_ref>
# Context keys:
#   context.release_class    ("build_time_reference" | "runtime_mount" |
#                             "operator_break_glass")
#   context.two_person_approval_id  (required for break-glass)
package ironflyer

allow_votes["secret_release"] {
    input.action == "secret.release"
    input.context.release_class == "build_time_reference"
    # Build-time reference is name-only; safe to expose to AI.
}

allow_votes["secret_release"] {
    input.action == "secret.release"
    input.context.release_class == "runtime_mount"
    # Runtime mount must NOT go to an AI delegate. The runtime PEP
    # consumes the obligation and refuses to mount into AI-visible
    # context.
    not _delegate_is_ai
}

allow_votes["secret_release"] {
    input.action == "secret.release"
    input.context.release_class == "operator_break_glass"
    input.principal.kind == "platform_operator"
    input.context.two_person_approval_id != ""
}

deny[reason] {
    input.action == "secret.release"
    input.context.release_class == "runtime_mount"
    _delegate_is_ai
    reason := "secret_runtime_mount_blocked_for_ai"
}

deny[reason] {
    input.action == "secret.release"
    input.context.release_class == "operator_break_glass"
    not input.context.two_person_approval_id
    reason := "secret_break_glass_requires_two_person_approval"
}

deny[reason] {
    input.action == "secret.release"
    not _known_release_class
    reason := "secret_unknown_release_class"
}

_delegate_is_ai {
    input.delegation.actor == "ai_agent"
}

_known_release_class {
    input.context.release_class == "build_time_reference"
}
_known_release_class {
    input.context.release_class == "runtime_mount"
}
_known_release_class {
    input.context.release_class == "operator_break_glass"
}

risk_for_allow["critical"] {
    input.action == "secret.release"
    input.context.release_class == "operator_break_glass"
}

risk_for_allow["high"] {
    input.action == "secret.release"
    input.context.release_class == "runtime_mount"
}

ttl_for_allow[0] {
    input.action == "secret.release"
    input.context.release_class == "operator_break_glass"
}

obligations_for_allow[o] {
    input.action == "secret.release"
    input.context.release_class == "runtime_mount"
    o := {"kind": "redact_model_context", "params": {"secret_ref": input.resource.id}}
}

obligations_for_allow[o] {
    input.action == "secret.release"
    input.context.release_class == "operator_break_glass"
    o := {"kind": "audit.high_risk_allow", "params": {"reason": "operator_break_glass"}}
}
