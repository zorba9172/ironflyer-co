package shippass

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// Tier is one row in the static price catalogue. Pricing decisions
// live here and nowhere else — resolvers read TierByKey, never invent.
type Tier struct {
	Key           string
	Label         string
	PriceUSD      decimal.Decimal
	RequiredGates []domain.GateName
	DeadlineDays  int
	Description   string
}

// Deadline returns the absolute deadline for a pass purchased now at
// this tier.
func (t Tier) Deadline(now time.Time) time.Time {
	return now.Add(time.Duration(t.DeadlineDays) * 24 * time.Hour)
}

// catalogue is the source of truth. Order matters — resolvers render
// tiers top-to-bottom in this exact sequence so cheap-first reads as
// "MVP → Pro → Production".
//
// Gate scopes deliberately escalate: every higher tier is a strict
// superset of the previous so the buyer sees an obvious "what does
// the upgrade actually buy me" diff.
var catalogue = []Tier{
	{
		Key:      "ship-pass-mvp",
		Label:    "Ship Pass — MVP",
		PriceUSD: decimal.NewFromInt(99),
		RequiredGates: []domain.GateName{
			domain.GateSpec,
			domain.GateCode,
			domain.GateLint,
			domain.GateSecurity,
			domain.GateBudget,
			domain.GateDeploy,
		},
		DeadlineDays: 14,
		Description:  "Demo-ready: build clean, lint clean, security clean, deploys to a preview URL. Charged only if every gate passes within 14 days.",
	},
	{
		Key:      "ship-pass-pro",
		Label:    "Ship Pass — Pro",
		PriceUSD: decimal.NewFromInt(199),
		RequiredGates: []domain.GateName{
			domain.GateSpec,
			domain.GateArch,
			domain.GateUX,
			domain.GateCode,
			domain.GateLint,
			domain.GateSecurity,
			domain.GateBudget,
			domain.GateDeploy,
			domain.GateVerifier,
			domain.GateLighthouse,
		},
		DeadlineDays: 21,
		Description:  "Production-grade: adds visual proof gate (Verifier) and Lighthouse quality floor on the live preview. Charged only on a complete pass set.",
	},
	{
		Key:      "ship-pass-prod",
		Label:    "Ship Pass — Production",
		PriceUSD: decimal.NewFromInt(499),
		RequiredGates: []domain.GateName{
			domain.GateSpec,
			domain.GateArch,
			domain.GateUX,
			domain.GateCode,
			domain.GateLint,
			domain.GateSecurity,
			domain.GateBudget,
			domain.GateDeploy,
			domain.GateVerifier,
			domain.GateLighthouse,
			domain.GateMobileSecurity,
			domain.GateMobileBuild,
		},
		DeadlineDays: 30,
		Description:  "Multi-surface launch: web Pro scope plus mobile build + mobile security gates. Charged only when web and mobile both clear.",
	},
}

// Tiers returns a fresh copy of the catalogue in canonical order.
// Resolvers fan this out as a GraphQL list so the frontend never has
// to hardcode tier metadata.
func Tiers() []Tier {
	out := make([]Tier, len(catalogue))
	copy(out, catalogue)
	return out
}

// TierByKey returns the tier matching key or false. Comparisons are
// case-sensitive — keys are wire identifiers and stable.
func TierByKey(key string) (Tier, bool) {
	for _, t := range catalogue {
		if t.Key == key {
			return t, true
		}
	}
	return Tier{}, false
}

// requiredGateSet returns a set-shaped view of the tier's required
// gates so the Settler can ask "is gate X part of this scope?" in
// O(1) on every verdict ingest.
func (t Tier) requiredGateSet() map[domain.GateName]struct{} {
	out := make(map[domain.GateName]struct{}, len(t.RequiredGates))
	for _, g := range t.RequiredGates {
		out[g] = struct{}{}
	}
	return out
}

// sortedGates returns the tier's required gates in stable
// alphabetical order. Used by the resolver render so the UI never
// reshuffles the chip row between renders.
func (t Tier) sortedGates() []domain.GateName {
	out := make([]domain.GateName, len(t.RequiredGates))
	copy(out, t.RequiredGates)
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}
