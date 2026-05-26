package providers

import (
	"net"
	"net/http"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/pkg/env"
	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// init wires the tuned streaming transport into the leaf httpclient
// package so any caller of httpclient.Streaming() gets the LLM-tuned
// pool without having to import providers (which would form a cycle
// for events / profitguard / outboxhooks call sites).
func init() {
	httpclient.SetStreamingTransport(StreamingTransport())
}

var (
	streamTransportOnce sync.Once
	streamTransport     *http.Transport
	streamClientOnce    sync.Once
	streamClient        *http.Client
)

// StreamingTransport returns the shared, lazily-initialised
// *http.Transport tuned for long-lived streaming traffic. Exposed so
// that internal/pkg/httpclient can compose it without duplicating the
// pool tuning and timeouts below.
func StreamingTransport() *http.Transport {
	streamTransportOnce.Do(buildStreamTransport)
	return streamTransport
}

func buildStreamTransport() {
	streamTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          env.Int("PROVIDER_MAX_IDLE_CONNS", defaultMaxIdleConns),
		MaxIdleConnsPerHost:   env.Int("PROVIDER_MAX_IDLE_CONNS_PER_HOST", defaultMaxIdleConnsPerHost),
		MaxConnsPerHost:       env.Int("PROVIDER_MAX_CONNS_PER_HOST", defaultMaxConnsPerHost),
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
}

// Connection-pool defaults tuned for production-scale LLM traffic. At
// 30 orchestrator pods × 200 max idle conns × 50 per host we cap at
// ~10k sockets per host across the fleet — comfortably under any
// provider's documented per-account ceiling and the OS socket budget.
//
// Each value is overrideable via env so operators can dial these up
// without a rebuild when a region's traffic doubles overnight:
//
//   PROVIDER_MAX_IDLE_CONNS          → MaxIdleConns          (default 200)
//   PROVIDER_MAX_IDLE_CONNS_PER_HOST → MaxIdleConnsPerHost   (default 50)
//   PROVIDER_MAX_CONNS_PER_HOST      → MaxConnsPerHost       (default 200)
const (
	defaultMaxIdleConns        = 200
	defaultMaxIdleConnsPerHost = 50
	defaultMaxConnsPerHost     = 200
)

// streamingHTTPClient returns a shared *http.Client suitable for
// streaming LLM providers. The client has no global Timeout (which would
// kill long SSE streams); instead the underlying transport bounds the
// pre-stream phases — dial, TLS handshake, and the wait for response
// headers — while letting the body run as long as the caller's context
// allows. ResponseHeaderTimeout is the critical guard: it caps how long
// a hung upstream can stall us before the first byte of headers.
//
// The *http.Client is built once and reused — every provider that
// instantiates a fresh stream was previously allocating a wrapper
// struct (and dragging the GC through a tiny per-request object) for
// no behaviour difference.
func streamingHTTPClient() *http.Client {
	streamClientOnce.Do(func() {
		streamClient = &http.Client{Transport: StreamingTransport()}
	})
	return streamClient
}
