package costcascade

import (
	"context"
	"strings"
	"unicode"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// =============================================================================
// PromptCompressor — pure-Go, LLMLingua-class prompt/context reduction.
// =============================================================================
//
// PromptCompressor is the package's Compressor implementation: a single-pass,
// O(n), deterministic heuristic shrinker that runs on EVERY model-bound call
// after the cache/knowledge layers miss and before the model tier is chosen
// (see Cascade.CompleteStreamWithFailover). Because a smaller prompt both
// costs fewer input tokens AND can drop the request into a cheaper tier, this
// is one of the highest-leverage, lowest-risk savings in the cascade — but
// ONLY if it is provably meaning-preserving. The whole design below is biased
// toward correctness: it removes redundancy and boilerplate, never substance.
//
// SCOPE OF WHAT IT TOUCHES
//
//   - req.ProjectContext — the large, repeated context (codebase summaries,
//     specs, file dumps). This is the primary target: it is informational, it
//     is the biggest token sink, and it is the safest to dedupe/strip because
//     it is reference material rather than the imperative ask.
//   - req.Prompt — only OPTIONALLY, and only its low-salience lines, and NEVER
//     its final/imperative instruction. The last non-blank line of Prompt is
//     treated as the load-bearing instruction and is always preserved verbatim.
//   - req.System — NEVER touched. The system prompt is the contract that
//     governs the model's behaviour; mutating it is a correctness hazard, so
//     it is out of scope by construction.
//
// FLOOR (no-op on small inputs)
//
// Compression has fixed risk and only pays off on large, repetitive context.
// Below compressFloorChars (default 8000 chars ≈ 2000 tokens of context) the
// request is returned untouched with 0 saved. This keeps short, hand-crafted
// prompts — where every line may matter and the savings are negligible —
// completely unmodified.
//
// CORRECTNESS ARGUMENT (per transform)
//
//	(a) Exact-duplicate line/paragraph removal. Removing a line that is byte-
//	    for-byte identical to a line already emitted cannot remove information:
//	    the model has already seen that exact content earlier in the same
//	    context window. We dedupe non-trivial lines (len ≥ compressDedupMinLen)
//	    only, so we never collapse meaningful repeated short tokens like "}",
//	    "---" table cells, or list bullets whose repetition is structural.
//	(b) Whitespace collapse. Runs of ≥2 blank lines become one blank line, and
//	    trailing horizontal whitespace is trimmed. Whitespace carries no
//	    semantic payload to an LLM tokenizer beyond paragraph separation, which
//	    a single blank line fully preserves.
//	(c) Boilerplate stripping. We drop lines that are conservatively recognised
//	    as decorative/structural noise: ASCII-art separators (a line that is
//	    nothing but a run of one punctuation rune, e.g. "======", "------",
//	    "******"), and long comment banners that are pure separator runs. These
//	    lines convey no instruction or fact; they exist for human visual
//	    grouping, which the model does not need. We deliberately do NOT strip
//	    license/comment TEXT (only pure-symbol rules), because comment text can
//	    carry intent.
//	(d) Salience pruning (opt-in, Aggressive). When enabled, after (a)-(c) we
//	    score each remaining ProjectContext line by information density and drop
//	    the lowest-salience filler until we reach the keep-ratio. A line is
//	    HIGH salience (always kept) if it bears code-like identifiers, symbols,
//	    digits, or imperative/instruction markers; a line is LOW salience (a
//	    drop candidate) only if it is prose-like filler. The keep-ratio floors
//	    the fraction retained so we never gut the context. This stage is off by
//	    default because it is the only lossy-by-design transform.
//
// All transforms are order-stable and deterministic: identical input yields
// identical output, which keeps the exact-hash response cache effective
// (a compressed prompt hashes consistently).
//
// SAFETY / DEGRADATION
//
// The compressor never panics, never blocks, and never errors. Any unexpected
// condition (empty fields, pathological input) falls through to "return the
// request untouched, 0 saved". A no-op is always a correct compressor.

// compressDefaultKeepRatio is the fraction of ProjectContext lines the
// salience stage aims to retain. 0.55 keeps the clear majority — enough that
// even an imperfect salience heuristic cannot strip the substance — while
// still trimming roughly the lowest-information 45% of prose filler.
const compressDefaultKeepRatio = 0.55

// compressDefaultFloorChars is the minimum combined compressible size below
// which the compressor is a strict no-op. 8000 chars ≈ 2000 tokens.
const compressDefaultFloorChars = 8000

// compressDedupMinLen is the minimum trimmed line length eligible for
// exact-duplicate removal. Short repeated lines ("}", "---", "- item") are
// frequently structural and are left in place.
const compressDedupMinLen = 24

// compressMinKeepRatio / compressMaxKeepRatio clamp a caller-supplied keep
// ratio into a sane band: never gut below 0.20, never claim to "keep" above
// 1.0 (which would disable the stage anyway).
const (
	compressMinKeepRatio = 0.20
	compressMaxKeepRatio = 1.0
)

// CompressOptions configures a PromptCompressor. The zero value is valid and
// is normalised to safe defaults by NewPromptCompressor, so callers can pass
// CompressOptions{} for the recommended behaviour.
type CompressOptions struct {
	// KeepRatio is the target fraction of ProjectContext lines retained by the
	// salience stage (only consulted when Aggressive is true). Clamped to
	// [compressMinKeepRatio, compressMaxKeepRatio]; 0 → compressDefaultKeepRatio.
	KeepRatio float64

	// FloorChars is the minimum combined compressible size (ProjectContext +
	// optionally Prompt) below which Compress is a strict no-op. 0 →
	// compressDefaultFloorChars.
	FloorChars int

	// Aggressive enables stage (d), salience pruning. Off by default because
	// it is the only lossy-by-design transform. Stages (a)-(c) are lossless
	// and always run.
	Aggressive bool

	// IncludePrompt extends the lossless stages (a)-(c) to req.Prompt as well
	// as req.ProjectContext. The final/imperative instruction line of Prompt
	// is ALWAYS preserved regardless. Off by default — most savings live in
	// ProjectContext, and leaving Prompt alone is the most conservative choice.
	IncludePrompt bool
}

// PromptCompressor is a deterministic, single-pass context reducer satisfying
// the Compressor interface. It holds only its normalised configuration and no
// mutable state, so a single instance is safe for concurrent use across calls.
type PromptCompressor struct {
	keepRatio     float64
	floorChars    int
	aggressive    bool
	includePrompt bool
}

// NewPromptCompressor builds a PromptCompressor from opts, normalising the
// zero value to the documented defaults (keep ~0.55, floor 8000 chars,
// lossless-only, ProjectContext-only). It never returns nil.
func NewPromptCompressor(opts CompressOptions) *PromptCompressor {
	keep := opts.KeepRatio
	if keep == 0 {
		keep = compressDefaultKeepRatio
	}
	if keep < compressMinKeepRatio {
		keep = compressMinKeepRatio
	}
	if keep > compressMaxKeepRatio {
		keep = compressMaxKeepRatio
	}
	floor := opts.FloorChars
	if floor <= 0 {
		floor = compressDefaultFloorChars
	}
	return &PromptCompressor{
		keepRatio:     keep,
		floorChars:    floor,
		aggressive:    opts.Aggressive,
		includePrompt: opts.IncludePrompt,
	}
}

// Compress reduces req's compressible fields and returns the reduced request
// plus an estimate of the input tokens saved (chars removed / 4). It is a
// strict no-op — returning (req, 0) — when the compressor is nil, when the
// compressible payload is below the floor, or when no transform removed
// anything. It never mutates the caller's request in place beyond replacing
// the String fields on the returned copy, and never panics.
func (p *PromptCompressor) Compress(_ context.Context, req providers.Request) (providers.Request, int) {
	if p == nil {
		return req, 0
	}

	// FLOOR: measure the compressible payload (the fields we are willing to
	// touch) and bail untouched when it is too small to be worth the risk.
	compressibleLen := len(req.ProjectContext)
	if p.includePrompt {
		compressibleLen += len(req.Prompt)
	}
	if compressibleLen < p.floorChars {
		return req, 0
	}

	var removed int

	// ProjectContext: the primary, safest target. Lossless stages always; the
	// salience stage only when Aggressive.
	if req.ProjectContext != "" {
		out, cut := p.compressBlock(req.ProjectContext, p.aggressive, "")
		if cut > 0 {
			req.ProjectContext = out
			removed += cut
		}
	}

	// Prompt: only when explicitly opted in, and only the lossless stages —
	// salience pruning is never applied to the ask. The final imperative line
	// is pinned so it can never be deduped or dropped.
	if p.includePrompt && req.Prompt != "" {
		pin := compressLastNonBlankLine(req.Prompt)
		out, cut := p.compressBlock(req.Prompt, false, pin)
		if cut > 0 {
			req.Prompt = out
			removed += cut
		}
	}

	if removed <= 0 {
		return req, 0
	}
	// Token estimate mirrors the package's ~4-chars-per-token convention.
	return req, removed / 4
}

// compressBlock applies the transforms to a single text block and returns the
// rewritten block plus the number of characters removed (input len − output
// len, never negative). salience selects whether stage (d) runs. pin, when
// non-empty, is a line that must never be deduped or pruned (the imperative
// instruction); it is matched on its trimmed form.
//
// The function is a single linear pass over the lines for the lossless stages;
// the optional salience stage is one more linear pass plus a partial selection,
// keeping the whole thing O(n) in practice for the line counts we see.
func (p *PromptCompressor) compressBlock(block string, salience bool, pin string) (string, int) {
	inLen := len(block)
	lines := strings.Split(block, "\n")

	// Stages (a)-(c): dedupe + whitespace-collapse + boilerplate strip, in a
	// single pass that preserves order.
	seen := make(map[string]struct{}, len(lines))
	kept := make([]string, 0, len(lines))
	blankRun := false
	// Normalise the pin through the SAME whitespace pipeline the per-line
	// comparison uses, so a pin with interior multi-space/tabs still matches
	// its normalised line — otherwise the final imperative instruction would
	// silently lose its pin protection.
	pinTrim := strings.TrimSpace(compressNormalizeWhitespace(pin))

	for _, raw := range lines {
		// (b) trim trailing horizontal whitespace; collapse interior runs of
		// spaces/tabs to a single space without disturbing leading indentation
		// (indentation can be structurally meaningful, e.g. in code/YAML).
		line := compressNormalizeWhitespace(raw)
		trimmed := strings.TrimSpace(line)
		isPinned := pinTrim != "" && trimmed == pinTrim

		// (b) collapse runs of blank lines to a single blank line.
		if trimmed == "" {
			if blankRun {
				continue
			}
			blankRun = true
			kept = append(kept, "")
			continue
		}
		blankRun = false

		if !isPinned {
			// (c) drop pure decorative separators / ASCII-art banners.
			if compressIsSeparator(trimmed) {
				continue
			}
			// (a) drop exact-duplicate non-trivial lines.
			if len(trimmed) >= compressDedupMinLen {
				if _, dup := seen[trimmed]; dup {
					continue
				}
				seen[trimmed] = struct{}{}
			}
		}
		kept = append(kept, line)
	}

	// (d) salience pruning over the surviving non-blank lines. Off unless the
	// caller enabled Aggressive. We only ever DROP low-salience filler lines,
	// and only down to the keep-ratio floor, so the bulk of the context — and
	// every code-/instruction-bearing line — survives.
	if salience {
		kept = p.prune(kept, pinTrim)
	}

	out := compressJoinTrim(kept)
	outLen := len(out)
	if outLen >= inLen {
		// Defensive: never grow. If normalisation somehow netted zero or
		// negative savings, treat as a no-op for this block.
		return block, 0
	}
	return out, inLen - outLen
}

// prune is salience stage (d). It scores each non-blank line and removes the
// lowest-scoring LOW-salience lines until the retained non-blank fraction
// reaches keepRatio. HIGH-salience lines (code/identifiers/instructions) and
// the pinned line are never dropped. Order is preserved.
func (p *PromptCompressor) prune(lines []string, pinTrim string) []string {
	// Count non-blank lines and identify drop candidates (low salience only).
	type cand struct {
		idx   int
		score float64
	}
	var nonBlank int
	var candidates []cand
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		nonBlank++
		if pinTrim != "" && t == pinTrim {
			continue
		}
		// Hard never-drop guard. A line carrying ANY code / identifier /
		// instruction signal is protected regardless of its blended salience
		// score — short substantive lines (e.g. `userId`, `return nil`,
		// `import os`, `const MAX = 3`) score below the soft cut yet must
		// never be silently deleted from the model's context. Only genuine
		// prose filler (no code symbol, digit, identifier-shaped capital,
		// leading code keyword, or instruction marker) remains a candidate.
		if compressIsCodeLike(t) {
			continue
		}
		s := compressSalience(t)
		if s < compressLowSalienceCut {
			candidates = append(candidates, cand{idx: i, score: s})
		}
	}
	if nonBlank == 0 || len(candidates) == 0 {
		return lines
	}

	// How many non-blank lines we are allowed to drop without breaching the
	// keep-ratio floor.
	target := int(float64(nonBlank) * p.keepRatio)
	if target < 1 {
		target = 1
	}
	maxDrop := nonBlank - target
	if maxDrop <= 0 {
		return lines
	}
	if maxDrop > len(candidates) {
		maxDrop = len(candidates)
	}

	// Drop the lowest-scoring candidates first. Deterministic: stable by
	// (score, index). We mark indices to delete, then rebuild order-stably.
	// Selection over a typically small candidate set; a simple partial
	// insertion of the maxDrop lowest is sufficient and avoids a full sort
	// dependency for determinism.
	drop := make(map[int]struct{}, maxDrop)
	for n := 0; n < maxDrop; n++ {
		best := -1
		for ci := range candidates {
			if _, used := drop[candidates[ci].idx]; used {
				continue
			}
			if best == -1 {
				best = ci
				continue
			}
			// Lower score wins; ties broken by earlier index for determinism.
			if candidates[ci].score < candidates[best].score ||
				(candidates[ci].score == candidates[best].score && candidates[ci].idx < candidates[best].idx) {
				best = ci
			}
		}
		if best == -1 {
			break
		}
		drop[candidates[best].idx] = struct{}{}
	}

	out := make([]string, 0, len(lines))
	for i, l := range lines {
		if _, gone := drop[i]; gone {
			continue
		}
		out = append(out, l)
	}
	return out
}

// compressLowSalienceCut is the salience threshold below which a line becomes a
// drop candidate. Tuned so that prose filler ("This is a paragraph that
// explains in words...") scores below it while any line bearing code symbols,
// identifiers, digits, or instruction markers scores above it.
const compressLowSalienceCut = 0.45

// compressSalience scores a single trimmed line's information density in
// [0,1]. Higher = more load-bearing. The heuristic rewards signals that
// correlate with substance an LLM needs:
//
//   - code/structure symbols ({}()[]<>=:;/\|`._-) and digits,
//   - mixed-case or ALLCAPS identifier-shaped tokens,
//   - imperative / instruction markers ("must", "return", "do not", etc.),
//   - short, dense lines (filler tends to be long flowing prose).
//
// and penalises long runs of lowercase prose words. It is intentionally simple
// and monotonic so its output is stable and explainable.
func compressSalience(line string) float64 {
	if line == "" {
		return 0
	}
	var symbols, digits, upper, letters int
	for _, r := range line {
		switch {
		case unicode.IsDigit(r):
			digits++
		case unicode.IsLetter(r):
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		case isCompressStructuralSymbol(r):
			symbols++
		}
	}
	n := float64(len([]rune(line)))
	if n == 0 {
		return 0
	}

	score := 0.0
	// Structural symbols are the strongest signal of code/structure.
	score += clamp01(float64(symbols)/n*3.0) * 0.45
	// Digits (versions, sizes, IDs, line numbers) carry specific facts.
	score += clamp01(float64(digits)/n*4.0) * 0.15
	// Identifier-shaped capitalisation density.
	if letters > 0 {
		score += clamp01(float64(upper)/float64(letters)*2.0) * 0.15
	}
	// Imperative / instruction markers — these lines often ARE the ask.
	if compressHasInstructionMarker(line) {
		score += 0.30
	}
	// Density bonus: short dense lines are usually substantive; long flowing
	// prose lines are the classic filler the salience stage exists to trim.
	if n <= 80 {
		score += 0.10
	}
	return clamp01(score)
}

// compressInstructionMarkers are lowercase tokens that frequently mark an
// imperative or a hard requirement. A line containing one is treated as
// instruction-bearing and is shielded from salience pruning.
var compressInstructionMarkers = []string{
	"must", "must not", "do not", "don't", "never", "always", "ensure",
	"require", "required", "should", "shall", "return", "implement",
	"add ", "remove ", "fix ", "use ", "avoid", "important", "note:",
	"warning", "constraint", "rule:", "todo", "task:", "goal:",
}

// compressHasInstructionMarker reports whether a line contains an imperative /
// requirement marker (case-insensitive).
func compressHasInstructionMarker(line string) bool {
	low := strings.ToLower(line)
	for _, m := range compressInstructionMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// compressCodeKeywords are tokens that, when they LEAD a line, unambiguously
// mark structured code/config — and are rare as a leading word in prose
// filler, so they are safe to treat as a never-drop signal.
var compressCodeKeywords = map[string]bool{
	"import": true, "package": true, "func": true, "fn": true, "def": true,
	"class": true, "struct": true, "interface": true, "enum": true, "type": true,
	"const": true, "var": true, "let": true, "namespace": true, "module": true,
	"public": true, "private": true, "protected": true, "export": true,
	"include": true, "using": true, "async": true,
}

// compressIsCodeLike reports whether a trimmed line carries a HARD code /
// identifier / instruction signal and therefore must NEVER be dropped by the
// aggressive salience stage — independent of its (soft, density-based)
// salience score, which short identifiers legitimately fall below. A single
// structural code symbol, a digit, an interior-capital identifier shape
// (camelCase / PascalCase / ALLCAPS), a leading code keyword, or an
// instruction marker shields the line. A mere leading capital or trailing
// period (ordinary prose) does NOT — that filler stays prunable.
func compressIsCodeLike(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	if compressHasInstructionMarker(trimmed) {
		return true
	}
	prevLetter := false
	for _, r := range trimmed {
		switch {
		case unicode.IsDigit(r):
			return true
		case isCompressCodeSymbol(r):
			return true
		case unicode.IsLetter(r):
			if prevLetter && unicode.IsUpper(r) {
				return true // interior capital → identifier shape (camelCase/ALLCAPS)
			}
			prevLetter = true
			continue
		}
		prevLetter = false
	}
	// Leading code keyword (the line's first letter-run).
	lead := trimmed
	if i := strings.IndexFunc(lead, func(r rune) bool { return !unicode.IsLetter(r) }); i >= 0 {
		lead = lead[:i]
	}
	return compressCodeKeywords[strings.ToLower(lead)]
}

// isCompressCodeSymbol is the STRICT code-symbol set used by the never-drop
// guard. It deliberately excludes prose punctuation (. , : ; ! ? ' " -) so an
// ordinary sentence is not mistaken for code; it keeps only runes that signal
// code/structure (brackets, operators, sigils, snake_case underscore).
func isCompressCodeSymbol(r rune) bool {
	switch r {
	case '{', '}', '(', ')', '[', ']', '<', '>', '=', '/', '\\', '|',
		'`', '_', '#', '@', '$', '*', '&', '%', '^', '~', '+':
		return true
	}
	return false
}

// isCompressStructuralSymbol reports whether r is a code/structure punctuation
// rune whose presence signals substance rather than prose.
func isCompressStructuralSymbol(r rune) bool {
	switch r {
	case '{', '}', '(', ')', '[', ']', '<', '>', '=', ':', ';',
		'/', '\\', '|', '`', '.', '_', '-', '+', '*', '&', '%',
		'$', '#', '@', '!', '?', '~', '^':
		return true
	}
	return false
}

// compressIsSeparator reports whether a trimmed line is a pure decorative
// separator / ASCII-art rule — a run of a single non-alphanumeric rune,
// length ≥ 3 (e.g. "======", "------", "******", "######", "~~~~~~"). Such
// lines carry no instruction or fact, only visual grouping the model ignores.
// Mixed-content lines and short ones (e.g. "---" YAML front-matter fences are
// length 3 and would be caught — but they convey no payload either) are
// handled conservatively: we require at least 4 identical symbol runes to
// avoid eating meaningful short markers.
func compressIsSeparator(trimmed string) bool {
	if len(trimmed) < 4 {
		return false
	}
	first := rune(trimmed[0])
	if unicode.IsLetter(first) || unicode.IsDigit(first) || unicode.IsSpace(first) {
		return false
	}
	for _, r := range trimmed {
		if r != first {
			return false
		}
	}
	return true
}

// compressNormalizeWhitespace trims trailing horizontal whitespace and
// collapses interior runs of spaces/tabs to a single space, while preserving
// leading indentation (which can be structurally meaningful). It does not
// touch the line's content otherwise.
func compressNormalizeWhitespace(line string) string {
	// Preserve leading indentation verbatim.
	lead := 0
	for lead < len(line) && (line[lead] == ' ' || line[lead] == '\t') {
		lead++
	}
	indent := line[:lead]
	body := strings.TrimRight(line[lead:], " \t")
	if body == "" {
		return ""
	}
	// Collapse interior whitespace runs in the body.
	var b strings.Builder
	b.Grow(len(body))
	prevSpace := false
	for _, r := range body {
		if r == ' ' || r == '\t' {
			if prevSpace {
				continue
			}
			prevSpace = true
			b.WriteByte(' ')
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return indent + b.String()
}

// compressJoinTrim joins kept lines with newlines and trims a single leading /
// trailing blank line that the collapse pass can leave behind, so the block
// has no surrounding empty padding.
func compressJoinTrim(lines []string) string {
	// Drop leading/trailing blank entries.
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

// compressLastNonBlankLine returns the trimmed text of the final non-blank
// line of s — treated as the imperative instruction that must be preserved.
// Empty when s has no non-blank line.
func compressLastNonBlankLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}

// clamp01 clamps v to [0,1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
