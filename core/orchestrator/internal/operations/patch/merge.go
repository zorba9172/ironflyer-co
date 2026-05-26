// Package patch — 3-way merge for concurrent file edits.
//
// When Engine.Apply discovers the target file body has changed since
// Propose (compare BaseHash vs current hash), it can't just stomp the
// user's manual edits with the AI's projected output. Instead we run a
// small diff3-style merger:
//
//	base   = file body at Propose-time (captured into FileChange.BaseBody)
//	ours   = current file body on disk (the user's manual edits)
//	theirs = body the AI would have produced (projectedBody)
//
// The merger walks both diff sides line-by-line. Hunks that touch
// disjoint regions are accepted on both sides. Hunks that overlap with
// equal content are accepted once. Hunks that overlap with different
// content emit conflict markers in the standard `<<<<<<<` /
// `=======` / `>>>>>>>` shape so the UI can render a 3-way diff and
// the user can pick.
//
// This is intentionally a small, dependency-free implementation rather
// than a full RCS-grade diff3 — production code-edit conflicts in
// Ironflyer are typically tens of lines, not thousands, and we want a
// merger that's easy to reason about.
package patch

import (
	"strings"
)

// threeWayMerge merges `ours` and `theirs` over a common `base`. The
// returned string is the merged file body; conflicted is true when at
// least one conflict marker block was emitted, in which case the
// "merged" body is the markup-decorated text the UI shows.
//
// Algorithm:
//  1. Diff base→ours and base→theirs into line-level hunks (LCS-based).
//  2. Walk base lines in order. At each position emit one of:
//     - the common base line, when neither side changed it,
//     - the modified line from the side that changed it (when only one
//     did),
//     - both sides' changes equal → emit once,
//     - both sides changed differently → conflict markers.
//  3. Append any pure-tail insertions from each side after the base ends.
func threeWayMerge(base, ours, theirs string) (string, bool) {
	if ours == theirs {
		return ours, false
	}
	if base == ours {
		return theirs, false
	}
	if base == theirs {
		return ours, false
	}

	bLines := splitLines(base)
	oLines := splitLines(ours)
	tLines := splitLines(theirs)

	oOps := diffOps(bLines, oLines)
	tOps := diffOps(bLines, tLines)

	var (
		out      []string
		i        int // base index
		conf     bool
		oIdx, tI int
	)
	for i < len(bLines) {
		oHunk := nextHunkAt(oOps, &oIdx, i)
		tHunk := nextHunkAt(tOps, &tI, i)

		switch {
		case oHunk == nil && tHunk == nil:
			out = append(out, bLines[i])
			i++
		case oHunk != nil && tHunk == nil:
			out = append(out, oHunk.replacement...)
			i = oHunk.baseEnd
		case oHunk == nil && tHunk != nil:
			out = append(out, tHunk.replacement...)
			i = tHunk.baseEnd
		default:
			// Both sides changed the same region.
			if sliceEqual(oHunk.replacement, tHunk.replacement) && oHunk.baseEnd == tHunk.baseEnd {
				out = append(out, oHunk.replacement...)
				i = oHunk.baseEnd
				continue
			}
			out = append(out, "<<<<<<< ours")
			out = append(out, oHunk.replacement...)
			out = append(out, "=======")
			out = append(out, tHunk.replacement...)
			out = append(out, ">>>>>>> theirs")
			conf = true
			// Advance past the farther of the two base ranges so we
			// don't double-emit either side.
			if oHunk.baseEnd > tHunk.baseEnd {
				i = oHunk.baseEnd
			} else {
				i = tHunk.baseEnd
			}
		}
	}
	// Pure-tail insertions (changes beyond the base length).
	if oTail := tailFrom(oOps, oIdx); len(oTail) > 0 && !sliceEqual(oTail, tailFrom(tOps, tI)) {
		out = append(out, oTail...)
	} else {
		out = append(out, oTail...)
	}
	if tTail := tailFrom(tOps, tI); len(tTail) > 0 && !sliceEqual(tTail, tailFrom(oOps, oIdx)) {
		// Avoid double-appending when both tails matched above.
		// We already appended ours; if theirs differs, emit a conflict.
		ourTail := tailFrom(oOps, oIdx)
		if !sliceEqual(ourTail, tTail) {
			out = append(out, "<<<<<<< ours-tail")
			out = append(out, ourTail...)
			out = append(out, "=======")
			out = append(out, tTail...)
			out = append(out, ">>>>>>> theirs-tail")
			conf = true
		}
	}
	merged := strings.Join(out, "\n")
	// Preserve a trailing newline when both inputs had one — keeps file
	// bodies "POSIX-clean" so a downstream Go file doesn't trip the
	// syntax pre-check.
	if strings.HasSuffix(base, "\n") || strings.HasSuffix(ours, "\n") || strings.HasSuffix(theirs, "\n") {
		merged += "\n"
	}
	return merged, conf
}

// hunk describes one contiguous edit relative to the base sequence.
// baseStart..baseEnd is the half-open range of base lines the hunk
// replaces; replacement is what the side wrote in their place.
type hunk struct {
	baseStart   int
	baseEnd     int
	replacement []string
}

// diffOps computes a minimal list of hunks that transform `base` into
// `target` via Myers-style LCS line diff. Equal runs become no-ops and
// are not represented; what's left is the change set.
func diffOps(base, target []string) []hunk {
	lcs := buildLCS(base, target)
	var hunks []hunk
	i, j := 0, 0
	for i < len(base) || j < len(target) {
		// Equal stretch — advance both indices.
		for i < len(base) && j < len(target) && base[i] == target[j] {
			i++
			j++
		}
		if i >= len(base) && j >= len(target) {
			break
		}
		bs := i
		// Greedy walk: find the next match (using the LCS table) and
		// emit the bracketed change as a single hunk.
		// Determine the next sync point — the next base index that
		// matches the next target index in LCS order.
		ni, nj := nextSync(lcs, i, j, len(base), len(target))
		h := hunk{
			baseStart:   bs,
			baseEnd:     ni,
			replacement: append([]string{}, target[j:nj]...),
		}
		hunks = append(hunks, h)
		i, j = ni, nj
	}
	return hunks
}

// buildLCS computes the classic LCS length table for line slices. Used
// by nextSync to walk forward to the next aligned match.
func buildLCS(a, b []string) [][]int {
	n, m := len(a), len(b)
	t := make([][]int, n+1)
	for i := range t {
		t[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				t[i][j] = t[i-1][j-1] + 1
			} else if t[i-1][j] >= t[i][j-1] {
				t[i][j] = t[i-1][j]
			} else {
				t[i][j] = t[i][j-1]
			}
		}
	}
	return t
}

// nextSync walks forward from (i, j) until it finds an aligned match
// in the LCS table. The returned (i', j') is the start of the next
// equal run; everything between (i, j) and (i', j') is the change.
func nextSync(t [][]int, i, j, n, m int) (int, int) {
	for i < n && j < m {
		if t[i+1][j+1] == t[i][j]+1 {
			// (i, j) is a match — but we only got here because base[i]
			// != target[j] (caller already skipped equals). Fall
			// through to advance.
		}
		// Walk along the side that doesn't decrease LCS length.
		if t[i+1][j] >= t[i][j+1] {
			i++
		} else {
			j++
		}
		if i < n && j < m && t[i+1][j+1] == t[i][j]+1 {
			// Found a match at (i, j). Return it as the sync point.
			return i, j
		}
	}
	return n, m
}

// nextHunkAt returns the hunk whose baseStart == baseIdx if one
// exists, advancing `cursor` past it. Returns nil when no hunk is
// pending at this base index.
func nextHunkAt(hunks []hunk, cursor *int, baseIdx int) *hunk {
	if *cursor >= len(hunks) {
		return nil
	}
	h := &hunks[*cursor]
	if h.baseStart != baseIdx {
		return nil
	}
	*cursor++
	return h
}

// tailFrom returns the replacement bodies of every remaining hunk past
// `cursor`. Used to flush pure-append changes (the side added lines
// beyond the end of base).
func tailFrom(hunks []hunk, cursor int) []string {
	var out []string
	for k := cursor; k < len(hunks); k++ {
		out = append(out, hunks[k].replacement...)
	}
	return out
}

// splitLines slices on \n and drops a trailing empty produced by a
// final newline. The merger re-attaches the trailing newline when
// joining so we don't lose POSIX-final-newline semantics.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
