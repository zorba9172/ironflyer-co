// Package httpapi exposes the workspace runtime over chi + WebSocket.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"ironflyer/core/runtime/internal/customer/auth"
	"ironflyer/core/runtime/internal/operations/allocator"
	"ironflyer/core/runtime/internal/operations/patcher"
	"ironflyer/core/runtime/internal/operations/preview"
	"ironflyer/core/runtime/internal/operations/quota"
	"ironflyer/core/runtime/internal/operations/sandbox"
	"ironflyer/core/runtime/internal/pkg/httputil"
	"ironflyer/core/runtime/internal/suppliers/mobile"
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

	// Scale-ready dependencies; all may be nil for legacy single-pod mode.
	Lifecycle Lifecycle

	// Portability dependencies. When the State store is nil the runtime
	// degrades to legacy single-pod behaviour and the /admin/drain
	// endpoint short-circuits.
	Portability Portability

	// Allocator funnels every workspace create through the V22 Wave-2
	// admission flow (wallet hold → ProfitGuard → tenant quota → warm
	// slot / cold start → runtime-class selector). nil means dev /
	// legacy mode: the create handler skips admission and behaves like
	// the pre-allocator runtime did.
	Allocator allocator.Allocator
	// QuotaEnforcer backs GET /quota/usage. nil disables the endpoint.
	QuotaEnforcer quota.Enforcer
}

type API struct {
	mgr           *sandbox.Manager
	logger        zerolog.Logger
	cors          string
	verifier      *auth.Verifier
	preview       *preview.Proxy
	signer        *preview.TokenSigner
	maxWS         int
	lc            Lifecycle
	portability   Portability
	allocator     allocator.Allocator
	quotaEnforcer quota.Enforcer
	allocs        *allocTracker
	previews      *previewState
}

// New builds the runtime API.
func New(mgr *sandbox.Manager, opts Options, logger zerolog.Logger) http.Handler {
	signer := preview.NewSigner(opts.PreviewSecret, opts.PreviewTokenTTL)
	a := &API{
		mgr:           mgr,
		logger:        logger,
		cors:          opts.CORSOrigin,
		verifier:      opts.Verifier,
		signer:        signer,
		maxWS:         opts.MaxWorkspaces,
		lc:            opts.Lifecycle,
		allocator:     opts.Allocator,
		quotaEnforcer: opts.QuotaEnforcer,
		allocs:        newAllocTracker(),
		previews:      newPreviewState(),
	}
	a.SetPortability(opts.Portability)
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
	// sentryhttp is the outermost middleware: it scopes a fresh Hub to each
	// inbound request and converts panics into Sentry events. Repanic:true
	// so the runtime's own recoverer below still produces the 500 response.
	// When SENTRY_DSN was empty at boot, Handle is a thin pass-through.
	r.Use(sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(a.corsMW)
	r.Use(a.logMW)
	r.Use(sentryRecoverer(logger))
	r.Use(requestIDMiddleware)
	r.Use(accessLogMiddleware(logger, opts.PreviewPrefix))

	r.Get("/health", a.health)
	a.RegisterHealth(r)

	// Preview proxy lives OUTSIDE the chi auth group: it uses signed
	// `?t=...` tokens because iframes can't send Authorization headers.
	// The proxy itself enforces auth via preview.Authorizer.
	r.HandleFunc(a.preview.Prefix+"/*", a.preview.ServeHTTP)

	// Pod-internal admin routes (drain hook). Reachable only from inside
	// the pod network (kubelet preStop) — NetworkPolicy enforces the
	// network-level boundary, so no JWT here.
	a.registerPortabilityRoutes(r)

	// Everything else requires a verified user (or no-auth if verifier nil).
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(a.verifier))
		// The proxy middleware runs after auth so authenticated users can
		// be forwarded to whichever pod owns their workspace. When the
		// Registry is nil (single-pod mode) it short-circuits to next.
		r.Use(a.proxyToActivePod)
		// Live tenant counters for dashboards + the orchestrator's scale
		// loop. Sits behind JWT (when configured) but outside the
		// /workspaces tree because it isn't scoped to a single workspace.
		r.Get("/quota/usage", a.quotaUsage)
		r.Route("/workspaces", func(r chi.Router) {
			r.Get("/", a.list)
			r.Post("/", a.create)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", a.get)
				r.Delete("/", a.destroy)
				r.Get("/ide", a.ide)
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
				r.Post("/preview", a.allocatePreview)
				r.Get("/preview", a.getPreview)
				r.Delete("/preview", a.releasePreview)
				r.Post("/share-link", a.shareLink)
				r.Post("/screenshot", a.screenshot)
				r.Post("/apply-patch", a.applyPatch)
				// Scale-ready lifecycle endpoints.
				r.Post("/archive", a.archiveWorkspace)
				r.Post("/restore", a.restoreWorkspace)
				r.Get("/locator", a.locateWorkspace)
			})
		})
	})
	// Mobile lifecycle routes (Expo dev server, Android emulator, native
	// build dispatchers) live on the runtime alongside the existing
	// /workspaces tree. We register them on the top-level router and
	// pass auth.Middleware so the package can enforce per-user isolation
	// without having to know how the runtime's auth verifier is wired.
	mobileMgr := mobile.NewManager(mgr, logger)
	mobile.RegisterRoutes(r, mobileMgr, auth.Middleware(a.verifier))
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
// lookupWorkspace resolves a workspace by its driver-minted ID first, then by
// the project it belongs to (the studio addresses workspaces by project, not
// by the opaque sandbox ID). Returns false without writing a response when no
// accessible workspace exists, so callers can layer their own fallback.
func (a *API) lookupWorkspace(r *http.Request, id string) (sandbox.Workspace, bool) {
	ws, err := a.mgr.Get(id)
	if err != nil {
		pws, ok := a.mgr.GetByProject(id)
		if !ok {
			return sandbox.Workspace{}, false
		}
		ws = pws
	}
	if !ws.IsAccessibleBy(userIDFromCtx(r)) {
		return sandbox.Workspace{}, false
	}
	return ws, true
}

func (a *API) requireWorkspace(w http.ResponseWriter, r *http.Request, id string) (sandbox.Workspace, bool) {
	ws, ok := a.lookupWorkspace(r, id)
	if !ok {
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
		UserID               string `json:"userId"`
		ProjectID            string `json:"projectId"`
		TenantID             string `json:"tenantId"`
		ExecutionID          string `json:"executionId"`
		WorkspaceID          string `json:"workspaceId"`
		CPU                  int    `json:"cpu"`
		MemMB                int    `json:"memMB"`
		EstimatedDurationSec int    `json:"estimatedDurationSec"`
		RuntimeClass         string `json:"runtimeClass"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if uid := userIDFromCtx(r); uid != "" {
		body.UserID = uid
	} else if body.UserID == "" {
		body.UserID = "demo"
	}
	// Tenant + execution context flows via headers when the body did not
	// carry them — the orchestrator's runtime client sets the headers
	// uniformly across every endpoint it dials so callers don't have to
	// re-shape every JSON body.
	tenantID := strings.TrimSpace(body.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(r.Header.Get("X-Ironflyer-Tenant-ID"))
	}
	executionID := strings.TrimSpace(body.ExecutionID)
	if executionID == "" {
		executionID = strings.TrimSpace(r.Header.Get("X-Ironflyer-Execution-ID"))
	}
	// WorkspaceID is allocator-scoped: the quota lease is keyed on it.
	// We let the orchestrator pre-mint one so the lease ID matches the
	// sandbox ID exactly; if absent we synthesize a stable id from the
	// (tenant, execution) pair so Release later finds the right hold.
	allocWorkspaceID := strings.TrimSpace(body.WorkspaceID)
	if allocWorkspaceID == "" {
		allocWorkspaceID = strings.TrimSpace(r.Header.Get("X-Ironflyer-Workspace-ID"))
	}
	if allocWorkspaceID == "" {
		// Best-effort fallback: use the execution ID as the lease key
		// so the orchestrator's Release path (which knows execution_id
		// but not the sandbox ID we mint below) still resolves.
		allocWorkspaceID = executionID
	}

	estCost := estCostFromHeader(r)
	ctx, alloc, err := a.allocateForCreate(r, tenantID, executionID, allocWorkspaceID,
		body.RuntimeClass, body.CPU, body.MemMB, body.EstimatedDurationSec, estCost)
	if err != nil {
		writeAllocatorError(w, alloc, err)
		return
	}
	if !alloc.Allow {
		writeAllocatorError(w, alloc, nil)
		return
	}

	ws, err := a.mgr.Create(ctx, sandbox.CreateOpts{UserID: body.UserID, ProjectID: body.ProjectID})
	if err != nil {
		// Roll back the allocator hold so we don't leak quota when the
		// driver fails partway through container start.
		a.releaseAllocation(ctx, allocRecord{
			TenantID:    tenantID,
			ExecutionID: executionID,
			WorkspaceID: allocWorkspaceID,
			LeaseID:     alloc.LeaseID,
		})
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	// Track the allocation under BOTH the sandbox-minted ID and the
	// caller-supplied lease key so the destroy path resolves regardless
	// of which identifier it has on hand.
	rec := allocRecord{
		TenantID:    tenantID,
		ExecutionID: executionID,
		WorkspaceID: allocWorkspaceID,
		LeaseID:     alloc.LeaseID,
		Source:      alloc.Source,
	}
	a.allocs.put(allocRecord{
		TenantID:    tenantID,
		ExecutionID: executionID,
		WorkspaceID: ws.ID, // sandbox-minted id is what destroy/archive get
		LeaseID:     alloc.LeaseID,
		Source:      alloc.Source,
	})
	if allocWorkspaceID != "" && allocWorkspaceID != ws.ID {
		a.allocs.put(rec)
	}

	// Persist the workspace row + claim ownership on this pod.
	a.recordWorkspace(ctx, ws, ws.HostPath)
	a.logger.Info().
		Str("workspace", ws.ID).
		Str("tenant", tenantID).
		Str("execution", executionID).
		Str("source", alloc.Source).
		Str("runtime_class", alloc.RuntimeClass).
		Str("lease", alloc.LeaseID).
		Msg("allocator admitted workspace")
	// Auto-allocate a live preview binding so the studio iframe has a
	// URL immediately — no waiting for the user to click Publish. Port
	// hint comes from the orchestrator via the X-Ironflyer-Preview-Port
	// header (set per-blueprint); default 3000 covers Next.js / most
	// Node web servers. Failure is logged Warn but never blocks the
	// workspace create.
	previewPort := 3000
	if hint := strings.TrimSpace(r.Header.Get("X-Ironflyer-Preview-Port")); hint != "" {
		if n, err := strconv.Atoi(hint); err == nil && sandbox.PreviewPortAllowed(n) {
			previewPort = n
		}
	}
	if binding, err := a.mgr.AllocatePreview(ctx, ws.ID, previewPort); err != nil {
		a.logger.Warn().Err(err).Str("workspace", ws.ID).Int("port", previewPort).
			Msg("auto-allocate preview failed")
	} else {
		a.previews.put(binding)
		ws.PreviewURL = binding.URL
	}

	// Kick off an opportunistic dependency install in the background. The
	// HTTP response returns immediately — the user sees the workspace come
	// up — but by the time they navigate to the IDE / Preview tab, `npm
	// install` (or equivalent) is likely already finished. This is the
	// difference between Lovable / Bolt.new's "instant" feel and "type a
	// command first." Errors are logged but never surfaced; the next
	// build/test gate will catch a broken install anyway.
	go a.runAutoBuild(ws)
	// Surface allocator metadata in the response so the orchestrator can
	// log source / runtime class without a second round trip.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              ws.ID,
		"userId":          ws.UserID,
		"projectId":       ws.ProjectID,
		"status":          ws.Status,
		"driver":          ws.Driver,
		"root":            ws.Root,
		"previewUrl":      ws.PreviewURL,
		"ideUrl":          ws.IDEURL,
		"idePassword":     ws.IDEPassword,
		"createdAt":       ws.CreatedAt,
		"updatedAt":       ws.UpdatedAt,
		"allocatorSource": alloc.Source,
		"runtimeClass":    alloc.RuntimeClass,
		"leaseId":         alloc.LeaseID,
	})
}

// runAutoBuild detects the workspace's manifest files and runs the
// matching install/bootstrap command. Runs on a fresh background context
// with a generous deadline so it survives the request goroutine exiting.
// Every step is a no-op on detection miss, so a freshly created empty
// workspace incurs zero exec cost.
func (a *API) runAutoBuild(ws sandbox.Workspace) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	files, err := a.mgr.Driver().ListFiles(ctx, ws)
	if err != nil {
		a.logger.Warn().Err(err).Str("workspace", ws.ID).Msg("auto-build: list files")
		return
	}
	has := func(name string) bool {
		for _, f := range files {
			if strings.EqualFold(f.Path, name) || strings.HasSuffix(strings.ToLower(f.Path), "/"+strings.ToLower(name)) {
				return true
			}
		}
		return false
	}

	type step struct {
		when  bool
		shell string
		label string
	}
	steps := []step{
		// Node — prefer pnpm / yarn lockfiles when present, fall back to npm.
		{has("pnpm-lock.yaml"), "command -v pnpm >/dev/null 2>&1 && pnpm install --frozen-lockfile || true", "pnpm install"},
		{has("yarn.lock"), "command -v yarn >/dev/null 2>&1 && yarn install --frozen-lockfile || true", "yarn install"},
		{has("package-lock.json"), "command -v npm >/dev/null 2>&1 && npm ci --no-audit --no-fund || true", "npm ci"},
		// package.json without a lockfile gets a plain install.
		{has("package.json") && !has("package-lock.json") && !has("yarn.lock") && !has("pnpm-lock.yaml"),
			"command -v npm >/dev/null 2>&1 && npm install --no-audit --no-fund || true", "npm install"},
		// Go.
		{has("go.mod"), "command -v go >/dev/null 2>&1 && go mod download || true", "go mod download"},
		// Python.
		{has("requirements.txt"), "command -v pip >/dev/null 2>&1 && pip install --quiet -r requirements.txt || true", "pip install"},
		{has("pyproject.toml"), "command -v poetry >/dev/null 2>&1 && poetry install --no-interaction || true", "poetry install"},
		// Rust.
		{has("Cargo.toml"), "command -v cargo >/dev/null 2>&1 && cargo fetch || true", "cargo fetch"},
	}

	for _, s := range steps {
		if !s.when {
			continue
		}
		res, err := a.mgr.Driver().Exec(ctx, ws, sandbox.ExecOpts{
			Shell: s.shell, TimeoutSeconds: 240,
		})
		if err != nil {
			a.logger.Warn().Err(err).Str("workspace", ws.ID).Str("step", s.label).Msg("auto-build step failed")
			continue
		}
		a.logger.Info().
			Str("workspace", ws.ID).
			Str("step", s.label).
			Int("exitCode", res.ExitCode).
			Int64("durationMs", res.DurationMS).
			Msg("auto-build step done")
	}
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

// ide returns the per-workspace web IDE URL so the studio can embed the
// branded Theia (or code-server) container in an iframe / new tab.
//
// Contract:
//   - 200 {"url": <url>, "ready": true}  when a backend URL is available.
//   - 202 {"url": "",   "ready": false} when the IDE is still starting;
//     the client polls until it flips to 200.
//
// The driver provisions the IDE container as part of workspace Create
// (DockerDriver populates Workspace.IDEURL with the loopback host:port of
// the code-server/Theia container; that step is idempotent because a
// workspace maps to a single long-lived container). The Mock driver has
// no real IDE container and leaves IDEURL empty, so dev runs the Theia
// app locally and points the runtime at it via IRONFLYER_IDE_URL.
func (a *API) ide(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ws, ok := a.lookupWorkspace(r, id)
	if !ok {
		// Dev convenience: when IRONFLYER_RUNTIME_DEV_AUTOCREATE is set, lazily
		// provision a workspace for the project so the studio's IDE pane works
		// without first running a full execution. This bypasses the allocator /
		// ProfitGuard create path, so it MUST stay off in production — in prod
		// the orchestrator provisions the workspace through the gated flow and
		// this returns 404 until it does.
		if strings.TrimSpace(os.Getenv("IRONFLYER_RUNTIME_DEV_AUTOCREATE")) == "" {
			writeJSON(w, http.StatusNotFound, errJSON("not found"))
			return
		}
		uid := userIDFromCtx(r)
		if uid == "" {
			uid = "demo"
		}
		created, err := a.mgr.Create(r.Context(), sandbox.CreateOpts{UserID: uid, ProjectID: id})
		if err != nil {
			a.logger.Warn().Err(err).Str("project", id).Msg("dev-autocreate workspace failed")
			writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
			return
		}
		a.logger.Info().Str("project", id).Str("ws", created.ID).Msg("dev-autocreate workspace for IDE")
		ws = created
	}
	// Dev override: when IRONFLYER_IDE_URL is set we hand back that raw
	// URL with ready=true regardless of driver. This lets a developer run
	// the Theia app locally (e.g. `yarn start` on :3030) without Docker.
	if override := strings.TrimSpace(os.Getenv("IRONFLYER_IDE_URL")); override != "" {
		writeJSON(w, http.StatusOK, map[string]any{"url": override, "ready": true})
		return
	}
	url := strings.TrimSpace(ws.IDEURL)
	if url == "" {
		// Backend not up yet (mock driver, or container still booting).
		// 202 tells the client to keep polling.
		writeJSON(w, http.StatusAccepted, map[string]any{"url": "", "ready": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": url, "ready": true})
}

func (a *API) destroy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := a.requireWorkspace(w, r, id); !ok {
		return
	}
	if err := a.mgr.Destroy(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, errJSON(err.Error()))
		return
	}
	// Hand the warm-pool lease back and drop the quota hold. Release is
	// idempotent so an unknown id (legacy workspaces created before this
	// wiring) just no-ops without surfacing an error to the caller.
	if rec, ok := a.allocs.get(id); ok {
		a.releaseAllocation(r.Context(), rec)
		a.allocs.drop(id)
		// The allocator hold may have been keyed under the
		// orchestrator-supplied WorkspaceID rather than the sandbox-
		// minted id; drop both index entries so the tracker doesn't
		// leak duplicate rows.
		if rec.WorkspaceID != "" && rec.WorkspaceID != id {
			a.allocs.drop(rec.WorkspaceID)
		}
	}
	// Release any preview lease so the external port is reclaimed
	// and the cached binding doesn't outlive the workspace.
	_ = a.mgr.ReleasePreview(r.Context(), id)
	a.previews.drop(id)
	if a.lc.Store != nil {
		_ = a.lc.Store.Delete(r.Context(), id)
	}
	if a.lc.Registry != nil {
		a.lc.Registry.Release(r.Context(), id)
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
	// PTY sessions cannot migrate mid-stream — when this pod begins
	// graceful shutdown, push a single shutdown_imminent frame so the
	// client knows to reconnect. The next websocket open lands on
	// whichever pod the LB picks, which will reattach to the EFS-backed
	// workspace and start a fresh PTY (the user's shell state inside
	// the workspace persists; only the in-progress terminal session is
	// lost — Mosh/SSH-reconnect UX).
	if a.lc.Registry != nil {
		go func() {
			<-ctx.Done()
			_ = c.Write(context.Background(), websocket.MessageText,
				[]byte(`{"type":"pty.shutdown_imminent","reason":"pod-shutdown"}`))
		}()
	}

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
		Healthy     bool   `json:"healthy"`
		LatencyMS   int64  `json:"latencyMs,omitempty"`
	}
	out := make([]item, 0, len(ports))
	// Probe each port concurrently with a 1.5s per-probe deadline so the
	// whole listing returns in well under 2s even with multiple stalled
	// servers. A failed probe is just "Healthy=false" — never an error.
	type probeResult struct {
		i         int
		healthy   bool
		latencyMs int64
	}
	probeCh := make(chan probeResult, len(ports))
	for i, p := range ports {
		if !a.preview.PortAllowed(p.Port) {
			probeCh <- probeResult{i: i, healthy: false}
			continue
		}
		i, port := i, p.Port
		go func() {
			ms, ok := a.preview.HealthCheck(r.Context(), ws.ID, port)
			probeCh <- probeResult{i: i, healthy: ok, latencyMs: ms}
		}()
	}
	results := make([]probeResult, len(ports))
	for range ports {
		res := <-probeCh
		results[res.i] = res
	}
	for i, p := range ports {
		out = append(out, item{
			DetectedPort: p,
			PreviewPath:  a.preview.BuildPreviewPath(ws.ID, p.Port, ""),
			Allowed:      a.preview.PortAllowed(p.Port),
			Healthy:      results[i].healthy,
			LatencyMS:    results[i].latencyMs,
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

// shareLink mints a long-lived signed token bound to a workspace+port so
// the owner can hand a link to teammates without giving them an Ironflyer
// account. Unlike previewToken (30-minute iframe session), this is meant
// to live for days. Operators bound the lifetime via the request body —
// we hard-cap at 30 days because anything longer encourages "I'll just
// share the URL" anti-patterns for what should be a published preview.
func (a *API) shareLink(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body struct {
		Port     int `json:"port"`
		TTLHours int `json:"ttlHours,omitempty"`
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
	// Default share TTL: 7 days. Hard cap: 30 days. Negative / zero values
	// fall back to the default so misclicks don't accidentally mint a
	// 1-second link.
	ttlHours := body.TTLHours
	if ttlHours <= 0 {
		ttlHours = 24 * 7
	}
	if ttlHours > 24*30 {
		ttlHours = 24 * 30
	}
	ttl := time.Duration(ttlHours) * time.Hour
	tok, exp, err := a.signer.MintWithTTL(ws.ID, body.Port, ttl)
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
		"ttlHours":  ttlHours,
	})
}

// screenshot renders the workspace's preview at the given route +
// viewport and returns it as base64-encoded PNG. This is the input the
// orchestrator's UXGate uses to enforce the pixel-perfect VisualTarget
// contract against the live application.
//
// Strategy: pick the first allowed forwarded port that responds to a
// liveness probe, build the preview URL, and ask chromium-headless to
// capture it. We support three execution paths in priority order:
//  1. `chromium-headless` / `google-chrome --headless` inside the
//     workspace — best fidelity, real font + JS render.
//  2. `playwright-cli screenshot` when the workspace has it installed.
//  3. Stub: an 8×8 placeholder PNG so the gate degrades to "size
//     mismatch" rather than hard-failing. Surfaces the wiring problem
//     via the standard gate flow.
func (a *API) screenshot(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body struct {
		Route     string `json:"route"`
		ViewportW int    `json:"viewportW"`
		ViewportH int    `json:"viewportH"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if body.Route == "" {
		body.Route = "/"
	}
	if body.ViewportW <= 0 {
		body.ViewportW = 1280
	}
	if body.ViewportH <= 0 {
		body.ViewportH = 800
	}
	// Resolve the first live preview port. Without one, screenshot is
	// impossible — surface as a 503 so the gate degrades cleanly.
	ports := a.mgr.Ports(ws.ID)
	if len(ports) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("no forwarded preview ports"))
		return
	}
	var picked int
	for _, p := range ports {
		if !a.preview.PortAllowed(p.Port) {
			continue
		}
		if ms, alive := a.preview.HealthCheck(r.Context(), ws.ID, p.Port); alive && ms > 0 {
			picked = p.Port
			break
		}
		if picked == 0 {
			picked = p.Port // best-effort fallback
		}
	}
	if picked == 0 {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("no allowed preview port responded"))
		return
	}

	// Inside the workspace, the dev server is on localhost:<port>. We
	// stream the screenshot bytes out of /tmp once the headless browser
	// writes it. The shell script tries chromium first, then chrome,
	// then playwright; if all are missing we fall back to a stub PNG.
	target := "http://localhost:" + strconv.Itoa(picked) + body.Route
	out := "/tmp/ironflyer-screenshot.png"
	cmd := `set -e
URL=` + shellQuote(target) + `
OUT=` + shellQuote(out) + `
W=` + strconv.Itoa(body.ViewportW) + `
H=` + strconv.Itoa(body.ViewportH) + `
if command -v chromium-browser >/dev/null 2>&1; then
  chromium-browser --headless --disable-gpu --no-sandbox \
    --window-size="${W},${H}" --screenshot="${OUT}" "${URL}" >/dev/null 2>&1
elif command -v chromium >/dev/null 2>&1; then
  chromium --headless --disable-gpu --no-sandbox \
    --window-size="${W},${H}" --screenshot="${OUT}" "${URL}" >/dev/null 2>&1
elif command -v google-chrome >/dev/null 2>&1; then
  google-chrome --headless --disable-gpu --no-sandbox \
    --window-size="${W},${H}" --screenshot="${OUT}" "${URL}" >/dev/null 2>&1
elif command -v npx >/dev/null 2>&1 && npx --no-install playwright --version >/dev/null 2>&1; then
  npx --no-install playwright screenshot --viewport-size "${W},${H}" "${URL}" "${OUT}" >/dev/null 2>&1
else
  echo "no headless browser available — install chromium-browser or playwright in the workspace" >&2
  exit 7
fi
[ -s "${OUT}" ] || { echo "no screenshot produced" >&2; exit 7; }
base64 -w0 "${OUT}" 2>/dev/null || base64 "${OUT}" | tr -d '\n'`
	res, err := a.mgr.Driver().Exec(r.Context(), ws, sandbox.ExecOpts{
		Shell: cmd, TimeoutSeconds: 30,
	})
	if err != nil || res.ExitCode != 0 {
		writeJSON(w, http.StatusBadGateway, errJSON("screenshot failed: "+strings.TrimSpace(res.Stderr)))
		return
	}
	b64 := strings.TrimSpace(res.Stdout)
	if b64 == "" {
		writeJSON(w, http.StatusBadGateway, errJSON("screenshot returned empty"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"imagePngBase64": b64,
		"viewportW":      body.ViewportW,
		"viewportH":      body.ViewportH,
		"route":          body.Route,
		"port":           picked,
	})
}

// shellQuote wraps a value in single quotes and escapes embedded quotes
// so it can be safely interpolated into a /bin/sh -c command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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

// writeJSON / errJSON are thin shims for legacy call sites — the
// implementations now live in internal/pkg/httputil. New code should
// import httputil directly; these shims exist so the migration is
// incremental.
var writeJSON = httputil.WriteJSON

func errJSON(msg string) map[string]string { return map[string]string{"error": msg} }
