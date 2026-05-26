package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Version is the runtime build label surfaced by /healthz + /readyz.
const Version = "v3-runtime-polish"

// startedAt captures process boot for the uptime field. Package-scoped
// so the health endpoints stay pure and side-effect free.
var startedAt = time.Now().UTC()

// RegisterHealth wires /healthz and /readyz on the supplied router. The
// chi router in New() appends a single call to this from api.go.
func (a *API) RegisterHealth(r chi.Router) {
	r.Get("/healthz", a.healthz)
	r.Get("/readyz", a.readyz)
}

// healthSnapshot describes which runtime sub-systems are wired and which
// driver is active. Identifying the driver matters operationally — we
// run the Mock driver in CI/dev and Docker in prod, and a mismatched
// driver in prod is one of our nastier classes of bug.
func (a *API) healthSnapshot() map[string]any {
	driver := ""
	if a.mgr != nil {
		driver = a.mgr.Driver().Name()
	}
	previewOK := a.preview != nil
	status := "ok"
	if driver == "" {
		status = "degraded"
	}
	return map[string]any{
		"status": status,
		"services": map[string]any{
			"driver":  driver,
			"preview": previewOK,
		},
		"version": Version,
		"uptime":  time.Since(startedAt).Round(time.Second).String(),
	}
}

func (a *API) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.healthSnapshot())
}

// readyz returns 503 when the sandbox manager isn't wired — without it,
// the runtime can't satisfy any workspace request.
func (a *API) readyz(w http.ResponseWriter, _ *http.Request) {
	snap := a.healthSnapshot()
	if a.mgr == nil {
		writeJSON(w, http.StatusServiceUnavailable, snap)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
