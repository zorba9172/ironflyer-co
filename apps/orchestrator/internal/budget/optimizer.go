package budget

import (
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// CallEstimate is the projected cost of a single call.
type CallEstimate struct {
	Provider   string
	Model      string
	EstTokensIn  int
	EstTokensOut int
	EstUSD     decimal.Decimal
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
func (o *Optimizer) Pick(required []string, plan Plan, estInTok, estOutTok int) (CallEstimate, bool) {
	allow := map[string]bool{}
	for _, a := range plan.AllowList {
		allow[a] = true
	}
	block := map[string]bool{}
	for _, b := range plan.BlockList {
		block[b] = true
	}

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
