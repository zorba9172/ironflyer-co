// Package finisher is the heart of Ironflyer: the gate-driven completion
// loop that turns an idea into a finished product. It runs gates in order,
// dispatches repair agents on failure, and exits only when all gates pass
// (or max iterations / blocked).
package finisher

import (
	"context"
	"sync"
	"time"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/memory"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/runtime"
	"ironflyer/apps/orchestrator/internal/store"
)

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
	authScaffolder   AuthScaffolder
	stripeScaffolder StripeScaffolder
	domainScaffolders []DomainScaffolder
	memory           memory.Store
	audit            audit.Store
	gates            []Gate
	maxIterations    int
	maxCoderRetries  int
	maxPatchBytes    int
	maxFilesPerPatch int
	subscribers      map[string][]chan domain.Event
	plans            *planCache
	patchesCache     *patchCache
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

// WithStripeScaffolder registers the payments scaffolder. Triggers only
// when the spec mentions billing/subscription/checkout — generated apps
// without payments are unaffected. Pass DefaultStripeScaffolder{} for the
// canonical Next.js + Stripe Checkout recipe.
func (e *Engine) WithStripeScaffolder(s StripeScaffolder) *Engine {
	e.stripeScaffolder = s
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

// WithAudit registers the immutable hash-chained audit log. The engine
// writes one entry per consequential action (patch proposed / applied
// / rolled back, gate verdict, agent dispatch). Nil disables auditing
// but the rest of the pipeline still runs.
func (e *Engine) WithAudit(a audit.Store) *Engine {
	e.audit = a
	return e
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
	var workspaceID string
	if e.runtime.Enabled() && bearer != "" {
		if ws, err := e.runtime.FindWorkspaceForProject(ctx, bearer, projectID); err == nil {
			workspaceID = ws.ID
		}
	}
	// Stamp the resolved workspace ID onto the context so any agent Run
	// triggered downstream (built-in tools like generate_image) can target
	// the caller's sandbox without plumbing it through every helper.
	ctx = withWorkspaceID(ctx, workspaceID)

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
		// Already emitted a structured event from inside runPipeline.
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
			p, err := e.projects.Get(projectID)
			if err != nil {
				return report, err
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
			}
			if e.budget != nil {
				if snap, err := e.budget(ctx, p.OwnerID, projectID); err == nil {
					env.Budget = snap
				}
			}
			issues := gate.Check(ctx, env)
			status := domain.GateStatusPassed
			if len(issues) > 0 {
				status = domain.GateStatusFailed
				allPassed = false
			}
			e.setGate(projectID, gate.Name(), domain.GateState{
				Name: gate.Name(), Status: status, Issues: issues, UpdatedAt: time.Now().UTC(),
			})

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
			res, err := e.registry.Run(ctx, task)
			if err != nil {
				e.emitProviderErr(projectID, "repair", role, err)
				e.setGate(projectID, gate.Name(), domain.GateState{
					Name: gate.Name(), Status: domain.GateStatusBlocked,
					Issues: append(issues, domain.Issue{
						Gate: gate.Name(), Severity: domain.SeverityError, Message: "agent error: " + err.Error(),
					}),
					UpdatedAt: time.Now().UTC(),
				})
				continue
			}
			report.AgentRuns = append(report.AgentRuns, res)
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: "repair", Gate: gate.Name(), Agent: string(role),
				Message: "repair_done provider=" + res.Provider, Status: StatusDone,
				CreatedAt: time.Now().UTC(),
			})
			e.setGate(projectID, gate.Name(), domain.GateState{
				Name: gate.Name(), Status: domain.GateStatusRepaired, Issues: issues,
				UpdatedAt: time.Now().UTC(),
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
func (e *Engine) Subscribe(projectID string) (<-chan domain.Event, func()) {
	ch := make(chan domain.Event, 32)
	e.mu.Lock()
	e.subscribers[projectID] = append(e.subscribers[projectID], ch)
	e.mu.Unlock()
	return ch, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		subs := e.subscribers[projectID]
		for i, s := range subs {
			if s == ch {
				e.subscribers[projectID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				return
			}
		}
	}
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
