package finisher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/patch"
	"ironflyer/core/orchestrator/internal/ai/scaffold"
)

// StepScaffold is the SSE step name for the scaffolder. Consumers (web UI,
// VSCode extension, SDK) dispatch on this string to render the "preparing
// your starter…" affordance during the first few seconds of a run.
const StepScaffold = "scaffold"

// scaffoldEngine is the lazy-initialised scaffold engine used by the loop.
// We keep it package-scoped (rather than as a field on finisher.Engine) so
// the loop integration is a single-line call and no constructor surface
// elsewhere has to learn about scaffolding. Discovery happens once on
// first use.
var scaffoldEngine = scaffold.Default()

// runScaffold materialises a starter project into the workspace immediately
// after the Architect decides the stack. The fast path is: build a patch
// via the scaffold engine, run it through patch.Engine.Propose (so the
// normal validate path approves it), Apply (so it lands on the in-memory
// project), then push the same patch through the RuntimeApplier so the
// user's live workspace gets the files (and any dev server picks them up
// via HMR).
//
// Failure modes are intentionally soft. If the scaffolder errors for any
// reason — template missing, oversized, runtime apply rejected — we emit
// "scaffold_skipped" and return nil. The pipeline continues; the Coder
// will still start from an empty tree, just without the magic-moment
// preview. The mission says: never fail the run because the scaffold
// failed.
func (e *Engine) runScaffold(
	ctx context.Context,
	projectID, workspaceID, bearer string,
	stack domain.StackDecision,
) error {
	e.emit(projectID, domain.Event{
		ID:        newEventID(),
		Step:      StepScaffold,
		Agent:     string(agents.RoleArchitect), // scaffold is downstream of arch
		Status:    StatusRunning,
		Message:   "scaffold_started",
		CreatedAt: time.Now().UTC(),
	})

	proj, err := e.projects.Get(projectID)
	if err != nil {
		e.emitScaffoldSkip(projectID, "load project: "+err.Error())
		return nil
	}

	// Idempotency: if a previous run already scaffolded, the sentinel will
	// be present on the in-memory project. Skip cheaply.
	for _, f := range proj.Files {
		if f.Path == scaffold.SentinelPath {
			e.emit(projectID, domain.Event{
				ID:        newEventID(),
				Step:      StepScaffold,
				Status:    StatusDone,
				Message:   "scaffold_already_present",
				CreatedAt: time.Now().UTC(),
			})
			return nil
		}
	}

	spec := stackSpecFromDomain(stack)
	built, err := scaffoldEngine.Scaffold(spec, proj.Name)
	if err != nil {
		e.emitScaffoldSkip(projectID, fmt.Errorf("build patch: %w", err).Error())
		return nil
	}
	built.ProjectID = projectID
	built.Author = string(agents.RoleArchitect) + "/scaffold"

	proposed, err := e.patches.Propose(built)
	if err != nil {
		e.emitScaffoldSkip(projectID, fmt.Errorf("propose: %w", err).Error())
		return nil
	}
	if proposed.Status == patch.StatusRejected {
		e.emitScaffoldSkip(projectID, "propose: "+joinIssues(proposed.Issues))
		return nil
	}

	applied, err := e.patches.Apply(proposed.ID)
	if err != nil {
		e.emitScaffoldSkip(projectID, fmt.Errorf("apply: %w", err).Error())
		return nil
	}

	if workspaceID != "" {
		if err := e.applier.Apply(ctx, bearer, workspaceID, applied); err != nil {
			// Runtime apply failure is reported as a soft skip — the
			// in-memory project still has the files, the gate phase will
			// detect drift. We do not roll back the in-memory state.
			e.emit(projectID, domain.Event{
				ID:        newEventID(),
				Step:      StepScaffold,
				Status:    StatusFailed,
				Message:   fmtErr(ErrCodeRuntimeError, "scaffold workspace apply: "+err.Error()),
				CreatedAt: time.Now().UTC(),
			})
		}
	}

	e.emit(projectID, domain.Event{
		ID:        newEventID(),
		Step:      StepScaffold,
		Status:    StatusDone,
		Message:   "scaffold_done files=" + itoaPositive(len(applied.Changes)) + " starter=" + applied.Title,
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

// emitScaffoldSkip publishes a single SSE event noting that scaffolding was
// skipped, with the underlying reason. The status is "done" so the UI
// doesn't show a red error — the run continues normally without a starter.
func (e *Engine) emitScaffoldSkip(projectID, reason string) {
	e.emit(projectID, domain.Event{
		ID:        newEventID(),
		Step:      StepScaffold,
		Status:    StatusDone,
		Message:   "scaffold_skipped: " + reason,
		CreatedAt: time.Now().UTC(),
	})
}

// stackSpecFromDomain maps the Architect's persisted StackDecision into the
// scaffolder's StackSpec. We pass framework + language + style through as
// lowercase strings; the scaffolder is permissive on phrasing.
func stackSpecFromDomain(s domain.StackDecision) scaffold.StackSpec {
	out := scaffold.StackSpec{
		Framework: s.Frontend,
		Language:  inferLanguage(s.Frontend, s.Backend),
		Database:  s.Storage,
		Auth:      strings.TrimSpace(s.Auth) != "" && !strings.EqualFold(s.Auth, "none"),
		Style:     inferStyle(s.Frontend),
	}
	// When the frontend is missing or non-web, fall back to the backend
	// string so a Go-only service still gets the go-chi starter.
	if strings.TrimSpace(out.Framework) == "" {
		out.Framework = s.Backend
	}
	return out
}

// inferLanguage maps a frontend/backend description into a coarse language
// key the scaffolder understands. We default to "ts" for web stacks since
// every web starter we ship is TypeScript.
func inferLanguage(frontend, backend string) string {
	f := strings.ToLower(frontend + " " + backend)
	switch {
	case strings.Contains(f, "typescript"), strings.Contains(f, "ts"):
		return "ts"
	case strings.Contains(f, "javascript"), strings.Contains(f, "js"):
		return "js"
	case strings.Contains(f, "golang"), strings.Contains(f, "go"):
		return "go"
	default:
		return "ts"
	}
}

// inferStyle reads the frontend string for a CSS library hint. MUI is the
// Ironflyer default so an absent or unknown hint resolves to "mui".
func inferStyle(frontend string) string {
	f := strings.ToLower(frontend)
	switch {
	case strings.Contains(f, "tailwind"):
		return "tailwind"
	case strings.Contains(f, "mui"), strings.Contains(f, "material"):
		return "mui"
	case strings.Contains(f, "plain"), strings.Contains(f, "vanilla"):
		return "plain"
	default:
		return "mui"
	}
}

