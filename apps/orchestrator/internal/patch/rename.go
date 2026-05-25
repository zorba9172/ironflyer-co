// Package patch — workspace-wide symbol rename. Anchor patches and
// in-file SymbolPatch handle the one-file case. A rename across many
// files (e.g. "rename HTTPClient → HttpClient everywhere") needs a
// special path: walk every project file whose extension has an AST
// adapter, parse, rewrite references, emit a SymbolRenameResult.
//
// The result is a slice of FileChange entries the caller can wrap into
// a regular Patch + ApplyPatch — so the multi-file rename rides the
// same lifecycle (validate, snapshot, apply, gates, rollback) as any
// other patch.
package patch

import (
	"errors"
	"path"
	"strings"

	"ironflyer/apps/orchestrator/internal/patch/ast"
)

// ProposeRename returns a list of FileChange entries that, taken
// together, rename every reference to oldName as newName across the
// workspace. Files whose extension lacks an AST adapter are skipped
// (with a Diff note for the review UI). Files whose body doesn't
// reference oldName are also skipped.
//
// The caller wraps the result in a regular Patch — that way the
// usual validation, anchor checks, snapshot, gates, and rollback path
// all apply unchanged. We do NOT mutate any file directly; the engine
// remains the single writer.
func (e *Engine) ProposeRename(projectID string, kind ast.SymbolKind, oldName, newName string) (Patch, error) {
	if oldName == "" || newName == "" {
		return Patch{}, errors.New("rename requires both oldName and newName")
	}
	if oldName == newName {
		return Patch{}, errors.New("oldName and newName are identical — nothing to do")
	}
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return Patch{}, err
	}
	var changes []FileChange
	for _, f := range proj.Files {
		// Quick reject: no substring match → no possible reference.
		if !strings.Contains(f.Content, oldName) {
			continue
		}
		adapter := ast.AdapterFor(f.Path)
		if !adapter.Supported() {
			// Fallback: when no AST adapter is available, do a
			// word-boundary string rewrite. Conservative — won't
			// touch substring matches inside other identifiers but
			// also won't catch refs through reflection / strings.
			rewritten, changed := wordBoundaryRename(f.Content, oldName, newName)
			if !changed {
				continue
			}
			changes = append(changes, FileChange{
				Op:      OpUpdate,
				Path:    f.Path,
				Content: rewritten,
				Diff:    "fallback rename in " + path.Base(f.Path) + " (no AST adapter for " + strings.TrimPrefix(path.Ext(f.Path), ".") + ")",
			})
			continue
		}
		rewritten, err := adapter.Rename(f.Content, kind, oldName, newName)
		if err != nil {
			return Patch{}, err
		}
		if rewritten == f.Content {
			continue
		}
		changes = append(changes, FileChange{
			Op:      OpUpdate,
			Path:    f.Path,
			Content: rewritten,
			Diff:    "ast rename " + oldName + " → " + newName + " in " + path.Base(f.Path),
		})
	}
	if len(changes) == 0 {
		return Patch{}, errors.New("no references to rename — symbol not found in any project file")
	}
	return Patch{
		ProjectID: projectID,
		Title:     "rename " + oldName + " → " + newName,
		Summary:   "workspace-wide symbol rename across " + itoa(len(changes)) + " files",
		Changes:   changes,
	}, nil
}

// wordBoundaryRename rewrites whole-identifier matches of oldName as
// newName. A "word boundary" is the absence of an ASCII letter, digit,
// or underscore on either side — close enough for most languages we
// care about. Used as a fallback when no AST adapter is wired (the
// default build, or an unsupported language).
func wordBoundaryRename(source, oldName, newName string) (string, bool) {
	if oldName == "" {
		return source, false
	}
	var b strings.Builder
	b.Grow(len(source))
	changed := false
	i := 0
	for i < len(source) {
		if !strings.HasPrefix(source[i:], oldName) {
			b.WriteByte(source[i])
			i++
			continue
		}
		// Check boundaries.
		left := byte(0)
		if i > 0 {
			left = source[i-1]
		}
		right := byte(0)
		if i+len(oldName) < len(source) {
			right = source[i+len(oldName)]
		}
		if isWordByte(left) || isWordByte(right) {
			b.WriteByte(source[i])
			i++
			continue
		}
		b.WriteString(newName)
		i += len(oldName)
		changed = true
	}
	return b.String(), changed
}

func isWordByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_':
		return true
	}
	return false
}
