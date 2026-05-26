// Package finisher is the heart of Ironflyer: the gate-driven completion
// loop that turns an idea into a finished product. It runs gates in order,
// dispatches repair agents on failure, and exits only when all gates pass
// (or max iterations / blocked).
package finisher

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/refactor"
	"ironflyer/core/orchestrator/internal/operations/arch"
	"ironflyer/core/orchestrator/internal/operations/audit"
	// Aliased to budgetpkg because a sibling file in this package
	// (gates_mobile_size.go) defines a local `type budget struct` for
	// mobile artifact size thresholds. Without the alias every file
	// in the package would have to rename the local type, which is
	// invasive across a parallel agent's mobile work.
	budgetpkg "ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/operations/bus"
	"ironflyer/core/orchestrator/internal/ai/completion"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/business/execution"
	"ironflyer/core/orchestrator/internal/business/lastprovider"
	"ironflyer/core/orchestrator/internal/operations/logctx"
	"ironflyer/core/orchestrator/internal/ai/memory"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/business/profitguard"
	"ironflyer/core/orchestrator/internal/business/profitguardbridge"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
	"ironflyer/core/orchestrator/internal/ai/providers"
	"ironflyer/core/orchestrator/internal/operations/redisbus"
	"ironflyer/core/orchestrator/internal/operations/runtime"
	"ironflyer/core/orchestrator/internal/operations/store"
)

// eventsChannel returns the Redis pub/sub channel name for a project's
// event stream. Centralised so emit + Subscribe stay in lockstep.
func eventsChannel(projectID string) string {
	return "ironflyer:events:" + projectID
}

// gateToAgent maps a finisher gate name onto the canonical agent
// role label the studio chat surfaces in stage events. Unknown
// gates fall through to "agent" so the surface never goes blank.
func gateToAgent(name domain.GateName) string {
	switch name {
	case domain.GateSpec:
		return "planner"
	case domain.GateUX:
		return "uxer"
	case domain.GateArch:
		return "architect"
	case domain.GateCode:
		return "coder"
	case domain.GateLint:
		return "coder"
	case domain.GateTest:
		return "tester"
	case domain.GateSecurity:
		return "sec"
	case domain.GateBudget:
		return "budget"
	case domain.GateDeploy:
		return "deployer"
	default:
		return "agent"
	}
}

// truncateMsg trims s to n runes with an ellipsis. Used so the
// studio chat doesn't render multi-paragraph refinements verbatim
// as a single bubble.
func truncateMsg(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ctxKey is unexported so callers must use WithBearer / bearerFromCtx.
type ctxKey struct{ name string }

var (
	bearerKey      = ctxKey{"bearer"}
	workspaceIDKey = ctxKey{"workspaceID"}
)

// WithBearer stamps the user's JWT onto the context so gates that hit the
// workspace runtime can re-present it for the ownership check.
func WithBearer(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, bearerKey, token)
}

func bearerFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(bearerKey).(string); ok {
		return v
	}
	return ""
}

// withWorkspaceID stamps the resolved workspace ID onto the context so
// built-in tools dispatched deep inside an agent Run (e.g. generate_image)
// can target the caller's sandbox without re-resolving it.
func withWorkspaceID(ctx context.Context, ws string) context.Context {
	if ws == "" {
		return ctx
	}
	return context.WithValue(ctx, workspaceIDKey, ws)
}

func workspaceIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(workspaceIDKey).(string); ok {
		return v
	}
	return ""
}

const (
	defaultMaxIterations    = 4
	defaultMaxCoderRetries  = 3
	defaultMaxPatchBytes    = 256 * 1024 // 256 KiB across all changes in one patch
	defaultMaxFilesPerPatch = 40
)

// adaptiveCaps scales the per-run resource budgets up for genuinely
// large projects. The defaults are tuned for MVPs (3-7 stories); a
// 25-story project with 12 subprojects shouldn't be choked off by a
// 40-file patch ceiling. We grow the ceilings monotonically with the
// project's apparent complexity (stories + subprojects + file count)
// so MVPs stay fast and "serious projects" don't hit invisible walls.
func adaptiveCaps(p *domain.Project) (maxIter, maxRetries, maxBytes, maxFiles int) {
	storyCount := len(p.Spec.UserStories)
	subCount := len(p.Subprojects)
	fileCount := len(p.Files)
	complexity := storyCount + 2*subCount + fileCount/20

	switch {
	case complexity >= 40:
		return 10, 6, 2 * 1024 * 1024, 200
	case complexity >= 20:
		return 7, 5, 1 * 1024 * 1024, 100
	case complexity >= 10:
		return 5, 4, 512 * 1024, 60
	default:
		return defaultMaxIterations, defaultMaxCoderRetries, defaultMaxPatchBytes, defaultMaxFilesPerPatch
	}
}

type RunReport struct {
	ProjectID  string             `json:"projectId"`
	Iterations int                `json:"iterations"`
	Gates      []domain.GateState `json:"gates"`
	Completed  bool               `json:"completed"`
	StartedAt  time.Time          `json:"startedAt"`
	FinishedAt time.Time          `json:"finishedAt"`
	AgentRuns  []agents.Result    `json:"agentRuns,omitempty"`
	PatchIDs   []string           `json:"patchIds,omitempty"`
}

// BudgetSource is the optional callback the Engine consults before each
// gate iteration to populate GateEnv.Budget. Implementations typically
// inspect the budget ledger + the user's plan; returning a nil snapshot is
// fine and degrades the Budget gate to a warning.
type BudgetSource func(ctx context.Context, userID, projectID string) (*BudgetSnapshot, error)

type Engine struct {
	mu               sync.RWMutex
	projects         store.Store
	registry         *agents.Registry
	patches          *patch.Engine
	runtime          *runtime.Client
	applier          RuntimeApplier
	budget           BudgetSource
	dbProvisioner    DBProvisioner
	authScaffolder    AuthScaffolder
	domainScaffolders []DomainScaffolder
	memory           memory.Store
	federation       memory.FederationStore
	audit            audit.Store
	redis            *redisbus.Client
	bus              *bus.Multiplexer
	gates            []Gate
	maxIterations    int
	maxCoderRetries  int
	maxPatchBytes    int
	maxFilesPerPatch int
	subscribers      map[string][]chan domain.Event
	plans            *planCache
	patchesCache     *patchCache
	runSlots         *runSlots

	// V22 ProfitGuard hook for the retry loop. Both fields nil means
	// "no enforcement" — the recovery engine runs unchanged. Wired by
	// the integration agent in main.go via WithProfitGuard.
	profitGuard      ProfitGuardHook

	// V22 Settler hook for terminal close-out. When wired, Run() calls
	// Close at the end with the derived finalStatus (succeeded vs.
	// failed) for the executionID currently on the context. Nil-safe
	// — no execution id on ctx means "internal call", no settlement
	// is attempted.
	settler          ExecutionSettler

	// V22 LearningHooks bridge — wired to the repair genome + patch
	// memory in main.go. The retry / patch-apply paths thread their
	// learning signals through here. Nil-safe: a zero hook is treated
	// as "learning disabled" and the run proceeds unchanged.
	learning         *LearningHooks

	// V22 SandboxBiller — wraps each workspace allocation in a ticker
	// that lands `sandbox_cost` debits on the active execution. Nil
	// means "billing disabled" (dev / no-execution callers): the
	// allocation site falls back to a no-op stop func and proceeds.
	sandboxBiller    *runtime.SandboxBiller

	// V22 completion scorer + execution service — after each gate
	// verdict the engine appends a GateOutcome to the scorer and
	// mirrors the resulting absolute score back onto the execution
	// row so ProfitGuard reads fresh signal. Both are nil-safe:
	// without a scorer or execution service the gate loop runs
	// unchanged and completion_score stays at zero (legacy
	// behaviour).
	completionScorer completion.Scorer
	executionService execution.Service

	// V22 QualitySink — per-gate verdict signal the bandit folds into
	// provider rewards. Nil-safe: without a sink wired, recordGateOutcome
	// only drives completion scoring. Provider/Capability context is not
	// yet threaded through the gate loop; for now the sink is held so
	// future restructuring can populate the GateOutcome fields without a
	// new wireup pass.
	qualitySink providers.QualitySink

	// V22 BeforeSandboxAllocation hook — the Guard the engine
	// consults before letting sandboxBiller.Track land. Nil means
	// "no economic gate on sandbox allocation" (sandbox runs
	// unconditionally). bridgeDeps carries the Registry + Genome
	// the SnapshotFor helper needs so SimilarBlueprintAvailable /
	// SimilarRepairAvailable are non-trivially populated.
	profitGuardFull profitguard.Guard
	bridgeDeps      profitguardbridge.BridgeDeps

	// V22 BeforeMobileBuild hook — consulted by MobileBuildGate
	// before kicking platform builds (gradlew assembleDebug, xcodebuild,
	// eas build --local). Nil-safe: without it the build runs unguarded.
	mobileBuildHook MobileBuildHook

	// Anti-Bloat Engine wireup (playbook §8.7). archManifest powers
	// the DepGraph / ArchBoundary gates; refactor powers the Dedup
	// gate's "and here's the fix" upgrade. Both are nil-safe; the
	// relevant gates degrade to SeverityInfo "tool not wired" when
	// unset. See docs/ANTI_BLOAT_ENGINE.md.
	archManifest *arch.Manifest
	refactor     *refactor.Service
}

// ExecutionSettler is the engine-facing seam onto the execution
// lifecycle Settler. Defined here so internal/finisher does not have
// to import internal/execution and risk a cycle later.
//
// The adapter wired by main.go is expected to drive the FSM
// transition (execSvc.Succeed / execSvc.Fail) AND call the canonical
// execution.Settler.Close in the same step — the engine is the
// terminal-status boundary for executions driven by Run().
type ExecutionSettler interface {
	// SettleSucceeded is called at the end of Run() when every gate
	// passed within the iteration budget. Implementations move the
	// FSM through Succeed and run the wallet/ledger close-out.
	SettleSucceeded(ctx context.Context, executionID string) error
	// SettleFailed is called at the end of Run() when the loop bailed
	// out (max iterations, unrepairable gate, throttle, etc).
	// `reason` lands on the execution row's failure_reason.
	SettleFailed(ctx context.Context, executionID, reason string) error
}

// ProfitGuardHook is the recovery-loop seam onto the profitguard
// package. Defined here as an interface so internal/finisher does not
// import the V22 packages directly — keeps the package's coupling
// surface unchanged.
type ProfitGuardHook interface {
	// BeforeRetry returns true to allow the retry to proceed. Returning
	// false short-circuits the recovery loop (the gate stays failed but
	// no more provider calls are made). reason is logged.
	BeforeRetry(ctx context.Context, executionID, gate string, attempt int, spentUSD float64) (allow bool, reason string)
}

func NewEngine(projects store.Store, registry *agents.Registry, patches *patch.Engine) *Engine {
	return &Engine{
		projects:         projects,
		registry:         registry,
		patches:          patches,
		applier:          NoopRuntimeApplier{},
		gates:            DefaultGates(),
		maxIterations:    defaultMaxIterations,
		maxCoderRetries:  defaultMaxCoderRetries,
		maxPatchBytes:    defaultMaxPatchBytes,
		maxFilesPerPatch: defaultMaxFilesPerPatch,
		subscribers:      make(map[string][]chan domain.Event),
		plans:            newPlanCache(),
		patchesCache:     newPatchCache(),
	}
}

// WithRuntime attaches a workspace-runtime client so build/test gates can
// execute commands inside the user's sandbox. Returns the engine for chained
// configuration during startup.
// WithArchManifest wires the parsed `.ironflyer/architecture.json`
// manifest so the DepGraph / ArchBoundary gates can validate every
// patch's paths against the declared layer set. Nil disables the
// projection — the gates degrade to a SeverityInfo "manifest not
// loaded" rather than block. See docs/ANTI_BLOAT_ENGINE.md.
func (e *Engine) WithArchManifest(m *arch.Manifest) *Engine {
	e.archManifest = m
	return e
}

// WithRefactor wires the Anti-Bloat Refactor Proposer so the DedupGate
// can attach extract-to-shared-util proposals to clone findings. Nil
// disables the upgrade — the gate keeps surfacing findings verbatim.
func (e *Engine) WithRefactor(s *refactor.Service) *Engine {
	e.refactor = s
	return e
}

func (e *Engine) WithRuntime(c *runtime.Client) *Engine {
	e.runtime = c
	return e
}

// WithApplier registers a RuntimeApplier the loop will call to materialise
// validated patches into the user's workspace. Passing nil keeps the
// default no-op applier (project state remains in-memory only).
func (e *Engine) WithApplier(a RuntimeApplier) *Engine {
	if a == nil {
		e.applier = NoopRuntimeApplier{}
	} else {
		e.applier = a
	}
	return e
}

// WithMaxCoderRetries overrides the number of revise-and-retry rounds the
// loop grants the Coder when the Reviewer rejects a patch. Default 3.
func (e *Engine) WithMaxCoderRetries(n int) *Engine {
	if n > 0 {
		e.maxCoderRetries = n
	}
	return e
}

// WithBudgetSource registers a callback the Budget gate uses to fetch the
// current spend posture for a project. Without this wired, the Budget gate
// degrades to a "spend tracking is dark" warning but the loop still
// progresses; callers should wire it in production.
func (e *Engine) WithBudgetSource(fn BudgetSource) *Engine {
	e.budget = fn
	return e
}

// WithDBProvisioner registers the database provisioner the pipeline calls
// before Coder. Pass NoopDBProvisioner{} (or leave unset) to disable —
// the loop still runs but generated apps that need a database will fail
// the Test gate. Wire a real backend (Supabase admin, Neon, in-cluster
// Postgres) in production deployments.
func (e *Engine) WithDBProvisioner(p DBProvisioner) *Engine {
	e.dbProvisioner = p
	return e
}

// WithAuthScaffolder registers the auth scaffolder. Pass
// DefaultAuthScaffolder{} for the canonical Supabase + Next.js recipe;
// nil disables auth scaffolding entirely so generated apps will lack a
// signup/login surface unless the Coder invents one.
func (e *Engine) WithAuthScaffolder(s AuthScaffolder) *Engine {
	e.authScaffolder = s
	return e
}

// WithMemory registers the persistent-intelligence store (Layer 6 of
// the AI Completion Infrastructure blueprint). The engine writes
// failure/fix lineage + decision records here and reads them back on
// every run to ground agent context. Nil disables the moat — gates
// still run, but the system stops accumulating intelligence between
// sessions.
func (e *Engine) WithMemory(m memory.Store) *Engine {
	e.memory = m
	return e
}

// WithFederation wires the owner-scoped memory-federation membership
// store. When set, contextBundle* helpers automatically widen their
// reads to also include relevant memories from other projects the
// same owner has opted into the federation pool. Nil-safe — passing
// nil keeps the per-project semantics intact.
func (e *Engine) WithFederation(f memory.FederationStore) *Engine {
	e.federation = f
	return e
}

// WithRedis attaches a Redis-backed bus so multi-pod deployments
// can coordinate finisher runs through a distributed lock. Nil-safe:
// passing nil leaves the engine on its single-pod path where the
// in-process mutex is sufficient.
func (e *Engine) WithRedis(c *redisbus.Client) *Engine {
	e.redis = c
	return e
}

// WithBus attaches a cross-pod Multiplexer that mirrors finisher run
// events to "finisher.run:<projectID>" so a Subscribe on another pod
// sees them. The legacy WithRedis path remains active for backwards
// compatibility but new wiring should prefer WithBus — the
// Multiplexer handles dedup via pod-id, removing the need for the
// per-Subscribe event-ID ring this file previously had to maintain.
// Nil-safe.
func (e *Engine) WithBus(b *bus.Multiplexer) *Engine {
	e.bus = b
	return e
}

// WithRunSlots installs a process-wide admission control layer for
// Run() calls: at most maxConcurrent in flight across the pod, and at
// most maxPerUser in flight for any single OwnerID. Callers that exceed
// either cap receive ErrRunThrottled within a short admission window.
// Nil-safe — leaving this unset means Run() proceeds unconstrained
// (the historical single-pod / dev behaviour).
func (e *Engine) WithRunSlots(maxConcurrent, maxPerUser int) *Engine {
	e.runSlots = newRunSlots(maxConcurrent, maxPerUser)
	return e
}

// WithAudit registers the immutable hash-chained audit log. The engine
// writes one entry per consequential action (patch proposed / applied
// / rolled back, gate verdict, agent dispatch). Nil disables auditing
// but the rest of the pipeline still runs.
func (e *Engine) WithAudit(a audit.Store) *Engine {
	e.audit = a
	return e
}

// WithProfitGuard wires the V22 ProfitGuard hook the retry loop calls
// before burning another Coder round on a failing patch. Nil-safe —
// passing nil leaves the engine on its historical "retry up to
// maxCoderRetries unconditionally" behaviour. The integration loop in
// cmd/orchestrator/main.go owns the adapter that turns the real
// profitguard.Guard + execution lookup into this small interface.
func (e *Engine) WithProfitGuard(h ProfitGuardHook) *Engine {
	e.profitGuard = h
	return e
}

// WithSettler wires the V22 terminal close-out hook. When set, the
// engine calls Close at the end of Run() with the derived finalStatus
// for the executionID on the request context. Nil-safe — leaving it
// unwired means terminal settlement is the caller's responsibility
// (the GraphQL resolver does it directly for Stop/Refund).
func (e *Engine) WithSettler(s ExecutionSettler) *Engine {
	e.settler = s
	return e
}

// WithLearning wires the V22 LearningHooks (repair genome + patch
// memory) into the finisher. Nil-safe: passing nil leaves learning
// disabled and the engine runs unchanged. main.go owns the store
// construction; this method just plumbs the hook through.
func (e *Engine) WithLearning(h *LearningHooks) *Engine {
	e.learning = h
	return e
}

// Learning returns the current learning hook (or nil). Exposed so
// adapter helpers in cmd/orchestrator/main.go can consult per-
// execution counters (RepairsFor) at terminal settle without
// re-creating the hook.
func (e *Engine) Learning() *LearningHooks { return e.learning }

// WithSandboxBiller wires the V22 SandboxBiller. When set, every
// workspace allocated for an execution is wrapped in a ticker that
// lands `sandbox_cost` debits on the execution row + tenant ledger
// for the lifetime of the allocation. Nil-safe — leaving it unwired
// means workspaces don't accrue sandbox cost (the unit-economics
// dashboard will under-count, but the loop still runs).
func (e *Engine) WithSandboxBiller(b *runtime.SandboxBiller) *Engine {
	e.sandboxBiller = b
	return e
}

// WithCompletionScorer wires the V22 completion scorer. After each
// gate verdict the engine appends a GateOutcome and mirrors the new
// absolute score back onto the execution row (via
// WithExecutionService) so ProfitGuard reads fresh signal on the
// very next decision. Nil-safe — without a scorer wired the gate
// loop runs unchanged and completion_score stays at zero.
func (e *Engine) WithCompletionScorer(s completion.Scorer) *Engine {
	e.completionScorer = s
	return e
}

// WithQualitySink wires the providers.QualitySink the engine notifies
// after each gate verdict. The bandit reads the resulting per-provider
// EMA on every Rerank. Nil-safe — without a sink wired, the gate loop
// only drives completion scoring.
//
// TODO(A31): provider/capability are not yet threaded through the gate
// loop, so the sink call inside recordGateOutcome currently emits an
// empty Provider (which the registry drops silently). When the
// finisher's provider-call context is restructured to surface the
// last-dispatched provider per gate, populate GateOutcome.Provider /
// Capability here and the bandit reward starts seeing real signal.
func (e *Engine) WithQualitySink(s providers.QualitySink) *Engine {
	e.qualitySink = s
	return e
}

// WithExecutionService gives the engine a back-reference to the
// execution.Service. Required by WithCompletionScorer so the engine
// can call SetCompletionScore after Scorer.Score; also consumed by
// the BeforeSandboxAllocation hook for live ExecState snapshots.
// Nil-safe — without it the scorer mirror is skipped and the
// sandbox allocation hook is bypassed.
func (e *Engine) WithExecutionService(s execution.Service) *Engine {
	e.executionService = s
	return e
}

// WithBeforeSandboxAllocation wires the V22 ProfitGuard hook the
// engine consults BEFORE allocating sandbox runtime to an
// execution. `deps` carries the Registry + Genome the bridge
// snapshot helper uses to populate SimilarBlueprintAvailable /
// SimilarRepairAvailable so the policy can fire reuse verdicts.
// Nil-safe — leaving guard nil means sandbox allocation is
// unconditional (the historical behaviour).
func (e *Engine) WithBeforeSandboxAllocation(guard profitguard.Guard, deps profitguardbridge.BridgeDeps) *Engine {
	e.profitGuardFull = guard
	e.bridgeDeps = deps
	return e
}

// WithMobileBuildHook wires the V22 BeforeMobileBuild ProfitGuard hook
// the MobileBuildGate consults before kicking gradlew / xcodebuild /
// eas build commands inside the runtime. Nil-safe: without it the
// build runs unguarded (legacy behaviour).
func (e *Engine) WithMobileBuildHook(h MobileBuildHook) *Engine {
	e.mobileBuildHook = h
	return e
}

// guardSandboxAllocation runs the BeforeSandboxAllocation
// ProfitGuard check for the active execution. Returns nil to allow
// allocation to proceed; non-nil to abort the caller. Nil-safe: when
// no Guard / execution service / executionID-on-ctx is wired, this
// returns nil and allocation proceeds (the historical behaviour).
//
// Emits a "profitguard_stop" event on a Stop/Kill/Pause verdict so
// the SSE feed surfaces the abort to the UI.
func (e *Engine) guardSandboxAllocation(ctx context.Context, projectID string) error {
	if e == nil || e.profitGuardFull == nil || e.executionService == nil {
		return nil
	}
	execID, ok := profitguardctx.ExecutionID(ctx)
	if !ok || execID == "" {
		return nil
	}
	state, err := profitguardbridge.SnapshotFor(ctx, e.executionService, execID, e.bridgeDeps, profitguardbridge.BridgeFlags{}, nil, nil)
	if err != nil {
		// Snapshot failed — fail open so workspaces still allocate.
		// BillingGuard remains the harder economic stop.
		return nil
	}
	decision, derr := e.profitGuardFull.Decide(ctx, profitguard.BeforeSandboxAllocation, state)
	if derr != nil {
		return nil
	}
	_ = e.profitGuardFull.Record(ctx, execID, profitguard.BeforeSandboxAllocation, decision, state)
	switch decision.Action {
	case profitguard.Stop, profitguard.KillBranch, profitguard.PauseForBudget:
		e.emit(projectID, domain.Event{
			ID:        newEventID(),
			Step:      StepRun,
			Status:    StatusFailed,
			Message:   "profitguard_stop point=sandbox_allocation action=" + string(decision.Action) + " reason=" + decision.Reason,
			CreatedAt: time.Now().UTC(),
		})
		return errors.New("profitguard blocked sandbox allocation: " + decision.Reason)
	}
	return nil
}

// recordGateOutcome funnels every gate verdict through the
// completion scorer + mirrors the new absolute score back onto the
// execution row. Nil-safe at every step: missing scorer, missing
// execution service, missing executionID on context, or scorer
// errors all degrade silently — completion scoring is purely
// additive signal.
func (e *Engine) recordGateOutcome(ctx context.Context, gateName domain.GateName, passed bool, issueCount int) {
	if e == nil {
		return
	}
	// Surface every gate verdict on the ctx-aware logger so the
	// diagnostics ring buffer captures the warn/error variants and the
	// log stream carries execution_id automatically. logctx.From is
	// nil-safe — when no logger is stamped on ctx the call lands on a
	// discarded-output logger.
	gateLogger := logctx.From(ctx)
	if !passed {
		gateLogger.Warn().
			Str("gate", string(gateName)).
			Int("issue_count", issueCount).
			Msg("gate verdict: failed")
	} else {
		gateLogger.Info().
			Str("gate", string(gateName)).
			Int("issue_count", issueCount).
			Msg("gate verdict: passed")
	}
	// Wow-loop event ring: ship a gate.verdict.v1 row through the
	// execution-events feed so the customer-facing executionSupportBundle
	// (core/orchestrator/internal/wowloop) can render the gate report
	// panel. Best-effort: the helper degrades silently on missing exec
	// service / execID / marshal error / RecordEvent error so a telemetry
	// hiccup never aborts the parent finisher loop.
	emitExecutionEvent(ctx, e.executionService, execution.EventGateVerdictV1, map[string]any{
		"gate":         string(gateName),
		"status":       gateStatusFromPassed(passed, issueCount),
		"issues_count": issueCount,
		"occurred_at":  time.Now().UTC().Format(time.RFC3339Nano),
	})
	// QualitySink fan-out — independent of the completion scorer leg so
	// a bandit-only deployment still gets the gate-pass EMA. A31 threads
	// the (provider, capability) of the last-served request through the
	// lastprovider tracker (written by BillingGuard.attributeCost) so
	// the bandit's per-provider EMA actually moves on gate verdicts.
	// QualityRegistry drops empty-provider rows silently — we mirror
	// that contract here and skip the call entirely when we have no
	// attribution (no execution on ctx, or no provider yet served).
	if e.qualitySink != nil {
		if execID, ok := profitguardctx.ExecutionID(ctx); ok && execID != "" {
			if rec, hit := lastprovider.Get(execID); hit && rec.Provider != "" {
				e.qualitySink.RecordGateOutcome(providers.GateOutcome{
					Provider:    rec.Provider,
					Capability:  rec.Capability,
					Passed:      passed,
					IssuesCount: issueCount,
				})
			}
		}
	}
	if e.completionScorer == nil {
		return
	}
	execID, _ := profitguardctx.ExecutionID(ctx)
	if execID == "" {
		return
	}
	outcome := completion.GateOutcome{
		Gate:           string(gateName),
		Passed:         passed,
		Issues:         issueCount,
		CoverageWeight: 1.0,
	}
	newScore, _, err := e.completionScorer.Score(ctx, execID, outcome)
	if err != nil {
		return
	}
	if e.executionService != nil {
		_ = e.executionService.SetCompletionScore(ctx, execID, newScore)
	}
}

// consumeRefinements drains user refinements queued via refineIdea
// since the last sweep and incorporates them into the engine's
// next-iteration context. Implementation today:
//
//   - For each drained refinement we emit a
//     agent.stage.action.v1 event with action="refinement_consumed"
//     so the studio chat surfaces "Incorporating your refinement…"
//     in real time.
//   - We append the message text to the project's Spec.Idea (under
//     a dedicated "User refinements" section) so the next agent
//     prompt actually reads the user's input — Planner +
//     Architect + UXer + Coder all derive their goal text from
//     Spec.Idea so the refinement lands in every downstream call
//     without per-agent plumbing.
//
// Nil-safe at every level — no execution service, no execID on ctx,
// or zero pending refinements all return cleanly. Refinement
// consumption is idempotent: DrainRefinements stamps a
// studio.refine.consumed.v1 marker so subsequent calls see the row
// as already-claimed.
func (e *Engine) consumeRefinements(ctx context.Context, projectID string) {
	if e == nil || e.executionService == nil {
		return
	}
	execID, ok := profitguardctx.ExecutionID(ctx)
	if !ok || execID == "" {
		return
	}
	refs, err := e.executionService.DrainRefinements(ctx, execID)
	if err != nil {
		lg := logctx.From(ctx)
		lg.Warn().
			Err(err).
			Str("execution_id", execID).
			Msg("finisher: drain refinements failed")
		return
	}
	if len(refs) == 0 {
		return
	}
	for _, ref := range refs {
		emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageActionV1, map[string]any{
			"stage":      "studio",
			"agent_role": "orchestrator",
			"action":     "refinement_consumed",
			"refine_id":  ref.ID,
			"message":    "Incorporating your refinement: " + truncateMsg(ref.Message, 160),
		})
	}
	// Best-effort spec append so the refinement actually lands in
	// the next agent prompt. We splice into Spec.Idea under a
	// dedicated tail section the agents read as additional intent.
	_, _ = e.projects.Update(projectID, func(p *domain.Project) {
		header := "\n\n## User refinements (live)\n"
		current := p.Spec.Idea
		// Strip the previous block if present so the section grows
		// monotonically without duplicates across multiple drains.
		if idx := indexOf(current, header); idx >= 0 {
			current = current[:idx]
		}
		var b []byte
		b = append(b, current...)
		b = append(b, header...)
		for _, ref := range refs {
			b = append(b, "- "...)
			b = append(b, ref.Message...)
			b = append(b, '\n')
		}
		p.Spec.Idea = string(b)
	})
}

// indexOf is a tiny strings.Index shim so consumeRefinements
// doesn't have to pull strings into engine.go's already-large
// import block for a single call site.
func indexOf(s, sub string) int {
	if sub == "" {
		return 0
	}
	if len(sub) > len(s) {
		return -1
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// recordAudit is the nil-safe entry point capture hooks use. Drops on
// nil store, so callers don't have to gate every call site themselves.
func (e *Engine) recordAudit(ctx context.Context, entry audit.Entry) {
	if e == nil || e.audit == nil {
		return
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	_, _ = e.audit.Record(ctx, entry)
}

// Run executes the finisher loop for a project. It first drives the
// generative pipeline (Planner → Architect → UXer → Coder, with Reviewer
// retries) and then runs the gate-based verification + repair loop. Any
// LLM or runtime error is surfaced as a structured SSE event with a
// stable ErrorCode; the function still returns normally so the HTTP
// handler closes the response cleanly.
func (e *Engine) Run(ctx context.Context, projectID string) (RunReport, error) {
	// Admission control: cap concurrent Run() calls per pod and per
	// owner so one busy user can't starve the rest of the fleet. Slot
	// is looked up by OwnerID; we read the project once here for that
	// purpose (the body re-reads later, but Get is cheap on the
	// in-process store and pgx caches the parsed plan).
	var ownerID string
	if p, err := e.projects.Get(projectID); err == nil {
		ownerID = p.OwnerID
	}
	slotRelease, slotErr := e.runSlots.acquire(ctx, ownerID)
	if slotErr != nil {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRun, Status: StatusFailed,
			Message:   fmtErr(ErrThrottled, "concurrent run limit reached, retry shortly"),
			CreatedAt: time.Now().UTC(),
		})
		return RunReport{ProjectID: projectID, Completed: false}, slotErr
	}
	defer slotRelease()

	// Distributed lock: when Redis is wired in main.go, two pods that
	// both receive a Run for the same project must not race. The lock
	// is keyed on the project and held for the worst-case run length —
	// the unlock script is token-bound so a slow holder can't release
	// a different pod's lock.
	//
	// When e.redis is nil, redisbus.Client.Lock returns acquired=true
	// with a no-op unlock so single-pod behaviour is preserved.
	unlock, acquired, _ := e.redis.Lock(ctx, "ironflyer:run:"+projectID, 30*time.Minute)
	if !acquired {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRun, Status: StatusFailed,
			Message:   "run already in progress on another instance",
			CreatedAt: time.Now().UTC(),
		})
		return RunReport{ProjectID: projectID, Completed: false}, errors.New("run already in progress")
	}
	defer unlock()

	// Adaptive caps: grow the per-run resource budgets when the project
	// is large (many stories, many subprojects, many files). MVPs keep
	// the existing tight defaults; serious projects get headroom.
	if p, err := e.projects.Get(projectID); err == nil {
		iter, retries, bytesCap, fileCap := adaptiveCaps(&p)
		e.maxIterations = iter
		e.maxCoderRetries = retries
		e.maxPatchBytes = bytesCap
		e.maxFilesPerPatch = fileCap
	}

	report := RunReport{ProjectID: projectID, StartedAt: time.Now().UTC()}

	defer func() {
		// Defence-in-depth: a panic anywhere in the loop becomes a structured
		// failure event rather than a 500 with no breadcrumbs in the SSE log.
		if r := recover(); r != nil {
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRun, Status: StatusFailed,
				Message: fmtErr(ErrCodeGateUnrecoverable, "panic in finisher loop"),
				CreatedAt: time.Now().UTC(),
			})
		}
	}()

	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepRun, Status: StatusRunning,
		Message: "run_started", CreatedAt: time.Now().UTC(),
	})

	bearer := bearerFromCtx(ctx)
	// Resolve a workspace for this project once per Run — if the runtime is
	// configured and the user has a running workspace bound to projectID we'll
	// surface it through GateEnv so build/test gates can execute commands.
	// When no workspace exists yet, auto-provision one so a freshly described
	// idea (describeIdea path) actually gets a sandbox to write into instead
	// of running every gate against an empty in-memory project. The user's
	// bearer is required either way because the runtime's owner check keys
	// off it; without a bearer we still degrade to the in-memory path.
	var workspaceID string
	if e.runtime.Enabled() && bearer != "" {
		lg := logctx.From(ctx)
		if ws, err := e.runtime.FindWorkspaceForProject(ctx, bearer, projectID); err == nil {
			workspaceID = ws.ID
			lg.Info().Str("workspace_id", workspaceID).Msg("engine: reused existing workspace")
		} else {
			lg.Info().Err(err).Msg("engine: no existing workspace; provisioning a new one")
			provCtx, provCancel := context.WithTimeout(ctx, 30*time.Second)
			ws, createErr := e.runtime.CreateWorkspace(provCtx, bearer, projectID, "")
			provCancel()
			if createErr != nil {
				lg.Warn().Err(createErr).Msg("engine: CreateWorkspace failed; pipeline will run in-memory only (no build/test/deploy)")
			} else {
				workspaceID = ws.ID
				lg.Info().Str("workspace_id", workspaceID).Msg("engine: provisioned new workspace")
			}
		}
		// Ask the runtime to bind a preview port to the workspace. This is
		// idempotent and 0 internal-port lets the runtime pick the dev
		// server's default (vite=5173, next=3000, ...). Without this the
		// wow-loop bundle's previewURL stays empty even after the coder
		// has finished writing files — the studio iframe has nothing to
		// load. Best-effort: a failure is logged but does not abort the
		// run (a stale preview from a previous run still works, and the
		// deploy gate will surface a structured error if no URL ever
		// lands).
		if workspaceID != "" {
			previewCtx, previewCancel := context.WithTimeout(ctx, 10*time.Second)
			binding, previewErr := e.runtime.AllocatePreview(previewCtx, bearer, workspaceID, 0)
			previewCancel()
			if previewErr != nil {
				lg.Warn().Err(previewErr).Str("workspace_id", workspaceID).Msg("engine: AllocatePreview failed; iframe will stay empty until next run")
			} else if binding.URL != "" {
				lg.Info().Str("workspace_id", workspaceID).Str("preview_url", binding.URL).Int("port", binding.ExternalPort).Msg("engine: preview URL allocated")
			}
		}
	}
	// Stamp the resolved workspace ID onto the context so any agent Run
	// triggered downstream (built-in tools like generate_image) can target
	// the caller's sandbox without plumbing it through every helper.
	ctx = withWorkspaceID(ctx, workspaceID)

	// A63 — persist the resolved workspace onto the executions row so the
	// wow-loop builder (and GraphQL Execution.workspaceID) can resolve
	// the live sandbox without proxying through projectID. Best-effort:
	// a failing SetWorkspaceID is warned but never aborts the run — the
	// wow-loop adapter falls back to ProjectID for backward compat when
	// the column is empty.
	if workspaceID != "" && e.executionService != nil {
		if execID, ok := profitguardctx.ExecutionID(ctx); ok && execID != "" {
			if err := e.executionService.SetWorkspaceID(ctx, execID, workspaceID); err != nil {
				lg := logctx.From(ctx)
				lg.Warn().
					Err(err).
					Str("execution_id", execID).
					Str("workspace_id", workspaceID).
					Msg("set workspace id failed")
			}
		}
	}

	// V22 sandbox billing — wrap the workspace lifetime in a ticker
	// that lands `sandbox_cost` debits on the active execution. Stops
	// at end-of-Run, flushing one final partial-interval tick. No-op
	// when there is no execution on ctx, no workspace, or no biller
	// configured.
	//
	// V22 BeforeSandboxAllocation ProfitGuard hook runs FIRST: a
	// Stop/Kill/Pause verdict aborts the run without ever spinning up
	// the sandbox ticker. Nil-safe — without WithBeforeSandboxAllocation
	// the check is a no-op and allocation proceeds unconditionally.
	if workspaceID != "" {
		if execID, ok := profitguardctx.ExecutionID(ctx); ok {
			if err := e.guardSandboxAllocation(ctx, projectID); err != nil {
				report.FinishedAt = time.Now().UTC()
				return report, err
			}
			stopBilling := e.sandboxBiller.Track(ctx, execID, workspaceID)
			defer stopBilling()
		}
	}

	// Phase A: generative pipeline. The pipeline mutates the project Spec +
	// Files in-place so the gate phase that follows sees the freshly drafted
	// plan / screen map / source. A pipeline error is logged via SSE but does
	// not abort the run — we still want partial gate reports.
	if err := e.runPipeline(ctx, projectID, workspaceID, bearer, &report); err != nil {
		if ctx.Err() != nil {
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRun, Status: StatusFailed,
				Message: fmtErr(ErrCodeContextCancelled, err.Error()),
				CreatedAt: time.Now().UTC(),
			})
			report.FinishedAt = time.Now().UTC()
			return report, nil
		}
		// Budget exhaustion is terminal — iterating gates without a
		// plan just produces noise (4× iterations × ~7 gates of
		// `gate verdict: failed` events because the gates have
		// nothing to verify). Surface ProfitGuard-style 402 semantics
		// and bail out of the iteration loop.
		//
		// We use errors.Is rather than string match so wrapped errors
		// from the agent registry / billing guard still trip the
		// short-circuit.
		if errors.Is(err, budgetpkg.ErrOverBudget) {
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRun, Status: StatusFailed,
				Message: fmtErr(ErrCodeBudgetExhausted, "execution halted: wallet budget exhausted; top up to continue"),
				CreatedAt: time.Now().UTC(),
			})
			if e.settler != nil {
				if execID, ok := profitguardctx.ExecutionID(ctx); ok && execID != "" {
					_ = e.settler.SettleFailed(ctx, execID, "budget_exhausted")
				}
			}
			report.FinishedAt = time.Now().UTC()
			return report, nil
		}
		// Other pipeline errors: structured event already emitted
		// from inside runPipeline; fall through so the gate loop can
		// still produce a partial report.
	}

	for i := 0; i < e.maxIterations; i++ {
		report.Iterations = i + 1
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepLoopIteration, Status: StatusRunning,
			Message: "iteration " + itoaPositive(i+1), CreatedAt: time.Now().UTC(),
		})
		allPassed := true

		for _, gate := range e.gates {
			if err := ctx.Err(); err != nil {
				e.emit(projectID, domain.Event{
					ID: newEventID(), Step: StepRun, Status: StatusFailed,
					Message: fmtErr(ErrCodeContextCancelled, err.Error()),
					CreatedAt: time.Now().UTC(),
				})
				report.FinishedAt = time.Now().UTC()
				return report, nil
			}

			// A55 refinement consumption — at the start of each gate
			// iteration we drain any user refinements queued via
			// refineIdea since the last sweep. Each drained
			// refinement emits an ack event so the studio chat shows
			// "Incorporating your refinement..." live; the message
			// itself is parked on the project's spec memory so the
			// next agent prompt actually reads it.
			e.consumeRefinements(ctx, projectID)

			p, err := e.projects.Get(projectID)
			if err != nil {
				return report, err
			}

			gateAgent := gateToAgent(gate.Name())
			gateStartedAt := time.Now().UTC()
			// A55 agent reasoning — stage started.
			emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageStartedV1, map[string]any{
				"stage":      string(gate.Name()),
				"agent_role": gateAgent,
				"iteration":  i + 1,
				"message":    "Starting " + string(gate.Name()) + " review",
				"started_at": gateStartedAt.Format(time.RFC3339Nano),
			})

			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepGate, Gate: gate.Name(),
				Message: "gate_started", Status: StatusRunning, CreatedAt: time.Now().UTC(),
			})

			env := &GateEnv{
				Project:         &p,
				Runtime:         e.runtime,
				WorkspaceID:     workspaceID,
				UserBearer:      bearer,
				MobileBuildHook: e.mobileBuildHook,
				Manifest:        e.archManifest,
				Refactor:        e.refactor,
			}
			if e.budget != nil {
				if snap, err := e.budget(ctx, p.OwnerID, projectID); err == nil {
					env.Budget = snap
				}
			}
			// Project the Anti-Bloat preflight decision onto env so the
			// reuse_check gate reads the same verdict propose-time
			// produced. Ctx-attached decisions win (agent loop set them
			// explicitly); the patch engine's per-project cache is the
			// mechanical fallback.
			if d, ok := agents.PreflightDecisionFromContext(ctx); ok {
				env.Preflight = &d
			} else if e.patches != nil {
				if d, ok := e.patches.PreflightFor(projectID); ok {
					env.Preflight = &d
				}
			}
			issues := runGateInstrumented(ctx, gate, env)
			status := domain.GateStatusPassed
			if len(issues) > 0 {
				status = domain.GateStatusFailed
				allPassed = false
			}
			e.setGate(projectID, gate.Name(), domain.GateState{
				Name: gate.Name(), Status: status, Issues: issues, UpdatedAt: time.Now().UTC(),
			})

			// V22 completion scoring — append this gate's verdict to
			// the scorer and mirror the new absolute score back onto
			// the execution row so ProfitGuard's next Decide reads
			// fresh signal. Nil-safe.
			e.recordGateOutcome(ctx, gate.Name(), len(issues) == 0, len(issues))

			// V22 security report — project Security-gate issues onto
			// execution_events so the customer-facing
			// securityreport.FindingSource has real signal to read.
			// Nil-safe + gate-scoped to GateSecurity inside the helper.
			e.emitSecurityFindings(ctx, gate.Name(), issues)

			// Audit trail entry — one per gate verdict so the production-
			// trust moat has a verifiable line-by-line history.
			outcome := audit.OutcomeSuccess
			if len(issues) > 0 {
				outcome = audit.OutcomeFailure
			}
			e.recordAudit(ctx, audit.Entry{
				Action:    audit.ActionGateVerdict,
				Outcome:   outcome,
				UserID:    p.OwnerID,
				ProjectID: projectID,
				GateName:  string(gate.Name()),
				Summary:   "gate=" + string(gate.Name()) + " status=" + string(status) + " issues=" + itoaPositive(len(issues)),
				Attrs: map[string]any{
					"iteration": i + 1,
					"issues":    len(issues),
				},
			})

			if len(issues) == 0 {
				e.emit(projectID, domain.Event{
					ID: newEventID(), Step: StepGate, Gate: gate.Name(),
					Message: "gate_passed", Status: StatusDone, CreatedAt: time.Now().UTC(),
				})
				emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageCompletedV1, map[string]any{
					"stage":       string(gate.Name()),
					"agent_role":  gateAgent,
					"status":      "passed",
					"duration_ms": time.Since(gateStartedAt).Milliseconds(),
				})
				continue
			}

			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepGate, Gate: gate.Name(),
				Message: "gate_failed issues=" + itoaPositive(len(issues)),
				Status: StatusFailed, CreatedAt: time.Now().UTC(),
			})

			// Auto-recovery: re-prompt the Coder with the failure context and
			// re-run this gate only. On success we mark the gate repaired and
			// move on; on failure we fall through to the existing repair-agent
			// path so behaviour without recovery still applies.
			if e.tryRecoverGate(ctx, projectID, workspaceID, bearer, gate.Name(), issues, &report) {
				emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageCompletedV1, map[string]any{
					"stage":       string(gate.Name()),
					"agent_role":  gateAgent,
					"status":      "repaired",
					"duration_ms": time.Since(gateStartedAt).Milliseconds(),
				})
				allPassed = false // gate was failing this iteration; require another full sweep
				continue
			}

			// Dispatch repair agent. A gate may opt out of agent-driven
			// repair by returning the empty role — used by the Budget gate,
			// where overruns must be resolved by plan/billing changes, not
			// by another LLM call. We mark such failures blocked so the
			// loop ends gracefully.
			role := gate.RepairAgent()
			if role == "" {
				e.setGate(projectID, gate.Name(), domain.GateState{
					Name: gate.Name(), Status: domain.GateStatusBlocked,
					Issues:    issues,
					UpdatedAt: time.Now().UTC(),
				})
				emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageCompletedV1, map[string]any{
					"stage":       string(gate.Name()),
					"agent_role":  gateAgent,
					"status":      "blocked",
					"duration_ms": time.Since(gateStartedAt).Milliseconds(),
				})
				continue
			}
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: "repair", Gate: gate.Name(), Agent: string(role),
				Message: "repair_started", Status: StatusRunning, CreatedAt: time.Now().UTC(),
			})

			task := agents.Task{
				Role:        role,
				Project:     &p,
				Goal:        "Repair gate " + string(gate.Name()),
				Issues:      issues,
				UserBearer:  bearerFromCtx(ctx),
				WorkspaceID: workspaceIDFromCtx(ctx),
			}
			// A55 agent reasoning — provider call wrap. The repair
			// agent is about to hit a model; surface the action +
			// result pair so the studio chat shows "Calling X..."
			// rather than freezing for the duration.
			emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageActionV1, map[string]any{
				"stage":      string(gate.Name()),
				"agent_role": string(role),
				"action":     "model_call",
				"message":    "Calling " + string(role) + " for " + string(gate.Name()) + " repair",
			})
			res, err := e.registry.Run(ctx, task)
			if err != nil {
				emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageResultV1, map[string]any{
					"stage":      string(gate.Name()),
					"agent_role": string(role),
					"action":     "model_call",
					"success":    false,
					"error":      err.Error(),
				})
				e.emitProviderErr(projectID, "repair", role, err)
				e.setGate(projectID, gate.Name(), domain.GateState{
					Name: gate.Name(), Status: domain.GateStatusBlocked,
					Issues: append(issues, domain.Issue{
						Gate: gate.Name(), Severity: domain.SeverityError, Message: "agent error: " + err.Error(),
					}),
					UpdatedAt: time.Now().UTC(),
				})
				emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageCompletedV1, map[string]any{
					"stage":       string(gate.Name()),
					"agent_role":  gateAgent,
					"status":      "failed",
					"duration_ms": time.Since(gateStartedAt).Milliseconds(),
				})
				continue
			}
			report.AgentRuns = append(report.AgentRuns, res)
			emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageResultV1, map[string]any{
				"stage":      string(gate.Name()),
				"agent_role": string(role),
				"action":     "model_call",
				"success":    true,
				"summary":    "Repair patch drafted by " + res.Provider,
				"cost_usd":   res.CostUSD,
				"provider":   res.Provider,
			})
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: "repair", Gate: gate.Name(), Agent: string(role),
				Message: "repair_done provider=" + res.Provider, Status: StatusDone,
				CreatedAt: time.Now().UTC(),
			})
			e.setGate(projectID, gate.Name(), domain.GateState{
				Name: gate.Name(), Status: domain.GateStatusRepaired, Issues: issues,
				UpdatedAt: time.Now().UTC(),
			})
			emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageCompletedV1, map[string]any{
				"stage":       string(gate.Name()),
				"agent_role":  gateAgent,
				"status":      "repaired",
				"duration_ms": time.Since(gateStartedAt).Milliseconds(),
			})
		}

		if allPassed {
			report.Completed = true
			break
		}
	}

	// Snapshot final gate state.
	p, err := e.projects.Get(projectID)
	if err == nil {
		for _, g := range domain.AllGates() {
			report.Gates = append(report.Gates, p.Gates[g])
		}
	}
	report.FinishedAt = time.Now().UTC()

	if report.Completed {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRun, Status: StatusDone,
			Message: "run_complete", CreatedAt: time.Now().UTC(),
		})
	} else {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRun, Status: StatusFailed,
			Message: fmtErr(ErrCodeGateUnrecoverable, "gates remained unrepaired after max iterations"),
			CreatedAt: time.Now().UTC(),
		})
	}

	// V22 terminal settlement. Best-effort — wallet/ledger drift is
	// recoverable via reconciliation, but a failing settler MUST NOT
	// rewrite the run report. Only fires when an execution id is on
	// the context (i.e. the run was admitted as a paid execution).
	if e.settler != nil {
		if execID, ok := profitguardctx.ExecutionID(ctx); ok {
			if report.Completed {
				_ = e.settler.SettleSucceeded(ctx, execID)
			} else {
				_ = e.settler.SettleFailed(ctx, execID, "gates_unrepaired_after_max_iterations")
			}
		}
	}
	// A31 — release the last-provider slot at terminal close so the
	// in-process tracker doesn't lean on FIFO eviction for happy-path
	// executions. Best-effort: an executionless Run() leaves nothing
	// to forget. Runs aborted earlier (sandbox-guard refusal, ctx
	// cancellation) fall back to FIFO eviction inside the tracker.
	if execID, ok := profitguardctx.ExecutionID(ctx); ok && execID != "" {
		lastprovider.Forget(execID)
	}
	return report, nil
}

func (e *Engine) setGate(projectID string, name domain.GateName, gs domain.GateState) {
	_, _ = e.projects.Update(projectID, func(p *domain.Project) {
		if p.Gates == nil {
			p.Gates = make(map[domain.GateName]domain.GateState)
		}
		p.Gates[name] = gs
	})
}

// Subscribe returns a channel that receives events for a project. Caller must
// call the returned unsubscribe func.
//
// When Redis is wired, the subscriber also receives events emitted by other
// pods through the `ironflyer:events:<projectID>` pub/sub channel. The
// returned channel merges the in-process feed with the Redis feed; events
// are de-duplicated by ID so a subscriber that happens to be on the same
// pod that produced the event sees it exactly once.
func (e *Engine) Subscribe(projectID string) (<-chan domain.Event, func()) {
	local := make(chan domain.Event, 32)
	e.mu.Lock()
	e.subscribers[projectID] = append(e.subscribers[projectID], local)
	e.mu.Unlock()

	localUnsub := func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		subs := e.subscribers[projectID]
		for i, s := range subs {
			if s == local {
				e.subscribers[projectID] = append(subs[:i], subs[i+1:]...)
				close(local)
				return
			}
		}
	}

	// Single-pod path: no Redis AND no bus — just hand back the local
	// channel and the local unsubscribe. Behaviour is identical to the
	// original.
	if e.redis == nil && e.bus == nil {
		return local, localUnsub
	}

	subCtx, subCancel := context.WithCancel(context.Background())
	var redisCh <-chan string
	redisCancel := func() {}
	if e.redis != nil {
		ch, cancel, err := e.redis.Subscribe(subCtx, eventsChannel(projectID))
		if err != nil {
			// Redis hiccup at subscribe time — degrade to local-only
			// rather than fail the SSE handler. Cross-pod events will
			// be missed for this subscriber's lifetime; SSE reconnects
			// naturally pick up a fresh attempt.
			subCancel()
			return local, localUnsub
		}
		redisCh = ch
		redisCancel = cancel
	}
	var busCh <-chan []byte
	busCancel := func() {}
	if e.bus != nil {
		ch, cancel, err := e.bus.Subscribe(subCtx, "finisher.run:"+projectID)
		if err == nil {
			busCh = ch
			busCancel = cancel
		}
	}

	out := make(chan domain.Event, 64)
	// FIFO de-dupe ring: keeps the last 256 IDs we've forwarded so an
	// event that arrives via both the in-process fan-out AND the Redis
	// mirror is delivered once.
	const dedupeCap = 256
	seen := make(map[string]struct{}, dedupeCap)
	order := make([]string, 0, dedupeCap)
	var dmu sync.Mutex
	markSeen := func(id string) bool {
		if id == "" {
			return false // can't de-dupe an unidentified event; let it through
		}
		dmu.Lock()
		defer dmu.Unlock()
		if _, ok := seen[id]; ok {
			return true
		}
		if len(order) >= dedupeCap {
			evict := order[0]
			order = order[1:]
			delete(seen, evict)
		}
		seen[id] = struct{}{}
		order = append(order, id)
		return false
	}

	var wg sync.WaitGroup
	// Always have the local goroutine; add one for each upstream.
	wg.Add(1)

	go func() {
		defer wg.Done()
		for evt := range local {
			if markSeen(evt.ID) {
				continue
			}
			select {
			case out <- evt:
			default:
			}
		}
	}()

	if redisCh != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for raw := range redisCh {
				var evt domain.Event
				if err := json.Unmarshal([]byte(raw), &evt); err != nil {
					continue
				}
				if markSeen(evt.ID) {
					continue
				}
				select {
				case out <- evt:
				default:
				}
			}
		}()
	}

	if busCh != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for raw := range busCh {
				var evt domain.Event
				if err := json.Unmarshal(raw, &evt); err != nil {
					continue
				}
				if markSeen(evt.ID) {
					continue
				}
				select {
				case out <- evt:
				default:
				}
			}
		}()
	}

	// When both upstreams are drained, close the merged channel so the
	// SSE handler's range loop exits cleanly.
	go func() {
		wg.Wait()
		close(out)
	}()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			localUnsub()    // closes `local`, which drains the local goroutine
			redisCancel()   // closes `redisCh`, which drains the redis goroutine
			busCancel()     // closes `busCh`, which drains the bus goroutine
			subCancel()
		})
	}
	return out, unsubscribe
}

func (e *Engine) emit(projectID string, evt domain.Event) {
	_, _ = e.projects.Update(projectID, func(p *domain.Project) {
		p.Events = append(p.Events, evt)
	})
	e.mu.RLock()
	subs := append([]chan domain.Event(nil), e.subscribers[projectID]...)
	e.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
			// drop if subscriber is slow; SSE will reconnect.
		}
	}
	// Mirror to Redis so subscribers on other pods see the event. The
	// publish is best-effort — a Redis hiccup must never block or fail
	// the finisher loop, and pods still see their own in-process events
	// through the local fan-out above.
	if e.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = e.redis.Publish(ctx, eventsChannel(projectID), evt)
		cancel()
	}
	// Same event mirrored on the cross-pod Multiplexer (when wired).
	// The Multiplexer handles its own local fan-out + pod-id dedup, so
	// subscribers attached via Subscribe (above) on this pod and on
	// other pods see the event exactly once.
	if e.bus != nil {
		if payload, err := json.Marshal(evt); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = e.bus.Publish(ctx, "finisher.run:"+projectID, payload)
			cancel()
		}
	}
}

var (
	eventCounter int
	eventMu      sync.Mutex
)

func newEventID() string {
	eventMu.Lock()
	defer eventMu.Unlock()
	eventCounter++
	return "evt-" + time.Now().UTC().Format("150405.000") + "-" + itoa(eventCounter)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// RunGate executes a single named gate against the project's current
// tree without going through the full finisher loop. Used by the
// GraphQL rerunGate mutation. Emits the same gate_started /
// gate_passed / gate_failed events Run emits, so dashboard
// subscribers see the rerun. The verdict is also persisted to the
// project's Gates map (so subsequent gates(projectId) queries
// reflect the new status) and recorded in the audit log.
//
// The named gate must match one of the registered DefaultGates; an
// unknown name returns an error without touching project state.
func (e *Engine) RunGate(ctx context.Context, projectID string, gateName string) (domain.GateState, error) {
	var gate Gate
	want := domain.GateName(gateName)
	for _, g := range e.gates {
		if g.Name() == want {
			gate = g
			break
		}
	}
	if gate == nil {
		return domain.GateState{}, errors.New("unknown gate: " + gateName)
	}
	p, err := e.projects.Get(projectID)
	if err != nil {
		return domain.GateState{}, err
	}

	bearer := bearerFromCtx(ctx)
	var workspaceID string
	if e.runtime.Enabled() && bearer != "" {
		if ws, err := e.runtime.FindWorkspaceForProject(ctx, bearer, projectID); err == nil {
			workspaceID = ws.ID
		}
	}

	// V22 sandbox billing — single-gate runs also accrue sandbox cost
	// against the active execution. Same no-op semantics as Run(). The
	// BeforeSandboxAllocation hook runs first so a Stop/Kill/Pause
	// verdict aborts the rerun cleanly without ever spinning up the
	// sandbox ticker.
	if workspaceID != "" {
		if execID, ok := profitguardctx.ExecutionID(ctx); ok {
			if err := e.guardSandboxAllocation(ctx, projectID); err != nil {
				return domain.GateState{}, err
			}
			stopBilling := e.sandboxBiller.Track(ctx, execID, workspaceID)
			defer stopBilling()
		}
	}

	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepGate, Gate: gate.Name(),
		Message: "gate_started", Status: StatusRunning, CreatedAt: time.Now().UTC(),
	})

	env := &GateEnv{
		Project:     &p,
		Runtime:     e.runtime,
		WorkspaceID: workspaceID,
		UserBearer:  bearer,
		Manifest:    e.archManifest,
		Refactor:    e.refactor,
	}
	if e.budget != nil {
		if snap, err := e.budget(ctx, p.OwnerID, projectID); err == nil {
			env.Budget = snap
		}
	}
	// Anti-Bloat preflight projection (mirrors the Run loop). Ctx wins
	// over the per-project cache so the agent loop can override an
	// older mechanical decision for a single rerun.
	if d, ok := agents.PreflightDecisionFromContext(ctx); ok {
		env.Preflight = &d
	} else if e.patches != nil {
		if d, ok := e.patches.PreflightFor(projectID); ok {
			env.Preflight = &d
		}
	}
	issues := runGateInstrumented(ctx, gate, env)
	status := domain.GateStatusPassed
	if len(issues) > 0 {
		status = domain.GateStatusFailed
	}
	gs := domain.GateState{
		Name: gate.Name(), Status: status, Issues: issues, UpdatedAt: time.Now().UTC(),
	}
	e.setGate(projectID, gate.Name(), gs)

	// V22 completion scoring — single-gate reruns also append a
	// GateOutcome so a rerun that flips a gate from failed→passed
	// promotes the absolute completion_score immediately.
	e.recordGateOutcome(ctx, gate.Name(), len(issues) == 0, len(issues))

	// V22 security report — mirror the rerun's issues into
	// execution_events so the per-execution security report reflects
	// the freshest verdict.
	e.emitSecurityFindings(ctx, gate.Name(), issues)

	outcome := audit.OutcomeSuccess
	if len(issues) > 0 {
		outcome = audit.OutcomeFailure
	}
	e.recordAudit(ctx, audit.Entry{
		Action:    audit.ActionGateVerdict,
		Outcome:   outcome,
		UserID:    p.OwnerID,
		ProjectID: projectID,
		GateName:  string(gate.Name()),
		Summary: "gate=" + string(gate.Name()) +
			" status=" + string(status) + " issues=" + itoaPositive(len(issues)),
		Attrs: map[string]any{
			"issues": len(issues),
			"rerun":  true,
		},
	})

	if len(issues) == 0 {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepGate, Gate: gate.Name(),
			Message: "gate_passed", Status: StatusDone, CreatedAt: time.Now().UTC(),
		})
	} else {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepGate, Gate: gate.Name(),
			Message: "gate_failed issues=" + itoaPositive(len(issues)),
			Status:  StatusFailed, CreatedAt: time.Now().UTC(),
		})
	}
	return gs, nil
}
