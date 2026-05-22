package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"ironflyer/apps/orchestrator/internal/webhooks"
)

// listWebhooks returns every subscription owned by the caller. We never
// surface other users' webhooks even by ID — Store.List is user-scoped.
func (a *API) listWebhooks(w http.ResponseWriter, r *http.Request) {
	if a.d.Webhooks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("webhooks not configured"))
		return
	}
	uid := userIDFromCtx(r)
	out, err := a.d.Webhooks.List(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// createWebhook persists a new subscription for the caller.
// Body: { url, events?[], projectId?, secret? }
func (a *API) createWebhook(w http.ResponseWriter, r *http.Request) {
	if a.d.Webhooks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("webhooks not configured"))
		return
	}
	var body struct {
		URL       string   `json:"url"`
		Events    []string `json:"events"`
		ProjectID string   `json:"projectId"`
		Secret    string   `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	url := strings.TrimSpace(body.URL)
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		writeJSON(w, http.StatusBadRequest, errJSON("url must be http(s)"))
		return
	}
	// When a projectId is supplied, make sure the caller can actually see it.
	if body.ProjectID != "" {
		if _, ok := a.requireProjectAccess(w, r, body.ProjectID); !ok {
			return
		}
	}
	sub := webhooks.Subscription{
		UserID:    userIDFromCtx(r),
		URL:       url,
		Events:    body.Events,
		ProjectID: body.ProjectID,
		Secret:    body.Secret,
	}
	out, err := a.d.Webhooks.Create(r.Context(), sub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// deleteWebhook removes a subscription. Store enforces owner-only delete.
func (a *API) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	if a.d.Webhooks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("webhooks not configured"))
		return
	}
	id := chi.URLParam(r, "id")
	if err := a.d.Webhooks.Delete(r.Context(), userIDFromCtx(r), id); err != nil {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testWebhook fires a synthetic webhook_test event so the user can verify
// their endpoint actually receives + verifies signed payloads.
func (a *API) testWebhook(w http.ResponseWriter, r *http.Request) {
	if a.d.Webhooks == nil || a.d.WebhookDispatcher == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("webhooks not configured"))
		return
	}
	id := chi.URLParam(r, "id")
	sub, err := a.d.Webhooks.Get(r.Context(), id)
	if err != nil || sub.UserID != userIDFromCtx(r) {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return
	}
	a.d.WebhookDispatcher.DeliverSynthetic(r.Context(), sub, sub.ProjectID)
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "id": sub.ID})
}
