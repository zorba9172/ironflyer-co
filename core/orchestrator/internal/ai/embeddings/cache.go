package embeddings

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"sync"
)

// defaultEmbedLRUCap is the fall-back capacity used when
// IRONFLYER_EMBED_LRU_CAP is unset, empty, or unparseable. 10_000 cached
// vectors at ~1.5 KiB apiece (384-dim float32) stays under ~15 MiB — well
// inside any realistic process budget — while absorbing the hot-path
// retrieval traffic without paying for an HF round-trip every time.
const defaultEmbedLRUCap = 10_000

// embedLRUCapFromEnv parses IRONFLYER_EMBED_LRU_CAP. Anything that isn't a
// positive integer falls back to the default cap; operators can still tune
// it without risking an instant misconfiguration.
func embedLRUCapFromEnv() int {
	raw := os.Getenv("IRONFLYER_EMBED_LRU_CAP")
	if raw == "" {
		return defaultEmbedLRUCap
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultEmbedLRUCap
	}
	return n
}

// cacheEntry is the per-key payload stored in the LRU list. The list
// element points back at this struct so eviction can recover the key
// without an auxiliary reverse-map.
type cacheEntry struct {
	key string // sha256(text)
	vec []float32
}

// CachedEmbedder wraps any Embedder with a bounded LRU keyed by the
// sha256 of the input text. Both single and batch calls are memoised —
// repeated retrieval of the same Substring (the common case in the
// orchestrator memory pipeline) hits the cache without paying for an
// inference round-trip. Backs both the HF and ONNX backends.
//
// Implementation note: the LRU is inline (container/list + map) rather
// than a dependency, mirroring the pattern in internal/notify/prefs.go.
// O(1) reads, writes, and evictions, no new go.mod bill.
type CachedEmbedder struct {
	inner Embedder

	mu    sync.Mutex
	cap   int
	order *list.List               // front = most recently used
	index map[string]*list.Element // sha256(text) → element holding *cacheEntry
}

// NewCachedEmbedder wraps inner with an LRU sized via
// IRONFLYER_EMBED_LRU_CAP (positive int). Invalid / missing values fall
// back to defaultEmbedLRUCap. A nil inner is a programmer error — callers
// must always pass a concrete backend (HuggingFace, ONNX, or Noop).
func NewCachedEmbedder(inner Embedder) *CachedEmbedder {
	return NewCachedEmbedderWithCap(inner, embedLRUCapFromEnv())
}

// NewCachedEmbedderWithCap constructs a CachedEmbedder with an explicit
// capacity. Non-positive caps are silently rounded up to
// defaultEmbedLRUCap so the cache is never effectively disabled.
func NewCachedEmbedderWithCap(inner Embedder, cap int) *CachedEmbedder {
	if cap <= 0 {
		cap = defaultEmbedLRUCap
	}
	return &CachedEmbedder{
		inner: inner,
		cap:   cap,
		order: list.New(),
		index: make(map[string]*list.Element),
	}
}

// Dim delegates to the inner embedder. The cache itself is dimension-
// agnostic; whatever the backend emits gets stored verbatim.
func (c *CachedEmbedder) Dim() int { return c.inner.Dim() }

// hashKey returns the canonical cache key for a piece of input text. We
// hash rather than store text directly so memory usage is bounded
// regardless of how long the inputs are.
func hashKey(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// lookup is the read path under the LRU mutex; promotes hits to MRU.
func (c *CachedEmbedder) lookup(key string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.index[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*cacheEntry).vec, true
}

// store inserts a vector under key, promoting or evicting as needed.
// A nil / empty vec is not cached — the backend likely errored, and we
// want a future call to retry rather than memoise the failure.
func (c *CachedEmbedder) store(key string, vec []float32) {
	if len(vec) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.index[key]; ok {
		el.Value.(*cacheEntry).vec = vec
		c.order.MoveToFront(el)
		return
	}
	el := c.order.PushFront(&cacheEntry{key: key, vec: vec})
	c.index[key] = el
	// Evict from the tail until we're back within cap. A single store can
	// only add one entry, so this loop runs at most once in practice;
	// the loop guards against future code paths that batch inserts.
	for c.order.Len() > c.cap {
		tail := c.order.Back()
		if tail == nil {
			break
		}
		c.order.Remove(tail)
		delete(c.index, tail.Value.(*cacheEntry).key)
	}
}

// Embed returns a cached vector for text if present, otherwise calls the
// inner embedder and caches a successful result.
func (c *CachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	key := hashKey(text)
	if vec, ok := c.lookup(key); ok {
		return vec, nil
	}
	vec, err := c.inner.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	c.store(key, vec)
	return vec, nil
}

// EmbedBatch fans out per-input cache lookups, then issues a single
// batched call for the misses. The result order matches the input
// order — the caller can't tell the cache exists from the response.
func (c *CachedEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float32, len(texts))
	missIdx := make([]int, 0, len(texts))
	missText := make([]string, 0, len(texts))
	missKey := make([]string, 0, len(texts))
	for i, t := range texts {
		key := hashKey(t)
		if vec, ok := c.lookup(key); ok {
			out[i] = vec
			continue
		}
		missIdx = append(missIdx, i)
		missText = append(missText, t)
		missKey = append(missKey, key)
	}
	if len(missText) == 0 {
		return out, nil
	}
	vecs, err := c.inner.EmbedBatch(ctx, missText)
	if err != nil {
		return nil, err
	}
	for j, v := range vecs {
		if j >= len(missIdx) {
			break
		}
		out[missIdx[j]] = v
		c.store(missKey[j], v)
	}
	return out, nil
}

// compile-time interface satisfaction.
var _ Embedder = (*CachedEmbedder)(nil)
