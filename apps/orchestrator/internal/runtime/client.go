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
)

// Client talks to the runtime over HTTP. Construct with New; reuse across
// calls to share the HTTP transport. A nil Client means "runtime not
// configured" — Enabled() reports that.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	if baseURL == "" {
		return nil
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 6 * time.Minute},
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
