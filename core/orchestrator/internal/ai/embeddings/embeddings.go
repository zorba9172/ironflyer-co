// Package embeddings turns text into fixed-dimension float32 vectors so the
// memory layer can do semantic retrieval instead of substring matching.
//
// The package is deliberately minimal: stdlib + net/http only, no extra
// go.mod dependencies. Operators wire it via the orchestrator config:
// set HF_API_KEY to enable the HuggingFace driver; leave it empty and the
// memory layer falls back to its built-in substring search.
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/pkg/httpclient"
)

// DefaultModel is the HuggingFace model id used when callers do not
// pick one. BAAI/bge-m3 is multilingual (English + Hebrew + 100+ langs),
// dense + sparse + ColBERT-style retrieval, and currently the strongest
// open-weights embedding for code+prose retrieval. It's larger than the
// prior bge-small default (1024-dim vs 384-dim) — operators who need
// the smaller footprint can pin BAAI/bge-small-en-v1.5 via the
// HF_EMBEDDINGS_MODEL env var.
const DefaultModel = "BAAI/bge-m3"

// DefaultBaseURL is the HuggingFace inference API root. Models are
// addressed by appending the model id to this base URL.
const DefaultBaseURL = "https://api-inference.huggingface.co/models/"

// Embedder maps text to a fixed-dimension float32 vector. Implementations
// must be safe for concurrent use.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch encodes many strings at once for efficiency.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dim() int
}

// HuggingFaceEmbedder uses HF's feature-extraction inference endpoint.
// Default model "BAAI/bge-m3" — multilingual (English + Hebrew + 100+
// langs), strong on code+prose retrieval. Set HF_EMBEDDINGS_MODEL on
// the orchestrator to pin a different id.
type HuggingFaceEmbedder struct {
	APIKey  string
	Model   string // default DefaultModel ("BAAI/bge-m3")
	BaseURL string // default "https://api-inference.huggingface.co/models/"
	HTTP    *http.Client

	mu  sync.RWMutex
	dim int // populated lazily on first Embed call
}

// NewHuggingFaceEmbedder constructs an HF-backed Embedder. An empty model
// falls back to DefaultModel. The HTTP client gets a 30s timeout — HF
// inference cold-starts can take several seconds but anything past that
// is a stuck request we should give up on.
func NewHuggingFaceEmbedder(apiKey, model string) *HuggingFaceEmbedder {
	if model == "" {
		model = DefaultModel
	}
	return &HuggingFaceEmbedder{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: DefaultBaseURL,
		HTTP:    httpclient.Standard(30 * time.Second),
	}
}

// Dim returns the embedding dimension. It is 0 until the first successful
// call to Embed / EmbedBatch — HF doesn't publish the dim out of band, so
// we learn it from the first response.
func (h *HuggingFaceEmbedder) Dim() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.dim
}

type hfRequest struct {
	Inputs  any        `json:"inputs"`
	Options hfReqOpts  `json:"options"`
}

type hfReqOpts struct {
	WaitForModel bool `json:"wait_for_model"`
}

// Embed encodes a single string into a vector.
func (h *HuggingFaceEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vec, err := h.callSingle(ctx, text)
	if err != nil {
		return nil, err
	}
	h.rememberDim(len(vec))
	return vec, nil
}

// EmbedBatch encodes many strings in a single inference call.
func (h *HuggingFaceEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	vecs, err := h.callBatch(ctx, texts)
	if err != nil {
		return nil, err
	}
	if len(vecs) > 0 {
		h.rememberDim(len(vecs[0]))
	}
	return vecs, nil
}

func (h *HuggingFaceEmbedder) rememberDim(d int) {
	if d <= 0 {
		return
	}
	h.mu.Lock()
	if h.dim == 0 {
		h.dim = d
	}
	h.mu.Unlock()
}

func (h *HuggingFaceEmbedder) endpoint() string {
	base := h.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	model := h.Model
	if model == "" {
		model = DefaultModel
	}
	return base + model
}

// callSingle posts one string and decodes a []float32 response.
//
// HF's feature-extraction pipeline returns []float32 for a single string
// input but, depending on the model, can wrap it in an extra outer array
// ([[...]]). We accept both shapes.
func (h *HuggingFaceEmbedder) callSingle(ctx context.Context, text string) ([]float32, error) {
	body, err := h.post(ctx, hfRequest{Inputs: text, Options: hfReqOpts{WaitForModel: true}})
	if err != nil {
		return nil, err
	}
	// Try [][]float32 first (wrapped), fall back to []float32.
	var wrapped [][]float32
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped) > 0 {
		return wrapped[0], nil
	}
	var flat []float32
	if err := json.Unmarshal(body, &flat); err == nil {
		return flat, nil
	}
	return nil, fmt.Errorf("embeddings: unexpected response shape: %s", truncate(body, 400))
}

// callBatch posts an array of strings and decodes a [][]float32 response.
func (h *HuggingFaceEmbedder) callBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := h.post(ctx, hfRequest{Inputs: texts, Options: hfReqOpts{WaitForModel: true}})
	if err != nil {
		return nil, err
	}
	var out [][]float32
	if err := json.Unmarshal(body, &out); err == nil {
		return out, nil
	}
	// Some models triple-nest ([[[...]]]); flatten the outer dim.
	var triple [][][]float32
	if err := json.Unmarshal(body, &triple); err == nil {
		flat := make([][]float32, 0, len(triple))
		for _, t := range triple {
			if len(t) > 0 {
				flat = append(flat, t[0])
			}
		}
		return flat, nil
	}
	return nil, fmt.Errorf("embeddings: unexpected batch response shape: %s", truncate(body, 400))
}

func (h *HuggingFaceEmbedder) post(ctx context.Context, payload hfRequest) ([]byte, error) {
	if h.APIKey == "" {
		return nil, errors.New("embeddings: huggingface api key not configured")
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("embeddings: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint(), bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("embeddings: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.APIKey)

	client := h.HTTP
	if client == nil {
		client = httpclient.Standard(30 * time.Second)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embeddings: http: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embeddings: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embeddings: huggingface returned %d: %s", resp.StatusCode, truncate(body, 400))
	}
	return body, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

// NoopEmbedder is the "embeddings disabled" stand-in. Returning an error
// from Embed / EmbedBatch lets callers (VectorStore) fall through to
// their substring-search path without any special-case nil checks.
type NoopEmbedder struct{}

// NewNoopEmbedder constructs a disabled Embedder.
func NewNoopEmbedder() *NoopEmbedder { return &NoopEmbedder{} }

// ErrDisabled is returned by NoopEmbedder so callers can detect the
// "no key configured" path cheaply via errors.Is.
var ErrDisabled = errors.New("embeddings disabled")

func (NoopEmbedder) Embed(context.Context, string) ([]float32, error) {
	return nil, ErrDisabled
}

func (NoopEmbedder) EmbedBatch(context.Context, []string) ([][]float32, error) {
	return nil, ErrDisabled
}

func (NoopEmbedder) Dim() int { return 0 }

// compile-time interface satisfaction.
var (
	_ Embedder = (*HuggingFaceEmbedder)(nil)
	_ Embedder = (*NoopEmbedder)(nil)
)
