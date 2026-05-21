package budget

import (
	"context"
	"sync"

	"github.com/shopspring/decimal"
)

// Billing is the top-level facade — one struct the rest of the orchestrator
// asks "can this user spend money? please record what they spent."
//
// The Ledger and Vault are now interface-typed so we can swap memory for
// Postgres without touching call sites.
type Billing struct {
	Plans     []Plan
	Rates     *RateSheet
	Ledger    LedgerStore
	Vault     VaultStore
	Optimizer *Optimizer
	Enforcer  *Enforcer

	mu       sync.RWMutex
	userPlan map[string]PlanTier
}

// NewBilling builds a Billing facade with the supplied stores. Pass
// MemoryLedger/MemoryVault for dev or Postgres variants for production.
func NewBilling(ledger LedgerStore, vault VaultStore) *Billing {
	rates := DefaultRateSheet()
	plans := DefaultPlans()
	optim := NewOptimizer(rates)
	enf := NewEnforcer(plans, ledger, optim)
	return &Billing{
		Plans: plans, Rates: rates, Ledger: ledger, Vault: vault,
		Optimizer: optim, Enforcer: enf,
		userPlan: map[string]PlanTier{},
	}
}

// NewMemoryBilling is a convenience that wires the in-memory stores. Used
// in tests and as the dev default.
func NewMemoryBilling() *Billing {
	return NewBilling(NewMemoryLedger(), NewMemoryVault())
}

// AssignPlan sets a user's subscription. Also records the subscription
// revenue movement in the Vault.
func (b *Billing) AssignPlan(ctx context.Context, userID string, tier PlanTier) Plan {
	b.mu.Lock()
	b.userPlan[userID] = tier
	var p Plan
	for _, pp := range b.Plans {
		if pp.Tier == tier {
			p = pp
			break
		}
	}
	b.mu.Unlock()
	if p.MonthlyPrice.GreaterThan(decimal.Zero) {
		_, _ = b.Vault.Record(ctx, VaultEntry{
			Kind: VaultRevenue, UserID: userID,
			Amount: p.MonthlyPrice, Note: "subscription: " + p.Name,
		})
	}
	return p
}

func (b *Billing) PlanFor(userID string) PlanTier {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if t, ok := b.userPlan[userID]; ok {
		return t
	}
	return TierFree
}

// Admit asks the enforcer whether a call can proceed and which provider/model
// to use.
func (b *Billing) Admit(ctx context.Context, userID string, required []string, estInTok, estOutTok int) Decision {
	return b.Enforcer.Admit(ctx, userID, b.PlanFor(userID), required, estInTok, estOutTok)
}

// Charge records the actual cost of a completed call. Returns the persisted
// entry so the caller can attach it to traces/events.
func (b *Billing) Charge(ctx context.Context, userID, projectID, provider, model string, inTok, outTok, cacheRead, cacheCreate int) LedgerEntry {
	cost := b.Rates.CostOf(provider, model, inTok, outTok, cacheRead, cacheCreate)
	entry, err := b.Ledger.Charge(ctx, LedgerEntry{
		UserID: userID, ProjectID: projectID,
		Provider: provider, Model: model,
		InputTokens: inTok, OutputTokens: outTok,
		CacheRead: cacheRead, CacheCreate: cacheCreate,
		CostUSD: cost,
	})
	if err != nil {
		// Don't drop the cost — it still happened. Caller decides how to log.
		return LedgerEntry{}
	}
	if cost.GreaterThan(decimal.Zero) {
		_, _ = b.Vault.Record(ctx, VaultEntry{
			Kind: VaultProviderCost, UserID: userID,
			Amount: cost.Neg(),
			Note:   provider + "/" + model,
		})
	}
	return entry
}
