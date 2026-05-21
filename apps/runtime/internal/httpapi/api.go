// Package httpapi exposes the workspace runtime over chi + WebSocket.
package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"ironflyer/apps/runtime/internal/auth"
	"ironflyer/apps/runtime/internal/sandbox"
)

type API struct {
	mgr      *sandbox.Manager
	logger   zerolog.Logger
	cors     string
	verifier *auth.Verifier // nil = no-auth dev mode
}

// New builds the runtime API. Pass a non-nil verifier to require JWT auth.
func New(mgr *sandbox.Manager, corsOrigin string, verifier *auth.Verifier, logger zerolog.Logger) http.Handler {
	a := &API{mgr: mgr, logger: logger, cors: corsOrigin, verifier: verifier}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(a.corsMW)
	r.Use(a.logMW)
	r.Use(middleware.Recoverer)

	r.Get("/health", a.health)

	// Everything else requires a verified user (or no-auth if verifier nil).
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(a.verifier))
		r.Route("/workspaces", func(r chi.Router) {
			r.Get("/", a.list)
			r.Post("/", a.create)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", a.get)
				r.Delete("/", a.destroy)
				r.Get("/files", a.listFiles)
				r.Get("/files/*", a.readFile)
				r.Put("/files/*", a.writeFile)
				r.Delete("/files/*", a.deleteFile)
				r.Get("/terminal", a.terminal)
				r.Post("/git-clone", a.gitClone)
				r.Post("/exec", a.exec)
			})
		})
	})
	return r
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"service":    "ironflyer-runtime",
		"driver":     a.mgr.Driver().Name(),
		"authMode":   a.authMode(),
	})
}

func (a *API) authMode() string {
	if a.verifier == nil {
		return "disabled"
	}
	return "jwt"
}

// requireWorkspace loads a workspace and 404s when the caller doesn't own it.
func (a *API) requireWorkspace(w http.ResponseWriter, r *http.Request, id string) (sandbox.Workspace, bool) {
	ws, err := a.mgr.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return sandbox.Workspace{}, false
	}
	uid := userIDFromCtx(r)
	if !ws.IsAccessibleBy(uid) {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return sandbox.Workspace{}, false
	}
	return ws, true
}

func userIDFromCtx(r *http.Request) string {
	if u, ok := auth.FromContext(r.Context()); ok {
		return u.ID
	}
	return ""
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromCtx(r)
	all := a.mgr.List()
	out := make([]sandbox.Workspace, 0, len(all))
	for _, ws := range all {
		if ws.IsAccessibleBy(uid) {
			out = append(out, ws)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID    string `json:"userId"` // ignored when JWT auth is on
		ProjectID string `json:"projectId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if uid := userIDFromCtx(r); uid != "" {
		body.UserID = uid
	} else if body.UserID == "" {
		body.UserID = "demo"
	}
	ws, err := a.mgr.Create(r.Context(), sandbox.CreateOpts{UserID: body.UserID, ProjectID: body.ProjectID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (a *API) destroy(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id")); !ok {
		return
	}
	if err := a.mgr.Destroy(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusNotFound, errJSON(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listFiles(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	files, err := a.mgr.Driver().ListFiles(r.Context(), ws)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, files)
}

func (a *API) readFile(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	p := chi.URLParam(r, "*")
	data, err := a.mgr.Driver().ReadFile(r.Context(), ws, p)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errJSON(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(data)
}

func (a *API) writeFile(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	p := chi.URLParam(r, "*")
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("body too large"))
		return
	}
	if err := a.mgr.Driver().WriteFile(r.Context(), ws, p, data); err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": p, "size": len(data)})
}

func (a *API) deleteFile(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	p := chi.URLParam(r, "*")
	if err := a.mgr.Driver().DeleteFile(r.Context(), ws, p); err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// gitClone shallow-clones a repository into an owned workspace. The token,
// if any, is consumed once and never logged.
func (a *API) gitClone(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body struct {
		CloneURL string `json:"cloneUrl"`
		Token    string `json:"token"`
		Ref      string `json:"ref"`
		Subdir   string `json:"subdir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.CloneURL) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("cloneUrl required"))
		return
	}
	if err := a.mgr.Driver().GitClone(r.Context(), ws, sandbox.CloneOpts{
		CloneURL: body.CloneURL, Token: body.Token,
		Ref: body.Ref, Subdir: body.Subdir,
	}); err != nil {
		a.logger.Warn().Err(err).Str("ws", ws.ID).Msg("git clone failed")
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cloned"})
}

// exec runs a one-shot command inside the workspace and returns the captured
// stdout/stderr/exit code. The orchestrator's finisher uses this to drive
// real build/test/lint gates instead of in-memory file inspection.
func (a *API) exec(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body sandbox.ExecOpts
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.Shell) == "" && len(body.Cmd) == 0 {
		writeJSON(w, http.StatusBadRequest, errJSON("shell or cmd required"))
		return
	}
	res, err := a.mgr.Driver().Exec(r.Context(), ws, body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// terminal is the PTY WebSocket bridge. Binary frames are passthrough I/O;
// text frames are control messages (resize, etc.) as JSON. The owner check
// runs before the WS upgrade so we don't 404 a connected socket.
func (a *API) terminal(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		a.logger.Warn().Err(err).Msg("ws accept")
		return
	}
	defer c.Close(websocket.StatusInternalError, "closing")

	session, err := a.mgr.Driver().Terminal(r.Context(), ws)
	if err != nil {
		_ = c.Write(r.Context(), websocket.MessageText,
			[]byte(`{"type":"error","msg":"`+err.Error()+`"}`))
		return
	}
	defer session.Close()

	ctx := r.Context()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := session.Read(buf)
			if n > 0 {
				if werr := c.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		mt, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		if mt == websocket.MessageText {
			var ctrl struct {
				Type string `json:"type"`
				Rows uint16 `json:"rows"`
				Cols uint16 `json:"cols"`
			}
			if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "resize" {
				_ = session.Resize(ctrl.Rows, ctrl.Cols)
			}
			continue
		}
		if _, err := session.Write(data); err != nil {
			return
		}
	}
}

func (a *API) corsMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", a.cors)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if !strings.HasSuffix(r.URL.Path, "/terminal") {
			a.logger.Info().
				Str("method", r.Method).Str("path", r.URL.Path).
				Int("status", ww.Status()).Dur("dur", time.Since(start)).Msg("http")
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(msg string) map[string]string { return map[string]string{"error": msg} }
