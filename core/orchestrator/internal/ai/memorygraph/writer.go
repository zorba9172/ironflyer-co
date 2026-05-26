package memorygraph

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// Event is the projection input. It is intentionally generic so the
// integration agent can drive the writer off the outbox (Agent 15) or
// any in-process event bus without coupling memorygraph to a transport.
//
// Convention: Kind values are versioned ("patch.applied.v1") so the
// projection rule set can evolve without breaking older outboxes.
type Event struct {
	Kind       string
	Payload    map[string]any
	Provenance Provenance
}

// Writer projects orchestrator events into the AI Memory Graph. Errors
// are logged and returned; the caller (the integration agent) decides
// whether projection failures should be retried, dead-lettered, or
// dropped. Per the architecture, projection failures MUST NOT
// invalidate the canonical execution.
type Writer struct {
	g   Graph
	log zerolog.Logger
}

// NewWriter binds a Graph to a logger. The integration agent
// subscribes the result to the event stream.
func NewWriter(g Graph, log zerolog.Logger) *Writer {
	return &Writer{g: g, log: log}
}

// Event kind constants. These are the projection rules the writer
// currently knows about. Unknown kinds are logged and ignored.
const (
	EventPatchApplied   = "patch.applied.v1"
	EventGateFailed     = "gate.failed.v1"
	EventRepairRecorded = "repair.recorded.v1"
	EventSpecCreated    = "spec.created.v1"
	EventFileTouched    = "file.touched.v1"
	EventProjectCreated = "project.created.v1"
	EventGateRequired   = "gate.required.v1"
	EventSymbolDefined  = "symbol.defined.v1"
	EventAgentMemory    = "agent.memory.v1"
)

// Handle dispatches by Event.Kind. Each branch upserts canonical nodes
// and the edges that link them per the V22 relation list.
func (w *Writer) Handle(ctx context.Context, e Event) error {
	if w == nil || w.g == nil {
		return nil
	}
	if e.Provenance.RecordedAt.IsZero() {
		e.Provenance.RecordedAt = time.Now().UTC()
	}
	switch e.Kind {
	case EventProjectCreated:
		return w.onProjectCreated(ctx, e)
	case EventSpecCreated:
		return w.onSpecCreated(ctx, e)
	case EventGateRequired:
		return w.onGateRequired(ctx, e)
	case EventFileTouched:
		return w.onFileTouched(ctx, e)
	case EventSymbolDefined:
		return w.onSymbolDefined(ctx, e)
	case EventPatchApplied:
		return w.onPatchApplied(ctx, e)
	case EventGateFailed:
		return w.onGateFailed(ctx, e)
	case EventRepairRecorded:
		return w.onRepairRecorded(ctx, e)
	case EventAgentMemory:
		return w.onAgentMemory(ctx, e)
	default:
		w.log.Debug().Str("kind", e.Kind).Msg("memorygraph writer: unhandled event kind, ignoring")
		return nil
	}
}

// onProjectCreated upserts the project root vertex.
func (w *Writer) onProjectCreated(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	if tenantID == "" || projectID == "" {
		return fmt.Errorf("memorygraph: project.created missing tenantId/projectId")
	}
	n := NewProjectNode(tenantID, projectID, str(e.Payload, "summary"), e.Provenance)
	_, err := w.g.UpsertNode(ctx, n)
	return err
}

// onSpecCreated upserts a spec vertex and links project -> has_spec -> spec.
func (w *Writer) onSpecCreated(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	specID := str(e.Payload, "specId")
	if tenantID == "" || projectID == "" || specID == "" {
		return fmt.Errorf("memorygraph: spec.created missing ids")
	}
	specNode := NewSpecNode(tenantID, projectID, specID, str(e.Payload, "summary"), e.Provenance)
	if _, err := w.g.UpsertNode(ctx, specNode); err != nil {
		return err
	}
	// We assume the project node was previously projected; if not the
	// integration agent should subscribe project.created first.
	projectNode := NewProjectNode(tenantID, projectID, "", e.Provenance)
	if _, err := w.g.UpsertNode(ctx, projectNode); err != nil {
		return err
	}
	edge, err := NewEdgeFromNodes(EdgeHasSpec, projectNode, specNode, e.Provenance)
	if err != nil {
		return err
	}
	_, err = w.g.UpsertEdge(ctx, edge)
	return err
}

// onGateRequired projects spec -> requires_gate -> gate.
func (w *Writer) onGateRequired(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	specID := str(e.Payload, "specId")
	gateID := str(e.Payload, "gateId")
	gateName := str(e.Payload, "gateName")
	if tenantID == "" || projectID == "" || specID == "" || gateID == "" {
		return fmt.Errorf("memorygraph: gate.required missing ids")
	}
	specNode := NewSpecNode(tenantID, projectID, specID, "", e.Provenance)
	gateNode := NewGateNode(tenantID, projectID, gateID, gateName, str(e.Payload, "summary"), e.Provenance)
	if _, err := w.g.UpsertNode(ctx, specNode); err != nil {
		return err
	}
	if _, err := w.g.UpsertNode(ctx, gateNode); err != nil {
		return err
	}
	edge, err := NewEdgeFromNodes(EdgeRequiresGate, specNode, gateNode, e.Provenance)
	if err != nil {
		return err
	}
	_, err = w.g.UpsertEdge(ctx, edge)
	return err
}

// onFileTouched upserts a file vertex. When the event carries one or
// more specIds, spec -> concerns_file -> file edges are projected too.
func (w *Writer) onFileTouched(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	path := str(e.Payload, "path")
	if tenantID == "" || projectID == "" || path == "" {
		return fmt.Errorf("memorygraph: file.touched missing ids/path")
	}
	fileNode := NewFileNode(tenantID, projectID, path, str(e.Payload, "summary"), e.Provenance)
	if _, err := w.g.UpsertNode(ctx, fileNode); err != nil {
		return err
	}
	for _, specID := range strSlice(e.Payload, "specIds") {
		specNode := NewSpecNode(tenantID, projectID, specID, "", e.Provenance)
		if _, err := w.g.UpsertNode(ctx, specNode); err != nil {
			return err
		}
		edge, err := NewEdgeFromNodes(EdgeConcernsFile, specNode, fileNode, e.Provenance)
		if err != nil {
			return err
		}
		if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
			return err
		}
	}
	return nil
}

// onSymbolDefined upserts a symbol vertex and links file -> defines_symbol.
// Also captures imports/calls/renders/queries when the event lists them.
func (w *Writer) onSymbolDefined(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	file := str(e.Payload, "file")
	symbol := str(e.Payload, "symbol")
	if tenantID == "" || projectID == "" || file == "" || symbol == "" {
		return fmt.Errorf("memorygraph: symbol.defined missing ids")
	}
	kindStr := str(e.Payload, "symbolKind")
	fileNode := NewFileNode(tenantID, projectID, file, "", e.Provenance)
	symNode := NewSymbolNode(tenantID, projectID, file, symbol, kindStr, str(e.Payload, "summary"), e.Provenance)
	if _, err := w.g.UpsertNode(ctx, fileNode); err != nil {
		return err
	}
	if _, err := w.g.UpsertNode(ctx, symNode); err != nil {
		return err
	}
	edge, err := NewEdgeFromNodes(EdgeDefinesSymbol, fileNode, symNode, e.Provenance)
	if err != nil {
		return err
	}
	if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
		return err
	}
	for _, relKind := range []struct {
		field string
		kind  EdgeKind
	}{
		{"imports", EdgeImports},
		{"calls", EdgeCalls},
		{"renders", EdgeRenders},
		{"queries", EdgeQueries},
	} {
		for _, ref := range strSlice(e.Payload, relKind.field) {
			// ref is "<file>#<symbol>" so the destination symbol node
			// keys consistently with NewSymbolNode.
			f, s := splitSymbolRef(ref, file)
			dest := NewSymbolNode(tenantID, projectID, f, s, "", "", e.Provenance)
			if _, err := w.g.UpsertNode(ctx, dest); err != nil {
				return err
			}
			edge, err := NewEdgeFromNodes(relKind.kind, symNode, dest, e.Provenance)
			if err != nil {
				return err
			}
			if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
				return err
			}
		}
	}
	return nil
}

// onPatchApplied upserts the patch vertex plus modifies_file edges per
// path in the event, and an implements_spec edge per spec id. The
// patch row keeps a back-link to the canonical patch via CanonicalRef.
func (w *Writer) onPatchApplied(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	patchID := str(e.Payload, "patchId")
	if tenantID == "" || projectID == "" || patchID == "" {
		return fmt.Errorf("memorygraph: patch.applied missing ids")
	}
	patchNode := NewPatchNode(tenantID, projectID, patchID, str(e.Payload, "summary"), e.Provenance)
	if _, err := w.g.UpsertNode(ctx, patchNode); err != nil {
		return err
	}
	for _, path := range strSlice(e.Payload, "paths") {
		fileNode := NewFileNode(tenantID, projectID, path, "", e.Provenance)
		if _, err := w.g.UpsertNode(ctx, fileNode); err != nil {
			return err
		}
		edge, err := NewEdgeFromNodes(EdgeModifiesFile, patchNode, fileNode, e.Provenance)
		if err != nil {
			return err
		}
		if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
			return err
		}
	}
	for _, specID := range strSlice(e.Payload, "specIds") {
		specNode := NewSpecNode(tenantID, projectID, specID, "", e.Provenance)
		if _, err := w.g.UpsertNode(ctx, specNode); err != nil {
			return err
		}
		edge, err := NewEdgeFromNodes(EdgeImplementsSpec, patchNode, specNode, e.Provenance)
		if err != nil {
			return err
		}
		if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
			return err
		}
	}
	return nil
}

// onGateFailed upserts the failure vertex keyed on the failure
// signature and links it to the offending file(s) and (when known)
// the patch that triggered the gate.
func (w *Writer) onGateFailed(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	signature := str(e.Payload, "signature")
	if tenantID == "" || projectID == "" || signature == "" {
		return fmt.Errorf("memorygraph: gate.failed missing ids/signature")
	}
	failNode := NewFailureNode(tenantID, projectID, signature, str(e.Payload, "summary"), e.Provenance)
	if _, err := w.g.UpsertNode(ctx, failNode); err != nil {
		return err
	}
	for _, path := range strSlice(e.Payload, "paths") {
		fileNode := NewFileNode(tenantID, projectID, path, "", e.Provenance)
		if _, err := w.g.UpsertNode(ctx, fileNode); err != nil {
			return err
		}
		edge, err := NewEdgeFromNodes(EdgeObservedInFile, failNode, fileNode, e.Provenance)
		if err != nil {
			return err
		}
		if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
			return err
		}
	}
	if patchID := str(e.Payload, "patchId"); patchID != "" {
		patchNode := NewPatchNode(tenantID, projectID, patchID, "", e.Provenance)
		if _, err := w.g.UpsertNode(ctx, patchNode); err != nil {
			return err
		}
		edge, err := NewEdgeFromNodes(EdgeCausedFailure, patchNode, failNode, e.Provenance)
		if err != nil {
			return err
		}
		if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
			return err
		}
	}
	return nil
}

// onRepairRecorded upserts the repair vertex and links it to the
// failure signature that matched. When the event includes a patchId,
// repair -> fixed_by_patch -> patch is projected too.
func (w *Writer) onRepairRecorded(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	repairID := str(e.Payload, "repairId")
	signature := str(e.Payload, "signature")
	if tenantID == "" || projectID == "" || repairID == "" {
		return fmt.Errorf("memorygraph: repair.recorded missing ids")
	}
	repairNode := NewRepairNode(tenantID, projectID, repairID, str(e.Payload, "summary"), e.Provenance)
	if _, err := w.g.UpsertNode(ctx, repairNode); err != nil {
		return err
	}
	if signature != "" {
		failNode := NewFailureNode(tenantID, projectID, signature, "", e.Provenance)
		if _, err := w.g.UpsertNode(ctx, failNode); err != nil {
			return err
		}
		edge, err := NewEdgeFromNodes(EdgeMatchesRepair, failNode, repairNode, e.Provenance)
		if err != nil {
			return err
		}
		if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
			return err
		}
	}
	if patchID := str(e.Payload, "patchId"); patchID != "" {
		patchNode := NewPatchNode(tenantID, projectID, patchID, "", e.Provenance)
		if _, err := w.g.UpsertNode(ctx, patchNode); err != nil {
			return err
		}
		edge, err := NewEdgeFromNodes(EdgeFixedByPatch, repairNode, patchNode, e.Provenance)
		if err != nil {
			return err
		}
		if _, err := w.g.UpsertEdge(ctx, edge); err != nil {
			return err
		}
	}
	return nil
}

// onAgentMemory wraps an agent-emitted observation. Subject is one of
// {project, file, failure, repair}, and Edge picks the matching
// about_* kind so the memory is queryable from its target vertex.
func (w *Writer) onAgentMemory(ctx context.Context, e Event) error {
	tenantID := str(e.Payload, "tenantId")
	projectID := str(e.Payload, "projectId")
	memoryID := str(e.Payload, "memoryId")
	if tenantID == "" || memoryID == "" {
		return fmt.Errorf("memorygraph: agent.memory missing ids")
	}
	memNode := NewAgentMemoryNode(tenantID, projectID, memoryID, str(e.Payload, "summary"), e.Provenance)
	if _, err := w.g.UpsertNode(ctx, memNode); err != nil {
		return err
	}
	subject := str(e.Payload, "subjectKind")
	subjectID := str(e.Payload, "subjectId")
	if subject == "" || subjectID == "" {
		return nil
	}
	var subjectNode Node
	var edgeKind EdgeKind
	switch subject {
	case string(NodeProject):
		subjectNode = NewProjectNode(tenantID, subjectID, "", e.Provenance)
		edgeKind = EdgeAboutProject
	case string(NodeFile):
		subjectNode = NewFileNode(tenantID, projectID, subjectID, "", e.Provenance)
		edgeKind = EdgeAboutFile
	case string(NodeFailure):
		subjectNode = NewFailureNode(tenantID, projectID, subjectID, "", e.Provenance)
		edgeKind = EdgeAboutFailure
	case string(NodeRepair):
		subjectNode = NewRepairNode(tenantID, projectID, subjectID, "", e.Provenance)
		edgeKind = EdgeAboutRepair
	default:
		return nil
	}
	if _, err := w.g.UpsertNode(ctx, subjectNode); err != nil {
		return err
	}
	edge, err := NewEdgeFromNodes(edgeKind, memNode, subjectNode, e.Provenance)
	if err != nil {
		return err
	}
	_, err = w.g.UpsertEdge(ctx, edge)
	return err
}

// str pulls a string field with a zero-default; payload values may be
// any JSON-shaped Go type so we accept string + types::stringer.
func str(p map[string]any, k string) string {
	if p == nil {
		return ""
	}
	v, ok := p[k]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case fmt.Stringer:
		return s.String()
	default:
		return fmt.Sprintf("%v", s)
	}
}

// strSlice pulls a []string field with a nil-default; coerces []any
// (the JSON-decoded shape) to []string transparently.
func strSlice(p map[string]any, k string) []string {
	if p == nil {
		return nil
	}
	v, ok := p[k]
	if !ok || v == nil {
		return nil
	}
	switch xs := v.(type) {
	case []string:
		return xs
	case []any:
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			out = append(out, fmt.Sprintf("%v", x))
		}
		return out
	}
	return nil
}

func splitSymbolRef(ref, defaultFile string) (string, string) {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '#' {
			return ref[:i], ref[i+1:]
		}
	}
	// No file qualifier — treat ref as a symbol in the same file.
	return defaultFile, ref
}
