package gqlhardening

import (
	"net/http"
	"net/url"
	"strings"
)

// OriginAllow returns a CheckOrigin closure suitable for the gorilla
// websocket Upgrader. It accepts an origin when:
//
//   - the allowlist is empty (development default — every origin is
//     allowed; the chi CORS layer in front already covers HTTP), or
//   - the request's Origin header host matches one of the allowlisted
//     entries exactly (scheme+host comparison; port included).
//
// Subdomain wildcards are not supported on purpose — every allowed
// origin must be enumerated explicitly so a misconfigured cloud
// provider doesn't accidentally trust *.googleusercontent.com.
func OriginAllow(allowlist []string) func(*http.Request) bool {
	allowed := normalizeAllowlist(allowlist)
	return func(r *http.Request) bool {
		if len(allowed) == 0 {
			return true
		}
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			// Same-origin requests omit Origin; allow them through —
			// the auth middleware still gates the actual upgrade.
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			originRejects.Inc()
			return false
		}
		needle := strings.ToLower(u.Scheme + "://" + u.Host)
		if _, ok := allowed[needle]; ok {
			return true
		}
		originRejects.Inc()
		return false
	}
}

func normalizeAllowlist(raw []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, entry := range raw {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		// Allow callers to pass bare hosts ("app.ironflyer.dev"); we
		// canonicalize to https:// because that's the only scheme the
		// production WS endpoint serves.
		if !strings.Contains(entry, "://") {
			entry = "https://" + entry
		}
		out[entry] = struct{}{}
	}
	return out
}
