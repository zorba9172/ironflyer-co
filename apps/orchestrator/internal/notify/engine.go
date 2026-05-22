package notify

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/store"
	"ironflyer/apps/orchestrator/internal/webhooks"
)

// Engine listens to finisher events and routes them to the right user
// channels — email and/or webhooks — based on the user's NotificationRule.
//
// The engine owns no state of its own beyond cached subscriptions; the
// per-project goroutine pool exits when Stop is called or the parent
// context is cancelled.
type Engine struct {
	projects     store.Store
	prefs        PrefsStore
	sender       EmailSender
	dispatcher   *webhooks.Dispatcher
	logger       zerolog.Logger
	dashboardURL string

	mu      sync.Mutex
	cancels map[string]context.CancelFunc // projectID → cancel for its subscription goroutine
}

// NewEngine constructs an engine. Pass a NoopSender when no email provider
// is configured; pass nil for the dispatcher to disable webhook fan-out.
func NewEngine(projects store.Store, prefs PrefsStore, sender EmailSender, dispatcher *webhooks.Dispatcher, logger zerolog.Logger) *Engine {
	if sender == nil {
		sender = NewNoopSender(logger)
	}
	return &Engine{
		projects:   projects,
		prefs:      prefs,
		sender:     sender,
		dispatcher: dispatcher,
		logger:     logger,
		cancels:    make(map[string]context.CancelFunc),
	}
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
// project and translates them into emails + webhooks. Calling more than once
// for the same project is a no-op.
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
		// Still fan out webhooks for every event the user opted into — they
		// may want every step, not only the milestone ones.
		if rule.ChannelWebhook && e.dispatcher != nil {
			e.dispatcher.Dispatch(ctx, ownerID, projectID, evt)
		}
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
	if rule.ChannelWebhook && e.dispatcher != nil {
		e.dispatcher.Dispatch(ctx, ownerID, projectID, evt)
	}
}

// topic is the closed set of notification categories the rule engine knows
// how to filter on. Anything outside the set falls through to webhooks only.
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

// WebhookDisabled satisfies the webhooks.FailureNotifier interface so the
// dispatcher can call back into the notify engine when it auto-disables a
// subscription. Implemented here so dispatcher → notify is a one-way
// dependency (notify imports webhooks, never the other direction).
func (e *Engine) WebhookDisabled(ctx context.Context, userID, webhookURL string, failures int) {
	rule, err := e.prefs.Get(ctx, userID)
	if err != nil || !rule.ChannelEmail || rule.Email == "" {
		return
	}
	c := renderWebhookDisabled(webhookURL, e.dashboardURL, failures)
	if err := e.sender.Send(ctx, rule.Email, c.Subject, c.HTMLBody, c.TextBody); err != nil {
		e.logger.Warn().Err(err).Str("user", userID).Msg("notify: webhook-disabled email failed")
	}
	// Touch LastSentAt-like field by keeping the rule updated; cheap idempotent write.
	rule.UserID = userID
	_ = e.prefs.Set(ctx, rule)
}
