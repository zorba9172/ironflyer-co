// Package auth verifies JWTs minted by the orchestrator. The runtime never
// owns a user database — it trusts the orchestrator's signature.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID    string
	Email string
}

type ctxKey struct{}

var userCtxKey = ctxKey{}

func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

type Verifier struct {
	secret []byte
	issuer string
}

func NewVerifier(secret []byte, issuer string) *Verifier {
	return &Verifier{secret: secret, issuer: issuer}
}

// Verify parses + validates an HS256 JWT, returns the user it identifies.
func (v *Verifier) Verify(tok string) (User, error) {
	type claims struct {
		Email string `json:"email"`
		jwt.RegisteredClaims
	}
	t, err := jwt.ParseWithClaims(tok, &claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return v.secret, nil
	})
	if err != nil {
		return User{}, err
	}
	c, ok := t.Claims.(*claims)
	if !ok || !t.Valid {
		return User{}, errors.New("invalid token")
	}
	if v.issuer != "" && c.Issuer != v.issuer {
		return User{}, errors.New("issuer mismatch")
	}
	return User{ID: c.Subject, Email: c.Email}, nil
}

// Middleware is the chi-compatible middleware. When secret is empty the
// middleware is a no-op (dev mode). WebSocket clients pass ?token= because
// they can't set headers.
func Middleware(v *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v == nil {
				next.ServeHTTP(w, r)
				return
			}
			tok := extractToken(r)
			if tok == "" {
				writeUnauthorized(w, "missing token")
				return
			}
			u, err := v.Verify(tok)
			if err != nil {
				writeUnauthorized(w, "invalid token")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}

func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(strings.ToLower(h), "bearer ") {
			return strings.TrimSpace(h[7:])
		}
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	return ""
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="ironflyer-runtime"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
