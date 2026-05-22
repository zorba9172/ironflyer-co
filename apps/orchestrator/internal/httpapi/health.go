package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Version is the human-friendly orchestrator build label surfaced by
// /healthz and /readyz. Bump when shipping a notable release.
const Version = "v14-finisher-polish"

// startedAt is captured at package init so the health endpoints can
// report uptime without threading state through Deps.
var startedAt = time.Now().UTC()

// RegisterHealth wires /healthz and /readyz on the given chi router using
// the API's Deps. Called from api.go via one appended line.
func (a *API) RegisterHealth(r chi.Router) {
	r.Get("/healthz", a.healthz)
	r.Get("/readyz", a.readyz)
}

// healthSnapshot probes the orchestrator's external dependencies and
// returns a JSON-ready map. Probes are intentionally cheap and non-blocking
// so the endpoint stays useful during partial outages.
func (a *API) healthSnapshot(ctx context.Context) map[string]any {
	services := map[string]bool{
		"postgres":  a.probePostgres(ctx),
		"anthropic": a.probeAnthropic(),
		"runtime":   a.probeRuntime(ctx),
		"stripe":    a.probeStripe(),
	}
	status := "ok"
	for _, ok := range services {
		if !ok {
			status = "degraded"
			break
		}
	}
	return map[string]any{
		"status":   status,
		"services": services,
		"version":  Version,
		"uptime":   time.Since(startedAt).Round(time.Second).String(),
	}
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.healthSnapshot(r.Context()))
}

// readyz returns 503 when any critical dependency is down. We treat
// postgres + anthropic as critical (the rest can degrade gracefully).
func (a *API) readyz(w http.ResponseWriter, r *http.Request) {
	snap := a.healthSnapshot(r.Context())
	services, _ := snap["services"].(map[string]bool)
	code := http.StatusOK
	if services != nil && (!services["postgres"] || !services["anthropic"]) {
		code = http.StatusServiceUnavailable
		snap["status"] = "degraded"
	}
	writeJSON(w, code, snap)
}

// probePostgres returns true when a Projects store is wired. The Memory
// driver is also a healthy state (dev mode); only an entirely-missing
// store is considered unhealthy.
func (a *API) probePostgres(_ context.Context) bool {
	return a.d.Projects != nil
}

// probeAnthropic reports the BillingGuard's readiness — the guard is the
// gateway every paid Anthropic call flows through.
func (a *API) probeAnthropic() bool {
	return a.d.Guard != nil
}

// probeRuntime pings the workspace runtime's /healthz when configured.
// A 2xx counts as healthy; anything else (incl. unreachable) is reported
// as degraded without failing the orchestrator itself.
func (a *API) probeRuntime(ctx context.Context) bool {
	if a.d.RuntimeURL == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.d.RuntimeURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// probeStripe reports whether the StripeService is wired. We avoid
// reaching out to Stripe on every health hit — their API has rate limits
// and we don't want a health probe to itself burn budget.
func (a *API) probeStripe() bool {
	return a.d.Stripe != nil
}
