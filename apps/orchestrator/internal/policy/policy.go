// Package policy is the Ironflyer policy plane: a Policy Decision Point
// (PDP) plus Policy Enforcement Point (PEP) helpers and middleware.
//
// The PDP evaluates Rego bundles via OPA and returns a small, stable
// Decision (effect, risk, reason, obligations, ttl, decision_id). The
// PEP wrappers (see pep.go) embed those decisions at the call sites
// that matter: GraphQL operations, runtime command exec, provider
// dispatch, deploy approval, secret release, patch apply.
//
// The trust law this package enforces:
//
//  1. Deny by default. The default bundle returns deny when no other
//     bundle allows.
//  2. AI is never a principal of record. Principal is the user;
//     Delegation captures the AI/agent acting under that user.
//  3. Every Decision carries a non-empty DecisionID so the audit chain
//     can pin it.
//
// Topology (see docs/ARCHITECTURE_POLICY_SECURITY.md):
//
//	GraphQL -> PEP.Allow("graphql.<op>.<name>") -> PDP -> audit
//	Resolver -> PEP.MustAllow("deploy.production.start") -> PDP -> audit
//	Runtime -> PEP.MustAllow("runtime.exec") -> PDP -> audit
//	Provider router -> PEP.MustAllow("provider.dispatch") -> PDP -> audit
//	Secret broker -> PEP.MustAllow("secret.release") -> PDP -> audit
package policy

import "context"

// Effect is the binary outcome of a policy evaluation.
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Risk is a coarse risk tier emitted by the PDP so the audit chain and
// the GraphQL surface can color denies and high-risk allows.
const (
	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"
)

// Decision is the stable PDP response shape. The contract is identical
// for local-in-process OPA and remote OPA sidecar deployments.
type Decision struct {
	// DecisionID is a `pdec_<uuid>` minted by the PDP for every call.
	// Audit rows reference this so an operator can replay why a side
	// effect was allowed or denied.
	DecisionID string `json:"decision_id"`
	// Effect is "allow" or "deny". Anything that fails to parse becomes
	// EffectDeny so a malformed bundle cannot accidentally allow.
	Effect Effect `json:"effect"`
	// Risk is one of low | medium | high | critical.
	Risk string `json:"risk"`
	// Reason is the matched policy rule's human-readable explanation.
	Reason string `json:"reason"`
	// Obligations are post-conditions the caller MUST satisfy before the
	// side effect proceeds (e.g. require_deploy_approval_id).
	Obligations []Obligation `json:"obligations,omitempty"`
	// TTLSeconds is how long this decision can be cached/replayed. High-
	// risk allows use single-use TTLs (0 or very small).
	TTLSeconds int `json:"ttl_seconds"`
	// PolicyBundleVersion is the bundle hash the PDP evaluated against.
	PolicyBundleVersion string `json:"policy_bundle_version"`
}

// Obligation is a kind + params bag. Kinds are intentionally enumerated
// in the bundles; callers should pattern-match on Kind.
type Obligation struct {
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params,omitempty"`
}

// DecisionRequest is the input envelope every PEP builds. The shape
// mirrors the JSON contract in docs/ARCHITECTURE_POLICY_SECURITY.md;
// see envelope.go for the canonical map serialization OPA sees.
type DecisionRequest struct {
	Principal  Principal
	Delegation Delegation
	Action     string
	Resource   Resource
	Context    map[string]any
}

// Principal is the human/system identity of record. AI is never a
// Principal — it goes into Delegation.
type Principal struct {
	Kind      string   // "user" | "service_account" | "platform_operator"
	UserID    string   // usr_...
	TenantID  string   // ten_...
	SessionID string   // sess_...
	Roles     []string // ["tenant_admin", ...]
	MFA       bool
}

// Delegation captures the AI/agent or other delegated actor acting on
// behalf of the Principal. Delegation can only reduce privileges; it
// can never grant new privileges (enforced in Rego).
type Delegation struct {
	Actor       string // "ai_agent" | "automation" | ""
	AgentRole   string // "coder" | "planner" | "deployer" | ""
	ExecutionID string // exe_...
	WorkspaceID string // ws_...
}

// Resource is the target object. TenantID MUST match Principal.TenantID
// unless Principal.Kind == platform_operator + break-glass approval.
type Resource struct {
	Kind        string // "project" | "execution" | "workspace" | "secret" | "deploy" | "graphql_op"
	ID          string // prj_... / exe_... / ...
	TenantID    string
	Environment string // "development" | "preview" | "production"
}

// PDP is the Policy Decision Point contract. Both opa_local.go and
// opa_remote.go implement it; opa_disabled.go provides a default-allow
// stub used only when policy is explicitly disabled.
type PDP interface {
	// Decide evaluates the request and returns a Decision. The returned
	// Decision always carries a non-empty DecisionID, even on error
	// paths, so the audit chain can pin the call.
	Decide(ctx context.Context, req DecisionRequest) (Decision, error)
	// BundleVersion returns the hash/identifier of the active policy
	// bundles. Exposed for /policy/version + audit.
	BundleVersion() string
}
