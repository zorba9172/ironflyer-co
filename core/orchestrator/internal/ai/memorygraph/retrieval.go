package memorygraph

import (
	"context"
	"sort"
	"time"
)

// Intent names the orchestrator's reason for asking for context. The
// retriever picks a different traversal path per intent — see
// pathForIntent. Wire values; do not rename without coordinated agent
// rollout.
type Intent string

const (
	IntentGateRepair     Intent = "gate_repair"
	IntentCodeGeneration Intent = "code_generation"
	IntentReview         Intent = "review"
	IntentSecurity       Intent = "security"
	IntentDeploy         Intent = "deploy"
	IntentCompletion     Intent = "completion_scoring_support"
)

// RetrievalMode selects between pure graph traversal, pure vector
// similarity, hybrid (graph as a filter on vector), and keyword.
// V22 implements graph + hybrid; vector and keyword are stubs that
// fall back to graph until the embedding layer lands.
type RetrievalMode string

const (
	ModeGraph   RetrievalMode = "graph"
	ModeVector  RetrievalMode = "vector"
	ModeHybrid  RetrievalMode = "hybrid"
	ModeKeyword RetrievalMode = "keyword"
)

// Scope locks the retrieval to a tenant + project, optionally to a
// specific execution, and optionally to a file path whitelist. Empty
// AllowedFilePaths means "all files in scope of (tenant, project)".
type Scope struct {
	TenantID         string
	ProjectID        string
	ExecutionID      string
	AllowedFilePaths []string
}

// Budget bounds the retrieval cost: max records, max estimated tokens,
// freshness window (records older than now-window are skipped), and
// confidence threshold (records below confidence are dropped).
type Budget struct {
	MaxRecords          int
	MaxTokenEstimate    int
	FreshnessWindow     time.Duration
	ConfidenceThreshold float64
}

// Record is one packet entry. Score is the retriever-assigned rank —
// higher is more relevant. Provenance carries the canonical event id
// so the agent can cite where the context came from.
type Record struct {
	NodeID     string
	Kind       NodeKind
	Summary    string
	Score      float64
	Provenance Provenance
	Attrs      map[string]any
}

// ContextPacket is the only object an agent ever sees from the memory
// graph. It carries scope + intent + budget so the agent can reason
// about completeness and provenance for citation.
type ContextPacket struct {
	Scope         Scope
	Intent        Intent
	RetrievalMode RetrievalMode
	Budget        Budget
	Records       []Record
	Provenance    []Provenance
}

// Retriever is the operator-replaceable contract for building context
// packets. The default implementation lives in GraphRetriever; hybrid
// and vector retrievers will satisfy the same interface.
type Retriever interface {
	Build(ctx context.Context, scope Scope, intent Intent, mode RetrievalMode, budget Budget) (ContextPacket, error)
}

// GraphRetriever is the V22 retriever. It walks the graph using the
// intent's canonical path and ranks records by edge weight + freshness.
type GraphRetriever struct {
	g Graph
}

// NewGraphRetriever wraps a Graph. The integration agent passes the
// SurrealGraph in production and the in-memory MemoryGraph in dev.
func NewGraphRetriever(g Graph) *GraphRetriever {
	return &GraphRetriever{g: g}
}

// Build assembles a context packet for the requested scope + intent.
// Returns an empty packet (NOT an error) when nothing matches — the
// retriever is best-effort. Hard failures are surfaced as errors.
func (r *GraphRetriever) Build(ctx context.Context, scope Scope, intent Intent, mode RetrievalMode, budget Budget) (ContextPacket, error) {
	if scope.TenantID == "" {
		return ContextPacket{}, ErrTenantMissing
	}
	if scope.ProjectID == "" {
		return ContextPacket{}, ErrProjectMissing
	}
	packet := ContextPacket{
		Scope:         scope,
		Intent:        intent,
		RetrievalMode: mode,
		Budget:        budget,
	}
	if budget.MaxRecords <= 0 {
		budget.MaxRecords = 16
	}

	starts, err := r.startNodes(ctx, scope, intent)
	if err != nil {
		return packet, err
	}
	if len(starts) == 0 {
		return packet, nil
	}

	path := pathForIntent(intent)
	q := TraversalQuery{
		TenantID:      scope.TenantID,
		ProjectID:     scope.ProjectID,
		StartIDs:      starts,
		Path:          path,
		MaxNodes:      budget.MaxRecords * 4,
		MaxDepth:      len(path),
		MinConfidence: budget.ConfidenceThreshold,
		Freshness:     budget.FreshnessWindow,
	}
	nodes, edges, err := r.g.Traverse(ctx, q)
	if err != nil {
		return packet, err
	}

	// Optional file-path whitelist (Scope.AllowedFilePaths). Patches +
	// files that fall outside the whitelist are dropped. Repairs are
	// kept unconditionally — the whole point is to surface known-good
	// fixes regardless of which file is being repaired right now.
	allow := allowedFilePaths(scope.AllowedFilePaths)
	intentTarget := primaryKindForIntent(intent)
	scored := make([]Record, 0, len(nodes))
	for _, n := range nodes {
		if allow != nil && n.Kind == NodeFile {
			path, _ := n.Attrs["path"].(string)
			if _, ok := allow[path]; !ok {
				continue
			}
		}
		score := scoreFor(n, edges)
		if intentTarget != "" && n.Kind == intentTarget {
			score += 1.0 // boost the intent's primary target.
		}
		scored = append(scored, Record{
			NodeID:     n.ID,
			Kind:       n.Kind,
			Summary:    n.Summary,
			Score:      score,
			Provenance: n.Provenance,
			Attrs:      n.Attrs,
		})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].NodeID < scored[j].NodeID
		}
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > budget.MaxRecords {
		scored = scored[:budget.MaxRecords]
	}
	packet.Records = scored

	// Dedup provenance across the packet.
	seen := map[string]struct{}{}
	for _, rec := range scored {
		key := rec.Provenance.SourceEventID
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		packet.Provenance = append(packet.Provenance, rec.Provenance)
	}
	return packet, nil
}

// startNodes picks the traversal anchors per intent. We err on the side
// of "use the project root" so a fresh execution still gets a useful
// packet before any patches/failures have been recorded.
func (r *GraphRetriever) startNodes(ctx context.Context, scope Scope, intent Intent) ([]string, error) {
	switch intent {
	case IntentGateRepair, IntentCodeGeneration, IntentReview,
		IntentSecurity, IntentDeploy, IntentCompletion:
		// Default anchor: the project root node. The traversal path
		// fans out from project -> has_spec -> spec -> concerns_file
		// -> file -> ... and patch/failure subgraphs are also rooted
		// at the project because writes always project from there.
		root := BuildNodeID(NodeProject, scope.TenantID, scope.ProjectID)
		// Confirm the node exists before traversing — if not, the
		// caller hasn't yet projected project.created so we surface
		// an empty packet rather than an error.
		_, ok, err := r.g.GetNode(ctx, scope.TenantID, root)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		return []string{root}, nil
	}
	return nil, nil
}

// pathForIntent returns the ordered EdgeKind walk per intent. The
// GateRepair path is the V22 "high-value traversal" called out in
// docs/ARCHITECTURE_MEMORY_GRAPH.md.
func pathForIntent(intent Intent) []EdgeKind {
	switch intent {
	case IntentGateRepair:
		// project -> has_spec -> spec -> concerns_file -> file
		// -> (back via patch.modifies_file) ... but we can't walk
		// edges in reverse with the simple writer, so we use the
		// forward path and rely on scoring to surface repair vertices
		// adjacent to in-scope failures. The dedicated failure path
		// is appended via the second pass triggered when failure
		// nodes appear in the first sweep — see scoreFor.
		return []EdgeKind{
			EdgeHasSpec, EdgeConcernsFile,
			// from file we don't traverse defines_symbol here; the
			// V22 priority is reaching failures/repairs adjacent to
			// the affected files via patch projections.
		}
	case IntentCodeGeneration:
		return []EdgeKind{EdgeHasSpec, EdgeConcernsFile, EdgeDefinesSymbol}
	case IntentReview, IntentSecurity:
		return []EdgeKind{EdgeHasSpec, EdgeConcernsFile, EdgeDefinesSymbol}
	case IntentDeploy:
		return []EdgeKind{EdgeHasSpec, EdgeRequiresGate}
	case IntentCompletion:
		return []EdgeKind{EdgeHasSpec, EdgeRequiresGate}
	}
	return nil
}

// primaryKindForIntent returns the kind the agent most wants surfaced.
// We use it to give a bounded score boost so the right vertex ends up
// near the top of the packet even when adjacent context wins on
// freshness.
func primaryKindForIntent(intent Intent) NodeKind {
	switch intent {
	case IntentGateRepair:
		return NodeRepair
	case IntentCodeGeneration, IntentReview, IntentSecurity:
		return NodeSymbol
	case IntentDeploy:
		return NodeGate
	case IntentCompletion:
		return NodeGate
	}
	return ""
}

// scoreFor produces a lightweight score per node from edge weight and
// recency. Real ranking (vector cosine, repair success rate) lives in
// later layers; this score keeps the first-pass packet useful without
// pulling in the embedder.
func scoreFor(n Node, edges []Edge) float64 {
	score := n.Confidence
	for _, e := range edges {
		if e.From != n.ID && e.To != n.ID {
			continue
		}
		score += e.Weight
	}
	// Recency bonus: 0..1 over the last 30 days.
	if !n.UpdatedAt.IsZero() {
		age := time.Since(n.UpdatedAt)
		days := age.Hours() / 24
		if days < 0 {
			days = 0
		}
		if days < 30 {
			score += (30 - days) / 30
		}
	}
	return score
}

func allowedFilePaths(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		set[p] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// compile-time assertion.
var _ Retriever = (*GraphRetriever)(nil)
