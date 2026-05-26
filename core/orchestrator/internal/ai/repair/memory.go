package repair

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// MemoryGenome is the in-memory Genome implementation. State is kept
// under a single mutex; the cardinality is small (one recipe per
// failure class) so finer-grained locking isn't worth it.
//
// semantic is the optional embedding-based similarity index. Wired by
// main.go via AttachSemanticIndex when IRONFLYER_REPAIR_SEMANTIC=true;
// nil by default so the exact-match path stays the only repair
// surface unless operators opt in.
type MemoryGenome struct {
	mu       sync.Mutex
	bySig    map[string]*Recipe
	semantic *SemanticIndex
}

// NewMemoryGenome returns a ready-to-use in-memory genome.
func NewMemoryGenome() *MemoryGenome {
	return &MemoryGenome{bySig: make(map[string]*Recipe)}
}

// Record upserts the (signature, category, fix) tuple.
func (g *MemoryGenome) Record(_ context.Context, sig, category string, fix map[string]any) (Recipe, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if existing, ok := g.bySig[sig]; ok {
		return *existing, nil
	}
	r := &Recipe{
		ID:               uuid.New(),
		FailureSignature: sig,
		Category:         category,
		Fix:              fix,
		CreatedAt:        time.Now().UTC(),
	}
	g.bySig[sig] = r
	return *r, nil
}

// Lookup returns the recipe and increments Hits / LastHitAt on match.
func (g *MemoryGenome) Lookup(ctx context.Context, sig string) (Recipe, bool, error) {
	g.mu.Lock()
	r, ok := g.bySig[sig]
	if !ok {
		g.mu.Unlock()
		return Recipe{}, false, nil
	}
	r.Hits++
	r.LastHitAt = time.Now().UTC()
	out := *r
	g.mu.Unlock()
	// Feedback Brain: a recipe match means we reused a learned fix.
	learning.Publish(ctx, learning.OutcomeEvent{
		Kind:       learning.KindRepairTriggered,
		Attributes: map[string]any{
			"signature": sig,
			"category":  out.Category,
			"hits":      out.Hits,
			"reused":    true,
		},
		Success: learning.BoolPtr(true),
	})
	return out, true, nil
}

// MarkSuccess increments the Successes counter for the signature.
func (g *MemoryGenome) MarkSuccess(_ context.Context, sig string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if r, ok := g.bySig[sig]; ok {
		r.Successes++
	}
	return nil
}

// AttemptsByExecution returns per-execution recovery attempts. The
// in-memory genome is keyed by failure signature, not execution, so
// today this returns nil + nil — the wow-loop adapter reads the
// authoritative per-execution view from execution_events via
// execution.Service.RecoveryAttemptsByExecution.
//
// TODO(wave-3): when the genome learns to index attempts by
// executionID (e.g. via an additional in-process per-execution
// counter wired from finisher/recovery.go), populate this slice
// from that index.
func (g *MemoryGenome) AttemptsByExecution(_ context.Context, executionID string) ([]Attempt, error) {
	if executionID == "" {
		return nil, nil
	}
	return nil, nil
}

// Top returns the most-used recipes by Hits, capped to limit.
func (g *MemoryGenome) Top(_ context.Context, limit int) ([]Recipe, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]Recipe, 0, len(g.bySig))
	for _, r := range g.bySig {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hits > out[j].Hits })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// MemoryPatchStore is the in-memory Memory implementation.
type MemoryPatchStore struct {
	mu      sync.Mutex
	byID    map[uuid.UUID]*PatchEntry
	byIntent map[string][]uuid.UUID
}

// NewMemoryPatchStore returns a ready-to-use in-memory patch store.
func NewMemoryPatchStore() *MemoryPatchStore {
	return &MemoryPatchStore{
		byID:     make(map[uuid.UUID]*PatchEntry),
		byIntent: make(map[string][]uuid.UUID),
	}
}

// Record inserts a new PatchEntry for the intent.
func (m *MemoryPatchStore) Record(_ context.Context, intent string, patch map[string]any, paths []string, cost decimal.Decimal) (PatchEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pathsCopy := append([]string(nil), paths...)
	e := &PatchEntry{
		ID:              uuid.New(),
		IntentSignature: intent,
		Patch:           patch,
		AffectedPaths:   pathsCopy,
		CostUSD:         cost,
		CreatedAt:       time.Now().UTC(),
	}
	m.byID[e.ID] = e
	m.byIntent[intent] = append(m.byIntent[intent], e.ID)
	return *e, nil
}

// Find returns every PatchEntry matching the intent signature.
func (m *MemoryPatchStore) Find(_ context.Context, intent string) ([]PatchEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.byIntent[intent]
	out := make([]PatchEntry, 0, len(ids))
	for _, id := range ids {
		if e, ok := m.byID[id]; ok {
			out = append(out, *e)
		}
	}
	return out, nil
}

// MarkApplied bumps AppliedCount + LastAppliedAt; SuccessCount on success.
func (m *MemoryPatchStore) MarkApplied(_ context.Context, id uuid.UUID, success bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.byID[id]
	if !ok {
		return nil
	}
	e.AppliedCount++
	if success {
		e.SuccessCount++
	}
	e.LastAppliedAt = time.Now().UTC()
	return nil
}
