// Package preview exposes a workspace's internal dev server (Vite, Next.js,
// etc.) to a browser iframe via a path-based reverse proxy on the runtime's
// own HTTP server. It also mints short-lived HMAC-signed `?t=...` tokens so
// iframes can authenticate without sending headers.
//
// Routes (mounted by httpapi):
//
//	GET/POST/...  {Prefix}/{workspaceID}/{port}/*        → container:port/*
//	GET           ws[s]:.../{Prefix}/{workspaceID}/{port}/* (Upgrade: websocket)
//
// The proxy is fully streaming — no body buffering — and pumps WebSocket
// upgrade requests (Vite HMR, Next.js dev) by hijacking the response and
// shuttling raw bytes both ways.
package preview

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TargetResolver maps (workspaceID, internalPort) to a dial target the
// proxy will reach. The string is either "host:port" or a full URL with
// scheme. Implemented by the sandbox driver.
type TargetResolver interface {
	PreviewTarget(ctx context.Context, workspaceID string, port int) (string, error)
}

// Authorizer checks that the bearer of a preview token may access the
// named workspace. The httpapi layer passes through the JWT user (if any)
// plus the token signer; we re-verify both because iframes can't send
// Authorization headers and we don't want to leak workspace IDs to anyone
// who knows the URL pattern.
type Authorizer interface {
	// AllowPreview returns nil when the request is authorised. Implementers
	// should check signed `?t=...` tokens, fall back to JWT cookies/bearer
	// where present, and ultimately decide based on workspace ownership.
	AllowPreview(r *http.Request, workspaceID string) error
}

// Logger is the minimal log surface we depend on (zerolog-compatible call
// sites). We avoid importing zerolog directly so this package stays
// reusable in tests.
type Logger interface {
	Warnf(format string, args ...any)
	Infof(format string, args ...any)
}

// Proxy is the live-preview reverse proxy and token mint.
type Proxy struct {
	Prefix       string
	AllowedPorts map[int]bool // nil or empty => block-all; "*" expressed as nil + WildcardAllowed
	Wildcard     bool
	Targets      TargetResolver
	Authz        Authorizer
	Signer       *TokenSigner
	Log          Logger
}

// Config controls Proxy construction. AllowedPorts is the comma-separated
// list from runtime config — "*" means "no allowlist".
type Config struct {
	Prefix       string
	AllowedPorts string
	TokenSecret  []byte
	TokenTTL     time.Duration
}

// ParseAllowedPorts turns "3000,5173,8080" (or "*") into a port set.
func ParseAllowedPorts(s string) (set map[int]bool, wildcard bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	if s == "*" {
		return nil, true
	}
	out := make(map[int]bool)
	for _, raw := range strings.Split(s, ",") {
		p, err := strconv.Atoi(strings.TrimSpace(raw))
		if err == nil && p > 0 && p < 65536 {
			out[p] = true
		}
	}
	return out, false
}

// New wires a Proxy with the given resolver/authorizer/signer. The Prefix
// must start with "/" and not end with "/"; we normalise it here.
func New(cfg Config, resolver TargetResolver, authz Authorizer, signer *TokenSigner, log Logger) *Proxy {
	prefix := strings.TrimRight(cfg.Prefix, "/")
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	set, wild := ParseAllowedPorts(cfg.AllowedPorts)
	return &Proxy{
		Prefix:       prefix,
		AllowedPorts: set,
		Wildcard:     wild,
		Targets:      resolver,
		Authz:        authz,
		Signer:       signer,
		Log:          log,
	}
}

// PortAllowed reports whether the configured allowlist permits port p.
func (p *Proxy) PortAllowed(port int) bool {
	if p.Wildcard {
		return true
	}
	return p.AllowedPorts[port]
}

// HealthCheck probes the workspace's dev server at workspaceID:port. It
// resolves the upstream via the same TargetResolver the proxy uses (so a
// docker driver and a mock driver are treated uniformly), then issues a
// short-deadline HTTP request to "/". Any 2xx-4xx response is "alive";
// connection refused / DNS error / timeout is "not alive". 5xx counts as
// alive (the server IS responding, just unhappy) so a flaky app doesn't
// disappear from the preview list. Returns (latencyMs, true) on a
// successful probe and (0, false) otherwise.
func (p *Proxy) HealthCheck(ctx context.Context, workspaceID string, port int) (int64, bool) {
	probeCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	target, err := p.Targets.PreviewTarget(probeCtx, workspaceID, port)
	if err != nil {
		return 0, false
	}
	dst, scheme := normaliseTarget(target)
	url := scheme + "://" + dst + "/"
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("User-Agent", "Ironflyer-PreviewHealth/1.0")
	// We don't want keep-alives polluting the runtime's pool just for a
	// liveness probe — use a one-shot transport with a small dial deadline.
	client := &http.Client{
		Timeout: 1500 * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives:   true,
			DialContext:         (&net.Dialer{Timeout: 700 * time.Millisecond}).DialContext,
			MaxIdleConns:        1,
			IdleConnTimeout:     500 * time.Millisecond,
			TLSHandshakeTimeout: 700 * time.Millisecond,
		},
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		// Server is reachable but broken — still "alive" for our purposes.
		return time.Since(start).Milliseconds(), true
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return time.Since(start).Milliseconds(), true
	}
	return 0, false
}

// BuildPreviewPath returns the public path (no host) for a workspace+port
// + optional sub-path. Always begins with the configured prefix.
func (p *Proxy) BuildPreviewPath(workspaceID string, port int, sub string) string {
	sub = strings.TrimPrefix(sub, "/")
	if sub == "" {
		return fmt.Sprintf("%s/%s/%d/", p.Prefix, workspaceID, port)
	}
	return fmt.Sprintf("%s/%s/%d/%s", p.Prefix, workspaceID, port, sub)
}

// ServeHTTP implements http.Handler — call it for any request whose path
// starts with the configured prefix. We parse the path, validate the
// port, check authorisation, then either proxy HTTP or hijack for WS.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsID, port, rest, ok := p.parsePath(r.URL.Path)
	if !ok {
		http.Error(w, "preview: bad URL shape", http.StatusBadRequest)
		return
	}
	if !p.PortAllowed(port) {
		http.Error(w, "preview: port not allowed", http.StatusForbidden)
		return
	}
	if err := p.Authz.AllowPreview(r, wsID); err != nil {
		http.Error(w, "preview: "+err.Error(), http.StatusUnauthorized)
		return
	}
	target, err := p.Targets.PreviewTarget(r.Context(), wsID, port)
	if err != nil {
		http.Error(w, "preview: "+err.Error(), http.StatusBadGateway)
		return
	}
	dst, scheme := normaliseTarget(target)

	// WebSocket upgrade? Hijack and tunnel raw bytes.
	if isWebSocketUpgrade(r) {
		p.proxyWebSocket(w, r, dst, wsID, port, rest)
		return
	}

	// Plain HTTP / SSE / streaming. NewSingleHostReverseProxy gives us
	// flushing for free and we customise the Director to rewrite paths.
	upstream := &url.URL{Scheme: scheme, Host: dst}
	rp := httputil.NewSingleHostReverseProxy(upstream)
	rp.FlushInterval = -1 // immediate flush; required for SSE + HMR over poll

	origDirector := rp.Director
	rp.Director = func(req *http.Request) {
		origDirector(req)
		req.URL.Path = "/" + rest
		req.URL.RawPath = ""
		// Pass through query string as-is.
		req.Host = dst
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", forwardedProto(r))
		if ip, _, splitErr := net.SplitHostPort(r.RemoteAddr); splitErr == nil {
			appendXFF(req.Header, ip)
		} else {
			appendXFF(req.Header, r.RemoteAddr)
		}
		req.Header.Set("X-Ironflyer-Workspace", wsID)
		req.Header.Set("X-Ironflyer-Port", strconv.Itoa(port))
		req.Header.Del("Authorization") // never leak runtime auth upstream
	}
	rp.ModifyResponse = func(resp *http.Response) error {
		// Strip cache headers that conflict with HMR.
		resp.Header.Del("Strict-Transport-Security")
		return nil
	}
	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		if p.Log != nil {
			p.Log.Warnf("preview proxy %s:%d → %s: %v", wsID, port, dst, err)
		}
		http.Error(w, "preview upstream: "+err.Error(), http.StatusBadGateway)
	}
	rp.ServeHTTP(w, r)
}

// parsePath validates `{prefix}/{workspaceID}/{port}[/sub...]`.
func (p *Proxy) parsePath(path string) (workspaceID string, port int, rest string, ok bool) {
	if !strings.HasPrefix(path, p.Prefix+"/") {
		return "", 0, "", false
	}
	tail := strings.TrimPrefix(path, p.Prefix+"/")
	parts := strings.SplitN(tail, "/", 3)
	if len(parts) < 2 {
		return "", 0, "", false
	}
	wsID := parts[0]
	pn, err := strconv.Atoi(parts[1])
	if err != nil || pn <= 0 || pn > 65535 {
		return "", 0, "", false
	}
	sub := ""
	if len(parts) == 3 {
		sub = parts[2]
	}
	if wsID == "" || strings.ContainsAny(wsID, "/?#") {
		return "", 0, "", false
	}
	return wsID, pn, sub, true
}

func normaliseTarget(t string) (hostPort, scheme string) {
	if strings.HasPrefix(t, "http://") {
		return strings.TrimPrefix(t, "http://"), "http"
	}
	if strings.HasPrefix(t, "https://") {
		return strings.TrimPrefix(t, "https://"), "https"
	}
	return t, "http"
}

func forwardedProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if v := r.Header.Get("X-Forwarded-Proto"); v != "" {
		return v
	}
	return "http"
}

func appendXFF(h http.Header, ip string) {
	if existing := h.Get("X-Forwarded-For"); existing != "" {
		h.Set("X-Forwarded-For", existing+", "+ip)
		return
	}
	h.Set("X-Forwarded-For", ip)
}

func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	for _, v := range r.Header.Values("Connection") {
		if strings.Contains(strings.ToLower(v), "upgrade") {
			return true
		}
	}
	return false
}

// proxyWebSocket dials the upstream, replays the client's request, then
// shuttles raw bytes in both directions until either side closes. We do
// this by hand (rather than httputil.ReverseProxy's WS support, which
// requires Go 1.20+ and a custom Transport) because we need precise
// control over the upgrade headers — Vite/Next.js HMR is picky.
func (p *Proxy) proxyWebSocket(w http.ResponseWriter, r *http.Request, dst, wsID string, port int, rest string) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "preview: server does not support hijacking", http.StatusInternalServerError)
		return
	}
	upstream, err := net.DialTimeout("tcp", dst, 10*time.Second)
	if err != nil {
		http.Error(w, "preview: dial upstream: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer upstream.Close()

	// Rewrite request line: /preview/{ws}/{port}/foo  →  /foo
	upstreamPath := "/" + rest
	if r.URL.RawQuery != "" {
		upstreamPath += "?" + r.URL.RawQuery
	}

	// Build a fresh request to send upstream so we can fully control the
	// headers (Connection/Upgrade/Sec-WebSocket-* must be preserved).
	upReq, _ := http.NewRequest(r.Method, upstreamPath, nil)
	upReq.Host = dst
	for k, vs := range r.Header {
		// Skip hop-by-hop headers except the upgrade trio which must
		// reach the upstream verbatim.
		if isHopByHop(k) && !isUpgradeHeader(k) {
			continue
		}
		for _, v := range vs {
			upReq.Header.Add(k, v)
		}
	}
	upReq.Header.Set("X-Forwarded-Host", r.Host)
	upReq.Header.Set("X-Forwarded-Proto", forwardedProto(r))
	if ip, _, splitErr := net.SplitHostPort(r.RemoteAddr); splitErr == nil {
		appendXFF(upReq.Header, ip)
	}
	upReq.Header.Set("X-Ironflyer-Workspace", wsID)
	upReq.Header.Set("X-Ironflyer-Port", strconv.Itoa(port))
	upReq.Header.Del("Authorization")

	if err := upReq.Write(upstream); err != nil {
		http.Error(w, "preview: write upstream: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Read upstream's response (should be 101 Switching Protocols).
	br := bufio.NewReader(upstream)
	resp, err := http.ReadResponse(br, upReq)
	if err != nil {
		http.Error(w, "preview: read upstream: "+err.Error(), http.StatusBadGateway)
		return
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		// Forward the non-101 response as-is.
		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		_ = resp.Body.Close()
		return
	}

	// Hijack the client side and stream both directions.
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "preview: hijack: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	if err := writeSwitchingResponse(clientBuf, resp); err != nil {
		return
	}
	if err := clientBuf.Flush(); err != nil {
		return
	}

	// If the upstream's bufio had any pre-read bytes, drain them first
	// so we don't lose the first WS frame.
	if n := br.Buffered(); n > 0 {
		pre, _ := br.Peek(n)
		if _, err := clientConn.Write(pre); err != nil {
			return
		}
		_, _ = br.Discard(n)
	}

	// Bi-directional copy. Either side closing tears down the whole thing.
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, clientConn); done <- struct{}{} }()
	go func() { _, _ = io.Copy(clientConn, upstream); done <- struct{}{} }()
	<-done
}

func writeSwitchingResponse(w io.Writer, resp *http.Response) error {
	if _, err := fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", resp.StatusCode, resp.Status); err != nil {
		return err
	}
	for k, vs := range resp.Header {
		for _, v := range vs {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}
	_, err := io.WriteString(w, "\r\n")
	return err
}

var hopByHop = map[string]bool{
	"Connection":          true,
	"Proxy-Connection":    true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	// Upgrade is handled specially.
}

func isHopByHop(name string) bool { return hopByHop[http.CanonicalHeaderKey(name)] }

func isUpgradeHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Connection", "Upgrade",
		"Sec-Websocket-Key", "Sec-Websocket-Version",
		"Sec-Websocket-Protocol", "Sec-Websocket-Extensions",
		"Sec-Websocket-Accept":
		return true
	}
	return false
}

func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

// --------------------------------------------------------------------------
// Signed `?t=...` tokens. Encoded as base64url(payloadJSON) "." base64url(hmac).
// Payload: {ws, port, exp}. HMAC-SHA256 over the payload bytes with Secret.
// --------------------------------------------------------------------------

// TokenSigner mints and verifies short-lived preview tokens.
type TokenSigner struct {
	secret []byte
	ttl    time.Duration
}

// NewSigner builds a signer. When secret is empty, a 32-byte random secret
// is generated in-process (dev-only; tokens won't survive restart).
func NewSigner(secret []byte, ttl time.Duration) *TokenSigner {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if len(secret) == 0 {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			// Fall back to a stable but obviously-dev secret if /dev/urandom
			// fails. Should never happen in practice.
			buf = []byte("ironflyer-dev-fallback-secret-32b")
		}
		secret = buf
	}
	return &TokenSigner{secret: secret, ttl: ttl}
}

// TTL returns the token lifetime as configured.
func (s *TokenSigner) TTL() time.Duration { return s.ttl }

type tokenPayload struct {
	Workspace string `json:"ws"`
	Port      int    `json:"port"`
	Exp       int64  `json:"exp"`
}

// Mint returns a fresh token with the signer's default TTL.
func (s *TokenSigner) Mint(workspaceID string, port int) (token string, exp time.Time, err error) {
	return s.MintWithTTL(workspaceID, port, s.ttl)
}

// MintWithTTL returns a fresh token with an explicit lifetime. Used by
// the share-link endpoint to issue long-lived tokens (default sessions
// stay short-lived at 30 min; share links can run for days).
func (s *TokenSigner) MintWithTTL(workspaceID string, port int, ttl time.Duration) (token string, exp time.Time, err error) {
	if ttl <= 0 {
		ttl = s.ttl
	}
	exp = time.Now().Add(ttl).UTC()
	payload := tokenPayload{Workspace: workspaceID, Port: port, Exp: exp.Unix()}
	body := marshalJSONCompact(payload)
	sig := hmacSHA256(s.secret, body)
	token = base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig)
	return token, exp, nil
}

// Verify checks the signature and expiry, and that the token names the
// given workspace+port (port may be 0 to skip the port check).
func (s *TokenSigner) Verify(token, workspaceID string, port int) error {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return errors.New("malformed token")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("bad token payload")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("bad token signature")
	}
	expected := hmacSHA256(s.secret, body)
	if !hmac.Equal(sig, expected) {
		return errors.New("token signature mismatch")
	}
	var p tokenPayload
	if err := unmarshalJSON(body, &p); err != nil {
		return errors.New("bad token payload json")
	}
	if p.Workspace != workspaceID {
		return errors.New("token workspace mismatch")
	}
	if port != 0 && p.Port != port {
		return errors.New("token port mismatch")
	}
	if time.Now().Unix() > p.Exp {
		return errors.New("token expired")
	}
	return nil
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// FingerprintSecret returns a short hex prefix of SHA-256(secret) for log
// lines so operators can spot when the runtime is using a fresh dev
// secret without exposing the secret itself.
func (s *TokenSigner) FingerprintSecret() string {
	sum := sha256.Sum256(s.secret)
	return hex.EncodeToString(sum[:4])
}

// --------------------------------------------------------------------------
// Tiny JSON helpers (no extra deps). encoding/json is fine but using it
// directly here keeps the package surface small and avoids a circular
// pull when callers vendor it.
// --------------------------------------------------------------------------

var _ = sync.Mutex{} // silence unused-import potential when refactoring

// We intentionally use encoding/json via these wrappers so that swapping
// to a hand-rolled encoder later doesn't ripple through callers.
func marshalJSONCompact(v any) []byte {
	b, _ := jsonMarshal(v)
	return b
}

func unmarshalJSON(data []byte, v any) error { return jsonUnmarshal(data, v) }
