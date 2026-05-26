// Package refactor implements the Refactor Proposer (playbook §8.6) —
// the differentiator from Lovable / Bolt / v0 / Cursor. Most copilots
// SURFACE duplication ("hey, you have the same 12-line block in three
// files"); Ironflyer PROPOSES the fix: a concrete patch that extracts
// the duplicated block into a shared util and rewrites every site to
// call it.
//
// The proposer is intentionally conservative. It refuses to touch a
// finding when:
//
//   - any site is shorter than minSiteLines (default 5) — small
//     duplicates aren't worth a util,
//   - the sites are not byte-identical after whitespace normalisation
//     (the safe-merge bar; subtle differences are an LLM job, not a
//     mechanical extractor),
//   - the language can't be parsed mechanically (TS/TSX/JSX/.py — the
//     MVP emits a TODO Diff so a human / agent can finish the merge).
//
// The output is a Proposal: a target util path, the affected sites,
// a unified-diff patch, and a human-readable justification. The
// finisher's dedup gate consumes proposals via the `refactor` field on
// GateEnv (see gates_antibloat.go) and attaches them to the gate
// verdict's evidence so the operator review chain has the proposal
// inline.

package refactor

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// minSiteLines is the floor below which the proposer refuses to act.
// Playbook §8.6 calls 5 lines the smallest duplicate worth extracting.
const minSiteLines = 5

// Proposal is what the Refactor Proposer produces in response to a
// duplication finding. The shape is intentionally compact: the util
// path tells the operator where the shared code will land, Sites
// names the affected files, Diff is a unified-diff patch the operator
// can review verbatim, and Justification explains why the refactor is
// safe.
type Proposal struct {
	TargetUtilPath string `json:"targetUtilPath"`
	Sites          []Site `json:"sites"`
	Diff           string `json:"diff"`
	Justification  string `json:"justification"`
	// Language is the inferred language of the sites — "go", "ts",
	// "tsx", "py", or "" when unknown. Callers can branch on it to
	// decide whether to apply the Diff mechanically or hand it to a
	// human for review.
	Language string `json:"language,omitempty"`
}

// Site is one location where the duplicated block appears. Lines is
// [start, end] inclusive, 1-indexed (jscpd's convention). Body is the
// raw substring as it appears at that location.
type Site struct {
	Path  string `json:"path"`
	Lines [2]int `json:"lines"`
	Body  string `json:"body"`
}

// Finding is the input shape — same as the jscpd/dupl output the
// finisher's DedupGate already parses (gates_antibloat.go). The
// proposer only needs the Sites + a stable Hash to key the util name
// on; everything else is recovered from the file paths.
type Finding struct {
	Sites []Site `json:"sites"`
	Hash  string `json:"hash"`
}

// Service is the Refactor Proposer. Root is the repository root used
// to anchor target util paths and resolve site files when the proposer
// wants to load the surrounding context (a follow-up).
type Service struct {
	Root string
}

// NewService returns a fresh proposer rooted at the given repository
// directory. Pass "" to disable absolute-path normalisation (proposals
// will use the literal paths from the Finding).
func NewService(root string) *Service {
	return &Service{Root: root}
}

// Propose returns a refactor Proposal for the given finding, or nil
// when the finding is too small / inconsistent to safely merge. The
// non-nil-error return is reserved for genuine internal failures
// (failed parse, empty hash, ...) — "this finding isn't worth a
// refactor" is signalled by (nil, nil).
func (s *Service) Propose(_ context.Context, f Finding) (*Proposal, error) {
	if s == nil {
		return nil, fmt.Errorf("refactor: nil service")
	}
	if len(f.Sites) < 2 {
		return nil, nil
	}
	// Validate every site has ≥ minSiteLines and matching content.
	for _, site := range f.Sites {
		if siteLineCount(site) < minSiteLines {
			return nil, nil
		}
	}
	first := normaliseBody(f.Sites[0].Body)
	if first == "" {
		return nil, nil
	}
	for _, site := range f.Sites[1:] {
		if normaliseBody(site.Body) != first {
			// Subtly different blocks — let an LLM-driven refactor
			// pass take a swing, not the mechanical proposer.
			return nil, nil
		}
	}

	lang := inferLanguage(f.Sites)
	target := s.chooseTargetUtilPath(f.Sites, f.Hash, lang)
	funcName := extractedFuncName(f.Hash)

	utilBody, diff, just := s.buildExtraction(f, lang, target, funcName)
	if utilBody == "" {
		return nil, nil
	}

	return &Proposal{
		TargetUtilPath: target,
		Sites:          f.Sites,
		Diff:           diff,
		Justification:  just,
		Language:       lang,
	}, nil
}

// siteLineCount returns the line count of a site. We prefer the
// explicit Lines pair when present; otherwise we count newlines in
// Body.
func siteLineCount(s Site) int {
	if s.Lines[1] >= s.Lines[0] && s.Lines[0] > 0 {
		return s.Lines[1] - s.Lines[0] + 1
	}
	if s.Body == "" {
		return 0
	}
	return strings.Count(s.Body, "\n") + 1
}

// normaliseBody collapses internal whitespace runs and trims each line
// so two sites that differ only in indentation hash to the same
// canonical form. This is the safety bar for "are these blocks
// functionally identical" — a more aggressive normalisation (token
// stream, AST hash) would catch more cases but also risks merging
// blocks the proposer can't safely combine.
func normaliseBody(body string) string {
	var b strings.Builder
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		b.WriteString(trimmed)
		b.WriteByte('\n')
	}
	return b.String()
}

// inferLanguage picks the language by file extension. Disagreements
// across sites (mixed Go + TS) return "" — the proposer falls back to
// the TODO-comment path for cross-language clones (which are almost
// always false positives in practice).
func inferLanguage(sites []Site) string {
	if len(sites) == 0 {
		return ""
	}
	first := strings.ToLower(strings.TrimPrefix(filepath.Ext(sites[0].Path), "."))
	for _, s := range sites[1:] {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(s.Path), "."))
		if ext != first {
			return ""
		}
	}
	switch first {
	case "go":
		return "go"
	case "ts":
		return "ts"
	case "tsx":
		return "tsx"
	case "js":
		return "js"
	case "jsx":
		return "jsx"
	case "py":
		return "py"
	}
	return first
}

// chooseTargetUtilPath picks where the extracted util will live.
//
// Strategy:
//
//	1. If every site shares a common parent directory, the util lands
//	   under <commonParent>/<stem>_<hash>.<ext>. This keeps the
//	   extracted util close to its first user.
//	2. Otherwise we default to internal/pkg/extracted/<stem>.<ext> so
//	   the operator immediately sees the proposal is cross-cutting.
func (s *Service) chooseTargetUtilPath(sites []Site, hash, lang string) string {
	if len(sites) == 0 {
		return ""
	}
	common := commonParent(sites)
	stem := stemFromHash(hash)
	ext := extForLang(lang)
	if common != "" {
		base := filepath.Base(common)
		// Heuristic: if the common parent already looks like a "pkg"
		// directory (httputil, db, etc.), drop into it; otherwise
		// nest into a `shared` sibling so we don't pollute domain
		// folders with util files.
		if strings.HasSuffix(base, "util") || strings.HasSuffix(base, "utils") ||
			strings.HasSuffix(base, "pkg") || strings.HasSuffix(base, "helpers") {
			return filepath.ToSlash(filepath.Join(common, stem+ext))
		}
		return filepath.ToSlash(filepath.Join(common, "shared", stem+ext))
	}
	return "core/orchestrator/internal/pkg/extracted/" + stem + ext
}

// commonParent returns the longest directory prefix shared by every
// site, or "" when the sites diverge at the root.
func commonParent(sites []Site) string {
	if len(sites) == 0 {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(filepath.Dir(sites[0].Path)), "/")
	for _, s := range sites[1:] {
		other := strings.Split(filepath.ToSlash(filepath.Dir(s.Path)), "/")
		n := len(parts)
		if len(other) < n {
			n = len(other)
		}
		i := 0
		for i < n && parts[i] == other[i] {
			i++
		}
		parts = parts[:i]
		if len(parts) == 0 {
			return ""
		}
	}
	return strings.Join(parts, "/")
}

// stemFromHash makes a stable, readable identifier from the finding's
// hash. We take the first 8 hex chars; jscpd hashes are already short
// enough but this also tolerates non-hex hashes (e.g., dupl's "fp" ids).
func stemFromHash(hash string) string {
	h := strings.TrimSpace(hash)
	if h == "" {
		return "extracted"
	}
	if len(h) > 8 {
		h = h[:8]
	}
	return "extracted_" + h
}

// extractedFuncName turns the hash into a CamelCase function name so
// the extracted util has a stable, mentionable identifier (the
// playbook's "ExtractedXxx" naming convention).
func extractedFuncName(hash string) string {
	h := strings.TrimSpace(hash)
	if h == "" {
		return "Extracted"
	}
	if len(h) > 8 {
		h = h[:8]
	}
	// Replace any non-alnum so even "fp:" prefixed ids land as a valid
	// Go identifier.
	var b strings.Builder
	b.WriteString("Extracted")
	upNext := true
	for _, r := range h {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			if upNext {
				if r >= 'a' && r <= 'z' {
					r = r - 'a' + 'A'
				}
				upNext = false
			}
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			upNext = true
		}
	}
	return b.String()
}

func extForLang(lang string) string {
	switch lang {
	case "go":
		return ".go"
	case "ts":
		return ".ts"
	case "tsx":
		return ".tsx"
	case "js":
		return ".js"
	case "jsx":
		return ".jsx"
	case "py":
		return ".py"
	case "":
		return ".txt"
	}
	return "." + lang
}

// buildExtraction produces (utilBody, unifiedDiff, justification). For
// Go the util is a real `func ExtractedXxx() { ... }` wrapper around
// the block. For TS / TSX / JS / Python we emit a stub + a TODO note
// in the diff so a human can finish the merge — the proposer doesn't
// have an AST for those languages yet.
func (s *Service) buildExtraction(f Finding, lang, target, funcName string) (utilBody, diff, justification string) {
	totalLines := 0
	for _, st := range f.Sites {
		totalLines += siteLineCount(st)
	}
	dupLines := siteLineCount(f.Sites[0])
	savedLines := dupLines * (len(f.Sites) - 1)

	switch lang {
	case "go":
		utilBody = buildGoUtil(target, funcName, f.Sites[0].Body)
		diff = buildGoDiff(target, utilBody, f.Sites, funcName)
	default:
		utilBody = buildGenericUtil(target, funcName, lang, f.Sites[0].Body)
		diff = buildGenericDiff(target, utilBody, f.Sites, funcName, lang)
	}

	justification = fmt.Sprintf(
		"Extracted %d occurrences of clone %s into %s. Total duplicated lines: %d; lines saved: %d.",
		len(f.Sites), shortHash(f.Hash), target, totalLines, savedLines,
	)
	return utilBody, diff, justification
}

// buildGoUtil writes a tiny `package extracted` wrapper around the
// block. The body is indented under a `func ExtractedXxx() { ... }`
// declaration. The package name is taken from the parent directory.
func buildGoUtil(target, funcName, body string) string {
	pkg := goPackageNameFor(target)
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by the Refactor Proposer. DO NOT EDIT manually —\n")
	fmt.Fprintf(&b, "// review the propagated diff and resolve the call sites first.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	fmt.Fprintf(&b, "// %s wraps a block that was duplicated across the call\n", funcName)
	fmt.Fprintf(&b, "// sites listed in the accompanying refactor proposal.\n")
	fmt.Fprintf(&b, "func %s() {\n", funcName)
	// Indent every non-empty line under the function body.
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteByte('\n')
			continue
		}
		b.WriteByte('\t')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("}\n")
	return b.String()
}

// goPackageNameFor derives a legal Go package name from the parent
// directory of the target path. Hyphens collapse to underscores; an
// empty derivation falls back to "extracted".
func goPackageNameFor(target string) string {
	dir := filepath.Base(filepath.Dir(target))
	if dir == "" || dir == "." || dir == "/" {
		return "extracted"
	}
	var b strings.Builder
	for _, r := range dir {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		default:
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" {
		return "extracted"
	}
	return name
}

// buildGoDiff renders a unified-diff that:
//
//   - creates the util file with the extracted function,
//   - rewrites every site to call the extracted function.
//
// The diff format mirrors what `diff -u` would emit so reviewers can
// pipe it to `patch -p0` in a pinch. Sites are diff'd against an
// empty target body in the hunk (i.e. "remove these lines, add a call
// site") rather than reproduced verbatim — the proposer is the
// renderer of intent, not a structural editor.
func buildGoDiff(target, utilBody string, sites []Site, funcName string) string {
	var b strings.Builder
	// 1. Create the util file.
	fmt.Fprintf(&b, "--- /dev/null\n+++ %s\n", target)
	for _, line := range strings.Split(strings.TrimRight(utilBody, "\n"), "\n") {
		fmt.Fprintf(&b, "+%s\n", line)
	}
	pkg := goPackageNameFor(target)
	// 2. Rewrite each site.
	sortedSites := append([]Site(nil), sites...)
	sort.SliceStable(sortedSites, func(i, j int) bool { return sortedSites[i].Path < sortedSites[j].Path })
	for _, site := range sortedSites {
		fmt.Fprintf(&b, "--- %s\n+++ %s\n", site.Path, site.Path)
		fmt.Fprintf(&b, "@@ %d,%d @@ replace duplicated block with call to %s.%s\n",
			site.Lines[0], siteLineCount(site), pkg, funcName)
		for _, line := range strings.Split(strings.TrimRight(site.Body, "\n"), "\n") {
			fmt.Fprintf(&b, "-%s\n", line)
		}
		fmt.Fprintf(&b, "+\t%s.%s()\n", pkg, funcName)
	}
	return b.String()
}

// buildGenericUtil emits a stub + TODO comment for non-Go languages.
// The shape is enough for a reviewer to see the intent; an LLM-driven
// follow-up pass can refine the signature once it has the surrounding
// context.
func buildGenericUtil(target, funcName, lang, body string) string {
	var b strings.Builder
	switch lang {
	case "ts", "tsx", "js", "jsx":
		fmt.Fprintf(&b, "// TODO(refactor-proposer): hand-merge — the proposer\n")
		fmt.Fprintf(&b, "// extracted this block from %d sites but does not have\n", 0)
		fmt.Fprintf(&b, "// a TS/JS AST yet. Verify the signature, add imports, and\n")
		fmt.Fprintf(&b, "// rewrite each call site to import { %s }.\n\n", funcName)
		fmt.Fprintf(&b, "export function %s(): void {\n", funcName)
		for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				b.WriteByte('\n')
				continue
			}
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteString("}\n")
	case "py":
		fmt.Fprintf(&b, "# TODO(refactor-proposer): hand-merge — the proposer\n")
		fmt.Fprintf(&b, "# extracted this block from multiple sites but does not\n")
		fmt.Fprintf(&b, "# have a Python AST yet. Verify the signature, add\n")
		fmt.Fprintf(&b, "# imports, and rewrite each call site.\n\n")
		fmt.Fprintf(&b, "def %s() -> None:\n", funcName)
		for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				b.WriteByte('\n')
				continue
			}
			b.WriteString("    ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	default:
		fmt.Fprintf(&b, "// TODO(refactor-proposer): unsupported language %q —\n", lang)
		fmt.Fprintf(&b, "// review the sites manually.\n\n")
		b.WriteString(body)
	}
	return b.String()
}

// buildGenericDiff mirrors the Go diff but uses a generic call site
// (`call_to_xxx()`) and a TODO marker so the reviewer knows the
// rewrite isn't safe to apply blindly.
func buildGenericDiff(target, utilBody string, sites []Site, funcName, lang string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- /dev/null\n+++ %s\n", target)
	for _, line := range strings.Split(strings.TrimRight(utilBody, "\n"), "\n") {
		fmt.Fprintf(&b, "+%s\n", line)
	}
	for _, site := range sites {
		fmt.Fprintf(&b, "--- %s\n+++ %s\n", site.Path, site.Path)
		fmt.Fprintf(&b, "@@ %d,%d @@ TODO(refactor-proposer) replace with call to %s (%s)\n",
			site.Lines[0], siteLineCount(site), funcName, lang)
		for _, line := range strings.Split(strings.TrimRight(site.Body, "\n"), "\n") {
			fmt.Fprintf(&b, "-%s\n", line)
		}
		fmt.Fprintf(&b, "+// TODO call %s() — verify import + signature\n", funcName)
	}
	return b.String()
}

// shortHash returns the first 8 chars of hash, or the empty marker
// "<no-hash>" when hash is blank. Used only in the human-readable
// justification — never as an identifier.
func shortHash(hash string) string {
	h := strings.TrimSpace(hash)
	if h == "" {
		return "<no-hash>"
	}
	if len(h) > 8 {
		return h[:8]
	}
	return h
}
