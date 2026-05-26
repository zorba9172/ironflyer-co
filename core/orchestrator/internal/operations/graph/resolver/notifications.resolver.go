package resolver

import (
	"context"
	"errors"

	"ironflyer/core/orchestrator/internal/customer/notify"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

// MarkNotificationRead flips a single notification to read.
func (r *mutationResolver) MarkNotificationRead(ctx context.Context, id string) (*model.Notification, error) {
	if r.NotifyStore == nil {
		return nil, gqlNotConfigured("notifications")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	n, err := r.NotifyStore.MarkRead(ctx, u.ID, id)
	if err != nil {
		if errors.Is(err, notify.ErrNotFound) {
			return nil, errors.New("notification not found")
		}
		return nil, err
	}
	return notificationToGraphQL(n), nil
}

// MarkAllNotificationsRead flips every unread row for the user.
func (r *mutationResolver) MarkAllNotificationsRead(ctx context.Context) (int, error) {
	if r.NotifyStore == nil {
		return 0, gqlNotConfigured("notifications")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return 0, err
	}
	return r.NotifyStore.MarkAllRead(ctx, u.ID)
}

// UpdateNotificationPreferences upserts the user's per-topic prefs.
func (r *mutationResolver) UpdateNotificationPreferences(ctx context.Context, input model.NotificationPreferencesInput) (*model.NotificationPreferences, error) {
	if r.NotifyPrefs == nil {
		return nil, gqlNotConfigured("notification preferences")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	current, err := r.NotifyPrefs.Get(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if current.UserID == "" {
		current = notify.DefaultRule(u.ID, u.Email)
	}
	if current.Email == "" {
		current.Email = u.Email
	}
	if input.PauseAll != nil {
		current.PauseAll = *input.PauseAll
	}
	applyChannelPref(&current.OnRunComplete, input.OnRunComplete)
	applyChannelPref(&current.OnGateFailed, input.OnGateFailed)
	applyChannelPref(&current.OnDeployDone, input.OnDeployDone)
	applyChannelPref(&current.OnBudgetWarning, input.OnBudgetWarning)
	applyChannelPref(&current.OnReceipt, input.OnReceipt)
	if err := r.NotifyPrefs.Set(ctx, current); err != nil {
		return nil, err
	}
	return notificationPrefsToGraphQL(current), nil
}

// Notifications lists the caller's notifications newest-first.
func (r *queryResolver) Notifications(ctx context.Context, unreadOnly *bool) ([]model.Notification, error) {
	if r.NotifyStore == nil {
		return nil, gqlNotConfigured("notifications")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	unread := false
	if unreadOnly != nil {
		unread = *unreadOnly
	}
	rows, err := r.NotifyStore.List(ctx, u.ID, unread, 100)
	if err != nil {
		return nil, err
	}
	out := make([]model.Notification, 0, len(rows))
	for i := range rows {
		out = append(out, *notificationToGraphQL(rows[i]))
	}
	return out, nil
}

// UnreadNotificationCount returns the count of the caller's unread rows.
func (r *queryResolver) UnreadNotificationCount(ctx context.Context) (int, error) {
	if r.NotifyStore == nil {
		return 0, gqlNotConfigured("notifications")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return 0, err
	}
	return r.NotifyStore.UnreadCount(ctx, u.ID)
}

// NotificationPreferences returns the caller's per-topic channel prefs.
func (r *queryResolver) NotificationPreferences(ctx context.Context) (*model.NotificationPreferences, error) {
	if r.NotifyPrefs == nil {
		return nil, gqlNotConfigured("notification preferences")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	rule, err := r.NotifyPrefs.Get(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if rule.UserID == "" {
		rule = notify.DefaultRule(u.ID, u.Email)
	}
	return notificationPrefsToGraphQL(rule), nil
}

// NotificationStream subscribes to the authenticated user's in-app
// notification fan-out. The hub is in-process pub/sub keyed by userID;
// each connection gets a buffered channel and a deferred unsubscribe so
// disconnects (ctx.Done) release the slot promptly.
func (r *subscriptionResolver) NotificationStream(ctx context.Context) (<-chan *model.Notification, error) {
	if r.NotifyHub == nil {
		return nil, gqlNotConfigured("notification stream")
	}
	u, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	upstream, unsub := r.NotifyHub.Subscribe(u.ID)
	out := make(chan *model.Notification, 16)
	go func() {
		defer close(out)
		defer unsub()
		for {
			select {
			case <-ctx.Done():
				return
			case n, ok := <-upstream:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- notificationToGraphQL(n):
				}
			}
		}
	}()
	return out, nil
}

func notificationToGraphQL(n notify.Notification) *model.Notification {
	var link *string
	if n.Link != "" {
		l := n.Link
		link = &l
	}
	return &model.Notification{
		ID:        n.ID,
		Kind:      n.Kind,
		Title:     n.Title,
		Body:      n.Body,
		Link:      link,
		Severity:  n.Severity,
		ReadAt:    n.ReadAt,
		CreatedAt: n.CreatedAt,
	}
}

func notificationPrefsToGraphQL(r notify.NotificationRule) *model.NotificationPreferences {
	return &model.NotificationPreferences{
		UserID:          r.UserID,
		PauseAll:        r.PauseAll,
		OnRunComplete:   model.ChannelPref{Email: r.OnRunComplete.Email, InApp: r.OnRunComplete.InApp},
		OnGateFailed:    model.ChannelPref{Email: r.OnGateFailed.Email, InApp: r.OnGateFailed.InApp},
		OnDeployDone:    model.ChannelPref{Email: r.OnDeployDone.Email, InApp: r.OnDeployDone.InApp},
		OnBudgetWarning: model.ChannelPref{Email: r.OnBudgetWarning.Email, InApp: r.OnBudgetWarning.InApp},
		OnReceipt:       model.ChannelPref{Email: r.OnReceipt.Email, InApp: r.OnReceipt.InApp},
	}
}

func applyChannelPref(dst *notify.ChannelPref, src *model.ChannelPrefInput) {
	if src == nil {
		return
	}
	dst.Email = src.Email
	dst.InApp = src.InApp
}
