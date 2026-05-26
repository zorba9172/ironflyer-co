package budget

import (
	"os"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// CallEstimate is the projected cost of a single call.
type CallEstimate struct {
	Provider     string
	Model        string
	EstTokensIn  int
	EstTokensOut int
	EstUSD       decimal.Decimal
}

// Optimizer picks the cheapest provider/model that satisfies the required
// capability tags AND respects the user's remaining budget.
type Optimizer struct {
	Rates *RateSheet
}

func NewOptimizer(rs *RateSheet) *Optimizer { return &Optimizer{Rates: rs} }

// Pick returns the candidate with lowest projected cost that has all the
// required capabilities. AllowList from the user's Plan, if non-empty,
// filters providers.
//
// Aggressive routing (opt-in via IRONFLYER_AGGRESSIVE_ROUTING=1):
// for SMALL calls (estInTok + estOutTok < threshold, default 1500)
// that asked for `reasoning` or `thinking`, retry the candidate scan
// AFTER stripping those caps. If the cheap-tier match costs at most
// half of the reasoning-tier match, return it. Rationale: short
// prompts rarely need a frontier model's depth — a 1k-token "rename
// these symbols" call doesn't justify Sonnet when Haiku nails it at
// 1/3 the price. The 0.5× ratio prevents the shortcut from picking
// a model that's only marginally cheaper (and would risk quality).
func (o *Optimizer) Pick(required []string, plan Plan, estInTok, estOutTok int) (CallEstimate, bool) {
	allow := map[string]bool{}
	for _, a := range plan.AllowList {
		allow[a] = true
	}
	block := map[string]bool{}
	for _, b := range plan.BlockList {
		block[b] = true
	}

	best, ok := pickRaw(o, required, allow, block, estInTok, estOutTok)
	if !ok {
		return CallEstimate{}, false
	}

	// Aggressive small-call shortcut. Only kicks in when the env
	// switch is on AND the call is short AND the caller asked for
	// reasoning/thinking — those are the asks most likely to be
	// pulling Sonnet when Haiku would have done.
	if isAggressiveRoutingOn() && (estInTok+estOutTok) < aggressiveSmallCallTokens() && requiresReasoning(required) {
		stripped := stripReasoningCaps(required)
		if len(stripped) < len(required) {
			cheap, ok := pickRaw(o, stripped, allow, block, estInTok, estOutTok)
			// Half-or-better ratio to avoid downgrading on a marginal
			// saving. decimal-safe: multiply rather than divide.
			if ok && cheap.EstUSD.Mul(decimal.NewFromInt(2)).LessThanOrEqual(best.EstUSD) {
				return cheap, true
			}
		}
	}
	return best, true
}

// pickRaw is the inner candidate scan shared by Pick and the
// aggressive small-call shortcut. Pure function of the rate sheet
// snapshot and the supplied filters — no env reads here.
func pickRaw(o *Optimizer, required []string, allow, block map[string]bool, estInTok, estOutTok int) (CallEstimate, bool) {
	var candidates []CallEstimate
	for _, r := range o.Rates.All() {
		if len(allow) > 0 && !allow[r.Provider] {
			continue
		}
		if block[r.Provider] {
			continue
		}
		if !hasAll(r.Capability, required) {
			continue
		}
		cost := o.Rates.CostOf(r.Provider, r.Model, estInTok, estOutTok, 0, 0)
		candidates = append(candidates, CallEstimate{
			Provider: r.Provider, Model: r.Model,
			EstTokensIn: estInTok, EstTokensOut: estOutTok, EstUSD: cost,
		})
	}
	if len(candidates) == 0 {
		return CallEstimate{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].EstUSD.LessThan(candidates[j].EstUSD)
	})
	return candidates[0], true
}

func isAggressiveRoutingOn() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("IRONFLYER_AGGRESSIVE_ROUTING")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func aggressiveSmallCallTokens() int {
	raw := strings.TrimSpace(os.Getenv("IRONFLYER_AGGRESSIVE_SMALL_CALL_TOKENS"))
	if raw == "" {
		return 1500
	}
	var n int
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 1500
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return 1500
	}
	return n
}

func requiresReasoning(required []string) bool {
	for _, c := range required {
		switch strings.ToLower(c) {
		case "reasoning", "thinking":
			return true
		}
	}
	return false
}

func stripReasoningCaps(required []string) []string {
	out := make([]string, 0, len(required))
	for _, c := range required {
		switch strings.ToLower(c) {
		case "reasoning", "thinking":
			continue
		default:
			out = append(out, c)
		}
	}
	return out
}

func hasAll(have, want []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, c := range have {
		set[strings.ToLower(c)] = struct{}{}
	}
	for _, c := range want {
		if _, ok := set[strings.ToLower(c)]; !ok {
			return false
		}
	}
	return true
}
