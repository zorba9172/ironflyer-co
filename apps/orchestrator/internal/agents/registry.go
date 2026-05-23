// Package agents defines typed agent contracts. Streaming-first: every Run
// returns a Delta channel that the orchestrator and the HTTP SSE endpoint can
// fan out.
package agents

import (
	"context"
	"errors"
	"strings"
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
	// RoleCritic is the post-Coder judge: it reads the proposed patch
	// against the story + spec and returns structured "missing X, weak Y"
	// findings the Coder can fix in a single retry without paying for a
	// full Reviewer re-run. Cheap model by design.
	RoleCritic Role = "critic"
	// RoleMigrator is the schema-evolution agent: when the Coder changes
	// the project's data model the Migrator emits a reversible migration
	// patch in the project's existing migration toolchain instead of
	// letting the DB drift behind freshly generated types.
	RoleMigrator Role = "migrator"
	// RoleFigmaTranslator turns a structured Figma extract (design
	// tokens + component inventory + per-component screenshots) into a
	// Coder-shaped patch that materialises the design pixel-perfect in
	// the project's existing stack. Powers the premium Figma → code
	// tier — see internal/figma for the extraction pipeline.
	RoleFigmaTranslator Role = "figma-translator"
)

type Task struct {
	Role    Role
	Project *domain.Project
	Goal    string
	Issues  []domain.Issue
	Hint    string
	// Context is an optional pre-rendered block of retrieved code (RAG) the
	// caller wants the agent to ground its reply in. Injected into the user
	// message between the goal and the issue list. Empty string disables it.
	Context string
	// ThinkingBudget, when > 0, overrides the provider-level extended-thinking
	// budget for this specific call. Used by the orchestrator to allocate
	// more reasoning tokens to harder steps (architecture, security) and
	// less to mechanical ones (lint repair, dockerfile generation).
	ThinkingBudget int
	// UserBearer + WorkspaceID are forwarded to in-process built-in
	// tools (currently generate_image) so the handler can write into
	// the caller's runtime sandbox. Empty values are tolerated — the
	// built-in handlers fail with a readable error rather than crash.
	UserBearer  string
	WorkspaceID string
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
	mu         sync.RWMutex
	router     *providers.Router
	agents     map[Role]Agent
	mcpClients *providers.MCPClientRegistry
	// builtinTools is the slice of in-process tools (e.g. generate_image)
	// the Coder sees alongside MCP-provided tools. builtinCalls maps
	// tool name → handler; dispatch checks this map first so a built-in
	// always wins over an external MCP server advertising the same name.
	builtinTools []providers.ToolSpec
	builtinCalls map[string]BuiltinToolFunc
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

// All returns the registered agents with their full configuration. Used by
// the public /agents endpoint so SDK / VSCode / MCP clients can introspect
// the system prompt + capability tags for each role.
func (r *Registry) All() []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Agent, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a)
	}
	return out
}

// RunStream invokes the agent and returns a streaming Delta channel.
// The Coder role additionally receives the union of every configured
// MCP server's tool catalogue so a downstream provider with native
// tool-use (e.g. Anthropic) can decide to invoke them inline.
func (r *Registry) RunStream(ctx context.Context, task Task) (<-chan providers.Delta, error) {
	a, ok := r.Get(task.Role)
	if !ok {
		return nil, errors.New("unknown agent role: " + string(task.Role))
	}
	req := providers.Request{
		System:         a.System,
		Prompt:         buildPrompt(task),
		Capabilities:   a.Capabilities,
		EnableThinking: a.EnableThinking,
		ProjectContext: projectContext(task.Project),
		ThinkingBudget: task.ThinkingBudget,
	}
	if task.Role == RoleCoder {
		if tools := r.mcpToolSpecs(ctx); len(tools) > 0 {
			req.Tools = tools
			if !containsCap(req.Capabilities, providers.CapTools) {
				req.Capabilities = append(req.Capabilities, providers.CapTools)
			}
		}
	}
	return r.router.CompleteStream(ctx, req)
}

// containsCap is a local helper duplicated from the providers package
// to avoid exporting an internal predicate just so the Registry can
// guard against double-appending CapTools.
func containsCap(caps []providers.Capability, want providers.Capability) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}

// Run drains the stream and returns the aggregate result. For the
// Coder role, when the provider emits tool_use deltas we collect
// them, dispatch each via the attached MCP client registry, then
// re-invoke the provider ONCE with a "## Tool results" continuation
// appended to the original prompt. The loop is capped at
// maxToolRounds rounds so a misbehaving model that keeps calling
// tools can't pin a single Run.
func (r *Registry) Run(ctx context.Context, task Task) (Result, error) {
	a, ok := r.Get(task.Role)
	if !ok {
		return Result{}, errors.New("unknown agent role: " + string(task.Role))
	}

	const maxToolRounds = 2

	prompt := buildPrompt(task)
	projectCtx := projectContext(task.Project)

	// Build the request once; the tool-loop rewrites Prompt between
	// rounds. CapTools is only attached for the Coder when either the
	// MCP registry or the built-in tools produced a non-empty
	// catalogue. `toolsEnabled` controls whether we dispatch tool_use
	// deltas (still named `mcpEnabled` in the closure below for
	// historical reasons — it now covers built-ins too).
	mcpEnabled := false
	var toolSpecs []providers.ToolSpec
	if task.Role == RoleCoder {
		toolSpecs = r.mcpToolSpecs(ctx)
		mcpEnabled = len(toolSpecs) > 0
	}

	var (
		text     []byte
		thinking []byte
		provider string
		tokens   int
		cost     float64
	)

	for round := 0; round < maxToolRounds+1; round++ {
		req := providers.Request{
			System:         a.System,
			Prompt:         prompt,
			Capabilities:   a.Capabilities,
			EnableThinking: a.EnableThinking,
			ProjectContext: projectCtx,
			ThinkingBudget: task.ThinkingBudget,
		}
		if mcpEnabled {
			req.Tools = toolSpecs
			if !containsCap(req.Capabilities, providers.CapTools) {
				req.Capabilities = append(req.Capabilities, providers.CapTools)
			}
		}

		ch, err := r.router.CompleteStream(ctx, req)
		if err != nil {
			return Result{}, err
		}

		// pendingTools collects the tool_use deltas emitted by this
		// round. Anthropic streams tool input as `_partial` JSON
		// fragments under the same (ID, Name) so we accumulate them
		// per-ID and decode once the round closes.
		type toolAcc struct {
			id        string
			name      string
			fragments []string
		}
		pending := map[string]*toolAcc{}
		order := []string{}

		var (
			roundText     []byte
			roundThinking []byte
		)

		for d := range ch {
			switch d.Type {
			case providers.DeltaText:
				roundText = append(roundText, d.Text...)
			case providers.DeltaThinking:
				roundThinking = append(roundThinking, d.Text...)
			case providers.DeltaToolUse:
				if d.ToolUse == nil {
					continue
				}
				acc, ok := pending[d.ToolUse.ID]
				if !ok {
					acc = &toolAcc{id: d.ToolUse.ID, name: d.ToolUse.Name}
					pending[d.ToolUse.ID] = acc
					order = append(order, d.ToolUse.ID)
				}
				if frag, ok := d.ToolUse.Input["_partial"].(string); ok && frag != "" {
					acc.fragments = append(acc.fragments, frag)
				}
			case providers.DeltaDone:
				provider = d.Provider
				if d.Usage != nil {
					tokens += d.Usage.InputTokens + d.Usage.OutputTokens
					cost += d.Usage.CostUSD
				}
			case providers.DeltaError:
				return Result{}, d.Err
			}
		}

		text = append(text, roundText...)
		thinking = append(thinking, roundThinking...)

		// No tool calls — we're done.
		if len(order) == 0 || !mcpEnabled {
			break
		}
		// Cap hit — return whatever text we have rather than spinning.
		if round == maxToolRounds {
			break
		}

		// Dispatch each tool call and build a continuation prompt.
		// Built-in tools (e.g. generate_image) take precedence over
		// MCP — see WithBuiltinTool. The bearer + workspace ID come
		// from the Task so the handler can hit the caller's sandbox.
		results := "\n\n## Tool results\n"
		for _, id := range order {
			acc := pending[id]
			args := decodeToolInput(acc.fragments)
			out, err := r.dispatchTool(ctx, task, acc.name, args)
			results += "- " + acc.name + ": "
			if err != nil {
				results += "error: " + err.Error() + "\n"
				continue
			}
			results += out + "\n"
		}
		prompt = buildPrompt(task) + string(roundText) + results +
			"\nContinue. Use the tool results above to complete the task."
	}

	return Result{
		Role: task.Role, Output: string(text), Thinking: string(thinking),
		Provider: provider, Tokens: tokens, CostUSD: cost,
	}, nil
}

// RegisterDefaults loads the bundled agents.yaml and registers every entry.
// Callers can mutate the resulting Agents via Register() afterwards (e.g. to
// inject operator-managed overrides loaded from disk).
func (r *Registry) RegisterDefaults() {
	defaults, err := LoadDefaults()
	if err != nil {
		// Embedded file is shipped with the binary, so a parse failure is a
		// programmer error — fail loud rather than serve a half-empty
		// registry that would silently degrade chat.
		panic("agents: load defaults: " + err.Error())
	}
	for _, a := range defaults {
		r.Register(a)
	}
}

func buildPrompt(t Task) string {
	out := "# Goal\n" + t.Goal
	if t.Hint != "" {
		out += "\n\n# Hint\n" + t.Hint
	}
	if strings.TrimSpace(t.Context) != "" {
		out += "\n\n" + t.Context
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
