// Package context7 wires the public Context7 MCP server into the
// orchestrator out of the box. Context7 ships fresh library
// documentation (npm, Go modules, PyPI, etc.) so the Coder always
// sees current API shapes instead of a year-old training cutoff.
//
// Wired two ways:
//  1. As an outbound MCP server registered with our MCPClientRegistry
//     under the name "context7" — the Coder tool catalogue auto-picks
//     up `context7.resolve-library-id`, `context7.get-library-docs`,
//     etc., next time tools/list runs.
//  2. As a higher-level builtin tool `lookup_docs(library, query)`
//     that hides the two-step MCP dance behind a single function the
//     Coder is more likely to invoke (resolve-then-fetch).
package context7

import (
	"ironflyer/apps/orchestrator/internal/providers"
)

// DefaultEndpoint is the public Context7 Streamable HTTP MCP endpoint.
// It is rate-limited but does not require authentication.
const DefaultEndpoint = "https://mcp.context7.com/mcp"

// Name is the prefix used when Context7 tools are surfaced in the
// flattened MCP tool catalogue (e.g. "context7.get-library-docs").
const Name = "context7"

// NewClient returns a configured *providers.MCPClient pointed at the
// public Context7 server. authToken is optional — Context7 is public
// but accepts a bearer token for higher rate limits.
func NewClient(authToken string) *providers.MCPClient {
	auth := ""
	if authToken != "" {
		auth = "Bearer " + authToken
	}
	return &providers.MCPClient{
		Name:          Name,
		Endpoint:      DefaultEndpoint,
		Authorization: auth,
	}
}
