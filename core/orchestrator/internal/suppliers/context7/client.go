// Package context7 is a production client for context7.com — the
// up-to-date, version-accurate library documentation service. The
// orchestrator exposes it to the Coder/Architect agents as a built-in
// `lookup_docs` tool so generated code is grounded against current
// library APIs instead of a model's training-cutoff memory.
//
// Two-step protocol (per https://context7.com/docs/api-guide):
//
//	GET /api/v2/libs/search?libraryName=&query=   -> resolve a name to a
//	                                                 context7 library id
//	GET /api/v2/context?libraryId=&query=         -> fetch doc snippets
//
// Auth is a bearer token (CONTEXT7_API_KEY). Every method takes a
// context.Context so the caller controls cancellation/timeout; a short
// default client timeout guards against a hung upstream.
package context7

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://context7.com"

// maxDocBytes caps the text handed back to the agent so a giant docs
// payload can't blow the model's context window. Snippets past this are
// truncated with a marker.
const maxDocBytes = 12000

// Client talks to the context7 HTTP API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Option customises the client.
type Option func(*Client)

// WithBaseURL overrides the API base (e.g. for a self-hosted mirror).
func WithBaseURL(base string) Option {
	return func(c *Client) {
		if b := strings.TrimRight(strings.TrimSpace(base), "/"); b != "" {
			c.baseURL = b
		}
	}
}

// WithHTTPClient injects a custom http.Client (tests, custom transport).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// New builds a Client. apiKey is required; New returns nil when it is
// empty so callers can treat "no key" as "feature disabled".
func New(apiKey string, opts ...Option) *Client {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// searchResponse models GET /api/v2/libs/search. The API may carry more
// fields; we decode only what we use and ignore the rest.
type searchResponse struct {
	Results []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"results"`
}

// contextResponse models GET /api/v2/context. Both snippet arrays are
// optional; we render whatever is present.
type contextResponse struct {
	CodeSnippets []struct {
		CodeTitle string   `json:"codeTitle"`
		CodeList  []string `json:"codeList"`
	} `json:"codeSnippets"`
	InfoSnippets []struct {
		Content string `json:"content"`
	} `json:"infoSnippets"`
}

// ResolveLibrary maps a human library name (e.g. "next.js") to a
// context7 library id (e.g. "/vercel/next.js"). When name already looks
// like an id (starts with "/") it is returned unchanged without a round
// trip. query is an optional natural-language hint that disambiguates
// when several libraries match the name.
func (c *Client) ResolveLibrary(ctx context.Context, name, query string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("context7: empty library name")
	}
	if strings.HasPrefix(name, "/") {
		return name, nil
	}
	q := url.Values{}
	q.Set("libraryName", name)
	if query != "" {
		q.Set("query", query)
	}
	var out searchResponse
	if err := c.get(ctx, "/api/v2/libs/search", q, &out); err != nil {
		return "", err
	}
	if len(out.Results) == 0 || out.Results[0].ID == "" {
		return "", fmt.Errorf("context7: no library matched %q", name)
	}
	return out.Results[0].ID, nil
}

// GetDocs fetches documentation for a resolved library id, scoped by a
// natural-language query, and renders it into a compact text block the
// agent can read. Capped at maxDocBytes.
func (c *Client) GetDocs(ctx context.Context, libraryID, query string) (string, error) {
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return "", fmt.Errorf("context7: empty library id")
	}
	q := url.Values{}
	q.Set("libraryId", libraryID)
	if query != "" {
		q.Set("query", query)
	}
	var out contextResponse
	if err := c.get(ctx, "/api/v2/context", q, &out); err != nil {
		return "", err
	}
	return renderDocs(libraryID, out), nil
}

// Lookup is the convenience the agent tool calls: resolve the library
// (if needed) then fetch its docs in one step.
func (c *Client) Lookup(ctx context.Context, library, query string) (string, error) {
	id, err := c.ResolveLibrary(ctx, library, query)
	if err != nil {
		return "", err
	}
	return c.GetDocs(ctx, id, query)
}

// get issues an authenticated GET and decodes the JSON body into out.
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	u := c.baseURL + path
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("context7: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("context7: request %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("context7: rate limited (retry-after %s)", resp.Header.Get("Retry-After"))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("context7: %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("context7: decode %s: %w", path, err)
	}
	return nil
}

// renderDocs flattens the snippet arrays into a readable text block.
// Falls back to a clear "no docs" line when the payload was empty so the
// agent gets a definite signal rather than silence.
func renderDocs(libraryID string, c contextResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Documentation for %s (via context7):\n\n", libraryID)
	for _, s := range c.InfoSnippets {
		if t := strings.TrimSpace(s.Content); t != "" {
			b.WriteString(t)
			b.WriteString("\n\n")
		}
	}
	for _, s := range c.CodeSnippets {
		if title := strings.TrimSpace(s.CodeTitle); title != "" {
			fmt.Fprintf(&b, "### %s\n", title)
		}
		for _, code := range s.CodeList {
			if t := strings.TrimSpace(code); t != "" {
				b.WriteString("```\n")
				b.WriteString(t)
				b.WriteString("\n```\n\n")
			}
		}
	}
	out := strings.TrimSpace(b.String())
	if len(c.InfoSnippets) == 0 && len(c.CodeSnippets) == 0 {
		return fmt.Sprintf("context7 returned no documentation snippets for %s.", libraryID)
	}
	if len(out) > maxDocBytes {
		out = out[:maxDocBytes] + "\n\n…[truncated]"
	}
	return out
}
