package diagnostics

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ironflyer/core/orchestrator/internal/operations/operator"
	"ironflyer/core/orchestrator/internal/pkg/httputil"
)

// TailHandler returns an http.HandlerFunc serving GET /admin/logs/tail.
// The endpoint streams recent ring entries as NDJSON (newline-
// delimited JSON), filtered by `since` (RFC3339) + `level` + `limit`.
// Operator-gated via operator.RequireOperator — non-operators get
// 403.
//
// The current implementation snapshots the ring once per request and
// streams the result. A follow-up can switch to true server-sent-events
// with a tail subscription; today's surface is sufficient for "what
// just broke?" investigations.
func (s *Service) TailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := operator.RequireOperator(r.Context()); err != nil {
			httputil.WriteError(w, http.StatusForbidden, "operator role required")
			return
		}
		if s == nil || s.ring == nil {
			httputil.WriteError(w, http.StatusServiceUnavailable, "diagnostics not configured")
			return
		}

		q := r.URL.Query()
		since := time.Time{}
		if raw := strings.TrimSpace(q.Get("since")); raw != "" {
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				since = t
			}
		}
		level := strings.TrimSpace(q.Get("level"))
		if level == "" {
			level = "warn"
		}
		limit := 200
		if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}
		if limit > 2000 {
			limit = 2000
		}

		entries := s.RecentLogs(since, limit, level)

		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)
		for _, e := range entries {
			if err := enc.Encode(e); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

