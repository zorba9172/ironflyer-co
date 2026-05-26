package resolver

// Helpers consumed by notifications.resolver.go. Live in a sibling
// file so gqlgen's "regenerate" pass does not strip them when the
// resolver file is rewritten.

import (
	"ironflyer/core/orchestrator/internal/customer/notify"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

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
		WeeklyDigest:    r.WeeklyDigest,
	}
}

func applyChannelPref(dst *notify.ChannelPref, src *model.ChannelPrefInput) {
	if src == nil {
		return
	}
	dst.Email = src.Email
	dst.InApp = src.InApp
}
