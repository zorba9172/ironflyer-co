// Package github_pr is the GitHub PR-review supplier. It owns the
// outbound REST client used by the webhook receiver to:
//
//   - fetch the PR + changed files,
//   - post a structured review comment back onto the PR,
//   - (optionally) create a formal review with approve/request_changes.
//
// Auth is per-project: the bearer token is read from
// Project.Secrets["GITHUB_PAT"]. The signature-verification key lives
// in Project.Secrets["GITHUB_WEBHOOK_SECRET"]. Both are NEVER serialised
// through the API surface — the receiver loads them in-process.
//
// Resilience contract:
//   - 30s per-request timeout.
//   - Single retry on 5xx / 429, honouring Retry-After when set.
//   - Caller closes response bodies (the helpers do so internally).
package github_pr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the GitHub REST API root. Override via
// NewClient(..., WithBaseURL(...)) for GitHub Enterprise.
const DefaultBaseURL = "https://api.github.com"

// defaultRequestTimeout caps each outbound HTTP call. The whole
// webhook handler is bounded by the orchestrator's higher-level
// context deadline; this is a defence-in-depth backstop.
const defaultRequestTimeout = 30 * time.Second

// Client is a thin GitHub REST client. Safe for concurrent use — every
// request constructs its own *http.Request and consumes its own body.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	userAgent  string
}

// Option mutates a *Client during construction.
type Option func(*Client)

// WithBaseURL points the client at a non-public GitHub host (GHE).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(strings.TrimSpace(url), "/")
	}
}

// WithHTTPClient injects a caller-owned http.Client. Useful when the
// orchestrator wires a shared transport with metrics / tracing.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithUserAgent overrides the default UA string. GitHub rejects calls
// without a User-Agent.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// NewClient builds a Client. `token` is the GitHub PAT (or fine-grained
// installation token); the empty string leaves the request unauthenticated
// which is rarely useful but kept legal for the readme-fetch path.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultRequestTimeout},
		baseURL:    DefaultBaseURL,
		token:      strings.TrimSpace(token),
		userAgent:  "Ironflyer-Orchestrator/1.0 (+https://ironflyer.dev)",
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.httpClient.Timeout == 0 {
		c.httpClient.Timeout = defaultRequestTimeout
	}
	return c
}

// PullRequest is the subset of the GitHub PR object the orchestrator
// consumes. Anything not used by the gate pipeline is intentionally
// dropped to keep the parse surface tight.
type PullRequest struct {
	Number    int    `json:"number"`
	State     string `json:"state"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
	DiffURL   string `json:"diff_url"`
	PatchURL  string `json:"patch_url"`
	Mergeable *bool  `json:"mergeable,omitempty"`
	Head      struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ChangedFile is one entry from /repos/{owner}/{repo}/pulls/{n}/files.
// We mirror the wire fields the gate pipeline consumes; the rest are
// dropped on parse.
type ChangedFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Changes   int    `json:"changes"`
	Patch     string `json:"patch,omitempty"` // unified-diff hunk
	RawURL    string `json:"raw_url,omitempty"`
	SHA       string `json:"sha,omitempty"`
}

// ReviewEvent maps to the GitHub review "event" enum.
type ReviewEvent string

const (
	ReviewEventComment        ReviewEvent = "COMMENT"
	ReviewEventApprove        ReviewEvent = "APPROVE"
	ReviewEventRequestChanges ReviewEvent = "REQUEST_CHANGES"
)

// GetPullRequest fetches a PR by (owner, repo, number).
func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	if err := validateRepoTuple(owner, repo, number); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	var pr PullRequest
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &pr); err != nil {
		return nil, fmt.Errorf("get pull: %w", err)
	}
	return &pr, nil
}

// ListChangedFiles returns the full list of files changed by the PR,
// paginating until GitHub stops setting the `next` Link.
func (c *Client) ListChangedFiles(ctx context.Context, owner, repo string, number int) ([]ChangedFile, error) {
	if err := validateRepoTuple(owner, repo, number); err != nil {
		return nil, err
	}
	const perPage = 100
	const maxPages = 30 // 3,000 files is plenty for any sane PR review
	var out []ChangedFile
	for page := 1; page <= maxPages; page++ {
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=%d&page=%d",
			owner, repo, number, perPage, page)
		var batch []ChangedFile
		if err := c.doJSON(ctx, http.MethodGet, path, nil, &batch); err != nil {
			return nil, fmt.Errorf("list files page %d: %w", page, err)
		}
		out = append(out, batch...)
		if len(batch) < perPage {
			break
		}
	}
	return out, nil
}

// CreatePRComment posts a markdown comment on the PR conversation
// thread (Issue-style, not file-anchored). Used by the Reviewer to
// surface the verdict.
func (c *Client) CreatePRComment(ctx context.Context, owner, repo string, number int, body string) error {
	if err := validateRepoTuple(owner, repo, number); err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("comment body is empty")
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	payload := map[string]string{"body": body}
	return c.doJSON(ctx, http.MethodPost, path, payload, nil)
}

// CreateReview posts a formal pull-request review. event may be
// COMMENT, APPROVE, or REQUEST_CHANGES.
func (c *Client) CreateReview(ctx context.Context, owner, repo string, number int, event ReviewEvent, body string) error {
	if err := validateRepoTuple(owner, repo, number); err != nil {
		return err
	}
	if event == "" {
		event = ReviewEventComment
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, number)
	payload := map[string]string{
		"event": string(event),
		"body":  body,
	}
	return c.doJSON(ctx, http.MethodPost, path, payload, nil)
}

// doJSON performs the HTTP round-trip and decodes the JSON body into
// out (when non-nil). 5xx + 429 responses are retried once after the
// server-recommended Retry-After delay (capped at 30s).
func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error {
	var bodyBytes []byte
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		bodyBytes = b
	}
	url := c.baseURL + path

	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", c.userAgent)
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == 0 {
				continue
			}
			return fmt.Errorf("http: %w", err)
		}

		// Retry path for transient failures.
		if (resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests) && attempt == 0 {
			delay := retryAfter(resp.Header.Get("Retry-After"))
			_ = drainAndClose(resp)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			return fmt.Errorf("github %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(body)))
		}

		if out == nil {
			_ = drainAndClose(resp)
			return nil
		}
		dec := json.NewDecoder(resp.Body)
		dec.UseNumber()
		err = dec.Decode(out)
		_ = drainAndClose(resp)
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("decode: %w", err)
		}
		return nil
	}
	return errors.New("github: exhausted retries")
}

// retryAfter parses an HTTP Retry-After header (seconds form). Falls
// back to a 1s delay when the header is absent or malformed; capped at
// 30s to bound the per-request budget.
func retryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return time.Second
	}
	if n, err := strconv.Atoi(h); err == nil {
		if n < 0 {
			n = 0
		}
		if n > 30 {
			n = 30
		}
		return time.Duration(n) * time.Second
	}
	return time.Second
}

func drainAndClose(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.Body.Close()
}

func validateRepoTuple(owner, repo string, number int) error {
	if strings.TrimSpace(owner) == "" {
		return errors.New("owner is empty")
	}
	if strings.TrimSpace(repo) == "" {
		return errors.New("repo is empty")
	}
	if number <= 0 {
		return fmt.Errorf("invalid PR number: %d", number)
	}
	return nil
}
