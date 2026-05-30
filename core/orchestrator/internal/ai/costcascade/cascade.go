package costcascade

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/providers"
	"ironflyer/core/orchestrator/internal/operations/metrics"
)

// errEmptyCascade is returned only when a Cascade is constructed without a
// downstream completer and invoked — a wiring bug, never a runtime path.
var errEmptyCascade = errors.New("costcascade: no downstream completer configured")

// Completer is the single model-call surface the cascade wraps and exposes.
// providers.BillingGuard satisfies it, and so does the Cascade itself — that
// is what makes the cascade a drop-in decorator for agents.NewRegistry.
type Completer interface {
	CompleteStreamWithFailover(ctx context.Context, req providers.Request) (<-chan providers.Delta, error)
}

// KnowledgeFunc is the optional Layer-3 short-circuit. It is handed the
// request and may return (answer, true) to resolve it from existing project
// knowledge / a reusable implementation without a model call, or
// (_, false) to pass it down to the model tiers. It MUST be fast and pure
// of side effects; it MUST NOT fabricate code it cannot ground. Default is
// nil (layer skipped) — wiring it is how an operator turns "most code
// already exists; avoid regeneration" into real savings.
type KnowledgeFunc func(ctx context.Context, req providers.Request) (string, bool)

// ResponseStore is the Layer-2 cache contract. The built-in ResponseCache
// (exact-hash) satisfies it; a semantic (embedding-similarity) store
// satisfies it too, so the cascade can be pointed at either without
// changing the resolution flow. Get/Put carry a context so a semantic
// implementation can embed under the caller's deadline. Eligible is the
// safety gate — a store decides which requests it is willing to cache.
type ResponseStore interface {
	Eligible(req providers.Request) bool
	Get(ctx context.Context, req providers.Request) (CachedResponse, bool)
	Put(ctx context.Context, req providers.Request, resp CachedResponse)
}

// Compressor is the optional pre-model context/prompt reducer (LLMLingua-
// class). It is applied AFTER the cache/knowledge layers miss and BEFORE
// the model tier is chosen, so a smaller prompt both costs less and may
// drop into a cheaper tier. It returns the reduced request and the
// estimated input tokens saved (0 = left untouched). It MUST preserve
// meaning — never drop the actual instruction — and MUST no-op on prompts
// already below its floor.
type Compressor interface {
	Compress(ctx context.Context, req providers.Request) (providers.Request, int)
}

// DifficultyScorer is the optional learned/heuristic router (RouteLLM-
// class). It scores a request's difficulty in [0,1]; the classifier maps
// that to the cheapest tier expected to answer well (low → reflex, mid →
// planning, high → reasoning). An explicit premium capability on the
// request is always honoured as a floor — the scorer can route UP, never
// silently below what the caller explicitly demanded.
type DifficultyScorer interface {
	Score(ctx context.Context, req providers.Request) float64
}

// Config tunes the cascade. The zero value (Enabled=false) makes the
// cascade a transparent pass-through, so wiring it is always safe.
type Config struct {
	Enabled         bool
	ResponseCache   bool          // gate Layer 2 (default off → zero correctness risk)
	CacheMaxEntries int           // LRU capacity (default 512)
	CacheTTL        time.Duration // entry freshness (default 30m)
	AllowDowngrade  bool          // let the classifier degrade premium → planning when hot
	CostRatioTarget float64       // aggression ceiling (default 0.20)
}

// Cascade is the layered cost-optimization front door. It implements
// Completer so it slots in wherever a BillingGuard goes.
type Cascade struct {
	next   Completer
	cfg    Config
	logger zerolog.Logger

	rules      *RuleSet
	cache      ResponseStore
	compressor Compressor
	knowledge  KnowledgeFunc
	classifier *Classifier
	aggr       *Aggression
	stats      *Stats
	budget     *WindowedBudgetManager
}

// New builds a cascade wrapping next (the real BillingGuard). When
// cfg.Enabled is false the returned cascade delegates every call verbatim.
func New(next Completer, cfg Config, logger zerolog.Logger) *Cascade {
	c := &Cascade{
		next:       next,
		cfg:        cfg,
		logger:     logger,
		rules:      NewRuleSet(),
		classifier: NewClassifier(cfg.AllowDowngrade, 0.5),
		aggr:       NewAggression(cfg.CostRatioTarget),
		stats:      &Stats{},
	}
	if cfg.ResponseCache {
		c.cache = NewResponseCache(cfg.CacheMaxEntries, cfg.CacheTTL)
	}
	return c
}

// WithKnowledge wires the optional Layer-3 short-circuit. Returns the
// cascade for chaining.
func (c *Cascade) WithKnowledge(fn KnowledgeFunc) *Cascade {
	c.knowledge = fn
	return c
}

// WithRules replaces the rule set (e.g. to register deployment-specific
// deterministic rules on top of the built-ins). Returns the cascade.
func (c *Cascade) WithRules(rs *RuleSet) *Cascade {
	if rs != nil {
		c.rules = rs
	}
	return c
}

// WithRevenueSource wires the revenue figure the aggression controller
// measures cost against. Returns the cascade for chaining.
func (c *Cascade) WithRevenueSource(fn func() float64) *Cascade {
	c.aggr.WithRevenueSource(fn)
	return c
}

// WithResponseStore replaces the Layer-2 cache (e.g. swap the exact-hash
// store for a semantic, embedding-similarity store). nil is ignored.
// Returns the cascade for chaining.
func (c *Cascade) WithResponseStore(s ResponseStore) *Cascade {
	if s != nil {
		c.cache = s
	}
	return c
}

// WithCompressor wires the optional prompt/context compressor applied
// before the model tier is chosen. nil is ignored. Returns the cascade.
func (c *Cascade) WithCompressor(comp Compressor) *Cascade {
	if comp != nil {
		c.compressor = comp
	}
	return c
}

// WithDifficultyScorer wires the optional learned/heuristic router the
// classifier consults to pick the cheapest viable tier. nil is ignored.
// Returns the cascade for chaining.
func (c *Cascade) WithDifficultyScorer(s DifficultyScorer) *Cascade {
	if s != nil {
		c.classifier.scorer = s
	}
	return c
}

// WithBudgetManager wires the optional multi-window (session/daily/task)
// budget manager. When set, the cascade consults the DAILY window — keyed
// by the request's tenant, the only window key derivable at this layer —
// as an advisory pre-admission gate before delegating to a model tier, and
// charges the window the actual cost on completion. It NEVER moves real
// money (the wallet/ledger remain the source of truth); a denial here
// surfaces as a budget error exactly like ProfitGuard's PauseForBudget.
// nil is ignored. Returns the cascade for chaining.
func (c *Cascade) WithBudgetManager(b *WindowedBudgetManager) *Cascade {
	if b != nil {
		c.budget = b
	}
	return c
}

// BudgetManager exposes the wired window manager (nil when unset) so an
// ops surface can read its per-feature / per-task rollups.
func (c *Cascade) BudgetManager() *WindowedBudgetManager { return c.budget }

// Stats returns the live resolution distribution for ops surfaces.
func (c *Cascade) Stats() Snapshot { return c.stats.Snapshot() }

// Aggression exposes the controller so a billing-period cron can Reset it.
func (c *Cascade) Aggression() *Aggression { return c.aggr }

// CompleteStreamWithFailover is the budgeted, cost-optimized entry point.
// It walks the cascade layers cheapest-first and only delegates to the
// wrapped guard when no cheaper layer can answer.
func (c *Cascade) CompleteStreamWithFailover(ctx context.Context, req providers.Request) (<-chan providers.Delta, error) {
	if c == nil || !c.cfg.Enabled || c.next == nil {
		if c != nil && c.next != nil {
			return c.next.CompleteStreamWithFailover(ctx, req)
		}
		return nil, errEmptyCascade
	}

	// Layer 1 — Rules. Deterministic answer or refusal, zero tokens.
	if res, ok := c.rules.Match(ctx, req); ok {
		if res.Refuse {
			c.resolve(LayerRules, req)
			err := res.RefuseErr
			if err == nil {
				err = ErrEmptyPrompt
			}
			return nil, err
		}
		c.resolve(LayerRules, req)
		return c.synth(req, res.Answer), nil
	}

	// Layer 2 — Cache. Exact-hash (or semantic) replay of a prior call.
	if c.cache != nil && c.cache.Eligible(req) {
		if hit, ok := c.cache.Get(ctx, req); ok {
			c.resolve(LayerCache, req)
			return c.replay(hit), nil
		}
	}

	// Layer 3 — Knowledge. Answer from existing project knowledge / reuse.
	if c.knowledge != nil {
		if ans, ok := c.knowledge(ctx, req); ok && strings.TrimSpace(ans) != "" {
			c.resolve(LayerKnowledge, req)
			return c.synth(req, ans), nil
		}
	}

	// Prompt/context compression — shrink the prompt before it hits a model
	// (LLMLingua-class). A smaller prompt costs less and can drop the call
	// into a cheaper tier. Credited to the savings counter at the chosen
	// tier's input rate.
	if c.compressor != nil {
		if compressed, savedTokens := c.compressor.Compress(ctx, req); savedTokens > 0 {
			saved := float64(savedTokens) * inputRatePerToken(tierOf(req.Capabilities))
			c.stats.addSavings(saved)
			metrics.AddCascadeSavings(saved)
			req = compressed
		}
	}

	// Windowed budget pre-gate — advisory daily ceiling keyed by tenant (the
	// only window key derivable here). A denial stops the call before any
	// model spend, mirroring ProfitGuard's PauseForBudget. The wallet/ledger
	// still own the real money; this is a finer-grained soft ceiling.
	if c.budget != nil && strings.TrimSpace(req.TenantID) != "" {
		dk := BudgetKey{Kind: BudgetWindowDaily, ID: req.TenantID}
		if d := c.budget.Admit(ctx, dk, decimal.NewFromFloat(estimateCostUSD(req))); !d.Admit {
			return nil, errors.New("costcascade: daily budget: " + d.Reason)
		}
	}

	// Layers 4-6 — model tiers. Classify into reflex / planning / reasoning
	// (a difficulty scorer may route, and the aggression controller may
	// degrade premium when hot) and delegate to the wrapped guard. The layer
	// is recorded now; the real cost is folded into the aggression controller
	// on DeltaDone, and an eligible deterministic response is cached for next
	// time.
	layer, outReq := c.classifier.Classify(ctx, req, c.aggr.Level())
	in, err := c.next.CompleteStreamWithFailover(ctx, outReq)
	if err != nil {
		return nil, err
	}
	c.stats.record(layer)
	metrics.ObserveCascadeLayer(string(layer))
	return c.tap(ctx, outReq, in), nil
}

// resolve records a zero-cost (rules/cache/knowledge) resolution: bump the
// layer tally + Prometheus, and credit the estimated avoided provider cost
// to the savings counters.
func (c *Cascade) resolve(layer Layer, req providers.Request) {
	c.stats.record(layer)
	metrics.ObserveCascadeLayer(string(layer))
	saved := estimateCostUSD(req)
	c.stats.addSavings(saved)
	metrics.AddCascadeSavings(saved)
}

// synth returns a synthetic, zero-cost Delta stream carrying a single text
// answer. Provider/Model are tagged "cascade"/"rules" so downstream
// telemetry never mistakes it for a real provider call. Usage is zero —
// nothing is charged.
func (c *Cascade) synth(req providers.Request, answer string) <-chan providers.Delta {
	out := make(chan providers.Delta, 4)
	go func() {
		defer close(out)
		out <- providers.Delta{Type: providers.DeltaStart, Provider: "cascade", Model: "rules"}
		if answer != "" {
			out <- providers.Delta{Type: providers.DeltaText, Text: answer, Provider: "cascade", Model: "rules"}
		}
		out <- providers.Delta{
			Type:     providers.DeltaDone,
			Provider: "cascade",
			Model:    "rules",
			Usage:    &providers.Usage{CostUSD: 0},
		}
	}()
	return out
}

// replay streams a cached response back to the caller with zero cost. The
// original provider/model are preserved so telemetry attributes the answer
// to where it actually came from; the Usage cost is zeroed because this hit
// is NOT billed again.
func (c *Cascade) replay(hit CachedResponse) <-chan providers.Delta {
	out := make(chan providers.Delta, 8)
	go func() {
		defer close(out)
		out <- providers.Delta{Type: providers.DeltaStart, Provider: hit.Provider, Model: hit.Model}
		if hit.Thinking != "" {
			out <- providers.Delta{Type: providers.DeltaThinking, Text: hit.Thinking, Provider: hit.Provider, Model: hit.Model}
		}
		if hit.Text != "" {
			out <- providers.Delta{Type: providers.DeltaText, Text: hit.Text, Provider: hit.Provider, Model: hit.Model}
		}
		u := hit.Usage
		u.CostUSD = 0 // cache hit is free — never recharge
		out <- providers.Delta{Type: providers.DeltaDone, Provider: hit.Provider, Model: hit.Model, Usage: &u}
	}()
	return out
}

// tap forwards a delegated provider stream to the caller while observing
// it: assembling the text for the response cache and folding the actual
// charged cost into the aggression controller on DeltaDone. The forward is
// 1:1 and never blocks the provider — backpressure is the caller's, exactly
// as with BillingGuard's own stream tap.
func (c *Cascade) tap(ctx context.Context, req providers.Request, in <-chan providers.Delta) <-chan providers.Delta {
	out := make(chan providers.Delta, 32)
	cacheable := c.cache != nil && c.cache.Eligible(req)
	go func() {
		defer close(out)
		var text, thinking strings.Builder
		for d := range in {
			out <- d
			switch d.Type {
			case providers.DeltaText:
				if cacheable {
					text.WriteString(d.Text)
				}
			case providers.DeltaThinking:
				if cacheable {
					thinking.WriteString(d.Text)
				}
			case providers.DeltaDone:
				if d.Usage != nil {
					c.aggr.RecordSpend(d.Usage.CostUSD)
					if c.budget != nil && strings.TrimSpace(req.TenantID) != "" {
						c.budget.Charge(ctx, BudgetKey{Kind: BudgetWindowDaily, ID: req.TenantID}, decimal.NewFromFloat(d.Usage.CostUSD), "", "")
					}
				}
				if cacheable && text.Len() > 0 {
					resp := CachedResponse{
						Text:     text.String(),
						Thinking: thinking.String(),
						Provider: d.Provider,
						Model:    d.Model,
					}
					if d.Usage != nil {
						resp.Usage = *d.Usage
					}
					c.cache.Put(ctx, req, resp)
				}
			}
		}
	}()
	return out
}
