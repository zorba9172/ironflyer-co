// Package ast exposes the tree-sitter-backed AST adapter contract the
// patch engine uses for symbol-level edits. Each supported language
// (.go .ts .tsx .py .rs) ships an Adapter implementation; the engine
// dispatches via AdapterFor(filename).
//
// The default build returns a no-op adapter (Supported() == false) so
// callers get a clean fallback instead of a hard error. Production
// images opt into the real tree-sitter backend with -tags treesitter
// — the build-tagged file `ast_treesitter.go` swaps in real adapters
// per language using github.com/smacker/go-tree-sitter.
//
// Why a dedicated package? The patch engine already has a single
// symbolApplier that handles in-file rewrites. AST adapters extend
// that with workspace-level operations — most importantly multi-file
// symbol rename, which iterates every file under the project tree and
// rewrites references in each language's idiomatic way.
package ast

import (
	"errors"
	"path"
	"strings"
)

// SymbolKind is the broad node-class an adapter resolves. The set
// covers Go / TS / Python / Rust; not every language exposes every
// kind ("class" is meaningless in Go; "method" requires a receiver).
type SymbolKind string

const (
	KindFunction  SymbolKind = "function"
	KindMethod    SymbolKind = "method"
	KindClass     SymbolKind = "class"
	KindStruct    SymbolKind = "struct"
	KindInterface SymbolKind = "interface"
	KindType      SymbolKind = "type"
	KindVar       SymbolKind = "var"
	KindConst     SymbolKind = "const"
)

// Adapter is the per-language contract. Implementations are pure: they
// take a source body and return a rewritten body without touching the
// filesystem. The engine handles store I/O.
type Adapter interface {
	// Language returns a stable label ("go", "ts", "tsx", "py", "rs")
	// used by telemetry + the fallback log message.
	Language() string

	// Supported is false for the no-op adapter (default build, no
	// treesitter tag). When false, callers must fall back to
	// anchor-based patches.
	Supported() bool

	// FindSymbol locates the named symbol in source and returns the
	// byte range its full declaration occupies. found is false when
	// the symbol isn't present; the engine then surfaces a clean
	// "symbol not found" issue.
	FindSymbol(source string, kind SymbolKind, name string) (start, end int, found bool, err error)

	// Rename rewrites every reference to oldName within source so it
	// reads as newName. The change is whole-identifier (won't touch
	// substring matches inside other identifiers, strings, or
	// comments). Returns the rewritten body. For the multi-file
	// project-wide rename, the engine calls Rename once per file.
	Rename(source string, kind SymbolKind, oldName, newName string) (string, error)

	// ReplaceBody swaps the resolved symbol's body with newBody. The
	// signature / declaration line stays.
	ReplaceBody(source string, kind SymbolKind, name, newBody string) (string, error)

	// InsertAfter appends newDecl immediately after the symbol's
	// closing delimiter — used to add a sibling function, method, or
	// type next to an existing one without finding a unique anchor.
	InsertAfter(source string, kind SymbolKind, name, newDecl string) (string, error)

	// Delete removes the symbol's full declaration (including any
	// preceding doc comment block when the grammar attaches one).
	Delete(source string, kind SymbolKind, name string) (string, error)
}

// AdapterFor returns the adapter for the file's language, or the no-op
// fallback when no language matches. The fallback returns Supported()
// == false so the engine logs a warning and falls back to anchor-based
// patches. Real adapters are registered by language code via the
// build-tagged init().
func AdapterFor(filename string) Adapter {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".go":
		return getAdapter("go")
	case ".ts":
		return getAdapter("ts")
	case ".tsx":
		return getAdapter("tsx")
	case ".py":
		return getAdapter("py")
	case ".rs":
		return getAdapter("rs")
	}
	return noop{}
}

// SupportedExtensions returns the file extensions for which an Adapter
// exists in the current build. Multi-file rename uses this to filter
// the workspace walk.
func SupportedExtensions() []string {
	out := []string{}
	for _, code := range []string{"go", "ts", "tsx", "py", "rs"} {
		if a := getAdapter(code); a != nil && a.Supported() {
			out = append(out, "."+code)
		}
	}
	return out
}

// registry is the per-build adapter map. Populated by ast_treesitter.go
// when the `treesitter` tag is set; empty otherwise.
var registry = map[string]Adapter{}

func register(code string, a Adapter) {
	registry[code] = a
}

func getAdapter(code string) Adapter {
	if a, ok := registry[code]; ok {
		return a
	}
	return noop{}
}

// noop is the default-build placeholder. It reports Supported() ==
// false so the engine routes around it; every mutator method returns
// the source unchanged with a "not enabled" error so callers can
// surface a uniform fallback message.
type noop struct{}

func (noop) Language() string  { return "noop" }
func (noop) Supported() bool   { return false }
func (noop) FindSymbol(string, SymbolKind, string) (int, int, bool, error) {
	return 0, 0, false, errAdapterDisabled
}
func (noop) Rename(s string, _ SymbolKind, _, _ string) (string, error) {
	return s, errAdapterDisabled
}
func (noop) ReplaceBody(s string, _ SymbolKind, _, _ string) (string, error) {
	return s, errAdapterDisabled
}
func (noop) InsertAfter(s string, _ SymbolKind, _, _ string) (string, error) {
	return s, errAdapterDisabled
}
func (noop) Delete(s string, _ SymbolKind, _ string) (string, error) {
	return s, errAdapterDisabled
}

// errAdapterDisabled is the sentinel the engine checks to decide
// whether to fall back to anchor-based patches versus surface a real
// error to the caller.
var errAdapterDisabled = errors.New("ast adapter not enabled in this build — rebuild with -tags treesitter")

// IsAdapterDisabled reports whether err is the not-enabled sentinel.
// The engine uses this to differentiate "fall back to anchor patches"
// from "the source has a real syntax error".
func IsAdapterDisabled(err error) bool {
	return errors.Is(err, errAdapterDisabled)
}
