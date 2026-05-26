package repair

// Semantic repair matching — V22 proprietary model #1.
//
// The exact-match repair Genome (signature -> Recipe) only catches
// failures whose normalised text already produced a recipe row. Two
// runs that hit the same bug but print the error differently look
// like two distinct failure classes to the exact matcher, so the
// reasoning loop pays full price on the second occurrence.
//
// SemanticIndex closes that gap by augmenting the exact Genome with
// an embedding-based similarity search. Each recipe's failure
// signature text is encoded with the configured bge-m3 embedder; on
// lookup we encode the incoming failure description and return the
// top-K recipes by cosine similarity. The threshold and K are
// caller-tunable.
//
// The index is opt-in via IRONFLYER_REPAIR_SEMANTIC=true. When
// disabled (the default), the existing exact-match Lookup path is
// the only repair surface — keeps the boot dependency on a running
// embedder out of the critical path.
//
// State: vectors live in an in-process cache keyed by recipe ID.
// ReindexAll rebuilds from the underlying Genome on startup; Index
// is invoked on each Record so newly-recorded recipes are
// immediately searchable without waiting for a sweep.

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"

	"github.com/google/uuid"

	"ironflyer/core/orchestrator/internal/ai/embeddings"
)

// DefaultSemanticThreshold is the cosine similarity floor below which a
// SemanticIndex hit is treated as "no match". 0.82 is the operator
// rule-of-thumb for bge-m3 on prose+error retrieval; tighter than the
// generic 0.7 because false positives in repair are expensive.
const DefaultSemanticThreshold = 0.82

// HighConfidenceSemanticThreshold is the cosine similarity above which a
// semantic hit is treated as "as good as an exact match" — the agent
// may apply the recipe without surfacing a candidate to the operator.
const HighConfidenceSemanticThreshold = 0.92

// SimilarHit is one ranked result from a semantic lookup. Score is the
// raw cosine similarity in [-1, 1] but in practice always lands in
// (0, 1] for non-degenerate inputs.
type SimilarHit struct {
	Recipe Recipe
	Score  float32
}

// SemanticIndex augments the exact-match Genome with embedding-based
// similarity. It is safe for concurrent use; the embed-and-search
// path is read-mostly and protected by an RWMutex.
type SemanticIndex struct {
	embedder  embeddings.Embedder
	threshold float64
	store     Genome

	mu      sync.RWMutex
	vectors map[string]semanticEntry // recipe ID -> entry
}

type semanticEntry struct {
	recipe Recipe
	vec    []float32
}

// SemanticOpt configures a SemanticIndex.
type SemanticOpt func(*SemanticIndex)

// WithSemanticThreshold pins the cosine similarity floor for matches.
// Values outside (0, 1] are silently coerced into DefaultSemanticThreshold.
func WithSemanticThreshold(t float64) SemanticOpt {
	return func(si *SemanticIndex) {
		if t <= 0 || t > 1 {
			t = DefaultSemanticThreshold
		}
		si.threshold = t
	}
}

// NewSemanticIndex constructs an index bound to an embedder and the
// existing exact-match Genome. The embedder is required — callers
// gate construction on IRONFLYER_REPAIR_SEMANTIC so the boot path
// never builds a SemanticIndex without a live embedder behind it.
func NewSemanticIndex(embedder embeddings.Embedder, store Genome, opts ...SemanticOpt) *SemanticIndex {
	si := &SemanticIndex{
		embedder:  embedder,
		threshold: DefaultSemanticThreshold,
		store:     store,
		vectors:   make(map[string]semanticEntry),
	}
	for _, opt := range opts {
		opt(si)
	}
	return si
}

// Threshold returns the configured cosine similarity floor.
func (si *SemanticIndex) Threshold() float64 {
	if si == nil {
		return DefaultSemanticThreshold
	}
	return si.threshold
}

// Index encodes the recipe's failure signature text and stores the
// resulting vector under the recipe ID. Idempotent — re-indexing the
// same recipe overwrites the prior vector.
//
// The recipe's FailureSignature is a SHA-256 hex string of the
// already-normalised failure text, which is not directly useful for
// embedding. Callers that have the raw failure text on hand should
// call IndexWithText instead; this method falls back to encoding the
// hex itself, which is a stable per-recipe key but carries no
// semantic signal. The intent is that Index is invoked from
// LookupSimilar's same-process indexer once we have the raw text,
// and from ReindexAll where we may only have the hex.
func (si *SemanticIndex) Index(ctx context.Context, recipe Recipe) error {
	return si.IndexWithText(ctx, recipe, recipe.FailureSignature)
}

// IndexWithText is the explicit-text indexer. text should be the raw
// failure description that produced recipe.FailureSignature; that
// gives the embedder real prose to encode rather than a hex digest.
func (si *SemanticIndex) IndexWithText(ctx context.Context, recipe Recipe, text string) error {
	if si == nil || si.embedder == nil {
		return errors.New("repair: semantic index not configured")
	}
	if text == "" {
		text = recipe.FailureSignature
	}
	vec, err := si.embedder.Embed(ctx, text)
	if err != nil {
		return err
	}
	si.mu.Lock()
	si.vectors[recipe.ID.String()] = semanticEntry{recipe: recipe, vec: vec}
	si.mu.Unlock()
	return nil
}

// LookupSimilar encodes the failure description and returns up to k
// hits with score >= threshold, sorted by score descending.
//
// The method is nil-safe at the receiver and returns (nil, nil) when
// the index is unconfigured so callers can short-circuit without
// special-casing.
func (si *SemanticIndex) LookupSimilar(ctx context.Context, failureDescription string, k int) ([]SimilarHit, error) {
	if si == nil || si.embedder == nil {
		return nil, nil
	}
	if failureDescription == "" || k <= 0 {
		return nil, nil
	}
	query, err := si.embedder.Embed(ctx, failureDescription)
	if err != nil {
		return nil, err
	}
	si.mu.RLock()
	hits := make([]SimilarHit, 0, len(si.vectors))
	for _, ent := range si.vectors {
		score := cosineSimilarity(query, ent.vec)
		if float64(score) < si.threshold {
			continue
		}
		hits = append(hits, SimilarHit{Recipe: ent.recipe, Score: score})
	}
	si.mu.RUnlock()
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits, nil
}

// ReindexAll re-embeds every recipe known to the underlying Genome.
// Intended for boot-time warm-up: the index is in-process state and
// is empty after a restart. The method walks Top(limit=10_000)
// because the Genome interface doesn't expose a full-scan iterator;
// repairs at scale are bounded — a tenant with more than 10k
// distinct failure classes is well outside the v22 design envelope.
func (si *SemanticIndex) ReindexAll(ctx context.Context, _ string) error {
	if si == nil || si.embedder == nil || si.store == nil {
		return nil
	}
	const reindexCap = 10_000
	recipes, err := si.store.Top(ctx, reindexCap)
	if err != nil {
		return err
	}
	for _, r := range recipes {
		if err := si.Index(ctx, r); err != nil {
			// Skip individual failures so one stuck recipe doesn't poison
			// the whole warm-up. The miss will be re-tried on the next
			// successful Record / Index call for that recipe.
			continue
		}
	}
	return nil
}

// Size reports the number of indexed vectors. Used by /healthz-style
// probes and the boot log so operators see the warm-up landed.
func (si *SemanticIndex) Size() int {
	if si == nil {
		return 0
	}
	si.mu.RLock()
	defer si.mu.RUnlock()
	return len(si.vectors)
}

// cosineSimilarity is the standard dot / (||a||*||b||) similarity in
// (-1, 1]. Returns 0 on a zero-norm input so callers don't have to
// special-case empty vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// _ keeps the uuid import live for callers that want to construct a
// SimilarHit with a synthetic Recipe (e.g. test/sandbox harnesses).
var _ = uuid.Nil
