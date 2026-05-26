package finisher

// LearningHooks is the V22 best-effort learning bridge for the
// finisher engine. The finisher loops invoke the methods on this type
// at the points where a "thing was learned" — a retry succeeded
// against a known failure class, a patch landed cleanly, a recovery
// short-circuited via the genome. The hooks turn those signals into
// durable rows in the repair genome + patch memory.
//
// Every method is nil-safe at the receiver, at the underlying store,
// and at the input level: a zero-valued LearningHooks (or a partially
// wired one) behaves as a no-op. Errors from the stores are logged on
// the engine logger but NEVER propagate — learning is purely additive
// and must not break the user-facing run.
//
// Concurrency: the underlying stores own their own locking. The hooks
// themselves carry no shared state.
//
// Wired by cmd/orchestrator/main.go via Engine.WithLearning().

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/repair"
)

// LearningHooks is the seam between the finisher's runtime hot path
// and the V22 repair genome + patch memory persistence layer.
//
// Two stores back the hooks:
//
//   - Genome (repair.Genome) — keyed by a normalised failure
//     signature. Records "this failure shape was fixed by this fix
//     shape" so the next occurrence can short-circuit reasoning.
//
//   - Patches (repair.Memory) — keyed by an intent signature derived
//     from the gate context. Records "this patch shape satisfied
//     this intent" so a repeat intent can re-apply (or rank up) the
//     known patch instead of regenerating.
//
// In addition, a small in-memory counter (repairsByExec) tracks how
// many recovery rounds an execution went through. Adapters in
// main.go consult it to back-fill the blueprint stats Repaired=true
// flag when the run terminates.
type LearningHooks struct {
	Genome  repair.Genome
	Patches repair.Memory

	// repairsByExec is the per-execution repair counter; bumped by
	// OnRetrySuccess and read by the engine-settler adapter so the
	// blueprint stats row carries Repaired=true on settle without
	// having to mutate the execution.Settler internals.
	mu             sync.Mutex
	repairsByExec  map[string]int
	patchHitsByExec map[string]int
}

// NewLearningHooks constructs a LearningHooks bound to the supplied
// stores. Either store may be nil — the corresponding leg of the hook
// degrades to a no-op so partial wiring is safe.
func NewLearningHooks(genome repair.Genome, patches repair.Memory) *LearningHooks {
	return &LearningHooks{
		Genome:          genome,
		Patches:         patches,
		repairsByExec:   make(map[string]int),
		patchHitsByExec: make(map[string]int),
	}
}

// OnRetrySuccess records that a retry on `gate` after a `failure`
// (raw text — the hook normalises) was fixed by `fix`. The signature
// is derived from the failure string so identical failure classes
// converge on the same recipe row. Also marks MarkSuccess so the
// genome can rank "actually works" recipes above "merely recorded"
// ones.
//
// executionID is used for the per-execution repair counter — it is
// fine to pass "" when the call site has no execution on context;
// the genome write still happens, only the per-execution counter is
// skipped.
func (h *LearningHooks) OnRetrySuccess(ctx context.Context, executionID, gate, failure string, fix map[string]any) {
	if h == nil {
		return
	}
	// Per-execution repair counter — best-effort; nil-safe.
	if executionID != "" {
		h.mu.Lock()
		h.repairsByExec[executionID]++
		h.mu.Unlock()
	}
	if h.Genome == nil || failure == "" {
		return
	}
	sig := repair.FailureSignature(failure)
	category := gate
	if category == "" {
		category = "unknown"
	}
	if _, err := h.Genome.Record(ctx, sig, category, fix); err != nil {
		// Swallow: learning is best-effort.
		return
	}
	_ = h.Genome.MarkSuccess(ctx, sig)
}

// OnPatchApplied records a patch shape against the intent that
// produced it. The intent string is hashed into a stable
// IntentSignature so callers can pass a free-form intent description
// (gate + story + summary) without worrying about format drift.
//
// The newly-recorded entry is immediately marked applied(success=true)
// so the dashboard's "patch reuse success rate" stays accurate even
// for first-time applications.
func (h *LearningHooks) OnPatchApplied(ctx context.Context, executionID, intent string, patch map[string]any, affectedPaths []string, costUSD decimal.Decimal) {
	if h == nil || h.Patches == nil || intent == "" {
		return
	}
	if executionID != "" {
		h.mu.Lock()
		h.patchHitsByExec[executionID]++
		h.mu.Unlock()
	}
	sig := repair.IntentSignature(intent, "")
	entry, err := h.Patches.Record(ctx, sig, patch, affectedPaths, costUSD)
	if err != nil {
		return
	}
	_ = h.Patches.MarkApplied(ctx, entry.ID, true)
}

// LookupRecipe is the read side of the genome. Returns ok=false (and
// nil error) when no recipe matches. nil-safe: a nil hook or nil
// store returns ok=false.
func (h *LearningHooks) LookupRecipe(ctx context.Context, failure string) (repair.Recipe, bool, error) {
	if h == nil || h.Genome == nil || failure == "" {
		return repair.Recipe{}, false, nil
	}
	sig := repair.FailureSignature(failure)
	return h.Genome.Lookup(ctx, sig)
}

// LookupPatch is the read side of the patch memory. Returns the
// PatchEntry list (possibly empty) for the intent. nil-safe.
func (h *LearningHooks) LookupPatch(ctx context.Context, intent string) ([]repair.PatchEntry, error) {
	if h == nil || h.Patches == nil || intent == "" {
		return nil, nil
	}
	sig := repair.IntentSignature(intent, "")
	return h.Patches.Find(ctx, sig)
}

// RepairsFor returns the per-execution repair counter and removes the
// entry. The engineSettler adapter calls this at terminal settle to
// decide whether to mark the blueprint run as Repaired=true. Safe to
// call with executionID == "" (returns 0).
func (h *LearningHooks) RepairsFor(executionID string) int {
	if h == nil || executionID == "" {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	n := h.repairsByExec[executionID]
	delete(h.repairsByExec, executionID)
	return n
}

// PatchHitsFor returns and clears the per-execution patch-apply
// counter. Kept symmetrical with RepairsFor for ops-side telemetry;
// not currently consumed by the settler.
func (h *LearningHooks) PatchHitsFor(executionID string) int {
	if h == nil || executionID == "" {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	n := h.patchHitsByExec[executionID]
	delete(h.patchHitsByExec, executionID)
	return n
}

// ---- helpers ---------------------------------------------------------

// IntentForGateStory builds a stable intent string for a (gate,
// storyID, title) tuple. Used by the engine call sites so the hook
// callers and any future readers agree on the intent shape.
func IntentForGateStory(gate, storyID, title string) string {
	return "gate=" + gate + "|story=" + storyID + "|title=" + title
}

// _ keeps the uuid import live for future ID-typed methods on the
// learning surface.
var _ = uuid.Nil
