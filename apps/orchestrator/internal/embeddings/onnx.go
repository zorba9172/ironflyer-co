//go:build onnx

// Local ONNX-based embedder for the memory retrieval pipeline. Selected
// at runtime via IRONFLYER_EMBEDDINGS_BACKEND=onnx (or =auto when the
// model artefacts are present); compiled in via `go build -tags onnx`.
//
// MODEL FILE: this code expects a BAAI/bge-small-en-v1.5 ONNX export.
// You can pull the official ONNX checkpoint from the model card at
// https://huggingface.co/BAAI/bge-small-en-v1.5 — the "onnx/model.onnx"
// file is the canonical artefact. We deliberately do NOT commit the
// model into the repo (it's ~130 MiB and licensed by BAAI) — it ships
// alongside the Docker image as a release artefact and the path is
// passed via IRONFLYER_ONNX_MODEL. The matching tokenizer vocabulary
// (vocab.txt) lives at IRONFLYER_ONNX_VOCAB.
//
// BUILD: requires CGO and the ONNX Runtime C++ shared library on the
// host. On Linux x64 (the Helm chart's target) install via apt or
// download from https://github.com/microsoft/onnxruntime/releases.
// Set ONNXRUNTIME_SHARED_LIBRARY to the absolute path of libonnxruntime.so
// before the orchestrator starts.

package embeddings

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"

	tokenizers "github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/model/wordpiece"
	"github.com/sugarme/tokenizer/pretokenizer"
	"github.com/sugarme/tokenizer/processor"
	ort "github.com/yalue/onnxruntime_go"
)

// ErrONNXUnavailable mirrors the disabled-build sentinel so call-sites
// can use a single errors.Is check across both build modes. It is
// returned by NewONNXEmbedder when required configuration is missing
// (model file path empty, file unreadable, etc.).
var ErrONNXUnavailable = errors.New("embeddings: onnx backend not configured")

// ONNXConfig captures the runtime configuration of the local ONNX
// embedder. See onnx_disabled.go for field meanings — the layout is
// kept identical between the two build modes.
type ONNXConfig struct {
	ModelPath string
	VocabPath string
	Dimension int
}

// ortInitOnce guards the global ONNX Runtime init. The C runtime is a
// process-wide singleton; calling InitializeEnvironment twice panics.
var ortInitOnce sync.Once
var ortInitErr error

func initORT() error {
	ortInitOnce.Do(func() {
		if lib := os.Getenv("ONNXRUNTIME_SHARED_LIBRARY"); lib != "" {
			ort.SetSharedLibraryPath(lib)
		}
		ortInitErr = ort.InitializeEnvironment()
	})
	return ortInitErr
}

// ONNXEmbedder runs bge-small-en-v1.5 (or a compatible BERT-style
// sentence encoder) locally via the ONNX Runtime. It is safe for
// concurrent use: the underlying session serialises calls, and the
// tokenizer is read-only after construction.
type ONNXEmbedder struct {
	mu        sync.Mutex // ORT sessions are not goroutine-safe
	sess      *ort.DynamicAdvancedSession
	tokenizer *tokenizers.Tokenizer
	dim       int
}

// NewONNXEmbedder loads the ONNX model + WordPiece vocab from cfg and
// returns a ready-to-use Embedder. Returns ErrONNXUnavailable when
// either path is empty so the "auto" strategy can fall back to HF
// without an alarming error in the logs.
func NewONNXEmbedder(cfg ONNXConfig) (*ONNXEmbedder, error) {
	if cfg.ModelPath == "" || cfg.VocabPath == "" {
		return nil, ErrONNXUnavailable
	}
	if _, err := os.Stat(cfg.ModelPath); err != nil {
		return nil, fmt.Errorf("embeddings: onnx model %q: %w", cfg.ModelPath, err)
	}
	if _, err := os.Stat(cfg.VocabPath); err != nil {
		return nil, fmt.Errorf("embeddings: onnx vocab %q: %w", cfg.VocabPath, err)
	}
	if err := initORT(); err != nil {
		return nil, fmt.Errorf("embeddings: onnx runtime init: %w", err)
	}

	tk, err := buildBertTokenizer(cfg.VocabPath)
	if err != nil {
		return nil, fmt.Errorf("embeddings: tokenizer: %w", err)
	}

	// bge-small exposes three named inputs (input_ids, attention_mask,
	// token_type_ids) and one named output (last_hidden_state). We pass
	// the names so DynamicAdvancedSession can resolve them without us
	// hard-coding their tensor indices.
	sess, err := ort.NewDynamicAdvancedSession(
		cfg.ModelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("embeddings: onnx session: %w", err)
	}

	dim := cfg.Dimension
	if dim <= 0 {
		dim = 384 // bge-small-en-v1.5 hidden size.
	}
	return &ONNXEmbedder{sess: sess, tokenizer: tk, dim: dim}, nil
}

// buildBertTokenizer wires up a BERT-style WordPiece tokenizer with
// the standard pre/post processors. We mirror the configuration the
// transformers library uses for bge-small so encode produces the same
// ids the model was trained on.
func buildBertTokenizer(vocabPath string) (*tokenizers.Tokenizer, error) {
	model, err := wordpiece.NewWordPieceFromFile(vocabPath, "[UNK]")
	if err != nil {
		return nil, err
	}
	tk := tokenizers.NewTokenizer(model)
	tk.WithPreTokenizer(pretokenizer.NewBertPreTokenizer())
	postProc := processor.NewBertProcessing(
		processor.PostToken{Value: "[SEP]", Id: 102},
		processor.PostToken{Value: "[CLS]", Id: 101},
	)
	tk.WithPostProcessor(postProc)
	return tk, nil
}

// Dim returns the embedding dimension (default 384 for bge-small).
// Unlike the HF backend this is known up-front from the config so
// callers never observe a 0 value after a successful constructor.
func (e *ONNXEmbedder) Dim() int { return e.dim }

// Embed encodes a single string. Delegates to EmbedBatch to keep the
// inference + pooling code in one place.
func (e *ONNXEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, errors.New("embeddings: onnx returned empty result")
	}
	return vecs[0], nil
}

// EmbedBatch runs a single inference call over all inputs, then mean-
// pools per-token hidden states (masked by attention) and L2-
// normalises the result. This is the canonical sentence-embedding
// recipe for bge-small — matching what HF's feature-extraction
// pipeline returns server-side.
func (e *ONNXEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 1. Tokenize each input. We pad to the max length in the batch
	//    rather than to a fixed 512 — short queries stay cheap.
	encoded, err := e.tokenizer.EncodeBatch(toInputs(texts), true)
	if err != nil {
		return nil, fmt.Errorf("embeddings: tokenize: %w", err)
	}
	batch := len(encoded)
	maxLen := 0
	for _, enc := range encoded {
		if l := len(enc.Ids); l > maxLen {
			maxLen = l
		}
	}
	if maxLen == 0 {
		return nil, errors.New("embeddings: tokenizer produced zero-length encoding")
	}

	ids := make([]int64, batch*maxLen)
	mask := make([]int64, batch*maxLen)
	typeIds := make([]int64, batch*maxLen)
	for i, enc := range encoded {
		base := i * maxLen
		for j, id := range enc.Ids {
			ids[base+j] = int64(id)
			mask[base+j] = 1
			if j < len(enc.TypeIds) {
				typeIds[base+j] = int64(enc.TypeIds[j])
			}
		}
	}

	shape := ort.NewShape(int64(batch), int64(maxLen))
	inIds, err := ort.NewTensor(shape, ids)
	if err != nil {
		return nil, fmt.Errorf("embeddings: ids tensor: %w", err)
	}
	defer inIds.Destroy()
	inMask, err := ort.NewTensor(shape, mask)
	if err != nil {
		return nil, fmt.Errorf("embeddings: mask tensor: %w", err)
	}
	defer inMask.Destroy()
	inTypes, err := ort.NewTensor(shape, typeIds)
	if err != nil {
		return nil, fmt.Errorf("embeddings: type-ids tensor: %w", err)
	}
	defer inTypes.Destroy()

	// 2. Run inference. ORT's Go binding is not goroutine-safe per
	//    session, so we serialise here. Embedding latency on bge-small
	//    runs ~5-20ms per batch on a modern CPU, so contention is rare.
	out, err := ort.NewEmptyTensor[float32](ort.NewShape(int64(batch), int64(maxLen), int64(e.dim)))
	if err != nil {
		return nil, fmt.Errorf("embeddings: output tensor: %w", err)
	}
	defer out.Destroy()

	e.mu.Lock()
	err = e.sess.Run(
		[]ort.Value{inIds, inMask, inTypes},
		[]ort.Value{out},
	)
	e.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("embeddings: onnx run: %w", err)
	}

	// 3. Mean-pool + L2-normalise. last_hidden_state has shape
	//    [batch, seq, dim]; we average across seq, weighted by the
	//    attention mask so pad tokens don't drag the centroid.
	hidden := out.GetData()
	results := make([][]float32, batch)
	for b := 0; b < batch; b++ {
		vec := make([]float32, e.dim)
		var count float64
		for t := 0; t < maxLen; t++ {
			if mask[b*maxLen+t] == 0 {
				continue
			}
			count++
			offset := (b*maxLen + t) * e.dim
			for d := 0; d < e.dim; d++ {
				vec[d] += hidden[offset+d]
			}
		}
		if count > 0 {
			inv := float32(1.0 / count)
			for d := 0; d < e.dim; d++ {
				vec[d] *= inv
			}
		}
		// L2-normalise so downstream cosine similarity matches the
		// HF-side vectors (which are also L2-normalised by bge-small).
		var norm float64
		for _, v := range vec {
			norm += float64(v) * float64(v)
		}
		if norm > 0 {
			inv := float32(1.0 / math.Sqrt(norm))
			for d := 0; d < e.dim; d++ {
				vec[d] *= inv
			}
		}
		results[b] = vec
	}
	return results, nil
}

// toInputs adapts plain strings into the tokenizer's input shape.
func toInputs(texts []string) []tokenizers.EncodeInput {
	out := make([]tokenizers.EncodeInput, len(texts))
	for i, t := range texts {
		out[i] = tokenizers.NewSingleEncodeInput(tokenizers.NewInputSequence(t))
	}
	return out
}

// compile-time interface satisfaction.
var _ Embedder = (*ONNXEmbedder)(nil)
