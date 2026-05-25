// Package logctx propagates request- and execution-scoped identifiers
// through context.Context, and decorates a zerolog.Logger with those
// identifiers so every log line emitted on a ctx-aware code path carries
// the same shape.
//
// The orchestrator's existing log surface was a mix: some lines had
// request_id, most didn't; some had tenant_id, most didn't; nothing
// carried execution_id automatically. This package gives every log
// line on a ctx-aware path the same five fields:
//
//   - request_id    (set by RequestIDMiddleware)
//   - tenant_id     (set by RequestIDMiddleware after auth resolves)
//   - execution_id  (set by profitguardctx.WithExecution + this helper)
//   - deploy_id     (set by deploy plane wireup)
//   - workspace_id  (set by runtime allocation)
//
// Call sites that want the decorated logger use logctx.From(ctx).
// Call sites that have a bare logger but want it decorated use
// logctx.Decorate(ctx, base).
package logctx

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	tenantIDKey
	executionIDKey
	deployIDKey
	workspaceIDKey
	loggerKey
)

// WithRequestID returns a ctx carrying the supplied request id. When
// `id` is empty a fresh UUID is minted so every code path downstream
// has something to correlate on.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.NewString()
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the request id stamped on ctx (or "").
func RequestID(ctx context.Context) string {
	return stringValue(ctx, requestIDKey)
}

// WithTenantID returns a ctx carrying the supplied tenant id. Empty
// ids leave ctx unchanged so callers can chain unconditionally.
func WithTenantID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, tenantIDKey, id)
}

// TenantID returns the tenant id stamped on ctx (or "").
func TenantID(ctx context.Context) string {
	return stringValue(ctx, tenantIDKey)
}

// WithExecutionID returns a ctx carrying the supplied execution id.
// Empty ids leave ctx unchanged.
func WithExecutionID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, executionIDKey, id)
}

// ExecutionID returns the execution id stamped on ctx (or "").
func ExecutionID(ctx context.Context) string {
	return stringValue(ctx, executionIDKey)
}

// WithDeployID returns a ctx carrying the supplied deploy id.
func WithDeployID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, deployIDKey, id)
}

// DeployID returns the deploy id stamped on ctx (or "").
func DeployID(ctx context.Context) string {
	return stringValue(ctx, deployIDKey)
}

// WithWorkspaceID returns a ctx carrying the supplied workspace id.
func WithWorkspaceID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, workspaceIDKey, id)
}

// WorkspaceID returns the workspace id stamped on ctx (or "").
func WorkspaceID(ctx context.Context) string {
	return stringValue(ctx, workspaceIDKey)
}

// ContextWithLogger stamps the supplied logger on ctx so From() can
// retrieve and decorate it. Useful when an early middleware (the
// orchestrator's RequestIDMiddleware) wants to forward the canonical
// base logger downstream without every call site importing it.
func ContextWithLogger(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// loggerFromCtx returns the base logger stamped via ContextWithLogger.
// The ok flag distinguishes "no logger set" from "explicitly nop".
func loggerFromCtx(ctx context.Context) (zerolog.Logger, bool) {
	if ctx == nil {
		return zerolog.Nop(), false
	}
	v, ok := ctx.Value(loggerKey).(zerolog.Logger)
	if !ok {
		return zerolog.Nop(), false
	}
	return v, true
}

// Decorate returns a zerolog.Logger derived from `base` with every
// ctx-carried id stamped as a structured field. Missing values are
// omitted (no empty-string fields) so the log shape stays clean.
func Decorate(ctx context.Context, base zerolog.Logger) zerolog.Logger {
	if ctx == nil {
		return base
	}
	ctxBuilder := base.With()
	if v := RequestID(ctx); v != "" {
		ctxBuilder = ctxBuilder.Str("request_id", v)
	}
	if v := TenantID(ctx); v != "" {
		ctxBuilder = ctxBuilder.Str("tenant_id", v)
	}
	if v := ExecutionID(ctx); v != "" {
		ctxBuilder = ctxBuilder.Str("execution_id", v)
	}
	if v := DeployID(ctx); v != "" {
		ctxBuilder = ctxBuilder.Str("deploy_id", v)
	}
	if v := WorkspaceID(ctx); v != "" {
		ctxBuilder = ctxBuilder.Str("workspace_id", v)
	}
	return ctxBuilder.Logger()
}

// From returns the decorated logger associated with ctx. If no base
// logger was stamped via ContextWithLogger a discarded-output logger
// is returned so callers can chain `.Info().Msg(...)` unconditionally
// without nil checks. The fallback is intentionally non-nop so the
// zerolog hook plumbed by the diagnostics package still observes the
// event when one is wired against the package-level fallback.
func From(ctx context.Context) zerolog.Logger {
	if base, ok := loggerFromCtx(ctx); ok {
		return Decorate(ctx, base)
	}
	return Decorate(ctx, fallbackLogger)
}

// fallbackLogger is the discarded-output logger From() returns when no
// base logger was stamped on ctx. Writing to io.Discard keeps the API
// safe even when call sites haven't been migrated yet — the hook plane
// only ever sees structured events that originated from a wired
// logger, so unwired call sites stay invisible (and harmless).
var fallbackLogger = zerolog.New(io.Discard)
