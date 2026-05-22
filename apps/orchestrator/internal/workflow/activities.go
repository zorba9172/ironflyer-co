package workflow

import (
	"context"
	"errors"
	"time"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/runtime"
	"ironflyer/apps/orchestrator/internal/store"
)

// Activities is the Temporal Activities implementation. It binds the
// stateless workflow code to the in-process services (store, registry,
// gates, runtime) of the orchestrator.
type Activities struct {
	Store    store.Store
	Registry *agents.Registry
	Gates    map[domain.GateName]finisher.Gate
	// Runtime is the workspace sandbox client. When non-nil and enabled the
	// gate Activities surface a runtime-bound GateEnv so build/test/lint
	// gates can execute real commands inside the user's workspace; when nil
	// the gates degrade to their static fallbacks.
	Runtime *runtime.Client
}

// NewActivities builds the Activities with the default gate set wired up.
// Callers that need runtime-backed gate execution should also call
// WithRuntime — main.go is the canonical injection site.
//
// TODO(main): wire NewActivities(store, registry).WithRuntime(runtimeClient)
// at orchestrator startup so the Temporal worker matches the in-process
// finisher.Engine's runtime behaviour.
func NewActivities(s store.Store, r *agents.Registry) *Activities {
	gates := map[domain.GateName]finisher.Gate{}
	for _, g := range finisher.DefaultGates() {
		gates[g.Name()] = g
	}
	return &Activities{Store: s, Registry: r, Gates: gates}
}

// WithRuntime attaches a workspace-runtime client. Returns the receiver for
// chaining at startup.
func (a *Activities) WithRuntime(c *runtime.Client) *Activities {
	a.Runtime = c
	return a
}

func (a *Activities) CheckGate(ctx context.Context, in CheckGateInput) (CheckGateResult, error) {
	gate, ok := a.Gates[in.Gate]
	if !ok {
		return CheckGateResult{}, errors.New("unknown gate: " + string(in.Gate))
	}
	p, err := a.Store.Get(in.ProjectID)
	if err != nil {
		return CheckGateResult{}, err
	}
	// Pass the runtime through so gates that can hit a workspace will when
	// one is bound. CheckGateInput does not yet carry a bearer or workspace
	// handle, so HasRuntime() stays false until the Workflow contract is
	// extended; the gate degrades to its static fallback in that case. The
	// Runtime field itself being non-nil is harmless — GateEnv.HasRuntime
	// guards on WorkspaceID too.
	// TODO(workflow): thread UserBearer + WorkspaceID through CheckGateInput
	// so the Temporal worker can resolve a workspace per project and run
	// the runtime-backed gates exactly like the in-process engine does.
	issues := gate.Check(ctx, &finisher.GateEnv{
		Project: &p,
		Runtime: a.Runtime,
	})
	status := domain.GateStatusPassed
	if len(issues) > 0 {
		status = domain.GateStatusFailed
	}
	// Persist the gate state.
	_, _ = a.Store.Update(in.ProjectID, func(proj *domain.Project) {
		if proj.Gates == nil {
			proj.Gates = make(map[domain.GateName]domain.GateState)
		}
		proj.Gates[in.Gate] = domain.GateState{
			Name: in.Gate, Status: status, Issues: issues, UpdatedAt: time.Now().UTC(),
		}
	})
	return CheckGateResult{
		Gate: in.Gate, Status: status, Issues: issues,
		RepairAgent: string(gate.RepairAgent()),
	}, nil
}

func (a *Activities) RunAgent(ctx context.Context, in RunAgentInput) (RunAgentResult, error) {
	p, err := a.Store.Get(in.ProjectID)
	if err != nil {
		return RunAgentResult{}, err
	}
	res, err := a.Registry.Run(ctx, agents.Task{
		Role:    agents.Role(in.Role),
		Project: &p,
		Goal:    "Repair gate " + string(in.Gate),
		Issues:  in.Issues,
	})
	if err != nil {
		return RunAgentResult{}, err
	}
	return RunAgentResult{
		Output:   res.Output,
		Provider: res.Provider,
		Tokens:   res.Tokens,
		CostUSD:  res.CostUSD,
	}, nil
}
