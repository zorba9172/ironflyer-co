// Package sentry — orchestrator-side client + webhook receiver for
// Sentry. Sentry feeds the orchestrator two ways:
//
//  1. As a target the Coder can call (via the MCP server entry in
//     the catalog) — that's the agent loop side.
//  2. As a SOURCE that pushes new issue alerts into the orchestrator
//     so a real Devin-style "Sentry error → Coder fix → PR" loop can
//     run with zero operator intervention. That's this package.
//
// The thin REST client below handles (1)-shaped calls the webhook
// handler needs after a signature-verified event lands: fetching
// extra issue context (stack trace, suspect commit, breadcrumbs) so
// the resulting finisher.Issue carries enough detail for the Coder.
package sentry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a thin Sentry REST wrapper. Construct via NewClient — the
// zero value has no auth and will 401 against every endpoint.
type Client struct {
	// AuthToken is the Sentry user / internal-integration auth token.
	// Sourced per-project from Project.Secrets["SENTRY_AUTH_TOKEN"]
	// where available; the global token wired at boot is the fallback.
	AuthToken string
	// BaseURL defaults to https://sentry.io/api/0; self-hosted Sentry
	// deployments override via Project.Secrets["SENTRY_BASE_URL"].
	BaseURL string
	// HTTP is the client used for the request. Leave nil to get a
	// 15-second default; production wires a retry-aware client.
	HTTP *http.Client
}

// Issue is the subset of the Sentry issue payload the orchestrator
// cares about. We intentionally keep the shape narrow so a Sentry
// payload schema change does not ripple through the rest of the code.
type Issue struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Culprit    string `json:"culprit"`
	Permalink  string `json:"permalink"`
	Level      string `json:"level"`
	Platform   string `json:"platform"`
	Project    struct {
		Slug string `json:"slug"`
	} `json:"project"`
	LastSeen string `json:"lastSeen"`
	Metadata struct {
		Type     string `json:"type"`
		Value    string `json:"value"`
		Filename string `json:"filename"`
		Function string `json:"function"`
	} `json:"metadata"`
}

// NewClient returns a configured client. authToken empty means the
// caller will set it lazily per-request (rare); BaseURL empty falls
// back to https://sentry.io/api/0.
func NewClient(authToken, baseURL string) *Client {
	c := &Client{AuthToken: authToken, BaseURL: baseURL}
	if c.BaseURL == "" {
		c.BaseURL = "https://sentry.io/api/0"
	}
	return c
}

// GetIssue fetches the canonical issue document by Sentry id. Empty
// id returns an error rather than hitting `/issues//` which would
// 404 with an unhelpful body.
func (c *Client) GetIssue(ctx context.Context, id string) (*Issue, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("sentry: issue id required")
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/issues/" + id + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("sentry: GET %s: %d: %s", url, resp.StatusCode, truncErr(string(body)))
	}
	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("sentry: decode issue: %w", err)
	}
	return &issue, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func truncErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
