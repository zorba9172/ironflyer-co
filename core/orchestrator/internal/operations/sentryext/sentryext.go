// Package sentryext is a thin wrapper around getsentry/sentry-go for the
// orchestrator. Mirrors core/runtime/internal/sentryext so both services
// share the same DSN / flush semantics without crossing go.mod boundaries.
// OTel handles spans + metrics; Sentry handles exceptions + HTTP panics.
// Empty DSN short-circuits Init to a no-op so local boots stay quiet.
package sentryext

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

// Opts groups the init knobs. All fields are optional — an empty DSN
// short-circuits Init.
type Opts struct {
	DSN              string
	Environment      string
	Release          string
	TracesSampleRate float64
	ServerName       string
}

// Init configures the Sentry SDK and returns a Flush function the caller
// MUST defer near process shutdown. When DSN is empty the returned flush
// is a no-op.
func Init(o Opts) (flush func(), err error) {
	noop := func() {}
	if strings.TrimSpace(o.DSN) == "" {
		return noop, nil
	}
	env := o.Environment
	if env == "" {
		env = "development"
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              o.DSN,
		Environment:      env,
		Release:          o.Release,
		ServerName:       o.ServerName,
		EnableTracing:    true,
		TracesSampleRate: o.TracesSampleRate,
		AttachStacktrace: true,
	}); err != nil {
		return noop, err
	}
	return func() {
		sentry.Flush(2 * time.Second)
	}, nil
}

// FloatFromEnv parses a float env var with a fallback.
func FloatFromEnv(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

// CaptureRecovered reports a panic to Sentry while preserving the request
// context.
func CaptureRecovered(ctx context.Context, recovered any) {
	if recovered == nil {
		return
	}
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub.RecoverWithContext(ctx, recovered)
}

// CaptureError reports an error to Sentry with optional tag overrides
// so callers (graph error presenter, chat stream) can attach
// operation-name / execution-id without reaching for the Sentry SDK
// types directly.
func CaptureError(ctx context.Context, err error, tags map[string]string) {
	if err == nil {
		return
	}
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}
	hub.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			if k == "" || v == "" {
				continue
			}
			scope.SetTag(k, v)
		}
		hub.CaptureException(err)
	})
}
