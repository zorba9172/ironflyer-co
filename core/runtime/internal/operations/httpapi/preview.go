// preview.go — workspace live-preview port allocation HTTP surface.
//
// Routes:
//
//	POST   /workspaces/{id}/preview            allocate a binding
//	GET    /workspaces/{id}/preview            current binding (404 if none)
//	DELETE /workspaces/{id}/preview            release
//
// All routes live behind the existing auth + ownership middleware so
// only the workspace owner can mint a preview URL. The handler is a
// thin wrapper around sandbox.Manager.AllocatePreview which delegates
// to the active driver and tracks the binding in the workspaces
// in-memory state.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"ironflyer/core/runtime/internal/operations/sandbox"
)

// previewState caches the most recent binding per workspace so a
// later GET can answer without dialling the driver. Cleared on
// release and on workspace destroy. Tracks the same content the
// driver returns; the driver remains the source of truth.
type previewState struct {
	mu   sync.RWMutex
	rows map[string]sandbox.PreviewBinding
}

func newPreviewState() *previewState {
	return &previewState{rows: make(map[string]sandbox.PreviewBinding)}
}

func (s *previewState) put(b sandbox.PreviewBinding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[b.WorkspaceID] = b
}

func (s *previewState) get(id string) (sandbox.PreviewBinding, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.rows[id]
	if !ok {
		return sandbox.PreviewBinding{}, false
	}
	if !b.ExpiresAt.IsZero() && time.Now().After(b.ExpiresAt) {
		return sandbox.PreviewBinding{}, false
	}
	return b, true
}

func (s *previewState) drop(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, id)
}

// allocatePreview handles POST /workspaces/{id}/preview.
func (a *API) allocatePreview(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body struct {
		InternalPort int `json:"internalPort"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if body.InternalPort == 0 {
		body.InternalPort = 3000
	}
	if !sandbox.PreviewPortAllowed(body.InternalPort) {
		writeJSON(w, http.StatusBadRequest, errJSON("internal port not on safelist"))
		return
	}
	binding, err := a.mgr.AllocatePreview(r.Context(), ws.ID, body.InternalPort)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	a.previews.put(binding)
	writeJSON(w, http.StatusOK, binding)
}

// getPreview handles GET /workspaces/{id}/preview.
func (a *API) getPreview(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if b, ok := a.previews.get(ws.ID); ok {
		writeJSON(w, http.StatusOK, b)
		return
	}
	writeJSON(w, http.StatusNotFound, errJSON("no preview allocated"))
}

// releasePreview handles DELETE /workspaces/{id}/preview.
func (a *API) releasePreview(w http.ResponseWriter, r *http.Request) {
	ws, ok := a.requireWorkspace(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if err := a.mgr.ReleasePreview(r.Context(), ws.ID); err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	a.previews.drop(ws.ID)
	w.WriteHeader(http.StatusNoContent)
}
