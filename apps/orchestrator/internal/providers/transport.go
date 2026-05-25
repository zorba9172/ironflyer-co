package providers

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	streamTransportOnce sync.Once
	streamTransport     *http.Transport
)

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
func streamingHTTPClient() *http.Client {
	streamTransportOnce.Do(func() {
		streamTransport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          intFromEnv("PROVIDER_MAX_IDLE_CONNS", defaultMaxIdleConns),
			MaxIdleConnsPerHost:   intFromEnv("PROVIDER_MAX_IDLE_CONNS_PER_HOST", defaultMaxIdleConnsPerHost),
			MaxConnsPerHost:       intFromEnv("PROVIDER_MAX_CONNS_PER_HOST", defaultMaxConnsPerHost),
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		}
	})
	return &http.Client{Transport: streamTransport}
}

// intFromEnv reads `name` as a positive integer; falls back to `def`
// on empty / unparseable / non-positive values. Keeping the parsing
// permissive avoids a panic on a typo in operator env config.
func intFromEnv(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
