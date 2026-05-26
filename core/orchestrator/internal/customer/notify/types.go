// Package notify is the user-facing notification layer. It bridges the
// finisher's SSE event bus to two outbound channels:
//   1. Email — Resend / SendGrid / Noop, behind an EmailSender interface.
//   2. In-app bell — durable rows in the notifications table, surfaced
//      via the notificationStream GraphQL subscription.
//
// Rules are per-user (NotificationRule), persisted via PrefsStore. The
// Engine subscribes per-project (see Engine.Subscribe) and routes each
// finisher.Event through the Dispatcher which writes to the outbox; the
// Worker drains the outbox and fans out to email + in-app.
package notify

import "time"

// ChannelPref captures whether a single topic should reach the user via
// email, in-app, or both. Both default to true.
type ChannelPref struct {
	Email bool `json:"email"`
	InApp bool `json:"inApp"`
}

// NotificationRule captures per-user channel + topic preferences. The web
// settings page reads and writes this struct verbatim; JSON tags are
// stable.
type NotificationRule struct {
	UserID          string      `json:"userId"`
	Email           string      `json:"email"`
	PauseAll        bool        `json:"pauseAll"`
	OnRunComplete   ChannelPref `json:"onRunComplete"`
	OnGateFailed    ChannelPref `json:"onGateFailed"`
	OnDeployDone    ChannelPref `json:"onDeployDone"`
	OnBudgetWarning ChannelPref `json:"onBudgetWarning"`
	OnReceipt       ChannelPref `json:"onReceipt"`
}

// DefaultRule returns the opt-in baseline: every topic enabled on both
// channels.
func DefaultRule(userID, email string) NotificationRule {
	on := ChannelPref{Email: true, InApp: true}
	return NotificationRule{
		UserID:          userID,
		Email:           email,
		PauseAll:        false,
		OnRunComplete:   on,
		OnGateFailed:    on,
		OnDeployDone:    on,
		OnBudgetWarning: on,
		OnReceipt:       on,
	}
}

// Notification is the durable in-app row. The bell pulls these via the
// notifications GraphQL query and the notificationStream subscription.
type Notification struct {
	ID        string
	UserID    string
	Kind      string
	Title     string
	Body      string
	Link      string
	Severity  string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// OutboxItem mirrors a row in notification_outbox. Payload is the JSON-
// serialised Dispatcher payload so the worker can reconstruct the
// rendered email + in-app content per Kind.
type OutboxItem struct {
	ID             string
	UserID         string
	Kind           Kind
	Payload        []byte
	EmailTarget    bool
	InAppTarget    bool
	Attempts       int
	NextAttemptAt  time.Time
	LastError      string
	DeliveredAt    *time.Time
	DeadLetteredAt *time.Time
	EmailSentAt    *time.Time
	InAppSentAt    *time.Time
	CreatedAt      time.Time
}
