package gqlhardening

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failingRegisterStore struct {
	*MemoryStore
}

func (s failingRegisterStore) Register(context.Context, string, string, string, string) error {
	return errors.New("store unavailable")
}

func TestPersistedQueriesMiddlewareFailsClosedWhenRegisterFails(t *testing.T) {
	query := "query Test { viewer { id } }"
	hash := HashQuery(query)
	body := `{"query":` + quoteJSON(query) + `,"extensions":{"persistedQuery":{"version":1,"sha256Hash":"` + hash + `"}}}`

	calledNext := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := PersistedQueriesMiddleware(failingRegisterStore{MemoryStore: NewMemoryStore()}, false, nil)(next)

	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if calledNext {
		t.Fatal("next handler was called after persisted-query registration failed")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "PERSISTED_QUERY_REGISTER_FAILED") {
		t.Fatalf("response body missing register failure code: %s", rec.Body.String())
	}
}

func TestOriginAllow(t *testing.T) {
	t.Run("development empty allowlist accepts any origin", func(t *testing.T) {
		check := OriginAllow(nil)
		req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
		req.Header.Set("Origin", "https://evil.example")
		if !check(req) {
			t.Fatal("empty allowlist should accept origins for development mode")
		}
	})

	t.Run("allowlist compares scheme host and port exactly", func(t *testing.T) {
		check := OriginAllow([]string{"https://studio.ironflyer.ai", "localhost:3000"})

		allowed := httptest.NewRequest(http.MethodGet, "/graphql", nil)
		allowed.Header.Set("Origin", "https://studio.ironflyer.ai")
		if !check(allowed) {
			t.Fatal("expected configured https origin to be allowed")
		}

		bareHost := httptest.NewRequest(http.MethodGet, "/graphql", nil)
		bareHost.Header.Set("Origin", "https://localhost:3000")
		if !check(bareHost) {
			t.Fatal("expected bare host allowlist entry to canonicalize to https")
		}

		rejected := httptest.NewRequest(http.MethodGet, "/graphql", nil)
		rejected.Header.Set("Origin", "https://studio.ironflyer.ai.evil.example")
		if check(rejected) {
			t.Fatal("unexpectedly accepted suffix-matched origin")
		}
	})

	t.Run("same origin upgrade without origin header is allowed", func(t *testing.T) {
		check := OriginAllow([]string{"https://studio.ironflyer.ai"})
		req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
		if !check(req) {
			t.Fatal("same-origin upgrade without Origin header should be allowed")
		}
	})
}

func quoteJSON(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(s) + `"`
}
