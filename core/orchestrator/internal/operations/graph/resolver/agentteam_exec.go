package resolver

// Conversions + crew execution for the agentteam resolvers. Kept OUT of
// agentteam.resolver.go on purpose: gqlgen rewrites *.resolver.go on every
// regen and strips any non-resolver code, so all helpers live here where they
// survive. The thin resolver methods in agentteam.resolver.go delegate here.

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/operations/agentteam"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// maxCrewParallelism bounds the fan-out so a large crew can't open dozens of
// concurrent provider streams at once. Reuses errgroup's SetLimit rather than a
// hand-rolled worker pool.
const maxCrewParallelism = 4

// estPerMemberUSD is the conservative per-member wallet reservation. The hold is
// an admission gate (law 1: no run without budget); actual spend is debited from
// the summed Result.CostUSD after the run, the unused remainder released.
const estPerMemberUSD = 0.25

// builtinRoleAlias maps the studio's built-in roster ids (a UI convention) onto
// runnable registry roles. Custom agents carry their own BaseRole instead.
var builtinRoleAlias = map[string]agents.Role{
	"orchestrator": agents.RolePlanner,
	"coder":        agents.RoleCoder,
	"identity":     agents.RoleIntegration,
	"payments":     agents.RoleIntegration,
	"data":         agents.RoleMigrator,
	"security":     agents.RoleSecurity,
	"deployer":     agents.RoleDeployer,
	"mobile":       agents.RoleMobileCoder,
}

// --- model conversions -------------------------------------------------

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func orEmpty(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

func scheduleToModel(s *agentteam.Schedule) *model.AgentSchedule {
	if s == nil {
		return nil
	}
	return &model.AgentSchedule{
		Mode: s.Mode, Every: strPtr(s.Every), At: strPtr(s.At),
		Weekday: s.Weekday, Trigger: strPtr(s.Trigger), Enabled: s.Enabled,
	}
}

func scheduleFromInput(in *model.AgentScheduleInput) *agentteam.Schedule {
	if in == nil {
		return nil
	}
	return &agentteam.Schedule{
		Mode: in.Mode, Every: deref(in.Every), At: deref(in.At),
		Weekday: in.Weekday, Trigger: deref(in.Trigger), Enabled: in.Enabled,
	}
}

func autonomyToModel(a agentteam.Autonomy) model.AgentAutonomy {
	switch a {
	case agentteam.AutonomySuggest:
		return model.AgentAutonomySuggest
	case agentteam.AutonomyAutonomous:
		return model.AgentAutonomyAutonomous
	default:
		return model.AgentAutonomyApproval
	}
}

func customAgentToModel(a agentteam.CustomAgent) model.CustomAgent {
	return model.CustomAgent{
		ID: a.ID, Name: a.Name, Role: a.Role,
		Description: strPtr(a.Description), Instructions: strPtr(a.Instructions),
		BaseRole: strPtr(a.BaseRole), GateID: strPtr(a.GateID),
		Skills: orEmpty(a.Skills), Tools: orEmpty(a.Tools),
		Responsibilities: orEmpty(a.Responsibilities), Guardrails: orEmpty(a.Guardrails),
		Knowledge: orEmpty(a.Knowledge), Model: strPtr(a.Model),
		Autonomy: autonomyToModel(a.Autonomy), CanDelegate: a.CanDelegate,
		HandoffTo: orEmpty(a.HandoffTo), Schedule: scheduleToModel(a.Schedule),
		UpdatedAt: a.UpdatedAt,
	}
}

func customAgentFromInput(ownerID string, in model.SaveCustomAgentInput) agentteam.CustomAgent {
	a := agentteam.CustomAgent{
		ID: deref(in.ID), OwnerID: ownerID, Name: in.Name, Role: in.Role,
		Description: deref(in.Description), Instructions: deref(in.Instructions),
		BaseRole: deref(in.BaseRole), GateID: deref(in.GateID),
		Skills: in.Skills, Tools: in.Tools, Responsibilities: in.Responsibilities,
		Guardrails: in.Guardrails, Knowledge: in.Knowledge, Model: deref(in.Model),
		CanDelegate: in.CanDelegate != nil && *in.CanDelegate,
		HandoffTo:   in.HandoffTo, Schedule: scheduleFromInput(in.Schedule),
		Autonomy: agentteam.AutonomyApproval,
	}
	if in.Autonomy != nil {
		a.Autonomy = agentteam.Autonomy(*in.Autonomy)
	}
	return a
}

func crewToModel(c agentteam.Crew) model.Crew {
	return model.Crew{
		ID: c.ID, Name: c.Name, Goal: c.Goal, Process: model.CrewProcess(c.Process),
		MemberIds: orEmpty(c.MemberIDs), ManagerID: strPtr(c.ManagerID),
		Schedule: scheduleToModel(c.Schedule), UpdatedAt: c.UpdatedAt,
	}
}

func crewFromInput(ownerID string, in model.SaveCrewInput) agentteam.Crew {
	return agentteam.Crew{
		ID: deref(in.ID), OwnerID: ownerID, Name: in.Name, Goal: in.Goal,
		Process: agentteam.Process(in.Process), MemberIDs: in.MemberIds,
		ManagerID: deref(in.ManagerID), Schedule: scheduleFromInput(in.Schedule),
	}
}

// --- crew execution ----------------------------------------------------

type memberPlan struct {
	id   string
	name string
	role agents.Role
	goal string // per-member instructions, threaded into the agent prompt
}

// resolveMembers turns crew member ids into runnable plans. Custom agent ids
// resolve through the store and use their BaseRole (falling back to Coder);
// built-in roster ids map via builtinRoleAlias; a bare role name is used as-is.
// Members that resolve to no known role are dropped.
func (r *Resolver) resolveMembers(ctx context.Context, ownerID string, crew agentteam.Crew) ([]memberPlan, error) {
	custom, err := r.AgentTeam.ListAgents(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]agentteam.CustomAgent, len(custom))
	for _, a := range custom {
		byID[a.ID] = a
	}
	out := make([]memberPlan, 0, len(crew.MemberIDs))
	for _, id := range crew.MemberIDs {
		var (
			role agents.Role
			name = id
			goal string
		)
		if ca, ok := byID[id]; ok {
			name = ca.Name
			goal = ca.Instructions
			if goal == "" {
				goal = ca.Role
			}
			role = agents.Role(ca.BaseRole)
			if !r.roleRunnable(role) {
				role = agents.RoleCoder
			}
		} else if alias, ok := builtinRoleAlias[id]; ok {
			role = alias
		} else if r.roleRunnable(agents.Role(id)) {
			role = agents.Role(id)
		} else {
			continue
		}
		out = append(out, memberPlan{id: id, name: name, role: role, goal: goal})
	}
	return out, nil
}

func (r *Resolver) roleRunnable(role agents.Role) bool {
	if r.Agents == nil || role == "" {
		return false
	}
	_, ok := r.Agents.Get(role)
	return ok
}

// runCrew executes a crew against a project. Members run per the crew's process
// — parallel fans out with bounded concurrency (errgroup.SetLimit), sequential
// chains output→context, hierarchical runs the manager first then fans the rest
// out with the manager's plan as shared context. Gated by a wallet hold so no
// run starts without budget; actual cost is debited after, the rest released.
func (r *Resolver) runCrew(ctx context.Context, crewID, projectID string) (*model.CrewRunResult, error) {
	if r.AgentTeam == nil || r.Agents == nil {
		return nil, gqlNotConfigured("agent crews")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	crew, err := r.AgentTeam.GetCrew(ctx, u.ID, crewID)
	if err != nil {
		return nil, errors.New("crew not found")
	}
	project, err := r.Projects.Get(projectID)
	if err != nil {
		return nil, errors.New("project not found")
	}
	if !project.IsAccessibleBy(u.ID) {
		return nil, errUnauthenticated
	}

	plans, err := r.resolveMembers(ctx, u.ID, crew)
	if err != nil {
		return nil, err
	}
	if len(plans) == 0 {
		return nil, errors.New("crew has no runnable members")
	}

	// Admission gate (law 1): reserve a conservative hold against the wallet.
	tenant := tenantFor(u)
	holdF := estPerMemberUSD * float64(len(plans))
	holdDec := decimal.NewFromFloat(holdF)
	if r.WalletSvc != nil {
		if err := r.WalletSvc.Hold(ctx, tenant, holdDec); err != nil {
			if errors.Is(err, wallet.ErrInsufficient) {
				return nil, gqlInsufficientFunds(r.WebBaseURL)
			}
			return nil, err
		}
	}

	results := r.dispatch(ctx, crew, project, plans)

	var total float64
	for _, m := range results {
		total += m.CostUsd
	}

	// Settle: debit the actual spend (capped at the hold), release the rest.
	if r.WalletSvc != nil {
		debitF := total
		if debitF > holdF {
			debitF = holdF
		}
		if debitF > 0 {
			_ = r.WalletSvc.Debit(ctx, tenant, decimal.NewFromFloat(debitF))
		}
		if rem := holdF - debitF; rem > 0 {
			_ = r.WalletSvc.Release(ctx, tenant, decimal.NewFromFloat(rem))
		}
	}

	return &model.CrewRunResult{
		CrewID:       crew.ID,
		Process:      model.CrewProcess(crew.Process),
		Members:      results,
		TotalCostUsd: total,
	}, nil
}

// dispatch runs the plans per the crew process and returns one result per
// member, in member order.
func (r *Resolver) dispatch(ctx context.Context, crew agentteam.Crew, project domain.Project, plans []memberPlan) []model.CrewMemberResult {
	switch crew.Process {
	case agentteam.ProcessSequential:
		return r.runSequential(ctx, crew.Goal, project, plans)
	case agentteam.ProcessHierarchical:
		return r.runHierarchical(ctx, crew, project, plans)
	default:
		return r.runParallel(ctx, crew.Goal, project, plans, "")
	}
}

// runParallel fans out every member at once, bounded by errgroup.SetLimit.
// sharedContext (a manager's plan, for hierarchical crews) is threaded into each
// member's prompt when non-empty.
func (r *Resolver) runParallel(ctx context.Context, goal string, project domain.Project, plans []memberPlan, sharedContext string) []model.CrewMemberResult {
	out := make([]model.CrewMemberResult, len(plans))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxCrewParallelism)
	for i, p := range plans {
		i, p := i, p
		g.Go(func() error {
			res := r.runMember(gctx, goal, sharedContext, project, p)
			mu.Lock()
			out[i] = res
			mu.Unlock()
			return nil // member errors are captured per-result, never abort the crew
		})
	}
	_ = g.Wait()
	return out
}

// runSequential chains members: each member's output becomes the next member's
// context, so the crew reads like a pipeline.
func (r *Resolver) runSequential(ctx context.Context, goal string, project domain.Project, plans []memberPlan) []model.CrewMemberResult {
	out := make([]model.CrewMemberResult, 0, len(plans))
	prevOutput := ""
	for _, p := range plans {
		res := r.runMember(ctx, goal, prevOutput, project, p)
		if res.Error == nil {
			prevOutput = res.Output
		}
		out = append(out, res)
	}
	return out
}

// runHierarchical runs the manager first; its output becomes the shared plan the
// remaining members fan out against.
func (r *Resolver) runHierarchical(ctx context.Context, crew agentteam.Crew, project domain.Project, plans []memberPlan) []model.CrewMemberResult {
	var manager *memberPlan
	rest := make([]memberPlan, 0, len(plans))
	for i := range plans {
		if crew.ManagerID != "" && plans[i].id == crew.ManagerID {
			m := plans[i]
			manager = &m
			continue
		}
		rest = append(rest, plans[i])
	}
	if manager == nil { // no manager picked — degrade to parallel
		return r.runParallel(ctx, crew.Goal, project, plans, "")
	}
	mgrRes := r.runMember(ctx, crew.Goal, "", project, *manager)
	plan := ""
	if mgrRes.Error == nil {
		plan = mgrRes.Output
	}
	out := []model.CrewMemberResult{mgrRes}
	out = append(out, r.runParallel(ctx, crew.Goal, project, rest, plan)...)
	return out
}

// runMember invokes one agent through the registry (the metered single-agent
// path) and maps the Result onto a CrewMemberResult. Member errors are captured
// in-band so one failure doesn't sink the whole crew. Each goroutine gets its
// own project copy so the shared pointer is never mutated concurrently.
func (r *Resolver) runMember(ctx context.Context, goal, sharedContext string, project domain.Project, p memberPlan) model.CrewMemberResult {
	res := model.CrewMemberResult{AgentID: p.id, Name: p.name, Role: string(p.role)}
	prj := project
	out, err := r.Agents.Run(ctx, agents.Task{
		Role:    p.role,
		Project: &prj,
		Goal:    composeGoal(goal, p.goal),
		Hint:    p.goal,
		Context: sharedContext,
	})
	if err != nil {
		msg := err.Error()
		res.Error = &msg
		return res
	}
	res.Output = out.Output
	res.CostUsd = out.CostUSD
	if out.Provider != "" {
		res.Provider = &out.Provider
	}
	if out.Tokens > 0 {
		t := out.Tokens
		res.Tokens = &t
	}
	return res
}

func composeGoal(crewGoal, memberGoal string) string {
	crewGoal = strings.TrimSpace(crewGoal)
	memberGoal = strings.TrimSpace(memberGoal)
	switch {
	case crewGoal != "" && memberGoal != "":
		return crewGoal + "\n\nYour part: " + memberGoal
	case memberGoal != "":
		return memberGoal
	default:
		return crewGoal
	}
}
