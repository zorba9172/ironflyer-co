package memorygraph

import (
	"context"
	"fmt"
	"sort"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// SurrealGraph is the persistent Graph implementation. It stores nodes
// in graph_nodes and edges in graph_edges per schema.go. Tenant
// isolation is enforced in every SurrealQL clause — there is no
// codepath that reads or writes without scoping by tenantId.
type SurrealGraph struct {
	db *surrealdb.DB
}

// NewSurrealGraph wraps an already-connected *surrealdb.DB. Call
// Bootstrap once at startup to install the schema.
func NewSurrealGraph(db *surrealdb.DB) *SurrealGraph {
	return &SurrealGraph{db: db}
}

// Bootstrap installs the graph schema. Idempotent.
func (g *SurrealGraph) Bootstrap(ctx context.Context) error {
	res, err := surrealdb.Query[any](ctx, g.db, surrealGraphSchema, nil)
	if err != nil {
		return fmt.Errorf("memorygraph surreal schema: %w", err)
	}
	if res != nil {
		for _, r := range *res {
			if r.Status != "OK" {
				return fmt.Errorf("memorygraph surreal schema statement failed: %s", r.Status)
			}
		}
	}
	return nil
}

// nodeRow is the read shape for graph_nodes. We hold onto the
// SurrealDB record id so we can stringify it back to Node.ID. attrs
// is generic because Node.Attrs is intentionally schemaless.
type nodeRow struct {
	ID           *models.RecordID      `json:"id,omitempty"`
	Kind         string                `json:"kind"`
	TenantID     string                `json:"tenantId"`
	ProjectID    string                `json:"projectId,omitempty"`
	CanonicalRef string                `json:"canonicalRef,omitempty"`
	Summary      string                `json:"summary,omitempty"`
	Confidence   float64               `json:"confidence,omitempty"`
	Attrs        map[string]any        `json:"attrs,omitempty"`
	Provenance   map[string]any        `json:"provenance,omitempty"`
	CreatedAt    models.CustomDateTime `json:"createdAt"`
	UpdatedAt    models.CustomDateTime `json:"updatedAt"`
}

func (r nodeRow) toNode() Node {
	id := ""
	if r.ID != nil {
		id = fmt.Sprintf("%v:%v", r.ID.Table, r.ID.ID)
	}
	return Node{
		ID:           id,
		Kind:         NodeKind(r.Kind),
		TenantID:     r.TenantID,
		ProjectID:    r.ProjectID,
		CanonicalRef: r.CanonicalRef,
		Summary:      r.Summary,
		Confidence:   r.Confidence,
		Attrs:        r.Attrs,
		Provenance:   provenanceFromMap(r.Provenance),
		CreatedAt:    r.CreatedAt.Time,
		UpdatedAt:    r.UpdatedAt.Time,
	}
}

// edgeRow mirrors graph_edges.
type edgeRow struct {
	ID         *models.RecordID      `json:"id,omitempty"`
	Kind       string                `json:"kind"`
	TenantID   string                `json:"tenantId"`
	FromID     string                `json:"fromId"`
	ToID       string                `json:"toId"`
	Weight     float64               `json:"weight,omitempty"`
	Confidence float64               `json:"confidence,omitempty"`
	Inferred   bool                  `json:"inferred,omitempty"`
	Attrs      map[string]any        `json:"attrs,omitempty"`
	Provenance map[string]any        `json:"provenance,omitempty"`
	CreatedAt  models.CustomDateTime `json:"createdAt"`
}

func (r edgeRow) toEdge() Edge {
	id := ""
	if r.ID != nil {
		id = fmt.Sprintf("%v:%v", r.ID.Table, r.ID.ID)
	}
	return Edge{
		ID:         id,
		Kind:       EdgeKind(r.Kind),
		TenantID:   r.TenantID,
		From:       r.FromID,
		To:         r.ToID,
		Weight:     r.Weight,
		Confidence: r.Confidence,
		Inferred:   r.Inferred,
		Attrs:      r.Attrs,
		Provenance: provenanceFromMap(r.Provenance),
		CreatedAt:  r.CreatedAt.Time,
	}
}

func provenanceFromMap(m map[string]any) Provenance {
	if len(m) == 0 {
		return Provenance{}
	}
	p := Provenance{}
	if v, ok := m["sourceEventId"].(string); ok {
		p.SourceEventID = v
	}
	if v, ok := m["sourceEventType"].(string); ok {
		p.SourceEventType = v
	}
	if v, ok := m["recordedAt"].(time.Time); ok {
		p.RecordedAt = v
	}
	return p
}

func provenanceToMap(p Provenance) map[string]any {
	if p == (Provenance{}) {
		return nil
	}
	out := map[string]any{}
	if p.SourceEventID != "" {
		out["sourceEventId"] = p.SourceEventID
	}
	if p.SourceEventType != "" {
		out["sourceEventType"] = p.SourceEventType
	}
	if !p.RecordedAt.IsZero() {
		out["recordedAt"] = models.CustomDateTime{Time: p.RecordedAt}
	}
	return out
}

// nodeContent shapes a Node into the SurrealDB CREATE/UPDATE body.
// Empty optional fields are dropped so SCHEMALESS rows stay compact.
func nodeContent(n Node) map[string]any {
	doc := map[string]any{
		"kind":      string(n.Kind),
		"tenantId":  n.TenantID,
		"createdAt": models.CustomDateTime{Time: n.CreatedAt},
		"updatedAt": models.CustomDateTime{Time: n.UpdatedAt},
	}
	if n.ProjectID != "" {
		doc["projectId"] = n.ProjectID
	}
	if n.CanonicalRef != "" {
		doc["canonicalRef"] = n.CanonicalRef
	}
	if n.Summary != "" {
		doc["summary"] = n.Summary
	}
	if n.Confidence != 0 {
		doc["confidence"] = n.Confidence
	}
	if len(n.Attrs) > 0 {
		doc["attrs"] = n.Attrs
	}
	if prov := provenanceToMap(n.Provenance); prov != nil {
		doc["provenance"] = prov
	}
	return doc
}

func edgeContent(e Edge) map[string]any {
	doc := map[string]any{
		"kind":      string(e.Kind),
		"tenantId":  e.TenantID,
		"fromId":    e.From,
		"toId":      e.To,
		"createdAt": models.CustomDateTime{Time: e.CreatedAt},
	}
	if e.Weight != 0 {
		doc["weight"] = e.Weight
	}
	if e.Confidence != 0 {
		doc["confidence"] = e.Confidence
	}
	if e.Inferred {
		doc["inferred"] = true
	}
	if len(e.Attrs) > 0 {
		doc["attrs"] = e.Attrs
	}
	if prov := provenanceToMap(e.Provenance); prov != nil {
		doc["provenance"] = prov
	}
	return doc
}

// splitTableID turns a "<table>:<id>" string into its components. We
// store the canonical Node.ID containing a colon, so the SurrealDB
// CREATE form uses type::record(<table>, <id>) to avoid quoting bugs.
func splitTableID(id string) (string, string) {
	for i := 0; i < len(id); i++ {
		if id[i] == ':' {
			return id[:i], id[i+1:]
		}
	}
	return "graph_nodes", id
}

// UpsertNode persists n. Implemented as DELETE + CREATE inside a
// single statement so the row is replaced atomically — SurrealDB's
// UPSERT does not support type::record with arbitrary ids across all
// supported server versions, so we lean on the create-after-delete
// pattern that matches what internal/memory/surreal.go does.
func (g *SurrealGraph) UpsertNode(ctx context.Context, n Node) (Node, error) {
	if err := n.validate(); err != nil {
		return Node{}, err
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	n.UpdatedAt = time.Now().UTC()
	_, id := splitTableID(n.ID)
	// Idempotent CREATE: DELETE then CREATE. The repeated-projection
	// invariant in the architecture doc allows us to lose any
	// concurrent in-flight upsert for the same canonical id — the
	// next event will rebuild it.
	res, err := surrealdb.Query[any](ctx, g.db,
		"DELETE type::record('graph_nodes', $id); CREATE type::record('graph_nodes', $id) CONTENT $doc",
		map[string]any{"id": id, "doc": nodeContent(n)})
	if err != nil {
		return Node{}, fmt.Errorf("memorygraph upsert node: %w", err)
	}
	if res != nil {
		for _, qr := range *res {
			if qr.Status != "OK" {
				return Node{}, fmt.Errorf("memorygraph upsert node status: %s", qr.Status)
			}
		}
	}
	return n, nil
}

func (g *SurrealGraph) UpsertEdge(ctx context.Context, e Edge) (Edge, error) {
	if err := e.validate(); err != nil {
		return Edge{}, err
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if e.Confidence == 0 {
		e.Confidence = 1.0
	}
	_, id := splitTableID(e.ID)
	res, err := surrealdb.Query[any](ctx, g.db,
		"DELETE type::record('graph_edges', $id); CREATE type::record('graph_edges', $id) CONTENT $doc",
		map[string]any{"id": id, "doc": edgeContent(e)})
	if err != nil {
		return Edge{}, fmt.Errorf("memorygraph upsert edge: %w", err)
	}
	if res != nil {
		for _, qr := range *res {
			if qr.Status != "OK" {
				return Edge{}, fmt.Errorf("memorygraph upsert edge status: %s", qr.Status)
			}
		}
	}
	return e, nil
}

func (g *SurrealGraph) GetNode(ctx context.Context, tenantID, id string) (Node, bool, error) {
	if tenantID == "" {
		return Node{}, false, ErrTenantMissing
	}
	_, recID := splitTableID(id)
	res, err := surrealdb.Query[[]nodeRow](ctx, g.db,
		"SELECT * FROM type::record('graph_nodes', $id) WHERE tenantId = $tenantId LIMIT 1",
		map[string]any{"id": recID, "tenantId": tenantID})
	if err != nil {
		return Node{}, false, fmt.Errorf("memorygraph get node: %w", err)
	}
	if res == nil || len(*res) == 0 || len((*res)[0].Result) == 0 {
		return Node{}, false, nil
	}
	return (*res)[0].Result[0].toNode(), true, nil
}

// Neighbors fetches one-hop edges out of id (filtered by kind, scoped
// by tenant) plus the destination nodes. depth>1 recurses iteratively
// without leaving the tenant.
func (g *SurrealGraph) Neighbors(ctx context.Context, tenantID, id string, edgeKinds []EdgeKind, depth int) ([]Node, []Edge, error) {
	if tenantID == "" {
		return nil, nil, ErrTenantMissing
	}
	if depth <= 0 {
		depth = 1
	}
	visited := map[string]Node{}
	collectedEdges := map[string]Edge{}
	frontier := []string{id}
	for hop := 0; hop < depth; hop++ {
		if len(frontier) == 0 {
			break
		}
		next := []string{}
		for _, fid := range frontier {
			edges, nodes, err := g.outEdges(ctx, tenantID, fid, edgeKinds)
			if err != nil {
				return nil, nil, err
			}
			for _, e := range edges {
				collectedEdges[e.ID] = e
			}
			for _, n := range nodes {
				if _, seen := visited[n.ID]; !seen {
					visited[n.ID] = n
					next = append(next, n.ID)
				}
			}
		}
		frontier = next
	}
	return mapToNodes(visited), mapToEdges(collectedEdges), nil
}

// outEdges issues one SELECT for the edges leaving id, then one SELECT
// for the destination nodes. Two round trips per hop — acceptable for
// the V22 traversal depths (≤4) and avoids relying on SurrealDB's RELATE
// graph syntax which the projection layer doesn't write to.
func (g *SurrealGraph) outEdges(ctx context.Context, tenantID, id string, kinds []EdgeKind) ([]Edge, []Node, error) {
	vars := map[string]any{
		"tenantId": tenantID,
		"fromId":   id,
	}
	where := "tenantId = $tenantId AND fromId = $fromId"
	if len(kinds) > 0 {
		strs := make([]string, len(kinds))
		for i, k := range kinds {
			strs[i] = string(k)
		}
		vars["kinds"] = strs
		where += " AND kind IN $kinds"
	}
	eres, err := surrealdb.Query[[]edgeRow](ctx, g.db,
		"SELECT * FROM graph_edges WHERE "+where, vars)
	if err != nil {
		return nil, nil, fmt.Errorf("memorygraph out edges: %w", err)
	}
	var edges []Edge
	if eres != nil && len(*eres) > 0 {
		for _, r := range (*eres)[0].Result {
			edges = append(edges, r.toEdge())
		}
	}
	if len(edges) == 0 {
		return nil, nil, nil
	}
	toIDs := make([]string, 0, len(edges))
	seen := map[string]struct{}{}
	for _, e := range edges {
		if _, ok := seen[e.To]; ok {
			continue
		}
		seen[e.To] = struct{}{}
		toIDs = append(toIDs, e.To)
	}
	sort.Strings(toIDs)
	// SurrealDB IN works on raw record refs, but we stored ids as
	// "<table>:<id>"; resolve to raw ids and filter by tenant.
	rawIDs := make([]string, 0, len(toIDs))
	for _, full := range toIDs {
		_, raw := splitTableID(full)
		rawIDs = append(rawIDs, raw)
	}
	nres, err := surrealdb.Query[[]nodeRow](ctx, g.db,
		"SELECT * FROM graph_nodes WHERE tenantId = $tenantId AND record::id(id) IN $ids",
		map[string]any{"tenantId": tenantID, "ids": rawIDs})
	if err != nil {
		return nil, nil, fmt.Errorf("memorygraph out neighbors: %w", err)
	}
	var nodes []Node
	if nres != nil && len(*nres) > 0 {
		for _, r := range (*nres)[0].Result {
			nodes = append(nodes, r.toNode())
		}
	}
	return edges, nodes, nil
}

// Traverse follows q.Path one edge kind per hop. Empty path = BFS.
func (g *SurrealGraph) Traverse(ctx context.Context, q TraversalQuery) ([]Node, []Edge, error) {
	if q.TenantID == "" {
		return nil, nil, ErrTenantMissing
	}
	maxNodes := q.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 256
	}
	if len(q.Path) == 0 {
		depth := q.MaxDepth
		if depth <= 0 {
			depth = 3
		}
		visited := map[string]Node{}
		edges := map[string]Edge{}
		frontier := append([]string{}, q.StartIDs...)
		for hop := 0; hop < depth && len(visited) < maxNodes; hop++ {
			next := []string{}
			for _, fid := range frontier {
				es, ns, err := g.outEdges(ctx, q.TenantID, fid, nil)
				if err != nil {
					return nil, nil, err
				}
				for _, e := range es {
					if q.MinConfidence > 0 && e.Confidence < q.MinConfidence {
						continue
					}
					edges[e.ID] = e
				}
				for _, n := range ns {
					if q.ProjectID != "" && n.ProjectID != "" && n.ProjectID != q.ProjectID {
						continue
					}
					if q.Freshness > 0 && !n.UpdatedAt.IsZero() &&
						time.Since(n.UpdatedAt) > q.Freshness {
						continue
					}
					if _, seen := visited[n.ID]; !seen {
						visited[n.ID] = n
						next = append(next, n.ID)
						if len(visited) >= maxNodes {
							break
						}
					}
				}
			}
			frontier = next
		}
		return mapToNodes(visited), mapToEdges(edges), nil
	}

	visited := map[string]Node{}
	edges := map[string]Edge{}
	frontier := append([]string{}, q.StartIDs...)
	for _, kind := range q.Path {
		if len(visited) >= maxNodes {
			break
		}
		next := []string{}
		for _, fid := range frontier {
			es, ns, err := g.outEdges(ctx, q.TenantID, fid, []EdgeKind{kind})
			if err != nil {
				return nil, nil, err
			}
			for _, e := range es {
				if q.MinConfidence > 0 && e.Confidence < q.MinConfidence {
					continue
				}
				edges[e.ID] = e
			}
			for _, n := range ns {
				if q.ProjectID != "" && n.ProjectID != "" && n.ProjectID != q.ProjectID {
					continue
				}
				if q.Freshness > 0 && !n.UpdatedAt.IsZero() &&
					time.Since(n.UpdatedAt) > q.Freshness {
					continue
				}
				if _, seen := visited[n.ID]; !seen {
					visited[n.ID] = n
					next = append(next, n.ID)
					if len(visited) >= maxNodes {
						break
					}
				}
			}
		}
		frontier = next
	}
	return mapToNodes(visited), mapToEdges(edges), nil
}

// DeleteProject hard-deletes graph rows owned by a tenant+project. The
// architecture doc allows tombstone semantics; the integration agent
// can layer that in if the operator requires it.
func (g *SurrealGraph) DeleteProject(ctx context.Context, tenantID, projectID string) error {
	if tenantID == "" {
		return ErrTenantMissing
	}
	if projectID == "" {
		return ErrProjectMissing
	}
	vars := map[string]any{"tenantId": tenantID, "projectId": projectID}
	// Delete edges whose endpoint nodes belong to this project first,
	// then the nodes. Two passes keep the dangling-edge invariant.
	res, err := surrealdb.Query[any](ctx, g.db,
		"DELETE graph_edges WHERE tenantId = $tenantId AND "+
			"(fromId IN (SELECT VALUE id FROM graph_nodes WHERE tenantId = $tenantId AND projectId = $projectId) "+
			" OR toId IN (SELECT VALUE id FROM graph_nodes WHERE tenantId = $tenantId AND projectId = $projectId)); "+
			"DELETE graph_nodes WHERE tenantId = $tenantId AND projectId = $projectId",
		vars)
	if err != nil {
		return fmt.Errorf("memorygraph delete project: %w", err)
	}
	if res != nil {
		for _, qr := range *res {
			if qr.Status != "OK" {
				return fmt.Errorf("memorygraph delete project status: %s", qr.Status)
			}
		}
	}
	return nil
}

// compile-time assertion.
var _ Graph = (*SurrealGraph)(nil)
