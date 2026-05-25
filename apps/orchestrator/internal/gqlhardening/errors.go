package gqlhardening

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

// RedactedErrorPresenter returns a gqlgen error presenter that scrubs
// internal IDs, stack traces, file paths, SQL fragments, and provider
// error bodies before the error reaches the client. The presenter
// preserves the original message + path when the error is an explicit
// *gqlerror.Error carrying a "code" extension (so policy denials,
// depth/complexity rejects, and persisted-query misses still flow
// through with their codes intact).
//
// In non-prod mode the presenter is a near-passthrough — only the path
// + stack-trace scrub remain — so developers still get usable errors
// locally without leaking sensitive content if they screenshot a
// staging response.
func RedactedErrorPresenter(prodMode bool) func(ctx context.Context, e error) *gqlerror.Error {
	return func(ctx context.Context, e error) *gqlerror.Error {
		var gqlErr *gqlerror.Error
		if errors.As(e, &gqlErr) {
			gqlErr.Message = redact(gqlErr.Message, prodMode)
			return gqlErr
		}
		msg := redact(e.Error(), prodMode)
		if prodMode {
			msg = "internal server error"
		}
		return &gqlerror.Error{Message: msg}
	}
}

// Patterns that show up in raw provider / DB / runtime errors. Each
// pattern is replaced with a stable token so the redacted message
// keeps shape (length, separators) but loses the sensitive payload.
var (
	// UUIDs in any standard hyphenated form.
	reUUID = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	// File paths (Unix absolute paths reaching into the orchestrator).
	reFilePath = regexp.MustCompile(`(/[A-Za-z0-9._-]+){2,}\.go(:\d+)?`)
	// Common SQL fragments that leak through pgx error wrapping.
	reSQL = regexp.MustCompile(`(?i)\b(SELECT|INSERT|UPDATE|DELETE|FROM|WHERE|JOIN|RETURNING)\b[^.]*`)
	// stack-trace style "goroutine N" or addresses.
	reGoroutine = regexp.MustCompile(`goroutine \d+ \[[^\]]+\]`)
	reHexAddr   = regexp.MustCompile(`0x[0-9a-f]{6,}`)
	// Anthropic / OpenAI / Google often quote raw JSON bodies — strip
	// anything that looks like a JSON object so a provider error body
	// can't reach the client untouched.
	reJSONBody = regexp.MustCompile(`\{[^{}]{40,}\}`)
)

func redact(msg string, prodMode bool) string {
	if msg == "" {
		return msg
	}
	out := msg
	out = reGoroutine.ReplaceAllString(out, "[goroutine]")
	out = reHexAddr.ReplaceAllString(out, "[addr]")
	out = reFilePath.ReplaceAllString(out, "[path]")
	if prodMode {
		out = reUUID.ReplaceAllString(out, "[id]")
		out = reSQL.ReplaceAllString(out, "[sql]")
		out = reJSONBody.ReplaceAllString(out, "[body]")
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "internal server error"
	}
	return out
}
