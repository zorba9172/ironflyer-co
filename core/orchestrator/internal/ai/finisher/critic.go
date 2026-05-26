// Package finisher — Critic stage. Runs a cheap LLM judge against the
// proposed patch BEFORE the more expensive Reviewer simulation. The Critic
// returns structured findings; on a "needs_fixes" verdict we feed the
// findings back into the next Coder attempt instead of paying for the
// Reviewer round-trip. Cheap-first is the budget play.
package finisher

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/patch"
)

type criticOutput struct {
	Verdict  string `json:"verdict"`
	Findings []struct {
		Severity string `json:"severity"`
		Path     string `json:"path"`
		Message  string `json:"message"`
		Hint     string `json:"hint"`
	} `json:"findings"`
}

const criticInstruction = `You are inspecting a single proposed patch. Reply with EXACTLY one JSON object:
{
  "verdict": "clean" | "needs_fixes",
  "findings": [ { "severity": "error" | "warning", "path": "<file>", "message": "...", "hint": "..." } ]
}

Rules:
- Maximum 5 findings. Pick the worst.
- A finding is something a competent engineer would obviously fix once flagged.
- If verdict is "clean", findings must be an empty array.
- No prose, no markdown, no commentary outside the JSON.`

// runCritic invokes the Critic role against the proposed patch and returns
// blocking findings (severity=error). The third return value is false when
// the Critic agent isn't registered, the call failed, or the output was
// unparseable — in those cases the caller should proceed as if the Critic
// did not run, preserving prior behaviour.
func (e *Engine) runCritic(
	ctx context.Context, projectID string, proposed *patch.Patch, story domain.UserStory,
) ([]domain.Issue, *agents.Result, bool) {
	if _, ok := e.registry.Get(agents.RoleCritic); !ok {
		return nil, nil, false
	}

	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepCritic, Agent: string(agents.RoleCritic),
		Status: StatusRunning, Message: "critic_started", CreatedAt: time.Now().UTC(),
	})

	proj, err := e.projects.Get(projectID)
	if err != nil {
		return nil, nil, false
	}

	criticTask := agents.Task{
		Role:        agents.RoleCritic,
		Project:     &proj,
		Goal:        buildCriticGoal(story, proposed),
		UserBearer:  bearerFromCtx(ctx),
		WorkspaceID: workspaceIDFromCtx(ctx),
	}

	// High-stakes decision: race N copies of the Critic and take the
	// majority verdict. Falls back to a single-shot Run when no bucket
	// reaches majority — some signal beats none, and the Reviewer +
	// gates downstream still catch what a split critic missed.
	const voteN = 3
	res, ok, share, err := RunVotedShare(ctx, e.registry, criticTask, VoteOpts{N: voteN, Confidence: 0.5})
	if err != nil {
		// Fail-open: critic is a cost-saving heuristic, not a blocker. A
		// provider error during critic should not stop the pipeline — the
		// Reviewer + gates that follow will catch issues unilaterally.
		return nil, nil, false
	}
	if ok {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepCritic, Agent: string(agents.RoleCritic),
			Status:    StatusDone,
			Message:   "critic_voted n=" + strconv.Itoa(voteN) + " winner_share=" + formatFloat(share),
			CreatedAt: time.Now().UTC(),
		})
	} else {
		res, err = e.registry.Run(ctx, criticTask)
		if err != nil {
			// Same fail-open contract: critic errors do not stop the
			// pipeline.
			return nil, nil, false
		}
	}

	var out criticOutput
	if err := unmarshalJSONFromText(res.Output, &out); err != nil {
		return nil, &res, false
	}

	if strings.EqualFold(out.Verdict, "clean") || len(out.Findings) == 0 {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepCritic, Agent: string(agents.RoleCritic),
			Status:    StatusDone,
			Message:   fmt.Sprintf("critic_clean provider=%s tokens=%d", res.Provider, res.Tokens),
			CreatedAt: time.Now().UTC(),
		})
		return nil, &res, true
	}

	issues := make([]domain.Issue, 0, len(out.Findings))
	for _, f := range out.Findings {
		sev := domain.SeverityError
		if strings.EqualFold(f.Severity, "warning") {
			sev = domain.SeverityWarning
		}
		issues = append(issues, domain.Issue{
			Gate:     domain.GateCode,
			Severity: sev,
			Message:  f.Message,
			Path:     f.Path,
			Hint:     f.Hint,
		})
	}
	// Only blocking-severity findings should stop the patch; warnings get
	// passed back to the Coder as context but don't trigger a retry.
	var blocking []domain.Issue
	for _, iss := range issues {
		if iss.Severity == domain.SeverityError || iss.Severity == domain.SeverityCritical {
			blocking = append(blocking, iss)
		}
	}
	return blocking, &res, true
}

func buildCriticGoal(story domain.UserStory, p *patch.Patch) string {
	var b strings.Builder
	b.WriteString("Critic pass on the proposed patch for story ")
	b.WriteString(story.ID)
	b.WriteString(".\n\n# Story\n")
	b.WriteString("As ")
	b.WriteString(story.As)
	b.WriteString("\nI want ")
	b.WriteString(story.IWant)
	if story.SoThat != "" {
		b.WriteString("\nSo that ")
		b.WriteString(story.SoThat)
	}
	if len(story.Acceptance) > 0 {
		b.WriteString("\nAcceptance:\n")
		for _, a := range story.Acceptance {
			b.WriteString("  - ")
			b.WriteString(a)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n# Patch\n")
	b.WriteString("Title: ")
	b.WriteString(p.Title)
	b.WriteString("\nSummary: ")
	b.WriteString(p.Summary)
	b.WriteString("\nChanges:\n")
	for _, c := range p.Changes {
		fmt.Fprintf(&b, "  --- %s %s ---\n", strings.ToUpper(string(c.Op)), c.Path)
		body := c.Content
		// Cap each file body at 8 KiB in the prompt. The Critic doesn't need
		// to re-derive 50 KB of JSX to flag a missing return — and the prompt
		// budget compounds across stories.
		if len(body) > 8192 {
			body = body[:8192] + "\n…[truncated]"
		}
		b.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(criticInstruction)
	return b.String()
}

// Ensure encoding/json is referenced — keeps the file safe to extend
// without a future "imported and not used" foot-gun.
var _ = json.Unmarshal
