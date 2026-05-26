// Indexer walks a repository root and produces Capability entries for
// every exported Go function and every exported TS/TSX symbol it
// finds. The walk is intentionally lightweight — `go/parser` + a
// regex-based TS extractor — because parsing TypeScript with Go would
// drag in a heavy AST dependency the repo refuses.
//
// The Indexer is the producer; the Store is the sink. The Reuse-First
// Preflight is the consumer.

package atlas

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/ai/embeddings"
)

// Indexer walks a repo and pushes capabilities into a Store.
type Indexer struct {
	Store Store
	Embed embeddings.Embedder
	Root  string
	// MaxFiles caps the walk so a runaway repo or accidental symlink
	// can't pin the indexer. Zero means no cap.
	MaxFiles int
	// SkipDirs lists directory names to prune on the walk. Defaults to
	// node_modules, .git, dist, build, .next, vendor, tmp, .ironflyer.
	SkipDirs []string
}

// defaultSkip is the set of directory names every Atlas walk prunes.
// We deliberately skip `vendor/` and `node_modules/` — those holdings
// are not "our reusable surface".
func defaultSkip() map[string]struct{} {
	return map[string]struct{}{
		"node_modules": {}, ".git": {}, "dist": {}, "build": {},
		".next": {}, "vendor": {}, "tmp": {}, ".ironflyer": {},
		".turbo": {}, ".cache": {}, "out": {}, "coverage": {},
	}
}

// IndexRepo walks Root and emits one Capability per exported symbol.
// Returns aggregate Stats after the walk completes. Errors from
// individual files are logged via the Store's error channel (when
// available) — a single malformed file should never abort the whole
// walk.
func (i *Indexer) IndexRepo(ctx context.Context) (Stats, error) {
	if i.Store == nil {
		return Stats{}, ErrNotFound // re-using; caller wires a real store
	}
	skip := i.skipSet()
	caps := make([]Capability, 0, 1024)
	var visited int
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, drop := skip[d.Name()]; drop {
				return filepath.SkipDir
			}
			return nil
		}
		if i.MaxFiles > 0 && visited >= i.MaxFiles {
			return filepath.SkipAll
		}
		visited++
		rel, _ := filepath.Rel(i.Root, path)
		switch {
		case strings.HasSuffix(path, ".go"):
			caps = append(caps, indexGoFile(path, rel)...)
		case strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx"):
			caps = append(caps, indexTSFile(path, rel)...)
		}
		return nil
	}
	if err := filepath.WalkDir(i.Root, walkFn); err != nil {
		return Stats{}, err
	}
	// Embed lazily — the embed call is the expensive bit; we keep it
	// out of the hot walk path so a slow HF endpoint doesn't blow up
	// the indexer's cycle time.
	if i.Embed != nil {
		i.embedAll(ctx, caps)
	}
	if err := i.Store.BatchIndex(ctx, caps); err != nil {
		return Stats{}, err
	}
	return i.Store.Stats(ctx)
}

// IndexPatch re-indexes the given paths. Used by the
// patch.Engine.OnApplied hook so the Atlas stays current after every
// approved patch. Paths outside Root are silently skipped.
func (i *Indexer) IndexPatch(ctx context.Context, paths []string) error {
	if i.Store == nil || len(paths) == 0 {
		return nil
	}
	caps := make([]Capability, 0, len(paths)*4)
	for _, p := range paths {
		full := p
		if !filepath.IsAbs(p) {
			full = filepath.Join(i.Root, p)
		}
		switch {
		case strings.HasSuffix(p, ".go"):
			caps = append(caps, indexGoFile(full, p)...)
		case strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx"):
			caps = append(caps, indexTSFile(full, p)...)
		}
	}
	if i.Embed != nil {
		i.embedAll(ctx, caps)
	}
	return i.Store.BatchIndex(ctx, caps)
}

func (i *Indexer) skipSet() map[string]struct{} {
	if len(i.SkipDirs) == 0 {
		return defaultSkip()
	}
	out := map[string]struct{}{}
	for _, d := range i.SkipDirs {
		out[d] = struct{}{}
	}
	return out
}

// embedAll runs through caps and fills in Embedding via the configured
// embedder. Failures degrade silently — a capability without an
// embedding still serves lexical search.
func (i *Indexer) embedAll(ctx context.Context, caps []Capability) {
	if i.Embed == nil {
		return
	}
	for idx := range caps {
		text := strings.TrimSpace(caps[idx].Doc + "\n" + caps[idx].Signature + "\n" + caps[idx].Symbol)
		if text == "" {
			continue
		}
		vec, err := i.Embed.Embed(ctx, text)
		if err != nil {
			continue
		}
		caps[idx].Embedding = vec
	}
}

// ---- Go extractor --------------------------------------------------

func indexGoFile(fullPath, relPath string) []Capability {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fullPath, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	out := make([]Capability, 0, 4)
	pkgDoc := ""
	if file.Doc != nil {
		pkgDoc = file.Doc.Text()
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		if !fn.Name.IsExported() {
			continue
		}
		sig := renderGoSignature(fn)
		doc := ""
		if fn.Doc != nil {
			doc = fn.Doc.Text()
		}
		if doc == "" {
			doc = pkgDoc
		}
		cap := Capability{
			ID:          CapabilityID(relPath, fn.Name.Name),
			Path:        relPath,
			Symbol:      fn.Name.Name,
			Kind:        goKind(fn),
			Signature:   sig,
			Doc:         truncate(doc, 800),
			LastIndexed: time.Now().UTC(),
		}
		out = append(out, cap)
	}
	return out
}

func goKind(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		return "method"
	}
	return "func"
}

func renderGoSignature(fn *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		b.WriteString("(")
		b.WriteString(renderFieldList(fn.Recv))
		b.WriteString(") ")
	}
	b.WriteString(fn.Name.Name)
	b.WriteString("(")
	if fn.Type.Params != nil {
		b.WriteString(renderFieldList(fn.Type.Params))
	}
	b.WriteString(")")
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		b.WriteString(" (")
		b.WriteString(renderFieldList(fn.Type.Results))
		b.WriteString(")")
	}
	return b.String()
}

func renderFieldList(fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	parts := make([]string, 0, len(fl.List))
	for _, f := range fl.List {
		name := ""
		if len(f.Names) > 0 {
			names := make([]string, 0, len(f.Names))
			for _, n := range f.Names {
				names = append(names, n.Name)
			}
			name = strings.Join(names, ",") + " "
		}
		parts = append(parts, name+renderGoExpr(f.Type))
	}
	return strings.Join(parts, ", ")
}

func renderGoExpr(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return "*" + renderGoExpr(x.X)
	case *ast.SelectorExpr:
		return renderGoExpr(x.X) + "." + x.Sel.Name
	case *ast.ArrayType:
		return "[]" + renderGoExpr(x.Elt)
	case *ast.MapType:
		return "map[" + renderGoExpr(x.Key) + "]" + renderGoExpr(x.Value)
	case *ast.Ellipsis:
		return "..." + renderGoExpr(x.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	default:
		return "?"
	}
}

// ---- TS / TSX extractor --------------------------------------------

// tsExportRe captures `export function|class|const|let|var Name`. The
// patterns are deliberately coarse — parsing TS in Go would drag in a
// heavyweight dependency. We accept some false positives in exchange
// for a 30-line extractor.
var (
	tsExportFuncRe  = regexp.MustCompile(`(?m)^export\s+(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	tsExportConstRe = regexp.MustCompile(`(?m)^export\s+(?:const|let|var)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	tsExportClassRe = regexp.MustCompile(`(?m)^export\s+(?:abstract\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	tsHookRe        = regexp.MustCompile(`^use[A-Z][A-Za-z0-9_]*$`)
	tsComponentRe   = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)
)

func indexTSFile(fullPath, relPath string) []Capability {
	raw, err := readFileLimited(fullPath, 256*1024)
	if err != nil {
		return nil
	}
	src := string(raw)
	seen := make(map[string]struct{})
	out := make([]Capability, 0, 4)

	add := func(symbol string, kind string) {
		if symbol == "" {
			return
		}
		if _, dup := seen[symbol]; dup {
			return
		}
		seen[symbol] = struct{}{}
		out = append(out, Capability{
			ID:          CapabilityID(relPath, symbol),
			Path:        relPath,
			Symbol:      symbol,
			Kind:        kind,
			Signature:   firstLineWith(src, symbol),
			Doc:         leadingComment(src, symbol),
			LastIndexed: time.Now().UTC(),
		})
	}

	for _, m := range tsExportFuncRe.FindAllStringSubmatch(src, -1) {
		name := m[1]
		kind := "func"
		if tsHookRe.MatchString(name) {
			kind = "hook"
		} else if tsComponentRe.MatchString(name) && strings.HasSuffix(relPath, ".tsx") {
			kind = "component"
		}
		add(name, kind)
	}
	for _, m := range tsExportConstRe.FindAllStringSubmatch(src, -1) {
		name := m[1]
		kind := "const"
		if tsHookRe.MatchString(name) {
			kind = "hook"
		} else if tsComponentRe.MatchString(name) && strings.HasSuffix(relPath, ".tsx") {
			kind = "component"
		}
		add(name, kind)
	}
	for _, m := range tsExportClassRe.FindAllStringSubmatch(src, -1) {
		add(m[1], "class")
	}
	return out
}

func firstLineWith(src, needle string) string {
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, needle) {
			t := strings.TrimSpace(line)
			if len(t) > 200 {
				t = t[:200]
			}
			return t
		}
	}
	return ""
}

// leadingComment returns the JSDoc / `//` block that immediately
// precedes the first line mentioning `symbol`. Best-effort; empty
// when we can't find a match.
func leadingComment(src, symbol string) string {
	lines := strings.Split(src, "\n")
	target := -1
	for i, line := range lines {
		if strings.Contains(line, symbol) && strings.Contains(line, "export") {
			target = i
			break
		}
	}
	if target <= 0 {
		return ""
	}
	// Walk upward collecting comment lines.
	var out []string
	for j := target - 1; j >= 0; j-- {
		l := strings.TrimSpace(lines[j])
		if l == "" {
			break
		}
		if strings.HasPrefix(l, "//") || strings.HasPrefix(l, "*") || strings.HasPrefix(l, "/**") || strings.HasPrefix(l, "*/") {
			out = append([]string{l}, out...)
			continue
		}
		break
	}
	doc := strings.Join(out, " ")
	if len(doc) > 800 {
		doc = doc[:800]
	}
	return doc
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
