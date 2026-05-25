package memorygraph

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryGraph is the in-process fallback Graph implementation used for
// dev / cold-boot / tests-free smoke runs. NOT a second source of truth
// — restart loses everything. Production runs against SurrealGraph.
type MemoryGraph struct {
	mu    sync.RWMutex
	nodes map[string]Node       // by Node.ID
	out   map[string][]string   // adjacency: from-id -> edge-ids
	in    map[string][]string   // adjacency: to-id   -> edge-ids
	edges map[string]Edge       // by Edge.ID
	byTen map[string]struct{}   // "<tenantId>|<nodeId>" sentinel for tenant isolation
}

// NewMemoryGraph returns a fresh in-process Graph. No persistence; no
// configuration. Safe for concurrent use.
func NewMemoryGraph() *MemoryGraph {
	return &MemoryGraph{
		nodes: map[string]Node{},
		out:   map[string][]string{},
		in:    map[string][]string{},
		edges: map[string]Edge{},
		byTen: map[string]struct{}{},
	}
}

// Bootstrap is a no-op for the in-memory backend.
func (g *MemoryGraph) Bootstrap(_ context.Context) error { return nil }

// UpsertNode inserts or merges. Merge semantics: existing summary,
// attrs, and confidence are overwritten by the incoming node; CreatedAt
// is preserved across writes so the first-seen timestamp survives.
func (g *MemoryGraph) UpsertNode(_ context.Context, n Node) (Node, error) {
	if err := n.validate(); err != nil {
		return Node{}, err
	}
	now := time.Now().UTC()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	g.mu.Lock()
	defer g.mu.Unlock()
	if existing, ok := g.nodes[n.ID]; ok {
		n.CreatedAt = existing.CreatedAt
	}
	g.nodes[n.ID] = n
	g.byTen[n.TenantID+"|"+n.ID] = struct{}{}
	return n, nil
}

// UpsertEdge inserts or merges. Adjacency is kept de-duplicated so a
// node's neighbor list doesn't grow on repeated writes.
func (g *MemoryGraph) UpsertEdge(_ context.Context, e Edge) (Edge, error) {
	if err := e.validate(); err != nil {
		return Edge{}, err
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	// Edges only get written if both endpoints exist; the graph is
	// derived, so a dangling edge means the projection raced ahead.
	if _, ok := g.nodes[e.From]; !ok {
		return Edge{}, ErrInvalidEdge
	}
	if _, ok := g.nodes[e.To]; !ok {
		return Edge{}, ErrInvalidEdge
	}
	if _, exists := g.edges[e.ID]; !exists {
		g.out[e.From] = append(g.out[e.From], e.ID)
		g.in[e.To] = append(g.in[e.To], e.ID)
	}
	g.edges[e.ID] = e
	return e, nil
}

// GetNode looks up a node by id; ErrNotFound semantics are encoded as
// (Node{}, false, nil) so callers don't have to import an error value
// just to detect a miss.
func (g *MemoryGraph) GetNode(_ context.Context, tenantID, id string) (Node, bool, error) {
	if tenantID == "" {
		return Node{}, false, ErrTenantMissing
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[id]
	if !ok || n.TenantID != tenantID {
		return Node{}, false, nil
	}
	return n, true, nil
}

// Neighbors walks one or more hops out from id. depth 1 returns direct
// neighbors only; higher depths BFS-walk while honoring edgeKinds.
func (g *MemoryGraph) Neighbors(_ context.Context, tenantID, id string, edgeKinds []EdgeKind, depth int) ([]Node, []Edge, error) {
	if tenantID == "" {
		return nil, nil, ErrTenantMissing
	}
	if depth <= 0 {
		depth = 1
	}
	allow := edgeKindSet(edgeKinds)
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.bfs(tenantID, []string{id}, allow, depth, 256)
}

// Traverse evaluates a Path-shaped walk. The IntentGateRepair planner
// passes a list like [EdgeConcernsFile, EdgeDefinesSymbol] to drill
// from spec down to symbol; or
// [EdgeCausedFailure, EdgeMatchesRepair, EdgeFixedByPatch] to climb
// from patch up to known-good repairs.
func (g *MemoryGraph) Traverse(_ context.Context, q TraversalQuery) ([]Node, []Edge, error) {
	if q.TenantID == "" {
		return nil, nil, ErrTenantMissing
	}
	maxNodes := q.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 256
	}
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Empty path → BFS bounded by MaxDepth (default 3).
	if len(q.Path) == 0 {
		depth := q.MaxDepth
		if depth <= 0 {
			depth = 3
		}
		return g.bfs(q.TenantID, q.StartIDs, edgeKindSet(nil), depth, maxNodes)
	}

	// Path-driven walk: at hop i we only follow Path[i].
	frontier := append([]string{}, q.StartIDs...)
	visited := map[string]Node{}
	collectedEdges := map[string]Edge{}
	for _, sid := range q.StartIDs {
		if n, ok := g.nodes[sid]; ok && n.TenantID == q.TenantID && passesFilters(n, q) {
			visited[n.ID] = n
		}
	}
	for hop, kind := range q.Path {
		_ = hop
		if len(visited) >= maxNodes {
			break
		}
		next := make([]string, 0, len(frontier))
		for _, fid := range frontier {
			for _, eid := range g.out[fid] {
				e, ok := g.edges[eid]
				if !ok || e.Kind != kind || e.TenantID != q.TenantID {
					continue
				}
				if q.MinConfidence > 0 && e.Confidence < q.MinConfidence {
					continue
				}
				tn, ok := g.nodes[e.To]
				if !ok || tn.TenantID != q.TenantID {
					continue
				}
				if q.ProjectID != "" && tn.ProjectID != "" && tn.ProjectID != q.ProjectID {
					continue
				}
				if !passesFilters(tn, q) {
					continue
				}
				collectedEdges[e.ID] = e
				if _, seen := visited[tn.ID]; !seen {
					visited[tn.ID] = tn
					next = append(next, tn.ID)
					if len(visited) >= maxNodes {
						break
					}
				}
			}
		}
		frontier = next
	}
	return mapToNodes(visited), mapToEdges(collectedEdges), nil
}

// DeleteProject removes every node/edge whose ProjectID matches.
// Tombstone semantics live in the persistent backend; the in-memory
// fallback hard-deletes since it has no audit role.
func (g *MemoryGraph) DeleteProject(_ context.Context, tenantID, projectID string) error {
	if tenantID == "" {
		return ErrTenantMissing
	}
	if projectID == "" {
		return ErrProjectMissing
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for id, n := range g.nodes {
		if n.TenantID != tenantID || n.ProjectID != projectID {
			continue
		}
		delete(g.nodes, id)
		delete(g.byTen, n.TenantID+"|"+id)
		for _, eid := range g.out[id] {
			if e, ok := g.edges[eid]; ok {
				delete(g.edges, eid)
				_ = e
			}
		}
		for _, eid := range g.in[id] {
			if e, ok := g.edges[eid]; ok {
				delete(g.edges, eid)
				_ = e
			}
		}
		delete(g.out, id)
		delete(g.in, id)
	}
	return nil
}

// bfs is the shared BFS helper used by Neighbors and Traverse's
// empty-path branch. Caller MUST hold the lock.
func (g *MemoryGraph) bfs(tenantID string, starts []string, allow map[EdgeKind]struct{}, depth, maxNodes int) ([]Node, []Edge, error) {
	visited := map[string]Node{}
	collectedEdges := map[string]Edge{}
	frontier := make([]string, 0, len(starts))
	for _, sid := range starts {
		if n, ok := g.nodes[sid]; ok && n.TenantID == tenantID {
			visited[sid] = n
			frontier = append(frontier, sid)
		}
	}
	for hop := 0; hop < depth && len(visited) < maxNodes; hop++ {
		next := make([]string, 0, len(frontier))
		for _, fid := range frontier {
			for _, eid := range g.out[fid] {
				e, ok := g.edges[eid]
				if !ok || e.TenantID != tenantID {
					continue
				}
				if len(allow) > 0 {
					if _, ok := allow[e.Kind]; !ok {
						continue
					}
				}
				tn, ok := g.nodes[e.To]
				if !ok || tn.TenantID != tenantID {
					continue
				}
				collectedEdges[e.ID] = e
				if _, seen := visited[tn.ID]; !seen {
					visited[tn.ID] = tn
					next = append(next, tn.ID)
					if len(visited) >= maxNodes {
						break
					}
				}
			}
		}
		frontier = next
	}
	return mapToNodes(visited), mapToEdges(collectedEdges), nil
}

func edgeKindSet(kinds []EdgeKind) map[EdgeKind]struct{} {
	if len(kinds) == 0 {
		return nil
	}
	set := make(map[EdgeKind]struct{}, len(kinds))
	for _, k := range kinds {
		set[k] = struct{}{}
	}
	return set
}

func passesFilters(n Node, q TraversalQuery) bool {
	if q.MinConfidence > 0 && n.Confidence > 0 && n.Confidence < q.MinConfidence {
		return false
	}
	if q.Freshness > 0 && !n.UpdatedAt.IsZero() &&
		time.Since(n.UpdatedAt) > q.Freshness {
		return false
	}
	return true
}

func mapToNodes(m map[string]Node) []Node {
	out := make([]Node, 0, len(m))
	for _, n := range m {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func mapToEdges(m map[string]Edge) []Edge {
	out := make([]Edge, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// compile-time assertion.
var _ Graph = (*MemoryGraph)(nil)
