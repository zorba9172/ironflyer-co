// Package atlas implements the Capability Atlas — the live index of
// every reusable utility, hook, component, service, and blueprint that
// already exists in the repository. The Reuse-First Preflight (see
// agents/preflight.go) queries the Atlas before the Coder agent is
// allowed to open a new file; the Anti-Bloat gate `reuse_check` blocks
// patches that skip the lookup. See docs/ANTI_BLOAT_ENGINE.md and
// playbook §8.3 for the design rationale.
//
// The Atlas is intentionally tiny in surface area. The Store interface
// captures the four operations every backend must support; the
// MemoryStore implementation is suitable for development; the
// PgVectorStore + SurrealStore wrappers reuse the existing ai/memory
// backends so operators don't pay for a second vector store.
package atlas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

// Capability represents one reusable artifact discovered in the repo.
// The same shape covers every kind ("func", "hook", "component",
// "service", "blueprint") so the Coder + Architect agents can render a
// uniform menu of reuse candidates.
//
// ID is a stable hash of `path:symbol`; collisions across kinds are
// impossible because a Go function and a TS hook can't share both the
// same path and the same exported name in the same file. The
// Embedding is whatever the wired embeddings.Embedder produces (bge-m3
// by default — 1024 dimensions); the Store is responsible for the
// similarity math.
type Capability struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Symbol      string    `json:"symbol"`
	Kind        string    `json:"kind"`
	Signature   string    `json:"signature,omitempty"`
	Doc         string    `json:"doc,omitempty"`
	Exports     []string  `json:"exports,omitempty"`
	UsageCount  int       `json:"usageCount,omitempty"`
	Embedding   []float32 `json:"embedding,omitempty"`
	LastIndexed time.Time `json:"lastIndexed"`
}

// Hit is one match returned by Search. Score is cosine similarity in
// [0, 1] for embedding-backed stores; the lexical fallback (used when
// no embedder is wired) returns a substring-overlap heuristic on the
// same scale.
type Hit struct {
	Capability Capability `json:"capability"`
	Score      float32    `json:"score"`
}

// Stats is the operator-facing snapshot of the Atlas's contents.
// Returned by Store.Stats; the Code Health Dashboard renders it under
// the "Atlas coverage" panel.
type Stats struct {
	Total        int            `json:"total"`
	ByKind       map[string]int `json:"byKind"`
	WithEmbedding int           `json:"withEmbedding"`
	LastIndexed  time.Time      `json:"lastIndexed"`
}

// Store is the operator-replaceable contract. Implementations MUST be
// safe for concurrent use; the indexer + gate + preflight paths hit
// this in parallel.
type Store interface {
	Index(ctx context.Context, cap Capability) error
	BatchIndex(ctx context.Context, caps []Capability) error
	Search(ctx context.Context, query string, k int) ([]Hit, error)
	Get(ctx context.Context, id string) (Capability, error)
	Stats(ctx context.Context) (Stats, error)
}

// ErrNotFound is returned by Get when no capability matches the id.
var ErrNotFound = errors.New("atlas: capability not found")

// CapabilityID computes a stable identifier from path + symbol. The
// indexer uses it; callers that build a Capability by hand should call
// it too so update-by-id semantics work.
func CapabilityID(path, symbol string) string {
	h := sha256.Sum256([]byte(path + ":" + symbol))
	return hex.EncodeToString(h[:12])
}

// MemoryStore is the default in-process backend. It keeps a bounded
// ring of capabilities and supports both embedding-cosine search and a
// lexical fallback when the capability has no embedding.
type MemoryStore struct {
	mu     sync.RWMutex
	rows   map[string]Capability // id → cap
	order  []string              // insertion order for eviction
	maxLen int
}

// NewMemoryStore returns a fresh backend with the given cap. Zero
// (the default) sets a 16k floor — large enough for any repo we ship
// today, small enough to bound RAM at ~60 MiB per instance.
func NewMemoryStore(maxLen int) *MemoryStore {
	if maxLen <= 0 {
		maxLen = 16 * 1024
	}
	return &MemoryStore{
		rows:   make(map[string]Capability, 1024),
		maxLen: maxLen,
	}
}

// Index inserts or replaces a single capability.
func (m *MemoryStore) Index(_ context.Context, c Capability) error {
	if c.ID == "" {
		c.ID = CapabilityID(c.Path, c.Symbol)
	}
	if c.LastIndexed.IsZero() {
		c.LastIndexed = time.Now().UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, existed := m.rows[c.ID]; !existed {
		if len(m.order) >= m.maxLen {
			// Evict oldest.
			old := m.order[0]
			m.order = m.order[1:]
			delete(m.rows, old)
		}
		m.order = append(m.order, c.ID)
	}
	m.rows[c.ID] = c
	return nil
}

// BatchIndex bulk-loads many capabilities. The default implementation
// loops Index; backends with a real batch API override this.
func (m *MemoryStore) BatchIndex(ctx context.Context, caps []Capability) error {
	for _, c := range caps {
		if err := m.Index(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// Search returns the top k matches. When query is empty, returns the k
// most recently indexed entries (operator browse).
//
// Two paths:
//
//   - If the query maps to an embedding (callers pass the already-
//     embedded vector via the QueryEmbedding context key, see
//     WithQueryEmbedding), every capability with a non-empty Embedding
//     is scored by cosine similarity.
//   - Otherwise a cheap lexical heuristic ranks by overlap of
//     lowercased tokens with `path + symbol + doc`.
func (m *MemoryStore) Search(ctx context.Context, query string, k int) ([]Hit, error) {
	if k <= 0 {
		k = 5
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if vec, ok := QueryEmbedding(ctx); ok && len(vec) > 0 {
		return topKByCosine(m.rows, vec, k), nil
	}
	if strings.TrimSpace(query) == "" {
		return topKByRecency(m.rows, m.order, k), nil
	}
	return topKByLexical(m.rows, query, k), nil
}

// Get returns the capability for id, or ErrNotFound.
func (m *MemoryStore) Get(_ context.Context, id string) (Capability, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.rows[id]
	if !ok {
		return Capability{}, ErrNotFound
	}
	return c, nil
}

// Stats returns a snapshot of the store's contents.
func (m *MemoryStore) Stats(_ context.Context) (Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := Stats{ByKind: map[string]int{}}
	for _, c := range m.rows {
		s.Total++
		s.ByKind[c.Kind]++
		if len(c.Embedding) > 0 {
			s.WithEmbedding++
		}
		if c.LastIndexed.After(s.LastIndexed) {
			s.LastIndexed = c.LastIndexed
		}
	}
	return s, nil
}

// ---- ranking helpers ------------------------------------------------

func topKByRecency(rows map[string]Capability, order []string, k int) []Hit {
	out := make([]Hit, 0, k)
	for i := len(order) - 1; i >= 0 && len(out) < k; i-- {
		c, ok := rows[order[i]]
		if !ok {
			continue
		}
		out = append(out, Hit{Capability: c, Score: 0})
	}
	return out
}

func topKByLexical(rows map[string]Capability, query string, k int) []Hit {
	q := strings.ToLower(query)
	tokens := tokenize(q)
	out := make([]Hit, 0, len(rows))
	for _, c := range rows {
		hay := strings.ToLower(c.Path + " " + c.Symbol + " " + c.Doc)
		if hay == "" {
			continue
		}
		var hits int
		for _, t := range tokens {
			if t == "" {
				continue
			}
			if strings.Contains(hay, t) {
				hits++
			}
		}
		if hits == 0 {
			continue
		}
		score := float32(hits) / float32(maxInt(len(tokens), 1))
		out = append(out, Hit{Capability: c, Score: score})
	}
	sortHitsDesc(out)
	if len(out) > k {
		out = out[:k]
	}
	return out
}

func topKByCosine(rows map[string]Capability, q []float32, k int) []Hit {
	out := make([]Hit, 0, len(rows))
	for _, c := range rows {
		if len(c.Embedding) != len(q) {
			continue
		}
		score := cosine(q, c.Embedding)
		out = append(out, Hit{Capability: c, Score: score})
	}
	sortHitsDesc(out)
	if len(out) > k {
		out = out[:k]
	}
	return out
}

func cosine(a, b []float32) float32 {
	var dot, na, nb float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		na += ai * ai
		nb += bi * bi
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (sqrt(na) * sqrt(nb)))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func sortHitsDesc(hits []Hit) {
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].Score > hits[j-1].Score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
}

func tokenize(s string) []string {
	out := make([]string, 0, 4)
	var cur strings.Builder
	for _, r := range s {
		if r == ' ' || r == '_' || r == '-' || r == '/' || r == '.' || r == ',' {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---- context-carried query embedding -------------------------------

type queryEmbeddingKey struct{}

// WithQueryEmbedding attaches a pre-computed query vector to ctx so
// Store.Search can do cosine-similarity ranking without re-embedding
// the query text every call. The preflight helper attaches it once.
func WithQueryEmbedding(ctx context.Context, v []float32) context.Context {
	return context.WithValue(ctx, queryEmbeddingKey{}, v)
}

// QueryEmbedding extracts the query vector attached by
// WithQueryEmbedding, or returns ok=false.
func QueryEmbedding(ctx context.Context) ([]float32, bool) {
	v, ok := ctx.Value(queryEmbeddingKey{}).([]float32)
	return v, ok && len(v) > 0
}
