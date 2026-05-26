package budget

import (
	"context"
	"errors"
	"sync"

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

	// Conversion-watcher state. The enforcer remembers the last
	// usage bracket it observed per user so the upsell hook fires
	// exactly once per bracket transition — not on every Admit call.
	// Read-modify-write under watchMu so concurrent Admits for the
	// same user don't double-fire the hook.
	watchMu       sync.Mutex
	lastBracket   map[string]UsageBracket
	upsellHooks   []UpsellHook
	thresholds    UsageThresholds
}

// UsageBracket is the coarse-grained bucket the enforcer reports the
// user as occupying. Used to drive UI banners ("you're nearing your
// plan cap, upgrade?") and analytics tiles. Ordered low→high so
// callers can compare with `>`.
type UsageBracket int

const (
	BracketLow      UsageBracket = iota // < approachingPct
	BracketApproach                     // approachingPct .. nearCapPct
	BracketNearCap                      // nearCapPct .. exhaustedPct
	BracketExhausted                    // >= exhaustedPct
)

// UsageThresholds tunes the bracket cut-offs. Defaults: 67 / 85 / 100.
// Tighten via env if a tier needs different pressure points.
type UsageThresholds struct {
	ApproachingPct float64
	NearCapPct     float64
	ExhaustedPct   float64
}

// DefaultUsageThresholds matches the proof-pack conversion model —
// 67% is the natural "this is getting expensive" inflection, 85% is
// "show the upgrade dialog by default", 100% is hard cap.
func DefaultUsageThresholds() UsageThresholds {
	return UsageThresholds{
		ApproachingPct: 67,
		NearCapPct:     85,
		ExhaustedPct:   100,
	}
}

// UpsellHook is invoked when a user's usage bracket increases. The
// hook fires AFTER the Admit decision is made so a hook that emits
// telemetry or queues an email never blocks the admission path. Fired
// only on UPWARD transitions; downward (e.g. a billing cycle reset)
// resets the watcher silently.
type UpsellHook func(ctx context.Context, userID string, tier PlanTier, prev, next UsageBracket, usagePct float64)

func NewEnforcer(plans []Plan, l LedgerStore, o *Optimizer) *Enforcer {
	m := make(map[PlanTier]Plan, len(plans))
	for _, p := range plans {
		m[p.Tier] = p
	}
	return &Enforcer{
		Plans:       m,
		Ledger:      l,
		Optim:       o,
		lastBracket: map[string]UsageBracket{},
		thresholds:  DefaultUsageThresholds(),
	}
}

// RegisterUpsellHook adds a callback for usage-bracket transitions.
// Safe to call after the enforcer is constructed — hooks fire on the
// next Admit. nil hooks are silently dropped.
func (e *Enforcer) RegisterUpsellHook(h UpsellHook) {
	if h == nil {
		return
	}
	e.watchMu.Lock()
	defer e.watchMu.Unlock()
	e.upsellHooks = append(e.upsellHooks, h)
}

// SetUsageThresholds overrides the default bracket cut-offs. Useful
// for tiered conversion experiments (a tier with a free-trial period
// may want earlier upsell pressure).
func (e *Enforcer) SetUsageThresholds(t UsageThresholds) {
	e.watchMu.Lock()
	defer e.watchMu.Unlock()
	e.thresholds = t
}

// bracketFor returns the UsageBracket that contains usagePct.
func (e *Enforcer) bracketFor(usagePct float64) UsageBracket {
	t := e.thresholds
	switch {
	case usagePct >= t.ExhaustedPct:
		return BracketExhausted
	case usagePct >= t.NearCapPct:
		return BracketNearCap
	case usagePct >= t.ApproachingPct:
		return BracketApproach
	default:
		return BracketLow
	}
}

// observeUsage compares the user's current bracket against the last
// one we saw and fires hooks on an upward transition. Caller holds
// no locks; observeUsage manages its own state.
func (e *Enforcer) observeUsage(ctx context.Context, userID string, tier PlanTier, usagePct float64) {
	next := e.bracketFor(usagePct)
	e.watchMu.Lock()
	prev, seen := e.lastBracket[userID]
	if seen && next <= prev {
		e.lastBracket[userID] = next
		e.watchMu.Unlock()
		return
	}
	e.lastBracket[userID] = next
	hooks := append([]UpsellHook(nil), e.upsellHooks...)
	e.watchMu.Unlock()
	if !seen || next > prev {
		for _, h := range hooks {
			func(hook UpsellHook) {
				defer func() { _ = recover() }()
				hook(ctx, userID, tier, prev, next, usagePct)
			}(h)
		}
	}
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

	// Fire the usage-bracket watcher BEFORE we return — UI banners
	// + conversion telemetry want to see the bracket transition the
	// moment it happens, not on the next request. Cost-cap zero (e.g.
	// custom Enterprise) collapses to BracketLow so we don't fire
	// noisy "approaching cap" events when there's no cap to approach.
	if plan.CostCapUSD.IsPositive() {
		usagePct, _ := spent.Div(plan.CostCapUSD).Mul(decimal.NewFromInt(100)).Float64()
		e.observeUsage(ctx, userID, tier, usagePct)
	}
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
