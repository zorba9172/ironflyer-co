package httpapi

import "net/http"

// SecurityHeadersOptions controls the set of headers the middleware
// stamps on every response. ProdMode flips HSTS on and selects the
// strict CSP default; CSPOverride replaces the default policy when
// non-empty.
type SecurityHeadersOptions struct {
	ProdMode    bool
	CSPOverride string
}

const (
	defaultCSPProd = "default-src 'self'; " +
		"connect-src 'self'; " +
		"img-src 'self' data: blob:; " +
		"style-src 'self' 'unsafe-inline'; " +
		"script-src 'self'; " +
		"font-src 'self' data:; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"
	// Dev allows the Apollo Sandbox bundle hosted on
	// embeddable-sandbox.cdn.apollographql.com.
	defaultCSPDev = "default-src 'self' https: data: blob:; " +
		"connect-src 'self' https: ws: wss:; " +
		"img-src 'self' https: data: blob:; " +
		"style-src 'self' 'unsafe-inline' https:; " +
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' https:; " +
		"font-src 'self' https: data:; " +
		"frame-ancestors 'self'"
)

func securityHeadersMiddleware(opts SecurityHeadersOptions) func(http.Handler) http.Handler {
	csp := opts.CSPOverride
	if csp == "" {
		if opts.ProdMode {
			csp = defaultCSPProd
		} else {
			csp = defaultCSPDev
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isProbeRoute(r.URL.Path) {
				h := w.Header()
				h.Set("X-Frame-Options", "DENY")
				h.Set("X-Content-Type-Options", "nosniff")
				h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
				h.Set("Content-Security-Policy", csp)
				if opts.ProdMode {
					h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isProbeRoute(path string) bool {
	switch path {
	case "/healthz", "/livez", "/readyz":
		return true
	}
	return false
}
