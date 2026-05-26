// Package verifier — criterion_executor.go owns the per-criterion
// orchestration helpers Run uses. Kept separate from verifier.go so
// the top-level Run reads as a small driver and the heavier
// "for each criterion, plan + execute + summarise" logic stays in
// its own file as the loop grows.
//
// The exported surface is intentionally small — only the helpers the
// finisher gate needs to project results onto domain.Issue live here.

package verifier

import (
	"ironflyer/core/orchestrator/internal/ai/domain"
)

// IssuesFromResults projects per-criterion verdicts onto gate Issues.
// One Issue per failed criterion (SeverityError) and one per warn
// (SeverityWarning). pass / skipped criteria produce no Issue.
//
// The Hint field is the agent's failure_reason when present, falling
// back to the Playwright tail. Path is left empty — the criterion
// doesn't map to a single file at this point.
func IssuesFromResults(results []CriterionResult) []domain.Issue {
	if len(results) == 0 {
		return nil
	}
	out := make([]domain.Issue, 0, len(results))
	for _, r := range results {
		if r.Skipped {
			continue
		}
		switch r.Verdict {
		case "fail":
			out = append(out, domain.Issue{
				Gate:     domain.GateVerifier,
				Severity: domain.SeverityError,
				Message:  "criterion " + r.CriterionID + " not provable on the live preview",
				Hint:     r.FailureReason,
			})
		case "warn":
			out = append(out, domain.Issue{
				Gate:     domain.GateVerifier,
				Severity: domain.SeverityWarning,
				Message:  "criterion " + r.CriterionID + " partially satisfied",
				Hint:     r.FailureReason,
			})
		}
	}
	return out
}

// PassedCriterionIDs returns the IDs that landed verdict=pass. The
// finisher gate uses this to stamp LastVerifiedAt on the matching
// AcceptanceCriterion records so the UI can render staleness.
func PassedCriterionIDs(results []CriterionResult) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		if r.Verdict == "pass" && !r.Skipped {
			out = append(out, r.CriterionID)
		}
	}
	return out
}
