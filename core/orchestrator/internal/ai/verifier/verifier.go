// Package verifier orchestrates the live-preview proof loop the
// finisher's VerifierGate sits on top of. The flow is:
//
//  1. For each AcceptanceCriterion the project carries, ask the
//     Verifier agent to plan the minimum Playwright actions that
//     would prove the criterion (vision + DOM).
//  2. Materialise that plan into a Playwright TypeScript script
//     inside the workspace sandbox.
//  3. Drive `npx playwright test --reporter=json` against the
//     live preview URL.
//  4. Capture the verdict: pass / fail / warn, attach the agent's
//     `failure_reason` as the gate Issue hint.
//
// The package is deliberately thin — it owns orchestration, not
// transport. All command execution lives in the workspace via
// runtime.Exec; the orchestrator process never shells out directly.
//
// Degradation: when there is no runtime, no preview URL, or no
// AcceptanceCriteria yet, Run returns nil issues and lets the gate
// short-circuit. Verification is additive signal, not a load-bearing
// gate for projects too early to have routes.
package verifier

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/logctx"
	"ironflyer/core/orchestrator/internal/operations/runtime"
)

// RunInput is everything Verify needs to evaluate one criterion. Kept
// as a struct so the caller can populate it from a GateEnv without
// the package importing finisher (which would cycle).
type RunInput struct {
	Project     *domain.Project
	Runtime     *runtime.Client
	UserBearer  string
	WorkspaceID string
	PreviewURL  string
	Registry    *agents.Registry
}

// CriterionResult is what one criterion's pass through the verifier
// emits. Verdict is exactly the agent's verdict so the gate can map
// it straight onto a domain.Severity.
type CriterionResult struct {
	CriterionID   string    `json:"criterionId"`
	Verdict       string    `json:"verdict"`
	Evidence      string    `json:"evidence,omitempty"`
	FailureReason string    `json:"failureReason,omitempty"`
	RouteHit      string    `json:"routeHit,omitempty"`
	StartedAt     time.Time `json:"startedAt"`
	FinishedAt    time.Time `json:"finishedAt"`
	// Skipped is true only for intentionally non-exercised criteria.
	// Agent planning failures must stay warn/fail so an evasive or
	// malformed verifier response cannot pass the gate.
	Skipped bool `json:"skipped,omitempty"`
}

// Run drives the verifier loop across every AcceptanceCriterion the
// project knows about and returns one CriterionResult per criterion.
// Best-effort: a per-criterion failure does NOT abort the whole loop
// — every criterion gets a chance so the operator sees the full
// picture in one iteration. The caller (finisher gate) decides how to
// project results onto domain.Issue.
func Run(ctx context.Context, in RunInput) ([]CriterionResult, error) {
	if in.Project == nil {
		return nil, errors.New("verifier: project required")
	}
	if in.Registry == nil {
		return nil, errors.New("verifier: agent registry required")
	}
	lg := logctx.From(ctx)
	criteria := collectCriteria(in.Project)
	if len(criteria) == 0 {
		lg.Info().Msg("verifier: project has no acceptance criteria — skipping")
		return nil, nil
	}
	if in.Runtime == nil || !in.Runtime.Enabled() || in.WorkspaceID == "" {
		lg.Info().Msg("verifier: no runtime/workspace — skipping (gate degrades)")
		return nil, nil
	}
	if strings.TrimSpace(in.PreviewURL) == "" {
		lg.Info().Msg("verifier: no preview URL — skipping (gate degrades)")
		return nil, nil
	}

	// Ensure chromium is installed in the workspace, once per workspace.
	// A failure here is a SeverityWarning at the gate layer; we don't
	// abort the whole run because the operator may have pre-baked the
	// image with chromium already present.
	if err := EnsurePlaywright(ctx, in.Runtime, in.UserBearer, in.WorkspaceID, lg); err != nil {
		lg.Warn().Err(err).Msg("verifier: playwright install failed; will try to run anyway")
	}

	results := make([]CriterionResult, 0, len(criteria))
	for _, c := range criteria {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}
		res := verifyOne(ctx, in, c, lg)
		results = append(results, res)
	}
	return results, nil
}

// collectCriteria flattens project user-stories into a stable list of
// criteria the verifier can iterate. We synthesise an ID when the
// criterion text is just a freeform string so per-criterion results
// remain joinable across runs.
func collectCriteria(p *domain.Project) []domain.AcceptanceCriterion {
	if p == nil {
		return nil
	}
	out := make([]domain.AcceptanceCriterion, 0, 16)
	for _, story := range p.Spec.UserStories {
		for i, line := range story.Acceptance {
			text := strings.TrimSpace(line)
			if text == "" {
				continue
			}
			out = append(out, domain.AcceptanceCriterion{
				ID:          story.ID + "#" + itoa(i),
				StoryID:     story.ID,
				Description: text,
			})
		}
	}
	return out
}

// verifyOne handles a single criterion: plan via agent, execute via
// Playwright, return a structured result. Errors collapse to a
// "warn" verdict so the gate still sees a row.
func verifyOne(ctx context.Context, in RunInput, c domain.AcceptanceCriterion, lg zerolog.Logger) CriterionResult {
	started := time.Now().UTC()
	res := CriterionResult{
		CriterionID: c.ID,
		StartedAt:   started,
		Verdict:     "warn",
	}
	plan, err := planCriterion(ctx, in, c)
	if err != nil {
		res.FailureReason = "verifier agent failed: " + err.Error()
		res.FinishedAt = time.Now().UTC()
		lg.Warn().Err(err).Str("criterion_id", c.ID).Msg("verifier: agent plan failed")
		return res
	}
	if plan == nil || len(plan.Actions) == 0 {
		res.Verdict = "warn"
		res.FailureReason = "verifier agent returned no actions"
		res.FinishedAt = time.Now().UTC()
		return res
	}
	res.RouteHit = firstGotoURL(plan.Actions, in.PreviewURL)

	exec, err := RunPlaywright(ctx, in.Runtime, in.UserBearer, in.WorkspaceID, in.PreviewURL, c.ID, plan.Actions)
	if err != nil {
		res.FailureReason = "playwright exec failed: " + err.Error()
		res.FinishedAt = time.Now().UTC()
		lg.Warn().Err(err).Str("criterion_id", c.ID).Msg("verifier: playwright failed")
		return res
	}
	if !exec.Success {
		res.Verdict = "fail"
		// The agent's own failure_reason wins; the Playwright tail is
		// the next-best evidence.
		if plan.FailureReason != "" {
			res.FailureReason = plan.FailureReason
		} else {
			res.FailureReason = tail(exec.Stderr+exec.Stdout, 400)
		}
		res.Evidence = plan.Evidence
		res.FinishedAt = time.Now().UTC()
		return res
	}
	// Playwright succeeded. We still honour the agent's verdict so a
	// case where the screenshot rendered cleanly but the agent decided
	// "warn" surfaces as a warn.
	if plan.Verdict == "" {
		res.Verdict = "pass"
	} else {
		res.Verdict = plan.Verdict
	}
	res.Evidence = plan.Evidence
	res.FailureReason = plan.FailureReason
	res.FinishedAt = time.Now().UTC()
	return res
}

func firstGotoURL(actions []Action, fallback string) string {
	for _, a := range actions {
		if strings.EqualFold(a.Type, "goto") && strings.TrimSpace(a.URL) != "" {
			return a.URL
		}
	}
	return fallback
}

// AgentPlan is the structured verifier-agent response. Mirrors the
// JSON contract documented in agents.yaml under role: verifier.
type AgentPlan struct {
	Verdict       string   `json:"verdict"`
	Evidence      string   `json:"evidence,omitempty"`
	FailureReason string   `json:"failure_reason,omitempty"`
	Actions       []Action `json:"actions,omitempty"`
}

// Action is one Playwright instruction the agent emitted.
type Action struct {
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
	Selector string `json:"selector,omitempty"`
	Value    string `json:"value,omitempty"`
	Name     string `json:"name,omitempty"`
}

// planCriterion asks the Verifier agent for a JSON plan. We tolerate
// a model that wraps the JSON in a code fence; the orchestrator's
// reviewer agent has the same tolerance.
func planCriterion(ctx context.Context, in RunInput, c domain.AcceptanceCriterion) (*AgentPlan, error) {
	goal := "Verify the following acceptance criterion is observably satisfied at the live preview.\n" +
		"Criterion: " + c.Description + "\n" +
		"Preview base URL: " + in.PreviewURL + "\n" +
		"Story ID: " + c.StoryID + "\n" +
		"Return ONLY the JSON object documented in your system prompt."
	task := agents.Task{
		Role:        agents.RoleVerifier,
		Project:     in.Project,
		Goal:        goal,
		UserBearer:  in.UserBearer,
		WorkspaceID: in.WorkspaceID,
	}
	res, err := in.Registry.Run(ctx, task)
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(res.Output)
	raw = stripFences(raw)
	if raw == "" {
		return nil, errors.New("verifier agent returned empty output")
	}
	var plan AgentPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, errors.New("verifier agent output not JSON: " + err.Error())
	}
	return &plan, nil
}

// stripFences removes optional ```json ... ``` wrappers — the system
// prompt forbids them but real-world models sometimes regress.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
