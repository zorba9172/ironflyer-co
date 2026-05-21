package budget

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

var (
	ErrOverBudget = errors.New("budget exhausted: subscription cost cap reached")
	ErrNotAllowed = errors.New("provider not allowed for this plan")
)

// Decision is what the Enforcer returns when asked to admit a call.
type Decision struct {
	Admit        bool            `json:"admit"`
	Provider     string          `json:"provider"`
	Model        string          `json:"model"`
	Reason       string          `json:"reason,omitempty"`
	Downgraded   bool            `json:"downgraded,omitempty"`
	RemainingUSD decimal.Decimal `json:"remainingUSD"`
}

// Enforcer wires the Plan + Ledger + Optimizer into a single admission gate.
type Enforcer struct {
	Plans  map[PlanTier]Plan
	Ledger LedgerStore
	Optim  *Optimizer
}

func NewEnforcer(plans []Plan, l LedgerStore, o *Optimizer) *Enforcer {
	m := make(map[PlanTier]Plan, len(plans))
	for _, p := range plans {
		m[p.Tier] = p
	}
	return &Enforcer{Plans: m, Ledger: l, Optim: o}
}

// Admit checks budget, picks the best provider, and returns a Decision.
func (e *Enforcer) Admit(ctx context.Context, userID string, tier PlanTier, required []string, estInTok, estOutTok int) Decision {
	plan, ok := e.Plans[tier]
	if !ok {
		plan = e.Plans[TierFree]
	}
	pick, found := e.Optim.Pick(required, plan, estInTok, estOutTok)
	if !found {
		return Decision{Admit: false, Reason: "no provider matches required capabilities"}
	}

	spent, err := e.Ledger.SpentByUser(ctx, userID)
	if err != nil {
		spent = decimal.Zero
	}
	remaining := plan.CostCapUSD.Sub(spent)
	if remaining.LessThanOrEqual(decimal.Zero) {
		if plan.HardStop {
			return Decision{Admit: false, Reason: ErrOverBudget.Error(), RemainingUSD: remaining}
		}
		downgrade, ok := e.Optim.Pick([]string{"cheap"}, plan, estInTok, estOutTok)
		if !ok {
			return Decision{Admit: false, Reason: ErrOverBudget.Error(), RemainingUSD: remaining}
		}
		return Decision{Admit: true, Provider: downgrade.Provider, Model: downgrade.Model,
			Downgraded: true, Reason: "budget exhausted — downgraded", RemainingUSD: remaining}
	}

	if pick.EstUSD.GreaterThan(remaining) && !plan.HardStop {
		downgrade, ok := e.Optim.Pick([]string{"cheap"}, plan, estInTok, estOutTok)
		if ok && downgrade.EstUSD.LessThanOrEqual(remaining) {
			return Decision{Admit: true, Provider: downgrade.Provider, Model: downgrade.Model,
				Downgraded: true, Reason: "single call would overshoot — downgraded", RemainingUSD: remaining}
		}
	}

	return Decision{Admit: true, Provider: pick.Provider, Model: pick.Model, RemainingUSD: remaining}
}
