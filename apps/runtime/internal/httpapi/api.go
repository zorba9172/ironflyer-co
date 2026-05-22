// Package httpapi exposes the workspace runtime over chi + WebSocket.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"ironflyer/apps/runtime/internal/auth"
	"ironflyer/apps/runtime/internal/patcher"
	"ironflyer/apps/runtime/internal/preview"
	"ironflyer/apps/runtime/internal/sandbox"
)

// Options bundles the non-trivial knobs httpapi.New needs. Keeping them in
// a struct lets us add more (preview tuning, max workspaces) without
// breaking the constructor signature every time.
type Options struct {
	CORSOrigin      string
	Verifier        *auth.Verifier // nil = no-auth dev mode
	PreviewPrefix   string
	AllowedPorts    string
	PreviewSecret   []byte
	PreviewTokenTTL time.Duration
	MaxWorkspaces   int
}

type API struct {
	mgr      *sandbox.Manager
	logger   zerolog.Logger
	cors     string
	verifier *auth.Verifier
	preview  *preview.Proxy
	signer   *preview.TokenSigner
	maxWS    int
}

// New builds the runtime API.
func New(mgr *sandbox.Manager, opts Options, logger zerolog.Logger) http.Handler {
	signer := preview.NewSigner(opts.PreviewSecret, opts.PreviewTokenTTL)
	a := &API{
		mgr:      mgr,
		logger:   logger,
		cors:     opts.CORSOrigin,
		verifier: opts.Verifier,
		signer:   signer,
		maxWS:    opts.MaxWorkspaces,
	}
	a.preview = preview.New(
		preview.Config{
			Prefix:       opts.PreviewPrefix,
			AllowedPorts: opts.AllowedPorts,
		},
		&driverTargetResolver{mgr: mgr},
		&apiAuthorizer{a: a},
		signer,
		zlogAdapter{l: logger},
	)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(a.corsMW)
	r.Use(a.logMW)
	r.Use(middleware.Recoverer)
	r.Use(requestIDMiddleware)
	r.Use(accessLogMiddleware(logger, opts.PreviewPrefix))

	r.Get("/health", a.health)
	a.RegisterHealth(r)

	// Preview proxy lives OUTSIDE the chi auth group: it uses signed
	// `?t=...` tokens because iframes can't send Authorization headers.
	// The proxy itself enforces auth via preview.Authorizer.
	r.HandleFunc(a.preview.Prefix+"/*", a.preview.ServeHTTP)

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
				r.Get("/ports", a.listPorts)
				r.Post("/ports", a.recordPort)
				r.Post("/preview-token", a.previewToken)
				r.Post("/apply-patch", a.applyPatch)
			})
		})
	})
	return r
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"service":       "ironflyer-runtime",
		"driver":        a.mgr.Driver().Name(),
		"authMode":      a.authMode(),
		"previewPrefix": a.preview.Prefix,
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
	if a.maxWS > 0 && len(a.mgr.List()) >= a.maxWS {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("max workspaces reached"))
		return
	}
	var body struct {
		UserID    string `json:"userId"`
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

// gitClone shallow-clones a repository into an owned workspace.
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

// exec runs a one-shot command inside the workspace. We also scan the
// captured stdout/stderr for "Listening on :3000" / "Local: http://...:5173"
// style breadcrumbs and register any ports we spot. That makes a developer's
// `npm run dev` immediately discoverable via /workspaces/{id}/ports.
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
	for _, port := range scanPorts(res.Stdout) {
		a.mgr.RecordPort(ws.ID, port, "exec-stdout")
	}
	for _, port := range scanPorts(res.Stderr) {
		a.mgr.RecordPort(ws.ID, port, "exec-stderr")
	}
	writeJSON(w, http.StatusOK, res)
}

// terminal is the PTY WebSocket bridge.
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

// listPorts returns the ports we've auto-detected for a workspace plus a
// suggested public preview URL for each.
func (a *API) listPorts(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	ports := a.mgr.Ports(ws.ID)
	type item struct {
		sandbox.DetectedPort
		PreviewPath string `json:"previewPath"`
		Allowed     bool   `json:"allowed"`
	}
	out := make([]item, 0, len(ports))
	for _, p := range ports {
		out = append(out, item{
			DetectedPort: p,
			PreviewPath:  a.preview.BuildPreviewPath(ws.ID, p.Port, ""),
			Allowed:      a.preview.PortAllowed(p.Port),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// recordPort lets the orchestrator or VSCode extension explicitly
// register a port the runtime didn't auto-detect (e.g. when a long-lived
// process was started via a terminal session rather than /exec).
func (a *API) recordPort(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body struct {
		Port   int    `json:"port"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if body.Port <= 0 || body.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, errJSON("port must be 1..65535"))
		return
	}
	if body.Source == "" {
		body.Source = "manual"
	}
	a.mgr.RecordPort(ws.ID, body.Port, body.Source)
	writeJSON(w, http.StatusOK, map[string]any{
		"port": body.Port, "previewPath": a.preview.BuildPreviewPath(ws.ID, body.Port, ""),
	})
}

// previewToken mints a signed `?t=...` for a workspace+port pair. The web
// app uses this to drop into an iframe `src` without setting headers.
func (a *API) previewToken(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body struct {
		Port int `json:"port"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if body.Port <= 0 || body.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, errJSON("port must be 1..65535"))
		return
	}
	if !a.preview.PortAllowed(body.Port) {
		writeJSON(w, http.StatusForbidden, errJSON("port not allowed"))
		return
	}
	tok, exp, err := a.signer.Mint(ws.ID, body.Port)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	path := a.preview.BuildPreviewPath(ws.ID, body.Port, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"url":       path + "?t=" + tok,
		"path":      path,
		"token":     tok,
		"expiresAt": exp.Format(time.RFC3339),
	})
}

// applyPatch applies a unified diff to the workspace's files. Returns the
// per-file outcome list. This is the RuntimeApplier contract Agent A's
// orchestrator calls after a patch passes the lifecycle gates.
func (a *API) applyPatch(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body struct {
		Diff string `json:"diff"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.Diff) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("diff required"))
		return
	}
	fs := &driverFS{ctx: r.Context(), drv: a.mgr.Driver(), ws: ws}
	changes, err := patcher.Apply(r.Context(), fs, body.Diff)
	if err != nil {
		// Partial success: return what we managed plus the error.
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":   err.Error(),
			"applied": changes,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"applied": changes,
		"count":   len(changes),
	})
}

// --------------------------------------------------------------------------
// Adapters: sandbox.Driver ↔ preview / patcher contracts.
// --------------------------------------------------------------------------

type driverTargetResolver struct{ mgr *sandbox.Manager }

func (d *driverTargetResolver) PreviewTarget(ctx context.Context, workspaceID string, port int) (string, error) {
	ws, err := d.mgr.Get(workspaceID)
	if err != nil {
		return "", err
	}
	return d.mgr.Driver().PreviewTarget(ctx, ws, port)
}

// apiAuthorizer enforces preview auth. Either:
//   - a valid signed `?t=...` token whose workspace+port match the URL, OR
//   - a valid JWT (Authorization or `?token=...`) whose user owns the workspace.
type apiAuthorizer struct{ a *API }

func (z *apiAuthorizer) AllowPreview(r *http.Request, workspaceID string) error {
	port := extractPathPort(r.URL.Path, z.a.preview.Prefix, workspaceID)

	if tok := r.URL.Query().Get("t"); tok != "" {
		if err := z.a.signer.Verify(tok, workspaceID, port); err == nil {
			return nil
		} else if z.a.verifier == nil {
			// In dev mode with auth disabled, accept the token even when
			// the signer was bootstrapped with a random secret restart.
			return err
		} else {
			return fmt.Errorf("token: %w", err)
		}
	}

	// JWT fallback — useful for curl / non-iframe API access.
	if z.a.verifier != nil {
		tok := bearerOrQueryToken(r)
		if tok == "" {
			return errors.New("preview token required (?t=...)")
		}
		user, err := z.a.verifier.Verify(tok)
		if err != nil {
			return fmt.Errorf("jwt: %w", err)
		}
		ws, err := z.a.mgr.Get(workspaceID)
		if err != nil {
			return errors.New("workspace not found")
		}
		if !ws.IsAccessibleBy(user.ID) {
			return errors.New("workspace not owned by caller")
		}
		return nil
	}

	// No-auth dev mode and no token: still require workspace to exist so
	// random URLs don't silently 200.
	if _, err := z.a.mgr.Get(workspaceID); err != nil {
		return errors.New("workspace not found")
	}
	return nil
}

func bearerOrQueryToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	return ""
}

func extractPathPort(path, prefix, workspaceID string) int {
	rest := strings.TrimPrefix(path, prefix+"/"+workspaceID+"/")
	cut := strings.IndexByte(rest, '/')
	var portStr string
	if cut < 0 {
		portStr = rest
	} else {
		portStr = rest[:cut]
	}
	p, _ := strconv.Atoi(portStr)
	return p
}

// driverFS adapts the sandbox driver to the patcher's Filesystem.
type driverFS struct {
	ctx context.Context
	drv sandbox.Driver
	ws  sandbox.Workspace
}

func (d *driverFS) ReadFile(_ context.Context, path string) ([]byte, error) {
	return d.drv.ReadFile(d.ctx, d.ws, path)
}
func (d *driverFS) WriteFile(_ context.Context, path string, data []byte) error {
	return d.drv.WriteFile(d.ctx, d.ws, path, data)
}
func (d *driverFS) DeleteFile(_ context.Context, path string) error {
	return d.drv.DeleteFile(d.ctx, d.ws, path)
}

// zlogAdapter bridges zerolog into the preview package's tiny Logger
// interface so we don't import zerolog there.
type zlogAdapter struct{ l zerolog.Logger }

func (z zlogAdapter) Warnf(format string, args ...any) { z.l.Warn().Msgf(format, args...) }
func (z zlogAdapter) Infof(format string, args ...any) { z.l.Info().Msgf(format, args...) }

// --------------------------------------------------------------------------
// Misc helpers.
// --------------------------------------------------------------------------

var portRegex = regexp.MustCompile(`(?:Local:\s+https?://[^:\s]+:|[Ll]istening on (?:https?://[^:\s]+)?:|port\s+|http://localhost:|http://127\.0\.0\.1:)(\d{2,5})\b`)

// scanPorts finds candidate dev-server ports in a captured-output blob.
// We err on the side of recall: every numeric capture between 1024 and
// 65535 is reported (callers gate by allowlist anyway).
func scanPorts(s string) []int {
	if s == "" {
		return nil
	}
	seen := make(map[int]bool)
	var out []int
	for _, m := range portRegex.FindAllStringSubmatch(s, -1) {
		if len(m) < 2 {
			continue
		}
		p, err := strconv.Atoi(m[1])
		if err != nil || p < 1024 || p > 65535 {
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
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
		if !strings.HasSuffix(r.URL.Path, "/terminal") &&
			!strings.HasPrefix(r.URL.Path, a.preview.Prefix+"/") {
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
