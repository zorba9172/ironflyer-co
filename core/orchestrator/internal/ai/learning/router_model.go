// Package learning — RouterModel bridges OutcomeEvents to the
// proprietary contextual bandit (LinUCB) defined in
// internal/ai/providers/bandit_contextual.go.
//
// The bandit lives in the providers package so the router can call
// Select without an import cycle. RouterModel sits in this package
// because (a) it needs the OutcomeEvent contract, and (b) it owns the
// reward formula. To bridge the two without dragging providers in
// here we accept the bandit via the ContextualBanditSink interface
// defined below — every method we need is satisfied by
// *providers.ContextualBandit at zero cost.
//
// Reward signal:
//
//	reward = clamp(margin_per_dollar, 0, 1) * outcomeFactor
//	  where margin_per_dollar = margin_usd / cost_usd (0 when cost == 0)
//	        outcomeFactor     = 1.0 on execution_complete success
//	                          = 0.5 on partial / failed completion
//	                          = 1.0 on provider_chosen (no failure signal yet)
//
// Provider_chosen events seed the bandit with neutral (0.5) reward
// scaled by margin_per_dollar so cheap providers stay attractive even
// before an execution_complete arrives. Execution_complete events
// adjust the bandit with the true realised reward — they look up
// every provider chosen in the same execution and credit each one
// proportionally.

package learning

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ContextualBanditSink is the minimal slice of providers.ContextualBandit
// the RouterModel touches. Defined as a struct of callback functions
// (rather than an interface over the concrete provider type) so the
// learning package stays free of an import cycle through
// internal/ai/providers. Main.go wires each callback to the
// providers.ContextualBandit method that matches.
type ContextualBanditSink struct {
	Update  func(arm string, rc RouteContext, reward float64)
	AddArm  func(name string)
	Samples func() int
}

// IsZero returns true when the sink has no callbacks wired — the
// RouterModel then short-circuits to a no-op.
func (s ContextualBanditSink) IsZero() bool {
	return s.Update == nil && s.AddArm == nil && s.Samples == nil
}

// RouteContext is a value type the bandit consumes on every Select /
// Update. Mirrors providers.RouteContext — kept here as well so
// learning stays import-cycle-free. The two types are bridged by
// adaptContextualBandit at wireup time (see main.go) or by callers
// passing an adapter.
type RouteContext struct {
	Capability   string
	PromptTokens int
	TenantTier   string
	TimeOfDay    int
	HasThinking  bool
	HasTools     bool
}

// RouterModel translates OutcomeEvents into ContextualBandit updates.
type RouterModel struct {
	bandit ContextualBanditSink
	store  Store
	log    zerolog.Logger

	// pendingExec keys the provider chosen for an in-flight execution.
	// On KindExecutionComplete we credit every provider in the slice
	// with the realised reward.
	mu          sync.Mutex
	pendingExec map[string][]providerRouteEvidence
}

// providerRouteEvidence is the cached body of one KindProviderChosen
// event so KindExecutionComplete can replay the reward computation
// against the original RouteContext.
type providerRouteEvidence struct {
	Provider string
	Context  RouteContext
	CostUSD  float64
}

// NewRouterModel wires the bridge. bandit MAY be nil — Process /
// LoadFromHistory then degrade to no-ops so callers can construct
// the model unconditionally at boot.
func NewRouterModel(bandit ContextualBanditSink, store Store, log zerolog.Logger) *RouterModel {
	return &RouterModel{
		bandit:      bandit,
		store:       store,
		log:         log,
		pendingExec: make(map[string][]providerRouteEvidence),
	}
}

// Subscribe wires the RouterModel as a Publisher observer. The
// returned closure is suitable for passing to Publisher.SetObserver.
// If a previous observer was set (e.g. the memory store), the caller
// should compose them — RouterModel does not chain by itself.
func (rm *RouterModel) Subscribe() func(OutcomeEvent) {
	return func(evt OutcomeEvent) {
		if err := rm.ProcessOutcome(evt); err != nil {
			rm.log.Debug().Err(err).Str("kind", string(evt.Kind)).
				Msg("router_model: process outcome failed")
		}
	}
}

// ProcessOutcome is the per-event entry point. It is safe to call
// concurrently and silently no-ops on irrelevant kinds.
func (rm *RouterModel) ProcessOutcome(evt OutcomeEvent) error {
	if rm == nil || rm.bandit.IsZero() {
		return nil
	}
	switch evt.Kind {
	case KindProviderChosen:
		rm.handleProviderChosen(evt)
	case KindExecutionComplete:
		rm.handleExecutionComplete(evt)
	}
	return nil
}

func (rm *RouterModel) handleProviderChosen(evt OutcomeEvent) {
	provider, _ := evt.Attributes["provider"].(string)
	if provider == "" {
		return
	}
	rc := routeContextFromEvent(evt)
	cost := 0.0
	if evt.CostUSD != nil {
		cost, _ = evt.CostUSD.Float64()
	}
	// Seed the arm immediately with a neutral-ish reward scaled by
	// inverse cost so cheap providers stay attractive before the
	// execution_complete signal arrives. Margin is unknown at this
	// point so we use the provider's cost-per-call as a stand-in
	// reward proxy: lower cost → higher reward.
	const provisionalCeiling = 0.5
	reward := provisionalCeiling
	if cost > 0 {
		// Map cost [0, 0.10) → reward [0.5, 0). 0.10 USD/call is the
		// same MaxCostUSD ceiling the legacy Bandit uses for its
		// reward normalisation — keep them in sync.
		const maxCost = 0.10
		ratio := cost / maxCost
		if ratio > 1 {
			ratio = 1
		}
		reward = provisionalCeiling * (1 - ratio)
	}
	rm.bandit.AddArm(provider)
	rm.bandit.Update(provider, rc, reward)

	if evt.ExecutionID == "" {
		return
	}
	rm.mu.Lock()
	rm.pendingExec[evt.ExecutionID] = append(rm.pendingExec[evt.ExecutionID], providerRouteEvidence{
		Provider: provider,
		Context:  rc,
		CostUSD:  cost,
	})
	if len(rm.pendingExec[evt.ExecutionID]) > 64 {
		// Cap per-execution chain to bound memory.
		rm.pendingExec[evt.ExecutionID] = rm.pendingExec[evt.ExecutionID][len(rm.pendingExec[evt.ExecutionID])-64:]
	}
	rm.mu.Unlock()
}

func (rm *RouterModel) handleExecutionComplete(evt OutcomeEvent) {
	if evt.ExecutionID == "" {
		return
	}
	rm.mu.Lock()
	chosen := rm.pendingExec[evt.ExecutionID]
	delete(rm.pendingExec, evt.ExecutionID)
	rm.mu.Unlock()
	if len(chosen) == 0 {
		return
	}

	// Compute the realised reward signal:
	//   margin_per_dollar = margin_usd / cost_usd
	//   outcomeFactor     = 1.0 on success, 0.5 otherwise
	//   reward            = clamp(margin_per_dollar, 0, 1) * outcomeFactor
	margin := 0.0
	if evt.MarginUSD != nil {
		margin, _ = evt.MarginUSD.Float64()
	}
	totalCost := 0.0
	if evt.CostUSD != nil {
		totalCost, _ = evt.CostUSD.Float64()
	}
	mpd := 0.0
	if totalCost > 0 {
		mpd = margin / totalCost
	}
	if mpd < 0 {
		mpd = 0
	}
	if mpd > 1 {
		mpd = 1
	}
	outcomeFactor := 0.5
	if evt.Success != nil && *evt.Success {
		outcomeFactor = 1.0
	}
	reward := mpd * outcomeFactor

	// Credit every provider that participated in the execution.
	for _, ev := range chosen {
		rm.bandit.Update(ev.Provider, ev.Context, reward)
	}
}

// LoadFromHistory seeds the bandit by replaying historical outcomes
// from the Store. It is a best-effort warm-start: when the store
// doesn't expose a tenant-scoped event slice (current Store interface
// is dashboard-projection-only) we fall back to a Snapshot pull so we
// at least record that the model is online.
//
// Future extension: when ClickHouseStore grows an EventsByTenant
// method, replace the snapshot fallback with a true replay loop.
func (rm *RouterModel) LoadFromHistory(ctx context.Context, tenantID string, lookback time.Duration) error {
	if rm == nil || rm.bandit == nil || rm.store == nil {
		return nil
	}
	// MemoryStore exposes raw events via a type assertion — the
	// production ClickHouseStore would need a dedicated query. Use
	// reflection-free path: try the assertion, fall back to snapshot.
	if mem, ok := rm.store.(*MemoryStore); ok {
		mem.mu.RLock()
		events := append([]OutcomeEvent(nil), mem.events...)
		mem.mu.RUnlock()
		cutoff := time.Now().UTC().Add(-lookback)
		replayed := 0
		for _, evt := range events {
			if tenantID != "" && evt.TenantID != tenantID {
				continue
			}
			if evt.Timestamp.Before(cutoff) {
				continue
			}
			_ = rm.ProcessOutcome(evt)
			replayed++
		}
		rm.log.Info().
			Int("replayed", replayed).
			Str("tenant_id", tenantID).
			Dur("lookback", lookback).
			Msg("router_model: warm-start from memory store complete")
		return nil
	}
	if _, err := rm.store.Snapshot(ctx, tenantID); err != nil {
		return err
	}
	rm.log.Info().
		Str("tenant_id", tenantID).
		Dur("lookback", lookback).
		Msg("router_model: cold-start (no event replay surface on store)")
	return nil
}

// Samples returns the lifetime bandit sample count. Convenience pass-
// through used by the boot log line.
func (rm *RouterModel) Samples() int {
	if rm == nil || rm.bandit.IsZero() {
		return 0
	}
	return rm.bandit.Samples()
}

// routeContextFromEvent reconstructs a RouteContext from the
// Attributes payload of a KindProviderChosen event. The producer in
// internal/business/budget/billing.go currently only sets provider /
// model / input_tokens / output_tokens — capability tags arrive when
// the billing emitter is extended. Until then we infer Capability
// from the "capabilities" attribute when present.
func routeContextFromEvent(evt OutcomeEvent) RouteContext {
	rc := RouteContext{TimeOfDay: evt.Timestamp.UTC().Hour()}
	if cap, ok := evt.Attributes["capability"].(string); ok {
		rc.Capability = cap
	}
	if caps, ok := evt.Attributes["capabilities"].([]any); ok {
		for _, c := range caps {
			if s, ok := c.(string); ok && rc.Capability == "" {
				switch s {
				case "reasoning", "code", "json", "cheap", "fast", "vision":
					rc.Capability = s
				}
			}
		}
	}
	if pt, ok := evt.Attributes["input_tokens"].(int); ok {
		rc.PromptTokens = pt
	} else if pt, ok := evt.Attributes["input_tokens"].(float64); ok {
		rc.PromptTokens = int(pt)
	}
	if tier, ok := evt.Tags["tier"]; ok {
		rc.TenantTier = tier
	} else if tier, ok := evt.Attributes["tenant_tier"].(string); ok {
		rc.TenantTier = tier
	}
	if think, ok := evt.Attributes["has_thinking"].(bool); ok {
		rc.HasThinking = think
	}
	if tools, ok := evt.Attributes["has_tools"].(bool); ok {
		rc.HasTools = tools
	}
	return rc
}
