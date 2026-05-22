// Deploy-related HTTP handlers + route registration.
//
// The deploy surface is intentionally self-contained: a small RegisterDeploy
// function appends every route to an existing chi.Router, leaving the main
// api.go file untouched apart from one call site. This keeps the boundary
// between agents clean while still exposing the routes under the same auth
// + project-ownership guarantees as the rest of the API.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/finisher"
	"ironflyer/apps/orchestrator/internal/integrations/deploy"
	"ironflyer/apps/orchestrator/internal/patch"
)

// deployDeps groups the runtime-resolved values the handlers need. We
// resolve them lazily from a.d so the API struct keeps a single source of
// truth and tests can stub providers via env.
type deployDeps struct {
	fly     *deploy.FlyClient
	railway *deploy.RailwayClient
	gh      *deploy.GitHubExporter
}

func (a *API) deployDeps() deployDeps {
	return deployDeps{
		fly:     deploy.NewFly(os.Getenv("FLY_API_TOKEN")),
		railway: deploy.NewRailway(os.Getenv("RAILWAY_TOKEN")),
		gh:      deploy.NewGitHubExporter(),
	}
}

// deploymentRecord captures everything the UI needs to render a row in the
// "past deployments" list. We hold these in memory keyed by project ID;
// switching to Postgres is straightforward (mirror the leads store).
type deploymentRecord struct {
	ID         string                 `json:"id"`
	ProjectID  string                 `json:"projectId"`
	UserID     string                 `json:"userId"`
	Provider   string                 `json:"provider"`
	Region     string                 `json:"region,omitempty"`
	Status     string                 `json:"status"`
	URL        string                 `json:"url,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Artifacts  []finisher.DeployArtifact `json:"artifacts,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
	FinishedAt *time.Time             `json:"finishedAt,omitempty"`
}

// deploymentStreamer fans deploy events out to one or more SSE subscribers.
// A deployment's lifecycle is: created → many events → terminal. Subscribers
// that attach after the terminal event still get the cached terminal so
// page reloads don't lose the URL.
type deploymentStreamer struct {
	mu       sync.Mutex
	events   []deploymentEvent
	subs     map[chan deploymentEvent]struct{}
	done     bool
	terminal deploymentEvent
}

type deploymentEvent struct {
	Kind  string `json:"kind"`
	Line  string `json:"line,omitempty"`
	URL   string `json:"url,omitempty"`
	Error string `json:"error,omitempty"`
	At    time.Time `json:"at"`
}

func (s *deploymentStreamer) push(ev deploymentEvent) {
	ev.At = time.Now().UTC()
	s.mu.Lock()
	s.events = append(s.events, ev)
	if ev.Kind == "deployed" || ev.Kind == "failed" {
		s.done = true
		s.terminal = ev
	}
	for ch := range s.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	if s.done {
		for ch := range s.subs {
			close(ch)
			delete(s.subs, ch)
		}
	}
	s.mu.Unlock()
}

func (s *deploymentStreamer) subscribe() (chan deploymentEvent, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan deploymentEvent, 32)
	for _, ev := range s.events {
		select {
		case ch <- ev:
		default:
		}
	}
	if s.done {
		close(ch)
		return ch, func() {}
	}
	if s.subs == nil {
		s.subs = make(map[chan deploymentEvent]struct{})
	}
	s.subs[ch] = struct{}{}
	return ch, func() {
		s.mu.Lock()
		if _, ok := s.subs[ch]; ok {
			delete(s.subs, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
}

// deployRegistry is the global in-memory store of deployment records +
// active streams. Bounded growth is a known limitation — we keep at most
// 50 records per project to avoid leaking memory while keeping the UI
// honest about deploy history.
type deployRegistry struct {
	mu      sync.RWMutex
	records map[string][]*deploymentRecord // projectID -> records
	streams map[string]*deploymentStreamer  // deploymentID -> streamer
}

var globalDeployRegistry = &deployRegistry{
	records: make(map[string][]*deploymentRecord),
	streams: make(map[string]*deploymentStreamer),
}

func (r *deployRegistry) create(rec *deploymentRecord) *deploymentStreamer {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records[rec.ProjectID] = append(r.records[rec.ProjectID], rec)
	if len(r.records[rec.ProjectID]) > 50 {
		r.records[rec.ProjectID] = r.records[rec.ProjectID][len(r.records[rec.ProjectID])-50:]
	}
	stream := &deploymentStreamer{}
	r.streams[rec.ID] = stream
	return stream
}

func (r *deployRegistry) list(projectID string) []*deploymentRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	src := r.records[projectID]
	out := make([]*deploymentRecord, len(src))
	copy(out, src)
	return out
}

func (r *deployRegistry) stream(deploymentID string) *deploymentStreamer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.streams[deploymentID]
}

func (r *deployRegistry) finish(deploymentID, status, url, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, list := range r.records {
		for _, rec := range list {
			if rec.ID == deploymentID {
				now := time.Now().UTC()
				rec.Status = status
				rec.URL = url
				rec.Error = errMsg
				rec.FinishedAt = &now
				return
			}
		}
	}
}

// RegisterDeploy is the single integration point with api.go. It is called
// once at startup; every deploy route gets the same auth middleware as
// the rest of the protected surface.
func (a *API) RegisterDeploy(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(a.authMiddleware())
		r.Route("/projects/{id}/deploy", func(r chi.Router) {
			r.Post("/", a.deployStart)
			r.Get("/plan", a.deployPlan)
		})
		r.Get("/projects/{id}/deployments", a.deployList)
		r.Get("/deployments/{deploymentId}/stream", a.deployStream)
		r.Post("/projects/{id}/export/zip", a.exportZip)
		r.Post("/projects/{id}/export/github", a.exportGitHub)
	})
}

// ---- /projects/{id}/deploy/plan ----

func (a *API) deployPlan(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	d := a.deployDeps()
	writeJSON(w, http.StatusOK, map[string]any{
		"stack":     detectStackForUI(&p),
		"artifacts": finisher.PlanDeployArtifacts(&p, ""),
		"providers": map[string]bool{
			"fly":     d.fly.Enabled(),
			"railway": d.railway.Enabled(),
		},
	})
}

func detectStackForUI(p *domain.Project) string {
	for _, f := range p.Files {
		low := strings.ToLower(f.Path)
		if low == "go.mod" {
			return "go"
		}
		if low == "package.json" {
			return "node"
		}
	}
	if strings.Contains(strings.ToLower(p.Spec.Stack.Backend), "go") {
		return "go"
	}
	return "node"
}

// ---- POST /projects/{id}/deploy ----

func (a *API) deployStart(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	p, ok := a.requireProjectAccess(w, r, projectID)
	if !ok {
		return
	}
	var body struct {
		Provider string            `json:"provider"`
		Region   string            `json:"region"`
		Env      map[string]string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	provider := strings.ToLower(strings.TrimSpace(body.Provider))
	if provider != "fly" && provider != "railway" {
		writeJSON(w, http.StatusBadRequest, errJSON("provider must be fly or railway"))
		return
	}
	d := a.deployDeps()
	if provider == "fly" && !d.fly.Enabled() {
		writeJSON(w, http.StatusPreconditionFailed, errJSON("FLY_API_TOKEN not configured on the orchestrator"))
		return
	}
	if provider == "railway" && !d.railway.Enabled() {
		writeJSON(w, http.StatusPreconditionFailed, errJSON("RAILWAY_TOKEN not configured on the orchestrator"))
		return
	}

	stack := detectStackForUI(&p)
	plan := finisher.PlanDeployArtifacts(&p, stack)
	if err := ensureDeployArtifacts(&p, plan, a.d.Patches, userIDFromCtx(r)); err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON("artifact bootstrap failed: "+err.Error()))
		return
	}

	rec := &deploymentRecord{
		ID:        "dep_" + uuid.NewString(),
		ProjectID: projectID,
		UserID:    userIDFromCtx(r),
		Provider:  provider,
		Region:    body.Region,
		Status:    "running",
		Artifacts: plan,
		CreatedAt: time.Now().UTC(),
	}
	stream := globalDeployRegistry.create(rec)
	stream.push(deploymentEvent{Kind: "deploy_started", Line: fmt.Sprintf("starting %s deploy for %s", provider, p.Name)})

	go a.runDeploy(context.Background(), rec, &p, body.Env, stream, d)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"deploymentId": rec.ID,
		"streamURL":    fmt.Sprintf("/deployments/%s/stream", rec.ID),
	})
}

// runDeploy is the actual background worker. It materializes the project
// files to a temp dir, then hands off to the provider client. Every line
// the provider emits is forwarded as an SSE event.
func (a *API) runDeploy(ctx context.Context, rec *deploymentRecord, p *domain.Project, env map[string]string, stream *deploymentStreamer, d deployDeps) {
	dir, cleanup, err := materializeProject(p)
	if err != nil {
		stream.push(deploymentEvent{Kind: "failed", Error: "materialize: " + err.Error()})
		globalDeployRegistry.finish(rec.ID, "failed", "", err.Error())
		return
	}
	defer cleanup()
	stream.push(deploymentEvent{Kind: "build_started", Line: "materialized project at " + dir})

	onLine := func(s string) {
		if s == "" {
			return
		}
		stream.push(deploymentEvent{Kind: "log", Line: s})
	}

	switch rec.Provider {
	case "fly":
		appName := flyAppName(p)
		stream.push(deploymentEvent{Kind: "push_started", Line: "fly: ensuring app " + appName})
		if err := d.fly.CreateApp(ctx, appName, ""); err != nil {
			stream.push(deploymentEvent{Kind: "failed", Error: err.Error()})
			globalDeployRegistry.finish(rec.ID, "failed", "", err.Error())
			return
		}
		if err := d.fly.DeployFromDir(ctx, appName, dir, env, onLine); err != nil {
			stream.push(deploymentEvent{Kind: "failed", Error: err.Error()})
			globalDeployRegistry.finish(rec.ID, "failed", "", err.Error())
			return
		}
		url := d.fly.GetURL(appName)
		stream.push(deploymentEvent{Kind: "deployed", URL: url, Line: "live at " + url})
		globalDeployRegistry.finish(rec.ID, "deployed", url, "")
	case "railway":
		app, err := d.railway.CreateApp(ctx, railwayProjectName(p))
		if err != nil {
			stream.push(deploymentEvent{Kind: "failed", Error: err.Error()})
			globalDeployRegistry.finish(rec.ID, "failed", "", err.Error())
			return
		}
		stream.push(deploymentEvent{Kind: "push_started", Line: "railway project " + app.ProjectID})
		if err := d.railway.DeployFromDir(ctx, app, dir, env, onLine); err != nil {
			stream.push(deploymentEvent{Kind: "failed", Error: err.Error()})
			globalDeployRegistry.finish(rec.ID, "failed", "", err.Error())
			return
		}
		url, _ := d.railway.GetURL(ctx, app.ServiceID)
		stream.push(deploymentEvent{Kind: "deployed", URL: url, Line: "deploy complete"})
		globalDeployRegistry.finish(rec.ID, "deployed", url, "")
	}
}

// materializeProject writes the project's in-memory files into a temp
// directory the provider CLIs can read. Returns the absolute path + a
// cleanup func that removes the tree.
func materializeProject(p *domain.Project) (string, func(), error) {
	dir, err := os.MkdirTemp("", "ironflyer-deploy-"+p.ID+"-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	for _, f := range p.Files {
		clean := strings.TrimPrefix(strings.TrimPrefix(f.Path, "/"), "./")
		if clean == "" {
			continue
		}
		full := filepath.Join(dir, clean)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			cleanup()
			return "", func() {}, err
		}
		if err := os.WriteFile(full, []byte(f.Content), 0o644); err != nil {
			cleanup()
			return "", func() {}, err
		}
	}
	return dir, cleanup, nil
}

// ensureDeployArtifacts proposes + applies a patch that writes any
// missing deploy artifacts. We go through the patch lifecycle so the
// security gate gets a chance to scan generated files.
func ensureDeployArtifacts(p *domain.Project, plan []finisher.DeployArtifact, engine *patch.Engine, userID string) error {
	if engine == nil {
		return errors.New("patch engine not configured")
	}
	var changes []patch.FileChange
	for _, art := range plan {
		if art.Source == "existing" {
			continue
		}
		body, ok := finisher.DeployTemplateBody(art, p)
		if !ok {
			continue
		}
		changes = append(changes, patch.FileChange{
			Op:      patch.OpCreate,
			Path:    art.Path,
			Content: body,
		})
	}
	if len(changes) == 0 {
		return nil
	}
	pt, err := engine.Propose(patch.Patch{
		ProjectID: p.ID,
		Author:    "deploy:" + userID,
		Title:     "Generate deploy artifacts",
		Summary:   fmt.Sprintf("DeployGate generated %d deploy artifact(s)", len(changes)),
		Changes:   changes,
	})
	if err != nil {
		return err
	}
	if pt.Status == patch.StatusRejected {
		return fmt.Errorf("patch rejected: %v", pt.Issues)
	}
	if _, err := engine.Apply(pt.ID); err != nil {
		return err
	}
	// Reflect the applied files into our in-memory copy so the deploy
	// worker sees them when it materializes the tree.
	for _, c := range changes {
		replaceOrAppendFile(p, c.Path, c.Content)
	}
	return nil
}

func replaceOrAppendFile(p *domain.Project, path, content string) {
	for i, f := range p.Files {
		if f.Path == path {
			p.Files[i].Content = content
			p.Files[i].Size = len(content)
			return
		}
	}
	p.Files = append(p.Files, domain.FileNode{Path: path, Type: "file", Size: len(content), Content: content})
}

func flyAppName(p *domain.Project) string {
	id := strings.ToLower(p.ID)
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ':
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "app"
	}
	if len(name) > 24 {
		name = name[:24]
	}
	return "iflyr-" + name
}

func railwayProjectName(p *domain.Project) string {
	return "ironflyer-" + strings.ToLower(strings.ReplaceAll(p.ID, " ", "-"))
}

// ---- GET /projects/{id}/deployments ----

func (a *API) deployList(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if _, ok := a.requireProjectAccess(w, r, projectID); !ok {
		return
	}
	records := globalDeployRegistry.list(projectID)
	uid := userIDFromCtx(r)
	out := make([]*deploymentRecord, 0, len(records))
	for _, rec := range records {
		// Per-user filter; in-mem registry doesn't enforce this on writes.
		if rec.UserID != "" && rec.UserID != uid {
			continue
		}
		out = append(out, rec)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- GET /deployments/{id}/stream ----

func (a *API) deployStream(w http.ResponseWriter, r *http.Request) {
	depID := chi.URLParam(r, "deploymentId")
	stream := globalDeployRegistry.stream(depID)
	if stream == nil {
		writeJSON(w, http.StatusNotFound, errJSON("deployment not found"))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	sseHeaders(w)
	ch, unsub := stream.subscribe()
	defer unsub()
	hb := time.NewTicker(15 * time.Second)
	defer hb.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, payload)
			flusher.Flush()
		case <-hb.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// ---- POST /projects/{id}/export/zip ----

func (a *API) exportZip(w http.ResponseWriter, r *http.Request) {
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	files := make([]deploy.SourceFile, 0, len(p.Files))
	for _, f := range p.Files {
		if f.Type == "dir" {
			continue
		}
		files = append(files, deploy.SourceFile{Path: f.Path, Content: f.Content})
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, sanitizeFilename(p.ID)))
	if _, err := deploy.WriteZip(w, files); err != nil {
		// Headers are already sent; best we can do is log + abort.
		a.d.Logger.Warn().Err(err).Str("project", p.ID).Msg("zip export failed mid-stream")
	}
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "ironflyer-project"
	}
	return b.String()
}

// ---- POST /projects/{id}/export/github ----

func (a *API) exportGitHub(w http.ResponseWriter, r *http.Request) {
	if a.d.GitHub == nil || !a.d.GitHub.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, errJSON("github integration disabled"))
		return
	}
	p, ok := a.requireProjectAccess(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body struct {
		RepoName    string `json:"repoName"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errJSON("invalid JSON"))
		return
	}
	if strings.TrimSpace(body.RepoName) == "" {
		body.RepoName = sanitizeFilename(p.ID)
	}
	token, err := a.d.GitHub.TokenFor(r.Context(), userIDFromCtx(r))
	if err != nil {
		writeJSON(w, http.StatusPreconditionRequired, errJSON("connect GitHub first"))
		return
	}
	files := make([]deploy.SourceFile, 0, len(p.Files))
	for _, f := range p.Files {
		if f.Type == "dir" {
			continue
		}
		files = append(files, deploy.SourceFile{Path: f.Path, Content: f.Content})
	}
	exporter := deploy.NewGitHubExporter()
	res, err := exporter.Export(r.Context(), deploy.ExportRequest{
		Token:       token,
		RepoName:    body.RepoName,
		Description: firstNonEmpty(body.Description, p.Description, "Exported from Ironflyer"),
		Private:     body.Private,
		Files:       files,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// silence unused import warnings during early scaffolds.
var _ = io.Discard
