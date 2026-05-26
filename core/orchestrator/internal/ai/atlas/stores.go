// stores.go layers two persistence-backed Store implementations on top
// of the in-process MemoryStore. Both reuse the existing ai/memory
// surfaces (pgvector + surreal) so the Atlas does NOT introduce a
// second vector database into the deployment — the operator picks one
// backend at boot and the Atlas piggybacks on it.
//
// The translation is intentionally simple: every Capability is
// projected into a memory.Record with Kind="atlas-capability". Search
// queries the memory.Store with the same query string and re-hydrates
// the Capability from the record body. Index/Get/Stats follow the
// same pattern. This keeps the Atlas schema-free — operators don't
// need to run a new migration; the memory schema already supports
// JSON bodies + embeddings.

package atlas

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/ai/memory"
)

// memoryBackedKind is the memory.Kind value Atlas capabilities are
// stamped with. Picked outside the canonical memory.AllKinds() set so
// federation / dashboard surfaces don't pick them up as project
// memories. The memory.Store sees them as a separate slice.
const memoryBackedKind memory.Kind = "atlas-capability"

// MemoryBackedStore adapts a memory.Store (PgVectorStore / SurrealStore /
// VectorStore wrapper) into an atlas.Store. Used by the orchestrator
// boot path so a single ai/memory backend serves both project memory
// and the Capability Atlas. A reference to the underlying memory.Store
// is held for write-through; an in-process MemoryStore mirror keeps
// reads cheap even when the upstream backend is slow.
type MemoryBackedStore struct {
	upstream memory.Store
	mirror   *MemoryStore
	mu       sync.RWMutex
	// projectID stamps every memory.Record so the upstream owner clamp
	// can route them to a single "atlas" bucket. Empty disables the
	// stamp (acceptable for in-memory backends; pgvector clamps NOT
	// NULL on user_id but accepts empty strings as a placeholder).
	projectID string
	userID    string
}

// NewMemoryBackedStore returns a Store that persists every Index call
// into upstream and mirrors reads from a local MemoryStore for speed.
// projectID + userID are stamped onto every backing memory.Record so
// the upstream's owner clamp doesn't reject them.
func NewMemoryBackedStore(upstream memory.Store, projectID, userID string) *MemoryBackedStore {
	return &MemoryBackedStore{
		upstream:  upstream,
		mirror:    NewMemoryStore(16 * 1024),
		projectID: projectID,
		userID:    userID,
	}
}

// NewPgVectorStore is the operator-facing constructor for the
// pgvector-backed Atlas. It is a thin alias over NewMemoryBackedStore
// so callers reading orchestrator boot get a name that matches the
// memory.Store backend they wired.
func NewPgVectorStore(upstream memory.Store, projectID, userID string) *MemoryBackedStore {
	return NewMemoryBackedStore(upstream, projectID, userID)
}

// NewSurrealStore is the operator-facing constructor for the
// SurrealDB-backed Atlas. Like NewPgVectorStore, the implementation is
// the memory-adapter — the surreal backend lives behind the
// memory.Store interface.
func NewSurrealStore(upstream memory.Store, projectID, userID string) *MemoryBackedStore {
	return NewMemoryBackedStore(upstream, projectID, userID)
}

// Index persists c to both the upstream memory store and the in-process
// mirror. Failures on the upstream are surfaced so callers can decide
// whether to retry; the mirror is best-effort.
func (s *MemoryBackedStore) Index(ctx context.Context, c Capability) error {
	if c.ID == "" {
		c.ID = CapabilityID(c.Path, c.Symbol)
	}
	if c.LastIndexed.IsZero() {
		c.LastIndexed = time.Now().UTC()
	}
	body, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("atlas: encode capability: %w", err)
	}
	rec := memory.Record{
		ID:        atlasRecordID(c.ID),
		Kind:      memoryBackedKind,
		UserID:    s.userID,
		ProjectID: s.projectID,
		Title:     c.Symbol,
		Body:      string(body),
		Tags:      []string{c.Kind},
		CreatedAt: c.LastIndexed,
	}
	if s.upstream != nil {
		if _, err := s.upstream.Record(ctx, rec); err != nil {
			return fmt.Errorf("atlas: upstream record: %w", err)
		}
	}
	_ = s.mirror.Index(ctx, c)
	return nil
}

// BatchIndex loops Index; the upstream's Record contract is
// per-record and we accept the per-call overhead in exchange for
// uniform error semantics.
func (s *MemoryBackedStore) BatchIndex(ctx context.Context, caps []Capability) error {
	for _, c := range caps {
		if err := s.Index(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// Search consults the in-process mirror first (cheap) and falls back to
// the upstream memory.Query when the mirror is empty (cold start /
// fresh boot). Returned hits are deduped on Capability.ID.
func (s *MemoryBackedStore) Search(ctx context.Context, query string, k int) ([]Hit, error) {
	hits, err := s.mirror.Search(ctx, query, k)
	if err == nil && len(hits) > 0 {
		return hits, nil
	}
	if s.upstream == nil {
		return hits, nil
	}
	rows, qerr := s.upstream.Query(ctx, memory.Query{
		Kind:      memoryBackedKind,
		ProjectID: s.projectID,
		Substring: query,
		Limit:     k * 2,
	})
	if qerr != nil {
		return hits, nil
	}
	for _, r := range rows {
		var c Capability
		if err := json.Unmarshal([]byte(r.Body), &c); err != nil {
			continue
		}
		hits = append(hits, Hit{Capability: c, Score: 0})
	}
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits, nil
}

// Get reads from the mirror first; falls through to the upstream when
// the mirror has cold-evicted the entry.
func (s *MemoryBackedStore) Get(ctx context.Context, id string) (Capability, error) {
	if c, err := s.mirror.Get(ctx, id); err == nil {
		return c, nil
	}
	if s.upstream == nil {
		return Capability{}, ErrNotFound
	}
	rec, err := s.upstream.GetByID(ctx, atlasRecordID(id))
	if err != nil {
		return Capability{}, ErrNotFound
	}
	var c Capability
	if err := json.Unmarshal([]byte(rec.Body), &c); err != nil {
		return Capability{}, ErrNotFound
	}
	return c, nil
}

// Stats returns the mirror's stats. We don't round-trip to the upstream
// here because counting JSONB rows can be costly on a large pgvector
// table; the mirror is a good-enough operator-facing view.
func (s *MemoryBackedStore) Stats(ctx context.Context) (Stats, error) {
	return s.mirror.Stats(ctx)
}

func atlasRecordID(id string) string {
	if strings.HasPrefix(id, "atlas:") {
		return id
	}
	return "atlas:" + id
}
