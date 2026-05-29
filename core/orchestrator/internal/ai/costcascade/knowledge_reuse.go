package costcascade

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/providers"
)

// knowledge_reuse.go implements Layer 3 of the cascade — the
// knowledge / code-reuse short-circuit. Its job is to answer a request
// from knowledge that ALREADY exists in the project (the text the caller
// hands us in req.ProjectContext) WITHOUT spending a model call, but ONLY
// when it can do so with high confidence.
//
// The premise: a large fraction of "where is X", "what does Y do",
// "which file holds Z" questions are pure lookups against context the
// orchestrator already assembled and shipped with the request. Routing
// those through a reasoning tier is pure waste — the answer is sitting in
// the prompt. This layer extracts it for free.
//
// THE INVARIANT THAT GOVERNS EVERYTHING HERE: never fabricate. A wrong
// reuse is strictly worse than a missed save — it corrupts the operator's
// mental model of their own codebase and can be silently wrong forever.
// So the gate is deliberately, aggressively biased toward returning
// ("", false). Two independent gates must BOTH pass before we answer:
//
//  1. Intent gate — the prompt must look like a conservative
//     LOOKUP / EXPLAIN question ("where is", "what does", "explain",
//     "find the", "which file", ...). Anything that asks to CREATE,
//     CHANGE, GENERATE, FIX, or REFACTOR is passed straight down; reuse
//     never produces new code.
//  2. Confidence gate — we build a tiny BM25-style index over the
//     project-context text, retrieve the single best-matching chunk, and
//     answer ONLY when that chunk's score clears a high, configurable
//     confidence floor AND its query-term coverage clears a separate
//     overlap floor. Either gate failing yields ("", false).
//
// If the project context is empty or unreachable (it is not wired to the
// live retriever in this front-door package), every request safely
// resolves to ("", false) — i.e. the layer becomes a no-op miss and the
// request flows on to the model tiers. Degrading to "always miss" is an
// explicitly acceptable, correctness-preserving baseline.

// KnowledgeOptions tunes the reuse short-circuit. The zero value is a
// safe, strict configuration: defaults() fills every field with a
// high-confidence, low-false-positive setting, so NewKnowledgeReuse with
// an empty struct already behaves conservatively.
type KnowledgeOptions struct {
	// KnowledgeMinScore is the BM25 confidence floor the single best
	// chunk must clear before its text may be returned as an answer.
	// Higher = stricter = fewer (but safer) reuses. Default 6.0, which on
	// the smoothed BM25 used here corresponds to a strong, multi-term
	// match rather than an incidental keyword overlap.
	KnowledgeMinScore float64

	// KnowledgeMinOverlap is the fraction of distinct query terms (after
	// stopword removal) that must actually appear in the winning chunk.
	// This is a second, score-independent guard against a single rare
	// term dragging an otherwise-irrelevant chunk to the top. In [0,1];
	// default 0.6 (a strong majority of the asked-about terms must be
	// present).
	KnowledgeMinOverlap float64

	// KnowledgeMaxChunkLines caps how large an indexed context chunk may
	// grow before it is split, mirroring the retriever's structure-aware
	// chunking. Default 40 — small enough that a returned answer is a
	// focused snippet, not a wall of unrelated context.
	KnowledgeMaxChunkLines int

	// KnowledgeMaxAnswerBytes hard-bounds the size of any answer this
	// layer will emit. A reuse answer is meant to be a pointed quote of
	// existing knowledge, never a document dump; oversize candidates are
	// rejected (miss) rather than truncated, because a truncated answer
	// can misrepresent the source. Default 4096.
	KnowledgeMaxAnswerBytes int

	// KnowledgeMaxContextBytes bounds how much project-context text we are
	// willing to index for a single request, protecting the hot path from
	// a pathologically large context. Beyond this the layer misses rather
	// than spending unbounded CPU. Default 1 MiB.
	KnowledgeMaxContextBytes int

	// Logger receives debug-level traces of gate decisions. The zero
	// value (a disabled logger) is fine — this layer never logs above
	// debug and never errors.
	Logger zerolog.Logger
}

func (o *KnowledgeOptions) defaults() {
	if o.KnowledgeMinScore <= 0 {
		o.KnowledgeMinScore = 6.0
	}
	if o.KnowledgeMinOverlap <= 0 {
		o.KnowledgeMinOverlap = 0.6
	}
	if o.KnowledgeMaxChunkLines <= 0 {
		o.KnowledgeMaxChunkLines = 40
	}
	if o.KnowledgeMaxAnswerBytes <= 0 {
		o.KnowledgeMaxAnswerBytes = 4096
	}
	if o.KnowledgeMaxContextBytes <= 0 {
		o.KnowledgeMaxContextBytes = 1 << 20
	}
}

// KnowledgeReuse is the Layer-3 short-circuit. It is stateless beyond its
// configuration and safe for concurrent use: Resolve builds a fresh,
// request-scoped index over req.ProjectContext on each call (the context
// is what changes between requests, so a persistent index would be wrong
// anyway). It holds no mutable global state.
type KnowledgeReuse struct {
	opts   KnowledgeOptions
	logger zerolog.Logger
}

// NewKnowledgeReuse constructs the reuse short-circuit with the given
// options (zero value = strict defaults). It never returns an error and
// never panics; a misconfiguration is normalised by defaults().
func NewKnowledgeReuse(opts KnowledgeOptions) *KnowledgeReuse {
	opts.defaults()
	return &KnowledgeReuse{opts: opts, logger: opts.Logger}
}

// AsKnowledgeFunc exposes Resolve as a KnowledgeFunc value suitable for
// cascade.WithKnowledge(...). It is a thin convenience over the method
// value kr.Resolve so callers don't have to spell the signature out.
// A nil receiver yields a func that always misses, so wiring is safe even
// before construction completes.
func (kr *KnowledgeReuse) AsKnowledgeFunc() KnowledgeFunc {
	if kr == nil {
		return func(context.Context, providers.Request) (string, bool) { return "", false }
	}
	return kr.Resolve
}

// Resolve is the KnowledgeFunc implementation. It returns (answer, true)
// only when BOTH the intent gate and the confidence gate pass; otherwise
// ("", false). It never fabricates, never blocks, and never panics: any
// internal anomaly (empty context, oversize context, tokenisation edge
// case) degrades to a clean miss so the request flows on to the model
// tiers untouched.
func (kr *KnowledgeReuse) Resolve(ctx context.Context, req providers.Request) (string, bool) {
	if kr == nil {
		return "", false
	}
	// Respect caller cancellation — a cancelled request should not pay for
	// indexing; just miss and let the upper layer surface the ctx error.
	if ctx != nil && ctx.Err() != nil {
		return "", false
	}

	// Gate 1 — intent. Cheapest check first; reject anything that is not a
	// conservative lookup/explain question before touching the context.
	if !knowledgeLooksLikeLookup(req.Prompt) {
		return "", false
	}

	projectCtx := strings.TrimSpace(req.ProjectContext)
	if projectCtx == "" || len(projectCtx) > kr.opts.KnowledgeMaxContextBytes {
		// No grounding material (or too much to safely index) → miss.
		return "", false
	}

	terms := knowledgeQueryTerms(req.Prompt)
	if len(terms) == 0 {
		return "", false
	}

	// Build a request-scoped index over the project context and retrieve
	// the single strongest chunk.
	idx := knowledgeBuildIndex(projectCtx, kr.opts.KnowledgeMaxChunkLines)
	best, score, ok := idx.top(terms)
	if !ok {
		return "", false
	}

	// Gate 2a — absolute confidence floor.
	if score < kr.opts.KnowledgeMinScore {
		kr.debug(score, len(terms), "below score floor")
		return "", false
	}

	// Gate 2b — term coverage. Guard against one rare term hauling an
	// otherwise-irrelevant chunk to the top: a strong majority of the
	// distinct asked-about terms must literally appear in the winner.
	overlap := knowledgeOverlap(best.terms, terms)
	if overlap < kr.opts.KnowledgeMinOverlap {
		kr.debug(score, len(terms), "below overlap floor")
		return "", false
	}

	// Final safety: never emit an oversize answer. A reuse answer is a
	// pointed quote of existing knowledge; if the winning chunk is larger
	// than the answer bound we miss rather than truncate (truncation can
	// misrepresent the source, violating "never fabricate").
	answer := strings.TrimSpace(best.text)
	if answer == "" || len(answer) > kr.opts.KnowledgeMaxAnswerBytes {
		return "", false
	}

	kr.logger.Debug().
		Float64("score", score).
		Float64("overlap", overlap).
		Int("answer_bytes", len(answer)).
		Msg("costcascade: knowledge reuse resolved request from project context")
	return answer, true
}

func (kr *KnowledgeReuse) debug(score float64, nTerms int, reason string) {
	kr.logger.Debug().
		Float64("score", score).
		Int("query_terms", nTerms).
		Str("reason", reason).
		Msg("costcascade: knowledge reuse declined (gate not met)")
}

// ---- intent gate ----------------------------------------------------

// knowledgeLookupCues are the conservative lead-ins that mark a prompt as
// a pure LOOKUP / EXPLAIN question — one whose answer, if it exists, is
// already present in the project context. The list is intentionally
// short and specific; a borderline phrasing should MISS, not match.
var knowledgeLookupCues = []string{
	"where is",
	"where are",
	"where does",
	"where can i find",
	"which file",
	"which files",
	"what file",
	"what does",
	"what is the",
	"what are the",
	"how does",
	"find the",
	"locate the",
	"show me where",
	"explain the",
	"explain what",
	"describe the",
	"summarize the",
	"summarise the",
}

// knowledgeMutationCues mark a prompt as a CREATE / CHANGE request. If any
// of these appears the prompt is disqualified outright: reuse must never
// answer a request to produce or modify code. This veto wins even when a
// lookup cue is also present ("explain and then refactor X" → miss).
var knowledgeMutationCues = []string{
	"create", "add", "implement", "build", "generate", "write",
	"refactor", "rewrite", "modify", "change", "update", "edit",
	"fix", "patch", "remove", "delete", "rename", "migrate",
	"optimize", "optimise", "improve", "convert", "replace",
	"install", "deploy", "configure", "scaffold", "make a", "make the",
}

// knowledgeLooksLikeLookup decides whether prompt is a conservative
// lookup/explain question. It requires (a) at least one lookup cue and
// (b) zero mutation cues. Both checks are case-insensitive substring
// matches against the normalised prompt — heuristic, not a parser, and
// tuned to err toward MISS.
func knowledgeLooksLikeLookup(prompt string) bool {
	p := strings.ToLower(strings.TrimSpace(prompt))
	if p == "" {
		return false
	}
	// Mutation veto first — a single create/change verb disqualifies.
	for _, c := range knowledgeMutationCues {
		if knowledgeContainsWord(p, c) {
			return false
		}
	}
	for _, c := range knowledgeLookupCues {
		if strings.Contains(p, c) {
			return true
		}
	}
	return false
}

// knowledgeContainsWord reports whether `word` appears in `s` on word
// boundaries (so "create" matches "create a hook" but not "creative" or
// "createdAt"). Multi-word cues fall back to a plain substring test since
// their boundaries are already implied by the surrounding spaces.
func knowledgeContainsWord(s, word string) bool {
	if strings.ContainsRune(word, ' ') {
		return strings.Contains(s, word)
	}
	idx := 0
	for {
		i := strings.Index(s[idx:], word)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(word)
		beforeOK := start == 0 || !knowledgeIsWordRune(rune(s[start-1]))
		afterOK := end == len(s) || !knowledgeIsWordRune(rune(s[end]))
		if beforeOK && afterOK {
			return true
		}
		idx = start + 1
		if idx >= len(s) {
			return false
		}
	}
}

func knowledgeIsWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// ---- request-scoped BM25 index over project context ----------------

// knowledgeChunk is one indexed slice of the project context plus its
// pre-tokenised term-frequency bag.
type knowledgeChunk struct {
	text  string
	terms map[string]int
	docLn float64
}

// knowledgeIndex is a tiny, request-scoped BM25 index. It is built fresh
// per Resolve call from the project-context text and discarded after; it
// never persists or escapes the call. This mirrors the retriever's BM25
// design (smoothed IDF, length normalisation) but operates on free-form
// context text rather than a domain.Project, so the front-door package
// stays free of domain coupling.
type knowledgeIndex struct {
	chunks    []knowledgeChunk
	idf       map[string]float64
	avgDocLen float64
}

const (
	knowledgeBM25K1 = 1.5
	knowledgeBM25B  = 0.75
)

// knowledgeBuildIndex chunks the project-context text and computes the IDF table + average
// document length. Chunking is blank-line / size-bounded: a blank line
// starts a new chunk (a natural paragraph / block boundary in assembled
// context), and any chunk reaching maxLines is flushed.
func knowledgeBuildIndex(projectCtx string, maxLines int) *knowledgeIndex {
	idx := &knowledgeIndex{idf: map[string]float64{}}
	if maxLines <= 0 {
		maxLines = 40
	}
	lines := strings.Split(projectCtx, "\n")
	var cur []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(cur, "\n"))
		cur = cur[:0]
		if text == "" {
			return
		}
		bag := map[string]int{}
		dl := 0
		for _, t := range knowledgeTokenize(text) {
			bag[t]++
			dl++
		}
		if dl == 0 {
			return
		}
		idx.chunks = append(idx.chunks, knowledgeChunk{
			text:  text,
			terms: bag,
			docLn: float64(dl),
		})
	}
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			flush()
			continue
		}
		cur = append(cur, ln)
		if len(cur) >= maxLines {
			flush()
		}
	}
	flush()
	idx.finalize()
	return idx
}

func (i *knowledgeIndex) finalize() {
	n := len(i.chunks)
	if n == 0 {
		return
	}
	df := map[string]int{}
	for _, c := range i.chunks {
		for term := range c.terms {
			df[term]++
		}
	}
	for term, freq := range df {
		i.idf[term] = math.Log(1 + (float64(n)-float64(freq)+0.5)/(float64(freq)+0.5))
	}
	var total float64
	for _, c := range i.chunks {
		total += c.docLn
	}
	i.avgDocLen = total / float64(n)
}

// top returns the single best-scoring chunk against the query terms, its
// BM25 score, and ok=false when the index is empty or nothing scored
// above zero.
func (i *knowledgeIndex) top(terms []string) (knowledgeChunk, float64, bool) {
	if len(i.chunks) == 0 || i.avgDocLen == 0 || len(terms) == 0 {
		return knowledgeChunk{}, 0, false
	}
	type scored struct {
		idx   int
		score float64
	}
	all := make([]scored, len(i.chunks))
	for j := range i.chunks {
		all[j] = scored{idx: j, score: i.bm25(j, terms)}
	}
	sort.SliceStable(all, func(a, b int) bool { return all[a].score > all[b].score })
	winner := all[0]
	if winner.score <= 0 {
		return knowledgeChunk{}, 0, false
	}
	return i.chunks[winner.idx], winner.score, true
}

func (i *knowledgeIndex) bm25(doc int, terms []string) float64 {
	c := i.chunks[doc]
	var s float64
	for _, term := range terms {
		idf, ok := i.idf[term]
		if !ok {
			continue
		}
		f := float64(c.terms[term])
		if f == 0 {
			continue
		}
		norm := knowledgeBM25K1 * (1 - knowledgeBM25B + knowledgeBM25B*c.docLn/i.avgDocLen)
		s += idf * (f * (knowledgeBM25K1 + 1)) / (f + norm)
	}
	return s
}

// knowledgeOverlap returns the fraction of distinct query terms that
// appear at least once in the chunk's term bag, in [0,1].
func knowledgeOverlap(bag map[string]int, terms []string) float64 {
	if len(terms) == 0 {
		return 0
	}
	seen := map[string]struct{}{}
	var present int
	for _, t := range terms {
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		if bag[t] > 0 {
			present++
		}
	}
	if len(seen) == 0 {
		return 0
	}
	return float64(present) / float64(len(seen))
}

// ---- tokenisation ---------------------------------------------------

// knowledgeQueryTerms tokenises a prompt into distinct, deduplicated
// search terms, after stripping the lookup-cue scaffolding so the BM25
// match keys off the SUBJECT of the question ("auth handler") rather than
// the interrogative frame ("where is the ..."). Dedup keeps the overlap
// fraction meaningful.
func knowledgeQueryTerms(prompt string) []string {
	toks := knowledgeTokenize(prompt)
	seen := make(map[string]struct{}, len(toks))
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// knowledgeTokenize lowercases and splits text on non-identifier runes,
// dropping very short tokens and a tight stopword list (generic English
// plus the interrogative frame words and a few code stop terms). It is
// deliberately simple and dependency-free; it mirrors the retriever's
// tokeniser closely enough that scores are comparable in spirit.
func knowledgeTokenize(s string) []string {
	if s == "" {
		return nil
	}
	out := make([]string, 0, len(s)/4+1)
	var cur []rune
	emit := func() {
		if len(cur) == 0 {
			return
		}
		w := string(cur)
		cur = cur[:0]
		if len(w) < 2 || knowledgeIsStopword(w) {
			return
		}
		out = append(out, w)
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			cur = append(cur, unicode.ToLower(r))
			continue
		}
		emit()
	}
	emit()
	return out
}

// knowledgeStopwords removes both generic filler and — crucially — the
// interrogative scaffolding of lookup questions, so a query keys off its
// subject. Without this, "where is the user token cache" would match any
// chunk dense in "the"/"is"/"where".
var knowledgeStopwords = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "are": {}, "but": {}, "not": {},
	"you": {}, "all": {}, "can": {}, "use": {}, "any": {}, "with": {},
	"this": {}, "that": {}, "from": {}, "into": {}, "your": {}, "have": {},
	"does": {}, "did": {}, "was": {}, "were": {}, "been": {}, "being": {},
	// interrogative / lookup-frame words — strip so the subject dominates.
	"where": {}, "what": {}, "which": {}, "who": {}, "whom": {}, "whose": {},
	"how": {}, "why": {}, "when": {}, "find": {}, "locate": {}, "show": {},
	"explain": {}, "describe": {}, "summarize": {}, "summarise": {},
	"file": {}, "files": {}, "code": {}, "function": {}, "method": {},
	// code stop terms.
	"func": {}, "def": {}, "let": {}, "var": {}, "const": {}, "return": {},
	"nil": {}, "null": {}, "true": {}, "false": {}, "none": {},
}

func knowledgeIsStopword(t string) bool { _, ok := knowledgeStopwords[t]; return ok }
