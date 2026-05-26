package finisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/agents"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/logctx"
	"ironflyer/apps/orchestrator/internal/patch"
	"ironflyer/apps/orchestrator/internal/profitguardctx"
	"ironflyer/apps/orchestrator/internal/retriever"
)

// lastAgentCostUSD returns the cost of the most-recently recorded
// agents.Result on the report, as a decimal.Decimal. Returns zero
// when the report has no runs. Used by the learning hook to attach a
// realistic cost prior to the patch-memory entry without re-querying
// the BillingGuard.
func lastAgentCostUSD(report *RunReport) decimal.Decimal {
	if report == nil || len(report.AgentRuns) == 0 {
		return decimal.Zero
	}
	last := report.AgentRuns[len(report.AgentRuns)-1]
	return decimal.NewFromFloat(last.CostUSD)
}

// LongVerificationHook is the V22 ProfitGuard seam for "before we
// kick off a verification step we know runs > 60s". Implementations
// return a profitguard.Action wire string ("continue", "stop",
// "kill_branch", "degrade", ...) plus a reason; nil errors only ever
// flow up when the hook itself failed (not when ProfitGuard chose to
// deny — that is signalled through the action string).
//
// Wired by the integration agent in cmd/orchestrator/main.go via
// Engine.WithLongVerificationHook. Nil-safe: when the hook is unset,
// verifications run unguarded just as they did pre-V22.
type LongVerificationHook interface {
	BeforeLongVerification(ctx context.Context, executionID string, estimatedSec float64) (action, reason string, err error)
}

// longVerifyState carries the per-engine LongVerificationHook wiring
// + the rolling duration estimate. We keep it in a package-level map
// keyed by *Engine instead of as fields on Engine so the file
// ownership boundary stays clean (engine.go is owned by the
// integration agent); the indirection costs one mutex lookup per
// reviewer pass which is negligible against the cost of the
// verification it gates.
type longVerifyState struct {
	hook   LongVerificationHook
	lastMS float64
}

var (
	longVerifyMu      sync.RWMutex
	longVerifyStates  = map[*Engine]*longVerifyState{}
)

// getLongVerifyState returns the per-engine state, creating an empty
// row on first access.
func getLongVerifyState(e *Engine) *longVerifyState {
	longVerifyMu.RLock()
	s, ok := longVerifyStates[e]
	longVerifyMu.RUnlock()
	if ok {
		return s
	}
	longVerifyMu.Lock()
	defer longVerifyMu.Unlock()
	if s, ok := longVerifyStates[e]; ok {
		return s
	}
	s = &longVerifyState{}
	longVerifyStates[e] = s
	return s
}

// WithLongVerificationHook wires the V22 BeforeLongVerification hook.
// The finisher consults it inside runReviewer (and at any other site
// that previously paid the > 60s cost of a Security / Tests sweep).
// Stop / KillBranch verdicts short-circuit the verification. Pass nil
// to disable.
func (e *Engine) WithLongVerificationHook(h LongVerificationHook) *Engine {
	if e == nil {
		return e
	}
	s := getLongVerifyState(e)
	longVerifyMu.Lock()
	s.hook = h
	longVerifyMu.Unlock()
	return e
}

// estimatedLongVerificationSec is the rolling estimate of how long
// the reviewer-side verification suite takes for the active project.
// Zero (the default for an unmeasured execution) returns 0 so the
// > 60s guard short-circuits to "no estimate yet, run unconditionally".
func (e *Engine) estimatedLongVerificationSec() float64 {
	if e == nil {
		return 0
	}
	s := getLongVerifyState(e)
	longVerifyMu.RLock()
	defer longVerifyMu.RUnlock()
	return s.lastMS
}

// RecordLongVerificationMeasurement updates the rolling estimate. Use
// after a verification run to feed the next iteration's > 60s guard.
// Currently exposed for the integration agent and any future call
// sites that measure their own verifications.
func (e *Engine) RecordLongVerificationMeasurement(seconds float64) {
	if e == nil || seconds < 0 {
		return
	}
	s := getLongVerifyState(e)
	longVerifyMu.Lock()
	s.lastMS = seconds
	longVerifyMu.Unlock()
}

// guardLongVerification consults the BeforeLongVerification hook for
// the current ctx-bound execution. Returns true when the hook
// instructed us to skip (Stop / KillBranch); false when there is no
// hook OR the hook said proceed. Other actions (Continue, Degrade,
// SwitchProvider) all return false here — the reviewer doesn't have
// the surface to degrade in place, so anything short of Stop/Kill
// runs the verification normally.
func (e *Engine) guardLongVerification(ctx context.Context, label string, estimatedSec float64) bool {
	if e == nil {
		return false
	}
	s := getLongVerifyState(e)
	longVerifyMu.RLock()
	hook := s.hook
	longVerifyMu.RUnlock()
	if hook == nil {
		return false
	}
	execID, _ := profitguardctx.ExecutionID(ctx)
	action, _, err := hook.BeforeLongVerification(ctx, execID, estimatedSec)
	if err != nil {
		// Fail-open: a hook error must not block the verification — we
		// log the issue via a tracer in production wiring but never
		// silently skip the gate on an infra failure.
		return false
	}
	_ = label // reserved for richer telemetry once metrics land
	switch action {
	case "stop", "kill_branch":
		return true
	default:
		return false
	}
}

// runPipeline drives the LLM-backed generative phase. Each stage persists
// its artefacts into the Project so the static gates that come after can
// inspect them deterministically. Every transition emits a structured SSE
// event; failure paths use ErrorCode-prefixed messages.
func (e *Engine) runPipeline(ctx context.Context, projectID, workspaceID, bearer string, report *RunReport) error {
	// Plan cache fast-path: if this project's spec hash matches a prior
	// successful pipeline run, restore the previously generated plan /
	// stack / screen map / tokens directly from cache. Skips Planner +
	// Architect + UXer entirely — three of the most expensive agent calls
	// in the loop. Only triggers when ALL four artifacts are cached so
	// gate state stays internally consistent.
	if e.tryRestorePlanFromCache(projectID) {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRun, Status: StatusDone,
			Message: "plan_cache_hit", CreatedAt: time.Now().UTC(),
		})
		if p, err := e.projects.Get(projectID); err == nil {
			_ = e.runScaffold(ctx, projectID, workspaceID, bearer, p.Spec.Stack)
		}
		e.ensureDatabase(ctx, projectID)
		e.ensureAuth(ctx, projectID)
		e.ensureDomains(ctx, projectID)
		return e.runCoderReviewLoop(ctx, projectID, workspaceID, bearer, report)
	}

	if err := e.runPlanner(ctx, projectID, report); err != nil {
		return err
	}
	if err := e.runArchitect(ctx, projectID, report); err != nil {
		return err
	}
	if p, err := e.projects.Get(projectID); err == nil {
		_ = e.runScaffold(ctx, projectID, workspaceID, bearer, p.Spec.Stack)
	}
	if err := e.runUXer(ctx, projectID, report); err != nil {
		return err
	}
	// Save the freshly generated plan so the next Run on the same spec
	// hits the cache. Captures only when all four artifacts landed.
	if p, err := e.projects.Get(projectID); err == nil {
		if entry, ok := captureFromProject(&p); ok {
			e.plans.put(projectID, entry)
		}
	}
	// Provision the database before the Coder writes a line of code that
	// touches it. ensureDatabase is idempotent and a no-op when there's no
	// data model or no provisioner wired, so this is safe to call every
	// pipeline run.
	e.ensureDatabase(ctx, projectID)
	// Auth scaffold drops deterministic auth files (Supabase + Next.js by
	// default) so the Coder builds product code on top of a known contract
	// instead of reinventing signup/login each run.
	e.ensureAuth(ctx, projectID)
	// Domain packs (games, e-commerce, social, learning, ...) — each
	// pack inspects the spec and self-decides whether to apply. Lets the
	// platform genuinely cover "everything" without bloating Auth/Stripe.
	e.ensureDomains(ctx, projectID)
	if err := e.runCoderReviewLoop(ctx, projectID, workspaceID, bearer, report); err != nil {
		return err
	}
	return nil
}

// tryRestorePlanFromCache returns true when the cache held a matching
// entry and the project state was updated from it. Hash mismatch, missing
// entry, or update error all return false so the caller falls through to
// the full generative pipeline.
func (e *Engine) tryRestorePlanFromCache(projectID string) bool {
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return false
	}
	specHash := hashSpec(&proj)
	entry, ok := e.plans.get(projectID, specHash)
	if !ok {
		return false
	}
	_, err = e.projects.Update(projectID, func(p *domain.Project) {
		// Restore typed fields the pipeline mutates.
		p.Spec.UserStories = append(p.Spec.UserStories[:0], entry.UserStories...)
		p.Spec.DataModel = append(p.Spec.DataModel[:0], entry.DataModel...)
		p.Spec.Stack = entry.Stack
		// Restore raw artifacts and their FileNode mirrors so downstream
		// gates that hunt for .ironflyer/*.json find them.
		_ = p.SetArtifact(domain.ArtifactPlan, entry.PlanJSON)
		_ = p.SetArtifact(domain.ArtifactStack, entry.StackJSON)
		_ = p.SetArtifact(domain.ArtifactScreenMap, entry.ScreenMapJSON)
		_ = p.SetArtifact(domain.ArtifactDesignTokens, entry.DesignTokens)
		writeProjectFile(p, ".ironflyer/plan.json", string(entry.PlanJSON))
		writeProjectFile(p, ".ironflyer/stack.json", string(entry.StackJSON))
		writeProjectFile(p, screenMapPath, string(entry.ScreenMapJSON))
		writeProjectFile(p, designTokensPath, string(entry.DesignTokens))
	})
	return err == nil
}

// ---------------- Planner ----------------

// plannerOutput is the JSON contract we ask the Planner to produce. We
// keep it small and load-bearing: the downstream UXer and Coder both rely
// on these IDs and acceptance criteria being present and well-formed.
type plannerOutput struct {
	Idea        string `json:"idea"`
	UserStories []struct {
		ID         string   `json:"id"`
		As         string   `json:"as"`
		IWant      string   `json:"iWant"`
		SoThat     string   `json:"soThat"`
		Acceptance []string `json:"acceptance"`
	} `json:"userStories"`
	DataModel []struct {
		Name   string   `json:"name"`
		Fields []string `json:"fields"`
	} `json:"dataModel"`
	FileList []string `json:"fileList"`
}

const plannerInstruction = `You are producing the canonical plan for an Ironflyer project. Reply with a SINGLE JSON object — no prose, no markdown fence — matching exactly:

{
  "idea": "<one-paragraph statement>",
  "userStories": [
    { "id": "US-1", "as": "...", "iWant": "...", "soThat": "...", "acceptance": ["...", "..."] }
  ],
  "dataModel": [ { "name": "...", "fields": ["id:string", "..."] } ],
  "fileList": ["apps/web/...", "apps/api/..."]
}

Rules:
- 3-7 user stories. Each story MUST have at least 2 acceptance criteria.
- Story IDs are stable: US-1, US-2, ... in declaration order.
- File list is the minimum set the Coder needs to ship the MVP.
- Output nothing but the JSON object.`

func (e *Engine) runPlanner(ctx context.Context, projectID string, report *RunReport) error {
	p, err := e.projects.Get(projectID)
	if err != nil {
		return err
	}
	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepPlanner, Agent: string(agents.RolePlanner),
		Status: StatusRunning, Message: "planner_started", CreatedAt: time.Now().UTC(),
	})

	goal := strings.TrimSpace(p.Spec.Idea)
	if goal == "" {
		goal = strings.TrimSpace(p.Description)
	}
	if goal == "" {
		goal = "Build a working MVP for: " + p.Name
	}

	// Memory grounding — feed prior project decisions, specs, and
	// business notes to the Planner so it doesn't re-derive stories
	// from a blank slate every run.
	memCtx := e.contextBundleForPlanner(ctx, projectID)

	task := agents.Task{
		Role:        agents.RolePlanner,
		Project:     &p,
		Goal:        goal + "\n\n" + plannerInstruction,
		Context:     memCtx,
		UserBearer:  bearerFromCtx(ctx),
		WorkspaceID: workspaceIDFromCtx(ctx),
	}
	lg := logctx.From(ctx).With().Str("agent", "planner").Str("project_id", projectID).Logger()
	lg.Info().Int("prompt_chars", len(task.Goal)+len(task.Context)).Msg("planner: dispatching")
	res, err := e.registry.Run(ctx, task)
	if err != nil {
		lg.Warn().Err(err).Msg("planner: agent dispatch failed")
		e.emitProviderErr(projectID, StepPlanner, agents.RolePlanner, err)
		return err
	}
	report.AgentRuns = append(report.AgentRuns, res)
	lg.Info().
		Str("provider", res.Provider).
		Int("tokens", res.Tokens).
		Float64("cost_usd", res.CostUSD).
		Int("output_chars", len(res.Output)).
		Msg("planner: agent response")

	var plan plannerOutput
	if err := unmarshalJSONFromText(res.Output, &plan); err != nil {
		preview := res.Output
		if len(preview) > 400 {
			preview = preview[:400]
		}
		lg.Warn().Err(err).Str("output_preview", preview).Msg("planner: JSON parse failed")
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepPlanner, Agent: string(agents.RolePlanner),
			Status:    StatusFailed,
			Message:   fmtErr(ErrCodePlanMalformed, "planner did not return parseable JSON"),
			CreatedAt: time.Now().UTC(),
		})
		return err
	}
	if len(plan.UserStories) == 0 {
		preview := res.Output
		if len(preview) > 400 {
			preview = preview[:400]
		}
		lg.Warn().Str("output_preview", preview).Msg("planner: zero user stories")
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepPlanner, Agent: string(agents.RolePlanner),
			Status:    StatusFailed,
			Message:   fmtErr(ErrCodePlanMalformed, "planner returned zero user stories"),
			CreatedAt: time.Now().UTC(),
		})
		return errors.New("planner: no stories")
	}
	lg.Info().Int("stories", len(plan.UserStories)).Int("data_entities", len(plan.DataModel)).Msg("planner: plan accepted")

	// Persist the plan onto the Project. We update Spec for downstream gates,
	// stash the raw JSON in a well-known file so other agents can re-read the
	// canonical document without trusting an in-memory copy.
	raw, _ := json.MarshalIndent(plan, "", "  ")
	if _, err := e.projects.Update(projectID, func(proj *domain.Project) {
		if strings.TrimSpace(plan.Idea) != "" {
			proj.Spec.Idea = plan.Idea
		}
		proj.Spec.UserStories = proj.Spec.UserStories[:0]
		for _, s := range plan.UserStories {
			proj.Spec.UserStories = append(proj.Spec.UserStories, domain.UserStory{
				ID: s.ID, As: s.As, IWant: s.IWant, SoThat: s.SoThat, Acceptance: s.Acceptance,
			})
		}
		proj.Spec.DataModel = proj.Spec.DataModel[:0]
		for _, m := range plan.DataModel {
			proj.Spec.DataModel = append(proj.Spec.DataModel, domain.EntityDef{
				Name: m.Name, Fields: m.Fields,
			})
		}
		writeProjectFile(proj, ".ironflyer/plan.json", string(raw))
		// Mirror the canonical plan into typed Artifacts so downstream gates
		// can read it without parsing a FileNode. The .ironflyer file write
		// above stays as the IDE-visible transparency layer.
		_ = proj.SetArtifact(domain.ArtifactPlan, json.RawMessage(raw))
	}); err != nil {
		return err
	}

	// Persist the chosen plan as a project spec memory so subsequent
	// runs of Planner (and downstream agents) see the story shape as
	// prior context rather than rediscovering it from scratch.
	{
		var b strings.Builder
		for i, s := range plan.UserStories {
			if i >= 8 {
				b.WriteString("- … and " + itoaPositive(len(plan.UserStories)-8) + " more\n")
				break
			}
			b.WriteString("- " + s.ID + ": " + s.IWant + "\n")
		}
		e.rememberDecision(ctx, projectID,
			"spec: "+itoaPositive(len(plan.UserStories))+" stories",
			b.String(),
			"spec")
	}

	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepPlanner, Agent: string(agents.RolePlanner),
		Status: StatusDone, Message: "planner_done", CreatedAt: time.Now().UTC(),
	})
	return nil
}

// ---------------- Architect ----------------

type architectOutput struct {
	Stack struct {
		Frontend string `json:"frontend"`
		Backend  string `json:"backend"`
		Storage  string `json:"storage"`
		Auth     string `json:"auth"`
	} `json:"stack"`
	Services []struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Dependencies []string `json:"dependencies,omitempty"`
	} `json:"services"`
	DataFlow string `json:"dataFlow"`
}

const architectInstruction = `Pick the concrete stack and service decomposition for this product. Reply with a SINGLE JSON object — no prose, no markdown fence — matching exactly:

{
  "stack": { "frontend": "...", "backend": "...", "storage": "...", "auth": "..." },
  "services": [ { "name": "...", "description": "...", "dependencies": ["..."] } ],
  "dataFlow": "<2-4 sentence summary, client → service → store>"
}

Rules:
- Choose mainstream, opinionated defaults; do not invent technologies.
- Every stack field is non-empty.
- "auth" must name an actual mechanism (JWT, OAuth, session, etc.).
- Output nothing but the JSON object.`

func (e *Engine) runArchitect(ctx context.Context, projectID string, report *RunReport) error {
	p, err := e.projects.Get(projectID)
	if err != nil {
		return err
	}
	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepArchitect, Agent: string(agents.RoleArchitect),
		Status: StatusRunning, Message: "architect_started", CreatedAt: time.Now().UTC(),
	})

	// Memory grounding — feed prior architectural decisions to the
	// Architect so it doesn't re-derive the stack on every run. The
	// recorded decisions carry the tag "decision", which is exactly
	// what contextBundleForArchitect filters on.
	memCtx := e.contextBundleForArchitect(ctx, projectID)

	task := agents.Task{
		Role:        agents.RoleArchitect,
		Project:     &p,
		Goal:        "Choose the stack and service layout.\n\n" + architectInstruction,
		Context:     memCtx,
		UserBearer:  bearerFromCtx(ctx),
		WorkspaceID: workspaceIDFromCtx(ctx),
	}
	res, err := e.registry.Run(ctx, task)
	if err != nil {
		e.emitProviderErr(projectID, StepArchitect, agents.RoleArchitect, err)
		return err
	}
	report.AgentRuns = append(report.AgentRuns, res)

	var arch architectOutput
	if err := unmarshalJSONFromText(res.Output, &arch); err != nil {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepArchitect, Agent: string(agents.RoleArchitect),
			Status:    StatusFailed,
			Message:   fmtErr(ErrCodePlanMalformed, "architect did not return parseable JSON"),
			CreatedAt: time.Now().UTC(),
		})
		return err
	}
	if arch.Stack.Frontend == "" || arch.Stack.Backend == "" {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepArchitect, Agent: string(agents.RoleArchitect),
			Status:    StatusFailed,
			Message:   fmtErr(ErrCodePlanMalformed, "architect returned an incomplete stack"),
			CreatedAt: time.Now().UTC(),
		})
		return errors.New("architect: incomplete stack")
	}

	raw, _ := json.MarshalIndent(arch, "", "  ")
	if _, err := e.projects.Update(projectID, func(proj *domain.Project) {
		proj.Spec.Stack = domain.StackDecision{
			Frontend: arch.Stack.Frontend,
			Backend:  arch.Stack.Backend,
			Storage:  arch.Stack.Storage,
			Auth:     arch.Stack.Auth,
		}
		writeProjectFile(proj, ".ironflyer/stack.json", string(raw))
		_ = proj.SetArtifact(domain.ArtifactStack, json.RawMessage(raw))
	}); err != nil {
		return err
	}

	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepArchitect, Agent: string(agents.RoleArchitect),
		Status: StatusDone, Message: "architect_done", CreatedAt: time.Now().UTC(),
	})
	// Persist the chosen stack as a project decision so subsequent runs
	// of Architect (and the Coder grounding bundle) see it as evidence
	// rather than rediscovering it from scratch.
	e.rememberDecision(ctx, projectID,
		"stack: "+arch.Stack.Frontend+" + "+arch.Stack.Backend,
		"frontend="+arch.Stack.Frontend+
			" backend="+arch.Stack.Backend+
			" storage="+arch.Stack.Storage+
			" auth="+arch.Stack.Auth,
		"stack")
	return nil
}

// ---------------- UXer ----------------

const uxerInstruction = `Produce the screen map and design tokens for this product. Reply with a SINGLE JSON object — no prose, no markdown fence — matching exactly:

{
  "screenMap": {
    "screens": [
      { "id": "S-1", "name": "...", "route": "/...", "components": ["AppShell","Page","Card","Button"], "storyIds": ["US-1"] }
    ]
  },
  "designTokens": {
    "color":   { "bg": "#...", "fg": "#...", "accent": "#..." },
    "spacing": { "xs": 4, "sm": 8, "md": 16, "lg": 24, "xl": 40 },
    "type":    { "body": "Inter, sans-serif", "mono": "JetBrains Mono, monospace" }
  }
}

Rules:
- One screen per primary user story; aim for 3-8 screens total.
- Components MUST be drawn from the standard set: AppShell, TopBar, Sidebar, Page, Section, Card, List, Table, Form, Field, Button, IconButton, Link, Menu, Modal, Tabs, Alert, Avatar, Chip, Empty, Loader, Chart, Search, Filter, Pagination, Uploader.
- color must include bg / fg / accent at minimum.
- spacing must include xs / sm / md / lg / xl.
- Output nothing but the JSON object.`

type uxerOutput struct {
	ScreenMap    screenMapDoc    `json:"screenMap"`
	DesignTokens designTokensDoc `json:"designTokens"`
}

func (e *Engine) runUXer(ctx context.Context, projectID string, report *RunReport) error {
	p, err := e.projects.Get(projectID)
	if err != nil {
		return err
	}
	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepUXer, Agent: string(agents.RoleUXer),
		Status: StatusRunning, Message: "uxer_started", CreatedAt: time.Now().UTC(),
	})

	// Memory grounding — surface prior design / ux notes recorded
	// against this project so the screen map stays coherent across
	// runs instead of drifting on each pass.
	memCtx := e.contextBundleForUXer(ctx, projectID)

	task := agents.Task{
		Role:        agents.RoleUXer,
		Project:     &p,
		Goal:        "Map every user story to a screen.\n\n" + uxerInstruction,
		Context:     memCtx,
		UserBearer:  bearerFromCtx(ctx),
		WorkspaceID: workspaceIDFromCtx(ctx),
	}
	res, err := e.registry.Run(ctx, task)
	if err != nil {
		e.emitProviderErr(projectID, StepUXer, agents.RoleUXer, err)
		return err
	}
	report.AgentRuns = append(report.AgentRuns, res)

	var ux uxerOutput
	if err := unmarshalJSONFromText(res.Output, &ux); err != nil {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepUXer, Agent: string(agents.RoleUXer),
			Status:    StatusFailed,
			Message:   fmtErr(ErrCodePlanMalformed, "UXer did not return parseable JSON"),
			CreatedAt: time.Now().UTC(),
		})
		return err
	}
	if len(ux.ScreenMap.Screens) == 0 {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepUXer, Agent: string(agents.RoleUXer),
			Status:    StatusFailed,
			Message:   fmtErr(ErrCodePlanMalformed, "UXer returned zero screens"),
			CreatedAt: time.Now().UTC(),
		})
		return errors.New("uxer: no screens")
	}

	mapRaw, _ := json.MarshalIndent(ux.ScreenMap, "", "  ")
	tokRaw, _ := json.MarshalIndent(ux.DesignTokens, "", "  ")
	if _, err := e.projects.Update(projectID, func(proj *domain.Project) {
		writeProjectFile(proj, screenMapPath, string(mapRaw))
		writeProjectFile(proj, designTokensPath, string(tokRaw))
		_ = proj.SetArtifact(domain.ArtifactScreenMap, json.RawMessage(mapRaw))
		_ = proj.SetArtifact(domain.ArtifactDesignTokens, json.RawMessage(tokRaw))
	}); err != nil {
		return err
	}

	// Persist the screen map as a project design memory so subsequent
	// runs of the UXer (and downstream Coder grounding) see what was
	// already chosen rather than redesigning the surface on every pass.
	{
		var b strings.Builder
		for i, s := range ux.ScreenMap.Screens {
			if i >= 8 {
				b.WriteString("- … and " + itoaPositive(len(ux.ScreenMap.Screens)-8) + " more\n")
				break
			}
			b.WriteString("- " + s.ID + " " + s.Name + " (" + s.Route + ")\n")
		}
		e.rememberDecision(ctx, projectID,
			"ux: "+itoaPositive(len(ux.ScreenMap.Screens))+" screens",
			b.String(),
			"design", "ux")
	}

	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepUXer, Agent: string(agents.RoleUXer),
		Status: StatusDone, Message: "uxer_done", CreatedAt: time.Now().UTC(),
	})
	return nil
}

// ---------------- Coder + Reviewer ----------------

// coderPatch is the on-the-wire shape we ask the Coder to emit per story.
// We deliberately use a JSON file-change array instead of unified diff text:
// the LLM produces it more reliably and our patch.Engine consumes it directly.
type coderPatch struct {
	Title   string                 `json:"title"`
	Summary string                 `json:"summary"`
	Changes []coderPatchFileChange `json:"changes"`
}

type coderPatchFileChange struct {
	Op          string `json:"op"`
	Path        string `json:"path"`
	Content     string `json:"content,omitempty"`
	Anchor      string `json:"anchor,omitempty"`
	Replacement string `json:"replacement,omitempty"`
}

const coderInstruction = `Implement the requested user story by producing a code patch. Reply with a SINGLE JSON object — no prose, no markdown fence — matching exactly:

{
  "title":   "<short imperative summary>",
  "summary": "<one paragraph explaining the change>",
  "changes": [
    { "op": "create" | "update" | "delete" | "replace" | "insert_after",
      "path": "<relative path>",
      "content": "<full file contents (create/update only)>",
      "anchor": "<verbatim substring of current file (replace/insert_after only)>",
      "replacement": "<new text (replace/insert_after only)>"
    }
  ]
}

Rules:
- Use "create" for new files, "update" for full-file rewrites, "delete" to remove a file.
- Prefer "replace" or "insert_after" for SMALL edits to existing files: provide a
  unique anchor substring + the new text. Anchor MUST appear exactly once in the
  current file body — pick enough surrounding context to make it unique. This
  saves output tokens and avoids reformatting whole files.
- "content" is for create/update only and must be the COMPLETE final file body.
- Paths are relative, never absolute. No "..", no "/etc/", no ".ssh/".
- Keep total patch under 200 KiB and 40 files.
- Implement the story end-to-end: routes, handlers, types, error states.
- Output nothing but the JSON object.`

func (e *Engine) runCoderReviewLoop(ctx context.Context, projectID, workspaceID, bearer string, report *RunReport) error {
	p, err := e.projects.Get(projectID)
	if err != nil {
		return err
	}
	stories := append([]domain.UserStory(nil), p.Spec.UserStories...)
	if len(stories) == 0 {
		return errors.New("coder: no stories to implement")
	}

	for _, story := range stories {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := e.codeOneStory(ctx, projectID, workspaceID, bearer, story, report); err != nil {
			// The story failure is already emitted as a structured event;
			// continue to the next story so partial progress lands on the
			// project rather than blocking the entire run.
			continue
		}
	}
	return nil
}

func (e *Engine) codeOneStory(ctx context.Context, projectID, workspaceID, bearer string, story domain.UserStory, report *RunReport) error {
	// Patch cache fast-path: if we have a previously accepted patch for
	// (projectID, storyID, specHash), try to push it through the live
	// Propose+Apply path. The patch engine's anchor validator will reject
	// it if the tree has drifted, in which case we fall through to a
	// fresh Coder call. Cache hits skip a Coder + Critic + Reviewer chain.
	if proj, err := e.projects.Get(projectID); err == nil {
		specHash := hashSpec(&proj)
		if cached, ok := e.patchesCache.get(projectID, story.ID, specHash); ok {
			cached.ProjectID = projectID
			cached.ID = "" // let patch.Engine mint a fresh one
			proposed, err := e.patches.Propose(cached)
			if err == nil && proposed.Status == patch.StatusProposed {
				applied, err := e.patches.Apply(proposed.ID)
				if err == nil {
					if workspaceID != "" {
						_ = e.applier.Apply(ctx, bearer, workspaceID, applied)
					}
					e.emit(projectID, domain.Event{
						ID: newEventID(), Step: StepPatch, Agent: string(agents.RoleCoder),
						Status:    StatusDone,
						Message:   "patch_cache_hit story=" + story.ID + " id=" + applied.ID,
						CreatedAt: time.Now().UTC(),
					})
					report.PatchIDs = append(report.PatchIDs, applied.ID)
					return nil
				}
			}
			// Cached patch no longer fits — fall through and re-prompt.
		}
	}

	var (
		failureContext string
		lastErrIssues  []domain.Issue
	)
	for attempt := 0; attempt < e.maxCoderRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		// V22 ProfitGuard BeforeRetryLoop hook. The integration loop in
		// main.go wires profitguard.Guard behind ProfitGuardHook so the
		// retry budget is short-circuited the moment the policy decides
		// the next round is unprofitable. Nil-safe — without the hook
		// the loop runs unchanged.
		if e.profitGuard != nil {
			if execID, ok := profitguardctx.ExecutionID(ctx); ok {
				allow, reason := e.profitGuard.BeforeRetry(ctx, execID, "coder", attempt, 0)
				if !allow {
					e.emit(projectID, domain.Event{
						ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
						Status:    StatusFailed,
						Message:   fmt.Sprintf("profitguard_stop attempt=%d reason=%s", attempt+1, reason),
						CreatedAt: time.Now().UTC(),
					})
					return errors.New("profitguard: " + reason)
				}
			}
		}
		p, err := e.projects.Get(projectID)
		if err != nil {
			return err
		}

		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
			Status:    StatusRunning,
			Message:   fmt.Sprintf("coder_started story=%s attempt=%d", story.ID, attempt+1),
			CreatedAt: time.Now().UTC(),
		})

		goal := buildCoderGoal(story, failureContext)
		// RAG: pull the few most relevant existing chunks for this story and
		// inject them as grounding context. Empty corpus → empty string → no
		// extra prompt cost. Token budget capped by retriever options.
		idx := retriever.Build(&p, retriever.Options{TopK: 8})
		hits := idx.Query(coderQueryForStory(story), 8)
		rag := retriever.FormatContext(hits)
		// Memory grounding — prior failure/fix lineage + project decisions
		// for the same story. Cheap append; the Coder reads them as part
		// of the same context block as the RAG snippets.
		if mem := e.contextBundleForCoder(ctx, projectID, story); mem != "" {
			if rag == "" {
				rag = mem
			} else {
				rag = mem + "\n\n" + rag
			}
		}
		// Parallel critic: the Coder streams tokens and a critic
		// goroutine runs alongside, firing partial critiques against a
		// sliding window. A `severity: blocker` finding cancels the
		// Coder mid-stream and we fail the gate fast rather than burn
		// the retry budget on a doomed patch. When
		// IRONFLYER_CRITIC_PARALLEL=false, this transparently falls
		// back to the original single-shot registry.Run().
		coderTask := agents.Task{
			Role:           agents.RoleCoder,
			Project:        &p,
			Goal:           goal,
			Issues:         lastErrIssues,
			Context:        rag,
			ThinkingBudget: coderThinkingBudget(story, p),
			UserBearer:     bearerFromCtx(ctx),
			WorkspaceID:    workspaceIDFromCtx(ctx),
		}
		// A55 agent reasoning — Coder is about to call the model.
		emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageActionV1, map[string]any{
			"stage":      "coder",
			"agent_role": "coder",
			"action":     "patch_propose",
			"story":      story.ID,
			"attempt":    attempt + 1,
			"message":    "Generating patch for " + story.ID + " (attempt " + itoaPositive(attempt+1) + ")",
		})
		res, runErr := e.runCoderStreamingWithCritic(ctx, projectID, story, coderTask)
		if errors.Is(runErr, abortedByCriticErr) {
			// The parallel critic killed the stream — the abort event
			// + audit entry were already emitted inside the helper.
			// Don't burn the rest of the retry budget on a doomed
			// generation; bail out of this story so the caller moves
			// on to the next one.
			return runErr
		}
		if runErr != nil {
			e.emitProviderErr(projectID, StepCoder, agents.RoleCoder, runErr)
			return runErr
		}
		report.AgentRuns = append(report.AgentRuns, res)

		var cp coderPatch
		if err := unmarshalJSONFromText(res.Output, &cp); err != nil {
			failureContext = "Your previous response was not parseable JSON. Return exactly the schema, no fences."
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchInvalid, "coder JSON unparseable on attempt "+itoaPositive(attempt+1)),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}

		// Build a patch.Patch and validate scope + size before letting the
		// Reviewer see it. Patches that bust the bounds are rejected hard.
		built := patch.Patch{
			ProjectID: projectID,
			Author:    string(agents.RoleCoder),
			Title:     cp.Title,
			Summary:   cp.Summary,
		}
		for _, c := range cp.Changes {
			built.Changes = append(built.Changes, patch.FileChange{
				Op:          patch.Op(strings.ToLower(strings.TrimSpace(c.Op))),
				Path:        strings.TrimPrefix(c.Path, "/"),
				Content:     c.Content,
				Anchor:      c.Anchor,
				Replacement: c.Replacement,
			})
		}
		if violations := e.enforcePatchBounds(built); len(violations) > 0 {
			failureContext = "Your patch violated bounds: " + joinIssues(violations) + ". Shrink and retry."
			lastErrIssues = violations
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchTooLarge, "patch out of bounds on attempt "+itoaPositive(attempt+1)),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}

		// Syntax precheck — reject patches that would yield un-parseable
		// files before they enter the lifecycle. Saves a Critic + Reviewer
		// round-trip on broken JSON / JS / TS / Go / Python output.
		if syntaxIssues := e.syntaxPrecheck(ctx, &p, workspaceID, bearer, built.Changes); len(syntaxIssues) > 0 {
			failureContext = "Your patch produced un-parseable files: " + joinIssues(syntaxIssues) + ". Fix the syntax and retry."
			lastErrIssues = syntaxIssues
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchInvalid, "syntax precheck rejected the patch"),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}

		proposed, err := e.patches.Propose(built)
		if err != nil {
			failureContext = "Patch was rejected by the engine: " + err.Error()
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchInvalid, err.Error()),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}
		if proposed.Status == patch.StatusRejected {
			failureContext = "Your patch was rejected during validation: " + joinIssues(proposed.Issues) + ". Fix and retry."
			lastErrIssues = proposed.Issues
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchInvalid, "validator rejected patch"),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}
		// Critic pass — a cheap, structured judge that reads the proposed
		// patch against the story and flags concrete blockers BEFORE we pay
		// for the full Reviewer simulation. On the first attempt only: if
		// the Coder has already revised once, additional critic input is
		// usually diminishing returns and we move straight to the Reviewer
		// for ground-truth gate checking.
		if attempt == 0 {
			if findings, critRes, ok := e.runCritic(ctx, projectID, &proposed, story); ok {
				if critRes != nil {
					report.AgentRuns = append(report.AgentRuns, *critRes)
				}
				if len(findings) > 0 {
					failureContext = "Critic flagged blockers: " + joinIssues(findings) + ". Address them in the next patch."
					lastErrIssues = findings
					e.emit(projectID, domain.Event{
						ID: newEventID(), Step: StepCritic, Agent: string(agents.RoleCritic),
						Status:    StatusFailed,
						Message:   fmt.Sprintf("critic_rejected story=%s findings=%d top=%s", story.ID, len(findings), tail(findings[0].Message, 120)),
						CreatedAt: time.Now().UTC(),
					})
					continue
				}
			}
		}
		report.PatchIDs = append(report.PatchIDs, proposed.ID)
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepPatch, Agent: string(agents.RoleCoder),
			Status:    StatusDone,
			Message:   "patch_proposed id=" + proposed.ID + " story=" + story.ID,
			CreatedAt: time.Now().UTC(),
		})

		// Reviewer phase: simulate the patch against the project state, then
		// run the cheap static gates against the would-be-applied tree. We
		// don't run runtime build/test gates here (they're handled by the
		// Run gate phase after apply) — keep this loop fast.
		reviewIssues := e.runReviewer(ctx, projectID, &proposed)
		if len(reviewIssues) > 0 {
			failureContext = "Reviewer found issues: " + joinIssues(reviewIssues) + ". Revise and retry."
			lastErrIssues = reviewIssues
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepReviewer, Agent: string(agents.RoleReviewer),
				Status: StatusFailed,
				Message: fmt.Sprintf("reviewer_rejected story=%s attempt=%d issues=%d",
					story.ID, attempt+1, len(reviewIssues)),
				CreatedAt: time.Now().UTC(),
			})
			continue
		}

		// Apply: in-memory always; runtime applier if configured. A runtime
		// error is surfaced but doesn't roll the in-memory state back — the
		// gate phase that follows will detect drift.
		applied, applyErr := e.patches.Apply(proposed.ID)
		if applyErr != nil {
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepPatch, Agent: string(agents.RoleCoder),
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodePatchInvalid, applyErr.Error()),
				CreatedAt: time.Now().UTC(),
			})
			return applyErr
		}
		if workspaceID != "" {
			if err := e.applier.Apply(ctx, bearer, workspaceID, applied); err != nil {
				e.emit(projectID, domain.Event{
					ID: newEventID(), Step: StepPatch, Agent: string(agents.RoleCoder),
					Status:    StatusFailed,
					Message:   fmtErr(ErrCodeRuntimeError, "workspace apply failed: "+err.Error()),
					CreatedAt: time.Now().UTC(),
				})
				// Continue — the gate phase will catch any divergence.
			}
		}
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepPatch, Agent: string(agents.RoleCoder),
			Status:    StatusDone,
			Message:   "patch_applied id=" + applied.ID + " story=" + story.ID,
			CreatedAt: time.Now().UTC(),
		})
		// A55 agent reasoning — patch landed.
		{
			pathNames := make([]string, 0, len(applied.Changes))
			for _, c := range applied.Changes {
				pathNames = append(pathNames, c.Path)
			}
			suffix := "s"
			if len(applied.Changes) == 1 {
				suffix = ""
			}
			summary := fmt.Sprintf("Applied %d file%s for %s",
				len(applied.Changes), suffix, story.ID)
			emitExecutionEvent(ctx, e.executionService, execution.EventAgentStageResultV1, map[string]any{
				"stage":      "coder",
				"agent_role": "coder",
				"action":     "patch_apply",
				"success":    true,
				"story":      story.ID,
				"patch_id":   applied.ID,
				"paths":      pathNames,
				"summary":    summary,
			})
		}
		// Wow-loop event ring: emit patch.applied.v1 so the customer-
		// facing executionSupportBundle's "what changed" panel reflects
		// the Coder's landed patch. Cost is sourced from the most-recent
		// AgentRuns entry (same source the learning hook below uses) so
		// the row carries realistic margin signal for downstream.
		{
			appliedPaths := make([]string, 0, len(applied.Changes))
			for _, c := range applied.Changes {
				appliedPaths = append(appliedPaths, c.Path)
			}
			emitExecutionEvent(ctx, e.executionService, execution.EventPatchAppliedV1, map[string]any{
				"patch_id":       applied.ID,
				"affected_paths": appliedPaths,
				"cost_usd":       lastAgentCostUSD(report).String(),
				"applied_at":     time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
		// Cache the successful patch keyed by spec hash so future
		// repair iterations on the same story short-circuit.
		if proj, getErr := e.projects.Get(projectID); getErr == nil {
			e.patchesCache.put(projectID, story.ID, hashSpec(&proj), applied)
		}
		// V22 learning: persist the patch shape against the (gate +
		// story) intent in patch memory. The intent string is stable
		// across runs of the same story; matching past patches feed
		// future Coder calls + dashboard "patch reuse rate" stats.
		if e.learning != nil {
			execID, _ := profitguardctx.ExecutionID(ctx)
			intent := IntentForGateStory(string(domain.GateCode), story.ID, applied.Title)
			paths := make([]string, 0, len(applied.Changes))
			for _, c := range applied.Changes {
				paths = append(paths, c.Path)
			}
			patchBlob := map[string]any{
				"title":   applied.Title,
				"summary": applied.Summary,
				"id":      applied.ID,
				"story":   story.ID,
			}
			// Cost USD — the Coder Result attached to report.AgentRuns
			// carries the realised provider cost. Sum any runs since
			// the last patch landed (kept as the last AgentRuns entry
			// in the v1 wiring).
			cost := lastAgentCostUSD(report)
			e.learning.OnPatchApplied(ctx, execID, intent, patchBlob, paths, cost)
		}
		// Memory capture — record the winning patch pattern, plus a
		// failure-fix lineage entry when this attempt followed a
		// rejection on the same story.
		e.rememberCoderPatch(ctx, projectID, applied)
		if len(lastErrIssues) > 0 {
			gateForLog := domain.GateCode
			if lastErrIssues[0].Gate != "" {
				gateForLog = lastErrIssues[0].Gate
			}
			e.rememberFailureFix(ctx, projectID, story.ID, gateForLog,
				lastErrIssues, applied.ID, applied.Title)
		}
		// Reflection pass — cheap judge reviews (story, patch, post-apply
		// state) and persists any "partial" / "drift" verdict as memory
		// so future runs avoid the same gap. Best-effort; intentionally
		// ignored on error so reflection never blocks the loop.
		_, _ = e.reflectOnPatch(ctx, projectID, story, applied)
		return nil
	}

	// All retries exhausted.
	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
		Status:    StatusFailed,
		Message:   fmtErr(ErrCodeGateUnrecoverable, "coder exhausted retries for story "+story.ID),
		CreatedAt: time.Now().UTC(),
	})
	return errors.New("coder: max retries for " + story.ID)
}

// runReviewer asks the Reviewer agent to inspect the proposed patch and
// also re-runs the cheap static gates (Spec, UX, Arch, Security) against a
// hypothetical post-apply project. We don't trust the LLM unilaterally:
// the static gates are the source of truth, the Reviewer commentary is
// surfaced as extra Hint context only.
func (e *Engine) runReviewer(ctx context.Context, projectID string, proposed *patch.Patch) []domain.Issue {
	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: StepReviewer, Agent: string(agents.RoleReviewer),
		Status: StatusRunning, Message: "reviewer_started", CreatedAt: time.Now().UTC(),
	})

	// Simulate apply against a project copy so static gates see the future.
	p, err := e.projects.Get(projectID)
	if err != nil {
		return []domain.Issue{{Gate: domain.GateCode, Severity: domain.SeverityError, Message: err.Error()}}
	}
	preview := simulateApply(p, proposed.Changes)

	env := &GateEnv{Project: &preview} // no Runtime — keep this loop fast.
	var issues []domain.Issue
	// V22 BeforeLongVerification gate. When the long-verification hook
	// is wired AND we have prior measurements suggesting the Security
	// gate (or any other reviewer-side verification) routinely runs
	// past 60s for this execution, consult ProfitGuard before paying
	// the cost again. Stop / KillBranch verdicts skip the verification
	// step and let the reviewer pass with a degraded (warn-only)
	// verdict. nil-safe — without the hook the cheap reviewer runs
	// unchanged.
	estSec := e.estimatedLongVerificationSec()
	if estSec > 60 && e.guardLongVerification(ctx, "reviewer.security", estSec) {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepReviewer, Agent: string(agents.RoleReviewer),
			Status: StatusDone, Message: "long_verification_skipped profitguard",
			CreatedAt: time.Now().UTC(),
		})
		return nil
	}
	for _, g := range cheapReviewerGates() {
		issues = append(issues, g.Check(ctx, env)...)
	}

	// Filter to severity Error/Critical — warnings are not blocking here.
	var blocking []domain.Issue
	for _, iss := range issues {
		if iss.Severity == domain.SeverityError || iss.Severity == domain.SeverityCritical {
			blocking = append(blocking, iss)
		}
	}
	if len(blocking) == 0 {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepReviewer, Agent: string(agents.RoleReviewer),
			Status: StatusDone, Message: "reviewer_passed", CreatedAt: time.Now().UTC(),
		})
	}
	return blocking
}

// cheapReviewerGates returns the subset of gates that are safe to run on a
// simulated tree without a workspace. Build, lint, and test gates are
// excluded — they require a real runtime and the Run gate phase covers them.
func cheapReviewerGates() []Gate {
	return []Gate{SpecGate{}, UXGate{}, ArchGate{}, SecurityGate{}}
}

// simulateApply returns a project copy with the patch changes applied.
// Used by the Reviewer to see static-gate state of the post-apply tree
// without mutating the real store.
func simulateApply(p domain.Project, changes []patch.FileChange) domain.Project {
	files := make([]domain.FileNode, len(p.Files))
	copy(files, p.Files)
	p.Files = files

	for _, c := range changes {
		switch c.Op {
		case patch.OpDelete:
			filtered := p.Files[:0]
			for _, f := range p.Files {
				if f.Path != c.Path {
					filtered = append(filtered, f)
				}
			}
			p.Files = filtered
		case patch.OpCreate, patch.OpUpdate:
			updated := false
			for i := range p.Files {
				if p.Files[i].Path == c.Path {
					p.Files[i].Content = c.Content
					p.Files[i].Size = len(c.Content)
					updated = true
					break
				}
			}
			if !updated {
				p.Files = append(p.Files, domain.FileNode{
					Path: c.Path, Type: "file", Content: c.Content, Size: len(c.Content),
				})
			}
		case patch.OpReplace, patch.OpInsertAfter:
			for i := range p.Files {
				if p.Files[i].Path != c.Path {
					continue
				}
				old := p.Files[i].Content
				var updated string
				if c.Op == patch.OpReplace {
					updated = strings.Replace(old, c.Anchor, c.Replacement, 1)
				} else {
					updated = strings.Replace(old, c.Anchor, c.Anchor+c.Replacement, 1)
				}
				p.Files[i].Content = updated
				p.Files[i].Size = len(updated)
				break
			}
		}
	}
	return p
}

// enforcePatchBounds is the bounds + safety pass that runs before the
// patch.Engine ever sees the payload. The Engine itself enforces forbidden
// paths and op validity; here we add size + count caps so a runaway agent
// can't DoS the loop with a 50 MB blob.
func (e *Engine) enforcePatchBounds(p patch.Patch) []domain.Issue {
	var issues []domain.Issue
	if len(p.Changes) == 0 {
		issues = append(issues, domain.Issue{
			Gate: domain.GateCode, Severity: domain.SeverityError,
			Message: "patch has zero changes",
		})
	}
	if len(p.Changes) > e.maxFilesPerPatch {
		issues = append(issues, domain.Issue{
			Gate: domain.GateCode, Severity: domain.SeverityError,
			Message: fmt.Sprintf("patch exceeds %d files (got %d)", e.maxFilesPerPatch, len(p.Changes)),
		})
	}
	total := 0
	for _, c := range p.Changes {
		if strings.TrimSpace(c.Path) == "" {
			issues = append(issues, domain.Issue{
				Gate: domain.GateCode, Severity: domain.SeverityError,
				Message: "patch change has empty path",
			})
			continue
		}
		if strings.HasPrefix(c.Path, "/") {
			issues = append(issues, domain.Issue{
				Gate: domain.GateCode, Severity: domain.SeverityError,
				Message: "patch path must be relative", Path: c.Path,
			})
		}
		if strings.Contains(c.Path, "..") {
			issues = append(issues, domain.Issue{
				Gate: domain.GateCode, Severity: domain.SeverityCritical,
				Message: "patch path escapes project", Path: c.Path,
			})
		}
		total += len(c.Content) + len(c.Anchor) + len(c.Replacement)
	}
	if total > e.maxPatchBytes {
		issues = append(issues, domain.Issue{
			Gate: domain.GateCode, Severity: domain.SeverityError,
			Message: fmt.Sprintf("patch exceeds %d bytes (got %d)", e.maxPatchBytes, total),
		})
	}
	return issues
}

// coderQueryForStory builds a free-text retrieval query from a user story.
// We concatenate the natural-language fields plus acceptance criteria so the
// BM25 ranker can match symbol names and keywords that appear in the
// existing project source.
func coderQueryForStory(story domain.UserStory) string {
	var b strings.Builder
	b.WriteString(story.As)
	b.WriteString(" ")
	b.WriteString(story.IWant)
	b.WriteString(" ")
	b.WriteString(story.SoThat)
	for _, a := range story.Acceptance {
		b.WriteString(" ")
		b.WriteString(a)
	}
	return b.String()
}

// coderThinkingBudget scales the extended-thinking budget allocated to a
// Coder run by the surface area of the story + project. Cheap fixes get a
// short budget; large multi-file changes against a populated codebase get a
// generous one. Returning 0 means "use the provider default."
func coderThinkingBudget(story domain.UserStory, p domain.Project) int {
	// Base budget: 0 (off) — Coder doesn't request thinking by default in
	// agents.yaml. We only bump it when the orchestrator decides this run
	// is hard enough to warrant it.
	if len(story.Acceptance) <= 1 && len(p.Files) < 10 {
		return 0
	}
	const (
		minBudget = 2000
		maxBudget = 12000
	)
	budget := minBudget + 600*len(story.Acceptance) + 40*len(p.Files)
	if budget > maxBudget {
		budget = maxBudget
	}
	return budget
}

// buildCoderGoal renders the per-story coder prompt, appending any prior
// failure context so the agent has the breadcrumb trail it needs to revise.
func buildCoderGoal(story domain.UserStory, failureContext string) string {
	var b strings.Builder
	b.WriteString("Implement user story " + story.ID + ".\n\n")
	b.WriteString("As " + story.As + "\n")
	b.WriteString("I want " + story.IWant + "\n")
	if story.SoThat != "" {
		b.WriteString("So that " + story.SoThat + "\n")
	}
	if len(story.Acceptance) > 0 {
		b.WriteString("\nAcceptance criteria:\n")
		for _, a := range story.Acceptance {
			b.WriteString("- " + a + "\n")
		}
	}
	if strings.TrimSpace(failureContext) != "" {
		b.WriteString("\nPrior attempt feedback:\n" + failureContext + "\n")
	}
	b.WriteString("\n" + coderInstruction)
	return b.String()
}

// tryRecoverGate is the single integration point between the engine's
// gate-check loop and the auto-recovery engine. It is invoked AFTER a
// gate_failed event has been emitted and BEFORE the run is allowed to
// terminate. Returns true when recovery succeeded (caller should treat
// the gate as repaired for this iteration); false otherwise.
//
// The function is intentionally tolerant of a nil-stdout/stderr failure:
// gates whose Check returns Issues but never executes a workspace command
// simply hand us empty trailers, which the recovery prompt handles.
func (e *Engine) tryRecoverGate(
	ctx context.Context,
	projectID, workspaceID, bearer string,
	gateName domain.GateName,
	issues []domain.Issue,
	report *RunReport,
) bool {
	lastPatchID := ""
	if n := len(report.PatchIDs); n > 0 {
		lastPatchID = report.PatchIDs[n-1]
	}
	rec := NewRecoveryEngine(e, RecoveryConfig{})
	recovered, err := rec.Recover(ctx, projectID, workspaceID, bearer, GateFailure{
		Gate:        gateName,
		Issues:      issues,
		LastPatchID: lastPatchID,
	}, report)
	if err != nil {
		// Hard infra error (e.g. ctx cancelled) — recovery already emitted
		// its own structured event where it could; we just decline the fix.
		return false
	}
	return recovered
}

func (e *Engine) emitProviderErr(projectID, step string, role agents.Role, err error) {
	code := classifyProviderErr(err)
	e.emit(projectID, domain.Event{
		ID: newEventID(), Step: step, Agent: string(role),
		Status: StatusFailed, Message: fmtErr(code, err.Error()),
		CreatedAt: time.Now().UTC(),
	})
}

func joinIssues(issues []domain.Issue) string {
	parts := make([]string, 0, len(issues))
	for _, i := range issues {
		s := string(i.Severity) + ": " + i.Message
		if i.Path != "" {
			s += " (" + i.Path + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "; ")
}

// writeProjectFile upserts a FileNode by path on the project. Used by the
// pipeline stages to publish their canonical JSON artefacts.
func writeProjectFile(p *domain.Project, path, content string) {
	for i := range p.Files {
		if p.Files[i].Path == path {
			p.Files[i].Content = content
			p.Files[i].Size = len(content)
			return
		}
	}
	p.Files = append(p.Files, domain.FileNode{
		Path: path, Type: "file", Content: content, Size: len(content),
	})
}

// unmarshalJSONFromText parses a JSON object that may be wrapped in a
// ```json fenced block or surrounded by short prose. We slice from the
// first '{' to the matching last '}' and feed that to encoding/json.
func unmarshalJSONFromText(s string, out any) error {
	s = strings.TrimSpace(s)
	// Strip common fences.
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx > 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return errors.New("no JSON object found in response")
	}
	return json.Unmarshal([]byte(s[start:end+1]), out)
}
