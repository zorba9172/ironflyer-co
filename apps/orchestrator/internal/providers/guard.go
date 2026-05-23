package providers

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"ironflyer/apps/orchestrator/internal/budget"
	"ironflyer/apps/orchestrator/internal/tracing"
)

// BillingGuard wraps the Router with the budget Enforcer + Ledger. Every
// request is admitted (or downgraded) before the call, and the actual cost
// is charged after the stream ends.
type BillingGuard struct {
	router  *Router
	billing *budget.Billing
	tel     TelemetrySink // optional; nil = no telemetry (still works)
}

func NewBillingGuard(r *Router, b *budget.Billing) *BillingGuard {
	return &BillingGuard{router: r, billing: b}
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
					g.billing.Charge(ctx, userID, "", d.Provider, d.Model,
						d.Usage.InputTokens, d.Usage.OutputTokens,
						d.Usage.CacheReadTokens, d.Usage.CacheCreationTokens,
					)
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
