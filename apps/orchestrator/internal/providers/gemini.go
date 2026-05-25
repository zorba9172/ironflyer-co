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

// GeminiProvider is Google Gemini wired through the streaming-first
// Provider contract using only stdlib HTTP. We avoid the official SDK so the
// orchestrator stays light on third-party deps — Gemini's REST surface is
// stable enough that a hand-rolled client is fine.
//
// Tiering mirrors the Anthropic provider:
//   - CapCheap          → CheapModel (gemini-2.5-flash)
//   - CapThinking/CapReasoning → PowerModel (gemini-2.5-pro w/ thinking)
//   - default           → Model (gemini-2.5-pro)
type GeminiProvider struct {
	apiKey     string
	model      string
	cheapModel string
	powerModel string
	baseURL    string
	httpClient *http.Client
}

// GeminiOpts configures the provider. Empty fields fall back to sensible
// defaults — operators can pin specific model versions via env wiring at
// the call site.
type GeminiOpts struct {
	APIKey     string
	Model      string // default "gemini-2.5-pro"
	CheapModel string // default "gemini-2.5-flash"
	PowerModel string // default "gemini-2.5-pro" (with thinking)
	BaseURL    string // override; empty -> https://generativelanguage.googleapis.com/v1beta
}

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

func NewGeminiProvider(opts GeminiOpts) *GeminiProvider {
	model := opts.Model
	if model == "" {
		model = "gemini-2.5-pro"
	}
	cheap := opts.CheapModel
	if cheap == "" {
		cheap = "gemini-2.5-flash"
	}
	power := opts.PowerModel
	if power == "" {
		power = "gemini-2.5-pro"
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultGeminiBaseURL
	}
	base = strings.TrimRight(base, "/")
	return &GeminiProvider{
		apiKey:     opts.APIKey,
		model:      model,
		cheapModel: cheap,
		powerModel: power,
		baseURL:    base,
		httpClient: &http.Client{},
	}
}

func (g *GeminiProvider) Name() string { return "gemini" }

func (g *GeminiProvider) Capabilities() []Capability {
	return []Capability{
		CapReasoning, CapCode, CapJSON, CapVision,
		CapTools, CapCache, CapThinking,
	}
}

// pickModel applies the cost-aware tier policy. cheap/fast/inline win
// outright (mapped to gemini-2.5-flash); quality/thinking/reasoning
// promote to the pro model; otherwise the base model stands.
func (g *GeminiProvider) pickModel(req Request) string {
	for _, c := range req.Capabilities {
		if c == CapCheap || c == CapFast || c == CapInline {
			return g.cheapModel
		}
	}
	if req.EnableThinking {
		return g.powerModel
	}
	for _, c := range req.Capabilities {
		if c == CapQuality || c == CapThinking || c == CapReasoning {
			return g.powerModel
		}
	}
	return g.model
}

// --- request body shapes -------------------------------------------------

type geminiInlineData struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiPart struct {
	Text         string            `json:"text,omitempty"`
	InlineData   *geminiInlineData `json:"inline_data,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiThinkingConfig struct {
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
	ThinkingBudget  int  `json:"thinkingBudget,omitempty"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens  int                   `json:"maxOutputTokens,omitempty"`
	ResponseMIMEType string                `json:"responseMimeType,omitempty"`
	ThinkingConfig   *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"function_declarations"`
}

type geminiRequest struct {
	SystemInstruction *geminiSystemInstruction `json:"system_instruction,omitempty"`
	Contents          []geminiContent          `json:"contents"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
	Tools             []geminiTool             `json:"tools,omitempty"`
}

// --- response body shapes ------------------------------------------------

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiResponsePart struct {
	Text         string              `json:"text"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiResponseContent struct {
	Parts []geminiResponsePart `json:"parts"`
	Role  string               `json:"role"`
}

type geminiCandidate struct {
	Content      geminiResponseContent `json:"content"`
	FinishReason string                `json:"finishReason"`
	Index        int                   `json:"index"`
}

type geminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
	TotalTokenCount         int `json:"totalTokenCount"`
}

type geminiStreamFrame struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// CompleteStream wires Gemini's SSE-flavoured streaming endpoint to the
// router's Delta channel. The output channel is owned by the goroutine
// spawned here and is closed exactly once on exit, regardless of branch.
func (g *GeminiProvider) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	out := make(chan Delta, 32)

	model := g.pickModel(req)
	body, err := buildGeminiBody(req)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini marshal: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s",
		g.baseURL, model, g.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gemini new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini http: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		// Drain a bounded slice of the body for context, then bail before
		// kicking off the streamer — the router fallback chain depends on
		// "couldn't even start" errors surfacing here rather than mid-stream.
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("gemini http status %d: %s", resp.StatusCode, strings.TrimSpace(string(buf)))
	}

	go func() {
		defer close(out)
		defer resp.Body.Close()

		out <- Delta{Type: DeltaStart, Provider: g.Name(), Model: model}

		scanner := bufio.NewScanner(resp.Body)
		// SSE frames carrying full Gemini responses can blow past the default
		// 64KB scanner buffer; bump the max so attachments + tool calls don't
		// silently truncate.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var (
			inputTokens, outputTokens, cacheReadTokens int
		)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(line[len("data:"):])
			if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
				continue
			}

			var frame geminiStreamFrame
			if err := json.Unmarshal(payload, &frame); err != nil {
				// Malformed frames are ignored — Gemini occasionally inserts
				// keepalives or comment lines that look like data but aren't
				// JSON. We don't want to abort an in-flight completion over
				// one bad chunk.
				continue
			}

			for _, cand := range frame.Candidates {
				for _, part := range cand.Content.Parts {
					if part.FunctionCall != nil {
						out <- Delta{
							Type: DeltaToolUse,
							ToolUse: &ToolUseDelta{
								Name:  part.FunctionCall.Name,
								Input: part.FunctionCall.Args,
							},
						}
						continue
					}
					if part.Text != "" {
						out <- Delta{Type: DeltaText, Text: part.Text}
					}
				}
			}

			if frame.UsageMetadata != nil {
				if frame.UsageMetadata.PromptTokenCount > 0 {
					inputTokens = frame.UsageMetadata.PromptTokenCount
				}
				if frame.UsageMetadata.CandidatesTokenCount > 0 {
					outputTokens = frame.UsageMetadata.CandidatesTokenCount
				}
				if frame.UsageMetadata.CachedContentTokenCount > 0 {
					cacheReadTokens = frame.UsageMetadata.CachedContentTokenCount
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("gemini stream: %w", err)}
			return
		}

		out <- Delta{
			Type: DeltaDone, Provider: g.Name(), Model: model,
			Usage: &Usage{
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				CacheReadTokens: cacheReadTokens,
				CostUSD:         estimateGeminiCost(model, inputTokens, outputTokens, cacheReadTokens),
			},
		}
	}()

	return out, nil
}

// buildGeminiBody assembles the request body. It centralises the quirks:
// system + project context merge into one system_instruction, attachments
// become inline_data parts inside the user turn, tools are omitted (not
// emptied) when there are none, and responseMimeType=application/json is
// only set when CapJSON is requested AND no tools are attached (Gemini
// rejects that combination).
func buildGeminiBody(req Request) (*geminiRequest, error) {
	body := &geminiRequest{}

	// System instruction = system + projectContext, joined with \n\n.
	sysText := req.System
	if req.ProjectContext != "" {
		if sysText != "" {
			sysText += "\n\n" + req.ProjectContext
		} else {
			sysText = req.ProjectContext
		}
	}
	if sysText != "" {
		body.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{Text: sysText}},
		}
	}

	// User turn: attachments first (matches the Anthropic vision shape),
	// then prompt text.
	parts := make([]geminiPart, 0, len(req.Attachments)+1)
	for _, att := range req.Attachments {
		if att.Base64 == "" {
			continue
		}
		mt := att.MediaType
		if mt == "" {
			mt = "image/png"
		}
		parts = append(parts, geminiPart{
			InlineData: &geminiInlineData{MIMEType: mt, Data: att.Base64},
		})
	}
	if req.Prompt != "" || len(parts) == 0 {
		parts = append(parts, geminiPart{Text: req.Prompt})
	}
	body.Contents = []geminiContent{{Role: "user", Parts: parts}}

	// Generation config — only populate fields the caller cared about so we
	// don't accidentally pin defaults we don't own.
	gen := &geminiGenerationConfig{}
	configured := false
	if req.MaxTokens > 0 {
		gen.MaxOutputTokens = req.MaxTokens
		configured = true
	}
	wantJSON := false
	for _, c := range req.Capabilities {
		if c == CapJSON {
			wantJSON = true
			break
		}
	}
	// Gemini rejects responseMimeType=application/json when tools are
	// declared — fall back to free-form text in that case and let the caller
	// validate the JSON from the tool args path.
	if wantJSON && len(req.Tools) == 0 {
		gen.ResponseMIMEType = "application/json"
		configured = true
	}
	if req.EnableThinking {
		budget := req.ThinkingBudget
		if budget == 0 {
			budget = 8000
		}
		gen.ThinkingConfig = &geminiThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  budget,
		}
		configured = true
	}
	if configured {
		body.GenerationConfig = gen
	}

	// Tools passthrough. Omit entirely when empty so Gemini doesn't see an
	// empty function_declarations array (which it rejects).
	if len(req.Tools) > 0 {
		decls := make([]geminiFunctionDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			decls = append(decls, geminiFunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		body.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}

	return body, nil
}

// estimateGeminiCost rough-bills tokens against published Gemini list pricing.
// Real ledger reads from the Google invoice; this is for the UI cost meter
// only. Unknown models fall back to flash rates so we never under-bill.
func estimateGeminiCost(model string, inputTok, outputTok, cacheReadTok int) float64 {
	var inP, outP, cacheReadP float64
	switch {
	case contains(model, "2.5-pro"), contains(model, "gemini-2.5-pro"):
		inP, outP, cacheReadP = 1.25, 5.00, 0.31
	case contains(model, "2.5-flash"), contains(model, "gemini-2.5-flash"):
		inP, outP, cacheReadP = 0.30, 1.20, 0.075
	default:
		inP, outP, cacheReadP = 0.30, 1.20, 0.075
	}
	const m = 1_000_000.0
	return (float64(inputTok)*inP + float64(outputTok)*outP + float64(cacheReadTok)*cacheReadP) / m
}

var _ Provider = (*GeminiProvider)(nil)
