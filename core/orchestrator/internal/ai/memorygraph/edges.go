package memorygraph

import "time"

// EdgeKind enumerates the V22 relation vocabulary. Like NodeKind these
// strings are wire-stable. The kind set MUST match the relation list in
// docs/ARCHITECTURE_MEMORY_GRAPH.md.
type EdgeKind string

const (
	EdgeHasSpec        EdgeKind = "has_spec"         // project -> spec
	EdgeRequiresGate   EdgeKind = "requires_gate"    // spec    -> gate
	EdgeConcernsFile   EdgeKind = "concerns_file"    // spec    -> file
	EdgeDefinesSymbol  EdgeKind = "defines_symbol"   // file    -> symbol
	EdgeImports        EdgeKind = "imports"          // symbol  -> symbol
	EdgeCalls          EdgeKind = "calls"            // symbol  -> symbol
	EdgeRenders        EdgeKind = "renders"          // symbol  -> symbol
	EdgeQueries        EdgeKind = "queries"          // symbol  -> symbol
	EdgeModifiesFile   EdgeKind = "modifies_file"    // patch   -> file
	EdgeImplementsSpec EdgeKind = "implements_spec"  // patch   -> spec
	EdgeCausedFailure  EdgeKind = "caused_failure"   // patch   -> failure
	EdgeObservedInFile EdgeKind = "observed_in_file" // failure -> file
	EdgeMatchesRepair  EdgeKind = "matches_repair"   // failure -> repair
	EdgeFixedByPatch   EdgeKind = "fixed_by_patch"   // repair  -> patch
	EdgeAboutProject   EdgeKind = "about_project"    // agent_memory -> project
	EdgeAboutFile      EdgeKind = "about_file"       // agent_memory -> file
	EdgeAboutFailure   EdgeKind = "about_failure"    // agent_memory -> failure
	EdgeAboutRepair    EdgeKind = "about_repair"     // agent_memory -> repair
)

// AllEdgeKinds returns the canonical iteration order. Used by schema
// bootstrap and traversal builders that want a deny-by-default kind set.
func AllEdgeKinds() []EdgeKind {
	return []EdgeKind{
		EdgeHasSpec, EdgeRequiresGate, EdgeConcernsFile, EdgeDefinesSymbol,
		EdgeImports, EdgeCalls, EdgeRenders, EdgeQueries,
		EdgeModifiesFile, EdgeImplementsSpec, EdgeCausedFailure,
		EdgeObservedInFile, EdgeMatchesRepair, EdgeFixedByPatch,
		EdgeAboutProject, EdgeAboutFile, EdgeAboutFailure, EdgeAboutRepair,
	}
}

// Edge is a single graph relation. Provenance + Confidence + Inferred
// are mandatory per the architecture doc: low-confidence inferred edges
// must never be promoted to canonical state without a canonical event.
type Edge struct {
	ID         string         `json:"id"`
	Kind       EdgeKind       `json:"kind"`
	From       string         `json:"from"`
	To         string         `json:"to"`
	TenantID   string         `json:"tenantId"`
	Weight     float64        `json:"weight,omitempty"`
	Inferred   bool           `json:"inferred,omitempty"`
	Confidence float64        `json:"confidence,omitempty"`
	Attrs      map[string]any `json:"attrs,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	Provenance Provenance     `json:"provenance"`
}

// edgeKindShape maps each EdgeKind to the (fromKind, toKind) pair the
// relation enforces. Used by Edge.validate so a "fixed_by_patch" edge
// can't accidentally point file -> spec.
type edgeKindShape struct{ from, to NodeKind }

var edgeShapes = map[EdgeKind]edgeKindShape{
	EdgeHasSpec:        {NodeProject, NodeSpec},
	EdgeRequiresGate:   {NodeSpec, NodeGate},
	EdgeConcernsFile:   {NodeSpec, NodeFile},
	EdgeDefinesSymbol:  {NodeFile, NodeSymbol},
	EdgeImports:        {NodeSymbol, NodeSymbol},
	EdgeCalls:          {NodeSymbol, NodeSymbol},
	EdgeRenders:        {NodeSymbol, NodeSymbol},
	EdgeQueries:        {NodeSymbol, NodeSymbol},
	EdgeModifiesFile:   {NodePatch, NodeFile},
	EdgeImplementsSpec: {NodePatch, NodeSpec},
	EdgeCausedFailure:  {NodePatch, NodeFailure},
	EdgeObservedInFile: {NodeFailure, NodeFile},
	EdgeMatchesRepair:  {NodeFailure, NodeRepair},
	EdgeFixedByPatch:   {NodeRepair, NodePatch},
	EdgeAboutProject:   {NodeAgentMemory, NodeProject},
	EdgeAboutFile:      {NodeAgentMemory, NodeFile},
	EdgeAboutFailure:   {NodeAgentMemory, NodeFailure},
	EdgeAboutRepair:    {NodeAgentMemory, NodeRepair},
}

// EdgeShape returns the expected from/to NodeKinds for kind. Returns
// (_, _, false) for unknown kinds. Useful for callers that want to
// guard edge construction before persistence.
func EdgeShape(kind EdgeKind) (NodeKind, NodeKind, bool) {
	s, ok := edgeShapes[kind]
	return s.from, s.to, ok
}

// validate enforces the minimum shape. Cross-kind enforcement requires
// the actual node kinds at the call site (see NewEdge below), so the
// pure-edge validation only checks identity and tenant.
func (e Edge) validate() error {
	if e.TenantID == "" {
		return ErrTenantMissing
	}
	if e.Kind == "" || e.From == "" || e.To == "" || e.ID == "" {
		return ErrInvalidEdge
	}
	if _, ok := edgeShapes[e.Kind]; !ok {
		return ErrInvalidEdge
	}
	return nil
}

// NewEdge is the canonical edge constructor. fromKind/toKind let the
// constructor verify the shape contract; callers that already have
// fully-populated Node values should use NewEdgeFromNodes.
func NewEdge(kind EdgeKind, tenantID, fromID, toID string, fromKind, toKind NodeKind, prov Provenance) (Edge, error) {
	shape, ok := edgeShapes[kind]
	if !ok {
		return Edge{}, ErrInvalidEdge
	}
	if fromKind != "" && shape.from != fromKind {
		return Edge{}, ErrInvalidEdge
	}
	if toKind != "" && shape.to != toKind {
		return Edge{}, ErrInvalidEdge
	}
	now := prov.RecordedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Edge{
		ID:         BuildEdgeID(kind, fromID, toID),
		Kind:       kind,
		From:       fromID,
		To:         toID,
		TenantID:   tenantID,
		Confidence: 1.0,
		CreatedAt:  now,
		Provenance: prov,
	}, nil
}

// NewEdgeFromNodes is the convenience overload when callers already
// have the resolved Node values; it pulls Kind off each so the shape
// contract is enforced automatically.
func NewEdgeFromNodes(kind EdgeKind, from, to Node, prov Provenance) (Edge, error) {
	if from.TenantID != to.TenantID {
		return Edge{}, ErrTenantMissing
	}
	return NewEdge(kind, from.TenantID, from.ID, to.ID, from.Kind, to.Kind, prov)
}
