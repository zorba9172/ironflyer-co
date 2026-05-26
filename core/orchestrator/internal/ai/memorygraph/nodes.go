package memorygraph

import "time"

// NodeKind enumerates the V22 graph vocabulary. The string values are
// stable — they appear on the wire (SurrealQL records, traversal
// queries, retrieval packets) and in agent prompts, so renaming them is
// a breaking change. See docs/ARCHITECTURE_MEMORY_GRAPH.md.
type NodeKind string

const (
	NodeProject        NodeKind = "project"
	NodeSpec           NodeKind = "spec"
	NodeGate           NodeKind = "gate"
	NodeFile           NodeKind = "file"
	NodeSymbol         NodeKind = "symbol"
	NodePatch          NodeKind = "patch"
	NodeFailure        NodeKind = "failure"
	NodeRepair         NodeKind = "repair"
	NodeAgentMemory    NodeKind = "agent_memory"
	NodeEmbeddingChunk NodeKind = "embedding_chunk"
)

// AllNodeKinds returns the canonical iteration order. Used by schema
// bootstrap and retention sweeps.
func AllNodeKinds() []NodeKind {
	return []NodeKind{
		NodeProject, NodeSpec, NodeGate, NodeFile, NodeSymbol,
		NodePatch, NodeFailure, NodeRepair, NodeAgentMemory, NodeEmbeddingChunk,
	}
}

// Provenance binds a graph row back to the orchestrator event that
// produced it. Every write MUST carry provenance per the
// "Operational Rules" in the architecture doc.
type Provenance struct {
	// SourceEventID is the canonical event id (outbox row, repair
	// recipe id, patch id, etc.) — the same id reprocessing the event
	// would carry, so writes converge.
	SourceEventID string `json:"sourceEventId,omitempty"`
	// SourceEventType names the projection rule that emitted this
	// row (e.g. "patch.applied.v1").
	SourceEventType string `json:"sourceEventType,omitempty"`
	// RecordedAt is the wall clock when the projection ran. Distinct
	// from CreatedAt so reprojection can preserve original order.
	RecordedAt time.Time `json:"recordedAt"`
}

// Node is a single graph vertex. The same shape covers all NodeKinds;
// kind-specific fields live in Attrs to keep the SurrealDB schema
// SCHEMALESS-friendly. ID is "<kind>:<canonical_id>" — see ids.go.
type Node struct {
	ID           string         `json:"id"`
	Kind         NodeKind       `json:"kind"`
	TenantID     string         `json:"tenantId"`
	ProjectID    string         `json:"projectId,omitempty"`
	CanonicalRef string         `json:"canonicalRef,omitempty"`
	Attrs        map[string]any `json:"attrs,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Confidence   float64        `json:"confidence,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	Provenance   Provenance     `json:"provenance"`
}

// validate enforces the minimum-shape contract before persistence.
// Idempotent upserts must reject malformed payloads early; partial
// rows would corrupt traversal.
func (n Node) validate() error {
	if n.TenantID == "" {
		return ErrTenantMissing
	}
	if n.Kind == "" || n.ID == "" {
		return ErrInvalidNode
	}
	// Project scoping is mandatory for all kinds that descend from a
	// project. Embedding chunks and agent memories may be globally
	// scoped (user-level), but every other kind needs ProjectID.
	switch n.Kind {
	case NodeAgentMemory, NodeEmbeddingChunk:
		// ProjectID optional.
	default:
		if n.ProjectID == "" {
			return ErrProjectMissing
		}
	}
	return nil
}

// NewProjectNode returns a project root vertex. canonicalProjectID is
// the Postgres project row id; we never re-derive project truth from
// the graph.
func NewProjectNode(tenantID, canonicalProjectID, summary string, prov Provenance) Node {
	return baseNode(NodeProject, tenantID, canonicalProjectID, canonicalProjectID, summary, prov)
}

// NewSpecNode binds a spec row to the project. canonicalSpecID is the
// Postgres spec row id.
func NewSpecNode(tenantID, projectID, canonicalSpecID, summary string, prov Provenance) Node {
	n := baseNode(NodeSpec, tenantID, canonicalSpecID, projectID, summary, prov)
	n.CanonicalRef = canonicalSpecID
	return n
}

// NewGateNode wraps a finisher gate instance. canonicalGateID is the
// gate execution row id in Postgres.
func NewGateNode(tenantID, projectID, canonicalGateID, gateName, summary string, prov Provenance) Node {
	n := baseNode(NodeGate, tenantID, canonicalGateID, projectID, summary, prov)
	n.CanonicalRef = canonicalGateID
	if n.Attrs == nil {
		n.Attrs = map[string]any{}
	}
	n.Attrs["gateName"] = gateName
	return n
}

// NewFileNode keys by repo-relative path. Snapshot-aware callers stamp
// snapshotId into Attrs.
func NewFileNode(tenantID, projectID, path, summary string, prov Provenance) Node {
	n := baseNode(NodeFile, tenantID, path, projectID, summary, prov)
	n.CanonicalRef = path
	if n.Attrs == nil {
		n.Attrs = map[string]any{}
	}
	n.Attrs["path"] = path
	return n
}

// NewSymbolNode keys by "<file>#<symbol>" so two files can each declare
// a symbol with the same simple name without colliding.
func NewSymbolNode(tenantID, projectID, file, symbol, kind, summary string, prov Provenance) Node {
	key := file + "#" + symbol
	n := baseNode(NodeSymbol, tenantID, key, projectID, summary, prov)
	n.CanonicalRef = key
	if n.Attrs == nil {
		n.Attrs = map[string]any{}
	}
	n.Attrs["file"] = file
	n.Attrs["symbol"] = symbol
	n.Attrs["symbolKind"] = kind
	return n
}

// NewPatchNode wraps a canonical patch row. canonicalPatchID is the
// Postgres patch_proposals.id or runtime snapshot id.
func NewPatchNode(tenantID, projectID, canonicalPatchID, summary string, prov Provenance) Node {
	n := baseNode(NodePatch, tenantID, canonicalPatchID, projectID, summary, prov)
	n.CanonicalRef = canonicalPatchID
	return n
}

// NewFailureNode wraps a failure signature. canonicalSignature is the
// repair.Signature key so the graph collapses repeated failures onto a
// single vertex.
func NewFailureNode(tenantID, projectID, canonicalSignature, summary string, prov Provenance) Node {
	n := baseNode(NodeFailure, tenantID, canonicalSignature, projectID, summary, prov)
	n.CanonicalRef = canonicalSignature
	return n
}

// NewRepairNode wraps a repair recipe. canonicalRepairID is the
// repair_recipes.id row.
func NewRepairNode(tenantID, projectID, canonicalRepairID, summary string, prov Provenance) Node {
	n := baseNode(NodeRepair, tenantID, canonicalRepairID, projectID, summary, prov)
	n.CanonicalRef = canonicalRepairID
	return n
}

// NewAgentMemoryNode is a bounded observation summary. ProjectID may
// be empty for user-level memory.
func NewAgentMemoryNode(tenantID, projectID, canonicalMemoryID, summary string, prov Provenance) Node {
	n := baseNode(NodeAgentMemory, tenantID, canonicalMemoryID, projectID, summary, prov)
	n.CanonicalRef = canonicalMemoryID
	return n
}

// NewEmbeddingChunkNode wraps a retrievable text/code/spec chunk.
func NewEmbeddingChunkNode(tenantID, projectID, canonicalChunkID, summary string, prov Provenance) Node {
	n := baseNode(NodeEmbeddingChunk, tenantID, canonicalChunkID, projectID, summary, prov)
	n.CanonicalRef = canonicalChunkID
	return n
}

// baseNode assembles the common fields with a canonical id and a
// safety net: idempotent CreatedAt/UpdatedAt stamping if the caller
// hasn't supplied them. SurrealDB upserts also set updatedAt server-side.
func baseNode(kind NodeKind, tenantID, canonicalID, projectID, summary string, prov Provenance) Node {
	now := prov.RecordedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Node{
		ID:         BuildNodeID(kind, tenantID, canonicalID),
		Kind:       kind,
		TenantID:   tenantID,
		ProjectID:  projectID,
		Summary:    summary,
		CreatedAt:  now,
		UpdatedAt:  now,
		Provenance: prov,
	}
}
