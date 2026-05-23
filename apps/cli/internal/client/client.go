// Package client is a typed HTTP client for the Ironflyer orchestrator.
// It is the only file in the CLI that knows the orchestrator's URL paths
// and JSON shapes — every command package consumes this client, never
// http directly.
//
// Design notes:
//   - Auth header is "Authorization: Bearer <token>".
//   - The CLI is intentionally a pure HTTP consumer. We do NOT depend on
//     ironflyer/apps/orchestrator (separate Go module) — the shapes below
//     are minimal duplicates of the relevant orchestrator types.
//   - SSE is read line-by-line (no third-party SSE lib) since the
//     orchestrator's `event: <name>\ndata: <json>` frames are simple.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a thread-safe wrapper around net/http for the orchestrator.
// One per process is plenty.
type Client struct {
	Host  string
	Token string
	HTTP  *http.Client
}

// New constructs a Client. host should NOT have a trailing slash; we
// normalize it just in case.
func New(host, token string) *Client {
	host = strings.TrimRight(host, "/")
	if host == "" {
		host = "http://localhost:8080"
	}
	return &Client{
		Host:  host,
		Token: token,
		HTTP: &http.Client{
			// 30s is generous for ordinary REST. Streaming endpoints use
			// a different code path with no timeout.
			Timeout: 30 * time.Second,
		},
	}
}

// User mirrors the orchestrator's auth.User. Keep field names matching
// the JSON tags emitted by the server.
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	Plan      string `json:"plan,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// AuthResponse is the shape of /auth/login + /auth/signup.
type AuthResponse struct {
	User  User   `json:"user"`
	Token string `json:"token"`
}

// Project mirrors the relevant fields of domain.Project. We omit nested
// gates/files/etc — fetch the full record via GetProject when needed.
type Project struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Status      string  `json:"status"`
	OwnerID     string  `json:"ownerId,omitempty"`
	IsPublic    bool    `json:"isPublic,omitempty"`
	Spec        Spec    `json:"spec,omitempty"`
	Files       []File  `json:"files,omitempty"`
	Gates       map[string]GateState `json:"gates,omitempty"`
	CreatedAt   string  `json:"createdAt,omitempty"`
	UpdatedAt   string  `json:"updatedAt,omitempty"`
}

// Spec is a minimal mirror of domain.ProductSpec.
type Spec struct {
	Idea       string   `json:"idea,omitempty"`
	Goals      []string `json:"goals,omitempty"`
	Audience   string   `json:"audience,omitempty"`
}

// File mirrors domain.File for the show command.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Type    string `json:"type,omitempty"`
}

// GateState mirrors domain.GateState.
type GateState struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Patch is the projection of patch.Patch we surface to the CLI.
type Patch struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
	Summary   string `json:"summary,omitempty"`
	Author    string `json:"author,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// HealthResponse is the parsed body of /health and /healthz.
type HealthResponse struct {
	OK      bool   `json:"ok"`
	Service string `json:"service,omitempty"`
	Version string `json:"version,omitempty"`
}

// ErrAPI carries the HTTP status + the server's error message body.
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

// do is the shared request runner — builds the request, attaches the
// bearer if present, parses non-2xx into ErrAPI.
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Host+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ironflyer-cli")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &ErrAPI{Status: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	return resp, nil
}

// JSON runs the request, decodes the body into out (if non-nil), then
// closes the response.
func (c *Client) JSON(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ---- Auth ----

// Login calls POST /auth/login with email + password.
func (c *Client) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	var out AuthResponse
	if err := c.JSON(ctx, http.MethodPost, "/auth/login",
		map[string]string{"email": email, "password": password}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Me calls GET /auth/me.
func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	if err := c.JSON(ctx, http.MethodGet, "/auth/me", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ---- Projects ----

// ListProjects calls GET /projects.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var out []Project
	if err := c.JSON(ctx, http.MethodGet, "/projects", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateProject calls POST /projects.
func (c *Client) CreateProject(ctx context.Context, name, idea, description string) (*Project, error) {
	body := map[string]string{
		"name":        name,
		"idea":        idea,
		"description": description,
	}
	var out Project
	if err := c.JSON(ctx, http.MethodPost, "/projects", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetProject calls GET /projects/{id}.
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var out Project
	if err := c.JSON(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BulkDeleteProjects calls POST /projects/bulk-delete with the supplied
// ids list. The orchestrator's bulk-delete endpoint is the only
// project-delete endpoint that currently exists; for the CLI's
// `projects delete <id>` we wrap a single-id call here.
func (c *Client) BulkDeleteProjects(ctx context.Context, ids []string) error {
	return c.JSON(ctx, http.MethodPost, "/projects/bulk-delete",
		map[string]any{"ids": ids}, nil)
}

// ListPatches calls GET /projects/{id}/patches.
func (c *Client) ListPatches(ctx context.Context, projectID string) ([]Patch, error) {
	var out []Patch
	if err := c.JSON(ctx, http.MethodGet,
		"/projects/"+url.PathEscape(projectID)+"/patches", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ApplyPatch calls POST /patches/{id}/apply.
func (c *Client) ApplyPatch(ctx context.Context, patchID string) error {
	return c.JSON(ctx, http.MethodPost,
		"/patches/"+url.PathEscape(patchID)+"/apply", nil, nil)
}

// RunFinisher calls POST /projects/{id}/run. The response is the engine
// report — we treat the body as opaque JSON since the report shape is
// internal to the orchestrator.
func (c *Client) RunFinisher(ctx context.Context, projectID string) (json.RawMessage, error) {
	resp, err := c.do(ctx, http.MethodPost,
		"/projects/"+url.PathEscape(projectID)+"/run", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ---- Deploy + Export ----

// StartDeploy calls POST /projects/{id}/deploy and returns the deployment
// id + stream URL.
type DeployStarted struct {
	DeploymentID string `json:"deploymentId"`
	StreamURL    string `json:"streamURL"`
}

func (c *Client) StartDeploy(ctx context.Context, projectID, provider, region string, env map[string]string) (*DeployStarted, error) {
	body := map[string]any{"provider": provider, "region": region}
	if len(env) > 0 {
		body["env"] = env
	}
	var out DeployStarted
	if err := c.JSON(ctx, http.MethodPost,
		"/projects/"+url.PathEscape(projectID)+"/deploy", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ExportZip downloads the project as a zip and writes it to w. Returns
// the number of bytes copied.
func (c *Client) ExportZip(ctx context.Context, projectID string, w io.Writer) (int64, error) {
	resp, err := c.do(ctx, http.MethodPost,
		"/projects/"+url.PathEscape(projectID)+"/export/zip", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return io.Copy(w, resp.Body)
}

// ExportGitHubResult mirrors the orchestrator's deploy.ExportResult that
// gets returned from POST /projects/{id}/export/github. We only need the
// URL for the CLI's UX.
type ExportGitHubResult struct {
	RepoURL  string `json:"repoUrl,omitempty"`
	HTMLURL  string `json:"htmlUrl,omitempty"`
	FullName string `json:"fullName,omitempty"`
}

func (c *Client) ExportGitHub(ctx context.Context, projectID, repoName, description string, private bool) (*ExportGitHubResult, error) {
	body := map[string]any{
		"repoName":    repoName,
		"description": description,
		"private":     private,
	}
	var out ExportGitHubResult
	if err := c.JSON(ctx, http.MethodPost,
		"/projects/"+url.PathEscape(projectID)+"/export/github", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- Health ----

// Health hits GET /health on the orchestrator.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var h HealthResponse
	if err := c.JSON(ctx, http.MethodGet, "/health", nil, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// ---- Memory ----

// MemoryRecord mirrors orchestrator memory.Record. We duplicate the
// shape (rather than import the orchestrator module) to keep the CLI
// dependency-free of the server module.
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

// MemoryListResponse is the envelope returned by GET /memory.
type MemoryListResponse struct {
	Records []MemoryRecord `json:"records"`
	Count   int            `json:"count"`
}

// ListMemory calls GET /memory with the supplied query parameters.
// Empty strings / zero limits are omitted so the server picks defaults.
func (c *Client) ListMemory(ctx context.Context, params map[string]string) (*MemoryListResponse, error) {
	q := url.Values{}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	path := "/memory"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var out MemoryListResponse
	if err := c.JSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddMemory calls POST /memory with the supplied record. The server
// assigns id + createdAt and returns the stored row.
func (c *Client) AddMemory(ctx context.Context, rec MemoryRecord) (*MemoryRecord, error) {
	var out MemoryRecord
	if err := c.JSON(ctx, http.MethodPost, "/memory", rec, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteMemory calls DELETE /memory/{id}. Idempotent — unknown ids
// return 204 from the orchestrator.
func (c *Client) DeleteMemory(ctx context.Context, id string) error {
	return c.JSON(ctx, http.MethodDelete, "/memory/"+url.PathEscape(id), nil, nil)
}

// ---- Audit ----

// AuditEntry mirrors orchestrator audit.Entry.
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

// AuditListResponse is the envelope returned by GET /audit.
type AuditListResponse struct {
	Entries []AuditEntry `json:"entries"`
	Count   int          `json:"count"`
}

// AuditVerifyResponse is the body of GET /audit/verify.
type AuditVerifyResponse struct {
	Intact        bool `json:"intact"`
	FirstBadIndex int  `json:"firstBadIndex"`
}

// ListAudit calls GET /audit with the supplied query parameters.
func (c *Client) ListAudit(ctx context.Context, params map[string]string) (*AuditListResponse, error) {
	q := url.Values{}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	path := "/audit"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var out AuditListResponse
	if err := c.JSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerifyAudit calls GET /audit/verify. The response carries Intact +
// the index of the first broken entry (or -1 if intact).
func (c *Client) VerifyAudit(ctx context.Context) (*AuditVerifyResponse, error) {
	var out AuditVerifyResponse
	if err := c.JSON(ctx, http.MethodGet, "/audit/verify", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
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

// TelemetryAgentsResponse is the envelope returned by GET
// /telemetry/agents.
type TelemetryAgentsResponse struct {
	Calls []AgentCall `json:"calls"`
	Count int         `json:"count"`
}

// ListAgentTelemetry calls GET /telemetry/agents with the supplied
// filter parameters.
func (c *Client) ListAgentTelemetry(ctx context.Context, params map[string]string) (*TelemetryAgentsResponse, error) {
	q := url.Values{}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	path := "/telemetry/agents"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var out TelemetryAgentsResponse
	if err := c.JSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
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

// GetProjectGraph calls GET /projects/{id}/graph.
func (c *Client) GetProjectGraph(ctx context.Context, projectID string) (*ProjectGraph, error) {
	var out ProjectGraph
	if err := c.JSON(ctx, http.MethodGet,
		"/projects/"+url.PathEscape(projectID)+"/graph", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HealthAt hits GET /healthz on an arbitrary base URL — used by `status`
// to ping the runtime as well as the orchestrator.
func (c *Client) HealthAt(ctx context.Context, baseURL string) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(baseURL, "/")+"/healthz", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, &ErrAPI{Status: resp.StatusCode}
	}
	var h HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		// /healthz may return a non-JSON 200 in some environments; treat
		// a successful status code as healthy.
		return &HealthResponse{OK: true}, nil
	}
	return &h, nil
}

// ---- SSE streaming ----

// SSEEvent is one Server-Sent Events frame.
type SSEEvent struct {
	Event string
	Data  string
}

// StreamProjectEvents subscribes to GET /projects/{id}/stream and pushes
// every event into the returned channel until ctx is cancelled or the
// server closes the connection. Errors are surfaced on the error channel.
func (c *Client) StreamProjectEvents(ctx context.Context, projectID string) (<-chan SSEEvent, <-chan error) {
	return c.streamSSE(ctx, "/projects/"+url.PathEscape(projectID)+"/stream", nil)
}

// StreamDeployment subscribes to GET /deployments/{deploymentId}/stream.
func (c *Client) StreamDeployment(ctx context.Context, deploymentID string) (<-chan SSEEvent, <-chan error) {
	return c.streamSSE(ctx, "/deployments/"+url.PathEscape(deploymentID)+"/stream", nil)
}

// streamSSE opens a long-lived GET request with no client timeout and
// parses standard SSE frames (`event: name\ndata: ...\n\n`). Comment
// lines (`: keepalive`) are dropped silently.
func (c *Client) streamSSE(ctx context.Context, path string, query url.Values) (<-chan SSEEvent, <-chan error) {
	events := make(chan SSEEvent, 32)
	errs := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errs)
		full := c.Host + path
		if len(query) > 0 {
			full += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			errs <- err
			return
		}
		// SSE clients use Authorization the same way other REST calls do.
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		client := &http.Client{Timeout: 0} // streaming — no overall timeout
		resp, err := client.Do(req)
		if err != nil {
			errs <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			errs <- &ErrAPI{Status: resp.StatusCode, Body: strings.TrimSpace(string(b))}
			return
		}
		scanner := bufio.NewScanner(resp.Body)
		// 1MiB max line — SSE frames are tiny but model output can be
		// chunky.
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		var current SSEEvent
		flush := func() {
			if current.Event == "" && current.Data == "" {
				return
			}
			select {
			case events <- current:
			case <-ctx.Done():
			}
			current = SSEEvent{}
		}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				flush()
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue // comment / keepalive
			}
			if k, v, ok := strings.Cut(line, ":"); ok {
				v = strings.TrimPrefix(v, " ")
				switch strings.TrimSpace(k) {
				case "event":
					current.Event = v
				case "data":
					if current.Data == "" {
						current.Data = v
					} else {
						current.Data += "\n" + v
					}
				}
			}
		}
		flush()
		if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
			errs <- err
		}
	}()
	return events, errs
}
