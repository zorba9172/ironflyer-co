// lookup_docs built-in tool wiring.
//
// The Coder is most useful when it has access to documentation that
// is fresher than its training cutoff. Context7's MCP server already
// exposes two primitives — `resolve-library-id` and `get-library-docs`
// — but chaining them requires two tool_use round-trips. This file
// exposes a single `lookup_docs(library, query?)` built-in that
// performs the resolve-then-fetch internally and returns the docs
// body verbatim. The Coder still sees the underlying Context7 tools
// via the MCP registry, but the named shortcut is what the prompt
// nudges it towards.
package context7

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ironflyer/apps/orchestrator/internal/providers"
)

// Tool wraps an MCPClient with a convenience two-step resolve+fetch
// that's friendlier than asking the Coder to chain two MCP calls.
type Tool struct {
	Client *providers.MCPClient
}

// Spec is the providers.ToolSpec the agents registry hands to the
// model so it knows the shape of lookup_docs. Keep the description
// concrete — every Coder turn pays for these tokens.
func (t *Tool) Spec() providers.ToolSpec {
	return providers.ToolSpec{
		Name:        "lookup_docs",
		Description: "Look up current documentation for a library or package. Returns up-to-date code samples, types, and migration notes — newer than any training data cutoff.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"library": map[string]any{
					"type":        "string",
					"description": "Library/package name, e.g. 'next', '@supabase/ssr', 'react-query'",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "What you want to know — e.g. 'how to set up middleware'",
				},
			},
			"required": []string{"library"},
		},
	}
}

// Call performs the two-step resolve-then-fetch dance against the
// configured Context7 MCP client. Any error from either step is
// converted into a friendly fallback string so the Coder keeps going
// rather than aborting the run when documentation is briefly
// unavailable.
//
// The userBearer + workspaceID arguments are part of the standard
// BuiltinToolFunc signature but unused here — lookup_docs is a pure
// outbound read against a public service.
func (t *Tool) Call(ctx context.Context, userBearer, workspaceID string, args map[string]any) (string, error) {
	if t == nil || t.Client == nil {
		return fallback(errors.New("not configured")), nil
	}
	library := strings.TrimSpace(stringArg(args, "library"))
	if library == "" {
		return "", errors.New("lookup_docs: library is required")
	}
	query := strings.TrimSpace(stringArg(args, "query"))

	// Step 1 — resolve the library name to a Context7-compatible ID.
	resolveRaw, err := t.Client.CallTool(ctx, "resolve-library-id", map[string]any{
		"libraryName": library,
	})
	if err != nil {
		return fallback(fmt.Errorf("resolve-library-id: %w", err)), nil
	}
	libID := parseLibraryID(resolveRaw)
	if libID == "" {
		return fallback(fmt.Errorf("no Context7 match for %q", library)), nil
	}

	// Step 2 — fetch the actual docs. `topic` is optional; passing an
	// empty string would still be valid but we omit it cleanly so the
	// server doesn't see a stray field.
	fetchArgs := map[string]any{
		"context7CompatibleLibraryID": libID,
	}
	if query != "" {
		fetchArgs["topic"] = query
	}
	docs, err := t.Client.CallTool(ctx, "get-library-docs", fetchArgs)
	if err != nil {
		return fallback(fmt.Errorf("get-library-docs: %w", err)), nil
	}
	return docs, nil
}

// fallback formats an error into the friendly degradation message
// the Coder sees as the tool_result. We return it as the (string,
// nil) pair on purpose: the Coder treats it as a soft signal and
// keeps generating, instead of unwinding on the error.
func fallback(err error) string {
	return "lookup_docs unavailable: " + err.Error()
}

// parseLibraryID extracts the first Context7-compatible library ID
// from a resolve-library-id response. Context7 returns a structured
// text body listing matches; the most reliable identifier is the
// line that begins with "- Context7-compatible library ID:" (the
// public format used by the hosted server). We accept several
// surface variants so a small change on Context7's side doesn't
// silently break the lookup.
func parseLibraryID(body string) string {
	// Most common shape — a labelled key/value pair, sometimes
	// preceded by a list-item dash or a stray asterisk for emphasis.
	for _, line := range strings.Split(body, "\n") {
		l := strings.TrimSpace(line)
		l = strings.TrimLeft(l, "-* ")
		// Strip markdown emphasis wrappers around the label, e.g.
		// "**Context7-compatible library ID:** /vercel/next.js".
		l = strings.ReplaceAll(l, "**", "")
		lower := strings.ToLower(l)
		const key = "context7-compatible library id:"
		if idx := strings.Index(lower, key); idx >= 0 {
			rest := strings.TrimSpace(l[idx+len(key):])
			// Trim trailing commentary like " (Trust: 9, Snippets: 1234)".
			if cut := strings.IndexAny(rest, " \t("); cut > 0 {
				rest = rest[:cut]
			}
			rest = strings.TrimSpace(rest)
			if rest != "" {
				return rest
			}
		}
	}
	// Some Context7 builds return JSON; tolerate that too.
	var jsonShape struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &jsonShape); err == nil {
		if len(jsonShape.Results) > 0 && jsonShape.Results[0].ID != "" {
			return jsonShape.Results[0].ID
		}
	}
	return ""
}

func stringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
