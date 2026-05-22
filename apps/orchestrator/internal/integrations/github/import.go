package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/integrations"
	"ironflyer/apps/orchestrator/internal/store"
)

// ImportRequest is the public input shape for "import a GitHub repo as a
// new Ironflyer project". Branch + Subdir are optional. MakePublic controls
// whether the resulting Project is readable by anyone (default: owner-only).
type ImportRequest struct {
	UserID     string
	UserBearer string // forwarded to the runtime so per-user ownership holds
	RepoURL    string
	Branch     string
	Subdir     string
	MakePublic bool
}

// ImportResult is what the engine hands back when the pipeline finishes.
type ImportResult struct {
	ProjectID   string                `json:"projectId"`
	WorkspaceID string                `json:"workspaceId"`
	Stack       domain.StackDecision  `json:"stack"`
	Files       []domain.FileNode     `json:"files,omitempty"`
	Warnings    []string              `json:"warnings,omitempty"`
}

// ProgressEvent is the heartbeat the SSE handler streams to the browser.
// `Type` is the canonical event name documented in the agent brief.
type ProgressEvent struct {
	Type        string `json:"type"`
	Message     string `json:"message,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	Stack       *domain.StackDecision `json:"stack,omitempty"`
	Warning     string `json:"warning,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Importer is the engine. Construct it once at server start (via httpapi
// wiring) and reuse — it is stateless aside from the HTTP client.
type Importer struct {
	Service    *Service               // for resolving OAuth tokens (may be nil if integration disabled)
	Tokens     integrations.TokenStore // direct fallback when Service is unavailable
	Projects   store.Store
	RuntimeURL string
	HTTP       *http.Client
	Detector   *StackDetector
}

// NewImporter wires the engine. A nil RuntimeURL or Projects store means
// Run will fail fast — those are hard preconditions.
func NewImporter(svc *Service, tokens integrations.TokenStore, projects store.Store, runtimeURL string) *Importer {
	hc := &http.Client{Timeout: 4 * time.Minute}
	return &Importer{
		Service:    svc,
		Tokens:     tokens,
		Projects:   projects,
		RuntimeURL: strings.TrimRight(runtimeURL, "/"),
		HTTP:       hc,
		Detector:   &StackDetector{RuntimeURL: strings.TrimRight(runtimeURL, "/"), HTTP: hc},
	}
}

// Run executes the full import pipeline. `emit` receives streaming
// progress; pass a no-op closure for non-streaming callers. The returned
// ImportResult is also delivered as the final `ready` event.
func (im *Importer) Run(ctx context.Context, req ImportRequest, emit func(ProgressEvent)) (ImportResult, error) {
	if emit == nil {
		emit = func(ProgressEvent) {}
	}
	if im.Projects == nil {
		return ImportResult{}, errors.New("import: projects store not configured")
	}
	if im.RuntimeURL == "" {
		return ImportResult{}, errors.New("import: runtime URL not configured")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return ImportResult{}, errors.New("import: userID required")
	}

	cloneURL, owner, repo, err := normaliseRepoURL(req.RepoURL)
	if err != nil {
		return ImportResult{}, err
	}

	// Resolve the GitHub access token, if the user has linked an account.
	// Anonymous flow still works for public repos.
	token := ""
	if im.Service != nil {
		if t, terr := im.Service.TokenFor(ctx, req.UserID); terr == nil {
			token = t
		}
	} else if im.Tokens != nil {
		if t, terr := im.Tokens.Get(ctx, req.UserID, integrations.KindGitHub); terr == nil {
			token = t.AccessToken
		}
	}

	emit(ProgressEvent{Type: "import_started", Message: "starting import of " + owner + "/" + repo})

	// Create the project up-front so the user can see a placeholder in their
	// dashboard while the workspace clones.
	projectID := projectIDFromRepo(owner, repo, im.Projects)
	ownerForProject := req.UserID
	if req.MakePublic {
		ownerForProject = "" // public — accessible by all authenticated users
	}
	now := time.Now().UTC()
	created, err := im.Projects.Create(domain.Project{
		ID:          projectID,
		Name:        repo,
		Description: "Imported from github.com/" + owner + "/" + repo,
		Status:      "importing",
		OwnerID:     ownerForProject,
		Spec: domain.ProductSpec{
			Idea: "Imported GitHub repository — continue finishing it through the Ironflyer gates.",
		},
		GitHub: &domain.GitHubLink{
			Owner:         owner,
			Repo:          repo,
			FullName:      owner + "/" + repo,
			DefaultBranch: orDefault(req.Branch, "main"),
			HTMLURL:       "https://github.com/" + owner + "/" + repo,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("create project: %w", err)
	}
	emit(ProgressEvent{Type: "project_created", ProjectID: created.ID, Message: "project record created"})

	// Spin up a workspace for the cloned tree.
	ws, err := im.createWorkspace(ctx, req.UserBearer, req.UserID, created.ID)
	if err != nil {
		im.markFailed(created.ID, "workspace create failed: "+err.Error())
		emit(ProgressEvent{Type: "failed", ProjectID: created.ID, Error: err.Error()})
		return ImportResult{}, fmt.Errorf("create workspace: %w", err)
	}
	emit(ProgressEvent{Type: "cloning", ProjectID: created.ID, WorkspaceID: ws, Message: "workspace ready, cloning repo"})

	// Trigger the git clone.
	if err := im.gitClone(ctx, req.UserBearer, ws, cloneURL, token, req.Branch, req.Subdir); err != nil {
		im.markFailed(created.ID, "git clone failed: "+err.Error())
		emit(ProgressEvent{Type: "failed", ProjectID: created.ID, WorkspaceID: ws, Error: err.Error()})
		return ImportResult{}, fmt.Errorf("git clone: %w", err)
	}
	emit(ProgressEvent{Type: "cloned", ProjectID: created.ID, WorkspaceID: ws, Message: "repo cloned"})

	// Stack detection.
	emit(ProgressEvent{Type: "detecting_stack", ProjectID: created.ID, WorkspaceID: ws})
	stack, warnings, sample, err := im.Detector.Detect(ctx, req.UserBearer, ws)
	if err != nil {
		// Detection failures are non-fatal — the project is still usable,
		// but stack metadata stays empty. Surface as a warning.
		warnings = append(warnings, "stack detection failed: "+err.Error())
	}
	stackCopy := stack
	emit(ProgressEvent{Type: "stack_detected", ProjectID: created.ID, WorkspaceID: ws, Stack: &stackCopy})
	for _, w := range warnings {
		emit(ProgressEvent{Type: "warning", ProjectID: created.ID, Warning: w})
	}

	// Persist the detection result onto the project.
	if _, err := im.Projects.Update(created.ID, func(p *domain.Project) {
		p.Spec.Stack = stack
		p.Files = sample
		p.Status = "ready"
	}); err != nil {
		emit(ProgressEvent{Type: "failed", ProjectID: created.ID, WorkspaceID: ws, Error: err.Error()})
		return ImportResult{}, fmt.Errorf("update project: %w", err)
	}

	result := ImportResult{
		ProjectID:   created.ID,
		WorkspaceID: ws,
		Stack:       stack,
		Files:       sample,
		Warnings:    warnings,
	}
	emit(ProgressEvent{Type: "ready", ProjectID: created.ID, WorkspaceID: ws, Stack: &stackCopy, Message: "import complete"})
	return result, nil
}

func (im *Importer) markFailed(projectID, msg string) {
	_, _ = im.Projects.Update(projectID, func(p *domain.Project) {
		p.Status = "failed"
		p.Description = p.Description + " — " + msg
	})
}

// ----------------------------------------------------------------------------
// HTTP helpers — talk to the runtime over its REST API. The runtime owns
// per-user authorisation via the Bearer it sees here.
// ----------------------------------------------------------------------------

type runtimeWorkspace struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	ProjectID string `json:"projectId,omitempty"`
	Status    string `json:"status"`
	Driver    string `json:"driver,omitempty"`
}

func (im *Importer) createWorkspace(ctx context.Context, bearer, userID, projectID string) (string, error) {
	body, _ := json.Marshal(map[string]string{"userId": userID, "projectId": projectID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		im.RuntimeURL+"/workspaces", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := im.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("runtime %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var ws runtimeWorkspace
	if err := json.Unmarshal(raw, &ws); err != nil {
		return "", fmt.Errorf("decode workspace: %w", err)
	}
	if ws.ID == "" {
		return "", errors.New("runtime returned workspace without id")
	}
	return ws.ID, nil
}

func (im *Importer) gitClone(ctx context.Context, bearer, ws, cloneURL, token, ref, subdir string) error {
	body, _ := json.Marshal(map[string]string{
		"cloneUrl": cloneURL,
		"token":    token,
		"ref":      ref,
		"subdir":   subdir,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		im.RuntimeURL+"/workspaces/"+ws+"/git-clone", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := im.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		bts, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("runtime %d: %s", resp.StatusCode, strings.TrimSpace(string(bts)))
	}
	return nil
}

// ----------------------------------------------------------------------------
// URL / id helpers
// ----------------------------------------------------------------------------

var (
	shorthandRE = regexp.MustCompile(`^[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+$`)
	idChars     = regexp.MustCompile(`[^a-z0-9-]+`)
)

// normaliseRepoURL accepts either a full https://github.com/<owner>/<repo>
// URL (with optional .git suffix or trailing slash) or the `owner/repo`
// shorthand. Anything else is rejected — we intentionally don't try to
// import non-HTTPS or non-github.com URLs from the public surface.
func normaliseRepoURL(in string) (cloneURL, owner, repo string, err error) {
	s := strings.TrimSpace(in)
	if s == "" {
		return "", "", "", errors.New("repoUrl required")
	}
	if shorthandRE.MatchString(s) {
		parts := strings.SplitN(s, "/", 2)
		owner, repo = parts[0], strings.TrimSuffix(parts[1], ".git")
		return "https://github.com/" + owner + "/" + repo, owner, repo, nil
	}
	u, perr := url.Parse(s)
	if perr != nil {
		return "", "", "", fmt.Errorf("invalid url: %w", perr)
	}
	if u.Scheme != "https" {
		return "", "", "", errors.New("repoUrl must use https://")
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", "", "", errors.New("only github.com URLs are supported")
	}
	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", errors.New("URL must include owner/repo")
	}
	owner, repo = parts[0], parts[1]
	return "https://github.com/" + owner + "/" + repo, owner, repo, nil
}

// projectIDFromRepo derives a slug from the repo name, falling back to a
// suffix if the slug collides with an existing project.
func projectIDFromRepo(owner, repo string, projects store.Store) string {
	base := slugify(repo)
	if base == "" {
		base = slugify(owner + "-" + repo)
	}
	if base == "" {
		base = "imported"
	}
	if _, err := projects.Get(base); err != nil {
		return base
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, err := projects.Get(candidate); err != nil {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = idChars.ReplaceAllString(s, "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
