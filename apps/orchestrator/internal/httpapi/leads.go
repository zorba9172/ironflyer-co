package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"ironflyer/apps/orchestrator/internal/leads"
)

type enterpriseLeadRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Company  string `json:"company"`
	TeamSize string `json:"teamSize"`
	UseCase  string `json:"useCase"`
	Budget   string `json:"budget"`
	Timeline string `json:"timeline"`
	Source   string `json:"source"`
}

func (a *API) enterpriseLead(w http.ResponseWriter, r *http.Request) {
	var body enterpriseLeadRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Email = strings.TrimSpace(body.Email)
	body.Company = strings.TrimSpace(body.Company)
	body.TeamSize = strings.TrimSpace(body.TeamSize)
	body.UseCase = strings.TrimSpace(body.UseCase)
	body.Budget = strings.TrimSpace(body.Budget)
	body.Timeline = strings.TrimSpace(body.Timeline)
	body.Source = strings.TrimSpace(body.Source)
	if body.Email == "" || !strings.Contains(body.Email, "@") {
		writeJSON(w, http.StatusBadRequest, errJSON("valid email required"))
		return
	}
	if body.Company == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("company required"))
		return
	}
	if a.d.Leads == nil {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("lead capture not configured"))
		return
	}

	lead := leads.Lead{
		Name: body.Name, Email: body.Email, Company: body.Company,
		TeamSize: body.TeamSize, UseCase: body.UseCase,
		Budget: body.Budget, Timeline: body.Timeline, Source: body.Source,
		UserAgent: r.Header.Get("User-Agent"),
		IP:        clientIP(r),
	}
	saved, err := a.d.Leads.Create(r.Context(), lead)
	if err != nil {
		a.d.Logger.Error().Err(err).Str("email", body.Email).Msg("persist enterprise lead failed")
		writeJSON(w, http.StatusInternalServerError, errJSON("failed to persist lead"))
		return
	}
	a.d.Logger.Info().
		Str("lead_id", saved.ID).Str("email", saved.Email).Str("company", saved.Company).
		Str("team_size", saved.TeamSize).Str("budget", saved.Budget).
		Str("timeline", saved.Timeline).Str("source", saved.Source).
		Msg("enterprise lead captured")

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        saved.ID,
		"status":    "received",
		"createdAt": saved.CreatedAt,
	})
}

// clientIP picks the real caller IP from X-Forwarded-For when behind an
// ingress / load balancer; falls back to RemoteAddr. Used for lead audit
// only — never for authorisation.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}
