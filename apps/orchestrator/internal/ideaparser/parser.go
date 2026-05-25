// Package ideaparser turns a free-text product idea ("build me a
// CRM for my landscape business") into a concrete, fully-priced
// execution plan: a blueprint id, a suggested budget, a stop-loss,
// and a distilled prompt summary the finisher loop will consume.
//
// It is intentionally a thin LLM-powered wrapper around the existing
// V22 primitives (blueprints.Registry, providers.Router) rather than
// a separate reasoning engine. The package exposes two backends:
//
//   - LLMParser — production path. Uses providers.Router with a
//     cheap-tier capability profile (cheap + json + fast) to ask
//     a model to choose a blueprint from the catalogue and return
//     a structured Idea. Falls back to the rules parser on any
//     model error or JSON-decode failure.
//   - RulesParser — keyword-only offline fallback. Always available
//     regardless of whether any provider has been registered;
//     guarantees the studio entrypoint stays functional on dev /
//     air-gapped deployments.
//
// Money is decimal.Decimal USD throughout. MaxBudgetUSD on ParseInput
// is the hard ceiling — the parser clamps the suggested budget so
// the resolver never asks the wallet to hold more than is available.
package ideaparser

import (
	"context"

	"github.com/shopspring/decimal"
)

// Parser is the surface every studio resolver consumes. Both backends
// (LLM, rules) implement it; the wireup helper picks one based on
// config.
//
// Parse MUST always return a valid Idea on a non-error return:
// BlueprintID must reference a blueprint known to the registry that
// was used to construct the parser, SuggestedBudget must be > 0 and
// <= ParseInput.MaxBudgetUSD, and StopLossUSD must be >=
// SuggestedBudget. Callers may rely on these invariants.
type Parser interface {
	Parse(ctx context.Context, in ParseInput) (Idea, error)
}

// ParseInput is the request payload. Text is the user's natural
// language prompt. TenantID / UserID anchor any future per-tenant
// shaping (the current implementation ignores them, but the
// signature is forward-compatible so we can later inject
// per-tenant history or preferred blueprints without breaking
// callers).
type ParseInput struct {
	Text         string
	TenantID     string
	UserID       string
	MaxBudgetUSD decimal.Decimal
}

// Idea is the structured plan the parser hands back. Every field
// is populated — the resolver does not have to defend against zero
// values (an empty Title means the parser failed and returned err).
type Idea struct {
	// Title is a short, descriptive name (3-6 words) suitable for
	// a project list row and the workspace tab.
	Title string
	// Summary is the 1-2 sentence product spec the finisher loop
	// uses as the canonical idea description.
	Summary string
	// BlueprintID is one of the catalogue ids (e.g. "nextjs-mvp",
	// "static-landing"). Validated against the registry.
	BlueprintID string
	// BlueprintReason is the short ("because the user asked for
	// X / Y") justification — surfaced in the studio chrome so the
	// builder knows why this blueprint was picked.
	BlueprintReason string
	// SuggestedBudget is the wallet hold to place. Always > 0 and
	// <= ParseInput.MaxBudgetUSD.
	SuggestedBudget decimal.Decimal
	// Tags is a normalised, lowercased classification list (e.g.
	// ["webapp", "saas", "crm"]) used by the forecaster + the
	// dashboards.
	Tags []string
	// StopLossUSD is the hard cap the execution will refuse to
	// cross. Always >= SuggestedBudget; clamped to
	// ParseInput.MaxBudgetUSD.
	StopLossUSD decimal.Decimal
	// Confidence in the parsing decision, in [0, 1]. The LLM path
	// returns the model's self-reported confidence; the rules
	// path returns a fixed prior based on keyword strength.
	Confidence float64
	// PromptSummary is the distilled prompt the finisher receives.
	// May equal Summary when the input is short; when the input
	// is long, the parser rewrites it as a concise spec.
	PromptSummary string
}
