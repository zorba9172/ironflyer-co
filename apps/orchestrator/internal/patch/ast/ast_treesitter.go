//go:build treesitter

// Package ast — tree-sitter-backed Adapter implementations. Compiled
// only with `-tags treesitter` so the default build stays CGO-free.
//
// Per-language adapters share `tsAdapter`, which centralises the
// boilerplate: parse, locate the named declaration, splice byte
// ranges. Each language plugs in a `grammar` describing the relevant
// node-type names (function declaration, body field, identifier
// field, doc-comment lookback rule).
package ast

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	tssctsx "github.com/smacker/go-tree-sitter/typescript/tsx"
	tsscts "github.com/smacker/go-tree-sitter/typescript/typescript"
)

func init() {
	register("go", newAdapter("go", golang.GetLanguage(), goGrammar()))
	register("ts", newAdapter("ts", tsscts.GetLanguage(), tsGrammar()))
	register("tsx", newAdapter("tsx", tssctsx.GetLanguage(), tsGrammar()))
	register("py", newAdapter("py", python.GetLanguage(), pyGrammar()))
	register("rs", newAdapter("rs", rust.GetLanguage(), rsGrammar()))
}

// grammar describes the per-language node-type vocabulary the adapter
// needs to do its job. We deliberately keep the map small — the four
// AST actions only need declaration types + body/name field names.
type grammar struct {
	// declTypes maps a SymbolKind onto the set of tree-sitter node
	// types that can hold a declaration of that kind. ("function" in
	// Go can be "function_declaration" or "method_declaration".)
	declTypes map[SymbolKind][]string
	// nameField is the field name on a declaration node that holds
	// the identifier. Almost always "name" but Python uses "name"
	// for both functions and classes.
	nameField string
	// bodyField is the field name on a declaration node that holds
	// the body. Go uses "body"; Python uses "body"; Rust uses "body";
	// TS uses "body".
	bodyField string
	// identifierType is the node-type the rename pass treats as a
	// reference site. The Rename implementation walks the tree and
	// rewrites every identifier whose text matches oldName.
	identifierType string
}

type tsAdapter struct {
	code string
	lang *sitter.Language
	g    grammar
}

func newAdapter(code string, lang *sitter.Language, g grammar) *tsAdapter {
	return &tsAdapter{code: code, lang: lang, g: g}
}

func (a *tsAdapter) Language() string { return a.code }
func (a *tsAdapter) Supported() bool  { return true }

func (a *tsAdapter) parse(source string) (*sitter.Tree, []byte, error) {
	src := []byte(source)
	p := sitter.NewParser()
	p.SetLanguage(a.lang)
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, nil, fmt.Errorf("tree-sitter parse: %w", err)
	}
	if tree == nil {
		return nil, nil, errors.New("tree-sitter returned nil tree")
	}
	return tree, src, nil
}

func (a *tsAdapter) FindSymbol(source string, kind SymbolKind, name string) (int, int, bool, error) {
	tree, src, err := a.parse(source)
	if err != nil {
		return 0, 0, false, err
	}
	defer tree.Close()
	n := a.findDecl(tree.RootNode(), src, kind, name)
	if n == nil {
		return 0, 0, false, nil
	}
	return int(n.StartByte()), int(n.EndByte()), true, nil
}

func (a *tsAdapter) ReplaceBody(source string, kind SymbolKind, name, newBody string) (string, error) {
	tree, src, err := a.parse(source)
	if err != nil {
		return source, err
	}
	defer tree.Close()
	n := a.findDecl(tree.RootNode(), src, kind, name)
	if n == nil {
		return source, fmt.Errorf("symbol %q not found", name)
	}
	body := n.ChildByFieldName(a.g.bodyField)
	if body == nil {
		return source, fmt.Errorf("symbol %q has no body to replace", name)
	}
	return string(src[:body.StartByte()]) + newBody + string(src[body.EndByte():]), nil
}

func (a *tsAdapter) InsertAfter(source string, kind SymbolKind, name, newDecl string) (string, error) {
	tree, src, err := a.parse(source)
	if err != nil {
		return source, err
	}
	defer tree.Close()
	n := a.findDecl(tree.RootNode(), src, kind, name)
	if n == nil {
		return source, fmt.Errorf("symbol %q not found", name)
	}
	end := n.EndByte()
	sep := "\n\n"
	return string(src[:end]) + sep + newDecl + string(src[end:]), nil
}

func (a *tsAdapter) Delete(source string, kind SymbolKind, name string) (string, error) {
	tree, src, err := a.parse(source)
	if err != nil {
		return source, err
	}
	defer tree.Close()
	n := a.findDecl(tree.RootNode(), src, kind, name)
	if n == nil {
		return source, fmt.Errorf("symbol %q not found", name)
	}
	start, end := n.StartByte(), n.EndByte()
	// Trim a trailing newline so the surrounding code doesn't end up
	// with a stray blank line.
	if int(end) < len(src) && src[end] == '\n' {
		end++
	}
	return string(src[:start]) + string(src[end:]), nil
}

func (a *tsAdapter) Rename(source string, _ SymbolKind, oldName, newName string) (string, error) {
	if oldName == "" || newName == "" || oldName == newName {
		return source, nil
	}
	tree, src, err := a.parse(source)
	if err != nil {
		return source, err
	}
	defer tree.Close()
	var sites []sitter.Range
	walk(tree.RootNode(), func(n *sitter.Node) bool {
		if n.Type() == a.g.identifierType && bytes.Equal(src[n.StartByte():n.EndByte()], []byte(oldName)) {
			sites = append(sites, sitter.Range{StartByte: n.StartByte(), EndByte: n.EndByte()})
		}
		return true
	})
	if len(sites) == 0 {
		return source, nil
	}
	// Walk sites in reverse-byte-order so earlier rewrites don't
	// shift the byte offsets of later ones.
	out := src
	for i := len(sites) - 1; i >= 0; i-- {
		s := sites[i]
		out = append(append(append([]byte{}, out[:s.StartByte]...), []byte(newName)...), out[s.EndByte:]...)
	}
	return string(out), nil
}

func (a *tsAdapter) findDecl(root *sitter.Node, src []byte, kind SymbolKind, name string) *sitter.Node {
	wantedTypes := a.g.declTypes[kind]
	// Empty kind → match any declaration type known to the grammar.
	if len(wantedTypes) == 0 {
		for _, ts := range a.g.declTypes {
			wantedTypes = append(wantedTypes, ts...)
		}
	}
	want := map[string]bool{}
	for _, t := range wantedTypes {
		want[t] = true
	}
	var hit *sitter.Node
	walk(root, func(n *sitter.Node) bool {
		if !want[n.Type()] {
			return true
		}
		nm := n.ChildByFieldName(a.g.nameField)
		if nm == nil {
			return true
		}
		if string(src[nm.StartByte():nm.EndByte()]) == name {
			hit = n
			return false
		}
		return true
	})
	return hit
}

func walk(n *sitter.Node, visit func(*sitter.Node) bool) bool {
	if n == nil {
		return true
	}
	if !visit(n) {
		return false
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		if !walk(n.NamedChild(i), visit) {
			return false
		}
	}
	return true
}

// ---- per-language grammar descriptors ----

func goGrammar() grammar {
	return grammar{
		declTypes: map[SymbolKind][]string{
			KindFunction:  {"function_declaration"},
			KindMethod:    {"method_declaration"},
			KindType:      {"type_declaration"},
			KindStruct:    {"type_declaration"},
			KindInterface: {"type_declaration"},
			KindVar:       {"var_declaration"},
			KindConst:     {"const_declaration"},
		},
		nameField:      "name",
		bodyField:      "body",
		identifierType: "identifier",
	}
}

func tsGrammar() grammar {
	return grammar{
		declTypes: map[SymbolKind][]string{
			KindFunction: {"function_declaration"},
			KindClass:    {"class_declaration"},
			KindMethod:   {"method_definition"},
			KindType:     {"type_alias_declaration"},
		},
		nameField:      "name",
		bodyField:      "body",
		identifierType: "identifier",
	}
}

func pyGrammar() grammar {
	return grammar{
		declTypes: map[SymbolKind][]string{
			KindFunction: {"function_definition"},
			KindMethod:   {"function_definition"},
			KindClass:    {"class_definition"},
		},
		nameField:      "name",
		bodyField:      "body",
		identifierType: "identifier",
	}
}

func rsGrammar() grammar {
	return grammar{
		declTypes: map[SymbolKind][]string{
			KindFunction: {"function_item"},
			KindStruct:   {"struct_item"},
			KindType:     {"type_item"},
		},
		nameField:      "name",
		bodyField:      "body",
		identifierType: "identifier",
	}
}
