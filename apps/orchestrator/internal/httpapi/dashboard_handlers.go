// Dashboard handlers — per-user vault snapshot, bulk project deletion,
// account self-deletion. Surfaced under the existing auth-protected group.

package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ironflyer/apps/orchestrator/internal/auth"
)

// RegisterDashboard wires the three dashboard-only endpoints into r. Call
// from inside the authenticated group so all routes require a valid user.
func (a *API) RegisterDashboard(r chi.Router) {
	r.Get("/budget/vault/me", a.vaultMine)
	r.Post("/projects/bulk-delete", a.bulkDeleteProjects)
	r.Delete("/projects/{id}", a.deleteProject)
	r.Post("/account/delete", a.deleteAccount)
}

// vaultMine returns the authenticated user's vault snapshot: lifetime spend,
// last-30 ledger entries, current plan cap, and the global vault snapshot
// for context (so the UI can render "you used X of Y this month").
func (a *API) vaultMine(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errJSON("unauthenticated"))
		return
	}
	spent, _ := a.d.Billing.Ledger.SpentByUser(r.Context(), u.ID)
	entries, _ := a.d.Billing.Ledger.EntriesByUser(r.Context(), u.ID)
	if len(entries) > 30 {
		entries = entries[len(entries)-30:]
	}
	global, _ := a.d.Billing.Vault.Snapshot(r.Context())
	// Find the user's plan in the catalogue (linear scan is fine — handful of plans).
	var plan any
	for _, p := range a.d.Billing.Plans {
		if string(p.Tier) == u.Plan {
			plan = p
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"userId":   u.ID,
		"email":    u.Email,
		"tier":     u.Plan,
		"plan":     plan,
		"spent":    spent,
		"entries":  entries,
		"snapshot": global,
	})
}

// bulkDeleteProjects deletes the caller's projects. Body is optional —
// when an `ids` array is present, only those owned by the caller are
// deleted (CLI-style targeted delete). When body is empty/omitted,
// every project the caller owns is wiped (dashboard "delete everything").
// Public projects they don't own are always left alone.
func (a *API) bulkDeleteProjects(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromCtx(r)
	if uid == "" {
		writeJSON(w, http.StatusUnauthorized, errJSON("unauthenticated"))
		return
	}
	var body struct {
		IDs []string `json:"ids"`
	}
	// JSON body is optional; ignore decode errors so an empty POST works.
	_ = json.NewDecoder(r.Body).Decode(&body)
	filter := map[string]struct{}{}
	for _, id := range body.IDs {
		filter[id] = struct{}{}
	}
	useFilter := len(filter) > 0

	all := a.d.Projects.List()
	deleted := make([]string, 0, 8)
	for _, p := range all {
		if p.OwnerID != uid {
			continue
		}
		if useFilter {
			if _, ok := filter[p.ID]; !ok {
				continue
			}
		}
		if err := a.d.Projects.Delete(p.ID); err == nil {
			deleted = append(deleted, p.ID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
		"count":   len(deleted),
	})
}

// deleteProject removes a single project the caller owns. RESTful counterpart
// to bulkDeleteProjects; the CLI prefers this when targeting one ID.
func (a *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromCtx(r)
	id := chi.URLParam(r, "id")
	if uid == "" {
		writeJSON(w, http.StatusUnauthorized, errJSON("unauthenticated"))
		return
	}
	p, err := a.d.Projects.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errJSON("project not found"))
		return
	}
	if p.OwnerID != uid {
		writeJSON(w, http.StatusNotFound, errJSON("project not found"))
		return
	}
	if err := a.d.Projects.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteAccount irreversibly removes the user's projects and user record.
// Issued tokens remain valid until expiry — clients should clear them
// immediately. Returns 204 on success.
func (a *API) deleteAccount(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errJSON("unauthenticated"))
		return
	}
	// Bulk-delete owned projects first so we don't leave orphans.
	for _, p := range a.d.Projects.List() {
		if p.OwnerID == u.ID {
			_ = a.d.Projects.Delete(p.ID)
		}
	}
	if a.d.Auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("auth not configured"))
		return
	}
	if err := a.d.Auth.Delete(r.Context(), u.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
