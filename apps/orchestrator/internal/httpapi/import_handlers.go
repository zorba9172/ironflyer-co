package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"ironflyer/apps/orchestrator/internal/integrations/github"
)

// importerOnce guards lazy construction of the Importer engine. We keep it
// scoped to the API instance via a package-level map keyed by the API
// pointer — simpler than threading another field through Deps and avoids
// modifying existing constructor code in this PR.
var (
	importerMu   sync.Mutex
	importerByAPI = map[*API]*github.Importer{}
)

func (a *API) importer() *github.Importer {
	importerMu.Lock()
	defer importerMu.Unlock()
	if im, ok := importerByAPI[a]; ok && im != nil {
		return im
	}
	im := github.NewImporter(a.d.GitHub, a.d.GitHubTokens, a.d.Projects, a.d.RuntimeURL)
	importerByAPI[a] = im
	return im
}

// startImport handles POST /imports.
//
// Content negotiation:
//   - If the caller sends `Accept: text/event-stream`, the response is an
//     SSE stream emitting one event per pipeline stage (import_started,
//     cloning, cloned, detecting_stack, stack_detected, ready, failed,
//     warning). The terminal `ready` event carries the full ImportResult.
//   - Otherwise the response is a single JSON object with the same
//     ImportResult shape, returned after the pipeline completes.
//
// Body: { repoUrl, branch?, subdir?, makePublic? }
func (a *API) startImport(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(a.d.RuntimeURL) == "" {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("runtime URL not configured"))
		return
	}
	uid := userIDFromCtx(r)
	if uid == "" {
		writeJSON(w, http.StatusUnauthorized, errJSON("unauthenticated"))
		return
	}
	var body struct {
		RepoURL    string `json:"repoUrl"`
		Branch     string `json:"branch"`
		Subdir     string `json:"subdir"`
		MakePublic bool   `json:"makePublic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.RepoURL) == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("repoUrl required"))
		return
	}

	req := github.ImportRequest{
		UserID:     uid,
		UserBearer: bearerFrom(r),
		RepoURL:    body.RepoURL,
		Branch:     body.Branch,
		Subdir:     body.Subdir,
		MakePublic: body.MakePublic,
	}

	if wantsSSE(r) {
		a.streamImport(w, r, req)
		return
	}

	res, err := a.importer().Run(r.Context(), req, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) streamImport(w http.ResponseWriter, r *http.Request, req github.ImportRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	sseHeaders(w)

	// Heartbeat ticker keeps the connection alive on long clones.
	hb := time.NewTicker(15 * time.Second)
	defer hb.Stop()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-hb.C:
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	}()

	var emitMu sync.Mutex
	emit := func(evt github.ProgressEvent) {
		emitMu.Lock()
		defer emitMu.Unlock()
		payload, _ := json.Marshal(evt)
		// `event:` is the canonical lifecycle name; the JSON `type` field
		// duplicates it for clients using the default `message` channel.
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, payload)
		flusher.Flush()
	}

	res, err := a.importer().Run(r.Context(), req, emit)
	close(done)
	if err != nil {
		payload, _ := json.Marshal(map[string]string{"type": "failed", "error": err.Error()})
		fmt.Fprintf(w, "event: failed\ndata: %s\n\n", payload)
		flusher.Flush()
		return
	}
	// Final result mirror — clients can subscribe to `result` for a fully
	// marshalled ImportResult once `ready` arrives.
	payload, _ := json.Marshal(res)
	fmt.Fprintf(w, "event: result\ndata: %s\n\n", payload)
	flusher.Flush()
}

// importStatus is a polling fallback for clients that don't want to hold
// an SSE connection open. It returns the current Project record (with
// .status reflecting the latest lifecycle phase).
func (a *API) importStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"projectId": p.ID,
		"status":    p.Status,
		"stack":     p.Spec.Stack,
		"files":     p.Files,
		"github":    p.GitHub,
		"updatedAt": p.UpdatedAt,
	})
}

func wantsSSE(r *http.Request) bool {
	a := r.Header.Get("Accept")
	return strings.Contains(strings.ToLower(a), "text/event-stream")
}
