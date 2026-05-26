package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Kind enumerates every notification template the Dispatcher knows how
// to render. The closed enum lets the worker switch on Kind to pick the
// renderer + in-app surface for a given payload.
type Kind string

const (
	KindWelcome       Kind = "welcome"
	KindPasswordReset Kind = "password_reset"
	KindReceipt       Kind = "receipt"
	KindRunComplete   Kind = "run_complete"
	KindGateFailed    Kind = "gate_failed"
	KindDeployDone    Kind = "deploy_done"
	KindBudgetWarning Kind = "budget_warning"
)

// WelcomePayload is the data bag for KindWelcome.
type WelcomePayload struct {
	Name            string `json:"name,omitempty"`
	Email           string `json:"email,omitempty"`
	SignupSessionID string `json:"signupSessionId,omitempty"`
}

// PasswordResetPayload is the data bag for KindPasswordReset.
type PasswordResetPayload struct {
	Name     string        `json:"name,omitempty"`
	ResetURL string        `json:"resetUrl"`
	TTL      time.Duration `json:"ttlNs,omitempty"`
	TokenJTI string        `json:"tokenJti,omitempty"`
}

// ReceiptPayload is the data bag for KindReceipt.
type ReceiptPayload struct {
	Name            string `json:"name,omitempty"`
	Currency        string `json:"currency"`
	AmountCents     int    `json:"amountCents"`
	TransactionID   string `json:"transactionId"`
	StripeSessionID string `json:"stripeSessionId,omitempty"`
}

// RunCompletePayload describes a finished project run.
type RunCompletePayload struct {
	ProjectName string `json:"projectName"`
	ProjectID   string `json:"projectId"`
	ExecutionID string `json:"executionId,omitempty"`
}

// GateFailedPayload describes a gate failure.
type GateFailedPayload struct {
	ProjectName string `json:"projectName"`
	ProjectID   string `json:"projectId"`
	GateName    string `json:"gateName"`
	Reason      string `json:"reason,omitempty"`
	ExecutionID string `json:"executionId,omitempty"`
}

// DeployDonePayload describes a successful deploy.
type DeployDonePayload struct {
	ProjectName string `json:"projectName"`
	ProjectID   string `json:"projectId"`
	ExecutionID string `json:"executionId,omitempty"`
}

// BudgetWarningPayload describes a wallet cap warning. ProjectID +
// YYYYMMDD form the idempotency key so the same user does not receive
// more than one budget warning per project per day.
type BudgetWarningPayload struct {
	ProjectName string `json:"projectName"`
	ProjectID   string `json:"projectId"`
}

// alwaysSendKinds are onboarding / security / legal kinds that bypass
// PrefsStore.PauseAll. Welcome, PasswordReset, and Receipt cannot be
// silenced.
var alwaysSendKinds = map[Kind]bool{
	KindWelcome:       true,
	KindPasswordReset: true,
	KindReceipt:       true,
}

// Dispatcher is the single entry point for every notification. It
// resolves the user's per-topic ChannelPref, computes the idempotency
// key, and enqueues onto the OutboxStore. The Worker drains the outbox
// and delivers to email + in-app.
type Dispatcher struct {
	outbox       OutboxStore
	prefs        PrefsStore
	logger       zerolog.Logger
	dashboardURL string
	from         string
	sender       EmailSender
}

// NewDispatcher constructs a Dispatcher. outbox may be nil during early
// boot — Dispatch returns a typed error so call sites can fail loud.
// sender is retained so legacy direct-send call sites (none today) can
// fall back; the worker is the canonical delivery path.
func NewDispatcher(sender EmailSender, prefs PrefsStore, dashboardURL, from string, logger zerolog.Logger) *Dispatcher {
	if sender == nil {
		sender = NewNoopSender(logger)
	}
	return &Dispatcher{
		sender:       sender,
		prefs:        prefs,
		logger:       logger,
		dashboardURL: dashboardURL,
		from:         from,
	}
}

// WithOutbox attaches the OutboxStore. Required for any real delivery —
// before this is set, Dispatch returns a typed error.
func (d *Dispatcher) WithOutbox(outbox OutboxStore) *Dispatcher {
	d.outbox = outbox
	return d
}

// From returns the configured sender address.
func (d *Dispatcher) From() string {
	if d == nil {
		return ""
	}
	return d.from
}

// DashboardURL returns the configured dashboard base URL.
func (d *Dispatcher) DashboardURL() string {
	if d == nil {
		return ""
	}
	return d.dashboardURL
}

// Sender returns the configured EmailSender — the Worker pulls this so
// every call site shares one sender instance.
func (d *Dispatcher) Sender() EmailSender {
	if d == nil {
		return nil
	}
	return d.sender
}

// Dispatch enqueues a notification for the given user. It computes the
// idempotency key, consults PrefsStore for per-topic channel targets,
// and writes one row to the outbox. The Worker picks it up on its next
// poll. Always-send kinds bypass PauseAll but still honour the per-
// topic ChannelPref.
func (d *Dispatcher) Dispatch(ctx context.Context, userID, email string, kind Kind, payload any) error {
	if d == nil {
		return errors.New("notify: dispatcher not configured")
	}
	if d.outbox == nil {
		return errors.New("notify: outbox not wired")
	}
	if userID == "" {
		return errors.New("notify: dispatch missing userID")
	}

	emailTarget, inAppTarget := d.resolveChannels(ctx, userID, kind)
	if !emailTarget && !inAppTarget {
		return nil
	}

	refID, err := payloadRefID(kind, payload)
	if err != nil {
		return err
	}
	key := userID + ":" + string(kind) + ":" + refID

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: marshal payload: %w", err)
	}

	now := time.Now().UTC()
	item := OutboxItem{
		ID:            uuid.NewString(),
		UserID:        userID,
		Kind:          kind,
		Payload:       raw,
		EmailTarget:   emailTarget && email != "",
		InAppTarget:   inAppTarget,
		NextAttemptAt: now,
		CreatedAt:     now,
	}
	if _, err := d.outbox.Enqueue(ctx, item, key); err != nil {
		return fmt.Errorf("notify: enqueue: %w", err)
	}
	d.logger.Debug().
		Str("user", userID).
		Str("kind", string(kind)).
		Bool("email", item.EmailTarget).
		Bool("inapp", item.InAppTarget).
		Str("idem", key).
		Msg("notify: dispatched")
	return nil
}

// resolveChannels reads the user's prefs and returns the (email,
// in-app) channel mask for the kind. Always-send kinds short-circuit
// PauseAll. Welcome / Reset are not user-configurable so they always
// deliver on both channels (subject to email-address availability).
func (d *Dispatcher) resolveChannels(ctx context.Context, userID string, kind Kind) (bool, bool) {
	if kind == KindWelcome || kind == KindPasswordReset {
		return true, true
	}
	if d.prefs == nil {
		return true, true
	}
	rule, err := d.prefs.Get(ctx, userID)
	if err != nil {
		d.logger.Debug().Err(err).Str("user", userID).Msg("notify: prefs lookup failed")
		return true, true
	}
	if rule.PauseAll && !alwaysSendKinds[kind] {
		return false, false
	}
	switch kind {
	case KindReceipt:
		return rule.OnReceipt.Email, rule.OnReceipt.InApp
	case KindRunComplete:
		return rule.OnRunComplete.Email, rule.OnRunComplete.InApp
	case KindGateFailed:
		return rule.OnGateFailed.Email, rule.OnGateFailed.InApp
	case KindDeployDone:
		return rule.OnDeployDone.Email, rule.OnDeployDone.InApp
	case KindBudgetWarning:
		return rule.OnBudgetWarning.Email, rule.OnBudgetWarning.InApp
	}
	return true, true
}

// payloadRefID extracts the idempotency-stable identifier from each
// payload type. The shape per Kind:
//
//   welcome:        signupSessionID, falling back to email
//   password_reset: tokenJTI
//   receipt:        stripeSessionID, falling back to transactionID
//   run_complete:   executionID, falling back to projectID
//   gate_failed:    executionID + ":" + gateName, falling back to project+gate
//   deploy_done:    executionID, falling back to projectID
//   budget_warning: projectID + ":" + YYYYMMDD (one per project per day)
func payloadRefID(kind Kind, payload any) (string, error) {
	switch kind {
	case KindWelcome:
		p, ok := payload.(WelcomePayload)
		if !ok {
			return "", fmt.Errorf("notify: kind=%s expected WelcomePayload, got %T", kind, payload)
		}
		if p.SignupSessionID != "" {
			return p.SignupSessionID, nil
		}
		return p.Email, nil
	case KindPasswordReset:
		p, ok := payload.(PasswordResetPayload)
		if !ok {
			return "", fmt.Errorf("notify: kind=%s expected PasswordResetPayload, got %T", kind, payload)
		}
		if p.TokenJTI != "" {
			return p.TokenJTI, nil
		}
		return p.ResetURL, nil
	case KindReceipt:
		p, ok := payload.(ReceiptPayload)
		if !ok {
			return "", fmt.Errorf("notify: kind=%s expected ReceiptPayload, got %T", kind, payload)
		}
		if p.StripeSessionID != "" {
			return p.StripeSessionID, nil
		}
		return p.TransactionID, nil
	case KindRunComplete:
		p, ok := payload.(RunCompletePayload)
		if !ok {
			return "", fmt.Errorf("notify: kind=%s expected RunCompletePayload, got %T", kind, payload)
		}
		if p.ExecutionID != "" {
			return p.ExecutionID, nil
		}
		return p.ProjectID, nil
	case KindGateFailed:
		p, ok := payload.(GateFailedPayload)
		if !ok {
			return "", fmt.Errorf("notify: kind=%s expected GateFailedPayload, got %T", kind, payload)
		}
		base := p.ExecutionID
		if base == "" {
			base = p.ProjectID
		}
		return base + ":" + p.GateName, nil
	case KindDeployDone:
		p, ok := payload.(DeployDonePayload)
		if !ok {
			return "", fmt.Errorf("notify: kind=%s expected DeployDonePayload, got %T", kind, payload)
		}
		if p.ExecutionID != "" {
			return p.ExecutionID, nil
		}
		return p.ProjectID, nil
	case KindBudgetWarning:
		p, ok := payload.(BudgetWarningPayload)
		if !ok {
			return "", fmt.Errorf("notify: kind=%s expected BudgetWarningPayload, got %T", kind, payload)
		}
		return p.ProjectID + ":" + time.Now().UTC().Format("20060102"), nil
	}
	return "", fmt.Errorf("notify: unknown kind %q", kind)
}

// renderEmail produces the EmailContent for a Kind+payload pair. The
// Worker calls this when delivering to the email channel.
func renderEmail(kind Kind, raw []byte, dashboardURL string) (EmailContent, error) {
	switch kind {
	case KindWelcome:
		var p WelcomePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return EmailContent{}, err
		}
		return renderWelcome(p.Name, p.Email, dashboardURL), nil
	case KindPasswordReset:
		var p PasswordResetPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return EmailContent{}, err
		}
		ttl := p.TTL
		if ttl <= 0 {
			ttl = 30 * time.Minute
		}
		return renderPasswordReset(p.Name, p.ResetURL, ttl, dashboardURL), nil
	case KindReceipt:
		var p ReceiptPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return EmailContent{}, err
		}
		return renderReceipt(p.Name, p.Currency, p.AmountCents, p.TransactionID, dashboardURL), nil
	case KindRunComplete:
		var p RunCompletePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return EmailContent{}, err
		}
		return renderRunComplete(p.ProjectName, p.ProjectID, dashboardURL), nil
	case KindGateFailed:
		var p GateFailedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return EmailContent{}, err
		}
		return renderGateFailed(p.ProjectName, p.ProjectID, p.GateName, p.Reason, dashboardURL), nil
	case KindDeployDone:
		var p DeployDonePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return EmailContent{}, err
		}
		return renderDeployDone(p.ProjectName, p.ProjectID, dashboardURL), nil
	case KindBudgetWarning:
		var p BudgetWarningPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return EmailContent{}, err
		}
		return renderBudgetWarning(p.ProjectName, dashboardURL), nil
	}
	return EmailContent{}, fmt.Errorf("notify: render unknown kind %q", kind)
}

// renderInApp produces the durable Notification row for a Kind+payload.
// Severity follows the topic: info for success/onboarding, warning for
// budget, critical for gate failures.
func renderInApp(kind Kind, raw []byte, dashboardURL string) (Notification, error) {
	switch kind {
	case KindWelcome:
		var p WelcomePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return Notification{}, err
		}
		return Notification{
			Kind:     string(kind),
			Title:    "Welcome to Ironflyer",
			Body:     "Your workspace is set up — sign in to start a build from a Figma file or a prompt.",
			Link:     dashboardURL,
			Severity: "info",
		}, nil
	case KindPasswordReset:
		var p PasswordResetPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return Notification{}, err
		}
		return Notification{
			Kind:     string(kind),
			Title:    "Password reset requested",
			Body:     "A password reset link was just emailed to you.",
			Link:     p.ResetURL,
			Severity: "warning",
		}, nil
	case KindReceipt:
		var p ReceiptPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return Notification{}, err
		}
		return Notification{
			Kind:     string(kind),
			Title:    "Wallet top-up confirmed",
			Body:     "Funds are available for paid executions.",
			Link:     dashboardURL,
			Severity: "info",
		}, nil
	case KindRunComplete:
		var p RunCompletePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return Notification{}, err
		}
		return Notification{
			Kind:     string(kind),
			Title:    p.ProjectName + " run completed",
			Body:     "Every gate passed and the project is ready for review.",
			Link:     projectLink(dashboardURL, p.ProjectID),
			Severity: "info",
		}, nil
	case KindGateFailed:
		var p GateFailedPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return Notification{}, err
		}
		body := "The " + p.GateName + " gate failed in " + p.ProjectName + "."
		if p.Reason != "" {
			body += " Reason: " + p.Reason
		}
		return Notification{
			Kind:     string(kind),
			Title:    "Gate failed in " + p.ProjectName,
			Body:     body,
			Link:     projectLink(dashboardURL, p.ProjectID),
			Severity: "critical",
		}, nil
	case KindDeployDone:
		var p DeployDonePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return Notification{}, err
		}
		return Notification{
			Kind:     string(kind),
			Title:    p.ProjectName + " is live",
			Body:     "Deployment completed successfully.",
			Link:     projectLink(dashboardURL, p.ProjectID),
			Severity: "info",
		}, nil
	case KindBudgetWarning:
		var p BudgetWarningPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return Notification{}, err
		}
		return Notification{
			Kind:     string(kind),
			Title:    "Budget warning",
			Body:     "Usage for " + p.ProjectName + " is approaching the plan cap.",
			Link:     dashboardURL,
			Severity: "warning",
		}, nil
	}
	return Notification{}, fmt.Errorf("notify: in-app render unknown kind %q", kind)
}
