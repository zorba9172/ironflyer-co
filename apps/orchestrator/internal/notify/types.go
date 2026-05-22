// Package notify is the user-facing notification layer. It bridges the
// finisher's SSE event bus to two outbound channels:
//   1. Email — Resend / SendGrid / Noop, behind an EmailSender interface.
//   2. Webhooks — fan-out to user-registered HTTPS endpoints, delegated to
//      the webhooks.Dispatcher so HMAC signing + retries are centralized.
//
// Rules are per-user (NotificationRule), persisted via PrefsStore. The
// Engine subscribes per-project (see Engine.Subscribe) and translates each
// raw domain.Event into either an email, a webhook fan-out, or both.
package notify

// NotificationRule captures per-user channel + topic preferences. The web
// settings page (settings/notifications) reads and writes this struct
// verbatim — keep the JSON tags stable.
type NotificationRule struct {
	UserID          string `json:"userId"`
	Email           string `json:"email"`
	OnRunComplete   bool   `json:"onRunComplete"`
	OnGateFailed    bool   `json:"onGateFailed"`
	OnDeployDone    bool   `json:"onDeployDone"`
	OnBudgetWarning bool   `json:"onBudgetWarning"`
	ChannelEmail    bool   `json:"channelEmail"`
	ChannelWebhook  bool   `json:"channelWebhook"`
}

// DefaultRule returns a sane opt-in baseline: webhooks on, email on for the
// important moments but not the noisy ones. The email field is filled in
// per-user when the rule is first persisted.
func DefaultRule(userID, email string) NotificationRule {
	return NotificationRule{
		UserID:          userID,
		Email:           email,
		OnRunComplete:   true,
		OnGateFailed:    true,
		OnDeployDone:    true,
		OnBudgetWarning: true,
		ChannelEmail:    true,
		ChannelWebhook:  true,
	}
}
