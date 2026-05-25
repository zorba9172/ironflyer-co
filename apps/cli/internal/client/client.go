// Package client is a typed GraphQL client for the Ironflyer
// orchestrator. It is the only file in the CLI that knows where the
// orchestrator lives — every command package consumes this client, never
// http or graphql directly.
//
// Design notes:
//   - All transport goes through `apps/cli/internal/gql`, the genqlient-
//     generated package. We add `Authorization: Bearer <token>` via a
//     custom http.RoundTripper.
//   - The legacy /healthz check stays REST: GraphQL has a `ping` query but
//     it requires reaching /graphql, which doesn't tell us anything new
//     about runtime liveness, and `status` needs to ping the runtime
//     anyway (which doesn't speak GraphQL). See docs/CLI_GRAPHQL_GAPS.md.
//   - Subscriptions reuse genqlient's built-in WebSocket client wired up
//     with github.com/coder/websocket (already vendored in apps/runtime).
//     The subprotocol is `graphql-transport-ws`. We translate the typed
//     subscription channel into the legacy SSEEvent shape so command
//     code (`run`, `logs`, `deploy`) can stay unchanged.
//
// The exported types below mirror the *previous* REST shapes so command
// packages compile without edits. Each field is populated from the
// equivalent GraphQL query; nullable fields default to the Go zero value.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/coder/websocket"

	"ironflyer/apps/cli/internal/gql"
)

// Client is the high-level CLI client. One per process is plenty.
type Client struct {
	Host  string
	Token string
	HTTP  *http.Client

	gql  graphql.Client
	once map[string]struct{} // future flag bag; reserved
}

// New constructs a Client. host should NOT have a trailing slash; we
// normalize it just in case.
func New(host, token string) *Client {
	host = strings.TrimRight(host, "/")
	if host == "" {
		host = "http://localhost:8080"
	}
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authRoundTripper{
			base:  http.DefaultTransport,
			token: token,
		},
	}
	endpoint := host + "/graphql"
	return &Client{
		Host:  host,
		Token: token,
		HTTP:  httpClient,
		gql:   graphql.NewClient(endpoint, httpClient),
	}
}

// authRoundTripper attaches a Bearer token + standard headers to every
// outbound request.
type authRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (a *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	if a.token != "" {
		r.Header.Set("Authorization", "Bearer "+a.token)
	}
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", "ironflyer-cli")
	}
	return a.base.RoundTrip(r)
}

// ErrAPI carries a non-2xx HTTP status + the server's error body. We
// keep this for backward compatibility — callers that inspect the type
// still work.
type ErrAPI struct {
	Status int
	Body   string
}

func (e *ErrAPI) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("http %d", e.Status)
	}
	return fmt.Sprintf("http %d: %s", e.Status, e.Body)
}

// ---- Shared models (kept for command-package compatibility) ----

// User mirrors the orchestrator's User. Field names match the prior REST
// JSON tags so command packages need no changes.
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	Plan      string `json:"plan,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// AuthResponse is what the old /auth/login returned. We populate it from
// the GraphQL signIn mutation.
type AuthResponse struct {
	User  User   `json:"user"`
	Token string `json:"token"`
}

// Project is the trimmed projection the CLI displays. We expose Spec.Idea
// to keep the `show` command working even though GraphQL flattens `idea`
// onto the Project node.
type Project struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Status      string               `json:"status"`
	OwnerID     string               `json:"ownerId,omitempty"`
	IsPublic    bool                 `json:"isPublic,omitempty"`
	Spec        Spec                 `json:"spec,omitempty"`
	Files       []File               `json:"files,omitempty"`
	Gates       map[string]GateState `json:"gates,omitempty"`
	CreatedAt   string               `json:"createdAt,omitempty"`
	UpdatedAt   string               `json:"updatedAt,omitempty"`
}

// Spec is a minimal mirror of the spec the CLI prints.
type Spec struct {
	Idea     string   `json:"idea,omitempty"`
	Goals    []string `json:"goals,omitempty"`
	Audience string   `json:"audience,omitempty"`
}

// File mirrors domain.File for the show command (currently unused by
// commands; kept for future zip/diff inspection commands).
type File struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Type    string `json:"type,omitempty"`
}

// GateState mirrors domain.GateState. The GraphQL schema does not expose
// the gate map directly on Project today; this struct stays in the API
// so the `show` command compiles. Gates is populated as an empty map.
type GateState struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Patch is the projection of patch.Patch surfaced to the CLI.
type Patch struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
	Summary   string `json:"summary,omitempty"`
	Author    string `json:"author,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// HealthResponse is the parsed body of /health and /healthz (REST).
type HealthResponse struct {
	OK      bool   `json:"ok"`
	Service string `json:"service,omitempty"`
	Version string `json:"version,omitempty"`
}

// ---- Auth ----

// Login calls the signIn mutation.
func (c *Client) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	resp, err := gql.SignIn(ctx, c.gql, email, password)
	if err != nil {
		return nil, err
	}
	return &AuthResponse{
		Token: resp.SignIn.Token,
		User: User{
			ID:    resp.SignIn.User.Id,
			Email: resp.SignIn.User.Email,
			Name:  resp.SignIn.User.Name,
			Plan:  resp.SignIn.User.Plan,
		},
	}, nil
}

// Me calls the me query.
func (c *Client) Me(ctx context.Context) (*User, error) {
	resp, err := gql.Me(ctx, c.gql)
	if err != nil {
		return nil, err
	}
	if resp.Me.Id == "" {
		return nil, fmt.Errorf("not authenticated")
	}
	return &User{
		ID:        resp.Me.Id,
		Email:     resp.Me.Email,
		Name:      resp.Me.Name,
		Plan:      resp.Me.Plan,
		CreatedAt: resp.Me.CreatedAt.Format(time.RFC3339),
	}, nil
}

// ---- Projects ----

// ListProjects calls the projects query.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	resp, err := gql.ListProjects(ctx, c.gql)
	if err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(resp.Projects))
	for _, p := range resp.Projects {
		out = append(out, Project{
			ID:          p.Id,
			Name:        p.Name,
			Description: p.Description,
			Status:      p.Status,
			IsPublic:    p.IsPublic,
			UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// CreateProject calls the createProject mutation. `idea` is optional in
// the schema but the CLI requires it at command level.
func (c *Client) CreateProject(ctx context.Context, name, idea, description string) (*Project, error) {
	resp, err := gql.CreateProject(ctx, c.gql, name, description, idea)
	if err != nil {
		return nil, err
	}
	p := resp.CreateProject
	return &Project{
		ID:          p.Id,
		Name:        p.Name,
		Description: p.Description,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
	}, nil
}

// GetProject calls the project query.
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	resp, err := gql.GetProject(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	if resp.Project.Id == "" {
		return nil, &ErrAPI{Status: http.StatusNotFound, Body: "project not found"}
	}
	p := resp.Project
	return &Project{
		ID:          p.Id,
		Name:        p.Name,
		Description: p.Description,
		Status:      p.Status,
		OwnerID:     p.OwnerId,
		IsPublic:    p.IsPublic,
		Spec:        Spec{Idea: p.Idea},
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
		// Gates intentionally empty: schema doesn't expose the gate map on
		// the Project type yet. See docs/CLI_GRAPHQL_GAPS.md.
	}, nil
}

// BulkDeleteProjects calls the bulkDeleteProjects mutation.
func (c *Client) BulkDeleteProjects(ctx context.Context, ids []string) error {
	resp, err := gql.BulkDeleteProjects(ctx, c.gql, ids)
	if err != nil {
		return err
	}
	if !resp.BulkDeleteProjects.Ok {
		msg := resp.BulkDeleteProjects.Message
		if msg == "" {
			msg = "bulk delete failed"
		}
		return errors.New(msg)
	}
	return nil
}

// ListPatches calls the patches query.
func (c *Client) ListPatches(ctx context.Context, projectID string) ([]Patch, error) {
	resp, err := gql.ListPatches(ctx, c.gql, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]Patch, 0, len(resp.Patches))
	for _, p := range resp.Patches {
		out = append(out, Patch{
			ID:        p.Id,
			ProjectID: p.ProjectId,
			Title:     p.Title,
			Summary:   p.Summary,
			Author:    p.Author,
			Status:    string(p.Status),
			CreatedAt: p.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// ApplyPatch calls the applyPatch mutation.
func (c *Client) ApplyPatch(ctx context.Context, patchID string) error {
	_, err := gql.ApplyPatch(ctx, c.gql, patchID)
	return err
}

// RunFinisher calls the runFinisher mutation. The server returns a JSON
// blob (the engine report); we return it verbatim so callers that print
// `--json` keep working.
func (c *Client) RunFinisher(ctx context.Context, projectID string) (json.RawMessage, error) {
	resp, err := gql.RunFinisher(ctx, c.gql, projectID)
	if err != nil {
		return nil, err
	}
	return resp.RunFinisher, nil
}

// ---- Deploy + Export ----

// DeployStarted is what the CLI's `deploy` command holds onto after
// kicking a deployment off — it needs the id to subscribe to the live
// stream.
type DeployStarted struct {
	DeploymentID string `json:"deploymentId"`
	StreamURL    string `json:"streamURL"`
}

// StartDeploy calls the startDeploy mutation. provider+region+env are
// rolled into a StartDeployInput target+env JSON envelope — the
// orchestrator's deploy engine still understands the (provider,region)
// pair via the legacy REST handler, so we keep both: provider goes into
// `target`, and region+env are passed through the JSON env blob.
func (c *Client) StartDeploy(ctx context.Context, projectID, provider, region string, env map[string]string) (*DeployStarted, error) {
	// Combine region into env so the orchestrator can read it from a
	// single field. Region is namespaced under __region__ to avoid
	// colliding with a user env var of the same name.
	combined := map[string]string{}
	for k, v := range env {
		combined[k] = v
	}
	if region != "" {
		combined["__region__"] = region
	}
	var envJSON json.RawMessage
	if len(combined) > 0 {
		b, err := json.Marshal(combined)
		if err != nil {
			return nil, err
		}
		envJSON = b
	}
	resp, err := gql.StartDeploy(ctx, c.gql, projectID, provider, "", envJSON)
	if err != nil {
		return nil, err
	}
	return &DeployStarted{
		DeploymentID: resp.StartDeploy.Id,
	}, nil
}

// ExportZip downloads the project as a zip and writes it to w. The
// GraphQL surface returns a *signed URL* (`exportZipUrl`) rather than the
// archive bytes; we GET the URL and stream it through. Returns the byte
// count copied.
func (c *Client) ExportZip(ctx context.Context, projectID string, w io.Writer) (int64, error) {
	resp, err := gql.ExportZipUrl(ctx, c.gql, projectID)
	if err != nil {
		return 0, err
	}
	if resp.ExportZipUrl == "" {
		return 0, errors.New("orchestrator returned an empty zip URL")
	}
	zipURL := resp.ExportZipUrl
	// Relative URL? Resolve against the orchestrator host.
	if strings.HasPrefix(zipURL, "/") {
		zipURL = c.Host + zipURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zipURL, nil)
	if err != nil {
		return 0, err
	}
	httpResp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(httpResp.Body, 4096))
		return 0, &ErrAPI{Status: httpResp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	return io.Copy(w, httpResp.Body)
}

// ExportGitHubResult is what the legacy /export/github returned. We
// reconstruct as much as the GraphQL exportGithub mutation provides.
type ExportGitHubResult struct {
	RepoURL  string `json:"repoUrl,omitempty"`
	HTMLURL  string `json:"htmlUrl,omitempty"`
	FullName string `json:"fullName,omitempty"`
}

// ExportGitHub calls the exportGithub mutation. The GraphQL schema takes
// (owner, repo) as separate args; the CLI's existing --repo-name flag is
// split heuristically on "/", with the calling user's GitHub login as
// the implicit owner when no slash is supplied.
func (c *Client) ExportGitHub(ctx context.Context, projectID, repoName, description string, private bool) (*ExportGitHubResult, error) {
	owner, repo := "", repoName
	if idx := strings.Index(repoName, "/"); idx > 0 {
		owner = repoName[:idx]
		repo = repoName[idx+1:]
	}
	if owner == "" {
		// Best-effort: fetch the authed user and use their id as owner.
		// This matches the orchestrator's behavior when no owner is
		// supplied via REST.
		if me, err := c.Me(ctx); err == nil && me != nil {
			owner = me.Email
			if at := strings.Index(owner, "@"); at > 0 {
				owner = owner[:at]
			}
		}
	}
	resp, err := gql.ExportGithub(ctx, c.gql, projectID, owner, repo)
	if err != nil {
		return nil, err
	}
	if !resp.ExportGithub.Ok {
		msg := resp.ExportGithub.Message
		if msg == "" {
			msg = "github export failed"
		}
		return nil, errors.New(msg)
	}
	fullName := owner + "/" + repo
	return &ExportGitHubResult{
		FullName: fullName,
		HTMLURL:  "https://github.com/" + fullName,
		RepoURL:  "https://github.com/" + fullName,
	}, nil
}

// ---- Health (REST — no equivalent GraphQL surface) ----

// Health hits GET /health on the orchestrator. Kept as REST because
// /health is intentionally outside the GraphQL surface (it must work
// even when the GraphQL stack is unhealthy).
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	return c.HealthAt(ctx, c.Host)
}

// HealthAt hits GET /healthz on an arbitrary base URL — used by `status`
// to ping the runtime as well as the orchestrator.
func (c *Client) HealthAt(ctx context.Context, baseURL string) (*HealthResponse, error) {
	base := strings.TrimRight(baseURL, "/")
	// Try /healthz first; if it doesn't exist fall back to /health.
	for _, path := range []string{"/healthz", "/health"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, &ErrAPI{Status: resp.StatusCode}
		}
		var h HealthResponse
		if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
			// Non-JSON 200 → assume healthy.
			return &HealthResponse{OK: true}, nil
		}
		if !h.OK && resp.StatusCode == http.StatusOK {
			h.OK = true
		}
		return &h, nil
	}
	return nil, &ErrAPI{Status: http.StatusNotFound}
}

// ---- Memory ----

// MemoryRecord mirrors orchestrator memory.Record.
type MemoryRecord struct {
	ID         string   `json:"id,omitempty"`
	Kind       string   `json:"kind"`
	ProjectID  string   `json:"projectId,omitempty"`
	UserID     string   `json:"userId,omitempty"`
	StoryID    string   `json:"storyId,omitempty"`
	GateName   string   `json:"gateName,omitempty"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Tags       []string `json:"tags,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	CreatedAt  string   `json:"createdAt,omitempty"`
}

// MemoryListResponse is the envelope the CLI's `memory list` consumes.
type MemoryListResponse struct {
	Records []MemoryRecord `json:"records"`
	Count   int            `json:"count"`
}

// ListMemory calls the memory query. The legacy params map is translated
// into a MemoryQueryInput.
func (c *Client) ListMemory(ctx context.Context, params map[string]string) (*MemoryListResponse, error) {
	q := gql.MemoryQueryInput{}
	if v := params["kind"]; v != "" {
		q.Kind = gql.MemoryKind(strings.ToUpper(v))
	}
	if v := params["projectId"]; v != "" {
		q.ProjectId = v
	}
	if v := params["userId"]; v != "" {
		q.UserId = v
	}
	if v := params["tag"]; v != "" {
		q.Tag = v
	}
	if v := params["q"]; v != "" {
		q.Q = v
	}
	if v := params["limit"]; v != "" {
		if n, err := parseInt(v); err == nil {
			q.Limit = n
		}
	}
	resp, err := gql.ListMemory(ctx, c.gql, q)
	if err != nil {
		return nil, err
	}
	out := MemoryListResponse{
		Records: make([]MemoryRecord, 0, len(resp.Memory)),
		Count:   len(resp.Memory),
	}
	for _, r := range resp.Memory {
		out.Records = append(out.Records, MemoryRecord{
			ID:        r.Id,
			Kind:      strings.ToLower(string(r.Kind)),
			UserID:    r.UserId,
			ProjectID: r.ProjectId,
			StoryID:   r.StoryId,
			GateName:  r.GateName,
			Title:     r.Title,
			Body:      r.Body,
			Tags:      r.Tags,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &out, nil
}

// AddMemory calls the addMemory mutation.
func (c *Client) AddMemory(ctx context.Context, rec MemoryRecord) (*MemoryRecord, error) {
	input := gql.AddMemoryInput{
		Kind:      gql.MemoryKind(strings.ToUpper(rec.Kind)),
		ProjectId: rec.ProjectID,
		StoryId:   rec.StoryID,
		GateName:  rec.GateName,
		Title:     rec.Title,
		Body:      rec.Body,
		Tags:      rec.Tags,
	}
	resp, err := gql.AddMemory(ctx, c.gql, input)
	if err != nil {
		return nil, err
	}
	r := resp.AddMemory
	return &MemoryRecord{
		ID:        r.Id,
		Kind:      strings.ToLower(string(r.Kind)),
		UserID:    r.UserId,
		ProjectID: r.ProjectId,
		GateName:  r.GateName,
		Title:     r.Title,
		Body:      r.Body,
		Tags:      r.Tags,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}, nil
}

// DeleteMemory calls the deleteMemory mutation. Idempotent — the
// orchestrator returns ok=true even for unknown ids.
func (c *Client) DeleteMemory(ctx context.Context, id string) error {
	resp, err := gql.DeleteMemory(ctx, c.gql, id)
	if err != nil {
		return err
	}
	if !resp.DeleteMemory.Ok {
		msg := resp.DeleteMemory.Message
		if msg == "" {
			msg = "delete failed"
		}
		return errors.New(msg)
	}
	return nil
}

// ---- Audit ----

// AuditEntry mirrors orchestrator audit.Entry, in the legacy REST shape
// expected by `ironflyer audit`. The GraphQL schema stores the chain in
// a single hash field; we expose it as ContentHash for backward compat.
type AuditEntry struct {
	ID          string         `json:"id"`
	Action      string         `json:"action"`
	Outcome     string         `json:"outcome"`
	UserID      string         `json:"userId,omitempty"`
	ProjectID   string         `json:"projectId,omitempty"`
	StoryID     string         `json:"storyId,omitempty"`
	GateName    string         `json:"gateName,omitempty"`
	AgentRole   string         `json:"agentRole,omitempty"`
	Summary     string         `json:"summary"`
	InputHash   string         `json:"inputHash,omitempty"`
	OutputHash  string         `json:"outputHash,omitempty"`
	Attrs       map[string]any `json:"attrs,omitempty"`
	CreatedAt   string         `json:"createdAt"`
	PrevHash    string         `json:"prevHash,omitempty"`
	ContentHash string         `json:"contentHash"`
}

// AuditListResponse is the envelope `audit list` consumes.
type AuditListResponse struct {
	Entries []AuditEntry `json:"entries"`
	Count   int          `json:"count"`
}

// AuditVerifyResponse is what `audit verify` consumes.
type AuditVerifyResponse struct {
	Intact        bool `json:"intact"`
	FirstBadIndex int  `json:"firstBadIndex"`
}

// ListAudit calls the audit query.
func (c *Client) ListAudit(ctx context.Context, params map[string]string) (*AuditListResponse, error) {
	q := gql.AuditQueryInput{}
	if v := params["userId"]; v != "" {
		q.UserId = v
	}
	if v := params["projectId"]; v != "" {
		q.ProjectId = v
	}
	if v := params["action"]; v != "" {
		q.Action = v
	}
	if v := params["outcome"]; v != "" {
		q.Outcome = gql.AuditOutcome(strings.ToUpper(v))
	}
	if v := params["since"]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.Since = t
		}
	}
	if v := params["until"]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.Until = t
		}
	}
	if v := params["limit"]; v != "" {
		if n, err := parseInt(v); err == nil {
			q.Limit = n
		}
	}
	resp, err := gql.ListAudit(ctx, c.gql, q)
	if err != nil {
		return nil, err
	}
	out := AuditListResponse{
		Entries: make([]AuditEntry, 0, len(resp.Audit)),
		Count:   len(resp.Audit),
	}
	for _, e := range resp.Audit {
		entry := AuditEntry{
			ID:          e.Id,
			Action:      e.Action,
			Outcome:     strings.ToLower(string(e.Outcome)),
			UserID:      e.UserId,
			ProjectID:   e.ProjectId,
			CreatedAt:   e.Ts.Format(time.RFC3339),
			ContentHash: e.Hash,
			PrevHash:    e.PrevHash,
		}
		// Hydrate the summary / attrs from the JSON payload when present.
		if len(e.Payload) > 0 {
			var payload map[string]any
			if json.Unmarshal(e.Payload, &payload) == nil {
				if s, ok := payload["summary"].(string); ok {
					entry.Summary = s
				}
				if s, ok := payload["storyId"].(string); ok {
					entry.StoryID = s
				}
				if s, ok := payload["gateName"].(string); ok {
					entry.GateName = s
				}
				if s, ok := payload["agentRole"].(string); ok {
					entry.AgentRole = s
				}
				entry.Attrs = payload
			}
		}
		out.Entries = append(out.Entries, entry)
	}
	return &out, nil
}

// VerifyAudit calls the verifyAudit query.
func (c *Client) VerifyAudit(ctx context.Context) (*AuditVerifyResponse, error) {
	resp, err := gql.VerifyAudit(ctx, c.gql)
	if err != nil {
		return nil, err
	}
	return &AuditVerifyResponse{
		Intact:        resp.VerifyAudit.Intact,
		FirstBadIndex: resp.VerifyAudit.FirstBadIndex,
	}, nil
}

// ---- Telemetry ----

// AgentCall mirrors orchestrator providers.AgentCall — one structured
// row of per-call telemetry.
type AgentCall struct {
	UserID          string   `json:"userId"`
	ProjectID       string   `json:"projectId,omitempty"`
	Role            string   `json:"role,omitempty"`
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	Capabilities    []string `json:"capabilities,omitempty"`
	InputTokens     int      `json:"inputTokens"`
	OutputTokens    int      `json:"outputTokens"`
	CacheReadTokens int      `json:"cacheReadTokens,omitempty"`
	CacheNewTokens  int      `json:"cacheNewTokens,omitempty"`
	CostUSD         float64  `json:"costUSD"`
	DurationMS      int64    `json:"durationMs"`
	StartedAt       string   `json:"startedAt"`
	Error           string   `json:"error,omitempty"`
}

// TelemetryAgentsResponse is the envelope `telemetry agents` consumes.
type TelemetryAgentsResponse struct {
	Calls []AgentCall `json:"calls"`
	Count int         `json:"count"`
}

// ListAgentTelemetry calls the agentTelemetry query.
func (c *Client) ListAgentTelemetry(ctx context.Context, params map[string]string) (*TelemetryAgentsResponse, error) {
	var limit int
	if v := params["limit"]; v != "" {
		if n, err := parseInt(v); err == nil {
			limit = n
		}
	}
	resp, err := gql.AgentTelemetry(ctx, c.gql, limit, params["role"], params["provider"], params["model"])
	if err != nil {
		return nil, err
	}
	out := TelemetryAgentsResponse{
		Calls: make([]AgentCall, 0, len(resp.AgentTelemetry)),
		Count: len(resp.AgentTelemetry),
	}
	for _, r := range resp.AgentTelemetry {
		cost, _ := r.CostUsd.Float64()
		out.Calls = append(out.Calls, AgentCall{
			Role:         r.Role,
			Provider:     r.Provider,
			Model:        r.Model,
			InputTokens:  r.PromptTokens,
			OutputTokens: r.CompletionTokens,
			CostUSD:      cost,
			DurationMS:   int64(r.DurationMs),
			StartedAt:    r.Ts.Format(time.RFC3339),
			Error:        r.Error,
		})
	}
	return &out, nil
}

// ---- Project graph ----

// GraphNode mirrors projectgraph.Node.
type GraphNode struct {
	Path        string   `json:"path"`
	Language    string   `json:"language"`
	Exports     []string `json:"exports,omitempty"`
	SymbolCount int      `json:"symbolCount,omitempty"`
}

// GraphEdge mirrors projectgraph.Edge.
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Raw  string `json:"raw"`
}

// ProjectGraph mirrors projectgraph.Graph.
type ProjectGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GetProjectGraph calls the projectGraph query.
func (c *Client) GetProjectGraph(ctx context.Context, projectID string) (*ProjectGraph, error) {
	resp, err := gql.GetProjectGraph(ctx, c.gql, projectID)
	if err != nil {
		return nil, err
	}
	out := ProjectGraph{
		Nodes: make([]GraphNode, 0, len(resp.ProjectGraph.Nodes)),
		Edges: make([]GraphEdge, 0, len(resp.ProjectGraph.Edges)),
	}
	for _, n := range resp.ProjectGraph.Nodes {
		out.Nodes = append(out.Nodes, GraphNode{
			Path:     n.Path,
			Language: n.Language,
		})
	}
	for _, e := range resp.ProjectGraph.Edges {
		out.Edges = append(out.Edges, GraphEdge{
			From: e.From,
			To:   e.To,
			Raw:  e.Kind,
		})
	}
	return &out, nil
}

// ---- Subscriptions ----

// SSEEvent is the legacy shape that command code (`run`, `logs`,
// `deploy`) consumes. We keep the type name so callers don't change.
// `Data` is JSON-encoded so existing renderers can unmarshal as before.
type SSEEvent struct {
	Event string
	Data  string
}

// StreamProjectEvents subscribes to the runProject GraphQL subscription
// and pushes one SSEEvent per server event into the returned channel.
func (c *Client) StreamProjectEvents(ctx context.Context, projectID string) (<-chan SSEEvent, <-chan error) {
	events := make(chan SSEEvent, 32)
	errs := make(chan error, 4)
	go func() {
		defer close(events)
		defer close(errs)
		wsClient, closer, err := c.newWSClient(ctx)
		if err != nil {
			errs <- err
			return
		}
		defer closer()

		dataCh, _, err := gql.RunProject(ctx, wsClient, projectID)
		if err != nil {
			errs <- err
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-dataCh:
				if !ok {
					return
				}
				if msg.Errors != nil {
					errs <- msg.Errors
					continue
				}
				if msg.Data == nil {
					continue
				}
				ev, data := flattenRunEvent(msg.Data.RunProject)
				select {
				case events <- SSEEvent{Event: ev, Data: data}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return events, errs
}

// StreamDeployment subscribes to the deployStream GraphQL subscription.
func (c *Client) StreamDeployment(ctx context.Context, deploymentID string) (<-chan SSEEvent, <-chan error) {
	events := make(chan SSEEvent, 32)
	errs := make(chan error, 4)
	go func() {
		defer close(events)
		defer close(errs)
		wsClient, closer, err := c.newWSClient(ctx)
		if err != nil {
			errs <- err
			return
		}
		defer closer()

		dataCh, _, err := gql.DeployStream(ctx, wsClient, deploymentID)
		if err != nil {
			errs <- err
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-dataCh:
				if !ok {
					return
				}
				if msg.Errors != nil {
					errs <- msg.Errors
					continue
				}
				if msg.Data == nil {
					continue
				}
				ev, data := flattenDeployEvent(msg.Data.DeployStream)
				select {
				case events <- SSEEvent{Event: ev, Data: data}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return events, errs
}

// flattenRunEvent collapses a typed runProject envelope into the JSON
// shape command code expects (a flat object with `role`, `message`,
// `gate`, `status` keys).
func flattenRunEvent(d gql.RunProjectRunProjectRunEvent) (string, string) {
	switch v := d.(type) {
	case *gql.RunProjectRunProjectRunExecutionEvent:
		// payload is already JSON; pass it through.
		return "execution", string(v.Payload)
	case *gql.RunProjectRunProjectRunGateEvent:
		b, _ := json.Marshal(map[string]any{
			"role":    "gate",
			"gate":    v.Gate,
			"status":  v.Status,
			"message": v.GateMessage,
		})
		return "gate", string(b)
	case *gql.RunProjectRunProjectRunDoneEvent:
		b, _ := json.Marshal(map[string]any{
			"role":    "done",
			"ok":      v.Ok,
			"summary": v.Summary,
		})
		return "done", string(b)
	case *gql.RunProjectRunProjectRunErrorEvent:
		b, _ := json.Marshal(map[string]any{
			"role":    "error",
			"code":    v.Code,
			"message": v.ErrorMessage,
		})
		return "error", string(b)
	}
	return "", ""
}

// flattenDeployEvent collapses a typed deployStream envelope.
func flattenDeployEvent(d gql.DeployStreamDeployStreamDeployEvent) (string, string) {
	switch v := d.(type) {
	case *gql.DeployStreamDeployStreamDeployStateEvent:
		b, _ := json.Marshal(map[string]any{
			"kind":     "state",
			"status":   string(v.Status),
			"deployId": v.DeployId,
		})
		return "state", string(b)
	case *gql.DeployStreamDeployStreamDeployLogEvent:
		b, _ := json.Marshal(map[string]any{
			"kind":  "log",
			"level": v.Line.Level,
			"line":  v.Line.Message,
		})
		return "log", string(b)
	case *gql.DeployStreamDeployStreamDeployFinishedEvent:
		b, _ := json.Marshal(map[string]any{
			"kind":   "deployed",
			"status": string(v.Status),
			"url":    v.Url,
		})
		return "finished", string(b)
	case *gql.DeployStreamDeployStreamDeployErrorEvent:
		b, _ := json.Marshal(map[string]any{
			"kind":    "failed",
			"code":    v.Code,
			"error":   v.Message,
		})
		return "error", string(b)
	}
	return "", ""
}

// newWSClient opens a websocket connection to the orchestrator's
// /graphql endpoint, performs the graphql-transport-ws handshake (via
// genqlient), and returns the underlying client plus a Close function.
func (c *Client) newWSClient(ctx context.Context) (graphql.WebSocketClient, func() error, error) {
	wsURL := wsEndpoint(c.Host)
	connParams := map[string]interface{}{}
	if c.Token != "" {
		connParams["authorization"] = "Bearer " + c.Token
	}
	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", "graphql-transport-ws")
	wsClient := graphql.NewClientUsingWebSocket(wsURL,
		coderDialer{token: c.Token},
		graphql.WithConnectionParams(connParams),
		graphql.WithWebsocketHeader(header),
	)
	errCh, err := wsClient.Start(ctx)
	if err != nil {
		return nil, func() error { return nil }, err
	}
	// Surface async ws errors via the per-stream error channel by
	// asking the caller to drain errCh; here we just guard against
	// the buffered channel filling.
	go func() {
		for range errCh {
			// errors are reported per-subscription via msg.Errors;
			// we drop transport-level noise to avoid leaking goroutines.
		}
	}()
	return wsClient, wsClient.Close, nil
}

// coderDialer adapts github.com/coder/websocket to genqlient's Dialer
// interface (which mirrors gorilla/websocket's shape).
type coderDialer struct {
	token string
}

func (d coderDialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (graphql.WSConn, error) {
	opts := &websocket.DialOptions{
		HTTPHeader:   requestHeader.Clone(),
		Subprotocols: []string{"graphql-transport-ws"},
	}
	if d.token != "" && opts.HTTPHeader.Get("Authorization") == "" {
		opts.HTTPHeader.Set("Authorization", "Bearer "+d.token)
	}
	conn, _, err := websocket.Dial(ctx, urlStr, opts)
	if err != nil {
		return nil, err
	}
	// genqlient won't enforce read limits — bump the default so big
	// engine payloads don't trip the protocol error.
	conn.SetReadLimit(16 * 1024 * 1024)
	return &coderConn{conn: conn, ctx: ctx}, nil
}

// coderConn adapts a coder/websocket *Conn into the genqlient WSConn
// interface used to send/receive text frames.
type coderConn struct {
	conn *websocket.Conn
	ctx  context.Context
}

func (c *coderConn) Close() error {
	return c.conn.Close(websocket.StatusNormalClosure, "client closing")
}

func (c *coderConn) WriteMessage(messageType int, data []byte) error {
	mt := websocket.MessageText
	if messageType == 2 { // gorilla.BinaryMessage
		mt = websocket.MessageBinary
	}
	return c.conn.Write(c.ctx, mt, data)
}

func (c *coderConn) ReadMessage() (int, []byte, error) {
	mt, data, err := c.conn.Read(c.ctx)
	if err != nil {
		return 0, nil, err
	}
	gorillaType := 1 // gorilla.TextMessage
	if mt == websocket.MessageBinary {
		gorillaType = 2
	}
	return gorillaType, data, nil
}

// wsEndpoint maps http(s)://host[:port] to ws(s)://host[:port]/graphql.
func wsEndpoint(host string) string {
	u, err := url.Parse(host)
	if err != nil {
		return "ws://localhost:8080/graphql"
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = "/graphql"
	return u.String()
}

// parseInt is a tiny strconv.Atoi shim that returns 0 on errors so the
// call sites can ignore them. We accept negative values; the server
// clamps.
func parseInt(s string) (int, error) {
	var n int
	var sign = 1
	if len(s) > 0 && s[0] == '-' {
		sign = -1
		s = s[1:]
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not an int: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n * sign, nil
}
