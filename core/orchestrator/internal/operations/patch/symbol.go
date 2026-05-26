// Package patch — symbol-level AST patches.
//
// SymbolPatch (Op == OpSymbol) is a more robust alternative to anchor-
// based diffs: instead of "after line X / before line Y", the Coder
// names a symbol ("function Foo in bar.go") and the engine rewrites the
// affected AST node via tree-sitter. This is whitespace-immune and
// resilient to formatter drift.
//
// The AST applier is gated behind the `treesitter` build tag because
// the grammars are CGO + native deps. The default build keeps the
// always-compilable anchor-based engine and reports a clean fallback
// issue when a SymbolPatch arrives — at which point the Coder can
// retry with an anchor-patch.
//
// Lifecycle integration: at Propose-time resolveSymbolPatches walks
// every OpSymbol change, resolves the symbol against the current
// project file body, performs the requested action, and (on success)
// MATERIALISES the rewritten content into FileChange.Content + flips
// Op to OpUpdate. From that point on the patch is op-uniform — the
// runtime applier, gates, snapshot ring, and rollback path all see a
// normal full-file update. Symbol / SymbolAction / NewSource / Diff
// remain populated on the FileChange so the human review UI can show
// "modified function Foo".
package patch

import (
	"path"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// SymbolApplier rewrites a single source file by resolving a symbol
// reference against its parse tree and applying the requested action.
// The default no-op implementation is wired by NewEngine; the build
// tagged tree-sitter implementation replaces it via the init()
// function in symbol_treesitter.go.
type SymbolApplier interface {
	// Apply parses content (per filename's extension), locates sym,
	// performs action with newSource, and returns the rewritten file
	// body + a human-readable diff blurb for the review UI. ok is
	// false when the language/grammar is unsupported, in which case
	// the engine surfaces a clean fallback issue instead of erroring.
	Apply(filename, content string, sym SymbolRef, action SymbolAction, newSource string) (rewritten string, diff string, ok bool, err error)
}

// symbolApplier is the package-level applier. The default value is the
// fallback stub which always reports the patch is unsupported so the
// Coder can retry with an anchor-patch. The treesitter build replaces
// it at init().
var symbolApplier SymbolApplier = fallbackSymbolApplier{}

// fallbackSymbolApplier is the always-compilable default. It reports
// ok=false so callers know to surface a "symbol patches not enabled"
// issue and the Coder can fall back to anchor-patches.
type fallbackSymbolApplier struct{}

func (fallbackSymbolApplier) Apply(_, _ string, _ SymbolRef, _ SymbolAction, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

// resolveSymbolPatches walks the change list, materialises every
// OpSymbol change into an OpUpdate carrying the rewritten content +
// diff, and returns any issues encountered. Non-OpSymbol entries are
// left untouched.
//
// Failure modes are surfaced as domain.Issue entries (not Go errors)
// so they flow through the same rejection path as anchor failures —
// the Coder sees them in the next loop iteration and retries with a
// different patch shape.
func resolveSymbolPatches(proj *domain.Project, changes []FileChange) []domain.Issue {
	var issues []domain.Issue
	for i := range changes {
		c := &changes[i]
		if c.Op != OpSymbol {
			continue
		}
		if c.Symbol == nil || c.Symbol.Name == "" {
			// Shape errors are already caught by Validate; skip here so
			// we don't double-report.
			continue
		}
		if c.SymbolAction == "" {
			continue
		}
		// Find current file body in the project.
		body, found := lookupFileBody(proj, c.Path)
		if !found {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityError,
				Message:  "symbol patch target file does not exist",
				Path:     c.Path,
				Hint:     "either OpCreate the file first or correct the path",
			})
			continue
		}
		if !isSupportedSymbolLang(c.Path) {
			issues = append(issues, domain.Issue{
				Gate:     domain.GateCode,
				Severity: domain.SeverityError,
				Message:  "symbol patches are not supported for this language",
				Path:     c.Path,
				Hint:     "retry as an anchor-patch (OpReplace / OpInsertAfter) — symbol patches cover .go .ts .tsx .py .rs",
			})
			continue
		}
		rewritten, diff, ok, err := symbolApplier.Apply(c.Path, body, *c.Symbol, c.SymbolAction, c.NewSource)
		if err != nil {
			issues = append(issues, domain.Issue{
				Gate:     domain.GateCode,
				Severity: domain.SeverityError,
				Message:  "symbol patch failed: " + truncate(err.Error(), 240),
				Path:     c.Path,
				Hint:     "the file may have a syntax error, or the symbol may not exist — retry with an anchor-patch",
			})
			continue
		}
		if !ok {
			issues = append(issues, domain.Issue{
				Gate:     domain.GateCode,
				Severity: domain.SeverityError,
				Message:  "symbol patches are not enabled in this build",
				Path:     c.Path,
				Hint:     "rebuild the orchestrator with `-tags treesitter` for AST patches, or retry this patch as an anchor-patch (OpReplace / OpInsertAfter)",
			})
			continue
		}
		// Materialise: switch the change to a full-file Update carrying
		// the rewritten body. Preserve Symbol / SymbolAction / NewSource
		// / Diff so the UI can label the patch as symbol-derived.
		c.Op = OpUpdate
		c.Content = rewritten
		c.Diff = diff
	}
	return issues
}

// isSupportedSymbolLang returns true when the file extension is one
// of the languages the tree-sitter applier knows. We accept this set
// even in the default (no-tag) build so that "unsupported language"
// and "build tag missing" stay distinct issues — the Coder gets a
// clearer error message either way.
func isSupportedSymbolLang(filePath string) bool {
	switch strings.ToLower(path.Ext(filePath)) {
	case ".go", ".ts", ".tsx", ".py", ".rs":
		return true
	}
	return false
}

func lookupFileBody(proj *domain.Project, p string) (string, bool) {
	for _, f := range proj.Files {
		if f.Path == p {
			return f.Content, true
		}
	}
	return "", false
}
