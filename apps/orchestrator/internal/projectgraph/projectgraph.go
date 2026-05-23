// Package projectgraph derives a dependency graph from a Project's source
// tree. It parses imports/exports with simple stdlib regex (no third-party
// dependencies) and resolves internal edges by walking the in-memory
// FileNode slice — there is no filesystem access.
//
// The graph is intentionally lossy: it is fast, language-agnostic at the
// edges, and intended for visualisation, impact-analysis, and AI context
// retrieval rather than precise compiler-grade resolution.
//
// Limitations worth knowing:
//   - Go internal edges are not resolved. The Ironflyer-generated context
//     rarely includes Go inside a Project's Files, and module-aware
//     resolution would require parsing go.mod. All Go imports are flagged
//     as "<external>" and no edge is emitted.
//   - TS/JS path aliases (tsconfig "paths") are ignored — only relative
//     and absolute path imports resolve to internal edges.
//   - Files larger than 256 KiB are emitted as nodes but not parsed.
//   - Vendor / build directories (node_modules, dist, build, .next,
//     vendor) are skipped entirely — they don't even get a node.
package projectgraph

import (
	"context"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"ironflyer/apps/orchestrator/internal/domain"
)

// Graph is the derived dependency view of a project's source tree.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node is one source file in the graph.
type Node struct {
	Path        string   `json:"path"`
	Language    string   `json:"language"` // "ts" | "go" | "py" | "other"
	Exports     []string `json:"exports,omitempty"`
	SymbolCount int      `json:"symbolCount,omitempty"`
}

// Edge is one resolved import from one project file to another.
type Edge struct {
	From string `json:"from"` // path of the importer
	To   string `json:"to"`   // resolved path of the importee
	Raw  string `json:"raw"`  // the literal import string (for debugging)
}

const maxParseBytes = 256 * 1024

var skipDirPrefixes = []string{
	"node_modules/",
	"dist/",
	"build/",
	".next/",
	"vendor/",
}

// Regexes are compiled once at package init.
var (
	// TS/JS
	reTSImportFrom = regexp.MustCompile(`(?m)^\s*import\s+.*?\s+from\s+['"]([^'"]+)['"]`)
	reTSImportBare = regexp.MustCompile(`(?m)^\s*import\s+['"]([^'"]+)['"]`)
	reTSRequire    = regexp.MustCompile(`require\(\s*['"]([^'"]+)['"]\s*\)`)
	reTSExportName = regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?(?:function|const|class|interface|type|enum|let|var)\s+(\w+)`)
	reTSExportDef  = regexp.MustCompile(`(?m)^\s*export\s+default\b`)

	// Go
	reGoImportSingle = regexp.MustCompile(`(?m)^\s*import\s+"([^"]+)"`)
	reGoImportBlock  = regexp.MustCompile(`(?s)import\s*\(([^)]+)\)`)
	reGoImportLine   = regexp.MustCompile(`"([^"]+)"`)
	reGoFunc         = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s*)?(\w+)`)
	reGoType         = regexp.MustCompile(`(?m)^type\s+(\w+)`)
	reGoConst        = regexp.MustCompile(`(?m)^const\s+(\w+)`)
	reGoVar          = regexp.MustCompile(`(?m)^var\s+(\w+)`)

	// Python
	rePyFromImport = regexp.MustCompile(`(?m)^\s*from\s+([\w.]+)\s+import`)
	rePyImport     = regexp.MustCompile(`(?m)^\s*import\s+([\w.]+)`)
	rePyDef        = regexp.MustCompile(`(?m)^def\s+(\w+)`)
	rePyClass      = regexp.MustCompile(`(?m)^class\s+(\w+)`)
)

// Build parses every file in the project and returns its dependency graph.
// Context is honoured so very large projects can be cancelled mid-parse;
// on cancellation we return the partial graph collected so far.
func Build(ctx context.Context, p *domain.Project) Graph {
	if p == nil {
		return Graph{Nodes: []Node{}, Edges: []Edge{}}
	}

	// Filter the file list up-front so the worker pool only sees parseable
	// candidates and so skipped directories never become nodes.
	candidates := make([]domain.FileNode, 0, len(p.Files))
	for _, f := range p.Files {
		if isSkipped(f.Path) {
			continue
		}
		candidates = append(candidates, f)
	}

	// Build an index of project paths so resolution is O(1).
	idx := make(map[string]struct{}, len(candidates))
	for _, f := range candidates {
		idx[f.Path] = struct{}{}
	}

	type result struct {
		node  Node
		edges []Edge
	}

	jobs := make(chan domain.FileNode)
	results := make(chan result)

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				n, e := parseFile(f, idx)
				select {
				case <-ctx.Done():
					return
				case results <- result{node: n, edges: e}:
				}
			}
		}()
	}

	// Feeder: respect ctx so we exit cleanly on cancellation.
	go func() {
		defer close(jobs)
		for _, f := range candidates {
			select {
			case <-ctx.Done():
				return
			case jobs <- f:
			}
		}
	}()

	// Closer: once workers drain `jobs` they exit; then we close `results`.
	go func() {
		wg.Wait()
		close(results)
	}()

	var (
		mu    sync.Mutex
		nodes = make([]Node, 0, len(candidates))
		edges = make([]Edge, 0, len(candidates))
	)

	for r := range results {
		mu.Lock()
		nodes = append(nodes, r.node)
		edges = append(edges, r.edges...)
		mu.Unlock()
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Path < nodes[j].Path
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Raw < edges[j].Raw
	})

	return Graph{Nodes: nodes, Edges: edges}
}

// isSkipped returns true if the file's path is inside a vendor/build dir.
func isSkipped(p string) bool {
	for _, prefix := range skipDirPrefixes {
		if strings.HasPrefix(p, prefix) || strings.Contains(p, "/"+prefix) {
			return true
		}
	}
	return false
}

// detectLanguage maps a file extension to a coarse language label.
func detectLanguage(p string) string {
	ext := strings.ToLower(path.Ext(p))
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return "ts"
	case ".go":
		return "go"
	case ".py":
		return "py"
	default:
		return "other"
	}
}

// parseFile returns the Node for a single file and any internal edges that
// originate from it. Files over maxParseBytes get a node but no parsing.
func parseFile(f domain.FileNode, idx map[string]struct{}) (Node, []Edge) {
	lang := detectLanguage(f.Path)
	node := Node{Path: f.Path, Language: lang}

	if lang == "other" {
		return node, nil
	}
	if len(f.Content) > maxParseBytes {
		return node, nil
	}

	src := f.Content
	dir := path.Dir(f.Path)
	var (
		rawImports []string
		exports    []string
		edges      []Edge
	)

	switch lang {
	case "ts":
		rawImports = extractTSImports(src)
		exports = extractTSExports(src)
		for _, raw := range rawImports {
			to, ok := resolveTSImport(raw, dir, idx)
			if !ok {
				continue
			}
			edges = append(edges, Edge{From: f.Path, To: to, Raw: raw})
		}

	case "go":
		rawImports = extractGoImports(src)
		exports = extractGoExports(src)
		// Per package docs: Go internal resolution is intentionally skipped.
		// All Go imports resolve to "<external>" and emit no edge.

	case "py":
		rawImports = extractPyImports(src)
		exports = extractPyExports(src)
		for _, raw := range rawImports {
			to, ok := resolvePyImport(raw, idx)
			if !ok {
				continue
			}
			edges = append(edges, Edge{From: f.Path, To: to, Raw: raw})
		}
	}

	node.Exports = uniqStrings(exports)
	node.SymbolCount = len(node.Exports)
	return node, edges
}

// extractTSImports finds every module specifier in `import ... from "..."`,
// bare `import "..."`, and `require("...")`.
func extractTSImports(src string) []string {
	out := make([]string, 0, 8)
	for _, m := range reTSImportFrom.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	for _, m := range reTSImportBare.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	for _, m := range reTSRequire.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

// extractTSExports gathers named top-level exports plus "default" when
// `export default` is present anywhere in the file.
func extractTSExports(src string) []string {
	out := make([]string, 0, 8)
	for _, m := range reTSExportName.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	if reTSExportDef.MatchString(src) {
		out = append(out, "default")
	}
	return out
}

// extractGoImports handles both single-line and parenthesised import blocks.
func extractGoImports(src string) []string {
	out := make([]string, 0, 8)
	for _, m := range reGoImportSingle.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	for _, m := range reGoImportBlock.FindAllStringSubmatch(src, -1) {
		block := m[1]
		for _, lm := range reGoImportLine.FindAllStringSubmatch(block, -1) {
			out = append(out, lm[1])
		}
	}
	return out
}

// extractGoExports collects exported (uppercase-leading) top-level symbols.
func extractGoExports(src string) []string {
	out := make([]string, 0, 8)
	pushIfExported := func(name string) {
		if name == "" {
			return
		}
		if c := name[0]; c >= 'A' && c <= 'Z' {
			out = append(out, name)
		}
	}
	for _, m := range reGoFunc.FindAllStringSubmatch(src, -1) {
		pushIfExported(m[1])
	}
	for _, m := range reGoType.FindAllStringSubmatch(src, -1) {
		pushIfExported(m[1])
	}
	for _, m := range reGoConst.FindAllStringSubmatch(src, -1) {
		pushIfExported(m[1])
	}
	for _, m := range reGoVar.FindAllStringSubmatch(src, -1) {
		pushIfExported(m[1])
	}
	return out
}

// extractPyImports captures the module name from both `from X import …` and
// plain `import X` lines (no aliasing or comma-list handling — we want one
// canonical name per matched line).
func extractPyImports(src string) []string {
	out := make([]string, 0, 8)
	for _, m := range rePyFromImport.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	for _, m := range rePyImport.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

// extractPyExports lists top-level `def` and `class` declarations.
func extractPyExports(src string) []string {
	out := make([]string, 0, 8)
	for _, m := range rePyDef.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	for _, m := range rePyClass.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

// tsCandidates returns the list of paths to probe when resolving a TS/JS
// import that lacks an explicit extension.
func tsCandidates(base string) []string {
	// If the path already has a recognised TS/JS extension, try it first
	// as-is — but still fall through to suffix variants in case the
	// importer wrote "./foo.css" and we can't resolve it; isInIndex will
	// reject the misses.
	ext := strings.ToLower(path.Ext(base))
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return []string{base}
	}
	return []string{
		base + ".ts",
		base + ".tsx",
		base + ".js",
		base + ".mjs",
		base + "/index.ts",
		base + "/index.tsx",
		base + "/index.js",
	}
}

// resolveTSImport maps a TS/JS module specifier to a project file path.
// Bare modules ("react") return ok=false and no edge is emitted.
func resolveTSImport(raw, importerDir string, idx map[string]struct{}) (string, bool) {
	if raw == "" {
		return "", false
	}
	var base string
	switch {
	case strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../"):
		base = path.Clean(path.Join(importerDir, raw))
	case strings.HasPrefix(raw, "/"):
		base = strings.TrimPrefix(raw, "/")
	default:
		// Bare module — external dependency.
		return "", false
	}

	for _, cand := range tsCandidates(base) {
		if _, ok := idx[cand]; ok {
			return cand, true
		}
	}
	return "", false
}

// resolvePyImport maps a dotted module name to either "foo/bar.py" or
// "foo/bar/__init__.py".
func resolvePyImport(raw string, idx map[string]struct{}) (string, bool) {
	if raw == "" {
		return "", false
	}
	slashed := strings.ReplaceAll(raw, ".", "/")
	candidates := []string{
		slashed + ".py",
		slashed + "/__init__.py",
	}
	for _, cand := range candidates {
		if _, ok := idx[cand]; ok {
			return cand, true
		}
	}
	return "", false
}

// uniqStrings preserves first-occurrence order while removing duplicates.
func uniqStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
