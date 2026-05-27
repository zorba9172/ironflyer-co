package sentinel

import (
	"context"

	"github.com/shopspring/decimal"
)

// ProjectContext is the slice of project state the suggestion engine
// needs to pick reroutes. The adapter shape lets the wireup decide
// where these values come from (orchestrator state, blueprint
// registry, mobile spec, etc.) without sentinel taking a dependency
// on every upstream package.
type ProjectContext struct {
	CurrentModelTier string // "opus" | "sonnet" | "haiku"
	HasMobileTarget  bool
	HasTemplateMatch bool
	ReferenceBytes   int     // attached PDFs / images / pasted blobs total
	RecentRetryRate  float64 // [0, 1]; high retries → cheaper model less risky
}

// ProjectContextLoader resolves a ProjectContext for the wireup.
type ProjectContextLoader interface {
	Load(ctx context.Context, tenant, projectID string) (ProjectContext, error)
}

// SuggestionEngine emits reroute recommendations for a project. It
// reads the current Forecast (so it can skip low-value suggestions
// on a green project) plus the ProjectContext (so it can skip
// suggestions that do not apply — e.g., "skip mobile" on a web-only
// project).
type SuggestionEngine struct {
	loader ProjectContextLoader
}

// NewSuggestionEngine wires the engine. A nil loader is legal — the
// engine then returns only the suggestions that do not require
// project context (currently: smaller-context, when ReferenceBytes
// is known to be non-zero — which it never is without the loader, so
// the engine effectively no-ops).
func NewSuggestionEngine(loader ProjectContextLoader) *SuggestionEngine {
	return &SuggestionEngine{loader: loader}
}

// Suggest returns the reroute options ranked by SavingsUSD desc.
// The engine deliberately returns an empty list on green forecasts
// so the dashboard renders nothing in the "Sentinel suggestions"
// panel — there is no point distracting a buyer whose project is on
// track.
func (e *SuggestionEngine) Suggest(ctx context.Context, tenant, projectID string, forecast Forecast) ([]Reroute, error) {
	if forecast.Level == WarnGreen {
		return nil, nil
	}
	var pctx ProjectContext
	if e.loader != nil {
		p, err := e.loader.Load(ctx, tenant, projectID)
		if err != nil {
			return nil, err
		}
		pctx = p
	}

	out := []Reroute{}

	// Cheaper-model is the universal reroute. Savings estimate is
	// based on the gross cost delta between the current tier and the
	// next-cheaper tier, applied to ExtrapolatedTotal - SpentUSD
	// (the unspent runway only).
	if delta, ok := cheaperModelSavings(pctx.CurrentModelTier, forecast); ok {
		out = append(out, Reroute{
			Kind:              RerouteCheaperModel,
			Label:             "Drop one model tier",
			Description:       "Switch the active reasoning model down one rung. Retries on the cheaper tier cost roughly 30% of the current rung; we keep premium reasoning available for the next blocker.",
			SavingsUSD:        delta,
			SavingsConfidence: 0.7,
			Reversible:        true,
		})
	}

	// Template swap is high-impact when a match is registered.
	if pctx.HasTemplateMatch {
		savings := remainingBudget(forecast).Mul(decimal.NewFromFloat(0.65))
		out = append(out, Reroute{
			Kind:              RerouteTemplateSwap,
			Label:             "Reuse a shipped template",
			Description:       "A verified template matches this project's intent. Switching cuts the from-scratch build to a configure-and-style pass.",
			SavingsUSD:        savings,
			SavingsConfidence: 0.85,
			Reversible:        true,
		})
	}

	// Smaller context fires when we know there is something to trim.
	if pctx.ReferenceBytes > 200_000 {
		savings := remainingBudget(forecast).Mul(decimal.NewFromFloat(0.2))
		out = append(out, Reroute{
			Kind:              RerouteSmallerContext,
			Label:             "Trim reference attachments",
			Description:       "Drop reference attachments below 200 KB. Per-call prompt cost contracts; the AI still has the project summary and the live workspace.",
			SavingsUSD:        savings,
			SavingsConfidence: 0.6,
			Reversible:        true,
		})
	}

	// Skip mobile is only valid when there is a mobile target to skip.
	if pctx.HasMobileTarget {
		savings := remainingBudget(forecast).Mul(decimal.NewFromFloat(0.45))
		out = append(out, Reroute{
			Kind:              RerouteSkipMobile,
			Label:             "Defer mobile build",
			Description:       "Ship the web surface first; restart the mobile build once the web build is paid for and the wallet refills.",
			SavingsUSD:        savings,
			SavingsConfidence: 0.75,
			Reversible:        true,
		})
	}

	sortRoutesBySavings(out)
	return out, nil
}

// cheaperModelSavings estimates the dollar saving of dropping one
// tier on the remaining runway. The savings factor is empirical: the
// cheaper tier is typically ~30-40% of the current tier per-token
// cost; we use 0.35 as the cross-vendor midpoint.
func cheaperModelSavings(currentTier string, f Forecast) (decimal.Decimal, bool) {
	remaining := remainingBudget(f)
	if !remaining.IsPositive() {
		return decimal.Zero, false
	}
	factor := decimal.NewFromFloat(0.35)
	switch currentTier {
	case "opus":
		// Opus → Sonnet is the cleanest tier drop.
		return remaining.Mul(factor), true
	case "sonnet":
		// Sonnet → Haiku saves less in absolute terms because Sonnet
		// is already mid-tier. We model the saving as 0.55 of
		// remaining (Haiku is ~45% of Sonnet per-token).
		return remaining.Mul(decimal.NewFromFloat(0.55)), true
	case "haiku":
		// No cheaper tier remains.
		return decimal.Zero, false
	default:
		// Unknown tier — return the conservative midpoint so the
		// suggestion still renders rather than silently dropping.
		return remaining.Mul(factor), true
	}
}

// remainingBudget is the unspent runway in the forecast — savings
// estimates are always relative to this number, never to the cap or
// the all-time spend.
func remainingBudget(f Forecast) decimal.Decimal {
	r := f.ExtrapolatedTotalUSD.Sub(f.SpentUSD)
	if r.IsNegative() {
		return decimal.Zero
	}
	return r
}

// sortRoutesBySavings ranks reroutes by SavingsUSD desc with Kind
// as a stable tiebreaker so identical-saving rows render in the
// same order on every dashboard refresh.
func sortRoutesBySavings(in []Reroute) {
	for i := 1; i < len(in); i++ {
		for j := i; j > 0; j-- {
			if in[j].SavingsUSD.GreaterThan(in[j-1].SavingsUSD) ||
				(in[j].SavingsUSD.Equal(in[j-1].SavingsUSD) && in[j].Kind < in[j-1].Kind) {
				in[j], in[j-1] = in[j-1], in[j]
			} else {
				break
			}
		}
	}
}
