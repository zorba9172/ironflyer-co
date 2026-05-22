// Package patcher applies a unified diff to a workspace via the File API.
// It supports the subset of `git diff`/`diff -u` output that our coder
// agent actually produces:
//
//   - File headers `--- a/path` / `+++ b/path` (or `--- path`/`+++ path`).
//   - `new file mode` / `deleted file mode` markers for create/delete.
//   - One or more `@@ -lstart,lcount +rstart,rcount @@` hunks.
//   - Context (` `), addition (`+`), removal (`-`) lines.
//
// Binary patches and rename-only diffs are not supported; the patch will
// fail fast with a clear error. That matches the orchestrator's existing
// patch.Engine contract.
package patcher

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Filesystem abstracts the per-driver file API the patcher needs. The
// runtime's sandbox.Driver satisfies this directly through the sandbox
// adapter in httpapi.
type Filesystem interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, data []byte) error
	DeleteFile(ctx context.Context, path string) error
}

// ChangeKind classifies the effect of applying one file's hunks.
type ChangeKind string

const (
	ChangeCreated  ChangeKind = "created"
	ChangeModified ChangeKind = "modified"
	ChangeDeleted  ChangeKind = "deleted"
)

// FileChange records the post-apply outcome for one file in the diff.
type FileChange struct {
	Path string     `json:"path"`
	Kind ChangeKind `json:"kind"`
	// Bytes is the size of the file after the change (0 for deletes).
	Bytes int `json:"bytes"`
}

// Apply parses `diff` and applies every file section through `fs`. It
// is atomic at the per-file level: a hunk mismatch aborts that file but
// does not roll back files that have already been written. Callers that
// need transactionality should snapshot/restore around Apply.
func Apply(ctx context.Context, fs Filesystem, diff string) ([]FileChange, error) {
	files, err := parseUnifiedDiff(diff)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("patch contains no file sections")
	}
	out := make([]FileChange, 0, len(files))
	for _, f := range files {
		change, err := applyFile(ctx, fs, f)
		if err != nil {
			return out, fmt.Errorf("patch %s: %w", f.path, err)
		}
		out = append(out, change)
	}
	return out, nil
}

// --------------------------------------------------------------------------

type filePatch struct {
	path       string
	newFile    bool
	deleteFile bool
	hunks      []hunk
}

type hunk struct {
	oldStart, oldCount int
	newStart, newCount int
	lines              []string // each prefixed with ' ', '+' or '-'
}

func parseUnifiedDiff(diff string) ([]filePatch, error) {
	// Pre-split into lines so we can do single-line lookahead trivially
	// without bufio.Scanner pushback gymnastics.
	all := splitLinesNoTrailingEmpty(diff)
	var out []filePatch
	var cur *filePatch
	finish := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}

	i := 0
	for i < len(all) {
		line := all[i]
		switch {
		case strings.HasPrefix(line, "diff --git "):
			finish()
			cur = &filePatch{}
			i++
		case strings.HasPrefix(line, "new file mode"):
			if cur == nil {
				cur = &filePatch{}
			}
			cur.newFile = true
			i++
		case strings.HasPrefix(line, "deleted file mode"):
			if cur == nil {
				cur = &filePatch{}
			}
			cur.deleteFile = true
			i++
		case strings.HasPrefix(line, "--- "):
			if cur == nil {
				cur = &filePatch{}
			}
			rhs := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			if rhs == "/dev/null" {
				cur.newFile = true
			}
			i++
		case strings.HasPrefix(line, "+++ "):
			if cur == nil {
				cur = &filePatch{}
			}
			rhs := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			if rhs == "/dev/null" {
				cur.deleteFile = true
			} else {
				cur.path = stripABPrefix(rhs)
			}
			i++
		case strings.HasPrefix(line, "@@"):
			if cur == nil {
				return nil, errors.New("hunk header before file header")
			}
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			i++
			h.lines = make([]string, 0, h.oldCount+h.newCount)
			for i < len(all) {
				bl := all[i]
				if strings.HasPrefix(bl, "@@") ||
					strings.HasPrefix(bl, "diff --git ") ||
					strings.HasPrefix(bl, "--- ") {
					break
				}
				h.lines = append(h.lines, bl)
				i++
			}
			cur.hunks = append(cur.hunks, h)
		default:
			// Ignore index, mode-bits, similarity, binary-stub, etc.
			i++
		}
	}
	finish()
	for i := range out {
		if out[i].path == "" {
			return nil, fmt.Errorf("file section %d missing path", i)
		}
	}
	return out, nil
}

// splitLinesNoTrailingEmpty splits on '\n' without producing the spurious
// empty element a normal `strings.Split` does when the input ends in '\n'.
func splitLinesNoTrailingEmpty(s string) []string {
	if s == "" {
		return nil
	}
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "\n")
}

func stripABPrefix(p string) string {
	if strings.HasPrefix(p, "a/") {
		return strings.TrimPrefix(p, "a/")
	}
	if strings.HasPrefix(p, "b/") {
		return strings.TrimPrefix(p, "b/")
	}
	return p
}

func parseHunkHeader(line string) (hunk, error) {
	// `@@ -1,4 +1,5 @@ optional trailing context`
	if !strings.HasPrefix(line, "@@") {
		return hunk{}, errors.New("not a hunk header")
	}
	rest := strings.TrimPrefix(line, "@@")
	end := strings.Index(rest, "@@")
	if end < 0 {
		return hunk{}, errors.New("malformed hunk header")
	}
	spec := strings.TrimSpace(rest[:end])
	// spec is "-A,B +C,D" or "-A +C" (counts default to 1).
	parts := strings.Fields(spec)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "-") || !strings.HasPrefix(parts[1], "+") {
		return hunk{}, fmt.Errorf("malformed hunk spec: %q", spec)
	}
	os, oc, err := parseRange(strings.TrimPrefix(parts[0], "-"))
	if err != nil {
		return hunk{}, fmt.Errorf("old range: %w", err)
	}
	ns, nc, err := parseRange(strings.TrimPrefix(parts[1], "+"))
	if err != nil {
		return hunk{}, fmt.Errorf("new range: %w", err)
	}
	return hunk{oldStart: os, oldCount: oc, newStart: ns, newCount: nc}, nil
}

func parseRange(s string) (start, count int, err error) {
	if idx := strings.Index(s, ","); idx >= 0 {
		if _, err := fmt.Sscanf(s, "%d,%d", &start, &count); err != nil {
			return 0, 0, err
		}
		return start, count, nil
	}
	if _, err := fmt.Sscanf(s, "%d", &start); err != nil {
		return 0, 0, err
	}
	return start, 1, nil
}

// applyFile takes one parsed file section and writes the result.
func applyFile(ctx context.Context, fs Filesystem, f filePatch) (FileChange, error) {
	if f.deleteFile {
		if err := fs.DeleteFile(ctx, f.path); err != nil {
			return FileChange{}, err
		}
		return FileChange{Path: f.path, Kind: ChangeDeleted}, nil
	}

	var original []string
	if !f.newFile {
		data, err := fs.ReadFile(ctx, f.path)
		if err != nil {
			// Treat missing files as "new" so an over-eager `---` header
			// without `new file mode` still works.
			f.newFile = true
		} else {
			original = splitKeepEOL(string(data))
		}
	}

	updated, err := applyHunks(original, f.hunks)
	if err != nil {
		return FileChange{}, err
	}
	body := strings.Join(updated, "")
	if err := fs.WriteFile(ctx, f.path, []byte(body)); err != nil {
		return FileChange{}, err
	}
	kind := ChangeModified
	if f.newFile {
		kind = ChangeCreated
	}
	return FileChange{Path: f.path, Kind: kind, Bytes: len(body)}, nil
}

// applyHunks plays the hunks against the original line list and returns
// the new file content. Each hunk's oldStart is 1-indexed.
func applyHunks(original []string, hunks []hunk) ([]string, error) {
	if len(hunks) == 0 {
		return original, nil
	}
	out := make([]string, 0, len(original)+16)
	cursor := 0 // index into original
	for hi, h := range hunks {
		target := h.oldStart - 1
		if target < 0 {
			target = 0
		}
		if target > len(original) {
			return nil, fmt.Errorf("hunk %d: starts past EOF (line %d, file has %d)", hi+1, h.oldStart, len(original))
		}
		if target < cursor {
			return nil, fmt.Errorf("hunk %d: out-of-order (start %d < cursor %d)", hi+1, h.oldStart, cursor+1)
		}
		// Copy unchanged lines up to the hunk start.
		out = append(out, original[cursor:target]...)
		cursor = target

		for _, line := range h.lines {
			if len(line) == 0 {
				// Empty line in a hunk = empty context line.
				if cursor >= len(original) {
					return nil, fmt.Errorf("hunk %d: context past EOF", hi+1)
				}
				out = append(out, original[cursor])
				cursor++
				continue
			}
			tag, payload := line[0], line[1:]
			switch tag {
			case ' ':
				if cursor >= len(original) {
					return nil, fmt.Errorf("hunk %d: context past EOF (expected %q)", hi+1, payload)
				}
				if stripEOL(original[cursor]) != payload {
					return nil, fmt.Errorf("hunk %d: context mismatch at line %d (have %q, want %q)",
						hi+1, cursor+1, stripEOL(original[cursor]), payload)
				}
				out = append(out, original[cursor])
				cursor++
			case '-':
				if cursor >= len(original) {
					return nil, fmt.Errorf("hunk %d: deletion past EOF", hi+1)
				}
				if stripEOL(original[cursor]) != payload {
					return nil, fmt.Errorf("hunk %d: deletion mismatch at line %d (have %q, want %q)",
						hi+1, cursor+1, stripEOL(original[cursor]), payload)
				}
				cursor++ // skip in original; don't emit
			case '+':
				out = append(out, payload+"\n")
			case '\\':
				// `\ No newline at end of file` — strip trailing newline
				// from the previous emitted line, if any.
				if len(out) > 0 {
					last := out[len(out)-1]
					out[len(out)-1] = strings.TrimRight(last, "\n")
				}
			default:
				// Some producers emit blank lines as truly empty (handled
				// above) or as " " (single space + EOL). Anything else is
				// unexpected.
				return nil, fmt.Errorf("hunk %d: unrecognised line prefix %q", hi+1, string(tag))
			}
		}
	}
	// Tail.
	out = append(out, original[cursor:]...)
	return out, nil
}

// splitKeepEOL splits a string into lines, keeping the trailing newline
// on every line except (optionally) the last one when the file doesn't
// end with `\n`.
func splitKeepEOL(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			out = append(out, s)
			return out
		}
		out = append(out, s[:idx+1])
		s = s[idx+1:]
		if s == "" {
			return out
		}
	}
}

func stripEOL(s string) string {
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")
	return s
}
