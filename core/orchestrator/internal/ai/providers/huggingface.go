package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HuggingFaceProvider speaks to the HuggingFace Inference API's OpenAI-
// compatible chat-completions endpoint. HF hosts a sprawling catalogue of
// open-weight models — Llama 3.x, Mixtral, DeepSeek-V3, Qwen-2.5, Mistral,
// Hermes — so registering this provider gives the router cheap inference
// (DeepSeek/Qwen) and a privacy-routed fallback (CapPrivate) that the
// commercial providers cannot satisfy.
//
// Design notes:
//   - HF's `{BaseURL}/chat/completions` is wire-compatible with OpenAI:
//     same request body, same SSE frame shape, same `data: [DONE]`
//     terminator. We lift openai.go's SSE parser verbatim rather than
//     introducing a new dependency.
//   - Vision is intentionally refused. The Inference API only exposes
//     image inputs for a small set of multimodal endpoints (and not via
//     this OpenAI-compat shim), so any attachment-bearing request is
//     rejected up front, letting the router fall through to a
//     vision-capable provider instead of failing mid-stream.
//   - We advertise CapPrivate so the router can drain privacy-sensitive
//     traffic here. The actual privacy guarantee is the operator's
//     responsibility (self-hosted endpoint, dedicated inference endpoint,
//     etc.); this layer only tags the routing intent.
type HuggingFaceProvider struct {
	apiKey       string
	model        string
	cheapModel   string
	powerModel   string
	privateModel string
	baseURL      string
	httpClient   *http.Client
}

type HuggingFaceOpts struct {
	APIKey       string // HF Inference API token; required.
	Model        string // default "meta-llama/Llama-3.3-70B-Instruct"
	CheapModel   string // default "Qwen/Qwen2.5-7B-Instruct"
	PowerModel   string // default "meta-llama/Llama-3.3-70B-Instruct"
	PrivateModel string // default "" -> falls back to Model when CapPrivate requested
	BaseURL      string // default "https://api-inference.huggingface.co/v1"
}

func NewHuggingFaceProvider(opts HuggingFaceOpts) *HuggingFaceProvider {
	model := opts.Model
	if model == "" {
		model = "meta-llama/Llama-3.3-70B-Instruct"
	}
	cheap := opts.CheapModel
	if cheap == "" {
		cheap = "Qwen/Qwen2.5-7B-Instruct"
	}
	power := opts.PowerModel
	if power == "" {
		power = "meta-llama/Llama-3.3-70B-Instruct"
	}
	base := opts.BaseURL
	if base == "" {
		base = "https://api-inference.huggingface.co/v1"
	}
	base = strings.TrimRight(base, "/")
	return &HuggingFaceProvider{
		apiKey:       opts.APIKey,
		model:        model,
		cheapModel:   cheap,
		powerModel:   power,
		privateModel: opts.PrivateModel,
		baseURL:      base,
		// No client timeout — streaming completions can legitimately run
		// for minutes. Cancellation flows through the caller's context.
		// Uses the shared tuned streaming transport (see transport.go).
		httpClient: streamingHTTPClient(),
	}
}

func (h *HuggingFaceProvider) Name() string { return "huggingface" }

func (h *HuggingFaceProvider) Capabilities() []Capability {
	return []Capability{
		CapReasoning, CapCode, CapJSON, CapCheap, CapPrivate,
	}
}

// pickModel routes by capability tag:
//   - CapPrivate + a configured PrivateModel pins to that endpoint.
//   - CapCheap drops to the cheap tier (e.g. Qwen 2.5 7B).
//   - CapReasoning / CapThinking promote to the power tier (e.g. Llama 70B).
//   - Otherwise the configured base Model is used.
func (h *HuggingFaceProvider) pickModel(req Request) string {
	if h.privateModel != "" {
		for _, c := range req.Capabilities {
			if c == CapPrivate {
				return h.privateModel
			}
		}
	}
	for _, c := range req.Capabilities {
		if c == CapCheap {
			return h.cheapModel
		}
	}
	if req.EnableThinking {
		return h.powerModel
	}
	for _, c := range req.Capabilities {
		if c == CapThinking || c == CapReasoning {
			return h.powerModel
		}
	}
	return h.model
}

// --- request DTOs ---------------------------------------------------------
//
// Wire-compatible with OpenAI's Chat Completions schema. We re-declare the
// types instead of reusing openai.go's so the two providers can evolve
// independently if HF ever drifts from the OpenAI shape.

type hfChatRequest struct {
	Model         string          `json:"model"`
	Messages      []hfMessage     `json:"messages"`
	Stream        bool            `json:"stream"`
	StreamOptions *hfStreamOpts   `json:"stream_options,omitempty"`
	MaxTokens     *int            `json:"max_tokens,omitempty"`
	Temperature   *float32        `json:"temperature,omitempty"`
	Tools         []hfTool        `json:"tools,omitempty"`
	ResponseFormat *hfResponseFormat `json:"response_format,omitempty"`
}

type hfStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type hfResponseFormat struct {
	Type string `json:"type"`
}

type hfMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type hfTool struct {
	Type     string         `json:"type"`
	Function hfToolFunction `json:"function"`
}

type hfToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// --- streaming response DTOs ---------------------------------------------

type hfStreamFrame struct {
	Choices []hfStreamChoice `json:"choices"`
	Usage   *hfUsage         `json:"usage,omitempty"`
}

type hfStreamChoice struct {
	Index        int             `json:"index"`
	Delta        hfStreamDelta   `json:"delta"`
	FinishReason *string         `json:"finish_reason,omitempty"`
}

type hfStreamDelta struct {
	Role      string             `json:"role,omitempty"`
	Content   string             `json:"content,omitempty"`
	ToolCalls []hfToolCallDelta  `json:"tool_calls,omitempty"`
}

type hfToolCallDelta struct {
	Index    int                  `json:"index"`
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"`
	Function *hfToolCallFnDelta   `json:"function,omitempty"`
}

type hfToolCallFnDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type hfUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- CompleteStream -------------------------------------------------------

func (h *HuggingFaceProvider) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	// Refuse vision attachments at the door. The HF chat-completions
	// shim does not surface image inputs across most model families; we
	// reject up front so the router walks down the chain to a provider
	// that can actually serve the request.
	if len(req.Attachments) > 0 {
		return nil, errors.New("huggingface: vision attachments not supported on this provider")
	}

	out := make(chan Delta, 32)

	model := h.pickModel(req)
	body := h.buildRequestBody(req, model)

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("huggingface: marshal request: %w", err)
	}

	url := h.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("huggingface: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+h.apiKey)

	go func() {
		defer close(out)

		out <- Delta{Type: DeltaStart, Provider: h.Name(), Model: model}

		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("huggingface: http: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Surface HF's structured error body — model-loading errors,
			// rate limits, and quota messages are all in there and far
			// more actionable than the bare status code.
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("huggingface: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))}
			return
		}

		// Aggregate state across the stream.
		var (
			inputTokens   int
			outputTokens  int
			toolCallByIdx = map[int]*hfToolCallDelta{}
		)

		scanner := bufio.NewScanner(resp.Body)
		// Tool-argument fragments can exceed the default 64 KiB line cap
		// when a model streams a whole JSON object as a single delta.
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			if line[0] == ':' {
				// SSE keep-alive comment.
				continue
			}
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			frameBytes := bytes.TrimSpace(line[len("data:"):])
			if len(frameBytes) == 0 {
				continue
			}
			if bytes.Equal(frameBytes, []byte("[DONE]")) {
				break
			}

			var frame hfStreamFrame
			if err := json.Unmarshal(frameBytes, &frame); err != nil {
				out <- Delta{Type: DeltaError, Err: fmt.Errorf("huggingface: decode frame: %w", err)}
				return
			}

			if frame.Usage != nil {
				if frame.Usage.PromptTokens > 0 {
					inputTokens = frame.Usage.PromptTokens
				}
				if frame.Usage.CompletionTokens > 0 {
					outputTokens = frame.Usage.CompletionTokens
				}
			}

			for _, choice := range frame.Choices {
				if choice.Delta.Content != "" {
					out <- Delta{Type: DeltaText, Text: choice.Delta.Content}
				}
				for _, tc := range choice.Delta.ToolCalls {
					tc := tc
					st, ok := toolCallByIdx[tc.Index]
					if !ok {
						st = &hfToolCallDelta{Index: tc.Index, Function: &hfToolCallFnDelta{}}
						toolCallByIdx[tc.Index] = st
					}
					if tc.ID != "" {
						st.ID = tc.ID
					}
					if tc.Function != nil {
						if tc.Function.Name != "" {
							if st.Function == nil {
								st.Function = &hfToolCallFnDelta{}
							}
							st.Function.Name = tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							out <- Delta{
								Type: DeltaToolUse,
								ToolUse: &ToolUseDelta{
									ID:    st.ID,
									Name:  hfFnName(st),
									Input: map[string]any{"_partial": tc.Function.Arguments},
								},
							}
						}
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("huggingface: read stream: %w", err)}
			return
		}

		out <- Delta{
			Type: DeltaDone, Provider: h.Name(), Model: model,
			Usage: &Usage{
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				CostUSD:      estimateHFCost(model, inputTokens, outputTokens),
			},
		}
	}()

	return out, nil
}

func hfFnName(st *hfToolCallDelta) string {
	if st == nil || st.Function == nil {
		return ""
	}
	return st.Function.Name
}

// buildRequestBody assembles the chat-completions payload from a generic
// Request. System + ProjectContext are concatenated into a single system
// message (same ordering as openai.go) so identical project context lands
// on a stable prefix — helpful for HF endpoints that do their own KV
// caching behind the scenes.
func (h *HuggingFaceProvider) buildRequestBody(req Request, model string) hfChatRequest {
	body := hfChatRequest{
		Model:         model,
		Stream:        true,
		StreamOptions: &hfStreamOpts{IncludeUsage: true},
	}

	var sys strings.Builder
	if req.System != "" {
		sys.WriteString(req.System)
	}
	if req.ProjectContext != "" {
		if sys.Len() > 0 {
			sys.WriteString("\n\n")
		}
		sys.WriteString(req.ProjectContext)
	}
	if sys.Len() > 0 {
		body.Messages = append(body.Messages, hfMessage{
			Role:    "system",
			Content: sys.String(),
		})
	}

	body.Messages = append(body.Messages, hfMessage{
		Role:    "user",
		Content: req.Prompt,
	})

	if req.MaxTokens > 0 {
		mt := req.MaxTokens
		body.MaxTokens = &mt
	}
	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	// Structured JSON output: ask the server for json_object when the
	// caller flagged JSON. HF passes this through to compatible
	// inference backends; models that ignore it just emit free-form
	// text, which the JSON gate will catch downstream.
	if req.JSONSchema != "" || containsCap(req.Capabilities, CapJSON) {
		body.ResponseFormat = &hfResponseFormat{Type: "json_object"}
	}

	if len(req.Tools) > 0 {
		tools := make([]hfTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, hfTool{
				Type: "function",
				Function: hfToolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
		body.Tools = tools
	}

	return body
}

// estimateHFCost approximates per-call cost against the published HF
// Inference rate card. Real billing comes from HF's invoice; this is the
// UI / budget-guard meter only. Unknown models default to Llama 3.3 70B
// rates so the budget guard errs on the safe side.
func estimateHFCost(model string, inputTok, outputTok int) float64 {
	var inP, outP float64
	switch {
	case contains(model, "Llama-3.1-405B"), contains(model, "llama-3.1-405b"):
		inP, outP = 1.79, 1.79
	case contains(model, "Mixtral-8x22B"), contains(model, "mixtral-8x22b"):
		inP, outP = 0.65, 0.65
	case contains(model, "DeepSeek-V3"), contains(model, "deepseek-v3"):
		inP, outP = 0.27, 1.10
	case contains(model, "Qwen2.5-7B"), contains(model, "qwen2.5-7b"):
		inP, outP = 0.20, 0.20
	case contains(model, "Llama-3.3-70B"), contains(model, "llama-3.3-70b"):
		inP, outP = 0.35, 0.40
	default:
		inP, outP = 0.35, 0.40
	}
	const m = 1_000_000.0
	return (float64(inputTok)*inP + float64(outputTok)*outP) / m
}

var _ Provider = (*HuggingFaceProvider)(nil)
