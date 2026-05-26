//go:build !onnx

// Stub OnnxService for the default (no -tags onnx) build. Mirrors the
// constructor signature in onnx.go so main.go can reference
// NewOnnxService unconditionally; the constructor always returns the
// NoopService so callers degrade gracefully.

package inference

import "github.com/rs/zerolog"

// NewOnnxService returns a NoopService in the default build. The real
// constructor lives in onnx.go (behind //go:build onnx). Kept here so
// the orchestrator's main.go can call NewOnnxService unconditionally
// without a build-tag fence at the call site — the build mode picks
// the implementation.
//
// modelsDir is ignored in this build (NoopService never opens files).
// The signature is kept identical to the real impl so a `git diff`
// across the two implementations stays small and reviewable.
func NewOnnxService(_ string, logger zerolog.Logger) Service {
	logger.Info().Msg("inference: OnnxService unavailable (binary built without -tags onnx) — using NoopService")
	return NewNoopService(logger)
}
