package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"ironflyer/core/runtime/internal/operations/sandbox"
	"ironflyer/core/runtime/internal/operations/workspaces"
)

// Lifecycle wires the scale-ready dependencies into the runtime HTTP
// API. It's a thin extension over the existing API struct: when its
// fields are nil (dev / single-pod) every handler degrades to a no-op
// or a local-only path, so the legacy mock + memory store flow keeps
// working without conditionals at call sites.
type Lifecycle struct {
	Store    workspaces.Store
	Registry *workspaces.Registry
	Archiver *workspaces.Archiver
}

// SetLifecycle attaches the durable workspace dependencies. Safe to
// call before or after Routes registration — the handlers look up the
// fields lazily on every request.
func (a *API) SetLifecycle(lc Lifecycle) {
	a.lc = lc
}

// registerLifecycleRoutes wires the new endpoints. Called from New().
func (a *API) registerLifecycleRoutes(r chi.Router) {
	r.Post("/workspaces/{id}/archive", a.archiveWorkspace)
	r.Post("/workspaces/{id}/restore", a.restoreWorkspace)
	r.Get("/workspaces/{id}/locator", a.locateWorkspace)
}

// archiveWorkspace stops the container and uploads the workspace dir
// to S3. Admin-only contract — the orchestrator does not call this on
// behalf of end users.
func (a *API) archiveWorkspace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if a.lc.Archiver == nil || !a.lc.Archiver.Enabled() {
		writeJSON(w, http.StatusNotImplemented, errJSON("archive: S3 bucket not configured"))
		return
	}
	rec, err := a.lc.Store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return
	}
	if !recordAccessibleBy(rec, userIDFromCtx(r)) {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return
	}
	// Best-effort: stop the container if currently running on this pod.
	if ws, err := a.mgr.Get(id); err == nil {
		_ = a.mgr.Driver().Destroy(r.Context(), ws)
		_ = a.mgr.Destroy(r.Context(), id)
	}
	// Archived workspaces are not coming back on this pod — drop the
	// warm-pool lease + quota hold so the tenant isn't billed against
	// quota for an offline workspace. Restore will run through the
	// normal create funnel and pull a fresh allocation.
	if rec, ok := a.allocs.get(id); ok {
		a.releaseAllocation(r.Context(), rec)
		a.allocs.drop(id)
		if rec.WorkspaceID != "" && rec.WorkspaceID != id {
			a.allocs.drop(rec.WorkspaceID)
		}
	}
	if err := a.lc.Archiver.Archive(r.Context(), rec); err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":       "archived",
		"s3ArchiveKey": a.lc.Archiver.Key(id),
	})
}

// restoreWorkspace pulls an archived workspace back to EFS and marks
// it `running`. The next file/exec request will lazily recreate the
// container.
func (a *API) restoreWorkspace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if a.lc.Archiver == nil || !a.lc.Archiver.Enabled() {
		writeJSON(w, http.StatusNotImplemented, errJSON("restore: S3 bucket not configured"))
		return
	}
	rec, err := a.lc.Store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return
	}
	if !recordAccessibleBy(rec, userIDFromCtx(r)) {
		writeJSON(w, http.StatusNotFound, errJSON("not found"))
		return
	}
	if rec.Status != workspaces.StatusArchived {
		writeJSON(w, http.StatusConflict, errJSON("workspace not archived"))
		return
	}
	path, err := a.lc.Archiver.Restore(r.Context(), rec)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "running",
		"efsPath": path,
	})
}

// locateWorkspace tells the caller which runtime pod currently owns the
// workspace. The orchestrator can use this to route follow-up calls
// (PTY open, file write) straight to the owning pod, bypassing the
// Service load balancer.
func (a *API) locateWorkspace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var status string
	rec, err := a.lc.lookupRecord(r.Context(), id)
	if err == nil {
		if !recordAccessibleBy(rec, userIDFromCtx(r)) {
			writeJSON(w, http.StatusNotFound, errJSON("not found"))
			return
		}
		status = string(rec.Status)
	}
	var activePod string
	if a.lc.Registry != nil {
		if v, err := a.lc.Registry.Lookup(r.Context(), id); err == nil {
			activePod = v
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workspaceID": id,
		"activePodIP": activePod,
		"status":      status,
		"selfPodIP":   a.lc.Registry.PodIP(),
	})
}

// proxyToActivePod is the entry-point middleware. If the workspace
// has an active pod that is not us, we reverse-proxy the request
// (excluding the WebSocket terminal path — those must hit the active
// pod directly, see PTY migration notes in docs/runtime-scale.md).
// When the workspace has no active pod, we claim it ourselves so
// follow-up calls converge.
func (a *API) proxyToActivePod(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.lc.Registry == nil {
			next.ServeHTTP(w, r)
			return
		}
		id := workspaceIDFromPath(r.URL.Path)
		if id == "" {
			next.ServeHTTP(w, r)
			return
		}
		// Hot-path: refresh activity so the idle scanner doesn't ship
		// us off to S3 while users are typing.
		if a.lc.Store != nil {
			_ = a.lc.Store.TouchActive(r.Context(), id)
		}

		owner, err := a.lc.Registry.Lookup(r.Context(), id)
		if err != nil {
			// On Redis trouble, prefer "serve locally" over "fail closed"
			// — Redis is for coordination, not authorization. The owner
			// check is enforced by the workspace.OwnerID column anyway.
			a.logger.Warn().Err(err).Msg("registry lookup")
		}
		self := a.lc.Registry.PodIP()
		if owner == "" {
			// Take ownership and serve.
			if _, cerr := a.lc.Registry.Claim(r.Context(), id); cerr != nil {
				a.logger.Warn().Err(cerr).Str("workspace", id).Msg("claim ownership")
			}
			next.ServeHTTP(w, r)
			return
		}
		if owner == self || self == "" || isProxyLoop(r) {
			next.ServeHTTP(w, r)
			return
		}
		// Lazy auto-restore: if the row says archived, surface a 409 so
		// the caller can issue /restore.
		if rec, err := a.lc.lookupRecord(r.Context(), id); err == nil && rec.Status == workspaces.StatusArchived {
			writeJSON(w, http.StatusConflict, errJSON("workspace archived; call POST /workspaces/<id>/restore first"))
			return
		}
		a.reverseProxy(owner, w, r)
	})
}

// reverseProxy forwards the request to the owning pod over HTTP. The
// `X-Ironflyer-Proxy` header prevents accidental loops if the target
// pod also reverse-proxies back to us.
func (a *API) reverseProxy(podIP string, w http.ResponseWriter, r *http.Request) {
	target := &url.URL{Scheme: "http", Host: podIP + ":8090"}
	rp := httputil.NewSingleHostReverseProxy(target)
	r.Header.Set("X-Ironflyer-Proxy", a.lc.Registry.PodIP())
	rp.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		a.logger.Warn().Err(err).Str("target", target.String()).Msg("reverse proxy failed")
		writeJSON(rw, http.StatusBadGateway, errJSON("active pod unreachable: "+err.Error()))
	}
	rp.ServeHTTP(w, r)
}

func isProxyLoop(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("X-Ironflyer-Proxy")) != ""
}

// workspaceIDFromPath plucks `{id}` from `/workspaces/{id}/...` paths.
// Returns "" for any other path so the middleware short-circuits to
// next.ServeHTTP for non-workspace routes (healthz, preview, etc.).
func workspaceIDFromPath(path string) string {
	const prefix = "/workspaces/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := path[len(prefix):]
	if cut := strings.IndexByte(rest, '/'); cut >= 0 {
		rest = rest[:cut]
	}
	if rest == "" {
		return ""
	}
	return rest
}

// recordAccessibleBy mirrors sandbox.Workspace.IsAccessibleBy for durable
// store records: an empty caller (no auth context, dev/single-pod) and an
// empty owner (legacy record) both pass; otherwise the owner must match.
func recordAccessibleBy(rec workspaces.Record, userID string) bool {
	return userID == "" || rec.OwnerID == "" || rec.OwnerID == userID
}

// lookupRecord is the nil-safe Store accessor. When the Lifecycle has
// no Store (legacy in-memory mode) we synthesize a "running" record
// from the sandbox manager so every code path keeps working.
func (lc Lifecycle) lookupRecord(ctx context.Context, id string) (workspaces.Record, error) {
	if lc.Store != nil {
		return lc.Store.Get(ctx, id)
	}
	return workspaces.Record{ID: id, Status: workspaces.StatusRunning}, nil
}

// HandleShutdown releases every workspace this pod owns. Called from
// main.go's SIGTERM handler after the HTTP server has been Shutdown'd.
// Hands off to whatever other pod the next request picks (via the LB).
func (a *API) HandleShutdown(ctx context.Context) {
	if a.lc.Registry == nil {
		return
	}
	for _, id := range a.lc.Registry.OwnedIDs() {
		a.lc.Registry.Release(ctx, id)
		// Surface a hint to subscribers (PTY clients) by setting the
		// shutdown flag in the workspace record. The client reconnects
		// will land elsewhere via the LB.
		if a.lc.Store != nil {
			_ = a.lc.Store.UpdateActivePod(ctx, id, "")
		}
	}
}

// recordWorkspace persists a freshly-created workspace into the
// durable store. Used by the create handler when Lifecycle is wired.
func (a *API) recordWorkspace(ctx context.Context, ws sandbox.Workspace, efsPath string) {
	if a.lc.Store == nil {
		return
	}
	now := workspaces.Record{
		ID:        ws.ID,
		OwnerID:   ws.UserID,
		ProjectID: ws.ProjectID,
		Driver:    ws.Driver,
		Status:    workspaces.StatusRunning,
		EFSPath:   efsPath,
	}
	if err := a.lc.Store.Insert(ctx, now); err != nil {
		a.logger.Warn().Err(err).Str("workspace", ws.ID).Msg("store insert")
	}
	if a.lc.Registry != nil {
		if _, err := a.lc.Registry.Claim(ctx, ws.ID); err != nil {
			a.logger.Warn().Err(err).Str("workspace", ws.ID).Msg("registry claim")
		}
	}
}

// readJSONBody is shared by lifecycle handlers that don't need their
// own bespoke parser. Errors return a uniform 400.
func readJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return false
	}
	return true
}
