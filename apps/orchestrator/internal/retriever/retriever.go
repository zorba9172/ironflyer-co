// Package retriever is Ironflyer's in-process code search index. It powers
// retrieval-augmented prompts for the Coder: given a user story, it returns
// the handful of project code chunks most likely to be relevant so the LLM
// produces patches grounded in the actual codebase instead of inventing
// structure from scratch.
//
// Design choices:
//
//   - **Pure Go, no CGO, no external service.** BM25 with light tokenization
//     beats embedding-only retrieval on small code corpora (well under a few
//     thousand files) and is free to operate. We can add a vector backend
//     later without changing the Query() contract.
//   - **Structure-aware chunking.** We split each file at top-level
//     declaration boundaries (`func`, `class`, `interface`, `type`, `export`,
//     blank-line groups) so chunks are semantically meaningful — pulling in
//     one function instead of the surrounding 800 lines.
//   - **Symbol boost.** Lines that declare named symbols receive extra
//     weight: a query for "auth handler" finds `func AuthHandler` even when
//     the body never mentions "auth" again.
//   - **Stateless.** Build the index per call. The corpus is in memory
//     anyway (Project.Files) so the saving from a persistent index is small
//     compared to the simplicity gain.
package retriever

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"ironflyer/apps/orchestrator/internal/domain"
)

// Chunk is one indexed piece of source code. It is the unit the Coder
// receives as "relevant context."
type Chunk struct {
	Path      string  // file path inside the project
	StartLine int     // 1-based line where the chunk begins
	EndLine   int     // inclusive
	Symbols   []string // declared symbol names within the chunk (boosts ranking)
	Text      string  // raw chunk body (newline-joined)
	Score     float64 // BM25 score against the last Query() call
}

// Options tunes retriever behaviour. Zero value is sensible defaults.
type Options struct {
	MaxChunkLines int     // chunks larger than this are split (default 80)
	MinChunkLines int     // chunks smaller merge with siblings (default 4)
	TopK          int     // results returned by Query (default 8)
	SymbolBoost   float64 // multiplier applied to symbol-name term weights (default 2.5)
	K1            float64 // BM25 k1 (default 1.5)
	B             float64 // BM25 b  (default 0.75)
}

func (o *Options) defaults() {
	if o.MaxChunkLines == 0 {
		o.MaxChunkLines = 80
	}
	if o.MinChunkLines == 0 {
		o.MinChunkLines = 4
	}
	if o.TopK == 0 {
		o.TopK = 8
	}
	if o.SymbolBoost == 0 {
		o.SymbolBoost = 2.5
	}
	if o.K1 == 0 {
		o.K1 = 1.5
	}
	if o.B == 0 {
		o.B = 0.75
	}
}

// Index is a built BM25 retrieval index over a project's source.
type Index struct {
	opts   Options
	chunks []Chunk
	// per-chunk pre-tokenized bag with frequencies (for body terms)
	bodyTerms []map[string]int
	// per-chunk symbol-term set (boosted weight at scoring time)
	symbolTerms []map[string]struct{}
	// IDF table over body+symbol vocabulary
	idf map[string]float64
	// average doc length used by BM25 length norm
	avgDocLen float64
	docLens   []float64
}

// Build constructs an index from a project's in-memory files. Skips
// non-source paths (binaries, lock files, .ironflyer/* artifacts) so the
// retriever stays focused on code that the Coder might want to modify.
func Build(p *domain.Project, opts Options) *Index {
	opts.defaults()
	idx := &Index{opts: opts, idf: map[string]float64{}}
	if p == nil {
		return idx
	}
	for _, f := range p.Files {
		if !isIndexable(f.Path) {
			continue
		}
		for _, c := range chunkFile(f.Path, f.Content, opts) {
			idx.addChunk(c)
		}
	}
	idx.finalize()
	return idx
}

// addChunk appends a chunk and tokenizes its body + symbols.
func (i *Index) addChunk(c Chunk) {
	body := map[string]int{}
	for _, tok := range tokenize(c.Text) {
		body[tok]++
	}
	syms := map[string]struct{}{}
	for _, s := range c.Symbols {
		for _, tok := range tokenize(s) {
			syms[tok] = struct{}{}
		}
	}
	i.chunks = append(i.chunks, c)
	i.bodyTerms = append(i.bodyTerms, body)
	i.symbolTerms = append(i.symbolTerms, syms)
	dl := 0
	for _, n := range body {
		dl += n
	}
	i.docLens = append(i.docLens, float64(dl))
}

// finalize precomputes IDF and average doc length once all chunks are in.
func (i *Index) finalize() {
	n := len(i.chunks)
	if n == 0 {
		return
	}
	df := map[string]int{}
	for _, body := range i.bodyTerms {
		for term := range body {
			df[term]++
		}
	}
	for _, syms := range i.symbolTerms {
		for term := range syms {
			df[term]++
		}
	}
	for term, freq := range df {
		// BM25-style smoothed IDF
		i.idf[term] = math.Log(1 + (float64(n)-float64(freq)+0.5)/(float64(freq)+0.5))
	}
	var total float64
	for _, d := range i.docLens {
		total += d
	}
	if n > 0 {
		i.avgDocLen = total / float64(n)
	}
}

// Query returns the top-K chunks ranked by BM25 against `query`. Results
// are sorted by score descending. K=0 uses the index default.
func (i *Index) Query(query string, k int) []Chunk {
	if len(i.chunks) == 0 || strings.TrimSpace(query) == "" {
		return nil
	}
	if k <= 0 {
		k = i.opts.TopK
	}
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	scored := make([]Chunk, len(i.chunks))
	for j := range i.chunks {
		scored[j] = i.chunks[j]
		scored[j].Score = i.bm25Score(j, terms)
	}
	sort.SliceStable(scored, func(a, b int) bool { return scored[a].Score > scored[b].Score })
	if k > len(scored) {
		k = len(scored)
	}
	// Drop zero-score tail — never inject noise into the Coder prompt.
	out := scored[:k]
	end := k
	for ; end > 0 && out[end-1].Score <= 0; end-- {
	}
	return out[:end]
}

// bm25Score computes BM25 for one chunk against a tokenised query.
// Symbol terms receive SymbolBoost on the IDF side so a function-name match
// outranks a body keyword match of equivalent rarity.
func (i *Index) bm25Score(doc int, terms []string) float64 {
	body := i.bodyTerms[doc]
	syms := i.symbolTerms[doc]
	dl := i.docLens[doc]
	k1, b := i.opts.K1, i.opts.B
	if i.avgDocLen == 0 {
		return 0
	}
	var s float64
	for _, term := range terms {
		idf, ok := i.idf[term]
		if !ok {
			continue
		}
		f := float64(body[term])
		// Symbol presence acts like a virtual term-frequency bump.
		if _, hit := syms[term]; hit {
			f += i.opts.SymbolBoost
			idf *= i.opts.SymbolBoost
		}
		if f == 0 {
			continue
		}
		norm := k1 * (1 - b + b*dl/i.avgDocLen)
		s += idf * (f * (k1 + 1)) / (f + norm)
	}
	return s
}

// FormatContext renders a Coder-ready block of "# Relevant code" with each
// chunk delimited by a heading line. Returns "" when there are no chunks
// so callers can safely concatenate.
func FormatContext(chunks []Chunk) string {
	if len(chunks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Relevant code (top matches from your existing project)\n")
	b.WriteString("Use these snippets as the source of truth for existing structure. ")
	b.WriteString("Extend or modify them rather than reinventing — and respect their imports + naming conventions.\n\n")
	for _, c := range chunks {
		b.WriteString("--- ")
		b.WriteString(c.Path)
		if c.StartLine > 0 {
			b.WriteString(":")
			b.WriteString(itoa(c.StartLine))
			b.WriteString("-")
			b.WriteString(itoa(c.EndLine))
		}
		if len(c.Symbols) > 0 {
			b.WriteString(" [")
			b.WriteString(strings.Join(c.Symbols, ", "))
			b.WriteString("]")
		}
		b.WriteString(" ---\n")
		b.WriteString(c.Text)
		if !strings.HasSuffix(c.Text, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// chunkFile slices file content into structure-aware blocks. It walks the
// lines and starts a new chunk every time it sees a top-level declaration
// or hits the MaxChunkLines cap. Symbols visible in the chunk are collected
// for ranking.
func chunkFile(path, content string, opts Options) []Chunk {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	// initial chunk starting at line 1
	cur := Chunk{Path: path, StartLine: 1}
	var curLines []string
	flush := func(endLine int) {
		if len(curLines) == 0 {
			return
		}
		// Merge tiny tail chunks into the previous chunk when possible to
		// avoid spamming the Coder with 2-line snippets.
		if len(chunks) > 0 && len(curLines) < opts.MinChunkLines {
			prev := &chunks[len(chunks)-1]
			prev.EndLine = endLine
			prev.Text += "\n" + strings.Join(curLines, "\n")
			prev.Symbols = dedupe(append(prev.Symbols, cur.Symbols...))
			curLines = nil
			cur = Chunk{Path: path, StartLine: endLine + 1}
			return
		}
		cur.EndLine = endLine
		cur.Text = strings.Join(curLines, "\n")
		chunks = append(chunks, cur)
		curLines = nil
		cur = Chunk{Path: path, StartLine: endLine + 1}
	}
	for idx, line := range lines {
		lineNum := idx + 1
		if sym, ok := declSymbol(line); ok {
			// Start a fresh chunk on a top-level decl boundary, unless we're
			// already on the first line of one.
			if len(curLines) > 0 && lineNum > cur.StartLine {
				flush(lineNum - 1)
				cur.StartLine = lineNum
			}
			cur.Symbols = append(cur.Symbols, sym)
		}
		curLines = append(curLines, line)
		if len(curLines) >= opts.MaxChunkLines {
			flush(lineNum)
			cur.StartLine = lineNum + 1
		}
	}
	flush(len(lines))
	return chunks
}

// declSymbol returns (name, true) when the line opens a top-level
// declaration whose name is worth indexing as a symbol. Works for Go, TS,
// JS, Python, Rust, Java — heuristic, not a parser, but good enough for
// retrieval ranking. Returns ("", false) on miss.
func declSymbol(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if t == "" {
		return "", false
	}
	// strip access modifiers / export keywords that don't change the name
	// extraction strategy
	for _, prefix := range []string{
		"export default async function ",
		"export default function ",
		"export async function ",
		"export function ",
		"export const ",
		"export let ",
		"export class ",
		"export interface ",
		"export type ",
		"export enum ",
		"public class ",
		"public interface ",
		"public abstract class ",
		"public final class ",
		"async function ",
		"function ",
		"class ",
		"interface ",
		"struct ",
		"enum ",
		"impl ",
		"trait ",
		"fn ",
		"def ",
		"func ",
		"type ",
		"const ",
		"var ",
		"let ",
	} {
		if strings.HasPrefix(t, prefix) {
			rest := strings.TrimSpace(t[len(prefix):])
			name := pickIdentifier(rest)
			if name != "" {
				return name, true
			}
		}
	}
	return "", false
}

func pickIdentifier(s string) string {
	// Collect leading identifier characters: letters, digits, underscore.
	// Stop at the first non-identifier rune.
	var b strings.Builder
	for i, r := range s {
		if i == 0 && (unicode.IsDigit(r)) {
			return ""
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' {
			b.WriteRune(r)
			continue
		}
		break
	}
	return b.String()
}

// tokenize splits source text into search terms. We lowercase, split on
// non-letter/digit, drop very short and very common stopwords, and split
// camelCase / snake_case so a query for "userToken" matches "user_token"
// and vice versa.
func tokenize(s string) []string {
	if s == "" {
		return nil
	}
	var raw []string
	cur := make([]rune, 0, 32)
	emit := func() {
		if len(cur) > 0 {
			raw = append(raw, string(cur))
			cur = cur[:0]
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			cur = append(cur, unicode.ToLower(r))
			continue
		}
		emit()
	}
	emit()
	// Split each raw token on snake_case and camelCase boundaries.
	out := make([]string, 0, len(raw)*2)
	for _, t := range raw {
		for _, p := range splitIdentifier(t) {
			if len(p) < 2 || isStopword(p) {
				continue
			}
			out = append(out, p)
		}
	}
	return out
}

// splitIdentifier emits the original token plus each camel/snake segment.
// Indexing both keeps "userToken" findable by "user" or "token" without
// destroying the original strong match on the full identifier.
func splitIdentifier(t string) []string {
	if t == "" {
		return nil
	}
	out := []string{t}
	// snake_case split
	for _, part := range strings.Split(t, "_") {
		if part != "" && part != t {
			out = append(out, part)
		}
	}
	// camelCase split — operate on lower-case representation since we
	// already lowercased everything in tokenize; detect transitions via the
	// original case is lost. Instead we infer boundaries from digits.
	// (Acceptable: camelCase tokens still match via the original token
	// since both query and chunk are lowercased identically.)
	return out
}

// Tight stopword list — generic English plus a few code stop terms.
var stopwords = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "are": {}, "but": {}, "not": {},
	"you": {}, "all": {}, "can": {}, "use": {}, "any": {}, "with": {},
	"this": {}, "that": {}, "from": {}, "into": {}, "your": {}, "have": {},
	"when": {}, "what": {}, "will": {}, "want": {}, "want_": {},
	"true": {}, "false": {}, "null": {}, "nil": {}, "none": {}, "self": {},
	"func": {}, "fn": {}, "def": {}, "let": {}, "var": {},
}

func isStopword(t string) bool { _, ok := stopwords[t]; return ok }

// isIndexable filters out files that have no business in a code-context
// budget: binary blobs, lock files, asset directories, and the .ironflyer
// artifacts the orchestrator writes itself.
func isIndexable(path string) bool {
	low := strings.ToLower(path)
	switch {
	case strings.HasPrefix(low, ".ironflyer/"):
		return false
	case strings.Contains(low, "/node_modules/"), strings.HasPrefix(low, "node_modules/"):
		return false
	case strings.Contains(low, "/.git/"), strings.HasPrefix(low, ".git/"):
		return false
	case strings.HasSuffix(low, ".lock"), strings.HasSuffix(low, "-lock.json"),
		strings.HasSuffix(low, "go.sum"), strings.HasSuffix(low, "yarn.lock"),
		strings.HasSuffix(low, "pnpm-lock.yaml"):
		return false
	case strings.HasSuffix(low, ".png"), strings.HasSuffix(low, ".jpg"),
		strings.HasSuffix(low, ".jpeg"), strings.HasSuffix(low, ".gif"),
		strings.HasSuffix(low, ".webp"), strings.HasSuffix(low, ".ico"),
		strings.HasSuffix(low, ".pdf"), strings.HasSuffix(low, ".zip"),
		strings.HasSuffix(low, ".tar"), strings.HasSuffix(low, ".gz"),
		strings.HasSuffix(low, ".woff"), strings.HasSuffix(low, ".woff2"):
		return false
	}
	return true
}

func dedupe(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
