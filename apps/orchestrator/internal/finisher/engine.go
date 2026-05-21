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
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/runtime"
	"ironflyer/apps/orchestrator/internal/store"
)

// ctxKey is unexported so callers must use WithBearer / bearerFromCtx.
type ctxKey struct{ name string }

var bearerKey = ctxKey{"bearer"}

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

const defaultMaxIterations = 4

type RunReport struct {
	ProjectID   string             `json:"projectId"`
	Iterations  int                `json:"iterations"`
	Gates       []domain.GateState `json:"gates"`
	Completed   bool               `json:"completed"`
	StartedAt   time.Time          `json:"startedAt"`
	FinishedAt  time.Time          `json:"finishedAt"`
	AgentRuns   []agents.Result    `json:"agentRuns,omitempty"`
}

type Engine struct {
	mu            sync.RWMutex
	projects      store.Store
	registry      *agents.Registry
	patches       *patch.Engine
	runtime       *runtime.Client
	gates         []Gate
	maxIterations int
	subscribers   map[string][]chan domain.Event
}

func NewEngine(projects store.Store, registry *agents.Registry, patches *patch.Engine) *Engine {
	return &Engine{
		projects:      projects,
		registry:      registry,
		patches:       patches,
		gates:         DefaultGates(),
		maxIterations: defaultMaxIterations,
		subscribers:   make(map[string][]chan domain.Event),
	}
}

// WithRuntime attaches a workspace-runtime client so build/test gates can
// execute commands inside the user's sandbox. Returns the engine for chained
// configuration during startup.
func (e *Engine) WithRuntime(c *runtime.Client) *Engine {
	e.runtime = c
	return e
}

// Run executes the finisher loop for a project.
func (e *Engine) Run(ctx context.Context, projectID string) (RunReport, error) {
	report := RunReport{ProjectID: projectID, StartedAt: time.Now().UTC()}

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

	for i := 0; i < e.maxIterations; i++ {
		report.Iterations = i + 1
		allPassed := true

		for _, gate := range e.gates {
			p, err := e.projects.Get(projectID)
			if err != nil {
				return report, err
			}

			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: "gate", Gate: gate.Name(),
				Message: "checking gate", Status: "running", CreatedAt: time.Now().UTC(),
			})

			env := &GateEnv{
				Project:     &p,
				Runtime:     e.runtime,
				WorkspaceID: workspaceID,
				UserBearer:  bearer,
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

			if len(issues) == 0 {
				e.emit(projectID, domain.Event{
					ID: newEventID(), Step: "gate", Gate: gate.Name(),
					Message: "passed", Status: "done", CreatedAt: time.Now().UTC(),
				})
				continue
			}

			// Dispatch repair agent.
			role := gate.RepairAgent()
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: "repair", Gate: gate.Name(), Agent: string(role),
				Message: "dispatching repair agent", Status: "running", CreatedAt: time.Now().UTC(),
			})

			task := agents.Task{
				Role:    role,
				Project: &p,
				Goal:    "Repair gate " + string(gate.Name()),
				Issues:  issues,
			}
			res, err := e.registry.Run(ctx, task)
			if err != nil {
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
				Message: "agent returned output (mock)", Status: "done", CreatedAt: time.Now().UTC(),
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
