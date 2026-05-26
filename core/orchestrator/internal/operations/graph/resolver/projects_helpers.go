package resolver

import (
	"context"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/ai/memorygraph"
)

// cascadeProjectGraph is the Agent-30 fire-and-forget AI Memory Graph
// cleanup invoked after a project delete commits. Skips silently when
// the graph is unwired so dev boxes without Surreal/pgvector still boot.
// Failures land in the orchestrator log; the caller never blocks.
func (r *Resolver) cascadeProjectGraph(tenantID, projectID string) {
	g := r.MemoryGraph
	if g == nil {
		return
	}
	logger := r.Logger
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := memorygraph.CascadeProjectDeletion(ctx, g, tenantID, projectID); err != nil {
			logger.Warn().Err(err).
				Str("tenantID", tenantID).
				Str("projectID", projectID).
				Msg("memorygraph cascade delete failed")
		}
	}()
}

// projectToGraphQL converts the internal domain.Project into the
// GraphQL model.Project surface. Gates collapse into the GateVerdict
// shape used everywhere else in the schema.
func projectToGraphQL(p domain.Project) model.Project {
	files := make([]model.ProjectFile, 0, len(p.Files))
	for _, f := range p.Files {
		files = append(files, fileToGraphQL(f))
	}
	gates := make([]model.GateVerdict, 0, len(p.Gates))
	for name, g := range p.Gates {
		gates = append(gates, gateStateToGraphQL(name, g))
	}
	out := model.Project{
		ID:        p.ID,
		Name:      p.Name,
		Status:    p.Status,
		OwnerID:   p.OwnerID,
		IsPublic:  p.OwnerID == "",
		Files:     files,
		Gates:     gates,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	if p.Description != "" {
		d := p.Description
		out.Description = &d
	}
	if p.Spec.Idea != "" {
		i := p.Spec.Idea
		out.Idea = &i
	}
	return out
}

func fileToGraphQL(f domain.FileNode) model.ProjectFile {
	out := model.ProjectFile{
		Path: f.Path,
	}
	if f.Content != "" {
		c := f.Content
		out.Content = &c
	}
	if f.Size > 0 {
		s := f.Size
		out.Size = &s
	}
	if f.Type != "" {
		l := f.Type
		out.Language = &l
	}
	return out
}

// gateStateToGraphQL is a thin shim from domain.GateState to
// model.GateVerdict. We keep it local because the GateVerdict shape
// is shared by both Project and gate resolvers.
func gateStateToGraphQL(name domain.GateName, g domain.GateState) model.GateVerdict {
	issues := make([]model.GateIssue, 0, len(g.Issues))
	for _, i := range g.Issues {
		path := i.Path
		sev := string(i.Severity)
		gi := model.GateIssue{
			Message: i.Message,
		}
		if path != "" {
			gi.Path = &path
		}
		if sev != "" {
			gi.Severity = &sev
		}
		issues = append(issues, gi)
	}
	return model.GateVerdict{
		Gate:   string(name),
		Status: model.GateStatus(g.Status),
		Issues: issues,
	}
}

// emptyProjectGates returns the default empty gate map (each gate at
// pending status). Used by CreateProject so the row is queryable from
// the very first response.
func emptyProjectGates(now time.Time) map[domain.GateName]domain.GateState {
	out := make(map[domain.GateName]domain.GateState, 9)
	for _, name := range domain.AllGates() {
		out[name] = domain.GateState{
			Name:      name,
			Status:    domain.GateStatusPending,
			Issues:    nil,
			UpdatedAt: now,
		}
	}
	return out
}

// runEventToGraphQL projects one domain.Event into the matching
// RunEvent union member for the runProject subscription. Falls back
// to RunGateEvent when status/gate are populated, otherwise
// RunDoneEvent so the channel still progresses.
func runEventToGraphQL(ev domain.Event) model.RunEvent {
	if ev.Gate != "" {
		msg := ev.Message
		return model.RunGateEvent{
			Ts:      ev.CreatedAt,
			Gate:    string(ev.Gate),
			Status:  ev.Status,
			Message: &msg,
		}
	}
	return model.RunExecutionEvent{
		Ts: ev.CreatedAt,
		Payload: model.JSON{
			"id":      ev.ID,
			"step":    ev.Step,
			"agent":   ev.Agent,
			"message": ev.Message,
			"status":  ev.Status,
		},
	}
}
