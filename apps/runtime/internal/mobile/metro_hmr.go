package mobile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// ProxyMetroHTTP returns an http.Handler that reverse-proxies plain HTTP
// requests to target. target must be a fully-qualified base URL such as
// "http://127.0.0.1:19000".
//
// The proxy is a thin wrapper around httputil.NewSingleHostReverseProxy
// with two tweaks: (1) the request URL is rewritten so the upstream sees
// the original path/query, not the proxy mount path; (2) the Director
// drops the chi-route prefix so /v1/workspaces/{id}/mobile/metro/proxy/foo
// rewrites to /foo on the Metro side.
func ProxyMetroHTTP(target, mountPrefix string) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parse target: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, errors.New("target must include scheme and host")
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	origDirector := rp.Director
	rp.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = u.Host
		if mountPrefix != "" && strings.HasPrefix(req.URL.Path, mountPrefix) {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, mountPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
	}
	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, "metro proxy: "+err.Error(), http.StatusBadGateway)
	}
	return rp, nil
}

// ProxyMetroWS returns an http.Handler that hijacks the incoming HTTP
// connection and shovels bytes both ways against an upstream WebSocket
// target. target must be of the form "host:port" (no scheme); the proxy
// dials a raw TCP connection and replays the upgrade handshake.
//
// This is the implementation pattern used by the koding/websocketproxy
// reference (and the Pion stack) — we inline it here so the runtime
// module doesn't gain a new dependency for a 100-line bridge.
func ProxyMetroWS(targetHostPort, mountPrefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWebSocketUpgrade(r) {
			http.Error(w, "expected WebSocket upgrade", http.StatusBadRequest)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}

		// Dial upstream and forward the original HTTP/1.1 upgrade request
		// line + headers. The HTTP request the runtime received is already
		// HTTP/1.1 (chi rejects HTTP/2 upgrades), so we can re-marshal it
		// byte-for-byte.
		upstream, err := net.DialTimeout("tcp", targetHostPort, 5*time.Second)
		if err != nil {
			http.Error(w, "metro ws dial: "+err.Error(), http.StatusBadGateway)
			return
		}

		// Rewrite Path so the upstream sees /message instead of the proxy
		// mount path.
		path := r.URL.Path
		if mountPrefix != "" && strings.HasPrefix(path, mountPrefix) {
			path = strings.TrimPrefix(path, mountPrefix)
			if path == "" {
				path = "/"
			}
		}
		raw := r.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		// Replay the request line + headers.
		var hdr strings.Builder
		fmt.Fprintf(&hdr, "GET %s HTTP/1.1\r\n", path)
		fmt.Fprintf(&hdr, "Host: %s\r\n", targetHostPort)
		for k, vs := range r.Header {
			// Skip hop-by-hop headers chi's middleware may have stamped.
			if strings.EqualFold(k, "Connection") || strings.EqualFold(k, "Upgrade") ||
				strings.EqualFold(k, "Sec-WebSocket-Version") || strings.EqualFold(k, "Sec-WebSocket-Key") ||
				strings.EqualFold(k, "Sec-WebSocket-Extensions") || strings.EqualFold(k, "Sec-WebSocket-Protocol") {
				continue
			}
			for _, v := range vs {
				fmt.Fprintf(&hdr, "%s: %s\r\n", k, v)
			}
		}
		// The websocket handshake headers must be passed through verbatim.
		fmt.Fprintf(&hdr, "Connection: Upgrade\r\n")
		fmt.Fprintf(&hdr, "Upgrade: websocket\r\n")
		if v := r.Header.Get("Sec-WebSocket-Version"); v != "" {
			fmt.Fprintf(&hdr, "Sec-WebSocket-Version: %s\r\n", v)
		}
		if v := r.Header.Get("Sec-WebSocket-Key"); v != "" {
			fmt.Fprintf(&hdr, "Sec-WebSocket-Key: %s\r\n", v)
		}
		if v := r.Header.Get("Sec-WebSocket-Protocol"); v != "" {
			fmt.Fprintf(&hdr, "Sec-WebSocket-Protocol: %s\r\n", v)
		}
		if v := r.Header.Get("Sec-WebSocket-Extensions"); v != "" {
			fmt.Fprintf(&hdr, "Sec-WebSocket-Extensions: %s\r\n", v)
		}
		hdr.WriteString("\r\n")
		if _, err := upstream.Write([]byte(hdr.String())); err != nil {
			upstream.Close()
			http.Error(w, "metro ws handshake: "+err.Error(), http.StatusBadGateway)
			return
		}

		// Read the upstream upgrade response and forward to the client.
		downstream, dsBuf, err := hijacker.Hijack()
		if err != nil {
			upstream.Close()
			return
		}
		defer downstream.Close()
		defer upstream.Close()

		upBR := bufio.NewReader(upstream)
		// Read response status line + headers.
		respHdr, err := readHTTPResponse(upBR)
		if err != nil {
			_, _ = downstream.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"))
			return
		}
		if _, err := downstream.Write(respHdr); err != nil {
			return
		}
		// Drain anything dsBuf already buffered into upstream first — that's
		// the start of the client's WebSocket payload after the upgrade.
		if dsBuf != nil && dsBuf.Reader != nil {
			if n := dsBuf.Reader.Buffered(); n > 0 {
				peek, _ := dsBuf.Reader.Peek(n)
				if len(peek) > 0 {
					if _, werr := upstream.Write(peek); werr != nil {
						return
					}
					_, _ = dsBuf.Reader.Discard(n)
				}
			}
		}

		errc := make(chan error, 2)
		go func() { _, err := io.Copy(upstream, downstream); errc <- err }()
		go func() { _, err := io.Copy(downstream, upBR); errc <- err }()
		<-errc
	})
}

// isWebSocketUpgrade reports whether r looks like a WebSocket upgrade.
// Both header values are case-insensitive per RFC 6455.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// readHTTPResponse reads a complete HTTP/1.1 response header block (up to
// and including the terminating CRLFCRLF) from br and returns the raw
// bytes. We don't parse the body — the caller will shovel bytes raw.
func readHTTPResponse(br *bufio.Reader) ([]byte, error) {
	var buf strings.Builder
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		buf.WriteString(line)
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	return []byte(buf.String()), nil
}
