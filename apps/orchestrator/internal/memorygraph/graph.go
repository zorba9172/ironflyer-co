package memorygraph

import (
	"context"
	"time"
)

// Graph is the operator-replaceable storage contract for the V22 AI
// Memory Graph. Implementations MUST be safe for concurrent use and
// MUST enforce tenant isolation on every method — see ErrTenantMissing.
//
// All writes are idempotent by the canonical Node.ID / Edge.ID. Re-
// playing the same projection event must converge on the same graph.
type Graph interface {
	// UpsertNode inserts or merges n keyed by Node.ID. Returns the
	// stored row (with timestamps populated) so callers can chain
	// edge writes off the canonical id.
	UpsertNode(ctx context.Context, n Node) (Node, error)

	// UpsertEdge inserts or merges e keyed by Edge.ID. Returns the
	// stored edge.
	UpsertEdge(ctx context.Context, e Edge) (Edge, error)

	// GetNode looks up a node by id, scoped to tenantID. Returns
	// (Node{}, false, nil) when the id is unknown; only returns an
	// error for transport/store-level failures.
	GetNode(ctx context.Context, tenantID, id string) (Node, bool, error)

	// Neighbors returns nodes one hop away from id following any of
	// edgeKinds (empty = all kinds). depth bounds traversal recursion;
	// 1 means direct neighbors only. Tenant isolation: only edges and
	// nodes carrying tenantID are returned.
	Neighbors(ctx context.Context, tenantID, id string, edgeKinds []EdgeKind, depth int) ([]Node, []Edge, error)

	// Traverse evaluates a path-shaped query against the graph. The
	// canonical use is the IntentGateRepair path:
	//   spec -> concerns_file -> file -> defines_symbol -> symbol -> ...
	//   patch -> caused_failure -> failure -> matches_repair -> repair -> fixed_by_patch -> patch
	Traverse(ctx context.Context, q TraversalQuery) ([]Node, []Edge, error)

	// DeleteProject removes (or tombstones) every node/edge owned by
	// projectID inside tenantID, per the architecture's retention rule.
	// MUST NOT touch ledger / execution / wallet rows — those live in
	// Postgres, not the graph.
	DeleteProject(ctx context.Context, tenantID, projectID string) error

	// Bootstrap installs the SurrealQL schema (tables, fields, indexes).
	// Idempotent; safe to call on every boot. In-memory implementations
	// may treat it as a no-op.
	Bootstrap(ctx context.Context) error
}

// TraversalQuery describes a path-shaped graph walk. StartIDs anchor
// the traversal; Path lists the ordered edge kinds to follow at each
// hop (one hop per kind). MaxDepth caps recursion when Path repeats or
// is empty. Tenant + project scoping is mandatory.
type TraversalQuery struct {
	TenantID  string
	ProjectID string
	StartIDs  []string
	// Path is the ordered traversal pattern. e.g.
	//   [EdgeConcernsFile, EdgeDefinesSymbol]
	// follows file edges off the start, then symbols off each file.
	// When empty, Traverse falls back to BFS bounded by MaxDepth.
	Path []EdgeKind
	// MaxNodes caps the total nodes returned. 0 means use the
	// implementation default (256).
	MaxNodes int
	// MaxDepth bounds recursion when Path is empty or repeats. 0
	// means use the implementation default (3).
	MaxDepth int
	// MinConfidence drops edges/nodes whose Confidence is below the
	// threshold. 0 keeps everything.
	MinConfidence float64
	// Freshness bounds Node.UpdatedAt; older rows are dropped. Zero
	// duration means no freshness filter.
	Freshness time.Duration
}
