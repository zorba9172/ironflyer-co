package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAuthRoundTripperScopesAuthorizationToAllowedOrigin(t *testing.T) {
	t.Parallel()

	allowed, err := url.Parse("https://api.ironflyer.example")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		reqURL   string
		existing string
		wantAuth string
	}{
		{
			name:     "same origin gets bearer token",
			reqURL:   "https://api.ironflyer.example/graphql",
			wantAuth: "Bearer test-token",
		},
		{
			name:     "same origin with default port gets bearer token",
			reqURL:   "https://api.ironflyer.example:443/graphql",
			wantAuth: "Bearer test-token",
		},
		{
			name:     "different host does not get bearer token",
			reqURL:   "https://exports.example/archive.zip",
			wantAuth: "",
		},
		{
			name:     "lookalike host does not get bearer token",
			reqURL:   "https://api.ironflyer.example.evil.test/archive.zip",
			wantAuth: "",
		},
		{
			name:     "different scheme does not get bearer token",
			reqURL:   "http://api.ironflyer.example/graphql",
			wantAuth: "",
		},
		{
			name:     "cross origin existing authorization is stripped",
			reqURL:   "https://exports.example/archive.zip",
			existing: "Bearer test-token",
			wantAuth: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotAuth string
			rt := &authRoundTripper{
				base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotAuth = req.Header.Get("Authorization")
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(http.NoBody),
						Request:    req,
					}, nil
				}),
				token:       "test-token",
				allowedHost: allowed,
			}
			req, err := http.NewRequest(http.MethodGet, tc.reqURL, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.existing != "" {
				req.Header.Set("Authorization", tc.existing)
			}

			resp, err := rt.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()

			if gotAuth != tc.wantAuth {
				t.Fatalf("Authorization = %q, want %q", gotAuth, tc.wantAuth)
			}
		})
	}
}

func TestHealthAtDoesNotSendAuthorizationToRuntimeOrigin(t *testing.T) {
	t.Parallel()

	var orchestratorAuth string
	orchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orchestratorAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer orchestrator.Close()

	var runtimeAuth string
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runtimeAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer runtime.Close()

	c := New(orchestrator.URL, "test-token")
	if _, err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.HealthAt(context.Background(), runtime.URL); err != nil {
		t.Fatal(err)
	}

	if orchestratorAuth != "Bearer test-token" {
		t.Fatalf("orchestrator Authorization = %q, want bearer token", orchestratorAuth)
	}
	if runtimeAuth != "" {
		t.Fatalf("runtime Authorization = %q, want empty", runtimeAuth)
	}
}
