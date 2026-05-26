// DeepSeek provider — OpenAI-compatible REST + SSE at api.deepseek.com.
//
// DeepSeek's wire format is a 1:1 copy of OpenAI's Chat Completions, so
// the streaming machinery (SSE framing, tool-call fragments, usage
// shape) is cribbed from openai.go / vercel_ai.go. We reuse the
// openAIChatRequest / openAIStreamFrame DTOs directly — re-declaring
// them would just create drift the first time DeepSeek matches an
// OpenAI patch we forget to mirror.
//
// The model picker maps capabilities to one of three SKUs:
//
//   - deepseek-chat (V3) — general-purpose; also the default for code
//     when DEEPSEEK_PREFER_V3_FOR_CODE=true (V3 is now the strongest of
//     the family on code benchmarks).
//   - deepseek-reasoner (R1) — reasoning; competitive with Opus 4.7 at
//     roughly 3% the price. Selected for CapReasoning / CapThinking and
//     when the caller flips EnableThinking.
//   - deepseek-coder — legacy code-tuned SKU; selected for CapCode when
//     the V3 override is off.
//
// Hard-disable posture: DeepSeek is a Chinese provider; enterprise
// operators with compliance constraints may need to keep it off even
// when an API key is configured. The Enabled flag is the kill switch —
// CompleteStream returns ErrProviderDisabled and the router rolls over
// to the next arm. Setting Enabled=false at boot also gates the
// registration in main.go so the bandit never sees DeepSeek as an arm.

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

// ErrProviderDisabled is returned by a provider whose operator-level
// kill switch has been flipped off. The failover layer treats it as a
// transient skip so the next provider in the chain serves the request;
// no breaker trip, no telemetry penalty — the provider is simply
// excluded by policy.
var ErrProviderDisabled = errors.New("provider: disabled by operator policy")

// DefaultDeepSeekBaseURL is the published v1 endpoint. The /beta path
// is also OpenAI-compatible and exposes the latest features; operators
// who need it set DEEPSEEK_BASE_URL=https://api.deepseek.com/beta.
const DefaultDeepSeekBaseURL = "https://api.deepseek.com/v1"

// DeepSeekConfig configures the provider. Token + Enabled together
// drive registration: when either is unset DeepSeek registers as
// disabled and never receives traffic.
type DeepSeekConfig struct {
	Token          string
	BaseURL        string // default https://api.deepseek.com/v1
	GeneralModel   string // default deepseek-chat (V3)
	ReasoningModel string // default deepseek-reasoner (R1)
	CoderModel     string // default deepseek-coder
	// PreferV3ForCode flips CapCode from deepseek-coder to deepseek-chat.
	// V3 outscores the legacy coder SKU on most code benchmarks; the
	// flag exists so operators can opt back into the coder SKU if their
	// own evals show otherwise. Mapped from DEEPSEEK_PREFER_V3_FOR_CODE.
	PreferV3ForCode bool
	// Enabled is the operator-level kill switch. False = hard-disable
	// independent of the API key. True + non-empty Token = register.
	Enabled bool
}

// DeepSeek is the streaming-first provider implementation. It speaks
// OpenAI Chat Completions over SSE — see openai.go for the canonical
// wire-format walkthrough; this file only overrides URL + auth +
// model selection.
type DeepSeek struct {
	token        string
	baseURL      string
	general      string
	reasoning    string
	coder        string
	preferV3Code bool
	enabled      bool
	httpClient   *http.Client

	// billing + telemetry are stashed for symmetry with the constructor
	// contract (and to keep a single seam if we ever want per-provider
	// charging that bypasses the router-level BillingGuard). The hot
	// path does not call them — DeltaDone carries Usage and the
	// router's BillingGuard wraps the charge at the channel boundary,
	// same as every other provider.
	billing   *BillingGuard
	telemetry TelemetrySink
}

// NewDeepSeek constructs the provider. Returns a non-nil *DeepSeek
// even when disabled — registration sites still need a value to
// expose via /providers/health so operators can see "deepseek:
// disabled" without grepping logs.
func NewDeepSeek(cfg DeepSeekConfig, billing *BillingGuard, telemetry TelemetrySink) (*DeepSeek, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = DefaultDeepSeekBaseURL
	}
	base = strings.TrimRight(base, "/")

	general := strings.TrimSpace(cfg.GeneralModel)
	if general == "" {
		general = "deepseek-chat"
	}
	reasoning := strings.TrimSpace(cfg.ReasoningModel)
	if reasoning == "" {
		reasoning = "deepseek-reasoner"
	}
	coder := strings.TrimSpace(cfg.CoderModel)
	if coder == "" {
		coder = "deepseek-coder"
	}

	return &DeepSeek{
		token:        strings.TrimSpace(cfg.Token),
		baseURL:      base,
		general:      general,
		reasoning:    reasoning,
		coder:        coder,
		preferV3Code: cfg.PreferV3ForCode,
		enabled:      cfg.Enabled,
		httpClient:   streamingHTTPClient(),
		billing:      billing,
		telemetry:    telemetry,
	}, nil
}

// Name is the stable identifier used by the bandit, breaker registry,
// telemetry sink, and audit/metrics labels. Keep it ASCII + lowercase.
func (d *DeepSeek) Name() string { return "deepseek" }

// Capabilities advertises the surface area DeepSeek covers:
//
//   - CapGeneral is implicit (every provider serves the no-cap default).
//   - CapCode + CapJSON + CapTools are first-class — both V3 and the
//     coder SKU handle function-calling and structured outputs.
//   - CapReasoning is honoured via deepseek-reasoner (R1).
//   - CapCheap / CapFast / CapInline reflect DeepSeek's price point:
//     ≈$0.14 / 1M input, $0.28 / 1M output for V3 is cheaper than most
//     incumbents and fast enough for inline completions.
//
// CapVision is deliberately omitted — DeepSeek's vision story is
// limited and the router would otherwise route image attachments here
// and 400 mid-stream.
func (d *DeepSeek) Capabilities() []Capability {
	return []Capability{
		CapCode, CapJSON, CapTools, CapReasoning,
		CapCheap, CapFast, CapInline,
	}
}

// pickModel maps the request to one of the three SKUs. Reasoning wins
// outright — R1 is the differentiator. CapCode normally maps to the
// legacy coder SKU; the PreferV3ForCode override redirects to V3,
// which is the stronger model on current benchmarks. Anything else
// falls through to the general-purpose default.
func (d *DeepSeek) pickModel(req Request) string {
	if req.EnableThinking {
		return d.reasoning
	}
	for _, c := range req.Capabilities {
		if c == CapReasoning || c == CapThinking {
			return d.reasoning
		}
	}
	for _, c := range req.Capabilities {
		if c == CapCode {
			if d.preferV3Code {
				return d.general
			}
			return d.coder
		}
	}
	return d.general
}

// CompleteStream is the streaming entry point. Disabled providers
// short-circuit with ErrProviderDisabled before any network I/O so the
// failover layer can roll the request over to the next arm without
// observing latency from a no-op call.
func (d *DeepSeek) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	if !d.enabled || d.token == "" {
		return nil, ErrProviderDisabled
	}

	model := d.pickModel(req)
	body := d.buildRequestBody(req, model)

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("deepseek: marshal request: %w", err)
	}

	url := d.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("deepseek: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+d.token)

	out := make(chan Delta, 32)
	go func() {
		defer close(out)

		out <- Delta{Type: DeltaStart, Provider: d.Name(), Model: model}

		resp, err := d.httpClient.Do(httpReq)
		if err != nil {
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("deepseek: http: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("deepseek: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))}
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
				out <- Delta{Type: DeltaError, Err: fmt.Errorf("deepseek: decode frame: %w", err)}
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
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("deepseek: read stream: %w", err)}
			return
		}

		out <- Delta{
			Type: DeltaDone, Provider: d.Name(), Model: model,
			Usage: &Usage{
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				CacheReadTokens: cacheReadToks,
				CostUSD:         estimateDeepSeekCost(model, inputTokens, outputTokens, cacheReadToks),
			},
		}
	}()

	return out, nil
}

// buildRequestBody mirrors the OpenAI request shape. DeepSeek auto-
// caches identical prefixes on the input side (see deepseekRates'
// cache-read tier), so ordering is the same as openai.go: per-call
// System first, then the larger, more stable ProjectContext.
func (d *DeepSeek) buildRequestBody(req Request, model string) openAIChatRequest {
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

	// DeepSeek does not advertise vision; we still forward the prompt
	// as a plain user turn and ignore any Attachments — the router has
	// already filtered DeepSeek out when CapVision was promoted.
	body.Messages = append(body.Messages, openAIMessage{
		Role:    "user",
		Content: req.Prompt,
	})

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

// deepseekRate is the per-1M-token rate card for one SKU. CacheInputUSD
// is DeepSeek's cache-hit rate (much cheaper than the raw input rate)
// applied to PromptTokensDetails.CachedTokens.
type deepseekRate struct {
	InputUSD       float64
	CacheInputUSD  float64
	OutputUSD      float64
}

// deepseekRates is the public list pricing as of 2026-Q2 (USD per 1M
// tokens). Operators can override per-deployment in a follow-up by
// reading these from env; for now the UI cost meter is approximate
// and the ledger of record is DeepSeek's own invoice.
var deepseekRates = map[string]deepseekRate{
	"deepseek-chat":      {InputUSD: 0.14, CacheInputUSD: 0.014, OutputUSD: 0.28},
	"deepseek-reasoner":  {InputUSD: 0.55, CacheInputUSD: 0.14, OutputUSD: 2.19},
	"deepseek-coder":     {InputUSD: 0.14, CacheInputUSD: 0.014, OutputUSD: 0.28},
}

// estimateDeepSeekCost rough-bills tokens against deepseekRates. The
// cache-read tier is applied to the cached portion of the prompt; the
// remaining input tokens pay the full rate.
func estimateDeepSeekCost(model string, inputTok, outputTok, cacheReadTok int) float64 {
	rate, ok := deepseekRates[strings.ToLower(strings.TrimSpace(model))]
	if !ok {
		// Unknown SKU — default to V3 rates so the UI over-bills
		// slightly rather than under-bills on an unexpected model id.
		rate = deepseekRates["deepseek-chat"]
	}
	const m = 1_000_000.0
	billedInput := inputTok - cacheReadTok
	if billedInput < 0 {
		billedInput = 0
	}
	return (float64(billedInput)*rate.InputUSD +
		float64(cacheReadTok)*rate.CacheInputUSD +
		float64(outputTok)*rate.OutputUSD) / m
}

var _ Provider = (*DeepSeek)(nil)
