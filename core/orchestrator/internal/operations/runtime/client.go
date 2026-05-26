// Package runtime is a Go client for the Ironflyer workspace runtime
// service. The orchestrator uses it to drive real build/test commands from
// inside finisher gates without owning a workspace directly.
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// Client talks to the runtime over HTTP. Construct with New; reuse across
// calls to share the HTTP transport. A nil Client means "runtime not
// configured" — Enabled() reports that.
//
// PodEndpointFor + the embedded PodResolver are how the orchestrator
// handles workspace portability: every PTY/WebSocket request must hit
// the specific pod that currently owns the workspace (the container
// can't be reverse-proxied across the pod boundary mid-stream). The
// resolver maps a workspace ID to the per-pod DNS name exposed by the
// runtime's headless Service.
type Client struct {
	BaseURL string
	HTTP    *http.Client

	// PodResolver, when set, returns the per-pod DNS endpoint that
	// currently owns a workspace. Nil means "not portable yet" — every
	// call routes through BaseURL (Service load balancer).
	PodResolver PodResolver

	// HeadlessSvc is the name of the headless Service exposing per-pod
	// DNS (defaults to "runtime-headless"). Joined with the pod name and
	// namespace to build "runtime-0.runtime-headless.<ns>.svc.cluster.local".
	HeadlessSvc string
	// Namespace is the k8s namespace the runtime StatefulSet lives in.
	Namespace string
	// Port is the HTTP port the runtime exposes (default 8090).
	Port int
}

// PodResolver is the interface the client uses to look up the owner pod
// of a workspace. Production wires this to a Postgres-backed lookup
// against workspaces_state; dev wires a stub that always returns "".
type PodResolver interface {
	PodForWorkspace(ctx context.Context, workspaceID string) (string, error)
}

func New(baseURL string) *Client {
	if baseURL == "" {
		return nil
	}
	return &Client{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		HTTP:        httpclient.Standard(6 * time.Minute),
		HeadlessSvc: "runtime-headless",
		Port:        8090,
	}
}

// Enabled is the nil-safe check callers use before reaching for the client.
func (c *Client) Enabled() bool { return c != nil && c.BaseURL != "" }

// Workspace is the subset of the runtime workspace record the orchestrator
// cares about. Kept loose on purpose so the runtime can evolve fields.
type Workspace struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	ProjectID string `json:"projectId,omitempty"`
	Status    string `json:"status"`
	Driver    string `json:"driver"`
}

// ExecOpts mirrors sandbox.ExecOpts on the runtime side.
type ExecOpts struct {
	Shell          string   `json:"shell,omitempty"`
	Cmd            []string `json:"cmd,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	Env            []string `json:"env,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
}

// ExecResult mirrors sandbox.ExecResult.
type ExecResult struct {
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exitCode"`
	DurationMS  int64  `json:"durationMs"`
	TimedOut    bool   `json:"timedOut,omitempty"`
	TruncatedAt int    `json:"truncatedAt,omitempty"`
}

// FileEntry mirrors sandbox.FileEntry on the runtime side. Returned by
// ListFiles. The runtime emits zero-value modifiedAt on drivers that
// don't surface a timestamp (mock driver).
type FileEntry struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	IsDir      bool      `json:"isDir"`
	ModifiedAt time.Time `json:"modifiedAt,omitempty"`
}

// ListWorkspaces returns every workspace visible to the bearer. The
// runtime filters by user already; the orchestrator additionally
// scopes via the resolver layer.
func (c *Client) ListWorkspaces(ctx context.Context, userBearer string) ([]Workspace, error) {
	if !c.Enabled() {
		return nil, errors.New("runtime not configured")
	}
	var list []Workspace
	if err := c.doJSON(ctx, http.MethodGet, "/workspaces", userBearer, nil, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// GetWorkspace returns a single workspace by id.
func (c *Client) GetWorkspace(ctx context.Context, userBearer, id string) (Workspace, error) {
	if !c.Enabled() {
		return Workspace{}, errors.New("runtime not configured")
	}
	var ws Workspace
	if err := c.doJSON(ctx, http.MethodGet, "/workspaces/"+id, userBearer, nil, &ws); err != nil {
		return Workspace{}, err
	}
	return ws, nil
}

// CreateWorkspace asks the runtime to provision a new sandbox bound to
// the bearer's user. Driver is honoured by the runtime's selection
// logic; pass empty to accept the runtime default.
//
// The runtime allocator (core/runtime/internal/allocator) requires
// `X-Ironflyer-Wallet-Hold: ok` and `X-Ironflyer-ProfitGuard: ok`
// before it will admit a workspace request — those are the first two
// of the five admission gates. The orchestrator enforces both laws
// before reaching this RPC (describeIdea places the wallet hold;
// ProfitGuard's BeforeSandboxAllocation hook fires upstream), so we
// stamp the markers here. Callers that *cannot* assert both — eg.
// internal tooling that creates a sandbox without paid context —
// should use a different entrypoint (none exists today; add one
// rather than loosening this default).
func (c *Client) CreateWorkspace(ctx context.Context, userBearer, projectID, driver string) (Workspace, error) {
	if !c.Enabled() {
		return Workspace{}, errors.New("runtime not configured")
	}
	in := map[string]string{}
	if projectID != "" {
		in["projectId"] = projectID
	}
	if driver != "" {
		in["driver"] = driver
	}
	var ws Workspace
	headers := map[string]string{
		"X-Ironflyer-Wallet-Hold": "ok",
		"X-Ironflyer-ProfitGuard": "ok",
	}
	if err := c.doJSONWithHeaders(ctx, http.MethodPost, "/workspaces", userBearer, headers, in, &ws); err != nil {
		return Workspace{}, err
	}
	return ws, nil
}

// DestroyWorkspace tears down a workspace. Returns nil even when the
// runtime answered 204 No Content (no body).
func (c *Client) DestroyWorkspace(ctx context.Context, userBearer, id string) error {
	if !c.Enabled() {
		return errors.New("runtime not configured")
	}
	return c.doJSON(ctx, http.MethodDelete, "/workspaces/"+id, userBearer, nil, nil)
}

// ListFiles enumerates files in the workspace. path may be the empty
// string for the root (the runtime accepts it).
func (c *Client) ListFiles(ctx context.Context, userBearer, id string) ([]FileEntry, error) {
	if !c.Enabled() {
		return nil, errors.New("runtime not configured")
	}
	var files []FileEntry
	if err := c.doJSON(ctx, http.MethodGet,
		"/workspaces/"+id+"/files", userBearer, nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// TerminalURL builds the WebSocket URL for the runtime's PTY endpoint.
// Used by the workspacePty GraphQL subscription. Scheme is rewritten
// from http(s) → ws(s) so callers can pass it straight to the
// coder/websocket dialer.
//
// When the workspace has a current owner pod (looked up via the
// PodResolver), the URL targets that pod directly via the headless
// Service so the WebSocket lands on the running container. Without a
// resolver we fall back to the load-balanced BaseURL, which works for
// single-replica dev installs.
func (c *Client) TerminalURL(workspaceID string) string {
	if !c.Enabled() {
		return ""
	}
	base, err := c.PodEndpointFor(context.Background(), workspaceID)
	if err != nil || base == "" {
		base = c.BaseURL
	}
	switch {
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + strings.TrimPrefix(base, "http://")
	}
	return base + "/workspaces/" + workspaceID + "/terminal"
}

// PodEndpointFor returns the base HTTP URL of the runtime pod that
// currently owns the workspace. The returned URL has scheme http://
// because the headless service is cluster-internal. Returns "" when
// the workspace is homeless (the caller should claim it via a
// normal request through BaseURL first) or when the PodResolver is
// not configured.
func (c *Client) PodEndpointFor(ctx context.Context, workspaceID string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("runtime not configured")
	}
	if c.PodResolver == nil {
		return "", nil
	}
	pod, err := c.PodResolver.PodForWorkspace(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if pod == "" {
		return "", nil
	}
	port := c.Port
	if port == 0 {
		port = 8090
	}
	svc := c.HeadlessSvc
	if svc == "" {
		svc = "runtime-headless"
	}
	ns := c.Namespace
	if ns == "" {
		// Fall back to <pod>.<svc>:<port> without a namespace suffix —
		// in-cluster DNS resolves the pod's namespace.
		return fmt.Sprintf("http://%s.%s:%d", pod, svc, port), nil
	}
	return fmt.Sprintf("http://%s.%s.%s.svc.cluster.local:%d", pod, svc, ns, port), nil
}

// FindWorkspaceForProject returns the most recently active workspace owned by
// `userBearer` and bound to projectID. Empty result is an error from the
// caller's perspective — gates should treat it as "no runtime to run in."
func (c *Client) FindWorkspaceForProject(ctx context.Context, userBearer, projectID string) (Workspace, error) {
	if !c.Enabled() {
		return Workspace{}, errors.New("runtime not configured")
	}
	var list []Workspace
	if err := c.doJSON(ctx, http.MethodGet, "/workspaces", userBearer, nil, &list); err != nil {
		return Workspace{}, err
	}
	for _, ws := range list {
		if ws.ProjectID == projectID && ws.Status == "running" {
			return ws, nil
		}
	}
	return Workspace{}, fmt.Errorf("no running workspace bound to project %q", projectID)
}

// PreviewBinding mirrors sandbox.PreviewBinding on the orchestrator
// side so wowloop / studio resolvers can surface a live-preview URL
// the moment the workspace exists.
type PreviewBinding struct {
	WorkspaceID  string    `json:"workspaceId"`
	InternalPort int       `json:"internalPort"`
	ExternalPort int       `json:"externalPort"`
	URL          string    `json:"url"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// AllocatePreview asks the runtime to assign a preview port for the
// workspace's dev server and returns the binding (including the URL
// the iframe can load). Idempotent on the runtime side — calling
// twice with the same workspace returns the same binding and refreshes
// its lease.
func (c *Client) AllocatePreview(ctx context.Context, userBearer, workspaceID string, internalPort int) (PreviewBinding, error) {
	if !c.Enabled() {
		return PreviewBinding{}, errors.New("runtime not configured")
	}
	if workspaceID == "" {
		return PreviewBinding{}, errors.New("workspaceID required")
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	in := map[string]int{}
	if internalPort > 0 {
		in["internalPort"] = internalPort
	}
	var out PreviewBinding
	if err := c.doJSON(cctx, http.MethodPost,
		"/workspaces/"+workspaceID+"/preview", userBearer, in, &out); err != nil {
		return PreviewBinding{}, err
	}
	return out, nil
}

// PreviewURL returns the current live-preview URL for the workspace,
// or "" if none has been allocated yet (404 from the runtime). Errors
// other than 404 propagate so the caller can surface them in logs.
func (c *Client) PreviewURL(ctx context.Context, userBearer, workspaceID string) (string, error) {
	if !c.Enabled() {
		return "", nil
	}
	if workspaceID == "" {
		return "", nil
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var out PreviewBinding
	err := c.doJSON(cctx, http.MethodGet,
		"/workspaces/"+workspaceID+"/preview", userBearer, nil, &out)
	if err != nil {
		// "no preview allocated" surfaces as a 404 via the runtime's
		// writeJSON path; we treat it as "no URL available yet"
		// rather than a hard error so callers can degrade quietly.
		if strings.Contains(err.Error(), "runtime 404") || strings.Contains(err.Error(), "no preview allocated") {
			return "", nil
		}
		return "", err
	}
	return out.URL, nil
}

// Exec runs a single command in the given workspace.
func (c *Client) Exec(ctx context.Context, userBearer, workspaceID string, opts ExecOpts) (ExecResult, error) {
	if !c.Enabled() {
		return ExecResult{}, errors.New("runtime not configured")
	}
	var res ExecResult
	if err := c.doJSON(ctx, http.MethodPost,
		"/workspaces/"+workspaceID+"/exec", userBearer, opts, &res); err != nil {
		return ExecResult{}, err
	}
	return res, nil
}

// doJSON is the shared request/response codec. userBearer, when set, is
// forwarded as `Authorization: Bearer <token>` so the runtime enforces the
// per-user ownership check.
func (c *Client) doJSON(ctx context.Context, method, path, userBearer string, in, out any) error {
	return c.doJSONWithHeaders(ctx, method, path, userBearer, nil, in, out)
}

func (c *Client) doJSONWithHeaders(ctx context.Context, method, path, userBearer string, headers map[string]string, in, out any) error {
	var body io.Reader
	if in != nil {
		bts, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		body = bytes.NewReader(bts)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if userBearer != "" {
		req.Header.Set("Authorization", "Bearer "+userBearer)
	}
	for k, v := range headers {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("runtime call: %w", err)
	}
	defer resp.Body.Close()
	bts, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("runtime %d: %s", resp.StatusCode, strings.TrimSpace(string(bts)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(bts, out); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}
