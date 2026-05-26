package notify

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/finisher"
	"ironflyer/core/orchestrator/internal/operations/store"
)

// Engine is a thin adapter from finisher events to the Dispatcher. It
// owns per-project subscription goroutines so the Dispatcher receives a
// typed payload for each milestone event (run complete, gate failed,
// deploy done). All persistence (outbox, in-app, email) flows through
// the Dispatcher and the Worker.
type Engine struct {
	projects   store.Store
	dispatcher *Dispatcher
	prefs      PrefsStore
	logger     zerolog.Logger

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// projectDeleteRegistrar is the optional capability the engine looks for
// on the project store: when the store implements it, the engine
// registers RemoveProject so per-project subscriptions get torn down
// the moment a project is deleted.
type projectDeleteRegistrar interface {
	RegisterDeleteHook(fn func(projectID string))
}

// NewEngine constructs an engine. dispatcher may be nil — the engine
// degrades to a logging no-op and warns at boot.
func NewEngine(projects store.Store, prefs PrefsStore, dispatcher *Dispatcher, logger zerolog.Logger) *Engine {
	e := &Engine{
		projects:   projects,
		prefs:      prefs,
		dispatcher: dispatcher,
		logger:     logger,
		cancels:    make(map[string]context.CancelFunc),
	}
	if dispatcher == nil {
		logger.Warn().Msg("notify: engine constructed without dispatcher — finisher events will no-op")
	}
	if reg, ok := projects.(projectDeleteRegistrar); ok {
		reg.RegisterDeleteHook(e.RemoveProject)
	}
	return e
}

// RemoveProject tears down the per-project subscription goroutine.
func (e *Engine) RemoveProject(projectID string) {
	e.mu.Lock()
	cancel, ok := e.cancels[projectID]
	if ok {
		delete(e.cancels, projectID)
	}
	e.mu.Unlock()
	if ok {
		cancel()
	}
}

func (e *Engine) dropCancel(projectID string) {
	e.mu.Lock()
	delete(e.cancels, projectID)
	e.mu.Unlock()
}

// SubscribeAll wires the engine to every currently-known project.
func (e *Engine) SubscribeAll(ctx context.Context, orchestrator *finisher.Engine) {
	for _, p := range e.projects.List() {
		e.SubscribeProject(ctx, orchestrator, p.ID)
	}
}

// SubscribeProject attaches a goroutine that drains finisher events for
// one project. Calling more than once for the same project is a no-op.
func (e *Engine) SubscribeProject(ctx context.Context, orchestrator *finisher.Engine, projectID string) {
	e.mu.Lock()
	if _, ok := e.cancels[projectID]; ok {
		e.mu.Unlock()
		return
	}
	subCtx, cancel := context.WithCancel(ctx)
	e.cancels[projectID] = cancel
	e.mu.Unlock()

	ch, unsub := orchestrator.Subscribe(projectID)
	go func() {
		defer unsub()
		defer e.dropCancel(projectID)
		for {
			select {
			case <-subCtx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				e.handle(subCtx, projectID, evt)
			}
		}
	}()
}

// Stop cancels every active project subscription.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.cancels {
		c()
	}
	e.cancels = make(map[string]context.CancelFunc)
}

// handle classifies the event and dispatches the matching Kind.
func (e *Engine) handle(ctx context.Context, projectID string, evt domain.Event) {
	if e.dispatcher == nil {
		return
	}
	proj, err := e.projects.Get(projectID)
	if err != nil || proj.OwnerID == "" {
		return
	}
	ownerEmail := e.lookupEmail(ctx, proj.OwnerID)

	switch evt.Step {
	case finisher.StepRun:
		if evt.Status == finisher.StatusDone {
			_ = e.dispatcher.Dispatch(ctx, proj.OwnerID, ownerEmail, KindRunComplete, RunCompletePayload{
				ProjectName: proj.Name,
				ProjectID:   proj.ID,
			})
		}
	case finisher.StepGate:
		if evt.Status == finisher.StatusFailed {
			_ = e.dispatcher.Dispatch(ctx, proj.OwnerID, ownerEmail, KindGateFailed, GateFailedPayload{
				ProjectName: proj.Name,
				ProjectID:   proj.ID,
				GateName:    string(evt.Gate),
				Reason:      evt.Message,
			})
			return
		}
		if evt.Status == finisher.StatusDone && evt.Gate == domain.GateDeploy {
			_ = e.dispatcher.Dispatch(ctx, proj.OwnerID, ownerEmail, KindDeployDone, DeployDonePayload{
				ProjectName: proj.Name,
				ProjectID:   proj.ID,
			})
		}
	}
}

// lookupEmail returns the user's preferred contact email from the
// PrefsStore. Empty string when not wired or not on file.
func (e *Engine) lookupEmail(ctx context.Context, userID string) string {
	if e.prefs == nil {
		return ""
	}
	rule, err := e.prefs.Get(ctx, userID)
	if err != nil {
		return ""
	}
	return rule.Email
}

// EmitBudgetWarning is the entry point billing code calls when a user
// is nearing their cap. Routes through the Dispatcher so the rule
// engine + outbox stay consistent.
func (e *Engine) EmitBudgetWarning(ctx context.Context, userID, projectID, projectName string) {
	if e.dispatcher == nil {
		return
	}
	email := e.lookupEmail(ctx, userID)
	_ = e.dispatcher.Dispatch(ctx, userID, email, KindBudgetWarning, BudgetWarningPayload{
		ProjectName: projectName,
		ProjectID:   projectID,
	})
}
