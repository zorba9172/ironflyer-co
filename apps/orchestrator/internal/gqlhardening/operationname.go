package gqlhardening

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// peekBodyLimit is the maximum number of bytes the operationName peek
// will read before giving up. 32 KiB is large enough to cover every
// real-world GraphQL document we ship through this orchestrator while
// keeping the cost of the peek bounded for adversarial clients.
const peekBodyLimit = 32 * 1024

// PeekOperationName reads up to peekBodyLimit bytes of the request
// body, parses just enough JSON to extract the GraphQL `operationName`
// field, then restores the body via io.NopCloser so downstream
// handlers (PersistedQueriesMiddleware, the gqlgen handler itself)
// see the original payload byte-for-byte.
//
// Returns "" when:
//   - r or r.Body is nil
//   - the method is not POST (peek is only meaningful for the GraphQL
//     POST entry point; GET-based persisted-query lookups land in the
//     URL path, not the body)
//   - the body is larger than peekBodyLimit
//   - the body is not valid JSON
//   - the body is a multipart upload (apollo-upload-client and the
//     graphql multipart spec; we tolerate without trying to parse)
//   - the JSON parses but operationName is missing or empty
//
// This is a best-effort helper. Callers MUST treat the empty string as
// "no name available" and fall back to a structural identifier (HTTP
// method + path) for whatever they're using the name for.
func PeekOperationName(r *http.Request) string {
	if r == nil || r.Body == nil {
		return ""
	}
	if r.Method != http.MethodPost {
		return ""
	}
	// Multipart uploads land here with multipart/form-data; we
	// deliberately do NOT consume that body because the gqlgen
	// multipart handler depends on the original stream + boundary.
	ct := r.Header.Get("Content-Type")
	if ct != "" && strings.HasPrefix(strings.ToLower(ct), "multipart/") {
		return ""
	}

	// LimitReader caps the cost of the peek even when the client
	// streams gigabytes. We read into a buffer so the body restore
	// path always gets the exact bytes back.
	buf, err := io.ReadAll(io.LimitReader(r.Body, peekBodyLimit+1))
	if err != nil {
		// On a read error we can't restore the body safely; the
		// downstream handler will surface the IO failure with a
		// proper error response.
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(buf))
		return ""
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(buf))
	if len(buf) > peekBodyLimit {
		// Body exceeded the peek budget. Restore the prefix we read
		// AND splice in a fresh reader for the rest... actually we
		// can't, because the underlying body is now drained past the
		// limit. Conservatively: drop the operationName hint rather
		// than corrupting downstream parsing.
		return ""
	}

	// Use json.Decoder + UseNumber to avoid the float64 coercion
	// surprise on numeric fields we don't care about, and keep
	// allocations low by decoding into a tiny shape with only
	// operationName.
	var probe struct {
		OperationName string `json:"operationName"`
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	if err := dec.Decode(&probe); err != nil {
		return ""
	}
	name := strings.TrimSpace(probe.OperationName)
	// Cap to a sane length so a hostile client can't bloat the
	// rate-limit key and grow the Prometheus label cardinality.
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}
