//go:build !onnx

// The default build of the orchestrator does NOT include the local ONNX
// embedder. The ONNX runtime binding (github.com/yalue/onnxruntime_go)
// is CGO-only and depends on a C++ ONNX Runtime shared library that
// isn't guaranteed to be present on every developer machine. Operators
// who want the local-inference path compile with `-tags onnx` (see
// docs/EMBEDDINGS.md) and the real implementation in onnx.go takes
// over via the build-tag flip.
//
// This stub keeps the package symbol set identical across the two
// build modes so callers (main.go, cache.go) can reference
// NewONNXEmbedder unconditionally. When the tag is off, the
// constructor always returns ErrONNXUnavailable; the higher-level
// strategy switch ("auto") interprets that as "fall back to HF".

package embeddings

import (
	"context"
	"errors"
)

// ErrONNXUnavailable is returned by NewONNXEmbedder when the binary
// was built without the `onnx` tag. It is also returned by the stub
// implementation's Embed / EmbedBatch so callers can detect the
// disabled state via errors.Is and degrade gracefully.
var ErrONNXUnavailable = errors.New("embeddings: onnx backend not compiled in (rebuild with -tags onnx)")

// ONNXConfig captures the runtime configuration of the local ONNX
// embedder. The struct is exported in both build modes so call-sites
// don't have to fence on the tag — only the constructor changes
// behaviour. ModelPath points at a bge-small-en-v1.5 .onnx file (see
// docs/EMBEDDINGS.md for the download instructions); VocabPath points
// at the matching vocab.txt for the WordPiece tokenizer; Dimension is
// the expected output dim (defaults to 384 for bge-small).
type ONNXConfig struct {
	ModelPath string
	VocabPath string
	Dimension int
}

// ONNXEmbedder is the placeholder type exposed when the orchestrator
// is built without the `onnx` tag. It satisfies the Embedder interface
// but every call returns ErrONNXUnavailable so the strategy switch in
// main.go can treat it as a degraded backend without any nil checks.
type ONNXEmbedder struct{}

// NewONNXEmbedder reports that local ONNX inference is unavailable in
// this build. The strategy switch in main.go converts this error into
// the auto-fallback path (HF) when IRONFLYER_EMBEDDINGS_BACKEND=auto.
func NewONNXEmbedder(_ ONNXConfig) (*ONNXEmbedder, error) {
	return nil, ErrONNXUnavailable
}

// Embed returns ErrONNXUnavailable. Included so the stub type still
// satisfies Embedder — keeps higher-level code tag-agnostic.
func (*ONNXEmbedder) Embed(context.Context, string) ([]float32, error) {
	return nil, ErrONNXUnavailable
}

// EmbedBatch returns ErrONNXUnavailable for the same reason as Embed.
func (*ONNXEmbedder) EmbedBatch(context.Context, []string) ([][]float32, error) {
	return nil, ErrONNXUnavailable
}

// Dim returns 0 in the disabled build — there is no model loaded.
func (*ONNXEmbedder) Dim() int { return 0 }

// compile-time interface satisfaction.
var _ Embedder = (*ONNXEmbedder)(nil)
