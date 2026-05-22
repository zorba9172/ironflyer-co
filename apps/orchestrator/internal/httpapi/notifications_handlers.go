package httpapi

import (
	"encoding/json"
	"net/http"

	"ironflyer/apps/orchestrator/internal/auth"
	"ironflyer/apps/orchestrator/internal/notify"
)

// getNotificationPrefs returns the caller's NotificationRule. First-time
// callers get a synthesised DefaultRule (no rows persisted yet) so the UI
// can render checkboxes without dealing with null.
func (a *API) getNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	if a.d.NotifyPrefs == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("notifications not configured"))
		return
	}
	uid := userIDFromCtx(r)
	rule, err := a.d.NotifyPrefs.Get(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	// Default the email field to the user's identity so the UI does not need
	// to fall back on its own.
	if rule.Email == "" {
		if u, ok := auth.FromContext(r.Context()); ok {
			rule.Email = u.Email
		}
	}
	rule.UserID = uid
	writeJSON(w, http.StatusOK, rule)
}

// setNotificationPrefs upserts the caller's NotificationRule. We force
// UserID server-side so a client can never overwrite someone else's rule.
func (a *API) setNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	if a.d.NotifyPrefs == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("notifications not configured"))
		return
	}
	var body notify.NotificationRule
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	body.UserID = userIDFromCtx(r)
	if err := a.d.NotifyPrefs.Set(r.Context(), body); err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, body)
}
