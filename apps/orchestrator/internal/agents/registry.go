// Package agents defines typed agent contracts. Streaming-first: every Run
// returns a Delta channel that the orchestrator and the HTTP SSE endpoint can
// fan out.
package agents

import (
	"context"
	"errors"
	"sync"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/providers"
)

type Role string

const (
	RolePlanner   Role = "planner"
	RoleUXer      Role = "uxer"
	RoleArchitect Role = "architect"
	RoleCoder     Role = "coder"
	RoleReviewer  Role = "reviewer"
	RoleTester    Role = "tester"
	RoleSecurity  Role = "security"
	RoleDeployer  Role = "deployer"
)

type Task struct {
	Role    Role
	Project *domain.Project
	Goal    string
	Issues  []domain.Issue
	Hint    string
}

type Result struct {
	Role     Role
	Output   string
	Thinking string
	Provider string
	Tokens   int
	CostUSD  float64
}

type Agent struct {
	Role           Role
	System         string
	Capabilities   []providers.Capability
	EnableThinking bool
}

type Registry struct {
	mu     sync.RWMutex
	router *providers.Router
	agents map[Role]Agent
}

func NewRegistry(r *providers.Router) *Registry {
	return &Registry{router: r, agents: make(map[Role]Agent)}
}

func (r *Registry) Register(a Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[a.Role] = a
}

func (r *Registry) Get(role Role) (Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[role]
	return a, ok
}

func (r *Registry) Roles() []Role {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Role, 0, len(r.agents))
	for role := range r.agents {
		out = append(out, role)
	}
	return out
}

// RunStream invokes the agent and returns a streaming Delta channel.
func (r *Registry) RunStream(ctx context.Context, task Task) (<-chan providers.Delta, error) {
	a, ok := r.Get(task.Role)
	if !ok {
		return nil, errors.New("unknown agent role: " + string(task.Role))
	}
	return r.router.CompleteStream(ctx, providers.Request{
		System:         a.System,
		Prompt:         buildPrompt(task),
		Capabilities:   a.Capabilities,
		EnableThinking: a.EnableThinking,
		ProjectContext: projectContext(task.Project),
	})
}

// Run drains the stream and returns the aggregate result.
func (r *Registry) Run(ctx context.Context, task Task) (Result, error) {
	ch, err := r.RunStream(ctx, task)
	if err != nil {
		return Result{}, err
	}
	var (
		text     []byte
		thinking []byte
		provider string
		tokens   int
		cost     float64
	)
	for d := range ch {
		switch d.Type {
		case providers.DeltaText:
			text = append(text, d.Text...)
		case providers.DeltaThinking:
			thinking = append(thinking, d.Text...)
		case providers.DeltaDone:
			provider = d.Provider
			if d.Usage != nil {
				tokens = d.Usage.InputTokens + d.Usage.OutputTokens
				cost = d.Usage.CostUSD
			}
		case providers.DeltaError:
			return Result{}, d.Err
		}
	}
	return Result{
		Role: task.Role, Output: string(text), Thinking: string(thinking),
		Provider: provider, Tokens: tokens, CostUSD: cost,
	}, nil
}

func (r *Registry) RegisterDefaults() {
	r.Register(Agent{
		Role: RolePlanner, EnableThinking: true,
		System: "You are the Ironflyer Planner. Turn ideas into product specs with user stories, acceptance criteria, and a data model sketch. Output structured Markdown.",
		Capabilities: []providers.Capability{providers.CapReasoning, providers.CapThinking, providers.CapCache},
	})
	r.Register(Agent{
		Role: RoleUXer,
		System: "You are the Ironflyer UXer. Map every user story to screens and state diagrams. Use Mermaid for diagrams.",
		Capabilities: []providers.Capability{providers.CapReasoning, providers.CapVision, providers.CapCache},
	})
	r.Register(Agent{
		Role: RoleArchitect, EnableThinking: true,
		System: "You are the Ironflyer Architect. Define services, data flow, and contracts. No dangling deps.",
		Capabilities: []providers.Capability{providers.CapReasoning, providers.CapThinking, providers.CapJSON, providers.CapCache},
	})
	r.Register(Agent{
		Role: RoleCoder,
		System: "You are the Ironflyer Coder. Produce patches, never direct edits. Every route, error state, and auth wire must be implemented.",
		Capabilities: []providers.Capability{providers.CapCode, providers.CapTools, providers.CapCache},
	})
	r.Register(Agent{
		Role: RoleReviewer,
		System: "You are the Ironflyer Reviewer. Return structured JSON listing issues by severity.",
		Capabilities: []providers.Capability{providers.CapJSON, providers.CapCheap},
	})
	r.Register(Agent{
		Role: RoleTester,
		System: "You are the Ironflyer Tester. Generate unit + e2e tests covering the happy path and at least two edge cases per story.",
		Capabilities: []providers.Capability{providers.CapCode, providers.CapCache},
	})
	r.Register(Agent{
		Role: RoleSecurity, EnableThinking: true,
		System: "You are the Ironflyer Security agent. Run OWASP top-10 and secrets scan reasoning. Return blocking issues only.",
		Capabilities: []providers.Capability{providers.CapReasoning, providers.CapThinking, providers.CapJSON},
	})
	r.Register(Agent{
		Role: RoleDeployer,
		System: "You are the Ironflyer Deployer. Produce Dockerfile, env doc, healthcheck, and a runbook.",
		Capabilities: []providers.Capability{providers.CapCode, providers.CapCache},
	})
}

func buildPrompt(t Task) string {
	out := "# Goal\n" + t.Goal
	if t.Hint != "" {
		out += "\n\n# Hint\n" + t.Hint
	}
	if len(t.Issues) > 0 {
		out += "\n\n# Issues to repair\n"
		for _, iss := range t.Issues {
			line := "- [" + string(iss.Severity) + "] " + iss.Message
			if iss.Path != "" {
				line += " (" + iss.Path + ")"
			}
			out += line + "\n"
		}
	}
	return out
}

// projectContext builds the cacheable per-project block that gets reused
// across many agent calls. Anthropic prompt cache amortizes this cost.
func projectContext(p *domain.Project) string {
	if p == nil {
		return ""
	}
	out := "# Project\n"
	out += "Name: " + p.Name + "\n"
	out += "Description: " + p.Description + "\n"
	if p.Spec.Idea != "" {
		out += "Idea: " + p.Spec.Idea + "\n"
	}
	if p.Spec.Stack.Frontend != "" || p.Spec.Stack.Backend != "" {
		out += "Stack: frontend=" + p.Spec.Stack.Frontend + ", backend=" + p.Spec.Stack.Backend + "\n"
	}
	if len(p.Files) > 0 {
		out += "\n## Files\n"
		for _, f := range p.Files {
			out += "- " + f.Path + "\n"
		}
	}
	return out
}
