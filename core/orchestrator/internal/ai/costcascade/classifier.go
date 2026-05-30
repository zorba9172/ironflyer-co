package costcascade

import (
	"context"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// premiumCaps are the capability tags that map to the premium reasoning
// tier (Opus / o3 / Gemini Pro) inside every provider's pickModel. Mirrors
// providers.requiresPremiumReasoning so the cascade classifies a delegated
// call into the same tier the provider will actually bill.
var premiumCaps = map[providers.Capability]bool{
	providers.CapQuality:   true,
	providers.CapReasoning: true,
	providers.CapThinking:  true,
}

// cheapCaps map to the small/reflex tier (Haiku / 4o-mini / Flash).
var cheapCaps = map[providers.Capability]bool{
	providers.CapCheap:  true,
	providers.CapFast:   true,
	providers.CapInline: true,
}

// Classifier picks the cheapest viable model tier for a delegated request
// and labels it as the matching cascade layer. By default it is a pure
// classifier — it reports the tier the request already asks for without
// touching the request. When downgrade is enabled (opt-in) and the
// aggression controller is hot, it strips premium capabilities so the
// router rolls onto a cheaper tier, exactly as ProfitGuard's Degrade
// verdict does — never silently, and never below the planning tier (we
// never downgrade a real reasoning task all the way to a reflex model;
// that trades too much quality for too little money).
type Classifier struct {
	// allowDowngrade gates the only behaviour that mutates the request. Off
	// by default so the cascade is observe-and-route only until an operator
	// opts in.
	allowDowngrade bool

	// downgradeThreshold is the aggression level at or above which a
	// premium request is degraded to planning. Default 0.5 — the ratio has
	// to be meaningfully over target, not a hair above it.
	downgradeThreshold float64

	// scorer is the optional difficulty router (RouteLLM-class). When set,
	// it picks the cheapest viable tier for requests that did NOT explicitly
	// demand premium reasoning — an explicit premium capability is always a
	// floor, so the scorer may route up but never silently below what the
	// caller asked for. Wired via Cascade.WithDifficultyScorer.
	scorer DifficultyScorer
}

// NewClassifier builds a classifier. allowDowngrade=false yields a pure
// observe-and-route classifier (recommended default).
func NewClassifier(allowDowngrade bool, downgradeThreshold float64) *Classifier {
	if downgradeThreshold <= 0 || downgradeThreshold > 1 {
		downgradeThreshold = 0.5
	}
	return &Classifier{allowDowngrade: allowDowngrade, downgradeThreshold: downgradeThreshold}
}

// tierOf reports the model tier a request's capabilities resolve to,
// without mutation.
func tierOf(caps []providers.Capability) Layer {
	for _, c := range caps {
		if premiumCaps[c] {
			return LayerReasoning
		}
	}
	for _, c := range caps {
		if cheapCaps[c] {
			return LayerReflex
		}
	}
	return LayerPlanning
}

// Classify returns the layer the delegated call will be billed at and the
// (possibly downgraded) request to forward. aggression is the current
// controller level in [0,1].
func (c *Classifier) Classify(ctx context.Context, req providers.Request, aggression float64) (Layer, providers.Request) {
	layer := tierOf(req.Capabilities)

	// Difficulty-aware routing (RouteLLM-class). Only applies when the
	// caller did NOT explicitly demand the premium tier — an explicit
	// CapQuality/CapReasoning/CapThinking is honoured as a hard floor. The
	// scorer can route a nominally-"planning" request DOWN to reflex when
	// it is easy, or UP to reasoning when it is hard, without ever
	// overriding an explicit premium request.
	if c != nil && c.scorer != nil && layer != LayerReasoning {
		switch d := c.scorer.Score(ctx, req); {
		case d >= 0.66:
			layer = LayerReasoning
		case d >= 0.33:
			layer = LayerPlanning
		default:
			layer = LayerReflex
		}
		req.Capabilities = retierCaps(req.Capabilities, layer)
	}

	if c == nil || !c.allowDowngrade {
		return layer, req
	}
	if layer == LayerReasoning && aggression >= c.downgradeThreshold {
		req.Capabilities = stripCaps(req.Capabilities, providers.CapQuality, providers.CapReasoning, providers.CapThinking)
		// Extended thinking is the most expensive knob — drop it too when
		// we are degrading the tier, so the downgrade actually saves money.
		req.EnableThinking = false
		req.ThinkingBudget = 0
		return tierOf(req.Capabilities), req
	}
	return layer, req
}

// stripCaps returns caps with every listed capability removed, preserving
// order. Allocates a fresh slice so the caller's request is never aliased.
func stripCaps(caps []providers.Capability, drop ...providers.Capability) []providers.Capability {
	if len(caps) == 0 {
		return caps
	}
	dropSet := make(map[providers.Capability]bool, len(drop))
	for _, d := range drop {
		dropSet[d] = true
	}
	out := make([]providers.Capability, 0, len(caps))
	for _, c := range caps {
		if !dropSet[c] {
			out = append(out, c)
		}
	}
	return out
}

// retierCaps rewrites a request's capability tags so the provider router
// picks the target tier. It strips the tier-selecting tags (cheap/fast/
// inline and quality/reasoning/thinking) then stamps the representative tag
// for the chosen tier: reflex → CapCheap, reasoning → CapReasoning,
// planning → none (the provider default tier). Capabilities that describe
// the WORK rather than the tier (CapCode, CapJSON, CapVision, CapTools…)
// are preserved.
func retierCaps(caps []providers.Capability, layer Layer) []providers.Capability {
	base := stripCaps(caps,
		providers.CapCheap, providers.CapFast, providers.CapInline,
		providers.CapQuality, providers.CapReasoning, providers.CapThinking,
	)
	switch layer {
	case LayerReflex:
		return append(base, providers.CapCheap)
	case LayerReasoning:
		return append(base, providers.CapReasoning)
	default:
		return base
	}
}

// inputRatePerToken returns the coarse USD-per-input-token rate for a tier,
// used only to credit the savings counter when the compressor shrinks a
// prompt. Mirrors the orders of magnitude in estimateCostUSD.
func inputRatePerToken(layer Layer) float64 {
	switch layer {
	case LayerReflex:
		return 0.80 / 1e6
	case LayerReasoning:
		return 15.00 / 1e6
	default:
		return 3.00 / 1e6
	}
}

// estimateCostUSD is a coarse forecast of the provider cost a request
// WOULD have incurred. It exists only to populate the cascade savings
// metric on a zero-cost (rules/cache/knowledge) hit — real charging is
// untouched and flows through BillingGuard as before. The blended rates
// mirror the orders of magnitude in providers/cost.go without coupling to
// its unexported table: input + output tokens at the tier's $/MTok.
func estimateCostUSD(req providers.Request) float64 {
	inTokens := (len(req.System) + len(req.ProjectContext) + len(req.Prompt)) / 4
	outTokens := req.MaxTokens
	if outTokens == 0 {
		outTokens = 2048
	}
	var inRate, outRate float64 // USD per token
	switch tierOf(req.Capabilities) {
	case LayerReflex: // Haiku / 4o-mini / Flash class
		inRate, outRate = 0.80/1e6, 4.00/1e6
	case LayerReasoning: // Opus / o3 class
		inRate, outRate = 15.00/1e6, 75.00/1e6
	default: // planning — Sonnet / gpt-4o class
		inRate, outRate = 3.00/1e6, 15.00/1e6
	}
	return float64(inTokens)*inRate + float64(outTokens)*outRate
}
