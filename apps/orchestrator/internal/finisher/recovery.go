package finisher

// RecoveryEngine is the auto-repair loop that fires when a gate fails AFTER
// a patch has been applied (i.e. the failure is a regression in the real
// build/test/etc., not a missing plan artefact). The main finisher loop
// emits gate_failed and then hands control here BEFORE marking the run
// failed. Each iteration:
//
//  1. Builds a structured Coder prompt with the last patch ID, the gate
//     name, the issues list, and a truncated stdout/stderr trailer.
//  2. Runs the Coder agent through the same registry+router seam as
//     codeOneStory (so prompt caching + BillingGuard apply transparently).
//  3. Validates the produced patch via patch.Engine.Propose.
//  4. Applies it in-memory and via the workspace RuntimeApplier.
//  5. Re-runs ONLY the failed gate (cheap; uses the gate's Check directly).
//  6. If the gate now passes => done. Otherwise iterate up to MaxAttempts.
//
// Cost guard: the cumulative cost of the Coder calls inside one Recover()
// is summed from the agents.Result.CostUSD that the BillingGuard already
// stamped onto each completion. If the total crosses PerGateBudgetUSD the
// loop aborts early with the recovery_budget error code so the upstream
// budget enforcer doesn't have to know about recovery semantics.
//
// SSE event vocabulary (subscribe to Step == StepRecovery):
//   - "recovery_started"   : status=running, message carries attempt + gate
//   - "recovery_done"      : status=done,    message carries attempt + gate
//   - "recovery_failed"    : status=failed,  one attempt failed but the
//                            loop will keep trying (reason in Message)
//   - "recovery_exhausted" : status=failed,  ErrRecoveryExhausted, all
//                            MaxAttempts attempts spent
//   - "recovery_aborted"   : status=failed,  ErrRecoveryBudget, the
//                            PerGateBudgetUSD cap was hit
//
// Concurrency: Recover runs synchronously on the caller's goroutine and
// honours ctx.Done() at every await. No background goroutines are spawned
// that could outlive the run context.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/patch"
)

// RecoveryConfig tunes the auto-recovery loop. Zero-valued fields fall back
// to the sane defaults so callers can construct {} and get the documented
// behaviour from the spec.
type RecoveryConfig struct {
	// MaxAttempts is the upper bound on Coder iterations per failed gate.
	// Default 2 — recovery is expensive and the marginal value of a third
	// shot is low in practice.
	MaxAttempts int
	// PerGateBudgetUSD caps the cumulative provider cost across one
	// Recover() call. When the running total crosses this number the loop
	// emits ErrRecoveryBudget and returns recovered=false.
	PerGateBudgetUSD float64
	// IncludeStdoutLines truncates the stdout/stderr trailer carried into
	// the Coder prompt. We surface only the tail so the prompt stays under
	// the model's context window even for noisy build failures.
	IncludeStdoutLines int
}

const (
	defaultRecoveryMaxAttempts        = 2
	defaultRecoveryPerGateBudgetUSD   = 0.50
	defaultRecoveryIncludeStdoutLines = 80
)

func (c RecoveryConfig) withDefaults() RecoveryConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultRecoveryMaxAttempts
	}
	if c.PerGateBudgetUSD <= 0 {
		c.PerGateBudgetUSD = defaultRecoveryPerGateBudgetUSD
	}
	if c.IncludeStdoutLines <= 0 {
		c.IncludeStdoutLines = defaultRecoveryIncludeStdoutLines
	}
	return c
}

// GateFailure is the structured snapshot of why a gate just failed, threaded
// from the gate-check loop into the recovery engine.
type GateFailure struct {
	Gate        domain.GateName
	Issues      []domain.Issue
	Stdout      string
	Stderr      string
	LastPatchID string
}

// RecoveryEngine is the auto-repair driver. It holds a back-reference to
// the parent finisher.Engine so it can reuse the projects store, agent
// registry, patch engine, runtime applier, and the SSE emit fan-out.
type RecoveryEngine struct {
	engine *Engine
	cfg    RecoveryConfig
}

// NewRecoveryEngine constructs a RecoveryEngine bound to a finisher.Engine.
// Pass RecoveryConfig{} for the documented defaults.
func NewRecoveryEngine(e *Engine, cfg RecoveryConfig) *RecoveryEngine {
	return &RecoveryEngine{engine: e, cfg: cfg.withDefaults()}
}

// Recover is the public entry-point. It returns (true, nil) when the
// failed gate now passes after one of the recovery attempts; (false, nil)
// when the loop ran cleanly but couldn't fix the failure; and (false, err)
// only on hard infrastructure errors (project lookup failure, context
// cancelled, etc.) so the caller can treat err == nil as "loop ran".
func (r *RecoveryEngine) Recover(
	ctx context.Context,
	projectID, workspaceID, bearer string,
	failure GateFailure,
	report *RunReport,
) (bool, error) {
	if r == nil || r.engine == nil {
		return false, errors.New("recovery: engine not configured")
	}
	gate, ok := r.findGate(failure.Gate)
	if !ok {
		// We can't re-run a gate we don't know about; emit a structured
		// failure event so the UI sees the dead-end and bail.
		r.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRecovery, Gate: failure.Gate,
			Status:    StatusFailed,
			Message:   fmtErr(ErrRecoveryExhausted, "no registered gate for "+string(failure.Gate)),
			CreatedAt: time.Now().UTC(),
		})
		return false, nil
	}

	var spentUSD float64
	for attempt := 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		r.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRecovery, Gate: failure.Gate,
			Status:    StatusRunning,
			Message:   fmt.Sprintf("recovery_started attempt=%d gate=%s", attempt, failure.Gate),
			CreatedAt: time.Now().UTC(),
		})

		// ---- 1. fetch fresh project + build the Coder prompt -----------------
		proj, err := r.engine.projects.Get(projectID)
		if err != nil {
			return false, err
		}
		goal := r.buildRecoveryGoal(proj, failure, attempt)

		// ---- 2. ask the Coder via the registry router -----------------------
		res, runErr := r.engine.registry.Run(ctx, agents.Task{
			Role:    agents.RoleCoder,
			Project: &proj,
			Goal:    goal,
			Issues:  failure.Issues,
		})
		if runErr != nil {
			r.engine.emitProviderErr(projectID, StepRecovery, agents.RoleCoder, runErr)
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			// Fall through to the next attempt — provider hiccups happen.
			continue
		}
		report.AgentRuns = append(report.AgentRuns, res)
		spentUSD += res.CostUSD

		if spentUSD > r.cfg.PerGateBudgetUSD {
			r.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRecovery, Gate: failure.Gate,
				Status: StatusFailed,
				Message: fmtErr(ErrRecoveryBudget,
					fmt.Sprintf("recovery_aborted reason=budget spent=%.4fUSD cap=%.4fUSD gate=%s",
						spentUSD, r.cfg.PerGateBudgetUSD, failure.Gate)),
				CreatedAt: time.Now().UTC(),
			})
			return false, nil
		}

		// ---- 3. parse + validate the patch ----------------------------------
		var cp coderPatch
		if err := unmarshalJSONFromText(res.Output, &cp); err != nil {
			r.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRecovery, Gate: failure.Gate, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchInvalid, fmt.Sprintf("recovery_failed reason=patch_invalid attempt=%d parse=%s", attempt, err.Error())),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}
		built := patch.Patch{
			ProjectID: projectID,
			Author:    string(agents.RoleCoder) + "/recovery",
			Title:     cp.Title,
			Summary:   cp.Summary,
		}
		for _, c := range cp.Changes {
			built.Changes = append(built.Changes, patch.FileChange{
				Op:      patch.Op(strings.ToLower(strings.TrimSpace(c.Op))),
				Path:    strings.TrimPrefix(c.Path, "/"),
				Content: c.Content,
			})
		}
		if violations := r.engine.enforcePatchBounds(built); len(violations) > 0 {
			r.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRecovery, Gate: failure.Gate, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchTooLarge, fmt.Sprintf("recovery_failed reason=patch_invalid attempt=%d bounds=%s", attempt, joinIssues(violations))),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}

		proposed, err := r.engine.patches.Propose(built)
		if err != nil || proposed.Status == patch.StatusRejected {
			reason := "validator_rejected"
			if err != nil {
				reason = err.Error()
			} else if len(proposed.Issues) > 0 {
				reason = joinIssues(proposed.Issues)
			}
			r.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRecovery, Gate: failure.Gate, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchInvalid, fmt.Sprintf("recovery_failed reason=patch_invalid attempt=%d %s", attempt, reason)),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}
		report.PatchIDs = append(report.PatchIDs, proposed.ID)

		// ---- 4. apply: in-memory always, runtime applier if available -------
		applied, applyErr := r.engine.patches.Apply(proposed.ID)
		if applyErr != nil {
			r.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRecovery, Gate: failure.Gate, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchInvalid, fmt.Sprintf("recovery_failed reason=apply attempt=%d %s", attempt, applyErr.Error())),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}
		if workspaceID != "" && r.engine.applier != nil {
			if err := r.engine.applier.Apply(ctx, bearer, workspaceID, applied); err != nil {
				if ctx.Err() != nil {
					return false, ctx.Err()
				}
				r.emit(projectID, domain.Event{
					ID: newEventID(), Step: StepRecovery, Gate: failure.Gate, Agent: string(agents.RoleCoder),
					Status:    StatusFailed,
					Message:   fmtErr(ErrCodeRuntimeError, fmt.Sprintf("recovery_failed reason=workspace_apply attempt=%d %s", attempt, err.Error())),
					CreatedAt: time.Now().UTC(),
				})
				// Re-running the gate against an out-of-sync workspace is
				// pointless — the gate sees stale bytes. Skip to next attempt.
				continue
			}
		}

		// ---- 5. re-run ONLY the failed gate ---------------------------------
		freshProj, err := r.engine.projects.Get(projectID)
		if err != nil {
			return false, err
		}
		env := &GateEnv{
			Project:     &freshProj,
			Runtime:     r.engine.runtime,
			WorkspaceID: workspaceID,
			UserBearer:  bearer,
		}
		newIssues := gate.Check(ctx, env)
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if len(newIssues) == 0 {
			// Persist gate state so the outer iteration sees the recovery.
			r.engine.setGate(projectID, gate.Name(), domain.GateState{
				Name:      gate.Name(),
				Status:    domain.GateStatusRepaired,
				UpdatedAt: time.Now().UTC(),
			})
			r.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRecovery, Gate: failure.Gate, Agent: string(agents.RoleCoder),
				Status:    StatusDone,
				Message:   fmt.Sprintf("recovery_done attempt=%d gate=%s", attempt, failure.Gate),
				CreatedAt: time.Now().UTC(),
			})
			return true, nil
		}

		// Gate still failing — refresh failure context for the next attempt.
		failure.Issues = newIssues
		failure.LastPatchID = proposed.ID
		// We don't currently re-capture stdout/stderr because the gate's
		// own Run path already surfaces fresh output through gate_failed
		// hints — re-fetching it here would duplicate the exec.
		r.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRecovery, Gate: failure.Gate, Agent: string(agents.RoleCoder),
			Status:    StatusFailed,
			Message:   fmt.Sprintf("recovery_failed attempt=%d gate=%s issues=%d", attempt, failure.Gate, len(newIssues)),
			CreatedAt: time.Now().UTC(),
		})
	}

	// All MaxAttempts exhausted without a green gate.
	r.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepRecovery, Gate: failure.Gate,
		Status:    StatusFailed,
		Message:   fmtErr(ErrRecoveryExhausted, fmt.Sprintf("recovery_exhausted attempts=%d gate=%s", r.cfg.MaxAttempts, failure.Gate)),
		CreatedAt: time.Now().UTC(),
	})
	return false, nil
}

// findGate looks up a registered gate by name on the parent engine.
func (r *RecoveryEngine) findGate(name domain.GateName) (Gate, bool) {
	for _, g := range r.engine.gates {
		if g.Name() == name {
			return g, true
		}
	}
	return nil, false
}

// emit thin-wraps the engine's emit so recovery has a single chokepoint
// for future telemetry (e.g. recovery attempt counters).
func (r *RecoveryEngine) emit(projectID string, evt domain.Event) {
	r.engine.emit(projectID, evt)
}

const recoveryInstruction = `You are repairing a regression. A previous patch was applied and a downstream gate is now failing. Produce a NEW unified change set that fixes the failure. Do NOT repeat the previous diff verbatim.

Reply with a SINGLE JSON object — no prose, no markdown fence — matching exactly:

{
  "title":   "<short imperative summary of the fix>",
  "summary": "<one paragraph explaining what was wrong and how this fixes it>",
  "changes": [
    { "op": "create" | "update" | "delete", "path": "<relative path>", "content": "<full file contents>" }
  ]
}

Rules of thumb for diagnosing failures:
- Missing module / "cannot find package" / "Cannot find module 'X'" => add X to package.json or go.mod with an appropriate version.
- Type errors / "is not assignable to" / "undefined: Foo" => fix the offending type or import, do not @ts-ignore it.
- Test failures => fix the production code so the test passes, do not weaken the assertion.
- Lint failures => fix the underlying code; do not silence the linter.
- Build OOM / timeout => split the change or simplify the implementation.
- Security gate hit => remove the credential or unsafe call entirely, never just rename it.

Output nothing but the JSON object.`

// buildRecoveryGoal renders the structured prompt the Coder sees when it's
// invoked from the recovery engine. We attach: the gate name, the issue
// list rendered as bullet points, a tail of stdout/stderr, the last
// applied patch ID, and the static instruction block above.
func (r *RecoveryEngine) buildRecoveryGoal(p domain.Project, f GateFailure, attempt int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Gate '%s' failed on attempt %d/%d after the last patch (id=%s) was applied.\n\n",
		f.Gate, attempt, r.cfg.MaxAttempts, f.LastPatchID)

	if len(f.Issues) > 0 {
		b.WriteString("# Failing issues\n")
		for _, iss := range f.Issues {
			line := "- [" + string(iss.Severity) + "] " + iss.Message
			if iss.Path != "" {
				line += " (" + iss.Path + ")"
			}
			if iss.Hint != "" {
				line += "  // hint: " + iss.Hint
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	if tailOut := tailLines(f.Stdout, r.cfg.IncludeStdoutLines); tailOut != "" {
		b.WriteString("# stdout (last " + itoaPositive(r.cfg.IncludeStdoutLines) + " lines)\n")
		b.WriteString("```\n" + tailOut + "\n```\n\n")
	}
	if tailErr := tailLines(f.Stderr, r.cfg.IncludeStdoutLines); tailErr != "" {
		b.WriteString("# stderr (last " + itoaPositive(r.cfg.IncludeStdoutLines) + " lines)\n")
		b.WriteString("```\n" + tailErr + "\n```\n\n")
	}

	// Surface the previous patch summary so the Coder can avoid repeating it.
	if f.LastPatchID != "" {
		if prev, ok := r.lastPatchSummary(f.LastPatchID); ok {
			b.WriteString("# Previous patch (do not repeat verbatim)\n")
			b.WriteString(prev + "\n\n")
		}
	}

	b.WriteString(recoveryInstruction)
	return b.String()
}

// lastPatchSummary fetches a short JSON-ish snippet describing the last
// applied patch so the Coder knows what it just tried. We deliberately
// don't dump full change content — that's already in the project tree
// the Coder is about to look at.
func (r *RecoveryEngine) lastPatchSummary(id string) (string, bool) {
	// patch.Engine has no public Get; we synthesize the summary from
	// RunReport state instead. The caller has the ID + the engine; we
	// json-encode a tiny descriptor so prompt parsing stays uniform.
	type prevDesc struct {
		ID      string `json:"id"`
		Note    string `json:"note"`
	}
	raw, err := json.Marshal(prevDesc{
		ID:   id,
		Note: "previous Coder patch ID — do not re-emit identical changes",
	})
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// tailLines returns the last n newline-delimited lines of s. Trailing
// whitespace is trimmed first so a final newline doesn't eat a real line.
func tailLines(s string, n int) string {
	s = strings.TrimRight(s, "\n\r\t ")
	if s == "" || n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
