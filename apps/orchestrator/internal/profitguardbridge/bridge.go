// Package profitguardbridge adapts execution.State + provider quotes
// into the profitguard.ExecState shape Guard.Decide consumes.
//
// The two domains intentionally stay decoupled (profitguard has no
// knowledge of execution.State; execution has no knowledge of the
// ProfitGuard input shape). The integration loop owns this adapter.
package profitguardbridge

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/blueprints"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/profitguard"
	"ironflyer/apps/orchestrator/internal/providers"
	"ironflyer/apps/orchestrator/internal/repair"
)

// ErrNoExecution is returned by SnapshotFor when the caller passes
// an empty executionID or a nil execution.Service. Callers treat
// this as "no execution context — skip the ProfitGuard step" rather
// than a hard failure.
var ErrNoExecution = errors.New("profitguardbridge: no execution on context")

// BridgeFlags are the per-call signals the runtime injects when it
// already knows whether a similar blueprint / repair is available.
// Zero values default to "no" — Decide simply skips those rescue
// branches.
//
// FailureSignature is the (already-normalised or raw) failure string
// the caller has on hand at the call site. When non-empty AND a
// BridgeDeps.Genome is wired into SnapshotFor, the bridge looks the
// signature up in the genome and flips SimilarRepairAvailable on a
// hit. This is the v1 wiring: only the recovery loop knows the
// signature, so all other call sites pass zero.
type BridgeFlags struct {
	SimilarBlueprintAvailable bool
	SimilarRepairAvailable    bool
	CurrentProvider           string
	EstimatedNextStepCostUSD  decimal.Decimal
	EstimatedPlatformCostUSD  decimal.Decimal
	StopLossUSD               decimal.Decimal
	FailureSignature          string
}

// BridgeDeps carries the optional reuse-signal sources the bridge
// consults when building an ExecState. Both fields are nil-safe:
// missing Registry leaves SimilarBlueprintAvailable at the
// BridgeFlags-provided value; missing Genome leaves
// SimilarRepairAvailable at the BridgeFlags-provided value.
//
// Wired by cmd/orchestrator/main.go alongside the rest of the V22
// glue.
type BridgeDeps struct {
	Registry blueprints.Registry
	Genome   repair.Genome
}

// StateToGuardInput converts execution.State + provider quotes +
// per-call flags into a profitguard.ExecState. Money fields flow
// through as decimal.Decimal so no precision is lost between the
// ledger and the policy.
//
// This signature is preserved for callers that don't have BridgeDeps
// on hand. New call sites should prefer StateToGuardInputWithDeps so
// the SimilarBlueprintAvailable / SimilarRepairAvailable signals are
// populated automatically when the deps are available.
func StateToGuardInput(s execution.State, providers []profitguard.ProviderQuote, flags BridgeFlags) profitguard.ExecState {
	return StateToGuardInputWithDeps(context.Background(), s, providers, flags, BridgeDeps{})
}

// StateToGuardInputWithDeps is the deps-aware variant of
// StateToGuardInput. When deps.Registry is wired, the bridge asks
// for a Recommend(...) and sets SimilarBlueprintAvailable=true when
// a fit is found that is NOT the currently-running blueprint. When
// deps.Genome is wired and BridgeFlags.FailureSignature is non-empty,
// the bridge does a Lookup and sets SimilarRepairAvailable=true on a
// hit.
//
// Caller-provided BridgeFlags.SimilarBlueprintAvailable /
// SimilarRepairAvailable are honoured (logical OR with the
// deps-derived signal) so existing call sites that already know "yes"
// remain authoritative.
func StateToGuardInputWithDeps(ctx context.Context, s execution.State, quotes []profitguard.ProviderQuote, flags BridgeFlags, deps BridgeDeps) profitguard.ExecState {
	estStep := flags.EstimatedNextStepCostUSD
	estPlatform := flags.EstimatedPlatformCostUSD

	stopLoss := flags.StopLossUSD
	if stopLoss.IsZero() && s.StopLossUSD != nil {
		stopLoss = *s.StopLossUSD
	}

	expDelta := float64(0)
	if s.ExpectedCompletionDelta != nil {
		expDelta = *s.ExpectedCompletionDelta
	}
	risk := float64(0)
	if s.RiskScore != nil {
		risk = *s.RiskScore
	}

	similarBlueprint := flags.SimilarBlueprintAvailable
	if !similarBlueprint && deps.Registry != nil {
		similarBlueprint = hasSimilarBlueprint(deps.Registry, s.BlueprintID)
	}
	similarRepair := flags.SimilarRepairAvailable
	if !similarRepair && deps.Genome != nil && flags.FailureSignature != "" {
		if _, hit, err := deps.Genome.Lookup(ctx, flags.FailureSignature); err == nil && hit {
			similarRepair = true
		}
	}

	return profitguard.ExecState{
		ExecutionID:               s.ID,
		TenantID:                  s.TenantID,
		UserBudgetUSD:             s.BudgetUSD,
		SpentUSD:                  s.SpentUSD,
		ReservedUSD:               s.ReservedUSD,
		EstimatedNextStepCostUSD:  estStep,
		EstimatedPlatformCostUSD:  estPlatform,
		ExpectedCompletionDelta:   expDelta,
		RiskScore:                 risk,
		StopLossUSD:               stopLoss,
		CurrentProvider:           flags.CurrentProvider,
		AvailableProviders:        quotes,
		SimilarBlueprintAvailable: similarBlueprint,
		SimilarRepairAvailable:    similarRepair,
	}
}

// hasSimilarBlueprint implements the v1 heuristic from
// docs/V22_PLAN: when the execution has a blueprint id, return true
// if Recommend(["<currentID>", "fallback"], 0.7) returns any
// blueprint OTHER than the currently-running one. When the
// execution has no blueprint id, return true if Recommend with
// empty tags returns anything at all (i.e. the catalogue is
// non-empty).
//
// The 0.7 risk ceiling matches the policy doc's "low risk" tier so
// SimilarBlueprintAvailable is only true for fits the policy would
// actually rescue with.
func hasSimilarBlueprint(reg blueprints.Registry, currentBlueprintID string) bool {
	if reg == nil {
		return false
	}
	if currentBlueprintID == "" {
		out := reg.Recommend(nil, 0.7)
		return len(out) > 0
	}
	out := reg.Recommend([]string{currentBlueprintID, "fallback"}, 0.7)
	for _, b := range out {
		if b.ID != currentBlueprintID {
			return true
		}
	}
	return false
}

// SnapshotFor is the bridge's "give me a fresh ExecState" helper.
// Callers that don't have a providers.Request on hand (the finisher
// engine at BeforeSandboxAllocation, for example) use this so they
// don't have to reimplement the snapshot plumbing.
//
// quotes are caller-supplied — pass nil when the call site has no
// provider preference signal. SnapshotFor will NOT call the router
// itself: the router lives one package away and we keep this bridge
// router-agnostic so the no-cycle invariant holds.
//
// log is optional; non-nil callers receive a structured warning when
// execSvc.GetState fails. err is non-nil only on execSvc errors.
func SnapshotFor(
	ctx context.Context,
	execSvc execution.Service,
	executionID string,
	deps BridgeDeps,
	flags BridgeFlags,
	quotes []profitguard.ProviderQuote,
	log *zerolog.Logger,
) (profitguard.ExecState, error) {
	if execSvc == nil || executionID == "" {
		return profitguard.ExecState{}, ErrNoExecution
	}
	state, err := execSvc.GetState(ctx, executionID)
	if err != nil {
		if log != nil {
			log.Warn().Err(err).Str("execution_id", executionID).Msg("profitguardbridge.SnapshotFor: execSvc.GetState")
		}
		return profitguard.ExecState{}, err
	}
	return StateToGuardInputWithDeps(ctx, state, quotes, flags, deps), nil
}

// QuotesFromRouter converts the router's []providers.QuoteEntry into
// []profitguard.ProviderQuote so the ProfitGuard ExecState gets a
// concrete candidate set. The integration loop calls this at the
// BillingGuard snapshot seam — see cmd/orchestrator/main.go.
func QuotesFromRouter(in []providers.QuoteEntry) []profitguard.ProviderQuote {
	if len(in) == 0 {
		return nil
	}
	out := make([]profitguard.ProviderQuote, 0, len(in))
	for _, q := range in {
		out = append(out, profitguard.ProviderQuote{
			Name:             q.Provider,
			EstimatedCostUSD: q.EstimatedCostUSD,
			ExpectedQuality:  q.ExpectedQuality,
			LatencyMS:        q.LatencyMS,
		})
	}
	return out
}
