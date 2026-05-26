// Mirror of core/orchestrator/internal/pkg/httpclient — kept in sync manually; the two-module layout precludes direct sharing.
//
// Package httpclient is the single factory the runtime should reach
// for when it needs an *http.Client. The runtime has no LLM streaming
// pool of its own, so Streaming returns a generic no-timeout client
// suitable for long-poll / SSE callers; the transport tuning lives
// alongside the orchestrator's providers package.
package httpclient

import (
	"net/http"
	"time"
)

// Standard returns a fresh *http.Client with a hard Timeout.
func Standard(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// Streaming returns an *http.Client with no global Timeout. SSE /
// long-poll callers should bound the request with a context instead.
func Streaming() *http.Client {
	return &http.Client{Transport: http.DefaultTransport}
}
