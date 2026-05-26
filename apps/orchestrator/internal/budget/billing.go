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

	// Metered is the Stripe usage-record reporter. Optional — when nil or
	// disabled, Charge stays a pure-ledger operation and no overage is sent
	// upstream. Wired in cmd/orchestrator/main.go once Stripe + plan
	// metered price IDs are loaded.
	Metered  *MeteredReporter
	SubItems SubscriptionItemStore
	PayFlags PaymentFlagStore

	mu       sync.RWMutex
	userPlan map[string]PlanTier

	// planChangeHooks fire after a plan transition lands. Wired by the
	// orchestrator's wireup layer so we can land a typed
	// EntryFreeToPaidConversion in the internal/ledger without this
	// package taking a dependency on it. Empty slice = no-op (dev /
	// tests). Read under mu.
	planChangeHooks []PlanChangeHook
}

// PlanChangeHook receives every successful plan flip. previous is
// the tier the user had before the call (TierFree on first signup);
// next is what they have now. plan is the canonical Plan that was
// assigned so subscribers can read the monthly price without a
// secondary lookup.
type PlanChangeHook func(ctx context.Context, userID string, previous, next PlanTier, plan Plan)

// NewBilling builds a Billing facade with the supplied stores. Pass
// MemoryLedger/MemoryVault for dev or Postgres variants for production.
func NewBilling(ledger LedgerStore, vault VaultStore) *Billing {
	rates := DefaultRateSheet()
	// Apply env-declared volume discounts BEFORE the optimizer reads
	// the rate sheet so cheaper-per-call ranking uses the negotiated
	// price, not list. The discount map is returned for the wireup
	// layer to log; nil-safe on the empty case.
	_ = rates.ApplyVolumeDiscounts()
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
// revenue movement in the Vault and fires every registered
// PlanChangeHook so downstream subsystems (ledger conversion entries,
// analytics, email triggers) can react without this package taking a
// direct dependency on them.
func (b *Billing) AssignPlan(ctx context.Context, userID string, tier PlanTier) Plan {
	b.mu.Lock()
	previous, hadPrevious := b.userPlan[userID]
	if !hadPrevious {
		previous = TierFree
	}
	b.userPlan[userID] = tier
	var p Plan
	for _, pp := range b.Plans {
		if pp.Tier == tier {
			p = pp
			break
		}
	}
	hooks := append([]PlanChangeHook(nil), b.planChangeHooks...)
	b.mu.Unlock()
	if p.MonthlyPrice.GreaterThan(decimal.Zero) {
		_, _ = b.Vault.Record(ctx, VaultEntry{
			Kind: VaultRevenue, UserID: userID,
			Amount: p.MonthlyPrice, Note: "subscription: " + p.Name,
		})
	}
	if previous != tier {
		for _, hook := range hooks {
			func(h PlanChangeHook) {
				defer func() { _ = recover() }()
				h(ctx, userID, previous, tier, p)
			}(hook)
		}
	}
	return p
}

// RegisterPlanChangeHook adds a fire-and-forget callback invoked on
// every plan flip. Hooks run synchronously inside AssignPlan after
// the userPlan map is updated and (when applicable) the Vault entry
// is recorded; a panic in a hook is recovered so one bad subscriber
// can't break subscription updates for everyone else.
func (b *Billing) RegisterPlanChangeHook(hook PlanChangeHook) {
	if hook == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.planChangeHooks = append(b.planChangeHooks, hook)
}

func (b *Billing) PlanFor(userID string) PlanTier {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if t, ok := b.userPlan[userID]; ok {
		return t
	}
	return TierFree
}

// PlanByTier returns the Plan whose Tier matches t, or false. Plan
// definitions are loaded once at startup so this is a cheap slice
// scan; the GraphQL dataloader uses it to batch User.plan projection
// lookups.
func (b *Billing) PlanByTier(t PlanTier) (Plan, bool) {
	if b == nil {
		return Plan{}, false
	}
	for _, p := range b.Plans {
		if p.Tier == t {
			return p, true
		}
	}
	return Plan{}, false
}

// PlansByTiers batch-fetches plans by tier id. Tiers without a
// matching Plan are omitted from the result so the dataloader can
// surface that absence per key.
func (b *Billing) PlansByTiers(tiers []PlanTier) map[PlanTier]Plan {
	out := make(map[PlanTier]Plan, len(tiers))
	if b == nil {
		return out
	}
	for _, t := range tiers {
		if p, ok := b.PlanByTier(t); ok {
			out[t] = p
		}
	}
	return out
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
	// Forward the cost to the metered reporter so users on paid plans pay
	// per-call for spend above their CostCapUSD. Skip when:
	//   * Free tier — HardStop already blocked the call before we got here,
	//     so anything that landed in the ledger was on a paid plan, but
	//     defensive guard avoids surprising the user if HardStop is ever
	//     loosened.
	//   * Reporter disabled (self-hosted / IRONFLYER_METERED_DISABLED).
	//   * No subscription item known for the user — webhook hasn't fired yet.
	if b.Metered != nil && b.Metered.Enabled() && cost.IsPositive() {
		tier := b.PlanFor(userID)
		if tier != TierFree {
			b.Metered.Record(ctx, MeteredEvent{
				UserID:    userID,
				ProjectID: projectID,
				Model:     model,
				InTokens:  inTok,
				OutTokens: outTok,
				CostUSD:   cost,
				At:        entry.CreatedAt,
			})
		}
	}
	return entry
}
