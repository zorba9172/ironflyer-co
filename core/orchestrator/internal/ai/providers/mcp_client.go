// Package providers — MCP client. Lets the orchestrator CALL external
// Model Context Protocol servers and surface their tools to our
// agents. The MCP *server* we expose lets external clients drive
// Ironflyer; this is the inverse — Ironflyer agents reach OUTWARDS to
// Notion, Linear, GitHub, Sentry, custom internal tools.
//
// Bolt's "MCP integration" headline is built on exactly this: their
// Coder can call `notion.create_page` or `linear.create_issue` mid-
// generation. By implementing the client side, our Coder gains the
// same superpower without per-integration code.
//
// We implement the Streamable HTTP transport (POST JSON-RPC). The
// stdio transport is left out of v1 — that requires spawning a child
// process per server, which is a separate operational concern we'd
// rather solve once the use-case demands it.

package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// MCPClient talks to a single MCP server over Streamable HTTP. Multiple
// servers are wrapped by MCPClientRegistry below.
type MCPClient struct {
	// Endpoint is the server's POST URL, e.g. "https://mcp.notion.com".
	Endpoint string
	// Authorization is the value sent on the Authorization header. Empty
	// means no auth. We do not bake in a scheme so callers can choose
	// Bearer / Basic / custom.
	Authorization string
	// HTTP is the client used for all calls. Leave nil to get a 20s
	// default; callers can swap for retry-aware clients in production.
	HTTP *http.Client
	// Name is a human label used to namespace tool names ("notion",
	// "linear"). Must be set; we prefix every surfaced tool with it.
	Name string

	mu       sync.Mutex
	nextID   int
	tools    []MCPTool
	toolsErr error
}

// MCPTool is a tool advertised by an MCP server. We carry the schema
// verbatim so the Coder can pass it to a provider that natively
// supports tool-use.
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

func (c *MCPClient) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return httpclient.Standard(20 * time.Second)
}

// Initialize performs the MCP handshake. Idempotent — safe to call
// multiple times. We swallow the version mismatch error and continue:
// most servers downgrade silently to our advertised version.
func (c *MCPClient) Initialize(ctx context.Context) error {
	_, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "ironflyer-orchestrator",
			"version": "1.0",
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	})
	return err
}

// ListTools fetches the server's tool catalogue and caches it. The
// returned slice has its Name field prefixed with "<clientName>." so
// the agent registry can route a call back to the right server.
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPTool, error) {
	c.mu.Lock()
	if c.tools != nil || c.toolsErr != nil {
		defer c.mu.Unlock()
		return c.tools, c.toolsErr
	}
	c.mu.Unlock()

	raw, err := c.call(ctx, "tools/list", map[string]any{})
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.toolsErr = err
		return nil, err
	}
	var parsed struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		c.toolsErr = err
		return nil, err
	}
	prefix := c.Name
	if prefix == "" {
		prefix = "mcp"
	}
	out := make([]MCPTool, 0, len(parsed.Tools))
	for _, t := range parsed.Tools {
		t.Name = prefix + "." + t.Name
		out = append(out, t)
	}
	c.tools = out
	return out, nil
}

// CallTool invokes a tool on the server. `toolName` may be supplied
// either prefixed ("notion.create_page") or bare ("create_page"); we
// strip the prefix if it matches our own Name.
func (c *MCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]any) (string, error) {
	bare := toolName
	if c.Name != "" {
		prefix := c.Name + "."
		if strings.HasPrefix(toolName, prefix) {
			bare = strings.TrimPrefix(toolName, prefix)
		}
	}
	raw, err := c.call(ctx, "tools/call", map[string]any{
		"name":      bare,
		"arguments": arguments,
	})
	if err != nil {
		return "", err
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
			b.WriteByte('\n')
		}
	}
	out := strings.TrimSpace(b.String())
	if parsed.IsError {
		return out, fmt.Errorf("mcp tool error: %s", tailLine(out))
	}
	return out, nil
}

func (c *MCPClient) call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	if c.Endpoint == "" {
		return nil, errors.New("mcp: endpoint required")
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.Authorization != "" {
		req.Header.Set("Authorization", c.Authorization)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("mcp %s %s: %d: %s", method, c.Endpoint, resp.StatusCode, tailLine(string(raw)))
	}
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("mcp decode: %w", err)
	}
	if env.Error != nil {
		return nil, fmt.Errorf("mcp %s error %d: %s", method, env.Error.Code, env.Error.Message)
	}
	return env.Result, nil
}

func tailLine(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

// MCPClientRegistry holds many configured MCP servers and aggregates
// their tool catalogues. Agents see one flat list of namespaced tools
// (e.g. "notion.create_page", "linear.create_issue") and the registry
// routes each CallTool back to the right backend.
type MCPClientRegistry struct {
	mu      sync.RWMutex
	clients []*MCPClient
}

func NewMCPClientRegistry() *MCPClientRegistry { return &MCPClientRegistry{} }

func (r *MCPClientRegistry) Register(c *MCPClient) {
	r.mu.Lock()
	r.clients = append(r.clients, c)
	r.mu.Unlock()
}

// Initialize calls Initialize on every registered client; errors are
// collected and returned wrapped so a partial outage doesn't disable
// the whole registry.
func (r *MCPClientRegistry) Initialize(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var errs []string
	for _, c := range r.clients {
		if err := c.Initialize(ctx); err != nil {
			errs = append(errs, c.Name+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("mcp init: %s", strings.Join(errs, "; "))
	}
	return nil
}

// AllTools returns the union of every backend's tool catalogue with
// per-backend prefixes already applied. Backends that error are
// silently skipped — the rest of the registry stays usable.
func (r *MCPClientRegistry) AllTools(ctx context.Context) []MCPTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []MCPTool
	for _, c := range r.clients {
		tools, err := c.ListTools(ctx)
		if err != nil {
			continue
		}
		out = append(out, tools...)
	}
	return out
}

// CallTool routes a prefixed tool name back to its backend. Returns
// (text, error). Unknown prefix → error.
func (r *MCPClientRegistry) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	dot := strings.IndexByte(name, '.')
	if dot < 0 {
		return "", errors.New("mcp call: tool name must be 'server.tool'")
	}
	prefix := name[:dot]
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.clients {
		if c.Name == prefix {
			return c.CallTool(ctx, name, args)
		}
	}
	return "", fmt.Errorf("mcp call: no server named %q", prefix)
}
