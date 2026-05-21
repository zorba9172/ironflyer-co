package providers

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider is Claude on steroids:
//  - Streaming SSE → Delta channel
//  - Prompt caching for system + project context (ephemeral cache markers)
//  - Extended thinking for reasoning-heavy tasks (planner/architect/security)
//  - Tool use passthrough
type AnthropicProvider struct {
	client        anthropic.Client
	model         anthropic.Model
	thinkingBudget int64
}

type AnthropicOpts struct {
	APIKey         string
	Model          string // e.g. "claude-opus-4-7"
	ThinkingBudget int64  // tokens budget for extended thinking; 0 = default
}

func NewAnthropicProvider(opts AnthropicOpts) *AnthropicProvider {
	client := anthropic.NewClient(option.WithAPIKey(opts.APIKey))
	model := anthropic.Model(opts.Model)
	if model == "" {
		model = anthropic.Model("claude-opus-4-7")
	}
	budget := opts.ThinkingBudget
	if budget == 0 {
		budget = 8000
	}
	return &AnthropicProvider{client: client, model: model, thinkingBudget: budget}
}

func (a *AnthropicProvider) Name() string { return "anthropic" }

func (a *AnthropicProvider) Capabilities() []Capability {
	return []Capability{
		CapReasoning, CapCode, CapJSON, CapVision,
		CapThinking, CapTools, CapCache,
	}
}

func (a *AnthropicProvider) CompleteStream(ctx context.Context, req Request) (<-chan Delta, error) {
	out := make(chan Delta, 32)

	systemBlocks := buildSystemBlocks(req)
	messages := buildMessages(req)

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 8192
	}

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: maxTokens,
		System:    systemBlocks,
		Messages:  messages,
	}
	if req.EnableThinking {
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(a.thinkingBudget)
	}
	// Tools passthrough (lightweight subset — custom tools only).
	if len(req.Tools) > 0 {
		tools := make([]anthropic.ToolUnionParam, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					Name:        t.Name,
					Description: anthropic.String(t.Description),
					InputSchema: anthropic.ToolInputSchemaParam{
						Properties: t.InputSchema,
					},
				},
			})
		}
		params.Tools = tools
	}

	go func() {
		defer close(out)
		out <- Delta{Type: DeltaStart, Provider: a.Name(), Model: string(a.model)}

		stream := a.client.Messages.NewStreaming(ctx, params)
		var (
			inputTokens, outputTokens             int
			cacheReadTokens, cacheCreationTokens  int
			pendingToolUse                        *ToolUseDelta
		)

		for stream.Next() {
			event := stream.Current()
			switch ev := event.AsAny().(type) {

			case anthropic.MessageStartEvent:
				if ev.Message.Usage.InputTokens > 0 {
					inputTokens = int(ev.Message.Usage.InputTokens)
				}
				if ev.Message.Usage.CacheReadInputTokens > 0 {
					cacheReadTokens = int(ev.Message.Usage.CacheReadInputTokens)
				}
				if ev.Message.Usage.CacheCreationInputTokens > 0 {
					cacheCreationTokens = int(ev.Message.Usage.CacheCreationInputTokens)
				}

			case anthropic.ContentBlockStartEvent:
				if ev.ContentBlock.Type == "tool_use" {
					pendingToolUse = &ToolUseDelta{
						ID:    ev.ContentBlock.ID,
						Name:  ev.ContentBlock.Name,
						Input: map[string]any{},
					}
				}

			case anthropic.ContentBlockDeltaEvent:
				switch ev.Delta.Type {
				case "text_delta":
					if ev.Delta.Text != "" {
						out <- Delta{Type: DeltaText, Text: ev.Delta.Text}
					}
				case "thinking_delta":
					if ev.Delta.Thinking != "" {
						out <- Delta{Type: DeltaThinking, Text: ev.Delta.Thinking}
					}
				case "input_json_delta":
					// Tool input arrives as JSON deltas; we forward the
					// raw partial JSON so the UI can render progress.
					if pendingToolUse != nil && ev.Delta.PartialJSON != "" {
						out <- Delta{Type: DeltaToolUse, ToolUse: &ToolUseDelta{
							ID: pendingToolUse.ID, Name: pendingToolUse.Name,
							Input: map[string]any{"_partial": ev.Delta.PartialJSON},
						}}
					}
				}

			case anthropic.ContentBlockStopEvent:
				pendingToolUse = nil

			case anthropic.MessageDeltaEvent:
				if ev.Usage.OutputTokens > 0 {
					outputTokens = int(ev.Usage.OutputTokens)
				}

			case anthropic.MessageStopEvent:
				// final
			}
		}
		if err := stream.Err(); err != nil {
			out <- Delta{Type: DeltaError, Err: fmt.Errorf("anthropic stream: %w", err)}
			return
		}

		out <- Delta{
			Type: DeltaDone, Provider: a.Name(), Model: string(a.model),
			Usage: &Usage{
				InputTokens:         inputTokens,
				OutputTokens:        outputTokens,
				CacheReadTokens:     cacheReadTokens,
				CacheCreationTokens: cacheCreationTokens,
				CostUSD:             estimateCost(string(a.model), inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens),
			},
		}
	}()

	return out, nil
}

// buildSystemBlocks creates the system prompt as cache-marked text blocks.
// The system instructions and the (typically large) project context are
// flagged as ephemeral cache breakpoints so subsequent calls amortize cost.
func buildSystemBlocks(req Request) []anthropic.TextBlockParam {
	var blocks []anthropic.TextBlockParam
	if req.System != "" {
		blocks = append(blocks, anthropic.TextBlockParam{
			Text:         req.System,
			CacheControl: anthropic.CacheControlEphemeralParam{},
		})
	}
	if req.ProjectContext != "" {
		blocks = append(blocks, anthropic.TextBlockParam{
			Text:         req.ProjectContext,
			CacheControl: anthropic.CacheControlEphemeralParam{},
		})
	}
	return blocks
}

func buildMessages(req Request) []anthropic.MessageParam {
	user := anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt))
	return []anthropic.MessageParam{user}
}

// estimateCost rough-bills tokens based on published Anthropic pricing tiers.
// Real ledger reads from Anthropic's invoice; this is for UI cost meter only.
func estimateCost(model string, in, out, cacheRead, cacheCreate int) float64 {
	// per 1M tokens (USD), rough order-of-magnitude
	var inP, outP, cacheReadP, cacheCreateP float64
	switch {
	case contains(model, "opus"):
		inP, outP, cacheReadP, cacheCreateP = 15, 75, 1.5, 18.75
	case contains(model, "sonnet"):
		inP, outP, cacheReadP, cacheCreateP = 3, 15, 0.3, 3.75
	case contains(model, "haiku"):
		inP, outP, cacheReadP, cacheCreateP = 1, 5, 0.1, 1.25
	default:
		inP, outP, cacheReadP, cacheCreateP = 3, 15, 0.3, 3.75
	}
	const m = 1_000_000.0
	return (float64(in)*inP + float64(out)*outP + float64(cacheRead)*cacheReadP + float64(cacheCreate)*cacheCreateP) / m
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

var _ Provider = (*AnthropicProvider)(nil)
