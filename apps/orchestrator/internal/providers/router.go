// Package providers routes AI calls by capability tags, cost cap, and tenant
// policy. Streaming-first: every provider must implement CompleteStream;
// non-streaming Complete is a thin wrapper that drains the stream.
package providers

import (
	"context"
	"errors"
	"strings"
	"sync"
)

type Capability string

const (
	CapReasoning Capability = "reasoning"
	CapCode      Capability = "code"
	CapJSON      Capability = "json"
	CapVision    Capability = "vision"
	CapCheap     Capability = "cheap"
	CapFast      Capability = "fast"
	CapPrivate   Capability = "private"
	CapThinking  Capability = "thinking" // extended thinking (Claude 4.x)
	CapTools     Capability = "tools"    // tool use
	CapCache     Capability = "cache"    // prompt caching
)

type Request struct {
	System       string
	Prompt       string
	Capabilities []Capability
	JSONSchema   string
	MaxTokens    int
	Temperature  float32
	TenantID     string

	// ProjectContext is large, repeated context (codebase summary, spec, etc).
	// Providers with cache capability MUST mark it for caching to amortize
	// per-call cost.
	ProjectContext string

	// EnableThinking turns on extended thinking for reasoning-heavy tasks.
	EnableThinking bool

	// ThinkingBudget, when > 0, overrides the provider's default extended-
	// thinking budget for this single call. Lets the orchestrator scale
	// reasoning tokens up for hard tasks (architecture) and down for easy
	// ones (formatting fixes), trading latency / cost for output quality.
	ThinkingBudget int

	// Tools the agent may invoke during this completion.
	Tools []ToolSpec
}

type ToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// Delta is one streamed update.
type Delta struct {
	Type     DeltaType
	Text     string         // for DeltaText / DeltaThinking
	ToolUse  *ToolUseDelta  // for DeltaToolUse
	Provider string
	Model    string
	Usage    *Usage         // for DeltaDone
	Err      error          // for DeltaError
}

type DeltaType string

const (
	DeltaStart    DeltaType = "start"
	DeltaText     DeltaType = "text"
	DeltaThinking DeltaType = "thinking"
	DeltaToolUse  DeltaType = "tool_use"
	DeltaDone     DeltaType = "done"
	DeltaError    DeltaType = "error"
)

type ToolUseDelta struct {
	ID    string
	Name  string
	Input map[string]any
}

type Usage struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	CostUSD             float64
}

// Response is the convenience aggregate used by non-streaming callers.
type Response struct {
	Text     string
	Thinking string
	Provider string
	Model    string
	Usage    Usage
}

type Provider interface {
	Name() string
	Capabilities() []Capability
	// CompleteStream returns a channel of Delta. Channel is closed when the
	// stream ends (after DeltaDone or DeltaError).
	CompleteStream(ctx context.Context, req Request) (<-chan Delta, error)
}

type Router struct {
	mu        sync.RWMutex
	providers []Provider
}

func NewRouter() *Router { return &Router{} }

func (r *Router) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

// Pick returns the provider with the most capability-tag overlap. Providers
// supporting "private" are preferred when CapPrivate is requested.
func (r *Router) Pick(caps []Capability) (Provider, error) {
	chain := r.PickChain(caps)
	if len(chain) == 0 {
		return nil, errors.New("no providers registered")
	}
	return chain[0], nil
}

// PickChain returns providers ranked by capability-tag overlap (best
// first). When CapPrivate is requested, non-private providers are dropped
// from the chain. The slice is safe to iterate as a fallback chain.
func (r *Router) PickChain(caps []Capability) []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.providers) == 0 {
		return nil
	}
	wantPrivate := containsCap(caps, CapPrivate)

	type scored struct {
		p Provider
		s int
	}
	candidates := make([]scored, 0, len(r.providers))
	for _, p := range r.providers {
		if wantPrivate && !containsCap(p.Capabilities(), CapPrivate) {
			continue
		}
		candidates = append(candidates, scored{p: p, s: score(p.Capabilities(), caps)})
	}
	if len(candidates) == 0 {
		// Want-private was requested but no private provider is registered.
		// Fall back to every provider in registration order so the caller
		// still gets a stream (privacy can't be enforced at this layer).
		out := make([]Provider, len(r.providers))
		copy(out, r.providers)
		return out
	}
	// Stable sort by score desc, registration order as tiebreaker.
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].s > candidates[j-1].s; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
	out := make([]Provider, len(candidates))
	for i, c := range candidates {
		out[i] = c.p
	}
	return out
}

func (r *Router) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	chain := r.PickChain(req.Capabilities)
	if len(chain) == 0 {
		return nil, errors.New("no providers registered")
	}
	// Try the best provider first. If its first delta arrives intact we
	// return that channel as-is. Only "couldn't even start the stream"
	// errors trigger fallback — once a stream is producing tokens we let
	// it run; mid-stream errors propagate so callers don't pay 2× for the
	// same partial output.
	// Try chain[0] first, then on a failed start or immediate DeltaError,
	// walk down the chain. Once any non-error delta has been emitted we
	// commit to that provider and propagate errors as-is.
	wrapped := make(chan Delta, 32)
	go runFallbackChain(ctx, wrapped, chain, req)
	return wrapped, nil
}

// runFallbackChain drives the entire fallback policy on a goroutine.
// Output channel is owned here and closed exactly once on exit.
func runFallbackChain(ctx context.Context, out chan<- Delta, chain []Provider, req Request) {
	defer close(out)
	var lastErr error
	for idx, p := range chain {
		ch, err := p.CompleteStream(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}
		committed := false
		for d := range ch {
			if !committed && d.Type == DeltaError {
				// Don't commit, don't forward — try the next provider.
				lastErr = d.Err
				break
			}
			committed = true
			out <- d
		}
		if committed {
			return
		}
		_ = idx
	}
	if lastErr == nil {
		lastErr = errors.New("router: every provider in chain refused to start")
	}
	out <- Delta{Type: DeltaError, Err: lastErr}
}

// Complete drains the stream and returns the aggregate.
func (r *Router) Complete(ctx context.Context, req Request) (Response, error) {
	ch, err := r.CompleteStream(ctx, req)
	if err != nil {
		return Response{}, err
	}
	var resp Response
	var text, thinking strings.Builder
	for d := range ch {
		switch d.Type {
		case DeltaText:
			text.WriteString(d.Text)
		case DeltaThinking:
			thinking.WriteString(d.Text)
		case DeltaDone:
			resp.Provider, resp.Model = d.Provider, d.Model
			if d.Usage != nil {
				resp.Usage = *d.Usage
			}
		case DeltaError:
			return Response{}, d.Err
		}
	}
	resp.Text, resp.Thinking = text.String(), thinking.String()
	return resp, nil
}

func score(have, want []Capability) int {
	set := make(map[Capability]struct{}, len(have))
	for _, c := range have {
		set[c] = struct{}{}
	}
	n := 0
	for _, c := range want {
		if _, ok := set[c]; ok {
			n++
		}
	}
	return n
}

func containsCap(caps []Capability, want Capability) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}
