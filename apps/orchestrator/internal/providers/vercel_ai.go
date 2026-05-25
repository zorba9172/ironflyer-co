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

// VercelAIGatewayProvider speaks the Vercel AI Gateway — an OpenAI-
// compatible API that proxies multiple LLM backends (Anthropic, OpenAI,
// Mistral, Google, etc.) with built-in caching, rate-limiting, and
// per-team observability. The wire shape is OpenAI Chat Completions, so
// we crib most of the streaming machinery from openai.go and only
// override:
//
//   - the base URL (gateway.ai.vercel.com/v1 by default, overridable
//     via VERCEL_AI_GATEWAY_URL),
//   - the auth token (VERCEL_AI_GATEWAY_TOKEN),
//   - the model id format (Vercel namespaces models as `<vendor>/<id>`
//     — e.g. `anthropic/claude-sonnet-4-6`, `openai/gpt-4o`).
//
// When to enable: operators who want a single billing relationship +
// caching layer across multiple LLM vendors, or who already pay for
// Vercel and want to consolidate. Direct providers (Anthropic / OpenAI
// / Gemini) stay first-class — the gateway is registered alongside as
// an additional arm the bandit can pick when it wins on reward.
type VercelAIGatewayProvider struct {
	token      string
	baseURL    string
	model      string // default vendor-namespaced model
	cheapModel string
	powerModel string
	httpClient *http.Client
}

// VercelAIGatewayOpts configures the provider. All fields are optional
// except Token — leave Token empty and the registration site MUST skip
// constructing the provider (NewVercelAIGatewayProvider returns nil so
// the gating is impossible to forget).
type VercelAIGatewayOpts struct {
	Token      string
	BaseURL    string // default https://gateway.ai.vercel.com/v1
	Model      string // default anthropic/claude-sonnet-4-6
	CheapModel string // default anthropic/claude-haiku-4-5-20251001
	PowerModel string // default anthropic/claude-opus-4-7
}

// DefaultVercelAIGatewayBaseURL is the published gateway endpoint. The
// gateway has been moving fast — operators can override via
// VERCEL_AI_GATEWAY_URL if Vercel renames the host or shards by region.
const DefaultVercelAIGatewayBaseURL = "https://gateway.ai.vercel.com/v1"

// NewVercelAIGatewayProvider constructs the provider. Returns nil when
// Token is empty so the caller's registration block reads "create →
// register only if non-nil" without needing a separate gating check.
func NewVercelAIGatewayProvider(opts VercelAIGatewayOpts) *VercelAIGatewayProvider {
	if strings.TrimSpace(opts.Token) == "" {
		return nil
	}
	base := opts.BaseURL
	if base == "" {
		base = DefaultVercelAIGatewayBaseURL
	}
	base = strings.TrimRight(base, "/")
	model := opts.Model
	if model == "" {
		model = "anthropic/claude-sonnet-4-6"
	}
	cheap := opts.CheapModel
	if cheap == "" {
		cheap = "anthropic/claude-haiku-4-5-20251001"
	}
	power := opts.PowerModel
	if power == "" {
		power = "anthropic/claude-opus-4-7"
	}
	return &VercelAIGatewayProvider{
		token:      opts.Token,
		baseURL:    base,
		model:      model,
		cheapModel: cheap,
		powerModel: power,
		// Streaming responses can run for minutes; caller context governs
		// cancellation. Same shape as openai.go.
		httpClient: &http.Client{Timeout: 0},
	}
}

func (v *VercelAIGatewayProvider) Name() string { return "vercel-ai" }

// Capabilities advertises the same broad set as OpenAI — the gateway is
// vendor-agnostic so we can't narrow further at this layer. Operators
// who want to pin a specific upstream choose by VERCEL_AI_GATEWAY_MODEL
// (e.g. force `openai/gpt-4o-mini` for inline completions).
func (v *VercelAIGatewayProvider) Capabilities() []Capability {
	return []Capability{
		CapReasoning, CapCode, CapJSON, CapVision, CapTools, CapCache,
		CapCheap, CapFast, CapQuality, CapInline,
	}
}

// pickModel mirrors the other providers' tiering: cheap/fast/inline →
// cheapModel, quality/thinking/reasoning → powerModel, default →
// configured Model. The gateway honours both vendor-namespaced ids
// (`anthropic/claude-haiku-4-5-20251001`) and short ids when the
// gateway has a default routing rule configured.
func (v *VercelAIGatewayProvider) pickModel(req Request) string {
	for _, c := range req.Capabilities {
		if c == CapCheap || c == CapFast || c == CapInline {
			return v.cheapModel
		}
	}
	if req.EnableThinking {
		return v.powerModel
	}
	for _, c := range req.Capabilities {
		if c == CapQuality || c == CapThinking || c == CapReasoning {
			return v.powerModel
		}
	}
	return v.model
}

// CompleteStream wires the gateway's OpenAI-shaped streaming endpoint
// to the router's Delta channel. The hot path is identical to
// OpenAIProvider.CompleteStream — same SSE framing, same usage shape —
// so we reuse the openAIChatRequest / openAIStreamFrame DTOs and only
// the base URL + auth header differ. Mirroring the OpenAI shape keeps
// upstream API drift on Vercel's side cheap to absorb.
func (v *VercelAIGatewayProvider) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	out := make(chan Delta, 32)

	model := v.pickModel(req)
	body := v.buildRequestBody(req, model)

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("vercel-ai: marshal request: %w", err)
	}

	url := v.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("vercel-ai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+v.token)

	go func() {
		defer close(out)

		out <- Delta{Type: DeltaStart, Provider: v.Name(), Model: model}

		resp, err := v.httpClient.Do(httpReq)
		if err != nil {
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("vercel-ai: http: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("vercel-ai: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))}
			return
		}

		var (
			inputTokens   int
			outputTokens  int
			cacheReadToks int
			toolCallByIdx = map[int]*openAIToolCallDelta{}
		)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			if line[0] == ':' {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data:")) {
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
				out <- Delta{Type: DeltaError, Err: fmt.Errorf("vercel-ai: decode frame: %w", err)}
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
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("vercel-ai: read stream: %w", err)}
			return
		}

		out <- Delta{
			Type: DeltaDone, Provider: v.Name(), Model: model,
			Usage: &Usage{
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				CacheReadTokens: cacheReadToks,
				// Vercel re-bills upstream cost + a small gateway markup;
				// we can't know the exact rate per route without polling
				// their pricing API, so we approximate from the embedded
				// vendor prefix. The real ledger reads from Vercel's
				// invoice; this is the UI cost meter only.
				CostUSD: estimateVercelAIGatewayCost(model, inputTokens, outputTokens),
			},
		}
	}()

	return out, nil
}

// buildRequestBody is a thin wrapper that re-uses the OpenAI request
// shape; the gateway documents 1:1 OpenAI compat for /chat/completions.
func (v *VercelAIGatewayProvider) buildRequestBody(req Request, model string) openAIChatRequest {
	body := openAIChatRequest{
		Model:         model,
		Stream:        true,
		StreamOptions: &openAIStreamOpts{IncludeUsage: true},
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
		body.Messages = append(body.Messages, openAIMessage{
			Role:    "system",
			Content: sys.String(),
		})
	}

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

// estimateVercelAIGatewayCost approximates by inspecting the vendor
// prefix of the model id (Vercel namespaces as `<vendor>/<id>`). The
// real bill comes from Vercel's invoice; this is the UI meter only.
func estimateVercelAIGatewayCost(model string, inputTok, outputTok int) float64 {
	low := strings.ToLower(model)
	switch {
	case strings.HasPrefix(low, "anthropic/"):
		// Approximate by stripping the prefix and reusing the Anthropic
		// rate card. estimateCost handles unknown ids defensively.
		return estimateCost(strings.TrimPrefix(low, "anthropic/"), inputTok, outputTok, 0, 0)
	case strings.HasPrefix(low, "openai/"):
		return estimateOpenAICost(strings.TrimPrefix(low, "openai/"), inputTok, outputTok)
	case strings.HasPrefix(low, "google/"), strings.HasPrefix(low, "gemini/"):
		return estimateGeminiCost(strings.TrimPrefix(strings.TrimPrefix(low, "google/"), "gemini/"), inputTok, outputTok, 0)
	default:
		// Conservative fallback — assume Sonnet-class rates so the UI
		// over-bills slightly rather than under-bills.
		return estimateCost("claude-sonnet-4-6", inputTok, outputTok, 0, 0)
	}
}

var _ Provider = (*VercelAIGatewayProvider)(nil)
