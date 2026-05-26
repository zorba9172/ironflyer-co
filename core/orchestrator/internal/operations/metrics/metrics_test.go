package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestHandler_ExposesGoRuntimeMetrics(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"go_goroutines", "process_resident_memory_bytes"} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q; first 200 bytes: %q", want, body[:min(200, len(body))])
		}
	}
}

func TestHTTPMiddleware_RecordsRequestAndLatency(t *testing.T) {
	r := chi.NewRouter()
	r.Use(HTTP)
	r.Get("/things/{id}", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Millisecond)
		w.WriteHeader(204)
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/things/abc", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 204 {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	}

	out := scrape(t)
	// Cardinality should collapse the dynamic ID to the route pattern.
	if !strings.Contains(out, `ironflyer_http_requests_total{method="GET",route="/things/{id}",status="204"} 3`) {
		t.Errorf("counter line missing or wrong; got:\n%s", out)
	}
	if !strings.Contains(out, `ironflyer_http_request_duration_seconds_count{method="GET",route="/things/{id}"} 3`) {
		t.Errorf("duration count line missing; got:\n%s", out)
	}
}

func TestObserveAgentAndCharge(t *testing.T) {
	ObserveAgent("planner", "anthropic", "ok", 1500*time.Millisecond)
	ObserveAgent("planner", "anthropic", "ok", 200*time.Millisecond)
	ObserveCharge("anthropic", "claude-opus-4-7", 0.0234)
	ObserveCharge("anthropic", "claude-opus-4-7", 0.0050)
	ObserveCharge("openai", "gpt-4o", 0)        // zero should be ignored
	ObserveCharge("openai", "gpt-4o", -1.0)     // negative should be ignored

	out := scrape(t)
	if !strings.Contains(out, `ironflyer_agent_runs_total{outcome="ok",provider="anthropic",role="planner"} 2`) {
		t.Errorf("agent counter wrong; got:\n%s", out)
	}
	if !strings.Contains(out, `ironflyer_charge_usd_total{model="claude-opus-4-7",provider="anthropic"} 0.0284`) {
		t.Errorf("charge counter wrong; got:\n%s", out)
	}
	if strings.Contains(out, `ironflyer_charge_usd_total{model="gpt-4o"`) {
		t.Errorf("zero/negative charges must not register; got:\n%s", out)
	}
}

func scrape(t *testing.T) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	return rec.Body.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
