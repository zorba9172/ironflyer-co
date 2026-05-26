package httpapi

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/operations/sentryext"
)

// sentryRecoverer replaces chi's middleware.Recoverer so panics get
// reported to Sentry before they are logged + converted into a 500.
// Mirrors core/runtime/internal/httpapi/sentry.go shape so operator
// expectations stay intact (suppress http.ErrAbortHandler sentinel,
// log + 500 otherwise).
func sentryRecoverer(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				sentryext.CaptureRecovered(r.Context(), rec)
				logger.Error().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("panic", fmt.Sprintf("%v", rec)).
					Bytes("stack", debug.Stack()).
					Msg("panic recovered")
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
