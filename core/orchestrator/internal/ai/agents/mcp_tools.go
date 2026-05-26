// MCP tool integration for the Coder agent.
//
// The Registry can be configured with an *providers.MCPClientRegistry —
// when present, the Coder's Run() loop fetches the union of every
// configured MCP server's tool catalogue and forwards it to the
// provider as native tool_use tools.
//
// The current integration is intentionally one-shot: when the model
// emits tool_use deltas we collect them, dispatch each via the MCP
// registry, then re-prompt the provider once with a "## Tool results"
// section appended to the original prompt. A full multi-turn tool
// loop would require provider-level changes (sending tool_result
// content blocks back through the SDK) that other workers are
// touching; this simplified path makes the catalogue available now
// without invasive refactors.

package agents

import (
	"context"
	"encoding/json"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// WithMCPClients attaches an MCP client registry to the agents
// Registry. Only the Coder role consults it — other roles must stay
// deterministic so gate dispatch never surprises the loop with an
// external side-effect.
func (r *Registry) WithMCPClients(reg *providers.MCPClientRegistry) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mcpClients = reg
	return r
}

// mcpToolSpecs flattens every configured MCP server's tool catalogue
// into providers.ToolSpec entries the provider layer understands and
// appends any built-in tools registered via WithBuiltinTool. Returns
// nil when neither source produced any tools so callers can append
// unconditionally.
func (r *Registry) mcpToolSpecs(ctx context.Context) []providers.ToolSpec {
	r.mu.RLock()
	reg := r.mcpClients
	builtin := append([]providers.ToolSpec(nil), r.builtinTools...)
	r.mu.RUnlock()

	var mcpTools []providers.MCPTool
	if reg != nil {
		mcpTools = reg.AllTools(ctx)
	}
	if len(mcpTools) == 0 && len(builtin) == 0 {
		return nil
	}
	out := make([]providers.ToolSpec, 0, len(mcpTools)+len(builtin))
	for _, t := range mcpTools {
		out = append(out, providers.ToolSpec{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	out = append(out, builtin...)
	return out
}

// BuiltinToolFunc is the handler signature for tools the orchestrator
// implements in-process (e.g. generate_image). It receives the calling
// user's bearer + workspace ID so the handler can write into the
// caller's runtime sandbox.
type BuiltinToolFunc func(ctx context.Context, userBearer, workspaceID string, args map[string]any) (string, error)

// WithBuiltinTool registers a built-in tool the Coder can call. The
// spec is appended to the catalogue returned by mcpToolSpecs and the
// handler is dispatched ahead of any MCP server with the same name —
// built-ins take precedence so the orchestrator can ship trusted
// capabilities (image generation, etc.) without being shadowed by a
// misconfigured external server.
func (r *Registry) WithBuiltinTool(spec providers.ToolSpec, call BuiltinToolFunc) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.builtinCalls == nil {
		r.builtinCalls = map[string]BuiltinToolFunc{}
	}
	// Replace any existing tool of the same name in builtinTools so a
	// second WithBuiltinTool overrides cleanly.
	replaced := false
	for i, existing := range r.builtinTools {
		if existing.Name == spec.Name {
			r.builtinTools[i] = spec
			replaced = true
			break
		}
	}
	if !replaced {
		r.builtinTools = append(r.builtinTools, spec)
	}
	r.builtinCalls[spec.Name] = call
	return r
}

// runMCPToolCall dispatches a single tool_use back to the MCP client
// registry. Returns the text body the server emitted (concatenated
// across content blocks) or the structured error.
func (r *Registry) runMCPToolCall(ctx context.Context, name string, args map[string]any) (string, error) {
	r.mu.RLock()
	reg := r.mcpClients
	r.mu.RUnlock()
	if reg == nil {
		return "", errNoMCPRegistry
	}
	return reg.CallTool(ctx, name, args)
}

// dispatchTool routes a tool_use to the right backend: built-in
// handlers (registered via WithBuiltinTool) win over MCP servers
// advertising the same name so the orchestrator can ship trusted
// capabilities that can't be shadowed.
func (r *Registry) dispatchTool(ctx context.Context, task Task, name string, args map[string]any) (string, error) {
	r.mu.RLock()
	handler, ok := r.builtinCalls[name]
	r.mu.RUnlock()
	if ok {
		return handler(ctx, task.UserBearer, task.WorkspaceID, args)
	}
	return r.runMCPToolCall(ctx, name, args)
}

// errNoMCPRegistry is returned when runMCPToolCall is invoked but no
// registry was attached. Kept package-private so the Registry stays
// the only entry point.
var errNoMCPRegistry = errMCPRegistryMissing{}

type errMCPRegistryMissing struct{}

func (errMCPRegistryMissing) Error() string {
	return "agents: MCP client registry not configured"
}

// decodeToolInput merges streamed partial-JSON fragments emitted by
// the Anthropic provider into a concrete arguments map. The provider
// surfaces each fragment under a `_partial` string key; we concatenate
// them in arrival order, attempt a final json.Unmarshal, and fall
// back to an empty map on parse failure so an upstream malformed
// stream never panics the Coder.
func decodeToolInput(fragments []string) map[string]any {
	if len(fragments) == 0 {
		return map[string]any{}
	}
	joined := ""
	for _, frag := range fragments {
		joined += frag
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(joined), &args); err != nil || args == nil {
		return map[string]any{}
	}
	return args
}
