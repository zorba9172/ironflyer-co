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
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.providers) == 0 {
		return nil, errors.New("no providers registered")
	}
	wantPrivate := containsCap(caps, CapPrivate)

	bestScore := -1
	var best Provider
	for _, p := range r.providers {
		score := score(p.Capabilities(), caps)
		if wantPrivate && !containsCap(p.Capabilities(), CapPrivate) {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = p
		}
	}
	if best == nil {
		best = r.providers[0]
	}
	return best, nil
}

func (r *Router) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	p, err := r.Pick(req.Capabilities)
	if err != nil {
		return nil, err
	}
	return p.CompleteStream(ctx, req)
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
