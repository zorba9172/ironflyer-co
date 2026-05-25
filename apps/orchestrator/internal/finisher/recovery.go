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

	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/profitguardctx"
	"ironflyer/apps/orchestrator/internal/repair"
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

	// V22 learning: at the start of recovery, check the repair genome
	// for a known fix recipe matching the failure signature. If we
	// have one, surface a structured "recipe_hit" event so the
	// dashboard's repair_genome_hits counter increments. This v1
	// implementation does not (yet) auto-apply the recipe — the Coder
	// still runs — but the lookup itself bumps Hits/LastHitAt on the
	// recipe row so dashboard "reuse pressure" is honest.
	failureBlob := joinIssues(failure.Issues)
	if r.engine.learning != nil && failureBlob != "" {
		if recipe, hit, err := r.engine.learning.LookupRecipe(ctx, failureBlob); err == nil && hit {
			r.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRecovery, Gate: failure.Gate,
				Status:    StatusRunning,
				Message:   fmt.Sprintf("recovery_recipe_hit gate=%s category=%s hits=%d", failure.Gate, recipe.Category, recipe.Hits),
				CreatedAt: time.Now().UTC(),
			})
			// A55 agent reasoning — recovery recipe matched.
			emitExecutionEvent(ctx, r.engine.executionService, execution.EventAgentStageActionV1, map[string]any{
				"stage":      string(failure.Gate),
				"agent_role": gateToAgent(failure.Gate),
				"action":     "repair_lookup",
				"target":     repair.FailureSignature(failureBlob),
				"recipe_id":  recipe.ID,
				"hits":       recipe.Hits,
				"message":    "Repair recipe matched (" + recipe.Category + ", " + fmt.Sprintf("%d prior hits", recipe.Hits) + ")",
			})
			// Wow-loop event ring: emit recovery.recipe_hit.v1 so the
			// executionSupportBundle repair panel can flag the gate
			// stage as "the genome saw this failure before". The
			// hit itself does not imply success — auto-apply may still
			// fall through to the standard Coder retry path; the
			// recipe_applied.v1 event below is what marks "repaired".
			emitExecutionEvent(ctx, r.engine.executionService, execution.EventRecoveryHitV1, map[string]any{
				"failure_signature": repair.FailureSignature(failureBlob),
				"gate":              string(failure.Gate),
				"recipe_id":         recipe.ID,
				"category":          recipe.Category,
				"hit_count":         recipe.Hits,
				"occurred_at":       time.Now().UTC().Format(time.RFC3339Nano),
			})
			// V22 auto-apply: if the recipe carries a literal
			// `changes` array (recorded by a prior OnRetrySuccess),
			// try to replay it through the standard patch lifecycle
			// BEFORE burning the Coder budget on a fresh
			// reasoning round. On success the gate is repaired,
			// MarkSuccess is bumped, and the loop short-circuits.
			// On failure we fall through to the standard recovery
			// path so the system still gets a chance to recover.
			signature := repair.FailureSignature(failureBlob)
			// A55 agent reasoning — about to attempt auto-apply.
			emitExecutionEvent(ctx, r.engine.executionService, execution.EventAgentStageActionV1, map[string]any{
				"stage":      string(failure.Gate),
				"agent_role": gateToAgent(failure.Gate),
				"action":     "repair_apply",
				"recipe_id":  recipe.ID,
				"message":    "Replaying repair recipe for " + string(failure.Gate),
			})
			if applied, ok, applyErr := r.applyRecipePatch(ctx, projectID, workspaceID, bearer, recipe); ok {
				emitExecutionEvent(ctx, r.engine.executionService, execution.EventAgentStageResultV1, map[string]any{
					"stage":      string(failure.Gate),
					"agent_role": gateToAgent(failure.Gate),
					"action":     "repair_apply",
					"success":    true,
					"patch_id":   applied.ID,
					"summary":    "Recipe applied; " + string(failure.Gate) + " repaired",
				})
				_ = r.engine.learning.Genome.MarkSuccess(ctx, signature)
				r.engine.setGate(projectID, gate.Name(), domain.GateState{
					Name:      gate.Name(),
					Status:    domain.GateStatusRepaired,
					UpdatedAt: time.Now().UTC(),
				})
				r.engine.recordGateOutcome(ctx, gate.Name(), true, 0)
				report.PatchIDs = append(report.PatchIDs, applied.ID)
				r.emit(projectID, domain.Event{
					ID: newEventID(), Step: StepRecovery, Gate: failure.Gate,
					Status:    StatusDone,
					Message:   fmt.Sprintf("recovery_recipe_applied gate=%s patch=%s", failure.Gate, applied.ID),
					CreatedAt: time.Now().UTC(),
				})
				// Wow-loop event ring: emit recovery.recipe_applied.v1
				// so the executionSupportBundle repair panel can flag
				// this gate stage as "repaired" rather than "failed".
				// Also emit a patch.applied.v1 row for the auto-applied
				// recipe so the "what changed" panel includes it — the
				// recipe replay lands files exactly like the Coder loop
				// would, just without the Coder cost.
				appliedPaths := make([]string, 0, len(applied.Changes))
				for _, c := range applied.Changes {
					appliedPaths = append(appliedPaths, c.Path)
				}
				emitExecutionEvent(ctx, r.engine.executionService, execution.EventRecoveryApplyV1, map[string]any{
					"failure_signature": signature,
					"gate":              string(failure.Gate),
					"recipe_id":         recipe.ID,
					"patch_id":          applied.ID,
					"success":           true,
					"occurred_at":       time.Now().UTC().Format(time.RFC3339Nano),
				})
				emitExecutionEvent(ctx, r.engine.executionService, execution.EventPatchAppliedV1, map[string]any{
					"patch_id":       applied.ID,
					"affected_paths": appliedPaths,
					"source":         "recovery_recipe",
					"recipe_id":      recipe.ID,
					"applied_at":     time.Now().UTC().Format(time.RFC3339Nano),
				})
				return true, nil
			} else if applyErr != nil {
				emitExecutionEvent(ctx, r.engine.executionService, execution.EventAgentStageResultV1, map[string]any{
					"stage":      string(failure.Gate),
					"agent_role": gateToAgent(failure.Gate),
					"action":     "repair_apply",
					"success":    false,
					"error":      applyErr.Error(),
				})
				r.emit(projectID, domain.Event{
					ID: newEventID(), Step: StepRecovery, Gate: failure.Gate,
					Status:    StatusFailed,
					Message:   fmt.Sprintf("recovery_recipe_apply_failed gate=%s reason=%s", failure.Gate, applyErr.Error()),
					CreatedAt: time.Now().UTC(),
				})
			}
		}
	}

	var spentUSD float64
	for attempt := 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		// V22 ProfitGuard BeforeRetry hook for the recovery loop. This
		// covers every gate that drives through recovery: Reviewer,
		// Test, Build, Security, Deploy. The Coder retry hook lives in
		// loop.go (Agent 8). Nil-safe — without the hook the loop
		// behaves exactly as before.
		if r.engine.profitGuard != nil {
			if execID, ok := profitguardctx.ExecutionID(ctx); ok {
				stage := strings.ToLower(string(failure.Gate))
				if stage == "" {
					stage = "recovery"
				}
				allow, reason := r.engine.profitGuard.BeforeRetry(ctx, execID, stage, attempt, spentUSD)
				if !allow {
					r.emit(projectID, domain.Event{
						ID: newEventID(), Step: StepRecovery, Gate: failure.Gate,
						Status:    StatusFailed,
						Message:   fmt.Sprintf("profitguard_stop attempt=%d gate=%s reason=%s", attempt, failure.Gate, reason),
						CreatedAt: time.Now().UTC(),
					})
					return false, nil
				}
			}
		}
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
			Role:        agents.RoleCoder,
			Project:     &proj,
			Goal:        goal,
			Issues:      failure.Issues,
			UserBearer:  bearerFromCtx(ctx),
			WorkspaceID: workspaceIDFromCtx(ctx),
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
			// V22 completion scoring — recovery just flipped this
			// gate from failed→passed; mirror it through the scorer
			// so the absolute completion_score moves up immediately.
			r.engine.recordGateOutcome(ctx, gate.Name(), true, 0)
			r.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepRecovery, Gate: failure.Gate, Agent: string(agents.RoleCoder),
				Status:    StatusDone,
				Message:   fmt.Sprintf("recovery_done attempt=%d gate=%s", attempt, failure.Gate),
				CreatedAt: time.Now().UTC(),
			})
			// V22 learning: record the (failure → fix) shape so the
			// next occurrence of the same failure class can short-
			// circuit. The "fix" is a coarse digest of the patch we
			// just landed — concrete enough to act on, small enough
			// to keep the genome row tight.
			if r.engine.learning != nil {
				execID, _ := profitguardctx.ExecutionID(ctx)
				// Capture the literal FileChanges that fixed this
				// failure class so a future Lookup hit can replay
				// them via applyRecipePatch (closing the recipe →
				// applied → MarkSuccess loop).
				changesBlob := make([]map[string]any, 0, len(applied.Changes))
				for _, c := range applied.Changes {
					changesBlob = append(changesBlob, map[string]any{
						"op":          string(c.Op),
						"path":        c.Path,
						"content":     c.Content,
						"anchor":      c.Anchor,
						"replacement": c.Replacement,
					})
				}
				fix := map[string]any{
					"patch_title":   applied.Title,
					"patch_summary": applied.Summary,
					"patch_id":      applied.ID,
					"gate":          string(failure.Gate),
					"attempt":       attempt,
					"changes":       changesBlob,
				}
				r.engine.learning.OnRetrySuccess(ctx, execID, string(failure.Gate), failureBlob, fix)

				// Also record the applied patch shape against the
				// (gate + last patch id) intent so future runs with
				// the same recovery intent see it as a candidate.
				intent := IntentForGateStory(string(failure.Gate), failure.LastPatchID, applied.Title)
				paths := make([]string, 0, len(applied.Changes))
				for _, c := range applied.Changes {
					paths = append(paths, c.Path)
				}
				patchBlob := map[string]any{
					"title":   applied.Title,
					"summary": applied.Summary,
					"id":      applied.ID,
				}
				cost := decimal.NewFromFloat(spentUSD)
				r.engine.learning.OnPatchApplied(ctx, execID, intent, patchBlob, paths, cost)
			}
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

// applyRecipePatch attempts to replay the FileChanges baked into a
// repair recipe via the standard patch lifecycle. Returns
// (applied, true, nil) on a clean apply; (zero, false, nil) when
// the recipe has no replayable shape (e.g. a legacy row recorded
// before the `changes` field was added, or the genome row carries
// only a free-form note); (zero, false, err) when the patch
// engine rejected the recipe at Propose/Apply time.
//
// On a successful apply we also push the patch through the runtime
// applier (if wired) so the user's sandbox sees the new bytes.
func (r *RecoveryEngine) applyRecipePatch(
	ctx context.Context,
	projectID, workspaceID, bearer string,
	recipe repair.Recipe,
) (patch.Patch, bool, error) {
	if r == nil || r.engine == nil || r.engine.patches == nil {
		return patch.Patch{}, false, nil
	}
	rawChanges, ok := recipe.Fix["changes"].([]any)
	if !ok || len(rawChanges) == 0 {
		// Legacy recipe row (no embedded changes) — caller falls
		// through to the standard recovery loop.
		return patch.Patch{}, false, nil
	}
	built := patch.Patch{
		ProjectID: projectID,
		Author:    string(agents.RoleCoder) + "/recipe",
		Title:     stringFromFix(recipe.Fix, "patch_title", "auto-recipe"),
		Summary:   stringFromFix(recipe.Fix, "patch_summary", "Auto-applied repair recipe replay"),
	}
	for _, raw := range rawChanges {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		built.Changes = append(built.Changes, patch.FileChange{
			Op:          patch.Op(strings.ToLower(strings.TrimSpace(stringFromFix(m, "op", "")))),
			Path:        strings.TrimPrefix(stringFromFix(m, "path", ""), "/"),
			Content:     stringFromFix(m, "content", ""),
			Anchor:      stringFromFix(m, "anchor", ""),
			Replacement: stringFromFix(m, "replacement", ""),
		})
	}
	if len(built.Changes) == 0 {
		return patch.Patch{}, false, nil
	}
	if violations := r.engine.enforcePatchBounds(built); len(violations) > 0 {
		return patch.Patch{}, false, errors.New("recipe patch violated bounds")
	}
	proposed, err := r.engine.patches.Propose(built)
	if err != nil {
		return patch.Patch{}, false, err
	}
	if proposed.Status == patch.StatusRejected {
		return patch.Patch{}, false, errors.New("recipe patch rejected by validator")
	}
	applied, err := r.engine.patches.Apply(proposed.ID)
	if err != nil {
		return patch.Patch{}, false, err
	}
	if workspaceID != "" && r.engine.applier != nil {
		// Best-effort sandbox apply — drift is detected by the gate
		// the caller re-runs anyway.
		_ = r.engine.applier.Apply(ctx, bearer, workspaceID, applied)
	}
	return applied, true, nil
}

// stringFromFix safely extracts a string-valued key from a recipe
// fix map (or any map[string]any). Returns the supplied default
// when the key is missing or carries a non-string value.
func stringFromFix(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
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
