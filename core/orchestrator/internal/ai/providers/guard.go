package providers

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"ironflyer/core/orchestrator/internal/business/budget"
	"ironflyer/core/orchestrator/internal/business/lastprovider"
	"ironflyer/core/orchestrator/internal/operations/logctx"
	"ironflyer/core/orchestrator/internal/business/profitguard"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
	"ironflyer/core/orchestrator/internal/operations/tracing"
)

// BillingGuard wraps the Router with the budget Enforcer + Ledger. Every
// request is admitted (or downgraded) before the call, and the actual cost
// is charged after the stream ends.
type BillingGuard struct {
	router  *Router
	billing *budget.Billing
	tel     TelemetrySink // optional; nil = no telemetry (still works)

	// pg is the optional ProfitGuard hook. When wired the guard calls
	// Decide(BeforeModelCall) before every provider call that carries
	// an active execution id on the context (see internal/profitguardctx).
	pg          profitguard.Guard
	pgStateFunc ProfitGuardStateFunc

	// costAttrib is the per-token cost-attribution callback. Fires
	// once per DeltaDone when the call carried an execution id on
	// ctx (see internal/profitguardctx). Lets V22 land
	// provider_inference_cost on the execution row + the ledger
	// without coupling this package to either store directly.
	costAttrib CostAttribFunc

	// logger is the optional zerolog instance used to surface
	// ProfitGuard verdicts that mutate the request (SwitchProvider,
	// Degrade). Nil-safe — set via WithLogger.
	logger    zerolog.Logger
	hasLogger bool
}

// WithLogger attaches a zerolog instance the guard uses to surface
// ProfitGuard verdicts that mutate the request. Returns the guard so
// it chains with NewBillingGuard.
func (g *BillingGuard) WithLogger(l zerolog.Logger) *BillingGuard {
	g.logger = l
	g.hasLogger = true
	return g
}

// CostAttribFunc is the per-token cost-attribution callback wired by
// main.go. The guard calls it once per completed provider stream
// (DeltaDone) when the request carried an executionID on ctx. The
// implementation is expected to be best-effort — it MUST NOT block
// the provider stream and MUST NOT return errors that abort the
// caller (the provider already produced tokens; we just want the
// bookkeeping to land).
type CostAttribFunc func(ctx context.Context, executionID, tenantID, provider, model string, costUSD decimal.Decimal, inTokens, outTokens int, capabilities []string)

// ProfitGuardStateFunc resolves the live execution state into the
// snapshot ProfitGuard.Decide consumes. The integration loop wires
// this from execution.Service + the bridge package. Optional — when
// nil the BillingGuard skips ProfitGuard entirely.
type ProfitGuardStateFunc func(ctx context.Context, executionID string, req Request) (profitguard.ExecState, error)

func NewBillingGuard(r *Router, b *budget.Billing) *BillingGuard {
	return &BillingGuard{router: r, billing: b}
}

// WithProfitGuard wires the ProfitGuard enforcement hook. Returns the
// guard so it chains with NewBillingGuard.
func (g *BillingGuard) WithProfitGuard(pg profitguard.Guard, snapshot ProfitGuardStateFunc) *BillingGuard {
	g.pg = pg
	g.pgStateFunc = snapshot
	return g
}

// WithCostAttribution wires the per-token cost-attribution callback
// V22 uses to land provider_inference_cost on the execution row and
// the ledger. Nil-safe — leaving this unwired skips the per-call
// bookkeeping entirely (existing budget/vault accounting is
// untouched).
func (g *BillingGuard) WithCostAttribution(fn CostAttribFunc) *BillingGuard {
	g.costAttrib = fn
	return g
}

// CompleteStream is the budgeted entry point. The Request.TenantID is used
// as the userID for ledger purposes (rename later when multi-tenant auth lands).
func (g *BillingGuard) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	userID := req.TenantID
	if userID == "" {
		userID = "anonymous"
	}
	required := capsAsStrings(req.Capabilities)

	// Open a billing-guard span around the whole admit + provider call.
	// We end it manually on the synchronous error paths so the span doesn't
	// leak past the function on a budget refusal; the streaming goroutine
	// ends it once the channel drains.
	ctx, span := tracing.StartSpan(ctx, "billing_guard.complete_stream",
		attribute.String("user.id", userID),
		attribute.StringSlice("capabilities", required),
	)

	estIn := estimateTokens(req.System) + estimateTokens(req.ProjectContext) + estimateTokens(req.Prompt)
	estOut := req.MaxTokens
	if estOut == 0 {
		estOut = 2048
	}

	// V22 prompt-cap guard — defense-in-depth shield 0. Refuses any
	// single call whose total estimated tokens would exceed the
	// configured ceiling BEFORE we burn an Admit / ProfitGuard /
	// provider round-trip on it. Catches the runaway prompt loop
	// (full history fed back, then fed back again) at minimum cost.
	// Env-driven; default cap is generous enough that healthy calls
	// pass while pathological prompts trip on iteration 2-3.
	if err := budget.DefaultPromptCap().CheckPromptCap(estIn, estOut); err != nil {
		g.logger.Warn().
			Err(err).
			Str("user_id", userID).
			Int("est_input_tokens", estIn).
			Int("est_output_tokens", estOut).
			Msg("billing-guard: prompt over single-call cap, refused")
		span.RecordError(err)
		span.End()
		return nil, err
	}

	// V22 ProfitGuard hook — BeforeModelCall. Only fires for paid
	// executions (those carrying an execution id on the context).
	// Internal / unmetered call paths skip ProfitGuard entirely so
	// background utilities (audit summarisation, etc.) stay
	// frictionless.
	//
	// Order matters: BeforeModelCall covers stop-loss / wallet
	// exhaustion / margin floor — the cheapest economic checks. When
	// the request also asks for the premium reasoning tier we fire a
	// SECOND Decide at BeforePremiumReasoning AFTER the model-call
	// verdict so the premium-margin floor (typically stricter than
	// the standard-web floor) gets a chance to switch / degrade /
	// kill before we burn Opus tokens.
	if g.pg != nil && g.pgStateFunc != nil {
		if execID, ok := profitguardctx.ExecutionID(ctx); ok {
			if state, err := g.pgStateFunc(ctx, execID, req); err == nil {
				if abort, newReq := g.applyDecision(ctx, span, execID, profitguard.BeforeModelCall, state, req); abort != nil {
					return nil, abort
				} else {
					req = newReq
				}
				if requiresPremiumReasoning(req.Capabilities) {
					if state2, err2 := g.pgStateFunc(ctx, execID, req); err2 == nil {
						if abort, newReq := g.applyDecision(ctx, span, execID, profitguard.BeforePremiumReasoning, state2, req); abort != nil {
							return nil, abort
						} else {
							req = newReq
						}
					}
				}
			}
		}
	}

	decision := g.billing.Admit(ctx, userID, required, estIn, estOut)
	if !decision.Admit {
		err := errors.New("budget: " + decision.Reason)
		span.RecordError(err)
		span.End()
		return nil, err
	}

	// Pick the provider by name to honor the optimizer's choice.
	provider := g.routerPickByName(decision.Provider)
	if provider == nil {
		provider = pickAnyMatching(g.router, req.Capabilities)
	}
	if provider == nil {
		err := errors.New("no provider available")
		span.RecordError(err)
		span.End()
		return nil, err
	}

	started := time.Now().UTC()
	in, err := provider.CompleteStream(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.End()
		if g.tel != nil {
			g.tel.Record(AgentCall{
				UserID: userID, Provider: decision.Provider, Model: decision.Model,
				Capabilities: required, StartedAt: started,
				DurationMS: time.Since(started).Milliseconds(),
				Error:      err.Error(),
			})
		}
		return nil, err
	}

	// Tap the stream: forward everything, charge on DeltaDone, and
	// emit one telemetry record per completed call (success or stream
	// error). The telemetry write is non-blocking; the sink owns its
	// own backpressure.
	out := make(chan Delta, 32)
	go func() {
		defer close(out)
		defer span.End()
		var lastDone *Delta
		var streamErr string
		for d := range in {
			out <- d
			switch d.Type {
			case DeltaDone:
				dCopy := d
				lastDone = &dCopy
				if d.Usage != nil {
					span.SetAttributes(
						attribute.Int("input_tokens", d.Usage.InputTokens),
						attribute.Int("output_tokens", d.Usage.OutputTokens),
						attribute.Float64("cost_usd", d.Usage.CostUSD),
						attribute.String("model", d.Model),
					)
					entry := g.billing.Charge(ctx, userID, "", d.Provider, d.Model,
						d.Usage.InputTokens, d.Usage.OutputTokens,
						d.Usage.CacheReadTokens, d.Usage.CacheCreationTokens,
					)
					g.attributeCost(ctx, d.Provider, d.Model, entry.CostUSD,
						d.Usage.InputTokens, d.Usage.OutputTokens, required)
				}
			case DeltaError:
				if d.Err != nil {
					streamErr = d.Err.Error()
					span.RecordError(d.Err)
				}
			}
		}
		if g.tel == nil {
			return
		}
		row := AgentCall{
			UserID: userID, Capabilities: required,
			StartedAt:  started,
			DurationMS: time.Since(started).Milliseconds(),
			Error:      streamErr,
		}
		if lastDone != nil {
			row.Provider = lastDone.Provider
			row.Model = lastDone.Model
			if lastDone.Usage != nil {
				row.InputTokens = lastDone.Usage.InputTokens
				row.OutputTokens = lastDone.Usage.OutputTokens
				row.CacheReadTokens = lastDone.Usage.CacheReadTokens
				row.CacheNewTokens = lastDone.Usage.CacheCreationTokens
				row.CostUSD = lastDone.Usage.CostUSD
			}
		}
		if row.Provider == "" {
			row.Provider = decision.Provider
		}
		if row.Model == "" {
			row.Model = decision.Model
		}
		g.tel.Record(row)
	}()
	return out, nil
}

// attributeCost is the per-token cost-attribution shim. It looks up
// the active execution / tenant on the context (see
// internal/profitguardctx) and forwards to the wired CostAttribFunc
// when present. Best-effort — wired callbacks must not block or
// error the provider stream.
//
// A31 side-effect: when an executionID is on ctx we also stamp the
// provider+primary-capability into the package-level lastprovider
// tracker so the finisher's recordGateOutcome can attribute the
// next gate verdict back to the provider whose patch produced it.
// This write fires even when g.costAttrib is nil — the bandit
// quality signal must not be gated on cost-attrib being wired.
func (g *BillingGuard) attributeCost(ctx context.Context, provider, model string, costUSD decimal.Decimal, inTokens, outTokens int, capabilities []string) {
	if g == nil {
		return
	}
	execID, ok := profitguardctx.ExecutionID(ctx)
	if !ok {
		return
	}
	// Stamp last provider/capability for this execution so the
	// finisher's QualitySink fan-out can attribute the next gate
	// outcome. Primary capability = first tag on the request (best
	// approximation of "what this call was for").
	if provider != "" {
		var primaryCap string
		if len(capabilities) > 0 {
			primaryCap = capabilities[0]
		}
		lastprovider.Touch(execID, provider, primaryCap)
	}
	if g.costAttrib == nil {
		return
	}
	tenantID, _ := profitguardctx.TenantID(ctx)
	g.costAttrib(ctx, execID, tenantID, provider, model, costUSD, inTokens, outTokens, capabilities)
}

// CompleteStreamWithFailover is the reliability-hardened entry point.
// It admits the call against the budget (just like CompleteStream),
// then drives Router.CompleteStreamWithFailover so a 5xx / 429 /
// network glitch on the bandit's top pick transparently rolls over
// to the next-best provider (up to 2 failovers, 3 total attempts).
// Charging happens on DeltaDone with the provider that actually
// produced tokens — failed attempts emit no DeltaDone and so cost
// the caller nothing.
func (g *BillingGuard) CompleteStreamWithFailover(ctx context.Context, req Request) (<-chan Delta, error) {
	userID := req.TenantID
	if userID == "" {
		userID = "anonymous"
	}
	required := capsAsStrings(req.Capabilities)

	ctx, span := tracing.StartSpan(ctx, "billing_guard.complete_stream_failover",
		attribute.String("user.id", userID),
		attribute.StringSlice("capabilities", required),
	)

	estIn := estimateTokens(req.System) + estimateTokens(req.ProjectContext) + estimateTokens(req.Prompt)
	estOut := req.MaxTokens
	if estOut == 0 {
		estOut = 2048
	}

	// V22 prompt-cap guard — defense-in-depth shield 0. Same contract
	// as CompleteStream: refuse a runaway prompt before paying the
	// Admit / ProfitGuard / provider round-trip on it.
	if err := budget.DefaultPromptCap().CheckPromptCap(estIn, estOut); err != nil {
		g.logger.Warn().
			Err(err).
			Str("user_id", userID).
			Int("est_input_tokens", estIn).
			Int("est_output_tokens", estOut).
			Msg("billing-guard: prompt over single-call cap, refused (failover)")
		span.RecordError(err)
		span.End()
		return nil, err
	}

	// V22 ProfitGuard hook — BeforeModelCall + BeforePremiumReasoning.
	// Mirrors CompleteStream so the failover-hardened path enjoys the
	// same economic gate. Without this hook the agents path (which is
	// driven through CompleteStreamWithFailover by agents.Registry)
	// would bypass ProfitGuard entirely — the single largest hole in
	// V22 coverage prior to this fix.
	if g.pg != nil && g.pgStateFunc != nil {
		if execID, ok := profitguardctx.ExecutionID(ctx); ok {
			if state, err := g.pgStateFunc(ctx, execID, req); err == nil {
				if abort, newReq := g.applyDecision(ctx, span, execID, profitguard.BeforeModelCall, state, req); abort != nil {
					return nil, abort
				} else {
					req = newReq
				}
				if requiresPremiumReasoning(req.Capabilities) {
					if state2, err2 := g.pgStateFunc(ctx, execID, req); err2 == nil {
						if abort, newReq := g.applyDecision(ctx, span, execID, profitguard.BeforePremiumReasoning, state2, req); abort != nil {
							return nil, abort
						} else {
							req = newReq
						}
					}
				}
			}
		}
	}

	decision := g.billing.Admit(ctx, userID, required, estIn, estOut)
	if !decision.Admit {
		err := errors.New("budget: " + decision.Reason)
		span.RecordError(err)
		span.End()
		return nil, err
	}

	started := time.Now().UTC()
	in, err := g.router.CompleteStreamWithFailover(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.End()
		if g.tel != nil {
			g.tel.Record(AgentCall{
				UserID: userID, Provider: decision.Provider, Model: decision.Model,
				Capabilities: required, StartedAt: started,
				DurationMS: time.Since(started).Milliseconds(),
				Error:      err.Error(),
			})
		}
		return nil, err
	}

	out := make(chan Delta, 32)
	go func() {
		defer close(out)
		defer span.End()
		var lastDone *Delta
		var streamErr string
		for d := range in {
			out <- d
			switch d.Type {
			case DeltaDone:
				dCopy := d
				lastDone = &dCopy
				if d.Usage != nil {
					span.SetAttributes(
						attribute.Int("input_tokens", d.Usage.InputTokens),
						attribute.Int("output_tokens", d.Usage.OutputTokens),
						attribute.Float64("cost_usd", d.Usage.CostUSD),
						attribute.String("model", d.Model),
					)
					// Charge ONLY the provider that successfully delivered
					// tokens. The router's failover penalties don't bill;
					// only DeltaDone reaches this branch.
					entry := g.billing.Charge(ctx, userID, "", d.Provider, d.Model,
						d.Usage.InputTokens, d.Usage.OutputTokens,
						d.Usage.CacheReadTokens, d.Usage.CacheCreationTokens,
					)
					g.attributeCost(ctx, d.Provider, d.Model, entry.CostUSD,
						d.Usage.InputTokens, d.Usage.OutputTokens, required)
				}
			case DeltaError:
				if d.Err != nil {
					streamErr = d.Err.Error()
					span.RecordError(d.Err)
				}
			}
		}
		if g.tel == nil {
			return
		}
		row := AgentCall{
			UserID: userID, Capabilities: required,
			StartedAt:  started,
			DurationMS: time.Since(started).Milliseconds(),
			Error:      streamErr,
		}
		if lastDone != nil {
			row.Provider = lastDone.Provider
			row.Model = lastDone.Model
			if lastDone.Usage != nil {
				row.InputTokens = lastDone.Usage.InputTokens
				row.OutputTokens = lastDone.Usage.OutputTokens
				row.CacheReadTokens = lastDone.Usage.CacheReadTokens
				row.CacheNewTokens = lastDone.Usage.CacheCreationTokens
				row.CostUSD = lastDone.Usage.CostUSD
			}
		}
		if row.Provider == "" {
			row.Provider = decision.Provider
		}
		if row.Model == "" {
			row.Model = decision.Model
		}
		g.tel.Record(row)
	}()
	return out, nil
}

// Router exposes the underlying provider router. Operational endpoints
// (status probes, ops dashboards) reach into the router directly to
// enumerate providers without going through the BillingGuard's admit /
// charge path — a status ping must never burn budget on behalf of a
// real user. Production code paths still use CompleteStream so every
// paid call lands in the ledger.
func (g *BillingGuard) Router() *Router { return g.router }

func (g *BillingGuard) routerPickByName(name string) Provider {
	g.router.mu.RLock()
	defer g.router.mu.RUnlock()
	for _, p := range g.router.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

func pickAnyMatching(r *Router, caps []Capability) Provider {
	p, err := r.Pick(caps)
	if err != nil {
		return nil
	}
	return p
}

func capsAsStrings(caps []Capability) []string {
	out := make([]string, len(caps))
	for i, c := range caps {
		out[i] = string(c)
	}
	return out
}

// estimateTokens is a crude ~4-chars-per-token heuristic, fine for budgeting.
func estimateTokens(s string) int {
	return len(s) / 4
}

// applyDecision runs Decide + Record at one enforcement point and
// folds the verdict into `req`. Returns (nil, mutated req) when the
// call may proceed; returns (err, _) when the verdict aborts the call
// (Stop / KillBranch / PauseForBudget). The span is closed on abort
// so the caller can return early without leaking the trace.
func (g *BillingGuard) applyDecision(ctx context.Context, span trace.Span, executionID string, point profitguard.EnforcementPoint, state profitguard.ExecState, req Request) (error, Request) {
	if g == nil || g.pg == nil {
		return nil, req
	}
	dec, derr := g.pg.Decide(ctx, point, state)
	if derr != nil {
		return nil, req
	}
	_ = g.pg.Record(ctx, executionID, point, dec, state)

	switch dec.Action {
	case profitguard.Stop, profitguard.KillBranch, profitguard.PauseForBudget:
		err := errors.New("profitguard: " + string(dec.Action) + ": " + dec.Reason)
		span.RecordError(err)
		span.End()
		return err, req

	case profitguard.SwitchProvider:
		// Enforce the swap by stamping the recommended provider on the
		// request. The router honours Request.PreferredProvider in
		// CompleteStream + PickByName, so the next pick lands on the
		// cheaper provider regardless of the bandit's incumbent. If
		// the recommended provider is unknown / lacks capabilities,
		// the router logs Warn and falls back — no abort.
		if dec.RecommendedProvider != "" && dec.RecommendedProvider != req.PreferredProvider {
			prev := req.PreferredProvider
			req.PreferredProvider = dec.RecommendedProvider
			if g.hasLogger {
				dec2 := logctx.Decorate(ctx, g.logger)
				dec2.Info().
					Str("execution_id", executionID).
					Str("enforcement_point", string(point)).
					Str("from_provider", first(prev, state.CurrentProvider)).
					Str("to_provider", dec.RecommendedProvider).
					Str("reason", dec.Reason).
					Msg("profitguard switched provider")
			}
		}

	case profitguard.Degrade:
		// Strip the premium-tier capabilities so PickChain rolls onto
		// the cheaper tier. Applies to both BeforeModelCall and
		// BeforePremiumReasoning so the second pass can still degrade
		// even if the first one continued unchanged.
		req.Capabilities = stripCap(req.Capabilities, CapQuality)
		req.Capabilities = stripCap(req.Capabilities, CapReasoning)
		req.Capabilities = stripCap(req.Capabilities, CapThinking)
	}
	return nil, req
}

// requiresPremiumReasoning returns true when the request's capability
// tags include any of the tiers that map to the premium model family
// (Opus / o3 / Gemini Pro). Mirrors the policy's WorkloadPremiumReasoning
// classifier so the guard fires the second Decide pass only for
// requests that would actually burn premium tokens.
func requiresPremiumReasoning(caps []Capability) bool {
	for _, c := range caps {
		switch c {
		case CapQuality, CapThinking, CapReasoning:
			return true
		}
	}
	return false
}

// first returns the first non-empty string of its arguments. Used by
// applyDecision so the "from_provider" log field falls back from the
// previously stamped PreferredProvider to the snapshot's
// CurrentProvider when the request had no prior preference.
func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
