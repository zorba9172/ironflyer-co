// Package finisher — post-patch reflection. After a story's patch
// is applied AND the workspace is updated, a Reflector agent reads
// the original story acceptance criteria, the patch summary, and a
// light snapshot of the touched files, then emits a structured
// verdict. The verdict feeds back into the memory store so future
// runs see "we shipped this once but the reflector flagged X".
//
// The reflector is opt-out (skipped silently when memory is
// disabled or the registry is nil) so it never blocks the loop.
//
// Cost: one CapCheap agent call per applied patch — should land on
// Haiku / gpt-4o-mini / Qwen-7B per the router's routing policy.
package finisher

import (
	"context"
	"errors"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/memory"
	"ironflyer/apps/orchestrator/internal/patch"
)

// Verdict is the structured verdict the Reflector emits about a
// just-applied patch. Three buckets, deliberately coarse so the
// downstream memory store and audit log can aggregate without
// parsing free-form text.
type Verdict string

const (
	VerdictAccomplished Verdict = "accomplished"
	VerdictPartial      Verdict = "partial"
	VerdictDrift        Verdict = "drift"
)

// ReflectionResult is the parsed output of one Reflector call.
type ReflectionResult struct {
	Verdict         Verdict
	Notes           string   // 1-3 sentences of why
	MissingCriteria []string // acceptance items the reflector thinks weren't covered
}

// reflectorOutput is the on-the-wire shape we ask the Reflector to
// emit. Kept tiny on purpose — the reflector is the cheapest agent
// in the loop and its job is to classify, not to argue.
type reflectorOutput struct {
	Verdict         string   `json:"verdict"`
	Notes           string   `json:"notes"`
	MissingCriteria []string `json:"missingCriteria"`
}

const reflectionInstruction = `You are auditing a code patch that was just applied for a single user story. Reply with EXACTLY one JSON object — no prose, no markdown fence:

{
  "verdict": "accomplished" | "partial" | "drift",
  "notes": "1-3 sentences explaining the verdict",
  "missingCriteria": ["<verbatim acceptance text>", "..."]
}

Rules:
- "accomplished" — every acceptance criterion is plausibly satisfied by the patch.
- "partial"      — the patch advanced the story but missed at least one criterion.
- "drift"        — the patch did something materially different from what the story asked for, or introduced churn unrelated to the acceptance criteria.
- missingCriteria MUST be empty when verdict is "accomplished".
- missingCriteria entries quote the original acceptance text; do not invent new criteria.
- Output nothing but the JSON object.`

// reflectOnPatch dispatches the Reflector agent and persists the
// verdict + notes as Execution memory. The reflector is best-effort:
// any error short-circuits the function but never bubbles up to the
// caller as a story failure.
func (e *Engine) reflectOnPatch(
	ctx context.Context,
	projectID string,
	story domain.UserStory,
	applied patch.Patch,
) (ReflectionResult, error) {
	if e == nil || e.registry == nil {
		return ReflectionResult{}, errors.New("reflection: nil engine or registry")
	}
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return ReflectionResult{}, err
	}

	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepCritic, Agent: string(agents.RoleReviewer),
		Status: StatusRunning, Message: "reflection_started story=" + story.ID,
		CreatedAt: time.Now().UTC(),
	})

	prompt := buildReflectionPrompt(story, applied)
	task := agents.Task{
		Role:        agents.RoleReviewer,
		Project:     &proj,
		Goal:        prompt,
		UserBearer:  bearerFromCtx(ctx),
		WorkspaceID: workspaceIDFromCtx(ctx),
	}
	res, runErr := e.registry.Run(ctx, task)
	if runErr != nil {
		return ReflectionResult{}, runErr
	}

	var raw reflectorOutput
	if err := unmarshalJSONFromText(res.Output, &raw); err != nil {
		return ReflectionResult{}, err
	}

	out := ReflectionResult{
		Verdict:         normalizeVerdict(raw.Verdict),
		Notes:           strings.TrimSpace(raw.Notes),
		MissingCriteria: trimStrings(raw.MissingCriteria),
	}

	// Persist drift / partial verdicts as Execution memory so the next
	// run on the same story sees "we shipped this once but the
	// reflector flagged X" and can compensate. Clean "accomplished"
	// verdicts are not worth a memory row — the existing pattern
	// capture in rememberCoderPatch already covers the happy path.
	if e.memoryEnabled() && (out.Verdict == VerdictDrift || out.Verdict == VerdictPartial) {
		var b strings.Builder
		b.WriteString("Reflector verdict on story `" + story.ID + "`: " + string(out.Verdict) + "\n")
		if out.Notes != "" {
			b.WriteString("\n" + out.Notes + "\n")
		}
		if len(out.MissingCriteria) > 0 {
			b.WriteString("\nUnsatisfied acceptance:\n")
			for i, m := range out.MissingCriteria {
				if i >= 5 {
					b.WriteString("- … and " + itoaPositive(len(out.MissingCriteria)-5) + " more\n")
					break
				}
				b.WriteString("- " + tail(m, 200) + "\n")
			}
		}
		b.WriteString("\nPatch `" + applied.ID + "` (" + applied.Title + ") landed but did not fully satisfy the story.")
		_, _ = e.memory.Record(ctx, memory.Record{
			Kind:       memory.KindExecution,
			ProjectID:  projectID,
			StoryID:    story.ID,
			Title:      "reflection: " + string(out.Verdict) + " — " + applied.Title,
			Body:       b.String(),
			Tags:       []string{"reflection", string(out.Verdict)},
			Confidence: 0.7,
		})
	}

	// Audit trail entry — one row per reflector dispatch so the
	// production-trust moat has a record of what the cheap judge said
	// about every applied patch. Outcome is success/failure/blocked
	// keyed on verdict so dashboards can aggregate without parsing
	// free-form text.
	outcome := audit.OutcomeSuccess
	switch out.Verdict {
	case VerdictPartial:
		outcome = audit.OutcomeBlocked
	case VerdictDrift:
		outcome = audit.OutcomeFailure
	}
	e.recordAudit(ctx, audit.Entry{
		Action:    audit.ActionAgentDispatch,
		Outcome:   outcome,
		UserID:    proj.OwnerID,
		ProjectID: projectID,
		StoryID:   story.ID,
		AgentRole: string(agents.RoleReviewer),
		Summary:   "reflection verdict=" + string(out.Verdict) + " story=" + story.ID + " patch=" + applied.ID,
		Attrs: map[string]any{
			"verdict":         string(out.Verdict),
			"missingCriteria": len(out.MissingCriteria),
			"provider":        res.Provider,
			"tokens":          res.Tokens,
		},
	})

	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepCritic, Agent: string(agents.RoleReviewer),
		Status:    StatusDone,
		Message:   "reflection verdict=" + string(out.Verdict) + " story=" + story.ID,
		CreatedAt: time.Now().UTC(),
	})
	return out, nil
}

// normalizeVerdict maps a free-text verdict string onto the closed
// set defined above. Unknown values collapse to "drift" — the
// pessimistic bucket — so a malformed reflector reply errs on the
// side of capturing memory rather than silently swallowing it.
func normalizeVerdict(s string) Verdict {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "accomplished", "done", "complete", "completed", "ok", "pass":
		return VerdictAccomplished
	case "partial", "incomplete":
		return VerdictPartial
	case "drift", "off", "wrong", "miss":
		return VerdictDrift
	default:
		return VerdictDrift
	}
}

// trimStrings strips whitespace and drops empty entries from a slice.
// Used to clean missingCriteria the model occasionally emits with
// stray padding or blank rows.
func trimStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// buildReflectionPrompt assembles the tight prompt the Reflector
// reads. Story summary + acceptance + applied.Title + applied.Summary
// + the first 1500 chars of each file change. Total capped at ~6000
// chars so the cheap judge stays cheap.
func buildReflectionPrompt(story domain.UserStory, applied patch.Patch) string {
	const (
		perFileCap  = 1500
		totalCap    = 6000
	)
	var b strings.Builder
	b.WriteString("Reflection pass on the just-applied patch for story ")
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
	b.WriteString("\n# Applied patch\n")
	b.WriteString("Title: ")
	b.WriteString(applied.Title)
	b.WriteString("\nSummary: ")
	b.WriteString(applied.Summary)
	b.WriteString("\nChanges:\n")
	for _, c := range applied.Changes {
		// Stop appending more file bodies once we're near the cap; we
		// still want the JSON instruction to land.
		if b.Len() > totalCap-len(reflectionInstruction)-512 {
			b.WriteString("  …[remaining changes truncated]\n")
			break
		}
		b.WriteString("  --- ")
		b.WriteString(strings.ToUpper(string(c.Op)))
		b.WriteString(" ")
		b.WriteString(c.Path)
		b.WriteString(" ---\n")
		body := c.Content
		if body == "" {
			// For replace / insert_after the new text lives in Replacement.
			body = c.Replacement
		}
		if len(body) > perFileCap {
			body = body[:perFileCap] + "\n…[truncated]"
		}
		b.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n")
	b.WriteString(reflectionInstruction)
	return b.String()
}
