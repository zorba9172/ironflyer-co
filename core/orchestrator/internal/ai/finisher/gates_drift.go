// Package finisher — DriftGate.
//
// The Spec gate validates AcceptanceCriterion records once when the spec
// is first wired up. Nothing in the loop re-runs that match as patches
// land, so a Coder that re-organises the workspace can silently delete
// the file that satisfied a criterion and the Spec gate keeps reporting
// "validated". DriftGate closes that window: every iteration it re-runs
// the same keyword-evidence sweep used by validateAcceptanceCriteria and
// reports criteria that lost their evidence (Warning) or that still
// match but on a file that shrank past a 50% threshold (Info).
//
// Repair agent is the Coder — the project still needs the behaviour, so
// the right repair is to re-implement, not to re-spec.
package finisher

import (
	"context"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
)

// DriftGate detects evidence loss on previously-validated acceptance
// criteria. Stays dark for projects whose Spec gate has never marked
// any criterion as Validated — we have nothing to drift against.
type DriftGate struct{}

func (DriftGate) Name() domain.GateName    { return domain.GateDrift }
func (DriftGate) RepairAgent() agents.Role { return agents.RoleCoder }

// driftShrinkThreshold is the fraction by which a file is allowed to
// shrink relative to its original Validated-at size before the gate
// emits a SeverityInfo "verify" nudge. 0.5 means "more than half of the
// original body is gone".
const driftShrinkThreshold = 0.5

// driftMinTrackedBytes is the smallest file size we bother tracking for
// shrink-warnings. Below this, even a single deleted line trips the
// threshold and the signal is mostly noise.
const driftMinTrackedBytes = 200

// previousEvidence is the in-process record of where a criterion was
// last seen validated. The Engine projects this onto GateEnv via
// PreviousAcceptanceEvidence so the gate is stateless across runs (the
// store layer owns persistence). Empty map → nothing to drift against;
// the gate returns no issues.
type previousEvidence struct {
	path string
	size int
}

func (DriftGate) Check(ctx context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	if len(p.Files) == 0 {
		// Code gate will fail first; staying silent here keeps the run
		// log from triple-reporting the same root cause.
		return nil
	}

	// Pull what the Spec gate has stamped onto the project so far. The
	// Spec gate writes the per-criterion EvidencePath when Validated.
	criteria := ExtractAcceptanceCriteria(p)
	if len(criteria) == 0 {
		return nil
	}

	// Build prod corpus (matches Spec-gate semantics).
	prod := make([]filePos, 0, len(p.Files))
	for _, f := range p.Files {
		if isTestFile(f.Path) {
			continue
		}
		prod = append(prod, filePos{path: f.Path, lower: strings.ToLower(f.Content)})
	}
	if len(prod) == 0 {
		return nil
	}

	prev := env.PreviousAcceptanceEvidence
	var issues []domain.Issue
	for _, c := range criteria {
		// Only act on criteria the Spec gate has previously marked Validated.
		// The Spec gate's in-memory output sets EvidencePath when match
		// landed; absence here means we have no prior signal and DriftGate
		// has nothing to compare against.
		base, hadPrev := prev[c.ID]
		if !hadPrev && c.EvidencePath == "" {
			continue
		}
		if base.path == "" && c.EvidencePath != "" {
			base.path = c.EvidencePath
		}
		keywords := keywordsForCriterion(c.Description)
		if len(keywords) == 0 {
			continue
		}
		evidence := findEvidence(keywords, prod)
		if evidence == "" {
			issues = append(issues, domain.Issue{
				Gate: domain.GateDrift, Severity: domain.SeverityWarning,
				Message: "criterion '" + c.ID + "' drifted: lost evidence at " + base.path,
				Path:    base.path,
				Hint:    "story=" + c.StoryID + " keywords=" + strings.Join(keywords, ",") + " — re-implement the behaviour or update the spec",
			})
			continue
		}
		// Same criterion still matches; check whether the file body
		// shrank significantly relative to the baseline.
		if base.size > driftMinTrackedBytes {
			currentSize := fileSize(p, evidence)
			if currentSize > 0 {
				delta := float64(base.size-currentSize) / float64(base.size)
				if delta > driftShrinkThreshold {
					issues = append(issues, domain.Issue{
						Gate: domain.GateDrift, Severity: domain.SeverityInfo,
						Message: "criterion '" + c.ID + "' evidence shrunk significantly — verify",
						Path:    evidence,
						Hint:    "previous size=" + itoaPositive(base.size) + "B now=" + itoaPositive(currentSize) + "B — confirm the criterion is still satisfied",
					})
				}
			}
		}
	}
	return issues
}

// fileSize returns the byte length of the file at path on the project,
// matching against the same case-insensitive convention as fileBody.
// Zero when no file matches.
func fileSize(p *domain.Project, path string) int {
	want := strings.ToLower(strings.TrimPrefix(path, "/"))
	for _, f := range p.Files {
		if strings.ToLower(strings.TrimPrefix(f.Path, "/")) == want {
			if f.Size > 0 {
				return f.Size
			}
			return len(f.Content)
		}
	}
	return 0
}
