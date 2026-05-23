// Package httpapi — MCP (Model Context Protocol) server endpoint.
//
// MCP is Anthropic's open protocol for exposing tools + resources to AI
// clients (Claude Desktop, Cursor, Zed, the Anthropic SDK Tool-Use loop,
// other agents). By implementing the server side, Ironflyer becomes
// addressable by any compliant client: a developer can wire Claude
// Desktop directly to their Ironflyer instance and ask the model to
// "list my projects", "read the files of project X", "propose a patch
// that fixes the failing tests".
//
// We implement the Streamable HTTP transport (single endpoint at
// /mcp accepting JSON-RPC 2.0 requests). Full bidirectional streaming
// is overkill for the surface we expose — the operations are short,
// not long-running.
//
// Methods implemented today (a strict subset of the MCP spec — enough
// to be useful, not so much that maintenance is a tax):
//   - initialize        → negotiate version + capabilities
//   - tools/list        → enumerate Ironflyer tools
//   - tools/call        → invoke a tool by name with arguments
//   - resources/list    → enumerate readable resources (each project)
//   - resources/read    → fetch the contents of a resource
//
// Auth: re-uses the orchestrator's auth middleware so MCP requests
// carry the user's bearer token. Anonymous calls only see public
// (OwnerID="") projects.
//
// Spec reference: https://modelcontextprotocol.io/specification

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/memory"
	"ironflyer/apps/orchestrator/internal/patch"
)

// mcpProtocolVersion is the version we advertise to clients. Clients
// that send a newer version downgrade to this; older clients we reject.
const mcpProtocolVersion = "2024-11-05"

// mcpServerInfo is the announce-side identity. Stable so reverse-proxy
// caches don't churn.
var mcpServerInfo = map[string]any{
	"name":    "ironflyer-orchestrator",
	"version": "1.0",
}

// mcpRPC is a JSON-RPC 2.0 envelope. We unmarshal Params into the
// per-method shape inside the handler instead of using interface{}.
type mcpRPC struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResult struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// mcpHandle dispatches a single JSON-RPC request. Notification messages
// (no ID) are accepted but produce no response per spec.
func (a *API) mcpHandle(w http.ResponseWriter, r *http.Request) {
	var req mcpRPC
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, mcpResult{
			JSONRPC: "2.0",
			Error:   &mcpError{Code: -32700, Message: "Parse error: " + err.Error()},
		})
		return
	}
	if req.JSONRPC != "2.0" {
		a.mcpReply(w, req.ID, nil, &mcpError{Code: -32600, Message: "Invalid Request: jsonrpc must be '2.0'"})
		return
	}
	switch req.Method {
	case "initialize":
		a.mcpReply(w, req.ID, map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"serverInfo":      mcpServerInfo,
			"capabilities": map[string]any{
				"tools":     map[string]any{"listChanged": false},
				"resources": map[string]any{"subscribe": false, "listChanged": false},
			},
		}, nil)
	case "notifications/initialized", "ping":
		// Notifications don't carry IDs. Still — respond 200 with empty body
		// when they slip in over HTTP so clients that wait for a reply don't
		// hang.
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		a.mcpReply(w, req.ID, map[string]any{}, nil)
	case "tools/list":
		a.mcpReply(w, req.ID, mcpToolsList(), nil)
	case "tools/call":
		a.mcpCallTool(r.Context(), w, r, req)
	case "resources/list":
		a.mcpResourcesList(w, r, req)
	case "resources/read":
		a.mcpResourcesRead(w, r, req)
	default:
		a.mcpReply(w, req.ID, nil, &mcpError{Code: -32601, Message: "Method not found: " + req.Method})
	}
}

func (a *API) mcpReply(w http.ResponseWriter, id json.RawMessage, result any, errObj *mcpError) {
	w.Header().Set("Content-Type", "application/json")
	resp := mcpResult{JSONRPC: "2.0", ID: id, Result: result, Error: errObj}
	_ = json.NewEncoder(w).Encode(resp)
}

// mcpToolsList enumerates the tools we accept. Each tool gets a strict
// JSON Schema so clients can validate args before sending. Schemas are
// hand-rolled here rather than reflected so a refactor of the internal
// types can't silently expand the public tool surface.
func mcpToolsList() map[string]any {
	return map[string]any{
		"tools": []map[string]any{
			{
				"name":        "list_projects",
				"description": "List Ironflyer projects accessible to the authenticated user. Returns id, name, owner, and per-gate status.",
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
			{
				"name":        "get_project",
				"description": "Fetch a single project including spec, file tree, gate verdicts, and recent events.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"projectId": map[string]any{"type": "string", "description": "Ironflyer project id"},
					},
					"required": []string{"projectId"},
				},
			},
			{
				"name":        "read_file",
				"description": "Read the contents of a single file from a project's tracked tree.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"projectId": map[string]any{"type": "string"},
						"path":      map[string]any{"type": "string", "description": "Relative path inside the project tree"},
					},
					"required": []string{"projectId", "path"},
				},
			},
			{
				"name":        "propose_patch",
				"description": "Propose a patch against a project. The patch goes through the same lifecycle as a Coder-emitted patch — validation + critic + apply. Returns the resulting Patch with its issues (if any).",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"projectId": map[string]any{"type": "string"},
						"title":     map[string]any{"type": "string"},
						"summary":   map[string]any{"type": "string"},
						"changes": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"op":          map[string]any{"type": "string", "enum": []string{"create", "update", "delete", "replace", "insert_after"}},
									"path":        map[string]any{"type": "string"},
									"content":     map[string]any{"type": "string"},
									"anchor":      map[string]any{"type": "string"},
									"replacement": map[string]any{"type": "string"},
								},
								"required": []string{"op", "path"},
							},
						},
					},
					"required": []string{"projectId", "changes"},
				},
			},
			{
				"name":        "list_gates",
				"description": "Return the current gate verdicts for a project as a flat list.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"projectId": map[string]any{"type": "string"},
					},
					"required": []string{"projectId"},
				},
			},
			{
				"name":        "add_memory",
				"description": "Persist a single memory record (project / execution / user / business). User-kind records are pinned to the caller's userId.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"kind":       map[string]any{"type": "string", "enum": []string{"project", "execution", "user", "business"}},
						"projectId":  map[string]any{"type": "string"},
						"userId":     map[string]any{"type": "string"},
						"storyId":    map[string]any{"type": "string"},
						"gateName":   map[string]any{"type": "string"},
						"title":      map[string]any{"type": "string"},
						"body":       map[string]any{"type": "string"},
						"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
					},
					"required": []string{"kind", "title", "body"},
				},
			},
			{
				"name":        "query_memory",
				"description": "Search memory records. Project-scoped reads require project access; user-scoped reads only return the caller's own records.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"kind":      map[string]any{"type": "string", "enum": []string{"project", "execution", "user", "business"}},
						"projectId": map[string]any{"type": "string"},
						"userId":    map[string]any{"type": "string"},
						"storyId":   map[string]any{"type": "string"},
						"gateName":  map[string]any{"type": "string"},
						"tag":       map[string]any{"type": "string"},
						"q":         map[string]any{"type": "string", "description": "Case-insensitive substring matched against title+body"},
						"limit":     map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
					},
				},
			},
			{
				"name":        "delete_memory",
				"description": "Delete a memory record by id. Idempotent — unknown ids succeed silently.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
					},
					"required": []string{"id"},
				},
			},
			{
				"name":        "list_audit",
				"description": "List audit-log entries. Project-scoped reads require project access; otherwise results are scoped to the caller's userId.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"projectId":    map[string]any{"type": "string"},
						"action":       map[string]any{"type": "string", "description": "patch.proposed | patch.applied | gate.verdict | ..."},
						"outcome":      map[string]any{"type": "string", "enum": []string{"success", "failure", "blocked"}},
						"sinceRfc3339": map[string]any{"type": "string", "description": "RFC3339 timestamp lower bound"},
						"untilRfc3339": map[string]any{"type": "string", "description": "RFC3339 timestamp upper bound"},
						"limit":        map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
					},
				},
			},
			{
				"name":        "verify_audit",
				"description": "Walk the audit log hash chain. Returns { intact, firstBadIndex } — intact=true when the log has not been tampered with.",
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
	}
}

// mcpCallTool dispatches a tools/call invocation. We use the same auth
// + ownership checks the HTTP routes use so MCP cannot exfiltrate other
// users' work.
func (a *API) mcpCallTool(ctx context.Context, w http.ResponseWriter, r *http.Request, req mcpRPC) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &call); err != nil {
		a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "Invalid params: " + err.Error()})
		return
	}
	userID := userIDFromCtx(r)

	switch call.Name {
	case "list_projects":
		all := a.d.Projects.List()
		out := make([]map[string]any, 0, len(all))
		for _, p := range all {
			if !p.IsAccessibleBy(userID) {
				continue
			}
			out = append(out, mcpProjectSummary(p))
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(out))

	case "get_project":
		var args struct {
			ProjectID string `json:"projectId"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil || args.ProjectID == "" {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "projectId required"})
			return
		}
		p, err := a.d.Projects.Get(args.ProjectID)
		if err != nil || !p.IsAccessibleBy(userID) {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "project not found or not accessible"})
			return
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(p))

	case "read_file":
		var args struct {
			ProjectID string `json:"projectId"`
			Path      string `json:"path"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil || args.ProjectID == "" || args.Path == "" {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "projectId and path required"})
			return
		}
		p, err := a.d.Projects.Get(args.ProjectID)
		if err != nil || !p.IsAccessibleBy(userID) {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "project not found or not accessible"})
			return
		}
		for _, f := range p.Files {
			if f.Path == args.Path {
				a.mcpToolResult(w, req.ID, []map[string]any{
					{"type": "text", "text": f.Content},
				})
				return
			}
		}
		a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "file not found: " + args.Path})

	case "propose_patch":
		var args struct {
			ProjectID string             `json:"projectId"`
			Title     string             `json:"title"`
			Summary   string             `json:"summary"`
			Changes   []patch.FileChange `json:"changes"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil || args.ProjectID == "" || len(args.Changes) == 0 {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "projectId and changes required"})
			return
		}
		proj, err := a.d.Projects.Get(args.ProjectID)
		if err != nil || !proj.IsAccessibleBy(userID) {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "project not found or not accessible"})
			return
		}
		// Sanitise op casing + path traversal — the patch engine's own
		// validator catches more, but cheap early checks improve error
		// messages over the wire.
		for i := range args.Changes {
			args.Changes[i].Op = patch.Op(strings.ToLower(strings.TrimSpace(string(args.Changes[i].Op))))
			args.Changes[i].Path = strings.TrimPrefix(args.Changes[i].Path, "/")
		}
		out, err := a.d.Patches.Propose(patch.Patch{
			ProjectID: args.ProjectID,
			Title:     args.Title,
			Summary:   args.Summary,
			Author:    "mcp:" + userID,
			Changes:   args.Changes,
		})
		if err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32000, Message: err.Error()})
			return
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(out))

	case "list_gates":
		var args struct {
			ProjectID string `json:"projectId"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil || args.ProjectID == "" {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "projectId required"})
			return
		}
		p, err := a.d.Projects.Get(args.ProjectID)
		if err != nil || !p.IsAccessibleBy(userID) {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "project not found or not accessible"})
			return
		}
		gates := make([]domain.GateState, 0, len(p.Gates))
		for _, name := range domain.AllGates() {
			if g, ok := p.Gates[name]; ok {
				gates = append(gates, g)
			}
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(gates))

	case "add_memory":
		if a.d.Memory == nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "memory not configured"})
			return
		}
		var rec memory.Record
		if err := json.Unmarshal(call.Arguments, &rec); err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "Invalid params: " + err.Error()})
			return
		}
		switch rec.Kind {
		case memory.KindProject, memory.KindExecution, memory.KindBusiness:
			if rec.ProjectID == "" {
				a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "projectId required for this kind"})
				return
			}
			p, err := a.d.Projects.Get(rec.ProjectID)
			if err != nil || !p.IsAccessibleBy(userID) {
				a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "project not found or not accessible"})
				return
			}
		case memory.KindUser:
			// Always stamp the caller — never trust a payload-supplied userId.
			rec.UserID = userID
			if rec.UserID == "" {
				a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "user memory requires authentication"})
				return
			}
		default:
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "kind must be project | execution | user | business"})
			return
		}
		stored, err := a.d.Memory.Record(ctx, rec)
		if err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32000, Message: err.Error()})
			return
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(stored))

	case "query_memory":
		if a.d.Memory == nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "memory not configured"})
			return
		}
		var args struct {
			Kind      string `json:"kind"`
			ProjectID string `json:"projectId"`
			UserID    string `json:"userId"`
			StoryID   string `json:"storyId"`
			GateName  string `json:"gateName"`
			Tag       string `json:"tag"`
			Q         string `json:"q"`
			Limit     int    `json:"limit"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "Invalid params: " + err.Error()})
			return
		}
		q := memory.Query{
			Kind:      memory.Kind(strings.ToLower(args.Kind)),
			ProjectID: args.ProjectID,
			UserID:    args.UserID,
			StoryID:   args.StoryID,
			GateName:  args.GateName,
			Tag:       args.Tag,
			Substring: args.Q,
			Limit:     args.Limit,
		}
		if q.Limit > 200 {
			q.Limit = 200
		}
		// Ownership: project-scoped requires access; user-scoped must match caller.
		if q.ProjectID != "" {
			p, err := a.d.Projects.Get(q.ProjectID)
			if err != nil || !p.IsAccessibleBy(userID) {
				a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "project not found or not accessible"})
				return
			}
		} else if q.Kind == "" && q.UserID == "" {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "at least one of kind / projectId / userId is required"})
			return
		}
		if q.UserID != "" && q.UserID != userID {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "cannot read another user's memory"})
			return
		}
		rows, err := a.d.Memory.Query(ctx, q)
		if err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32000, Message: err.Error()})
			return
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(map[string]any{"records": rows, "count": len(rows)}))

	case "delete_memory":
		if a.d.Memory == nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "memory not configured"})
			return
		}
		var args struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil || args.ID == "" {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "id required"})
			return
		}
		if err := a.d.Memory.Delete(ctx, args.ID); err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32000, Message: err.Error()})
			return
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(map[string]any{"ok": true, "id": args.ID}))

	case "list_audit":
		if a.d.Audit == nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "audit not configured"})
			return
		}
		var args struct {
			ProjectID    string `json:"projectId"`
			Action       string `json:"action"`
			Outcome      string `json:"outcome"`
			SinceRfc3339 string `json:"sinceRfc3339"`
			UntilRfc3339 string `json:"untilRfc3339"`
			Limit        int    `json:"limit"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "Invalid params: " + err.Error()})
			return
		}
		q := audit.Query{
			ProjectID: args.ProjectID,
			Action:    audit.Action(args.Action),
			Outcome:   audit.Outcome(args.Outcome),
			Limit:     args.Limit,
		}
		if args.SinceRfc3339 != "" {
			if t, err := time.Parse(time.RFC3339, args.SinceRfc3339); err == nil {
				q.Since = t
			}
		}
		if args.UntilRfc3339 != "" {
			if t, err := time.Parse(time.RFC3339, args.UntilRfc3339); err == nil {
				q.Until = t
			}
		}
		if q.Limit > 1000 {
			q.Limit = 1000
		}
		// Ownership: project-scoped requires access; otherwise scope to caller.
		if q.ProjectID != "" {
			p, err := a.d.Projects.Get(q.ProjectID)
			if err != nil || !p.IsAccessibleBy(userID) {
				a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "project not found or not accessible"})
				return
			}
		} else {
			q.UserID = userID
		}
		rows, err := a.d.Audit.Query(ctx, q)
		if err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32000, Message: err.Error()})
			return
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(map[string]any{"entries": rows, "count": len(rows)}))

	case "verify_audit":
		if a.d.Audit == nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "audit not configured"})
			return
		}
		idx, err := a.d.Audit.Verify(ctx)
		if err != nil {
			a.mcpReply(w, req.ID, nil, &mcpError{Code: -32000, Message: err.Error()})
			return
		}
		a.mcpToolResult(w, req.ID, mcpJSONContent(map[string]any{
			"intact":        idx < 0,
			"firstBadIndex": idx,
		}))

	default:
		a.mcpReply(w, req.ID, nil, &mcpError{Code: -32601, Message: "Unknown tool: " + call.Name})
	}
}

// mcpToolResult wraps a tool's content in the MCP CallToolResult envelope.
// content is a slice of content blocks per spec — usually one text block
// carrying JSON, but multi-block responses are allowed.
func (a *API) mcpToolResult(w http.ResponseWriter, id json.RawMessage, content []map[string]any) {
	a.mcpReply(w, id, map[string]any{
		"content": content,
		"isError": false,
	}, nil)
}

// mcpJSONContent turns an arbitrary Go value into a single MCP "text"
// content block with the JSON payload. Clients can re-parse, or simply
// show it verbatim.
func mcpJSONContent(v any) []map[string]any {
	body, _ := json.MarshalIndent(v, "", "  ")
	return []map[string]any{{
		"type": "text",
		"text": string(body),
	}}
}

func mcpProjectSummary(p domain.Project) map[string]any {
	gates := make(map[string]string, len(p.Gates))
	for k, g := range p.Gates {
		gates[string(k)] = string(g.Status)
	}
	return map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"status":      p.Status,
		"owner":       p.OwnerID,
		"gates":       gates,
		"updatedAt":   p.UpdatedAt,
	}
}

// mcpResourcesList enumerates each accessible project as an MCP resource.
// Resource URIs use the scheme `ironflyer-project://<id>` so any MCP
// client can later READ them via the same URI.
func (a *API) mcpResourcesList(w http.ResponseWriter, r *http.Request, req mcpRPC) {
	userID := userIDFromCtx(r)
	all := a.d.Projects.List()
	out := make([]map[string]any, 0, len(all))
	for _, p := range all {
		if !p.IsAccessibleBy(userID) {
			continue
		}
		out = append(out, map[string]any{
			"uri":         "ironflyer-project://" + p.ID,
			"name":        p.Name,
			"description": p.Description,
			"mimeType":    "application/json",
		})
	}
	a.mcpReply(w, req.ID, map[string]any{"resources": out}, nil)
}

// mcpResourcesRead returns the JSON-rendered project at the given URI.
func (a *API) mcpResourcesRead(w http.ResponseWriter, r *http.Request, req mcpRPC) {
	var args struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &args); err != nil || args.URI == "" {
		a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "uri required"})
		return
	}
	const prefix = "ironflyer-project://"
	if !strings.HasPrefix(args.URI, prefix) {
		a.mcpReply(w, req.ID, nil, &mcpError{Code: -32602, Message: "unsupported resource URI"})
		return
	}
	id := strings.TrimPrefix(args.URI, prefix)
	p, err := a.d.Projects.Get(id)
	if err != nil || !p.IsAccessibleBy(userIDFromCtx(r)) {
		a.mcpReply(w, req.ID, nil, &mcpError{Code: -32001, Message: "resource not found"})
		return
	}
	body, _ := json.MarshalIndent(p, "", "  ")
	a.mcpReply(w, req.ID, map[string]any{
		"contents": []map[string]any{{
			"uri":      args.URI,
			"mimeType": "application/json",
			"text":     string(body),
		}},
	}, nil)
}

// RegisterMCP wires the /mcp endpoint onto a chi router. Caller decides
// whether to put it under authMiddleware (recommended) or expose it
// anonymously (only public projects will be visible).
func (a *API) RegisterMCP(r chi.Router) {
	r.Post("/mcp", a.mcpHandle)
	// GET for the SSE/Streamable side of the spec — we serve the same
	// envelope semantics via query-string in the future. For now, 405 is
	// honest: clients should POST JSON-RPC.
	r.Get("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", "POST")
		http.Error(w, "MCP transport: POST JSON-RPC requests to this URL", http.StatusMethodNotAllowed)
	})
}

// silence unused warning when compiling subsets — fmt is referenced
// inside the error message helpers.
var _ = fmt.Sprintf
