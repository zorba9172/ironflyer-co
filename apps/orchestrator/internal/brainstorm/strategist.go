// Package brainstorm decides *how* to attack a task before any code runs.
//
// The Strategist classifies an incoming task into one of four modes:
//
//   direct     — one agent, one shot. Default for unambiguous tasks.
//   brainstorm — fan out to N agents, score, synthesize the best output.
//   debate     — alternating rebuttals between two strong models; judge picks.
//   research   — agent gets tool_use first (web_search, code_search) before
//                producing the answer.
//
// The aim is always: shortest path to the goal. Brainstorm/debate are
// expensive — they're reserved for ambiguous specs and high-stakes
// architectural calls.
package brainstorm

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/providers"
)

type Mode string

const (
	ModeDirect     Mode = "direct"
	ModeBrainstorm Mode = "brainstorm"
	ModeDebate     Mode = "debate"
	ModeResearch   Mode = "research"
)

// Plan is the Strategist's decision for a single task.
type Plan struct {
	Mode       Mode          `json:"mode"`
	Roles      []agents.Role `json:"roles"`
	Rounds     int           `json:"rounds,omitempty"`
	Goal       string        `json:"goal"`
	Reason     string        `json:"reason"`
}

// Strategist is heuristic-first. The same rules can later be swapped for a
// small LLM classifier (intent model via ONNX → very cheap).
type Strategist struct{}

func NewStrategist() *Strategist { return &Strategist{} }

// Decide classifies a task into a brainstorm Plan.
func (s *Strategist) Decide(task agents.Task) Plan {
	goal := strings.ToLower(task.Goal)
	criticalIssues := 0
	for _, iss := range task.Issues {
		if iss.Severity == domain.SeverityCritical || iss.Severity == domain.SeverityError {
			criticalIssues++
		}
	}

	switch {
	case strings.Contains(goal, "architect") || strings.Contains(goal, "design") || strings.Contains(goal, "choose stack"):
		return Plan{
			Mode:   ModeBrainstorm,
			Roles:  []agents.Role{agents.RoleArchitect, agents.RolePlanner, agents.RoleReviewer},
			Goal:   task.Goal,
			Reason: "architectural decision benefits from multiple perspectives",
		}
	case strings.Contains(goal, "compare") || strings.Contains(goal, "trade-off") || strings.Contains(goal, "should we"):
		return Plan{
			Mode:   ModeDebate,
			Roles:  []agents.Role{agents.RolePlanner, agents.RoleArchitect},
			Rounds: 2,
			Goal:   task.Goal,
			Reason: "trade-off question — debate produces clearer rationale",
		}
	case strings.Contains(goal, "research") || strings.Contains(goal, "what does") || strings.Contains(goal, "find out"):
		return Plan{
			Mode:   ModeResearch,
			Roles:  []agents.Role{agents.RolePlanner},
			Goal:   task.Goal,
			Reason: "needs external lookup before generation",
		}
	case criticalIssues >= 2:
		return Plan{
			Mode:   ModeBrainstorm,
			Roles:  []agents.Role{task.Role, agents.RoleReviewer, agents.RoleArchitect},
			Goal:   task.Goal,
			Reason: "multiple critical issues — synthesize repair from several angles",
		}
	default:
		role := task.Role
		if role == "" {
			role = agents.RolePlanner
		}
		return Plan{
			Mode:   ModeDirect,
			Roles:  []agents.Role{role},
			Goal:   task.Goal,
			Reason: "single agent, single shot — shortest path to goal",
		}
	}
}

// Outcome is the synthesized result of a brainstorm/debate/research run.
type Outcome struct {
	Mode       Mode               `json:"mode"`
	Winner     agents.Role        `json:"winner,omitempty"`
	Synthesis  string             `json:"synthesis"`
	Proposals  []ScoredProposal   `json:"proposals,omitempty"`
	StartedAt  time.Time          `json:"startedAt"`
	FinishedAt time.Time          `json:"finishedAt"`
	TotalCost  float64            `json:"totalCostUSD"`
}

type ScoredProposal struct {
	Role     agents.Role `json:"role"`
	Provider string      `json:"provider"`
	Output   string      `json:"output"`
	Score    int         `json:"score"`
	Tokens   int         `json:"tokens"`
	CostUSD  float64     `json:"costUSD"`
}

// Runner executes the Plan produced by the Strategist.
type Runner struct {
	Registry *agents.Registry
	Router   *providers.Router
}

func NewRunner(r *agents.Registry, router *providers.Router) *Runner {
	return &Runner{Registry: r, Router: router}
}

// Execute runs a Plan and returns the synthesized Outcome.
func (r *Runner) Execute(ctx context.Context, plan Plan, task agents.Task) (Outcome, error) {
	out := Outcome{Mode: plan.Mode, StartedAt: time.Now().UTC()}
	switch plan.Mode {
	case ModeDirect:
		return r.runDirect(ctx, plan, task, out)
	case ModeBrainstorm:
		return r.runBrainstorm(ctx, plan, task, out)
	case ModeDebate:
		return r.runDebate(ctx, plan, task, out)
	case ModeResearch:
		return r.runDirect(ctx, plan, task, out) // research mode = direct + tools (registered on agent)
	default:
		return out, errors.New("unknown mode")
	}
}

func (r *Runner) runDirect(ctx context.Context, plan Plan, task agents.Task, out Outcome) (Outcome, error) {
	if len(plan.Roles) == 0 {
		return out, errors.New("no roles in plan")
	}
	t := task
	t.Role = plan.Roles[0]
	t.Goal = plan.Goal
	res, err := r.Registry.Run(ctx, t)
	if err != nil {
		return out, err
	}
	out.Winner = res.Role
	out.Synthesis = res.Output
	out.TotalCost = res.CostUSD
	out.Proposals = []ScoredProposal{{
		Role: res.Role, Provider: res.Provider, Output: res.Output,
		Score: 100, Tokens: res.Tokens, CostUSD: res.CostUSD,
	}}
	out.FinishedAt = time.Now().UTC()
	return out, nil
}

func (r *Runner) runBrainstorm(ctx context.Context, plan Plan, task agents.Task, out Outcome) (Outcome, error) {
	// Fan out to all roles concurrently.
	var wg sync.WaitGroup
	results := make([]ScoredProposal, len(plan.Roles))
	for i, role := range plan.Roles {
		wg.Add(1)
		go func(i int, role agents.Role) {
			defer wg.Done()
			t := task
			t.Role = role
			t.Goal = plan.Goal
			res, err := r.Registry.Run(ctx, t)
			if err != nil {
				results[i] = ScoredProposal{Role: role, Output: "error: " + err.Error()}
				return
			}
			results[i] = ScoredProposal{
				Role: res.Role, Provider: res.Provider, Output: res.Output,
				Tokens: res.Tokens, CostUSD: res.CostUSD,
			}
		}(i, role)
	}
	wg.Wait()

	// Heuristic scoring: prefer longer, structured responses with bullet
	// points / code blocks. Real scorer is a Critic agent (next iteration).
	for i := range results {
		results[i].Score = heuristicScore(results[i].Output)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	out.Proposals = results
	if len(results) > 0 {
		out.Winner = results[0].Role
		out.Synthesis = results[0].Output
	}
	for _, p := range results {
		out.TotalCost += p.CostUSD
	}
	out.FinishedAt = time.Now().UTC()
	return out, nil
}

func (r *Runner) runDebate(ctx context.Context, plan Plan, task agents.Task, out Outcome) (Outcome, error) {
	if len(plan.Roles) < 2 {
		return out, errors.New("debate needs at least 2 roles")
	}
	rounds := plan.Rounds
	if rounds <= 0 {
		rounds = 2
	}
	var transcript strings.Builder
	transcript.WriteString("# Debate transcript\n")
	transcript.WriteString("Topic: " + plan.Goal + "\n\n")

	var totalCost float64
	for round := 0; round < rounds; round++ {
		for _, role := range plan.Roles {
			t := task
			t.Role = role
			t.Goal = plan.Goal
			t.Hint = transcript.String()
			res, err := r.Registry.Run(ctx, t)
			if err != nil {
				return out, err
			}
			transcript.WriteString("## " + string(role) + " (round " + itoa(round+1) + ")\n")
			transcript.WriteString(res.Output)
			transcript.WriteString("\n\n")
			totalCost += res.CostUSD
		}
	}

	// Judge: planner picks the winner.
	judgeTask := task
	judgeTask.Role = agents.RolePlanner
	judgeTask.Goal = "Read the debate transcript and produce a one-paragraph synthesis with the chosen direction."
	judgeTask.Hint = transcript.String()
	judge, err := r.Registry.Run(ctx, judgeTask)
	if err != nil {
		return out, err
	}
	out.Synthesis = judge.Output
	out.TotalCost = totalCost + judge.CostUSD
	out.FinishedAt = time.Now().UTC()
	return out, nil
}

func heuristicScore(s string) int {
	score := len(s) / 20
	score += strings.Count(s, "\n- ") * 3
	score += strings.Count(s, "```") * 5
	if strings.Contains(s, "error:") {
		score -= 100
	}
	return score
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
