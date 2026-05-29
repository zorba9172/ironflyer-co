package costcascade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/embeddings"
	"ironflyer/core/orchestrator/internal/ai/providers"
)

// SemanticCache is the GPTCache-class upgrade of the Layer-2 response store:
// a fuzzy, embedding-similarity cache that can serve a prior completion when a
// NEW request is semantically close enough to one already answered — even when
// the prompts are not byte-identical, which the exact-hash ResponseCache always
// misses.
//
// The win is real cost: many paid executions issue prompts that differ only in
// whitespace, phrasing, or ordering ("refactor this fn" vs "please refactor
// this function") yet expect an equivalent answer. The exact-hash store falls
// through to a billed model call for every one of those; the semantic store
// replays the cached answer at zero cost.
//
// The danger is equally real, so this type is conservative by construction.
// Three independent safety gates must ALL hold before a stored answer is
// served to a query:
//
//  1. cosine(query, candidate) >= Threshold (default 0.95) — only a very
//     close neighbour, never a loose topical match.
//  2. tierOf(candidate caps) == tierOf(query caps) — a reasoning-grade answer
//     is never served to a reflex-tier request or vice versa; the structural
//     capability profile must match.
//  3. JSONSchema(query) == JSONSchema(candidate) — a free-form answer is
//     never served to a structured-output request, and two different schemas
//     never share a cached body.
//
// Anything short of all three is a miss and falls through to the real model.
// A single near-match never serves a structurally different answer.
//
// SemanticCache satisfies the package ResponseStore interface, so the cascade
// can be pointed at it via Cascade.WithResponseStore without touching the
// resolution flow.
//
// Degradation is total: a nil embedder, an embedder error, a dimension
// mismatch, or any internal anomaly is treated as a miss (Get) or a no-op
// (Put). The cache NEVER panics, NEVER blocks the provider stream, and NEVER
// serves a wrong answer in preference to falling through — correctness wins
// over the cache hit every time.
type SemanticCache struct {
	embedder embeddings.Embedder
	opts     SemanticCacheOptions
	logger   zerolog.Logger

	mu      sync.Mutex
	ring    []semanticEntry // bounded FIFO ring of stored completions
	next    int             // write cursor into ring (wraps at cap)
	filled  bool            // true once the ring has wrapped at least once
	indexOf map[string]int  // dedupe key → ring slot, so re-Put overwrites in place
}

// semanticEntry is one stored, replayable completion plus the metadata the
// safety gates compare against. vec is the L2-normalized embedding of the key
// text, so a cosine similarity collapses to a plain dot product.
type semanticEntry struct {
	key        string // dedupe key (sha256 of the normalized key text + tier + schema)
	vec        []float32
	resp       CachedResponse
	tier       Layer
	schemaHash string
	storedAt   time.Time
}

// SemanticCacheOptions tunes the cache. The zero value is usable: normalize()
// fills every field with a safe default, so NewSemanticCache(embedder,
// SemanticCacheOptions{}) is a valid, conservatively-configured cache.
type SemanticCacheOptions struct {
	// Capacity is the maximum number of completions held in the ring. When
	// full, the oldest entry is overwritten (FIFO). <= 0 → semanticDefaultCapacity.
	Capacity int

	// Threshold is the minimum cosine similarity in [0,1] for a hit. Higher is
	// safer (fewer, closer matches). <= 0 or > 1 → semanticDefaultThreshold.
	Threshold float64

	// MinPromptLen is the minimum trimmed prompt length (chars) for a request
	// to be Eligible. Short prompts ("ok", "yes", "fix it") carry too little
	// signal for a safe semantic match. < 0 → semanticDefaultMinPromptLen.
	MinPromptLen int

	// TTL bounds entry freshness. A stored answer older than TTL is treated as
	// a miss on read (and lazily dropped). <= 0 → semanticDefaultTTL.
	TTL time.Duration
}

const (
	// semanticDefaultCapacity is the ring size when unset. Each entry is a
	// normalized vector (BAAI/bge-m3 → 1024 float32 ≈ 4 KiB) plus the cached
	// text, so ~2048 entries stays comfortably inside a process budget while
	// covering a large hot set of recently-answered prompts.
	semanticDefaultCapacity = 2048

	// semanticDefaultThreshold is intentionally high. 0.95 cosine on a strong
	// multilingual retrieval model (bge-m3) corresponds to near-paraphrase, not
	// mere topical overlap — exactly the "same question, different words" case
	// we want to collapse, and nothing looser.
	semanticDefaultThreshold = 0.95

	// semanticDefaultMinPromptLen guards against matching trivially-short
	// prompts whose embeddings are dominated by noise.
	semanticDefaultMinPromptLen = 24

	// semanticDefaultTTL mirrors the exact-hash ResponseCache default so the
	// two stores age out on the same clock.
	semanticDefaultTTL = 30 * time.Minute
)

// normalize returns a copy of the options with every unset / out-of-range
// field replaced by its safe default. It never mutates the receiver.
func (o SemanticCacheOptions) normalize() SemanticCacheOptions {
	out := o
	if out.Capacity <= 0 {
		out.Capacity = semanticDefaultCapacity
	}
	if out.Threshold <= 0 || out.Threshold > 1 {
		out.Threshold = semanticDefaultThreshold
	}
	if out.MinPromptLen < 0 {
		out.MinPromptLen = semanticDefaultMinPromptLen
	}
	if out.TTL <= 0 {
		out.TTL = semanticDefaultTTL
	}
	return out
}

// NewSemanticCache builds a semantic response store backed by the given
// embeddings.Embedder. A nil embedder is permitted — the cache then degrades
// to a permanent miss/no-op, so wiring it before an embedder is configured is
// always safe. opts are normalized; the zero value is fine.
func NewSemanticCache(embedder embeddings.Embedder, opts SemanticCacheOptions) *SemanticCache {
	n := opts.normalize()
	return &SemanticCache{
		embedder: embedder,
		opts:     n,
		logger:   zerolog.Nop(),
		ring:     make([]semanticEntry, 0, n.Capacity),
		indexOf:  make(map[string]int, n.Capacity),
	}
}

// WithLogger attaches a zerolog logger used for debug-level traces of evictions
// and embed failures. nil-safe; returns the cache for chaining.
func (s *SemanticCache) WithLogger(l zerolog.Logger) *SemanticCache {
	if s == nil {
		return s
	}
	s.logger = l
	return s
}

// Eligible reports whether a request may be served from / stored in the
// semantic cache. It is conservative by design and intentionally a superset of
// the constraints the exact-hash store enforces: deterministic intent only
// (Temperature == 0), no tool calls, no attachments (a vision/tool turn is not
// safely replayable), and a prompt long enough to carry a reliable embedding.
//
// A nil embedder makes nothing eligible — there is no point classifying a
// request as cacheable when we can never produce a vector for it.
func (s *SemanticCache) Eligible(req providers.Request) bool {
	if s == nil || s.embedder == nil {
		return false
	}
	if req.Temperature != 0 {
		return false
	}
	if len(req.Tools) > 0 || len(req.Attachments) > 0 {
		return false
	}
	if len(strings.TrimSpace(req.Prompt)) < s.opts.MinPromptLen {
		return false
	}
	return true
}

// Get embeds the query and returns the cached completion of the nearest stored
// vector, IFF all three safety gates hold (cosine >= Threshold, same tier, same
// schema) and the entry is within TTL. Any embed failure, dimension mismatch,
// or empty store is a clean miss.
//
// The embedding round-trip happens OUTSIDE the mutex (it may hit the network);
// only the in-memory nearest-neighbour scan holds the lock. The CachedEmbedder
// LRU in front of a real backend usually makes the embed a local hit anyway.
func (s *SemanticCache) Get(ctx context.Context, req providers.Request) (CachedResponse, bool) {
	if s == nil || s.embedder == nil {
		return CachedResponse{}, false
	}

	queryTier := tierOf(req.Capabilities)
	querySchema := semanticSchemaHash(req.JSONSchema)

	vec, ok := s.embed(ctx, req)
	if !ok {
		return CachedResponse{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	bestSim := s.opts.Threshold
	bestIdx := -1
	now := time.Now()
	for i := range s.ring {
		e := &s.ring[i]
		if len(e.vec) == 0 {
			continue
		}
		// Gate 2 + 3 are cheap scalar checks — apply them before the dot
		// product so a structurally-different candidate is skipped outright.
		if e.tier != queryTier || e.schemaHash != querySchema {
			continue
		}
		if now.Sub(e.storedAt) > s.opts.TTL {
			continue
		}
		if len(e.vec) != len(vec) {
			continue // dimension drift (model swap) — never compare across dims
		}
		sim := semanticCosine(vec, e.vec)
		if sim >= bestSim {
			bestSim = sim
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		return CachedResponse{}, false
	}
	hit := s.ring[bestIdx].resp
	s.logger.Debug().
		Float64("similarity", bestSim).
		Float64("threshold", s.opts.Threshold).
		Str("tier", string(queryTier)).
		Msg("costcascade: semantic cache hit")
	return hit, true
}

// Put embeds the request's key text and stores the completion in the ring under
// the query's tier + schema. An existing entry for the same dedupe key is
// overwritten in place (freshening it) rather than appended; otherwise the
// completion takes the next FIFO slot, overwriting the oldest entry when the
// ring is full.
//
// An empty completion, a nil embedder, or any embed failure is a silent no-op —
// never an error, never a panic.
func (s *SemanticCache) Put(ctx context.Context, req providers.Request, resp CachedResponse) {
	if s == nil || s.embedder == nil {
		return
	}
	if strings.TrimSpace(resp.Text) == "" {
		return // never cache an empty completion — replaying it serves nothing
	}

	vec, ok := s.embed(ctx, req)
	if !ok {
		return
	}

	tier := tierOf(req.Capabilities)
	schema := semanticSchemaHash(req.JSONSchema)
	key := semanticKey(req, tier, schema)
	resp.storedAt = time.Now()

	entry := semanticEntry{
		key:        key,
		vec:        vec,
		resp:       resp,
		tier:       tier,
		schemaHash: schema,
		storedAt:   resp.storedAt,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Overwrite in place if we have already cached this exact key — keeps the
	// ring free of duplicates and refreshes the stored answer + timestamp.
	if idx, exists := s.indexOf[key]; exists && idx >= 0 && idx < len(s.ring) {
		s.ring[idx] = entry
		return
	}

	if len(s.ring) < s.opts.Capacity {
		// Ring not yet full — append.
		s.ring = append(s.ring, entry)
		s.indexOf[key] = len(s.ring) - 1
		return
	}

	// Ring full — overwrite the oldest slot (FIFO eviction at s.next).
	if s.next < 0 || s.next >= len(s.ring) {
		s.next = 0
	}
	evicted := s.ring[s.next]
	if evicted.key != "" {
		delete(s.indexOf, evicted.key)
	}
	s.ring[s.next] = entry
	s.indexOf[key] = s.next
	s.next = (s.next + 1) % s.opts.Capacity
	s.filled = true
}

// embed produces an L2-normalized embedding of the request's key text. It
// returns ok=false on a nil embedder, an embed error, an empty vector, or a
// vector that normalizes to zero (degenerate) — every one of which the callers
// treat as a clean miss / no-op. The embed call is bounded by the caller's ctx.
func (s *SemanticCache) embed(ctx context.Context, req providers.Request) ([]float32, bool) {
	if s.embedder == nil {
		return nil, false
	}
	text := semanticKeyText(req)
	if text == "" {
		return nil, false
	}
	raw, err := s.embedder.Embed(ctx, text)
	if err != nil {
		// Disabled / transient backend failure — degrade to a miss. Debug only:
		// an embed failure is expected when no key is configured (NoopEmbedder).
		s.logger.Debug().Err(err).Msg("costcascade: semantic embed failed; treating as miss")
		return nil, false
	}
	if len(raw) == 0 {
		return nil, false
	}
	vec := semanticNormalize(raw)
	if vec == nil {
		return nil, false
	}
	return vec, true
}

// semanticKeyText builds the canonical text that is embedded for a request:
// System + Prompt + ProjectContext, each trimmed, joined with a separator so
// adjacent fields never bleed into one token boundary. This is what the cosine
// similarity actually compares — phrasing drift in any of these fields is what
// the semantic match is designed to absorb.
func semanticKeyText(req providers.Request) string {
	sys := strings.TrimSpace(req.System)
	prompt := strings.TrimSpace(req.Prompt)
	pctx := strings.TrimSpace(req.ProjectContext)
	parts := make([]string, 0, 3)
	if sys != "" {
		parts = append(parts, sys)
	}
	if prompt != "" {
		parts = append(parts, prompt)
	}
	if pctx != "" {
		parts = append(parts, pctx)
	}
	return strings.Join(parts, "\n␟\n") // U+241F SYMBOL FOR UNIT SEPARATOR
}

// semanticKey is the dedupe identity for an entry: the hash of the key text
// folded together with tier and schema. Two requests share a slot only when
// their embedded text AND their structural gates are identical — so a re-Put of
// the same deterministic request freshens in place rather than duplicating.
func semanticKey(req providers.Request, tier Layer, schemaHash string) string {
	h := sha256.New()
	h.Write([]byte(semanticKeyText(req)))
	h.Write([]byte{0x1f})
	h.Write([]byte(string(tier)))
	h.Write([]byte{0x1f})
	h.Write([]byte(schemaHash))
	return hex.EncodeToString(h.Sum(nil))
}

// semanticSchemaHash collapses a (possibly large) JSONSchema string to a short,
// comparable fingerprint. An empty schema maps to a stable empty marker so
// free-form requests group together and never collide with a structured one.
func semanticSchemaHash(schema string) string {
	trimmed := strings.TrimSpace(schema)
	if trimmed == "" {
		return "" // free-form; distinct from any non-empty schema's hash
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

// semanticNormalize returns an L2-normalized copy of v so a cosine similarity
// reduces to a dot product on the stored vectors. A zero-magnitude (or NaN/Inf-
// poisoned) vector returns nil so callers treat it as a miss rather than divide
// by zero.
func semanticNormalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		f := float64(x)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil
		}
		sum += f * f
	}
	if sum <= 0 {
		return nil
	}
	inv := 1.0 / math.Sqrt(sum)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) * inv)
	}
	return out
}

// semanticCosine returns the cosine similarity of two L2-normalized vectors —
// i.e. their dot product, clamped to [-1, 1] to absorb float rounding so a
// >= Threshold comparison never trips on a 1.0000001 artifact. Mismatched or
// empty lengths return 0 (a guaranteed miss); callers also pre-screen length.
func semanticCosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	if dot > 1 {
		return 1
	}
	if dot < -1 {
		return -1
	}
	return dot
}

// compile-time interface satisfaction — SemanticCache is a drop-in Layer-2
// ResponseStore alongside the exact-hash ResponseCache.
var _ ResponseStore = (*SemanticCache)(nil)
