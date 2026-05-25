package resolver

// Studio entrypoint helpers — extracted from studio.resolver.go so
// gqlgen's regeneration leaves them in place. Every symbol here is
// referenced from DescribeIdea / RefineIdea.

import (
	"fmt"
	"strings"

	"ironflyer/apps/orchestrator/internal/blueprints"
	"ironflyer/apps/orchestrator/internal/graph/model"
)

// defaultStudioBudgetUSD is the wallet hold placed when the caller did
// not pin one. Conservative — high enough to cover a small finisher
// run on Sonnet 4.6, low enough that a $25 top-up still funds many.
const defaultStudioBudgetUSD = 2.0

// studioBlueprintRef is the shared blueprint shape both DescribeIdea
// and RefineIdea hand to studioIdeaFor. It survives ideaparser /
// rules-only paths so the resolver doesn't have to fork its return
// statement.
type studioBlueprintRef struct {
	ID        string
	Name      string
	Reason    string
	StopLoss  float64
	CostPrior float64
	TimeSec   int
	Gates     []string
}

// studioPickBlueprint returns the first blueprint whose category or
// supported gates appear in the prompt. Falls back to the catalogue's
// first row, or a synthetic "auto" blueprint when the registry is
// empty or nil. Used when the LLM-backed ideaparser is unavailable.
func studioPickBlueprint(reg blueprints.Registry, text string) studioBlueprintRef {
	if reg == nil {
		return studioBlueprintRef{ID: "auto", Name: "Auto-detected"}
	}
	list := reg.List()
	if len(list) == 0 {
		return studioBlueprintRef{ID: "auto", Name: "Auto-detected"}
	}
	lower := strings.ToLower(text)
	for _, bp := range list {
		if strings.Contains(lower, strings.ToLower(bp.Category)) {
			return studioBlueprintRef{
				ID:        bp.ID,
				Name:      bp.Name,
				Reason:    fmt.Sprintf("Matched category %q in prompt.", bp.Category),
				StopLoss:  bp.CostPriorUSD.InexactFloat64() * 1.5,
				CostPrior: bp.CostPriorUSD.InexactFloat64(),
				TimeSec:   bp.ExpectedTimeToPreviewSec,
				Gates:     bp.SupportedGates,
			}
		}
	}
	bp := list[0]
	return studioBlueprintRef{
		ID:        bp.ID,
		Name:      bp.Name,
		Reason:    "Default blueprint chosen — no category match in prompt.",
		StopLoss:  bp.CostPriorUSD.InexactFloat64() * 1.5,
		CostPrior: bp.CostPriorUSD.InexactFloat64(),
		TimeSec:   bp.ExpectedTimeToPreviewSec,
		Gates:     bp.SupportedGates,
	}
}

// studioMaybeBlueprint returns the blueprint with the given id, or a
// synthetic ref when the registry is nil or the id is unknown.
func studioMaybeBlueprint(reg blueprints.Registry, id string) studioBlueprintRef {
	if reg == nil || id == "" {
		return studioBlueprintRef{ID: id, Name: "Auto-detected"}
	}
	if bp, ok := reg.Get(id); ok {
		return studioBlueprintRef{
			ID:        bp.ID,
			Name:      bp.Name,
			Reason:    "Resumed from prior execution.",
			StopLoss:  bp.CostPriorUSD.InexactFloat64() * 1.5,
			CostPrior: bp.CostPriorUSD.InexactFloat64(),
			TimeSec:   bp.ExpectedTimeToPreviewSec,
			Gates:     bp.SupportedGates,
		}
	}
	return studioBlueprintRef{ID: id, Name: "Auto-detected"}
}

func studioDefaultEstimate(budgetUSD float64) model.CostEstimate {
	low := budgetUSD * 0.35
	med := budgetUSD * 0.6
	high := budgetUSD * 0.9
	p95 := budgetUSD * 0.98
	caveat := "Estimate is a flat-rate heuristic; replace with Forecaster once wired."
	return model.CostEstimate{
		LowUsd:      low,
		MedianUsd:   med,
		HighUsd:     high,
		P95usd:      p95,
		Breakdown:   model.JSON{"note": "heuristic", "budgetUSD": budgetUSD},
		Confidence:  0.3,
		BasedOnRuns: 0,
		Caveat:      &caveat,
	}
}

func studioIdeaFor(title, summary string, bp studioBlueprintRef) model.ParsedIdea {
	if title == "" {
		title = "Untitled"
	}
	if bp.Reason == "" {
		bp.Reason = "Auto-detected from prompt."
	}
	return model.ParsedIdea{
		Title:              title,
		Summary:            summary,
		BlueprintID:        bp.ID,
		BlueprintReason:    bp.Reason,
		SuggestedBudgetUsd: bp.CostPrior,
		Tags:               []string{},
		StopLossUsd:        bp.StopLoss,
		Confidence:         0.5,
	}
}

// studioTitleFrom derives a short title from the prompt: first 5
// non-filler words, title-cased. Mirrors the deriveProjectName helper
// in the web app.
func studioTitleFrom(text string) string {
	fillers := map[string]bool{
		"a": true, "an": true, "the": true, "build": true, "make": true,
		"create": true, "please": true, "i": true, "want": true, "need": true,
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return "Untitled"
	}
	i := 0
	for i < len(words) && fillers[strings.ToLower(strings.TrimRight(words[i], ".,;:!?"))] {
		i++
	}
	if i >= len(words) {
		return "Untitled"
	}
	end := i + 5
	if end > len(words) {
		end = len(words)
	}
	head := make([]string, 0, end-i)
	for _, w := range words[i:end] {
		w = strings.TrimRight(w, ".,;:!?-")
		if w == "" {
			continue
		}
		if len(w) <= 4 && w == strings.ToUpper(w) {
			head = append(head, w)
			continue
		}
		head = append(head, strings.ToUpper(w[:1])+strings.ToLower(w[1:]))
	}
	out := strings.Join(head, " ")
	if len(out) > 60 {
		out = out[:60]
		if cut := strings.LastIndex(out, " "); cut > 30 {
			out = out[:cut]
		}
	}
	if out == "" {
		return "Untitled"
	}
	return out
}

// studioInsufficientFunds returns a typed GraphQL error indicating
// the wallet is too low for the requested run. Carries a top_up_url
// extension so the UI can render a one-click "Add credits" CTA.
func studioInsufficientFunds(webBaseURL string) error {
	if webBaseURL == "" {
		webBaseURL = "http://localhost:3000"
	}
	return &studioGraphQLError{
		message:  "Your wallet is too low to start this run.",
		code:     "INSUFFICIENT_FUNDS",
		topUpURL: strings.TrimRight(webBaseURL, "/") + "/wallet",
	}
}

type studioGraphQLError struct {
	message  string
	code     string
	topUpURL string
}

func (e *studioGraphQLError) Error() string { return e.message }
