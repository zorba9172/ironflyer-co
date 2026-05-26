//go:build treesitter

// Package patch — tree-sitter backed SymbolApplier.
//
// Compiled only when the `treesitter` build tag is set, because the
// per-grammar packages are CGO + native and pull in clang. Production
// images that want AST patches build with:
//
//	go build -tags treesitter ./...
//
// The default build keeps the always-compilable anchor-based engine
// and reports a clean fallback issue when a SymbolPatch arrives.
//
// Supported grammars: Go, TypeScript / TSX, Python, Rust. Each
// language's grammar exposes a slightly different node-type vocabulary
// so symbolNode dispatches per ext to find the named declaration and
// returns the body / signature / full byte ranges the four actions
// (replace_body, replace_signature, insert_after, delete) operate on.
package patch

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

func init() {
	symbolApplier = treesitterApplier{}
}

type treesitterApplier struct{}

func (treesitterApplier) Apply(filename, content string, sym SymbolRef, action SymbolAction, newSource string) (string, string, bool, error) {
	lang, ext, ok := languageFor(filename)
	if !ok {
		return "", "", false, nil
	}
	src := []byte(content)
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return "", "", true, fmt.Errorf("tree-sitter parse: %w", err)
	}
	if tree == nil {
		return "", "", true, errors.New("tree-sitter returned a nil tree")
	}
	defer tree.Close()
	root := tree.RootNode()
	if root.HasError() {
		return "", "", true, errors.New("source has syntax errors — fix them or retry with an anchor-patch")
	}

	hit, err := findSymbol(root, src, ext, sym)
	if err != nil {
		return "", "", true, err
	}
	if hit == nil {
		return "", "", true, fmt.Errorf("symbol %q not found", symbolLabel(sym))
	}

	startByte, endByte, err := actionRange(hit, action, ext)
	if err != nil {
		return "", "", true, err
	}

	// Splice [start, end) → newSource. For insert_after the range is a
	// zero-width point at the symbol's end byte.
	rewritten := string(src[:startByte]) + newSource + string(src[endByte:])
	diff := fmt.Sprintf("%s %s: %s (bytes %d-%d, %+d)", action, symbolLabel(sym), path.Base(filename), startByte, endByte, len(newSource)-(int(endByte)-int(startByte)))
	return rewritten, diff, true, nil
}

func languageFor(filename string) (*sitter.Language, string, bool) {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".go":
		return golang.GetLanguage(), ext, true
	case ".ts":
		return typescript.GetLanguage(), ext, true
	case ".tsx":
		return tsx.GetLanguage(), ext, true
	case ".py":
		return python.GetLanguage(), ext, true
	case ".rs":
		return rust.GetLanguage(), ext, true
	}
	return nil, "", false
}

// symbolHit is the resolved AST node for a SymbolRef plus the per-kind
// child node references used to compute byte ranges for replace_body
// vs. replace_signature.
type symbolHit struct {
	whole     *sitter.Node // full declaration (used by delete / insert_after)
	body      *sitter.Node // body node (used by replace_body), nil when none
	signature *sitter.Node // signature subrange (used by replace_signature), nil when none
}

func findSymbol(root *sitter.Node, src []byte, ext string, sym SymbolRef) (*symbolHit, error) {
	switch ext {
	case ".go":
		return findSymbolGo(root, src, sym)
	case ".ts", ".tsx":
		return findSymbolTS(root, src, sym)
	case ".py":
		return findSymbolPy(root, src, sym)
	case ".rs":
		return findSymbolRs(root, src, sym)
	}
	return nil, fmt.Errorf("unsupported extension %q", ext)
}

func actionRange(hit *symbolHit, action SymbolAction, _ string) (uint32, uint32, error) {
	switch action {
	case SymbolReplaceBody:
		if hit.body == nil {
			return 0, 0, errors.New("symbol has no body to replace — try replace_signature or delete instead")
		}
		return hit.body.StartByte(), hit.body.EndByte(), nil
	case SymbolReplaceSignature:
		if hit.signature == nil {
			return 0, 0, errors.New("symbol has no signature node to replace")
		}
		return hit.signature.StartByte(), hit.signature.EndByte(), nil
	case SymbolInsertAfter:
		return hit.whole.EndByte(), hit.whole.EndByte(), nil
	case SymbolDelete:
		return hit.whole.StartByte(), hit.whole.EndByte(), nil
	}
	return 0, 0, fmt.Errorf("unknown symbol action %q", action)
}

func nodeText(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}

func symbolLabel(s SymbolRef) string {
	if s.Receiver != "" {
		return s.Kind + " " + s.Receiver + "." + s.Name
	}
	return s.Kind + " " + s.Name
}

// ---------- Go ----------

func findSymbolGo(root *sitter.Node, src []byte, sym SymbolRef) (*symbolHit, error) {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		n := root.NamedChild(i)
		switch n.Type() {
		case "function_declaration":
			if (sym.Kind == "function" || sym.Kind == "") && childFieldText(n, "name", src) == sym.Name && sym.Receiver == "" {
				return &symbolHit{whole: n, body: n.ChildByFieldName("body"), signature: nil}, nil
			}
		case "method_declaration":
			if sym.Kind != "" && sym.Kind != "method" {
				continue
			}
			if childFieldText(n, "name", src) != sym.Name {
				continue
			}
			if sym.Receiver != "" && goReceiverType(n.ChildByFieldName("receiver"), src) != sym.Receiver {
				continue
			}
			return &symbolHit{whole: n, body: n.ChildByFieldName("body"), signature: nil}, nil
		case "type_declaration":
			if sym.Kind != "" && sym.Kind != "type" && sym.Kind != "struct" && sym.Kind != "interface" {
				continue
			}
			for j := 0; j < int(n.NamedChildCount()); j++ {
				spec := n.NamedChild(j)
				if spec.Type() != "type_spec" {
					continue
				}
				if childFieldText(spec, "name", src) == sym.Name {
					return &symbolHit{whole: n, body: spec.ChildByFieldName("type"), signature: nil}, nil
				}
			}
		}
	}
	return nil, nil
}

func goReceiverType(recv *sitter.Node, src []byte) string {
	if recv == nil {
		return ""
	}
	// recv is parameter_list with a single parameter_declaration whose
	// `type` is either an identifier or a pointer_type wrapping one.
	for i := 0; i < int(recv.NamedChildCount()); i++ {
		p := recv.NamedChild(i)
		t := p.ChildByFieldName("type")
		if t == nil {
			continue
		}
		if t.Type() == "pointer_type" && t.NamedChildCount() > 0 {
			t = t.NamedChild(0)
		}
		return nodeText(t, src)
	}
	return ""
}

func childFieldText(n *sitter.Node, field string, src []byte) string {
	c := n.ChildByFieldName(field)
	if c == nil {
		return ""
	}
	return nodeText(c, src)
}

// ---------- TypeScript / TSX ----------

func findSymbolTS(root *sitter.Node, src []byte, sym SymbolRef) (*symbolHit, error) {
	var hit *symbolHit
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "function_declaration":
			if sym.Receiver != "" {
				return true
			}
			if (sym.Kind == "function" || sym.Kind == "") && childFieldText(n, "name", src) == sym.Name {
				hit = &symbolHit{whole: n, body: n.ChildByFieldName("body"), signature: nil}
				return false
			}
		case "class_declaration":
			if (sym.Kind == "class" || sym.Kind == "") && sym.Receiver == "" && childFieldText(n, "name", src) == sym.Name {
				hit = &symbolHit{whole: n, body: n.ChildByFieldName("body"), signature: nil}
				return false
			}
			if sym.Receiver != "" && childFieldText(n, "name", src) == sym.Receiver {
				body := n.ChildByFieldName("body")
				if body != nil {
					for i := 0; i < int(body.NamedChildCount()); i++ {
						m := body.NamedChild(i)
						if m.Type() != "method_definition" {
							continue
						}
						if childFieldText(m, "name", src) == sym.Name {
							hit = &symbolHit{whole: m, body: m.ChildByFieldName("body"), signature: nil}
							return false
						}
					}
				}
			}
		}
		return true
	})
	return hit, nil
}

// ---------- Python ----------

func findSymbolPy(root *sitter.Node, src []byte, sym SymbolRef) (*symbolHit, error) {
	var hit *symbolHit
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "function_definition":
			if sym.Receiver != "" {
				return true
			}
			if (sym.Kind == "function" || sym.Kind == "") && childFieldText(n, "name", src) == sym.Name {
				hit = &symbolHit{whole: n, body: n.ChildByFieldName("body"), signature: nil}
				return false
			}
		case "class_definition":
			if (sym.Kind == "class" || sym.Kind == "") && sym.Receiver == "" && childFieldText(n, "name", src) == sym.Name {
				hit = &symbolHit{whole: n, body: n.ChildByFieldName("body"), signature: nil}
				return false
			}
			if sym.Receiver != "" && childFieldText(n, "name", src) == sym.Receiver {
				body := n.ChildByFieldName("body")
				if body != nil {
					for i := 0; i < int(body.NamedChildCount()); i++ {
						m := body.NamedChild(i)
						if m.Type() == "function_definition" && childFieldText(m, "name", src) == sym.Name {
							hit = &symbolHit{whole: m, body: m.ChildByFieldName("body"), signature: nil}
							return false
						}
					}
				}
			}
		}
		return true
	})
	return hit, nil
}

// ---------- Rust ----------

func findSymbolRs(root *sitter.Node, src []byte, sym SymbolRef) (*symbolHit, error) {
	var hit *symbolHit
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "function_item":
			if sym.Receiver != "" {
				return true
			}
			if (sym.Kind == "function" || sym.Kind == "") && childFieldText(n, "name", src) == sym.Name {
				hit = &symbolHit{whole: n, body: n.ChildByFieldName("body"), signature: nil}
				return false
			}
		case "struct_item":
			if (sym.Kind == "struct" || sym.Kind == "type" || sym.Kind == "") && sym.Receiver == "" && childFieldText(n, "name", src) == sym.Name {
				hit = &symbolHit{whole: n, body: n.ChildByFieldName("body"), signature: nil}
				return false
			}
		case "impl_item":
			if sym.Receiver == "" {
				return true
			}
			// impl <Type> { fn name() {...} } — look up the receiver type
			// via the `type` field, then walk the impl body.
			implType := nodeText(n.ChildByFieldName("type"), src)
			if implType != sym.Receiver {
				return true
			}
			body := n.ChildByFieldName("body")
			if body == nil {
				return true
			}
			for i := 0; i < int(body.NamedChildCount()); i++ {
				m := body.NamedChild(i)
				if m.Type() == "function_item" && childFieldText(m, "name", src) == sym.Name {
					hit = &symbolHit{whole: m, body: m.ChildByFieldName("body"), signature: nil}
					return false
				}
			}
		}
		return true
	})
	return hit, nil
}

// walk does a pre-order traversal of the named-child tree, stopping
// when visit returns false. Used by the language find* helpers to
// reach nested declarations without unrolling per-grammar.
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
