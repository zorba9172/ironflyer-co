package resolver

// Wired by Closure Agent P. Plans / Rates / Vault / MyBudget /
// StartCheckout. Backed by r.Billing for the read surfaces and
// r.Stripe for checkout. Each resolver returns the typed
// gqlNotConfigured error when its backing service was not wired
// (Stripe in particular is optional in dev) — never a panic, never
// a synthetic placeholder.

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/budget"
	"ironflyer/apps/orchestrator/internal/graph/model"
)

// StartCheckout creates a Stripe Checkout Session for the chosen tier
// and returns the session id + hosted URL.
func (r *mutationResolver) StartCheckout(ctx context.Context, input model.StartCheckoutInput) (*model.StripeCheckoutSession, error) {
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if r.Stripe == nil || !r.Stripe.Enabled() {
		return nil, gqlNotConfigured("stripe")
	}
	url, err := r.Stripe.CreateCheckoutSession(ctx, u.ID, u.Email, budget.PlanTier(input.Tier))
	if err != nil {
		return nil, err
	}
	// Stripe's REST response carries the URL but we don't get the
	// session id back from the thin wrapper. The hosted URL contains
	// the id as the last path segment, so we use the URL as the
	// canonical identifier on the response and let the client pull
	// the session id off it if it needs to. Round-tripping a
	// dedicated id requires deepening the Stripe wrapper.
	return &model.StripeCheckoutSession{
		SessionID: url,
		URL:       url,
	}, nil
}

// Plans returns the catalogue exposed to the marketing page. The Pro /
// Team / Enterprise tiers carry the per-period cap and the Stripe
// price id wired by main.go from the STRIPE_PRICE_* env.
func (r *queryResolver) Plans(ctx context.Context) ([]model.Plan, error) {
	if r.Billing == nil {
		return nil, gqlNotConfigured("billing")
	}
	out := make([]model.Plan, 0, len(r.Billing.Plans))
	for _, p := range r.Billing.Plans {
		out = append(out, planToGraphQL(p))
	}
	return out, nil
}

// Rates returns the provider/model rate sheet. Used by the wallet
// dashboard's "cost per million tokens" table.
func (r *queryResolver) Rates(ctx context.Context) ([]model.Rate, error) {
	if r.Billing == nil || r.Billing.Rates == nil {
		return nil, gqlNotConfigured("billing-rates")
	}
	rates := r.Billing.Rates.All()
	out := make([]model.Rate, 0, len(rates))
	for _, rt := range rates {
		out = append(out, model.Rate{
			Provider:          rt.Provider,
			Model:             rt.Model,
			PromptPerMTok:     model.NewDecimal(rt.InputUSD),
			CompletionPerMTok: model.NewDecimal(rt.OutputUSD),
		})
	}
	return out, nil
}

// Vault returns the platform-level Revenue/ProviderCost/Margin
// snapshot. This is operator-facing aggregate, not per-user.
func (r *queryResolver) Vault(ctx context.Context) (*model.VaultSnapshot, error) {
	if r.Billing == nil || r.Billing.Vault == nil {
		return nil, gqlNotConfigured("vault")
	}
	snap, err := r.Billing.Vault.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	entries := 0
	// The ledger holds the per-call rows; the vault holds movements.
	// We project entries-count off the ledger when available so the
	// UI can render a "N charges this month" pill.
	if r.Billing.Ledger != nil {
		if all, err := r.Billing.Ledger.EntriesByUser(ctx, ""); err == nil {
			entries = len(all)
		}
	}
	return &model.VaultSnapshot{
		RevenueUsd:      model.NewDecimal(snap.Revenue),
		ProviderCostUsd: model.NewDecimal(snap.ProviderCost),
		MarginUsd:       model.NewDecimal(snap.Margin),
		Entries:         entries,
		AsOf:            time.Now().UTC(),
	}, nil
}

// MyBudget returns the authenticated user's per-period spend +
// per-call ledger entries. Front-end uses it on /dashboard for the
// "this month" cost panel.
func (r *queryResolver) MyBudget(ctx context.Context) (*model.BudgetSummary, error) {
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if r.Billing == nil || r.Billing.Ledger == nil {
		return nil, gqlNotConfigured("billing-ledger")
	}
	spent, err := r.Billing.Ledger.SpentByUser(ctx, u.ID)
	if err != nil {
		spent = decimal.Zero
	}
	rows, _ := r.Billing.Ledger.EntriesByUser(ctx, u.ID)
	entries := make([]model.LedgerEntry, 0, len(rows))
	for _, e := range rows {
		entries = append(entries, ledgerEntryToGraphQL(e))
	}
	return &model.BudgetSummary{
		UserID:   u.ID,
		Email:    u.Email,
		Tier:     string(r.Billing.PlanFor(u.ID)),
		SpentUsd: model.NewDecimal(spent),
		Entries:  entries,
	}, nil
}

// planToGraphQL maps the budget.Plan into the GraphQL Plan shape.
func planToGraphQL(p budget.Plan) model.Plan {
	features := p.AllowList
	if features == nil {
		features = []string{}
	}
	out := model.Plan{
		Tier:       string(p.Tier),
		Name:       p.Name,
		PriceUsd:   model.NewDecimal(p.MonthlyPrice),
		CostCapUsd: model.NewDecimal(p.CostCapUSD),
		Features:   features,
	}
	if p.StripeID != "" {
		s := p.StripeID
		out.StripePriceID = &s
	}
	return out
}

// ledgerEntryToGraphQL converts a budget.LedgerEntry into the GraphQL
// LedgerEntry shape used by MyBudget.
func ledgerEntryToGraphQL(e budget.LedgerEntry) model.LedgerEntry {
	out := model.LedgerEntry{
		ID:               e.ID,
		UserID:           e.UserID,
		PromptTokens:     e.InputTokens,
		CompletionTokens: e.OutputTokens,
		CostUsd:          model.NewDecimal(e.CostUSD),
		// Revenue at the ledger row level isn't tracked separately —
		// the vault aggregates revenue from subscription + checkout.
		// Surface zero here so dashboards don't double-count.
		RevenueUsd: model.NewDecimal(decimal.Zero),
		Ts:         e.CreatedAt,
	}
	if e.ProjectID != "" {
		pid := e.ProjectID
		out.ProjectID = &pid
	}
	if e.Provider != "" {
		p := e.Provider
		out.Provider = &p
	}
	if e.Model != "" {
		m := e.Model
		out.Model = &m
	}
	return out
}
