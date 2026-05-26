# GraphQL operation gate — allows authenticated operations through
# the middleware. Domain-level authorization happens later via
# resolver-side PEP.MustAllow calls; this bundle exists so the
# middleware doesn't blanket-deny every authenticated mutation.
#
# Anonymous principals are denied for non-public operations. The
# orchestrator owns the public-op allowlist (login, signup, public
# probes) elsewhere — the middleware skips this PEP for those.
#
# Action: "graphql.<operationType>.<operationName>"
# Resource: kind=graphql_op, id=<operationName>
package ironflyer

allow_votes["graphql_op"] {
    startswith(input.action, "graphql.")
    input.principal.kind == "user"
    input.principal.user_id != ""
}

allow_votes["graphql_op"] {
    startswith(input.action, "graphql.")
    input.principal.kind == "platform_operator"
}

deny[reason] {
    startswith(input.action, "graphql.")
    input.principal.kind == "anonymous"
    reason := "graphql_requires_authentication"
}
