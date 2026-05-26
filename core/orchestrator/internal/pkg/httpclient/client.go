// Package httpclient is the single factory the orchestrator should
// reach for whenever it needs an *http.Client. Standard is for short,
// bounded outbound calls (webhooks, REST APIs, batch endpoints).
// Streaming returns a tuned client for long-poll / SSE callers; the
// tuned transport is registered by internal/providers at init time
// (via SetStreamingTransport) so this package stays a leaf with no
// dependency on providers, avoiding import cycles with the many
// packages (events, profitguard, outboxhooks, ...) that providers
// transitively pulls in.
//
// All previously-tracked ad-hoc &http.Client{Timeout: ...} sites
// (deploy/vercel, deploy/domain_providers, business/budget/{stripe,
// metered,payments/paddle}, mobile/{appetize,eas,devicecloud/
// browserstack}, finisher/gates_lighthouse, policy/opa_remote,
// operations/runtime/client, ai/providers/{vercel_ai,openai,
// huggingface,mcp_client}, and runtime/suppliers/mobile/bridge) have
// been migrated to Standard / Streaming. Future ad-hoc sites should
// fail review unless they carry a bespoke Transport (TLS overrides,
// custom dialers) explained in a // custom transport — not migrated
// comment.
package httpclient

import (
	"net/http"
	"sync/atomic"
	"time"
)

// streamingTransport is the tuned transport registered by
// internal/providers. Until SetStreamingTransport is called Streaming
// falls back to http.DefaultTransport.
var streamingTransport atomic.Pointer[http.Transport]

// SetStreamingTransport registers the tuned LLM streaming transport.
// internal/providers calls this from an init hook so Streaming reuses
// the same connection pool as the LLM streaming path without this
// package needing to import providers.
func SetStreamingTransport(t *http.Transport) {
	streamingTransport.Store(t)
}

// Standard returns a fresh *http.Client with a hard Timeout. Use for
// any short, bounded outbound request: webhook calls, REST APIs, batch
// endpoints.
func Standard(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// Streaming returns an *http.Client backed by the tuned transport
// (when registered) or http.DefaultTransport. The client deliberately
// has no global Timeout — SSE / long-poll callers bound the request
// with a context instead.
func Streaming() *http.Client {
	if t := streamingTransport.Load(); t != nil {
		return &http.Client{Transport: t}
	}
	return &http.Client{Transport: http.DefaultTransport}
}
