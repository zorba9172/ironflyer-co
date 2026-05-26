package httpapi

import (
	"bytes"
	"crypto/subtle"
	"net/http"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

// leakProbeHandler exposes a goroutine snapshot at /debug/leak/snapshot.
//
// This is a REST exception in the spirit of /metrics: it is a
// diagnostic surface, not part of the V22 API of record. It is
// disabled unless IRONFLYER_LEAK_PROBE_TOKEN is set, and protected
// by a constant-time bearer comparison so a leaked token does not
// expose the goroutine stack of every prod pod.
//
// Smoke harness contract (see scripts/lint/run-goleak-smoke.sh):
//
//	GET /debug/leak/snapshot
//	Authorization: Bearer <IRONFLYER_LEAK_PROBE_TOKEN>
//	→ 200 application/json
//	   {
//	     "ts": "<RFC3339>",
//	     "goroutines": <int>,
//	     "stacks": "<full pprof goroutine dump>"
//	   }
//
// The stacks field is the raw output of pprof.Lookup("goroutine").
// WriteTo(_, 1) — text, not JSON-escaped per stack. Operators diff it
// against the baseline at scripts/health/goleak-baseline.json to
// detect goroutine leaks at smoke-time WITHOUT depending on
// go.uber.org/goleak (which is test-only and banned per the
// repo-wide no-tests rule).
func leakProbeHandler(token string) http.Handler {
	expected := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="leak-probe"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := []byte(strings.TrimSpace(header[len(prefix):]))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Capture the goroutine snapshot. debug=1 emits a per-goroutine
		// header + symbolised stack; debug=2 is heavier (all frames
		// expanded) and we don't need it for steady-state leak
		// detection.
		var buf bytes.Buffer
		profile := pprof.Lookup("goroutine")
		if profile == nil {
			http.Error(w, "goroutine profile unavailable", http.StatusInternalServerError)
			return
		}
		if err := profile.WriteTo(&buf, 1); err != nil {
			http.Error(w, "pprof write failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ts":         time.Now().UTC().Format(time.RFC3339),
			"goroutines": runtime.NumGoroutine(),
			"stacks":     buf.String(),
		})
	})
}
