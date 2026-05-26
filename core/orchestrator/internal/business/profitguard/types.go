// Package profitguard is the economic safety layer of the V22
// orchestrator. Every expensive runtime action — premium model call,
// sandbox allocation, mobile build, Vercel production deploy, retry
// loop, long verification, large artifact write, and execution admit —
// passes through Guard.Decide before it runs.
//
// The full policy specification lives at
//   docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/
//     01-unit-economics/03-profit-guard-policy.md
// and the per-workload margin thresholds at
//     01-unit-economics/05-margin-thresholds.md
//
// This package deliberately does NOT import internal/execution,
// internal/wallet, internal/ledger, or internal/blueprints. The
// integration agent wires a thin adapter at call sites — keeping
// profitguard a pure decision module means it can be unit-reasoned
// about (and, eventually, exercised by simulators) without booting
// the rest of the orchestrator.
package profitguard

// Action is one of the eight canonical Profit Guard decisions defined
// in 03-profit-guard-policy.md. The string values are stable wire
// values — they land in the profit_guard_decisions table, in the
// GraphQL ProfitGuardDecision.decision field, and in Prometheus label
// values. Do NOT rename without coordinating a migration of the
// audit table.
type Action string

const (
	// Continue — the step is allowed to proceed unchanged.
	Continue Action = "continue"
	// Degrade — proceed but with a cheaper model tier or reduced
	// quality budget.
	Degrade Action = "degrade"
	// PauseForBudget — execution paused; the user must top up the
	// wallet before progress resumes.
	PauseForBudget Action = "pause_for_budget"
	// Stop — terminate the current execution; refund the unused hold.
	Stop Action = "stop"
	// KillBranch — terminate this particular reasoning / retry branch
	// but keep the execution alive on alternative branches.
	KillBranch Action = "kill_branch"
	// SwitchProvider — proceed with a cheaper provider that still
	// meets the quality bar.
	SwitchProvider Action = "switch_provider"
	// ReuseBlueprint — short-circuit work by reusing a known
	// blueprint that matches the current intent.
	ReuseBlueprint Action = "reuse_blueprint"
	// ReuseRepair — short-circuit a retry by replaying a known
	// repair recipe for the failure signature.
	ReuseRepair Action = "reuse_repair"
)

// String makes Action satisfy fmt.Stringer; useful for log/metric
// formatters that take any.
func (a Action) String() string { return string(a) }

// EnforcementPoint enumerates every hook where Decide is called. The
// list mirrors 03-profit-guard-policy.md; the BeforeExecutionAdmit
// variant is added by V22 so admission and per-step gates share the
// same Guard surface.
type EnforcementPoint string

const (
	// BeforeModelCall — any provider chat/completion call.
	BeforeModelCall EnforcementPoint = "before_model_call"
	// BeforeSandboxAllocation — runtime asks for a per-user sandbox.
	BeforeSandboxAllocation EnforcementPoint = "before_sandbox_allocation"
	// BeforeMobileBuild — kicking a Capacitor / Xcode / Gradle build.
	BeforeMobileBuild EnforcementPoint = "before_mobile_build"
	// BeforeVercelDeploy — Vercel production deploy step.
	BeforeVercelDeploy EnforcementPoint = "before_vercel_deploy"
	// BeforePremiumReasoning — invoking the premium reasoning tier
	// (Opus 4.7 / o3 / equivalent).
	BeforePremiumReasoning EnforcementPoint = "before_premium_reasoning"
	// BeforeRetryLoop — entering an automatic retry loop after a gate
	// or test failure.
	BeforeRetryLoop EnforcementPoint = "before_retry_loop"
	// BeforeLongVerification — long-running verification (e.g. full
	// test sweep, large diff review).
	BeforeLongVerification EnforcementPoint = "before_long_verification"
	// BeforeArtifactStore — persisting a large artifact (>quota) to
	// S3 / R2 / MinIO.
	BeforeArtifactStore EnforcementPoint = "before_artifact_store"
	// BeforeExecutionAdmit — admission gate for a freshly created
	// paid execution. Runs before the wallet hold is committed.
	BeforeExecutionAdmit EnforcementPoint = "before_execution_admit"
	// BeforeDomainPurchase — registrar Purchase() call. Already capped
	// at $75 by DomainPurchasePolicy.MaxPriceUSD; this gate adds a
	// margin-aware verdict so a domain buy doesn't push a tenant into
	// negative margin. Verdict semantics for this point are strictly
	// allow / refuse — a one-shot registrar purchase has no graceful
	// downgrade path. Stop / KillBranch / PauseForBudget all translate
	// to ErrProfitGuardBlocked on the call site; every other action
	// (Continue, Degrade, SwitchProvider, ReuseBlueprint, ReuseRepair)
	// is treated as "go ahead" because the price ceiling has already
	// been clamped by the policy layer.
	BeforeDomainPurchase EnforcementPoint = "before_domain_purchase"
)

// String makes EnforcementPoint satisfy fmt.Stringer.
func (p EnforcementPoint) String() string { return string(p) }
