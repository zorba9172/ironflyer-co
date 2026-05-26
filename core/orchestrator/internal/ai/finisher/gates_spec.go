// Package finisher — Spec-gate acceptance-criteria validator.
//
// The base Spec gate only verifies that the spec text exists and that every
// story has at least one acceptance line. Commercial customers need more:
// a guarantee that every acceptance criterion is actually addressed by the
// generated code, AND a signal when nothing tests that the addressing
// code works.
//
// This file extracts the spec's acceptance lines into structured
// `AcceptanceCriterion` records and matches them against the workspace.
// Verdict semantics, applied per criterion:
//
//   - Unaddressed                 → fail (a missing requirement is a bug)
//   - Addressed, no automated test → warn (caller may still ship)
//   - Addressed + tested          → pass
//
// The match heuristic is intentionally lightweight (keyword grep across the
// file body, plus a per-keyword presence check across test files) so the
// gate runs offline against the in-memory project tree. An LLM tiebreaker
// could refine this later — for now, false positives are biased toward
// "pass" so we don't fail a project on a synonym mismatch.

package finisher

import (
	"context"
	"regexp"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// filePos is one (path, lowercased-body) tuple consumed by findEvidence.
// Declared at file scope so the helper can take []filePos by value.
type filePos struct {
	path  string
	lower string
}

// validateAcceptanceCriteria walks every UserStory's Acceptance list and
// returns one Issue per unaddressed criterion (error severity), plus one
// per addressed-but-untested criterion (warning severity).
func validateAcceptanceCriteria(_ context.Context, env *GateEnv) []domain.Issue {
	if env == nil || env.Project == nil {
		return nil
	}
	p := env.Project
	// Skip projects with no files yet — the Code gate will fail first and
	// running the Spec match here would emit noise saying "everything is
	// unaddressed" for an obvious reason.
	if len(p.Files) == 0 {
		return nil
	}

	criteria := ExtractAcceptanceCriteria(p)
	if len(criteria) == 0 {
		return nil
	}

	// Build a corpus of (non-test) file bodies and a parallel test corpus.
	// We lowercase once so the per-criterion loop is O(1) on the
	// case-folding.
	var prodCorpus, testCorpus strings.Builder
	prod := make([]filePos, 0, len(p.Files))
	tests := make([]filePos, 0)
	for _, f := range p.Files {
		low := strings.ToLower(f.Content)
		if isTestFile(f.Path) {
			testCorpus.WriteString(low)
			testCorpus.WriteByte('\n')
			tests = append(tests, filePos{path: f.Path, lower: low})
		} else {
			prodCorpus.WriteString(low)
			prodCorpus.WriteByte('\n')
			prod = append(prod, filePos{path: f.Path, lower: low})
		}
	}

	var issues []domain.Issue
	for _, c := range criteria {
		keywords := keywordsForCriterion(c.Description)
		if len(keywords) == 0 {
			continue
		}
		evidence := findEvidence(keywords, prod)
		if evidence == "" {
			issues = append(issues, domain.Issue{
				Gate: domain.GateSpec, Severity: domain.SeverityError,
				Message: "acceptance criterion not addressed in code: " + truncateForMsg(c.Description, 120),
				Hint:    "story=" + c.StoryID + " keywords=" + strings.Join(keywords, ","),
			})
			continue
		}
		if findEvidence(keywords, tests) == "" {
			issues = append(issues, domain.Issue{
				Gate: domain.GateSpec, Severity: domain.SeverityWarning,
				Message: "acceptance criterion addressed but not covered by an automated test: " +
					truncateForMsg(c.Description, 120),
				Path: evidence,
				Hint: "story=" + c.StoryID + " — Tester should add coverage",
			})
		}
	}
	return issues
}

// ExtractAcceptanceCriteria flattens every UserStory's Acceptance list
// into a slice of AcceptanceCriterion records with stable, deterministic
// IDs. Exposed so the GraphQL resolver layer (or a future API) can hand
// the structured list to the UI without re-deriving it.
func ExtractAcceptanceCriteria(p *domain.Project) []domain.AcceptanceCriterion {
	if p == nil {
		return nil
	}
	var out []domain.AcceptanceCriterion
	for _, s := range p.Spec.UserStories {
		for i, a := range s.Acceptance {
			line := strings.TrimSpace(a)
			if line == "" {
				continue
			}
			out = append(out, domain.AcceptanceCriterion{
				ID:          s.ID + "-AC" + itoaPositive(i+1),
				StoryID:     s.ID,
				Description: line,
			})
		}
	}
	return out
}

// isTestFile is a coarse heuristic that recognises the major test-file
// conventions: Go (_test.go), JS/TS (.test.* / .spec.*), pytest
// (test_*.py / *_test.py), Rust (tests/ dir).
func isTestFile(path string) bool {
	low := strings.ToLower(path)
	switch {
	case strings.HasSuffix(low, "_test.go"):
		return true
	case strings.Contains(low, ".test."), strings.Contains(low, ".spec."):
		return true
	case strings.Contains(low, "/tests/"), strings.HasPrefix(low, "tests/"):
		return true
	case strings.HasPrefix(filepathBase(low), "test_"):
		return true
	case strings.HasSuffix(low, "_test.py"):
		return true
	}
	return false
}

// filepathBase returns the final path segment (no slash). We avoid pulling
// in path/filepath here because every other helper in this package is
// stdlib-strings-only for portability across OSes.
func filepathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// keywordTokenRe extracts word-like tokens from a free-form acceptance
// sentence. We keep tokens >= 4 characters so trivial English glue words
// don't dominate the match. Lowercased at extraction time.
var keywordTokenRe = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9_]{3,}`)

// stopwords drops common English filler so an acceptance sentence like
// "user can see the list of orders" matches against "list" / "orders"
// rather than "user" / "can" / "see" / "the". Lowercased.
var stopwords = map[string]struct{}{
	"user": {}, "users": {}, "with": {}, "this": {}, "that": {},
	"from": {}, "into": {}, "when": {}, "then": {}, "must": {},
	"will": {}, "should": {}, "shall": {}, "have": {}, "they": {},
	"their": {}, "there": {}, "which": {}, "what": {}, "where": {},
	"about": {}, "also": {}, "some": {}, "more": {}, "very": {},
	"like": {}, "make": {}, "such": {}, "even": {}, "able": {},
}

// keywordsForCriterion returns up to 6 distinctive keywords from a
// criterion description. The cap keeps matching cheap; the long-tail
// keywords are usually the most product-specific anyway.
func keywordsForCriterion(desc string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 6)
	for _, tok := range keywordTokenRe.FindAllString(desc, -1) {
		low := strings.ToLower(tok)
		if _, drop := stopwords[low]; drop {
			continue
		}
		if _, dup := seen[low]; dup {
			continue
		}
		seen[low] = struct{}{}
		out = append(out, low)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// findEvidence returns the first file path whose contents contain ALL of
// the given keywords (logical AND). Returns "" when no single file
// matches — we deliberately do not OR across files because that would
// over-count: one file naming "list" and another naming "orders" doesn't
// prove the workspace implements "list of orders".
func findEvidence(keywords []string, files []filePos) string {
	if len(keywords) == 0 || len(files) == 0 {
		return ""
	}
	for _, f := range files {
		all := true
		for _, k := range keywords {
			if !strings.Contains(f.lower, k) {
				all = false
				break
			}
		}
		if all {
			return f.path
		}
	}
	return ""
}

// truncateForMsg returns s capped at n bytes, with an ellipsis when
// truncation actually applied. Used to keep Issue messages bounded so
// dashboards render cleanly.
func truncateForMsg(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
