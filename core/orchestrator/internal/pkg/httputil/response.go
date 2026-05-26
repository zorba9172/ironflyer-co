// Package httputil collects the tiny HTTP-response primitives every
// handler in the orchestrator reaches for: JSON write with the right
// Content-Type, a uniform {"error": "..."} body shape, a hard-capped
// body read, and a JSON decode that wraps its error with context.
package httputil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const jsonContentType = "application/json; charset=utf-8"

// WriteJSON sets Content-Type and the status code, then encodes v.
// Encoding errors are swallowed — the status header has already been
// written, so there is nothing useful the caller can do with the err.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError emits the canonical {"error": msg} body at the given
// status. Use this instead of hand-rolling map[string]string everywhere.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// ReadLimitedBody drains r.Body with a hard byte cap so a malicious or
// misconfigured client cannot OOM the process. Returns the bytes read
// so far on any error.
func ReadLimitedBody(r *http.Request, maxBytes int64) ([]byte, error) {
	limited := http.MaxBytesReader(nil, r.Body, maxBytes)
	return io.ReadAll(limited)
}

// DecodeJSON reads JSON from r into v, wrapping any failure with a
// "decode json:" prefix so call sites can return it directly.
func DecodeJSON(r io.Reader, v any) error {
	if err := json.NewDecoder(r).Decode(v); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}
