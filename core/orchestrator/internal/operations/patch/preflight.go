// preflight.go is the Anti-Bloat Engine's customer-facing hook from
// the patch lifecycle. Before patch.Engine.Propose ratifies a patch
// that creates a new file, we ask the Capability Atlas whether the
// repo already exposes a reusable surface for the planned work. A
// high-confidence hit blocks the patch with a SeverityWarning so the
// operator gets a "candidate reuse exists" prompt instead of yet
// another near-duplicate file landing in the tree.
//
// The decision is recorded on the Patch via Preflight, surfaced as an
// Issue on the patch's Issues, and projected onto every audit-chain
// record via OnProposed. The downstream `reuse_check` gate
// (gates_antibloat.go) reads the same Preflight via the engine's
// PreflightFor(projectID) accessor so the gate verdict stays in
// lockstep with what propose-time decided.
//
// Why the hook lives in patch and not in the agent loop: every code
// path that produces a patch (Coder, Architect, mobile-coder, repair
// recipes, the recovery sub-loop) eventually calls patch.Engine.Propose
// — placing the check there gives Anti-Bloat one chokepoint instead of
// six per-agent integrations to keep in sync.

package patch

import (
	"context"
	"fmt"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/atlas"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/embeddings"
)

// defaultPreflightThreshold is the cosine-similarity bar above which a
// hit is treated as "you almost certainly have this already". 0.85
// matches the agents.yaml coder prompt + playbook §8.4 contract.
const defaultPreflightThreshold = 0.85

// WithAtlas wires the Capability Atlas + Embedder so Propose can run
// the Reuse-First preflight on every OpCreate. Pass a nil store to
// disable (the default — the engine remains a pure structural validator
// until the host wires Atlas at boot). Threshold ≤ 0 keeps the
// package default (0.85).
func (e *Engine) WithAtlas(store atlas.Store, embed embeddings.Embedder, threshold float32) *Engine {
	e.atlasStore = store
	e.atlasEmbed = embed
	if threshold > 0 {
		e.preflightThreshold = threshold
	} else {
		e.preflightThreshold = defaultPreflightThreshold
	}
	return e
}

// PreflightFor returns the most recent PreflightDecision the engine
// computed for projectID, or ok=false when no patch with a create-op
// has been proposed yet. The finisher engine consumes this in the gate
// loop so the `reuse_check` gate reads the same decision the propose-
// time check produced.
func (e *Engine) PreflightFor(projectID string) (agents.PreflightDecision, bool) {
	if e == nil {
		return agents.PreflightDecision{}, false
	}
	e.preflightMu.RLock()
	defer e.preflightMu.RUnlock()
	d, ok := e.preflightByProject[projectID]
	if !ok {
		return agents.PreflightDecision{}, false
	}
	return d, true
}

// preflightCheck runs the Reuse-First Preflight for every OpCreate in
// the patch whose target path doesn't already exist in the project.
// It returns the decision (always non-nil on a successful call so the
// audit chain can record "preflight ran") and any Issues that should
// ride on the patch.
//
// The decision is:
//   - PreflightReuse  — at least one hit scored ≥ threshold;
//                       targetPath is the highest-scoring path.
//   - PreflightNew    — the caller's intent didn't surface a candidate;
//                       justification is generated from the patch title.
//
// PreflightExtend is reserved for the agent-driven path where the LLM
// chose to extend a specific symbol — the propose-time mechanical
// check can't tell extension from new file creation, so it never emits
// it. The agent loop still can via agents.WithPreflightDecision.
func (e *Engine) preflightCheck(ctx context.Context, p *Patch) (*agents.PreflightDecision, []domain.Issue) {
	if e == nil || e.atlasStore == nil || p == nil {
		return nil, nil
	}
	// Collect the create-ops whose path doesn't exist yet in the project
	// tree. OpUpdate / OpReplace / OpInsertAfter / OpSymbol / OpDelete
	// edit existing surface; only OpCreate is the "are you about to
	// duplicate something?" trigger.
	proj, err := e.projects.Get(p.ProjectID)
	if err != nil {
		return nil, nil
	}
	var creates []FileChange
	for _, c := range p.Changes {
		if c.Op != OpCreate {
			continue
		}
		if _, exists := lookupFileBody(&proj, c.Path); exists {
			continue
		}
		creates = append(creates, c)
	}
	if len(creates) == 0 {
		return nil, nil
	}

	threshold := e.preflightThreshold
	if threshold <= 0 {
		threshold = defaultPreflightThreshold
	}

	// Build a search intent. Title + the new file's path + the first
	// ~512 bytes of the new content gives the embedder enough signal to
	// pick up "new HTTP timeout helper" or "extracted form schema"
	// without ballooning the embedding budget.
	intent := preflightIntent(p, creates)

	hits, err := agents.PreflightSearch(ctx, e.atlasStore, e.atlasEmbed, intent)
	if err != nil {
		return nil, nil
	}

	decision := agents.PreflightDecision{
		Query:     intent,
		AtlasHits: hits,
	}
	var issues []domain.Issue
	if len(hits) > 0 && hits[0].Score >= threshold {
		top := hits[0]
		decision.Action = agents.PreflightReuse
		decision.TargetPath = top.Capability.Path
		decision.Symbol = top.Capability.Symbol
		decision.Justification = fmt.Sprintf(
			"atlas suggests reusing %s:%s (score %.2f ≥ %.2f)",
			top.Capability.Path, top.Capability.Symbol, top.Score, threshold,
		)
		issues = append(issues, domain.Issue{
			Gate:     domain.GateReuseCheck,
			Severity: domain.SeverityWarning,
			Message: fmt.Sprintf("candidate reuse exists: %s:%s (score %.2f)",
				top.Capability.Path, top.Capability.Symbol, top.Score),
			Hint: "confirm `new` with justification, or rewrite the patch to reuse / extend the existing surface",
			Path: creates[0].Path,
		})
	} else {
		decision.Action = agents.PreflightNew
		justification := strings.TrimSpace(p.Title)
		if justification == "" {
			justification = "no atlas hit above threshold"
		}
		decision.Justification = justification
	}
	return &decision, issues
}

// preflightIntent builds the natural-language query the Atlas embeds
// for the reuse-search. Kept short on purpose — the Atlas's embedded
// capabilities are 1-2 sentence symbol-doc lines, so a focused intent
// beats a kitchen-sink one.
func preflightIntent(p *Patch, creates []FileChange) string {
	var b strings.Builder
	if t := strings.TrimSpace(p.Title); t != "" {
		b.WriteString(t)
	}
	if s := strings.TrimSpace(p.Summary); s != "" {
		if b.Len() > 0 {
			b.WriteString(" — ")
		}
		b.WriteString(s)
	}
	if b.Len() == 0 {
		// Last-resort: synthesise from the first create's path so the
		// Atlas still gets a signal.
		b.WriteString("create new file: ")
		b.WriteString(creates[0].Path)
	} else {
		b.WriteString(" — new file ")
		b.WriteString(creates[0].Path)
	}
	const maxContent = 512
	body := strings.TrimSpace(creates[0].Content)
	if body != "" {
		if len(body) > maxContent {
			body = body[:maxContent]
		}
		b.WriteString(" — ")
		b.WriteString(body)
	}
	return b.String()
}

// rememberPreflight stores the decision for the project so the
// finisher's gate loop can read it back via PreflightFor when the
// reuse_check gate fires.
func (e *Engine) rememberPreflight(projectID string, d agents.PreflightDecision) {
	if e == nil || projectID == "" {
		return
	}
	e.preflightMu.Lock()
	if e.preflightByProject == nil {
		e.preflightByProject = make(map[string]agents.PreflightDecision, 8)
	}
	e.preflightByProject[projectID] = d
	e.preflightMu.Unlock()
}

// Engine's preflight state lives on the struct itself (engine.go);
// the hook in this file is the only writer.
