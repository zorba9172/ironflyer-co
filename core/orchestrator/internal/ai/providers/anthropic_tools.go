// Helpers for the Anthropic provider's native multi-turn tool-use loop.
//
// The provider's CompleteStream can optionally run the canonical Anthropic
// Messages API loop: when the assistant turn ends with stop_reason=tool_use
// we append the assistant message verbatim (including its tool_use blocks),
// dispatch every tool call via the caller-supplied ToolDispatcher, then send
// the matching tool_result blocks back as the next user message — looping
// until stop_reason != tool_use or the iteration cap fires.
//
// The dispatcher is plumbed through context.Context rather than the Request
// struct so the Provider interface and Request schema stay untouched.
// Callers opt into native multi-turn by wrapping their context with
// WithToolDispatcher; without a dispatcher the provider falls back to the
// legacy single-turn behavior (emit DeltaToolUse, stop, let the caller
// re-prompt).

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolDispatcher executes a single tool_use the assistant requested. It
// returns the textual result that will be packed into a tool_result block
// and the isError flag the Anthropic API expects on failure. Returning a
// non-nil error is equivalent to (err.Error(), true) — the provider folds
// errors into the tool_result so the assistant can recover gracefully.
type ToolDispatcher func(ctx context.Context, toolUseID, name string, args map[string]any) (string, error)

type toolDispatcherKey struct{}

// WithToolDispatcher returns a derived context that carries fn. The
// Anthropic provider checks for this dispatcher on every CompleteStream
// call. When present it activates the native multi-turn loop; when absent
// (legacy callers) the provider keeps its old single-turn behavior so
// existing code paths remain backward-compatible.
func WithToolDispatcher(ctx context.Context, fn ToolDispatcher) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, toolDispatcherKey{}, fn)
}

func toolDispatcherFromContext(ctx context.Context) ToolDispatcher {
	v, _ := ctx.Value(toolDispatcherKey{}).(ToolDispatcher)
	return v
}

// maxToolResultBytes caps the size of any single tool_result content blob
// we send back to the model. Large reads (e.g. a 200kB file) would
// otherwise consume the full context window in a single turn. ~30k chars
// is roughly ~7.5k tokens — generous for tool output, tight enough to keep
// a multi-turn loop within budget.
const maxToolResultBytes = 30_000

// maxToolIterations bounds the native multi-turn loop so a misbehaving
// model that keeps calling tools cannot pin a single CompleteStream. The
// cap is generous (8) because legitimate plans — read, search, edit,
// verify — can use several round-trips. When the cap is hit we surface
// a structured DeltaError that includes the trace of tool calls so the
// caller can debug.
const maxToolIterations = 8

// truncateToolResult enforces the size limit on a single tool_result body.
// We keep the head intact (most tool output puts the salient signal at
// the start) and append a clear marker so the model is told explicitly
// that more output was dropped.
func truncateToolResult(s string) string {
	if len(s) <= maxToolResultBytes {
		return s
	}
	return s[:maxToolResultBytes] + "\n\n[truncated: tool output exceeded " +
		fmt.Sprintf("%d", maxToolResultBytes) + " bytes]"
}

// toolUseTrace is one entry in the structured error we surface when the
// iteration cap fires. It lets the caller (and ultimately the user) see
// which tools the model invoked before we gave up.
type toolUseTrace struct {
	Round int            `json:"round"`
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Args  map[string]any `json:"args,omitempty"`
	Err   string         `json:"error,omitempty"`
}

// toolLoopError is returned via DeltaError when the multi-turn loop hits
// maxToolIterations without the assistant settling on a final response.
type toolLoopError struct {
	Iterations int            `json:"iterations"`
	Trace      []toolUseTrace `json:"trace"`
}

func (e *toolLoopError) Error() string {
	b, _ := json.Marshal(e)
	return "anthropic: tool-use loop exceeded cap: " + string(b)
}

// joinPartialJSON merges the streaming `input_json_delta` fragments
// emitted by the Anthropic SSE protocol. The protocol guarantees they
// arrive in order; we concatenate them and decode once. Malformed JSON
// degrades to an empty map so a single bad tool call never panics the
// whole loop.
func joinPartialJSON(fragments []string) map[string]any {
	if len(fragments) == 0 {
		return map[string]any{}
	}
	joined := strings.Join(fragments, "")
	var out map[string]any
	if err := json.Unmarshal([]byte(joined), &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}
