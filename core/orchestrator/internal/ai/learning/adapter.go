package learning

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// BlueprintWeighter is the optional sink for blueprint selection
// updates. The blueprint registry implements this when a SetSelectionWeight
// method is available; the adapter degrades to a no-op otherwise.
type BlueprintWeighter interface {
	SetSelectionWeight(id string, weight float64)
}

// ForecastCorrector lets the adapter nudge cost-variance estimates.
// The forecast package implements this as a thin shim around its
// variance tracker. nil-safe.
type ForecastCorrector interface {
	RecordCostObservation(ctx context.Context, capability string, observed decimal.Decimal)
}

// BanditPriorRegistrar is the minimal slice of providers.Bandit the
// adapter mutates. The full bandit type lives in
// internal/ai/providers; we accept it through this interface so the
// learning package never has to import providers (which would create
// an import cycle via business/profitguard).
type BanditPriorRegistrar interface {
	// RegisterPrior injects a synthetic (mean, samples) prior for the
	// named provider arm. The bandit treats Samples as the warmth of
	// the prior — small numbers (1–3) bias gently; larger numbers
	// dominate.
	RegisterPrior(provider string, reward float64, samples int)
}

// Adapter consumes PatternObservations and applies them to the live
// strategy surfaces (bandit, blueprint selection weight, forecast
// variance). It is intentionally a passive sink — the miner emits;
// the adapter writes.
type Adapter struct {
	mu        sync.Mutex
	bandit    BanditPriorRegistrar
	blueprint BlueprintWeighter
	forecast  ForecastCorrector
	log       zerolog.Logger
}

// NewAdapter wires the adapter to the live bandit + optional
// blueprint and forecast sinks. Any field MAY be nil — the matching
// observation type is then a no-op.
func NewAdapter(bandit BanditPriorRegistrar, bp BlueprintWeighter, fc ForecastCorrector, log zerolog.Logger) *Adapter {
	return &Adapter{
		bandit:    bandit,
		blueprint: bp,
		forecast:  fc,
		log:       log,
	}
}

// Apply routes one observation to the right sink. Returns true when
// the adapter mutated state, false otherwise.
func (a *Adapter) Apply(ctx context.Context, obs PatternObservation) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch obs.Pattern {
	case "blueprint_success_rate":
		if a.blueprint == nil || obs.Target == "" {
			return false
		}
		weight := 1.0
		if success, ok := obs.Evidence["success_rate"].(float64); ok {
			weight = clamp01(success)
		}
		a.blueprint.SetSelectionWeight(obs.Target, weight)
		return true
	case "provider_margin":
		if a.bandit == nil || obs.Target == "" {
			return false
		}
		reward := 0.5
		if r, ok := obs.Evidence["realized_margin_pct"].(float64); ok {
			reward = clamp01((r + 100) / 200) // map [-100, 100] → [0, 1]
		}
		a.bandit.RegisterPrior(obs.Target, reward, 1)
		return true
	case "forecast_correction":
		if a.forecast == nil {
			return false
		}
		var observed decimal.Decimal
		if v, ok := obs.Evidence["observed_cost_usd"].(float64); ok {
			observed = decimal.NewFromFloat(v)
		}
		a.forecast.RecordCostObservation(ctx, obs.Target, observed)
		return true
	}
	// Gate-failure / repair-hit observations are informational —
	// the dashboard surfaces them but no strategy mutates on them
	// (we don't want to disable a gate just because it fails often).
	return false
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
