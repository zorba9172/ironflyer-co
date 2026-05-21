package workflow

import (
	"context"
	"errors"
	"time"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/store"
)

// Activities is the Temporal Activities implementation. It binds the
// stateless workflow code to the in-process services (store, registry,
// gates) of the orchestrator.
type Activities struct {
	Store    store.Store
	Registry *agents.Registry
	Gates    map[domain.GateName]finisher.Gate
}

func NewActivities(s store.Store, r *agents.Registry) *Activities {
	gates := map[domain.GateName]finisher.Gate{}
	for _, g := range finisher.DefaultGates() {
		gates[g.Name()] = g
	}
	return &Activities{Store: s, Registry: r, Gates: gates}
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
	// Temporal-driven gate checks run without a workspace handle today; the
	// gate degrades to its static fallback when Runtime is nil. Wiring a
	// runtime client through the Workflow path is a follow-up.
	issues := gate.Check(ctx, &finisher.GateEnv{Project: &p})
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
