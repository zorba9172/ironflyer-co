// Package finisher — memory capture + replay. This is the wiring that
// turns isolated agent calls into a compounding asset: every failure
// the loop survives, every fix that worked, every architectural
// decision the planner committed to gets written to the memory store;
// every subsequent agent run reads back what's relevant and inlines
// it into its context.
//
// Capture happens at the boundaries of the existing flows so the
// memory store is fed automatically — no extra prompt overhead, no
// "remember this" instruction the agent has to be told.

package finisher

import (
	"context"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/memory"
	"ironflyer/apps/orchestrator/internal/patch"
)

// memoryEnabled is the nil-safe predicate the capture helpers gate on.
func (e *Engine) memoryEnabled() bool {
	return e != nil && e.memory != nil
}

// rememberDecision records a structured architectural decision. Called
// from runArchitect after a successful StackDecision lands so future
// runs (and the Architect itself, via inject below) see the chosen
// path without re-deriving it.
func (e *Engine) rememberDecision(ctx context.Context, projectID, title, body string, tags ...string) {
	if !e.memoryEnabled() {
		return
	}
	_, _ = e.memory.Record(ctx, memory.Record{
		Kind:       memory.KindProject,
		ProjectID:  projectID,
		Title:      title,
		Body:       body,
		Tags:       append([]string{"decision"}, tags...),
		Confidence: 0.9,
	})
}

// rememberFailureFix writes a failure-→-fix lineage entry. Called from
// codeOneStory after a previously failing story passes — we know which
// gate / issue category was open before the patch landed, so the next
// repair iteration on a similar failure sees "we hit this before, here
// is what worked."
func (e *Engine) rememberFailureFix(
	ctx context.Context,
	projectID, storyID string,
	gateName domain.GateName,
	previousIssues []domain.Issue,
	winningPatchID, winningPatchTitle string,
) {
	if !e.memoryEnabled() || len(previousIssues) == 0 {
		return
	}
	var b strings.Builder
	b.WriteString("Story `" + storyID + "` failed on gate `" + string(gateName) + "` with:\n")
	for i, iss := range previousIssues {
		if i >= 3 {
			b.WriteString("- … and " + itoaPositive(len(previousIssues)-3) + " more\n")
			break
		}
		b.WriteString("- " + tail(iss.Message, 200) + "\n")
	}
	b.WriteString("\nResolved by patch `" + winningPatchID + "`: " + winningPatchTitle)
	_, _ = e.memory.Record(ctx, memory.Record{
		Kind:       memory.KindExecution,
		ProjectID:  projectID,
		StoryID:    storyID,
		GateName:   string(gateName),
		Title:      "fix: " + winningPatchTitle,
		Body:       b.String(),
		Tags:       []string{"failure", "fix", string(gateName)},
		Confidence: 0.8,
	})
}

// rememberCoderPatch records a clean (no critic findings) patch as a
// successful pattern. Cheaper than failure-fix lineage but still
// useful: future Coder calls on the same project see "we shipped this
// shape before; reuse it."
func (e *Engine) rememberCoderPatch(ctx context.Context, projectID string, p patch.Patch) {
	if !e.memoryEnabled() || len(p.Changes) == 0 {
		return
	}
	paths := make([]string, 0, len(p.Changes))
	for _, c := range p.Changes {
		paths = append(paths, c.Path)
		if len(paths) >= 6 {
			break
		}
	}
	body := p.Summary
	if body == "" {
		body = "Patch touched: " + strings.Join(paths, ", ")
	}
	_, _ = e.memory.Record(ctx, memory.Record{
		Kind:       memory.KindProject,
		ProjectID:  projectID,
		Title:      "pattern: " + p.Title,
		Body:       body,
		Tags:       []string{"patch", "pattern"},
		Confidence: 0.6,
	})
}

// contextBundleForArchitect returns the project's prior architectural
// decisions as a markdown block the Architect reads before re-deriving
// the stack. The point isn't to lock the agent in — it's to make sure
// "we already picked Supabase + Next.js" doesn't get re-litigated on
// every run. Empty string when memory is disabled or has no prior
// decisions.
func (e *Engine) contextBundleForArchitect(ctx context.Context, projectID string) string {
	if !e.memoryEnabled() {
		return ""
	}
	deadline, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	decisions, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindProject,
		ProjectID: projectID,
		Tag:       "decision",
		Limit:     12,
	})
	if len(decisions) == 0 {
		return ""
	}
	return memory.FormatForContext(decisions)
}

// contextBundleForPlanner returns prior project decisions, prior specs,
// and business-level notes so the Planner re-grounds in what's already
// been chosen instead of re-deriving stories from a blank slate. Empty
// string when memory is disabled or empty.
func (e *Engine) contextBundleForPlanner(ctx context.Context, projectID string) string {
	if !e.memoryEnabled() {
		return ""
	}
	deadline, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	decisions, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindProject,
		ProjectID: projectID,
		Tag:       "decision",
		Limit:     10,
	})
	specs, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindProject,
		ProjectID: projectID,
		Tag:       "spec",
		Limit:     6,
	})
	business, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindBusiness,
		ProjectID: projectID,
		Limit:     6,
	})
	merged := append(decisions, specs...)
	merged = append(merged, business...)
	if len(merged) == 0 {
		return ""
	}
	return memory.FormatForContext(merged)
}

// contextBundleForUXer returns prior design / ux notes recorded against
// this project so the UXer's screen map and tokens stay coherent across
// runs. Empty string when memory is disabled or empty.
func (e *Engine) contextBundleForUXer(ctx context.Context, projectID string) string {
	if !e.memoryEnabled() {
		return ""
	}
	deadline, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	design, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindProject,
		ProjectID: projectID,
		Tag:       "design",
		Limit:     8,
	})
	ux, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindProject,
		ProjectID: projectID,
		Tag:       "ux",
		Limit:     8,
	})
	merged := append(design, ux...)
	if len(merged) == 0 {
		return ""
	}
	return memory.FormatForContext(merged)
}

// contextBundleForCoder returns a markdown block of the top-N most
// relevant memories for the Coder's current task. Filters by project
// (always), story (when known), and substring (story keywords) so the
// context budget is spent on signal, not on every memory the project
// has ever accumulated. Empty string when memory is disabled or empty.
func (e *Engine) contextBundleForCoder(ctx context.Context, projectID string, story domain.UserStory) string {
	if !e.memoryEnabled() {
		return ""
	}
	// Two passes: per-story execution lineage (highest signal), then
	// general project decisions/patterns. We cap each.
	deadline, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	lineage, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindExecution,
		ProjectID: projectID,
		StoryID:   story.ID,
		Limit:     5,
	})
	decisions, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindProject,
		ProjectID: projectID,
		Tag:       "decision",
		Limit:     5,
	})
	patterns, _ := e.memory.Query(deadline, memory.Query{
		Kind:      memory.KindProject,
		ProjectID: projectID,
		Tag:       "pattern",
		Substring: story.IWant,
		Limit:     3,
	})

	merged := append(lineage, decisions...)
	merged = append(merged, patterns...)
	if len(merged) == 0 {
		return ""
	}
	return memory.FormatForContext(merged)
}
