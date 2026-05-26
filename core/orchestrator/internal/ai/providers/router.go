// Package providers routes AI calls by capability tags, cost cap, and tenant
// policy. Streaming-first: every provider must implement CompleteStream;
// non-streaming Complete is a thin wrapper that drains the stream.
package providers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
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
	// CapSpeculative tells the router to race the top two matching
	// providers in parallel and return the first one to emit a token. The
	// loser is cancelled. Use only for calls where any non-error response
	// is a valid answer (Critic verdicts, brainstorm rounds, intent
	// classification) — never for calls where the model's identity affects
	// downstream behaviour (a Coder vs. Coder race could produce two
	// different patches; the winner is the one that started talking first,
	// not necessarily the better one).
	CapSpeculative Capability = "speculative"
	// CapInline tags low-latency middle-fill-in code completions
	// (Cursor-style ghost-text). Bandit treats it as its own capability
	// set so a Haiku/4o-mini that wins on `inline_completion` reward
	// keeps winning, regardless of how it ranks on long-form `code` /
	// `reasoning` calls.
	CapInline Capability = "inline_completion"
	// CapQuality requests the highest-quality (most expensive) tier the
	// provider exposes. Use sparingly — architecture passes, security
	// review, hardest reasoning calls. Per-provider pickModel maps this
	// to Opus 4.7 / Gemini 2.5 Pro / o3, paid accordingly.
	CapQuality Capability = "quality"
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

	// Attachments are user-supplied images (screenshots, mockups, design
	// references) sent alongside Prompt. Providers MUST advertise CapVision
	// before they will honour these; the router skips vision-bearing calls
	// to text-only providers in the failover chain.
	Attachments []Attachment

	// PreferredProvider names a specific provider the caller wants to
	// honour for this call. Set by ProfitGuard's SwitchProvider verdict
	// (see guard.go) so the next call lands on the recommended cheaper
	// provider regardless of the bandit's incumbent. Empty means "let
	// the router pick normally". If the named provider is unknown or
	// cannot honour the requested capabilities, the router logs a Warn
	// and falls back to the capability-scored pick.
	PreferredProvider string
}

// Attachment is one user-supplied image attached to the prompt. MediaType
// is the IANA type ("image/png", "image/jpeg", "image/webp", "image/gif").
// Base64 is the raw image bytes, base64-encoded with no data: prefix.
type Attachment struct {
	MediaType string
	Base64    string
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
	bandit    *Bandit
	// ctxBandit, when non-nil, is the proprietary contextual bandit
	// (LinUCB) that re-orders PickChain output by per-tenant learned
	// reward. Enabled via IRONFLYER_BANDIT_STRATEGY=contextual at
	// boot. The legacy `bandit` field stays wired so the existing
	// UCB1/Thompson rerank continues to run when ctxBandit is nil.
	ctxBandit *ContextualBandit

	// Failover wiring. Optional — zero values keep the existing
	// best-effort fallback in runFallbackChain working unchanged.
	logger           zerolog.Logger
	hasLogger        bool
	tel              TelemetrySink
	failoverDepth    int
	hasFailoverDepth bool

	// Per-provider circuit breakers. Lazily constructed on first
	// observation of a provider name so the router with N enabled
	// providers gets N breakers. nil-safe: when no logger has been
	// attached the registry still functions (just doesn't log state
	// transitions). See breaker.go.
	breakers *breakerRegistry
}

func NewRouter() *Router { return &Router{} }

// breakerFor returns the per-provider circuit breaker, constructing
// the registry + the entry lazily on first call. Concurrency-safe.
func (r *Router) breakerFor(name string) *ProviderBreaker {
	r.mu.Lock()
	if r.breakers == nil {
		r.breakers = newBreakerRegistry(r.logger, r.hasLogger)
	}
	reg := r.breakers
	r.mu.Unlock()
	return reg.breakerFor(name)
}

// WithBandit attaches a UCB1 bandit that re-ranks PickChain output using
// historical telemetry. Pass nil to disable. Returns the router so it
// chains with NewRouter().
func (r *Router) WithBandit(b *Bandit) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bandit = b
	return r
}

// WithContextualBandit attaches the LinUCB contextual bandit. When
// set, PickChain hoists the bandit's Select winner to the head of
// the chain (after the legacy bandit's rerank, so the two policies
// compose cleanly during the rollout window). Pass nil to disable.
func (r *Router) WithContextualBandit(b *ContextualBandit) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctxBandit = b
	return r
}

// ContextualBandit returns the currently attached contextual bandit
// or nil. Used by main.go wireup so the RouterModel can subscribe to
// the exact instance the router is consulting.
func (r *Router) ContextualBandit() *ContextualBandit {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ctxBandit
}

func (r *Router) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

// AllProviders returns a stable snapshot of every registered provider
// in registration order. Used by the operational /providers/health
// probe so the status page can ping each configured provider directly
// rather than going through the BillingGuard (which would charge the
// caller for the ping). The returned slice is a fresh copy — mutating
// it does not affect the router's internal state.
func (r *Router) AllProviders() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Provider, len(r.providers))
	copy(out, r.providers)
	return out
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

// PickByName returns the registered provider with the given Name(),
// or (nil, false) when no such provider exists. Used by ProfitGuard's
// SwitchProvider verdict to honour the recommended provider when its
// capabilities cover the request — see Router.CompleteStream / guard.go.
func (r *Router) PickByName(name string) (Provider, bool) {
	if name == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.providers {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
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
	if r.bandit != nil {
		out = r.bandit.Rerank(out, caps)
	}
	if r.ctxBandit != nil && len(out) > 1 {
		rc := routeContextFromCaps(caps, "", "", 0)
		if winner := r.ctxBandit.Select(context.Background(), rc); winner != "" {
			for i, p := range out {
				if p.Name() == winner {
					if i != 0 {
						w := out[i]
						copy(out[1:i+1], out[0:i])
						out[0] = w
					}
					break
				}
			}
		}
	}
	return out
}

// routeContextFromCaps builds a RouteContext for the contextual
// bandit from the request's capability tags. The first matching
// capability bucket wins (reasoning > code > json > cheap > fast >
// vision) so a single Capability string is a faithful summary of
// the request intent. tenantID is currently unused in feature
// encoding — kept in the signature so per-tenant feature lanes can
// be added without a router-wide breaking change.
func routeContextFromCaps(caps []Capability, tenantTier, _ string, promptTokens int) RouteContext {
	rc := RouteContext{
		PromptTokens: promptTokens,
		TenantTier:   tenantTier,
		TimeOfDay:    time.Now().UTC().Hour(),
	}
	for _, c := range caps {
		switch c {
		case CapReasoning:
			if rc.Capability == "" {
				rc.Capability = "reasoning"
			}
		case CapCode:
			if rc.Capability == "" {
				rc.Capability = "code"
			}
		case CapJSON:
			if rc.Capability == "" {
				rc.Capability = "json"
			}
		case CapCheap:
			if rc.Capability == "" {
				rc.Capability = "cheap"
			}
		case CapFast:
			if rc.Capability == "" {
				rc.Capability = "fast"
			}
		case CapVision:
			if rc.Capability == "" {
				rc.Capability = "vision"
			}
		case CapThinking:
			rc.HasThinking = true
		case CapTools:
			rc.HasTools = true
		}
	}
	return rc
}

func (r *Router) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	// Speculative path: race the top 2 matching providers. We strip the
	// CapSpeculative tag before forwarding so the inner picker doesn't
	// try to filter on a tag no provider actually advertises.
	if containsCap(req.Capabilities, CapSpeculative) {
		inner := req
		inner.Capabilities = stripCap(inner.Capabilities, CapSpeculative)
		return r.RaceFirst(ctx, inner, 2)
	}

	// Attachments imply CapVision — silently promote so the chain only
	// considers providers that advertise it. Without this a vision payload
	// could be routed to a text-only fallback and 400 mid-stream.
	caps := req.Capabilities
	if len(req.Attachments) > 0 && !containsCap(caps, CapVision) {
		caps = append([]Capability{CapVision}, caps...)
	}
	chain := r.PickChain(caps)
	if len(chain) == 0 {
		return nil, errors.New("no providers registered")
	}
	// Honour PreferredProvider (set by ProfitGuard's SwitchProvider
	// verdict) when the named provider exists AND covers the request's
	// capability tags. If it cannot, log Warn and fall back to the
	// capability-scored chain unchanged.
	if name := req.PreferredProvider; name != "" {
		if p, ok := r.PickByName(name); ok {
			if score(p.Capabilities(), caps) >= len(caps) || len(caps) == 0 {
				chain = hoistProvider(chain, p)
			} else if r.hasLogger {
				r.logger.Warn().Str("preferred_provider", name).
					Strs("capabilities", capStrings(caps)).
					Msg("router: preferred provider lacks required capabilities; falling back to chain")
			}
		} else if r.hasLogger {
			r.logger.Warn().Str("preferred_provider", name).
				Msg("router: preferred provider not registered; falling back to chain")
		}
	}
	// Drop providers that can't honour vision when attachments are present.
	if len(req.Attachments) > 0 {
		filtered := chain[:0]
		for _, p := range chain {
			if containsCap(p.Capabilities(), CapVision) {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) == 0 {
			return nil, errors.New("no vision-capable provider registered for image attachments")
		}
		chain = filtered
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

// RaceFirst launches up to `n` providers in parallel, returns the channel
// of the FIRST one that emits any non-error delta, and cancels the rest.
// This is the speculative-execution play: when the cheap model usually
// produces a correct answer in half the time, we let it race the
// expensive model; only when it errors or stalls past the deadline does
// the expensive winner pay off. Callers should use this only for tasks
// where ANY non-error stream is a valid answer (e.g. Critic verdicts,
// brainstorm rounds) — never for tasks where the model's identity
// affects downstream behaviour.
//
// `n <= 1` falls back to plain CompleteStream so the call shape is safe to
// adopt unconditionally.
func (r *Router) RaceFirst(ctx context.Context, req Request, n int) (<-chan Delta, error) {
	if n <= 1 {
		return r.CompleteStream(ctx, req)
	}
	chain := r.PickChain(req.Capabilities)
	if len(chain) == 0 {
		return nil, errors.New("no providers registered")
	}
	if n > len(chain) {
		n = len(chain)
	}

	out := make(chan Delta, 64)

	// Each speculator gets its own context so the loser can be cancelled
	// the instant a winner commits. We don't cancel the parent: the caller
	// owns it.
	type result struct {
		ch     <-chan Delta
		cancel func()
		err    error
	}

	type firstFrame struct {
		delta Delta
		ch    <-chan Delta
		cancel func()
		idx   int
	}
	firstCh := make(chan firstFrame, n)
	cancellers := make([]func(), 0, n)
	var spawnMu sync.Mutex

	for i := 0; i < n; i++ {
		i := i
		childCtx, cancel := context.WithCancel(ctx)
		spawnMu.Lock()
		cancellers = append(cancellers, cancel)
		spawnMu.Unlock()
		go func() {
			ch, err := chain[i].CompleteStream(childCtx, req)
			if err != nil {
				cancel()
				return
			}
			// Wait for the first delta. If it's an error frame, the
			// speculator is dead — silently drop and let other racers win.
			// Anything else qualifies as "this one is winning."
			d, ok := <-ch
			if !ok {
				cancel()
				return
			}
			if d.Type == DeltaError {
				cancel()
				return
			}
			firstCh <- firstFrame{delta: d, ch: ch, cancel: cancel, idx: i}
		}()
		_ = result{}
	}

	go func() {
		defer close(out)
		select {
		case <-ctx.Done():
			spawnMu.Lock()
			for _, c := range cancellers {
				c()
			}
			spawnMu.Unlock()
			out <- Delta{Type: DeltaError, Err: ctx.Err()}
			return
		case ff := <-firstCh:
			// Cancel every loser.
			spawnMu.Lock()
			for j, c := range cancellers {
				if j != ff.idx {
					c()
				}
			}
			spawnMu.Unlock()
			out <- ff.delta
			for d := range ff.ch {
				out <- d
			}
		}
	}()

	return out, nil
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

// stripCap returns caps without `drop`. Used by CompleteStream to remove
// router-only tags (e.g. CapSpeculative) before forwarding the request
// to a provider that doesn't recognise them.
func stripCap(caps []Capability, drop Capability) []Capability {
	out := caps[:0:0]
	for _, c := range caps {
		if c != drop {
			out = append(out, c)
		}
	}
	return out
}

// hoistProvider returns a copy of `chain` with `target` moved to the
// head. If target is not already in the chain it is prepended. Used to
// honour PreferredProvider without losing the rest of the fallback
// candidates (so a transient error on the preferred pick still rolls
// over to the bandit's runners-up).
func hoistProvider(chain []Provider, target Provider) []Provider {
	out := make([]Provider, 0, len(chain)+1)
	out = append(out, target)
	for _, p := range chain {
		if p == target || p.Name() == target.Name() {
			continue
		}
		out = append(out, p)
	}
	return out
}

// capStrings converts []Capability to []string. Used for log/telemetry
// formatting where we don't want to leak the Capability newtype.
func capStrings(caps []Capability) []string {
	out := make([]string, len(caps))
	for i, c := range caps {
		out[i] = string(c)
	}
	return out
}
