package notify

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/ai/finisher"
	"ironflyer/core/orchestrator/internal/operations/store"
)

// Engine listens to finisher events and routes them to the user's email
// channel based on the user's NotificationRule. The webhook fan-out has
// been removed in V22 alongside the rest of the legacy webhooks
// package; email remains the single transactional channel until a V22
// agent introduces ledger/wallet notifications.
//
// The engine owns no state of its own beyond cached subscriptions; the
// per-project goroutine pool exits when Stop is called or the parent
// context is cancelled.
type Engine struct {
	projects     store.Store
	prefs        PrefsStore
	sender       EmailSender
	logger       zerolog.Logger
	dashboardURL string

	mu      sync.Mutex
	cancels map[string]context.CancelFunc // projectID → cancel for its subscription goroutine
}

// projectDeleteRegistrar is the optional capability the engine looks for on
// the project store: when the store implements it, the engine registers
// RemoveProject so per-project subscriptions get torn down the moment a
// project is deleted. Implemented by store.MemoryStore and store.SurrealStore
// — kept here as a local interface so notify doesn't have to grow its store
// import surface and so other store backends can opt in without code churn.
type projectDeleteRegistrar interface {
	RegisterDeleteHook(fn func(projectID string))
}

// NewEngine constructs an engine. Pass a NoopSender when no email provider
// is configured.
//
// If the supplied project store implements RegisterDeleteHook, the engine
// auto-registers RemoveProject so the per-project cancels map cannot leak
// past a project deletion. Callers can also drive RemoveProject directly.
func NewEngine(projects store.Store, prefs PrefsStore, sender EmailSender, logger zerolog.Logger) *Engine {
	if sender == nil {
		sender = NewNoopSender(logger)
	}
	e := &Engine{
		projects: projects,
		prefs:    prefs,
		sender:   sender,
		logger:   logger,
		cancels:  make(map[string]context.CancelFunc),
	}
	if reg, ok := projects.(projectDeleteRegistrar); ok {
		reg.RegisterDeleteHook(e.RemoveProject)
	}
	return e
}

// RemoveProject tears down the per-project subscription goroutine and clears
// the map entry so the cancels map cannot grow without bound across the
// lifetime of the process. Safe to call for unknown project IDs (no-op) and
// safe to call concurrently with SubscribeProject — both serialize on e.mu.
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

// dropCancel deletes the cancels entry once a subscription goroutine exits
// for any reason (orchestrator closed the channel, ctx cancelled, etc.). It
// is deliberately tolerant of the entry already being gone — RemoveProject
// may have removed it just before the goroutine noticed.
func (e *Engine) dropCancel(projectID string) {
	e.mu.Lock()
	delete(e.cancels, projectID)
	e.mu.Unlock()
}

// WithDashboardURL configures the absolute URL used by email CTAs. Returns
// the engine for chained configuration during startup.
func (e *Engine) WithDashboardURL(url string) *Engine {
	e.dashboardURL = url
	return e
}

// SubscribeAll wires the engine to every currently-known project. Call this
// from main.go after constructing the orchestrator engine. New projects
// created later will be picked up by SubscribeProject when the relevant
// HTTP handler invokes it (left to the wire layer to call).
func (e *Engine) SubscribeAll(ctx context.Context, orchestrator *finisher.Engine) {
	for _, p := range e.projects.List() {
		e.SubscribeProject(ctx, orchestrator, p.ID)
	}
}

// SubscribeProject attaches a goroutine that drains finisher events for one
// project and translates them into emails. Calling more than once for the
// same project is a no-op.
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

// Stop cancels every active project subscription and waits for the goroutines
// to drain. Safe to call multiple times.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.cancels {
		c()
	}
	e.cancels = make(map[string]context.CancelFunc)
}

// handle dispatches a single event. It loads the project (to recover the
// owner ID + name) and the owner's preferences, then routes per channel.
func (e *Engine) handle(ctx context.Context, projectID string, evt domain.Event) {
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return
	}
	ownerID := proj.OwnerID
	if ownerID == "" {
		// Public seed project — nobody to notify.
		return
	}
	rule, err := e.prefs.Get(ctx, ownerID)
	if err != nil {
		e.logger.Debug().Err(err).Str("user", ownerID).Msg("notify: prefs lookup failed")
		return
	}

	topic, content, ok := e.classify(evt, proj.Name, proj.ID)
	if !ok {
		return
	}

	if !topicEnabled(rule, topic) {
		return
	}

	if rule.ChannelEmail && rule.Email != "" {
		if err := e.sender.Send(ctx, rule.Email, content.Subject, content.HTMLBody, content.TextBody); err != nil {
			e.logger.Warn().Err(err).Str("to", rule.Email).Str("topic", string(topic)).Msg("notify: email send failed")
		}
	}
}

// topic is the closed set of notification categories the rule engine knows
// how to filter on.
type topic string

const (
	topicRunComplete   topic = "run_complete"
	topicGateFailed    topic = "gate_failed"
	topicDeployDone    topic = "deploy_done"
	topicBudgetWarning topic = "budget_warning"
)

// classify inspects an event and returns its topic + rendered content. The
// boolean is false when the event doesn't map to a milestone we email about.
func (e *Engine) classify(evt domain.Event, projectName, projectID string) (topic, EmailContent, bool) {
	switch evt.Step {
	case finisher.StepRun:
		if evt.Status == finisher.StatusDone {
			return topicRunComplete, renderRunComplete(projectName, projectID, e.dashboardURL), true
		}
	case finisher.StepGate:
		if evt.Status == finisher.StatusFailed {
			gate := string(evt.Gate)
			if gate == string(domain.GateDeploy) {
				return topicGateFailed, renderGateFailed(projectName, projectID, gate, evt.Message, e.dashboardURL), true
			}
			return topicGateFailed, renderGateFailed(projectName, projectID, gate, evt.Message, e.dashboardURL), true
		}
		if evt.Status == finisher.StatusDone && evt.Gate == domain.GateDeploy {
			return topicDeployDone, renderDeployDone(projectName, projectID, e.dashboardURL), true
		}
	}
	return "", EmailContent{}, false
}

// topicEnabled honours the per-topic toggles on the rule. Channel toggles
// are checked separately by the caller.
func topicEnabled(rule NotificationRule, t topic) bool {
	switch t {
	case topicRunComplete:
		return rule.OnRunComplete
	case topicGateFailed:
		return rule.OnGateFailed
	case topicDeployDone:
		return rule.OnDeployDone
	case topicBudgetWarning:
		return rule.OnBudgetWarning
	}
	return false
}

// EmitBudgetWarning is the entry point billing code calls when a user is
// nearing their cap. It sidesteps the SSE path because budget warnings
// aren't project-scoped events.
func (e *Engine) EmitBudgetWarning(ctx context.Context, userID, projectName string) {
	rule, err := e.prefs.Get(ctx, userID)
	if err != nil || !rule.OnBudgetWarning || !rule.ChannelEmail || rule.Email == "" {
		return
	}
	c := renderBudgetWarning(projectName, e.dashboardURL)
	if err := e.sender.Send(ctx, rule.Email, c.Subject, c.HTMLBody, c.TextBody); err != nil {
		e.logger.Warn().Err(err).Str("user", userID).Msg("notify: budget warning email failed")
	}
}
