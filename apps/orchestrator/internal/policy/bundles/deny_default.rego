# Deny-by-default base bundle.
#
# The Ironflyer policy plane is deny-by-default. The query
# data.ironflyer.decision composes:
#
#   * any_allow  := at least one action-scoped bundle voted allow
#   * any_deny   := at least one bundle (tenant_isolation, action
#                   bundle, abuse) voted deny[reason]
#   * allow      := any_allow AND NOT any_deny
#
# A deny anywhere vetoes every allow. Silence is denial.
package ironflyer

# allow_<action> rules are contributed by per-action bundles. They
# fire only when the principal/resource/context fully satisfies the
# bundle's preconditions.

# any_allow is true when at least one per-bundle allow vote landed.
any_allow {
    allow_votes[_]
}

# allow_votes is the set of allow contributors. Bundles add to it via
# `allow_votes[\"deploy_approval\"]` style rules.

# any_deny is true when any deny[reason] fired.
any_deny {
    deny[_]
}

default allow := false

allow {
    any_allow
    not any_deny
}

# decision is the canonical PDP response object.
decision := out {
    allow
    out := {
        "effect": "allow",
        "risk": _risk_for_allow,
        "reason": _reason_for_allow,
        "ttl_seconds": _ttl_for_allow,
        "obligations": _obligations_for_allow,
    }
}

decision := out {
    not allow
    out := {
        "effect": "deny",
        "risk": _risk_for_deny,
        "reason": _deny_reason,
        "ttl_seconds": 0,
        "obligations": [],
    }
}

# _deny_reason picks any deny reason if a deny fired; otherwise the
# distinct "no_matching_allow" signal so the audit chain separates
# explicit deny from "no bundle voted".
_deny_reason := r {
    any_deny
    some r
    deny[r]
} else := "no_matching_allow" {
    not any_deny
}

# When a deny fired, surface critical; otherwise default-deny is high.
_risk_for_deny := "critical" {
    any_deny
} else := "high"

# Default risk for an allow is "low" unless a bundle pinned a higher
# tier in risk_for_allow.
_risk_for_allow := r {
    some r
    risk_for_allow[r]
} else := "low"

_reason_for_allow := r {
    some r
    reason_for_allow[r]
} else := "allowed"

_ttl_for_allow := t {
    some t
    ttl_for_allow[t]
} else := 60

_obligations_for_allow := arr {
    arr := [o | obligations_for_allow[o]]
}
