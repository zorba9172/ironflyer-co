// preflight.go is the Reuse-First Preflight integration point for the
// Coder and Architect agents. Before either agent proposes a new file
// or a new public function, it must call PreflightSearch with a brief
// description of the planned symbol. The returned hits are the
// canonical menu of reuse candidates; the agent then emits a
// PreflightDecision committing to one of `reuse` / `extend` / `new`.
//
// The decision rides alongside the patch through the wowloop
// `reuse_check` gate (see business/wowloop/antibloat_reuse.go). A
// missing or `new`-without-justification decision is a high-severity
// gate finding — the playbook ranks it next to a layering violation.
//
// This file does NOT call providers itself. It hands the agent a
// structured slate of options and parses the agent's decision back.
// All embedding / similarity math lives in the atlas package.

package agents

import (
	"context"
	"fmt"
	"strings"

	"ironflyer/core/orchestrator/internal/ai/atlas"
	"ironflyer/core/orchestrator/internal/ai/embeddings"
)

// PreflightAction enumerates the three legal outcomes of a preflight
// decision. Anything else is rejected by the wowloop reuse_check gate.
type PreflightAction string

const (
	PreflightReuse  PreflightAction = "reuse"
	PreflightExtend PreflightAction = "extend"
	PreflightNew    PreflightAction = "new"
)

// PreflightDecision is what a coder / architect agent emits BEFORE the
// orchestrator hands the patch to patch.Engine.Propose. The Engine's
// gate plumbing reads the attached AtlasHits + Action to score the
// reuse_check verdict. Justification is required when Action == "new"
// — the agent must defend why the existing surface is insufficient.
type PreflightDecision struct {
	Action        PreflightAction `json:"action"`
	TargetPath    string          `json:"targetPath,omitempty"`
	Symbol        string          `json:"symbol,omitempty"`
	Justification string          `json:"justification,omitempty"`
	Query         string          `json:"query,omitempty"`
	AtlasHits     []atlas.Hit     `json:"atlasHits,omitempty"`
}

// Validate enforces the decision's invariants. Returns nil when the
// decision is well-formed; the wowloop gate calls this when an explicit
// PreflightDecision is attached so a malformed decision still fails
// the gate instead of slipping through as "preflight was performed".
func (d PreflightDecision) Validate() error {
	switch d.Action {
	case PreflightReuse, PreflightExtend:
		if strings.TrimSpace(d.TargetPath) == "" {
			return fmt.Errorf("preflight: action %q requires targetPath", d.Action)
		}
	case PreflightNew:
		if strings.TrimSpace(d.Justification) == "" {
			return fmt.Errorf("preflight: action %q requires justification", d.Action)
		}
	default:
		return fmt.Errorf("preflight: unknown action %q", d.Action)
	}
	return nil
}

// PreflightSearch runs the semantic + lexical search against the
// Atlas. When an embeddings.Embedder is wired the query is embedded
// once and attached to ctx so the Atlas can rank by cosine similarity;
// without one the Atlas falls back to lexical overlap on path / symbol
// / doc.
//
// Top-K is fixed to 5 per the playbook §8.4 contract; callers that
// want a different cut should call atlas.Store.Search directly.
func PreflightSearch(ctx context.Context, store atlas.Store, embed embeddings.Embedder, query string) ([]atlas.Hit, error) {
	if store == nil {
		return nil, nil
	}
	if embed != nil && strings.TrimSpace(query) != "" {
		if vec, err := embed.Embed(ctx, query); err == nil && len(vec) > 0 {
			ctx = atlas.WithQueryEmbedding(ctx, vec)
		}
	}
	return store.Search(ctx, query, 5)
}

// PreflightDecisionKey is the context key used to forward an emitted
// PreflightDecision from the agent loop to the wowloop reuse_check
// gate. Keeping it on the orchestrator's per-call context avoids
// threading it through every gate signature.
type PreflightDecisionKey struct{}

// WithPreflightDecision attaches d to ctx. The wowloop reuse_check
// gate reads it back via PreflightDecisionFromContext.
func WithPreflightDecision(ctx context.Context, d PreflightDecision) context.Context {
	return contextWithValue(ctx, PreflightDecisionKey{}, d)
}

// PreflightDecisionFromContext returns the decision attached by
// WithPreflightDecision, or ok=false.
func PreflightDecisionFromContext(ctx context.Context) (PreflightDecision, bool) {
	v, ok := ctx.Value(PreflightDecisionKey{}).(PreflightDecision)
	return v, ok
}

// contextWithValue is the tiny shim that lets us avoid pulling
// `context` types into this file's import block twice (once for the
// key, once for the helper). The compiler still inlines it.
func contextWithValue(ctx context.Context, key, value any) context.Context {
	return context.WithValue(ctx, key, value)
}
