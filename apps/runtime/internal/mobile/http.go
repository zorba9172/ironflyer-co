package mobile

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"ironflyer/apps/runtime/internal/auth"
	"ironflyer/apps/runtime/internal/sandbox"
)

// RegisterRoutes wires the mobile endpoints onto r. The caller is
// responsible for mounting r inside whatever auth-protected subtree the
// runtime exposes; authMiddleware is applied here as well so the mobile
// routes remain protected even when mounted on a top-level router.
//
// Routes are scoped to /v1/workspaces/{id}/mobile/... so they nest under
// the same prefix the other runtime endpoints use.
func RegisterRoutes(r chi.Router, m *Manager, authMiddleware func(http.Handler) http.Handler) {
	h := &handler{m: m}
	mount := func(sub chi.Router) {
		sub.Post("/v1/workspaces/{id}/mobile/build", h.startBuild)
		sub.Get("/v1/workspaces/{id}/mobile/build/{buildId}", h.getBuild)
		sub.Post("/v1/workspaces/{id}/mobile/expo/start", h.expoStart)
		sub.Post("/v1/workspaces/{id}/mobile/expo/stop", h.expoStop)
		sub.Post("/v1/workspaces/{id}/mobile/android-emulator/start", h.emuStart)
		sub.Post("/v1/workspaces/{id}/mobile/android-emulator/stop", h.emuStop)
		// Metro hot-reload lifecycle + dynamic proxy. The /proxy/* mount
		// switches between HTTP reverse-proxy and a raw-TCP WebSocket
		// hijack based on the Upgrade header.
		sub.Post("/v1/workspaces/{id}/mobile/metro/start", h.metroStart)
		sub.Post("/v1/workspaces/{id}/mobile/metro/stop", h.metroStop)
		sub.Get("/v1/workspaces/{id}/mobile/metro", h.metroGet)
		sub.HandleFunc("/v1/workspaces/{id}/mobile/metro/proxy/*", h.metroProxy)
	}
	if authMiddleware == nil {
		mount(r)
		return
	}
	r.Group(func(sub chi.Router) {
		sub.Use(authMiddleware)
		mount(sub)
	})
}

type handler struct {
	m *Manager
}

// requireWorkspace loads the workspace and 404s when the caller doesn't
// own it — same pattern as the existing httpapi routes.
func (h *handler) requireWorkspace(w http.ResponseWriter, r *http.Request) (sandbox.Workspace, bool) {
	id := chi.URLParam(r, "id")
	ws, err := h.m.sandbox.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return sandbox.Workspace{}, false
	}
	uid := userIDFromCtx(r.Context())
	if !ws.IsAccessibleBy(uid) {
		writeErr(w, http.StatusNotFound, "not found")
		return sandbox.Workspace{}, false
	}
	return ws, true
}

func (h *handler) startBuild(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	var body BuildRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.WorkspaceID = ws.ID
	if body.Kind == "" || body.Target == "" {
		writeErr(w, http.StatusBadRequest, "kind and target required")
		return
	}
	buildID := h.m.StartBuild(ws, body)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"buildId":     buildID,
		"workspaceId": ws.ID,
		"status":      BuildStatusRunning,
	})
}

func (h *handler) getBuild(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	buildID := chi.URLParam(r, "buildId")
	if !h.m.BuildBelongsTo(ws.ID, buildID) {
		writeErr(w, http.StatusNotFound, "build not found")
		return
	}
	rec, ok := h.m.LookupBuild(buildID)
	if !ok {
		writeErr(w, http.StatusNotFound, "build not found")
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (h *handler) expoStart(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	session, err := h.m.StartExpo(r.Context(), ws)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *handler) expoStop(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	if err := h.m.StopExpo(r.Context(), ws.ID); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) emuStart(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	var body struct {
		AVD string `json:"avd"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.AVD) == "" {
		writeErr(w, http.StatusBadRequest, "avd required")
		return
	}
	session, err := h.m.StartAndroidEmulator(r.Context(), ws, body.AVD)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *handler) emuStop(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	if err := h.m.StopAndroidEmulator(r.Context(), ws.ID); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) metroStart(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	session, err := h.m.MetroProxy().Start(r.Context(), ws)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *handler) metroStop(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	if err := h.m.MetroProxy().Stop(r.Context(), ws.ID); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) metroGet(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	session, ok := h.m.MetroProxy().Get(ws.ID)
	if !ok {
		writeErr(w, http.StatusNotFound, "metro not running")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// metroProxy dispatches each incoming request to either the HTTP reverse
// proxy or the raw-TCP WebSocket hijack based on the Upgrade header. The
// proxy mount path is stripped before forwarding so Metro sees the
// canonical /message, /status, /symbolicate, etc. paths it expects.
func (h *handler) metroProxy(w http.ResponseWriter, r *http.Request) {
	ws, ok := h.requireWorkspace(w, r)
	if !ok {
		return
	}
	session, ok := h.m.MetroProxy().Get(ws.ID)
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "metro not running")
		return
	}
	mountPrefix := "/v1/workspaces/" + ws.ID + "/mobile/metro/proxy"
	target := session.LANBaseURL
	if strings.TrimSpace(target) == "" {
		writeErr(w, http.StatusBadGateway, "metro target unknown")
		return
	}
	if isWebSocketUpgrade(r) {
		// Strip scheme to host:port for the raw dialer.
		hp := target
		hp = strings.TrimPrefix(hp, "http://")
		hp = strings.TrimPrefix(hp, "https://")
		if idx := strings.IndexByte(hp, '/'); idx >= 0 {
			hp = hp[:idx]
		}
		ProxyMetroWS(hp, mountPrefix).ServeHTTP(w, r)
		return
	}
	handler, err := ProxyMetroHTTP(target, mountPrefix)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "metro proxy: "+err.Error())
		return
	}
	handler.ServeHTTP(w, r)
}

func userIDFromCtx(ctx context.Context) string {
	if u, ok := auth.FromContext(ctx); ok {
		return u.ID
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
