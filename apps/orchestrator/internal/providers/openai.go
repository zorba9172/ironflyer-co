package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIProvider is the stdlib-only OpenAI Chat Completions client. It
// streams via SSE and exposes the same Delta semantics as anthropic.go so
// the router treats the two interchangeably.
//
// Design notes:
//   - We deliberately avoid the official openai-go SDK so the orchestrator
//     stays light on third-party deps. The Chat Completions schema is
//     stable enough that hand-rolled JSON is the right trade.
//   - Prompt caching on OpenAI is automatic on identical prefixes; there
//     are no cache-control markers to attach. We still advertise CapCache
//     so the router will prefer this provider for long-context calls.
//   - SSE frames are read with bufio.Scanner and a 1 MiB buffer — large
//     tool-arguments deltas (whole JSON payloads streamed as one fragment)
//     would overflow the default 64 KiB line cap otherwise.
type OpenAIProvider struct {
	apiKey     string
	model      string
	cheapModel string
	powerModel string
	baseURL    string
	httpClient *http.Client
}

type OpenAIOpts struct {
	APIKey     string
	Model      string // default "gpt-4o"
	CheapModel string // default "gpt-4o-mini" — selected when CapCheap is requested
	PowerModel string // default "o3" — selected when CapReasoning/CapThinking
	BaseURL    string // override for Azure / proxies; empty -> https://api.openai.com/v1
}

func NewOpenAIProvider(opts OpenAIOpts) *OpenAIProvider {
	model := opts.Model
	if model == "" {
		model = "gpt-4o"
	}
	cheap := opts.CheapModel
	if cheap == "" {
		cheap = "gpt-4o-mini"
	}
	power := opts.PowerModel
	if power == "" {
		power = "o3"
	}
	base := opts.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	base = strings.TrimRight(base, "/")
	return &OpenAIProvider{
		apiKey:     opts.APIKey,
		model:      model,
		cheapModel: cheap,
		powerModel: power,
		baseURL:    base,
		// No timeout: streaming responses can legitimately run for minutes.
		// The caller's context governs cancellation.
		httpClient: &http.Client{Timeout: 0},
	}
}

func (o *OpenAIProvider) Name() string { return "openai" }

func (o *OpenAIProvider) Capabilities() []Capability {
	return []Capability{
		CapReasoning, CapCode, CapJSON, CapVision, CapTools, CapCache,
		// CapInline — gpt-4o-mini is the cheap+fast fit for Cursor-
		// style middle-fill-in completions; advertising it here lets
		// the router score OpenAI alongside Anthropic when the caller
		// asks for inline completions.
		CapInline,
	}
}

// pickModel mirrors anthropic.go: cheap/fast/inline win outright (any of
// those = "we'd rather pay mini rates"), thinking / reasoning / quality
// promote to the power tier, otherwise the configured base model is
// used. gpt-4o-mini is the canonical inline-completion default — fast
// and cheap enough for Cursor-style ghost-text without taxing the
// ledger.
func (o *OpenAIProvider) pickModel(req Request) string {
	for _, c := range req.Capabilities {
		if c == CapCheap || c == CapFast || c == CapInline {
			return o.cheapModel
		}
	}
	if req.EnableThinking {
		return o.powerModel
	}
	for _, c := range req.Capabilities {
		if c == CapQuality || c == CapThinking || c == CapReasoning {
			return o.powerModel
		}
	}
	return o.model
}

// --- request DTOs ---------------------------------------------------------

type openAIChatRequest struct {
	Model               string                 `json:"model"`
	Messages            []openAIMessage        `json:"messages"`
	Stream              bool                   `json:"stream"`
	StreamOptions       *openAIStreamOpts      `json:"stream_options,omitempty"`
	MaxCompletionTokens *int                   `json:"max_completion_tokens,omitempty"`
	Temperature         *float32               `json:"temperature,omitempty"`
	Tools               []openAITool           `json:"tools,omitempty"`
	ResponseFormat      *openAIResponseFormat  `json:"response_format,omitempty"`
}

type openAIStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	// Content is either a string (simple text message) or an array of
	// content parts (multimodal). We let json.Marshal pick by type.
	Content any `json:"content"`
}

type openAIContentPart struct {
	Type     string             `json:"type"`
	Text     string             `json:"text,omitempty"`
	ImageURL *openAIImageURL    `json:"image_url,omitempty"`
}

type openAIImageURL struct {
	URL string `json:"url"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// --- streaming response DTOs ---------------------------------------------

type openAIStreamFrame struct {
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *openAIUsage         `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Index        int                `json:"index"`
	Delta        openAIStreamDelta  `json:"delta"`
	FinishReason *string            `json:"finish_reason,omitempty"`
}

type openAIStreamDelta struct {
	Role      string                 `json:"role,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ToolCalls []openAIToolCallDelta  `json:"tool_calls,omitempty"`
}

type openAIToolCallDelta struct {
	Index    int                   `json:"index"`
	ID       string                `json:"id,omitempty"`
	Type     string                `json:"type,omitempty"`
	Function *openAIToolCallFnDelta `json:"function,omitempty"`
}

type openAIToolCallFnDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openAIUsage struct {
	PromptTokens          int                       `json:"prompt_tokens"`
	CompletionTokens      int                       `json:"completion_tokens"`
	TotalTokens           int                       `json:"total_tokens"`
	PromptTokensDetails   *openAIPromptTokensDetail `json:"prompt_tokens_details,omitempty"`
}

type openAIPromptTokensDetail struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// --- CompleteStream -------------------------------------------------------

func (o *OpenAIProvider) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	out := make(chan Delta, 32)

	model := o.pickModel(req)
	body := o.buildRequestBody(req, model)

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	url := o.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	go func() {
		defer close(out)

		out <- Delta{Type: DeltaStart, Provider: o.Name(), Model: model}

		resp, err := o.httpClient.Do(httpReq)
		if err != nil {
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("openai: http: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Drain the body to surface the API's error message — OpenAI
			// returns structured JSON with `error.message` that's far more
			// actionable than the bare status code.
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("openai: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))}
			return
		}

		// Aggregate state across the stream.
		var (
			inputTokens    int
			outputTokens   int
			cacheReadToks  int
			toolCallByIdx  = map[int]*openAIToolCallDelta{}
		)

		scanner := bufio.NewScanner(resp.Body)
		// SSE chunks (especially tool_calls.arguments fragments) can exceed
		// the default 64 KiB line cap. Give the scanner room to breathe.
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			// SSE spec: lines starting with ':' are comments. OpenAI uses
			// them as keep-alives on idle streams.
			if line[0] == ':' {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data:")) {
				// "event:" or other SSE fields — ignore.
				continue
			}
			payload := bytes.TrimSpace(line[len("data:"):])
			if len(payload) == 0 {
				continue
			}
			if bytes.Equal(payload, []byte("[DONE]")) {
				break
			}

			var frame openAIStreamFrame
			if err := json.Unmarshal(payload, &frame); err != nil {
				out <- Delta{Type: DeltaError, Err: fmt.Errorf("openai: decode frame: %w", err)}
				return
			}

			if frame.Usage != nil {
				if frame.Usage.PromptTokens > 0 {
					inputTokens = frame.Usage.PromptTokens
				}
				if frame.Usage.CompletionTokens > 0 {
					outputTokens = frame.Usage.CompletionTokens
				}
				if frame.Usage.PromptTokensDetails != nil && frame.Usage.PromptTokensDetails.CachedTokens > 0 {
					cacheReadToks = frame.Usage.PromptTokensDetails.CachedTokens
				}
			}

			for _, choice := range frame.Choices {
				if choice.Delta.Content != "" {
					out <- Delta{Type: DeltaText, Text: choice.Delta.Content}
				}
				for _, tc := range choice.Delta.ToolCalls {
					tc := tc
					// OpenAI streams tool calls in fragments keyed by
					// index: the first fragment carries id+name, the
					// rest carry arguments slices. We track the running
					// id/name per index so each emitted DeltaToolUse is
					// addressable end-to-end even when the UI only sees
					// the partial argument shards.
					st, ok := toolCallByIdx[tc.Index]
					if !ok {
						st = &openAIToolCallDelta{Index: tc.Index, Function: &openAIToolCallFnDelta{}}
						toolCallByIdx[tc.Index] = st
					}
					if tc.ID != "" {
						st.ID = tc.ID
					}
					if tc.Function != nil {
						if tc.Function.Name != "" {
							if st.Function == nil {
								st.Function = &openAIToolCallFnDelta{}
							}
							st.Function.Name = tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							out <- Delta{
								Type: DeltaToolUse,
								ToolUse: &ToolUseDelta{
									ID:    st.ID,
									Name:  fnName(st),
									Input: map[string]any{"_partial": tc.Function.Arguments},
								},
							}
						}
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("openai: read stream: %w", err)}
			return
		}

		out <- Delta{
			Type: DeltaDone, Provider: o.Name(), Model: model,
			Usage: &Usage{
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				CacheReadTokens: cacheReadToks,
				CostUSD:         estimateOpenAICost(model, inputTokens, outputTokens),
			},
		}
	}()

	return out, nil
}

func fnName(st *openAIToolCallDelta) string {
	if st == nil || st.Function == nil {
		return ""
	}
	return st.Function.Name
}

// buildRequestBody assembles the Chat Completions payload from a generic
// Request. System + ProjectContext are concatenated into a single system
// message — OpenAI auto-caches identical prefixes, so the concat order
// matters for cache hit rate (largest stable block first).
func (o *OpenAIProvider) buildRequestBody(req Request, model string) openAIChatRequest {
	body := openAIChatRequest{
		Model:         model,
		Stream:        true,
		StreamOptions: &openAIStreamOpts{IncludeUsage: true},
	}

	// System block: concat System then ProjectContext. ProjectContext is
	// typically the larger, more stable chunk; placing it after System
	// keeps the per-call instructions on top where humans read them
	// without breaking the cacheable prefix (everything up through the
	// last identical byte is what OpenAI caches).
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
		body.Messages = append(body.Messages, openAIMessage{
			Role:    "system",
			Content: sys.String(),
		})
	}

	// User turn.
	if len(req.Attachments) == 0 {
		body.Messages = append(body.Messages, openAIMessage{
			Role:    "user",
			Content: req.Prompt,
		})
	} else {
		parts := make([]openAIContentPart, 0, len(req.Attachments)+1)
		for _, att := range req.Attachments {
			if att.Base64 == "" {
				continue
			}
			mt := att.MediaType
			if mt == "" {
				mt = "image/png"
			}
			parts = append(parts, openAIContentPart{
				Type:     "image_url",
				ImageURL: &openAIImageURL{URL: "data:" + mt + ";base64," + att.Base64},
			})
		}
		parts = append(parts, openAIContentPart{Type: "text", Text: req.Prompt})
		body.Messages = append(body.Messages, openAIMessage{
			Role:    "user",
			Content: parts,
		})
	}

	if req.MaxTokens > 0 {
		mt := req.MaxTokens
		body.MaxCompletionTokens = &mt
	}
	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	// JSONSchema: when the caller asked for structured JSON output we ask
	// the API for json_object. (Full schema enforcement uses a different
	// response_format type; we keep it conservative here so this layer
	// stays compatible across the gpt-4o / o3 fleet.)
	if req.JSONSchema != "" || containsCap(req.Capabilities, CapJSON) {
		body.ResponseFormat = &openAIResponseFormat{Type: "json_object"}
	}

	if len(req.Tools) > 0 {
		tools := make([]openAITool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, openAITool{
				Type: "function",
				Function: openAIToolFunction{
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

// estimateOpenAICost rough-bills tokens against the published OpenAI rate
// card. Real billing reads from the OpenAI invoice; this is the UI cost
// meter only.
func estimateOpenAICost(model string, inputTok, outputTok int) float64 {
	// per 1M tokens (USD).
	var inP, outP float64
	switch {
	case contains(model, "gpt-4o-mini"):
		inP, outP = 0.15, 0.60
	case contains(model, "gpt-4o"):
		inP, outP = 2.50, 10.00
	case contains(model, "o3"):
		inP, outP = 2.00, 8.00
	default:
		inP, outP = 2.50, 10.00
	}
	const m = 1_000_000.0
	return (float64(inputTok)*inP + float64(outputTok)*outP) / m
}

var _ Provider = (*OpenAIProvider)(nil)
