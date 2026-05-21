package providers

import (
	"context"
	"errors"

	"ironflyer/apps/orchestrator/internal/budget"
)

// BillingGuard wraps the Router with the budget Enforcer + Ledger. Every
// request is admitted (or downgraded) before the call, and the actual cost
// is charged after the stream ends.
type BillingGuard struct {
	router  *Router
	billing *budget.Billing
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
	estIn := estimateTokens(req.System) + estimateTokens(req.ProjectContext) + estimateTokens(req.Prompt)
	estOut := req.MaxTokens
	if estOut == 0 {
		estOut = 2048
	}
	decision := g.billing.Admit(ctx, userID, required, estIn, estOut)
	if !decision.Admit {
		return nil, errors.New("budget: " + decision.Reason)
	}

	// Pick the provider by name to honor the optimizer's choice.
	provider := g.routerPickByName(decision.Provider)
	if provider == nil {
		provider = pickAnyMatching(g.router, req.Capabilities)
	}
	if provider == nil {
		return nil, errors.New("no provider available")
	}

	in, err := provider.CompleteStream(ctx, req)
	if err != nil {
		return nil, err
	}

	// Tap the stream: forward everything, and on DeltaDone record the charge.
	out := make(chan Delta, 32)
	go func() {
		defer close(out)
		for d := range in {
			out <- d
			if d.Type == DeltaDone && d.Usage != nil {
				g.billing.Charge(ctx, userID, "", d.Provider, d.Model,
					d.Usage.InputTokens, d.Usage.OutputTokens,
					d.Usage.CacheReadTokens, d.Usage.CacheCreationTokens,
				)
			}
		}
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
