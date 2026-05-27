package gqlhardening

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"
)

// CSRFMiddleware implements the double-submit cookie pattern over the
// orchestrator's GraphQL POST endpoint. The flow:
//
//  1. GET /graphql (or any GraphQL handshake) issues a random token
//     cookie scoped to the orchestrator's domain.
//  2. Every state-changing POST must echo that token in the configured
//     header. The server compares header + cookie in constant time;
//     non-match → 403 with code CSRF_INVALID.
//
// The middleware is a no-op for unauthenticated requests (no session
// cookie present) and for any request carrying a Bearer token —
// bearer-auth APIs (CLI, SDK, VSCode extension) are not vulnerable to
// the browser-cookie attack class so they skip the CSRF dance.
//
// The cookie is HttpOnly:false on purpose — the client JS needs to
// read it to copy it into the header. It is Secure + SameSite=Lax so
// it still resists cross-site form submission attacks.
type CSRFOptions struct {
	Header     string
	Cookie     string
	CookiePath string
	Domain     string
	Secure     bool
	MaxAge     time.Duration
}

// DefaultCSRFOptions returns the production-safe defaults. Callers
// override the cookie domain at the call site so the cookie scope
// matches the deployed origin.
func DefaultCSRFOptions(cfg Config) CSRFOptions {
	header := cfg.CSRFHeader
	if header == "" {
		header = "X-Ironflyer-CSRF"
	}
	cookie := cfg.CSRFCookie
	if cookie == "" {
		cookie = "ironflyer_csrf"
	}
	return CSRFOptions{
		Header:     header,
		Cookie:     cookie,
		CookiePath: "/",
		Domain:     cfg.CSRFCookieDomain,
		Secure:     cfg.ProdMode,
		MaxAge:     12 * time.Hour,
	}
}

// CSRFMiddleware is the chi-compatible middleware. It only enforces
// on POST + PUT + DELETE; reads (GET) are responsible only for
// issuing the cookie when it's missing.
func CSRFMiddleware(opts CSRFOptions) func(http.Handler) http.Handler {
	if opts.Header == "" {
		opts.Header = "X-Ironflyer-CSRF"
	}
	if opts.Cookie == "" {
		opts.Cookie = "ironflyer_csrf"
	}
	if opts.CookiePath == "" {
		opts.CookiePath = "/"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always ensure the cookie is present so the SPA can
			// pre-stage the header on its first POST.
			if c, _ := r.Cookie(opts.Cookie); c == nil {
				_ = issueCSRFCookie(w, opts)
			}
			if !mutates(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			// Bearer-auth requests are exempt — the cookie attack class
			// only applies when the browser auto-attaches credentials.
			if hasBearer(r) {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(opts.Cookie)
			if err != nil || cookie == nil || cookie.Value == "" {
				csrfRejects.Inc()
				writePersistedError(w, http.StatusForbidden, "CSRF_MISSING", "missing CSRF cookie")
				return
			}
			header := strings.TrimSpace(r.Header.Get(opts.Header))
			if header == "" {
				csrfRejects.Inc()
				writePersistedError(w, http.StatusForbidden, "CSRF_MISSING", "missing CSRF header")
				return
			}
			if subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) != 1 {
				csrfRejects.Inc()
				writePersistedError(w, http.StatusForbidden, "CSRF_INVALID", "CSRF token mismatch")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func issueCSRFCookie(w http.ResponseWriter, opts CSRFOptions) error {
	tok, err := randomToken(32)
	if err != nil {
		return err
	}
	c := &http.Cookie{
		Name:     opts.Cookie,
		Value:    tok,
		Path:     opts.CookiePath,
		Domain:   opts.Domain,
		Secure:   opts.Secure,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	}
	if opts.MaxAge > 0 {
		c.MaxAge = int(opts.MaxAge.Seconds())
		c.Expires = time.Now().Add(opts.MaxAge)
	}
	http.SetCookie(w, c)
	return nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func mutates(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func hasBearer(r *http.Request) bool {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	return len(h) > 7 && strings.EqualFold(h[:7], "Bearer ")
}

// ErrCSRFMissing is exported for any caller that wants to assert the
// middleware ran. The CSRF path uses raw JSON responses today; the
// typed error is reserved for future composition.
var ErrCSRFMissing = errors.New("gqlhardening: csrf missing")
