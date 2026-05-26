// VectorStore wraps an existing Store with semantic search powered by
// an Embedder. Writes encode the record body into a vector that's
// stored alongside the record; queries with a non-empty Substring
// turn the substring into a query vector and rank existing records
// by cosine similarity.
//
// The wrapper is fully nil-safe: when Embedder is nil OR the encode
// call errors, we transparently fall through to the wrapped Store's
// default behaviour (substring contains match). That keeps the
// orchestrator bootable without HF configured.

package memory

import (
	"context"
	"math"
	"sort"
	"sync"

	"ironflyer/core/orchestrator/internal/ai/embeddings"
)

// VectorStore wraps a Store with semantic search.
type VectorStore struct {
	Inner    Store
	Embedder embeddings.Embedder
	// Threshold is the minimum cosine similarity 0..1 a record needs
	// to clear before it's returned. Default 0.0 (return everything,
	// ranked by similarity).
	Threshold float32

	vectors sync.Map // map[string][]float32 keyed by record.ID
}

// NewVectorStore is the canonical constructor. Inner must be non-nil.
// A nil embedder is allowed: the wrapper degrades to plain Inner calls.
func NewVectorStore(inner Store, e embeddings.Embedder, threshold float32) *VectorStore {
	return &VectorStore{Inner: inner, Embedder: e, Threshold: threshold}
}

// Record persists r through the inner store, then asynchronously
// encodes its (title + body) into a vector cached in-process. The
// async path keeps writes off HF's latency budget.
func (v *VectorStore) Record(ctx context.Context, r Record) (Record, error) {
	saved, err := v.Inner.Record(ctx, r)
	if err != nil {
		return saved, err
	}
	if v.Embedder == nil {
		return saved, nil
	}
	text := saved.Title
	if saved.Body != "" {
		if text != "" {
			text += "\n\n"
		}
		text += saved.Body
	}
	if text == "" {
		return saved, nil
	}
	id := saved.ID
	go func(id, text string) {
		// Detach from the caller's context so cancellation of the
		// originating request doesn't drop the embed. Use a fresh
		// background context; the HTTP client's 30s timeout caps
		// runaway calls.
		vec, err := v.Embedder.Embed(context.Background(), text)
		if err != nil || len(vec) == 0 {
			return
		}
		v.vectors.Store(id, vec)
	}(id, text)
	return saved, nil
}

// Query routes through semantic re-ranking when Substring is set and
// an Embedder is wired. All other paths delegate straight to Inner.
func (v *VectorStore) Query(ctx context.Context, q Query) ([]Record, error) {
	if q.Substring == "" || v.Embedder == nil {
		return v.Inner.Query(ctx, q)
	}

	// Encode the query first; if HF fails we keep the inner result
	// so the orchestrator never goes blind on a transient HF outage.
	qVec, err := v.Embedder.Embed(ctx, q.Substring)
	if err != nil || len(qVec) == 0 {
		return v.Inner.Query(ctx, q)
	}

	// Pull a wider candidate set so the re-rank has room to work.
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	candidateLimit := limit * 4
	if candidateLimit > 200 {
		candidateLimit = 200
	}
	// Drop the Substring filter from the inner query so the contains
	// match doesn't pre-prune records that semantic search would have
	// surfaced. The other scopes (Kind, ProjectID, …) stay.
	innerQ := q
	innerQ.Substring = ""
	innerQ.Limit = candidateLimit
	candidates, err := v.Inner.Query(ctx, innerQ)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return candidates, nil
	}

	type scored struct {
		rec Record
		sim float32
		idx int // original order, used as a stable tiebreaker
	}
	scoredAll := make([]scored, 0, len(candidates))
	missing := make([]scored, 0)
	for i, rec := range candidates {
		raw, ok := v.vectors.Load(rec.ID)
		if !ok {
			missing = append(missing, scored{rec: rec, idx: i})
			continue
		}
		vec, ok := raw.([]float32)
		if !ok || len(vec) == 0 {
			missing = append(missing, scored{rec: rec, idx: i})
			continue
		}
		sim := cosine(qVec, vec)
		if sim < v.Threshold {
			continue
		}
		scoredAll = append(scoredAll, scored{rec: rec, sim: sim, idx: i})
	}

	// If every candidate is missing a stored vector, semantic search
	// can't help yet — return the inner ordering so we don't degrade.
	if len(scoredAll) == 0 {
		out := make([]Record, 0, limit)
		for _, m := range missing {
			out = append(out, m.rec)
			if len(out) >= limit {
				break
			}
		}
		return out, nil
	}

	sort.SliceStable(scoredAll, func(i, j int) bool {
		if scoredAll[i].sim == scoredAll[j].sim {
			return scoredAll[i].idx < scoredAll[j].idx
		}
		return scoredAll[i].sim > scoredAll[j].sim
	})

	out := make([]Record, 0, limit)
	for _, s := range scoredAll {
		out = append(out, s.rec)
		if len(out) >= limit {
			break
		}
	}
	// Backfill from records we couldn't score yet so callers still see
	// recent writes that haven't been embedded in the background.
	if len(out) < limit {
		for _, m := range missing {
			out = append(out, m.rec)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// GetByID delegates to the inner store. The vector cache is per-id but
// does not hold a Record so we cannot answer without the inner store.
func (v *VectorStore) GetByID(ctx context.Context, id string) (Record, error) {
	return v.Inner.GetByID(ctx, id)
}

// Delete clears the cached vector and delegates.
//
// TENANT ISOLATION: ownership-check is the caller's job; the wrapper
// does NOT re-derive the owner from the cached vector entry. See the
// invariant on memory.Store.
func (v *VectorStore) Delete(ctx context.Context, id string) error {
	v.vectors.Delete(id)
	return v.Inner.Delete(ctx, id)
}

// cosine returns the cosine similarity of two equal-length vectors,
// clamped to 0 on degenerate inputs (mismatched lengths, zero vectors).
func cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// compile-time interface satisfaction.
var _ Store = (*VectorStore)(nil)
