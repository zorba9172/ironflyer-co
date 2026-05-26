package providers

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider is Claude on steroids:
//   - Streaming SSE → Delta channel
//   - Prompt caching for system + project context (ephemeral cache markers)
//     PLUS rolling cache breakpoints on the last few turns of the
//     conversation, which roughly halves cost on multi-turn tool-use loops
//     where the system + recent history is stable across iterations.
//   - Extended thinking for reasoning-heavy tasks (planner/architect/security)
//   - Tool use: when a ToolDispatcher is attached to the request context
//     via WithToolDispatcher, the provider runs the canonical Anthropic
//     Messages API multi-turn loop natively (append assistant message
//     verbatim, send tool_result blocks back, loop). Without a dispatcher
//     the provider falls back to single-turn emission of DeltaToolUse so
//     legacy callers keep working.
//   - Cost-aware tiering: CapCheap → Haiku, CapThinking/CapReasoning → Opus,
//     default → Sonnet. The configured base Model is used when no
//     capability hint applies, so operators retain full override control.
type AnthropicProvider struct {
	client         anthropic.Client
	model          anthropic.Model
	cheapModel     anthropic.Model
	powerModel     anthropic.Model
	thinkingBudget int64
}

type AnthropicOpts struct {
	APIKey         string
	Model          string // e.g. "claude-opus-4-7"
	CheapModel     string // override the "cheap" tier; default claude-haiku-4-5-20251001
	PowerModel     string // override the "power/reasoning" tier; default claude-opus-4-7
	ThinkingBudget int64  // tokens budget for extended thinking; 0 = default
}

// anthropicCapabilityDefaults is the capability → model id table the
// provider consults when the caller doesn't pin an explicit override.
// Operators wire env defaults via AnthropicOpts; this table is the
// fallback when both env AND request capability hints are silent.
//
//	CapQuality / CapThinking / CapReasoning → Opus 4.7 (deepest reasoning)
//	CapCheap / CapFast / CapInline           → Haiku 4.5 (fast + cheap)
//	general / CapCode / CapJSON / CapVision  → Sonnet 4.6 (balanced)
//
// Centralising this table keeps the model defaults in one place — when
// Anthropic ships the next tier we update one map.
var anthropicCapabilityDefaults = map[Capability]string{
	CapQuality:   "claude-opus-4-7",
	CapThinking:  "claude-opus-4-7",
	CapReasoning: "claude-opus-4-7",
	CapCheap:     "claude-haiku-4-5-20251001",
	CapFast:      "claude-haiku-4-5-20251001",
	CapInline:    "claude-haiku-4-5-20251001",
}

func NewAnthropicProvider(opts AnthropicOpts) *AnthropicProvider {
	// anthropic-beta: prompt-caching-2024-07-31 unlocks cache_control
	// breakpoints on system blocks AND assistant/user message turns.
	// Setting it once at client construction means every request — including
	// each iteration of the native multi-turn loop — benefits without
	// per-call boilerplate.
	client := anthropic.NewClient(
		option.WithAPIKey(opts.APIKey),
		option.WithHeader("anthropic-beta", "prompt-caching-2024-07-31"),
	)
	model := anthropic.Model(opts.Model)
	if model == "" {
		// General-purpose default — balanced cost vs quality.
		model = anthropic.Model("claude-sonnet-4-6")
	}
	cheap := anthropic.Model(opts.CheapModel)
	if cheap == "" {
		cheap = anthropic.Model(anthropicCapabilityDefaults[CapCheap])
	}
	power := anthropic.Model(opts.PowerModel)
	if power == "" {
		power = anthropic.Model(anthropicCapabilityDefaults[CapQuality])
	}
	budget := opts.ThinkingBudget
	if budget == 0 {
		budget = 8000
	}
	return &AnthropicProvider{
		client: client, model: model,
		cheapModel: cheap, powerModel: power,
		thinkingBudget: budget,
	}
}

// pickModel applies the cost-aware tier policy to a single request. CapCheap
// wins outright (we'd rather pay Haiku rates for a Critic verdict than rent
// Opus to compose a JSON object). When the caller asked for extended
// thinking or reasoning we promote to Opus — those are the tasks where
// the larger model meaningfully changes output quality. Anything else
// uses the configured base model.
func (a *AnthropicProvider) pickModel(req Request) anthropic.Model {
	// Cheap/fast/inline win outright — these are explicit cost signals;
	// we'd rather pay Haiku rates for a Critic verdict than burn Opus
	// on a Cursor-style ghost-text completion.
	for _, c := range req.Capabilities {
		if c == CapCheap || c == CapFast || c == CapInline {
			return a.cheapModel
		}
	}
	if req.EnableThinking {
		return a.powerModel
	}
	// Quality / thinking / reasoning promote to Opus — these are the
	// tasks where the larger model meaningfully changes output quality.
	for _, c := range req.Capabilities {
		if c == CapQuality || c == CapThinking || c == CapReasoning {
			return a.powerModel
		}
	}
	return a.model
}

func (a *AnthropicProvider) Name() string { return "anthropic" }

func (a *AnthropicProvider) Capabilities() []Capability {
	return []Capability{
		CapReasoning, CapCode, CapJSON, CapVision,
		CapThinking, CapTools, CapCache,
		// CapInline — Haiku is a strong, cheap, low-latency fit for
		// Cursor-style middle-fill-in completions; advertising it here
		// lets the router score Anthropic above text-only fallbacks
		// when the caller asks for inline completions.
		CapInline,
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

	model := a.pickModel(req)
	tools := buildAnthropicTools(req)
	dispatcher := toolDispatcherFromContext(ctx)

	go func() {
		defer close(out)
		out <- Delta{Type: DeltaStart, Provider: a.Name(), Model: string(model)}

		// Running totals across every iteration of the multi-turn loop.
		// We emit a single DeltaDone at the end so BillingGuard charges
		// the full provider spend in one ledger entry; intermediate
		// turns accumulate here without flushing.
		var (
			totalInput, totalOutput          int
			totalCacheRead, totalCacheCreate int
		)

		// Trace of every tool call we dispatched, used in the structured
		// error surfaced when the iteration cap fires.
		var trace []toolUseTrace

		for iter := 0; iter < maxToolIterations; iter++ {
			params := anthropic.MessageNewParams{
				Model:     model,
				MaxTokens: maxTokens,
				System:    systemBlocks,
				Messages:  messages,
			}
			if req.EnableThinking {
				// Claude 4.x (Opus 4.7, Sonnet 4.6, Haiku 4.5) deprecated the
				// `thinking.type=enabled` shape with an explicit budget_tokens.
				// They require `thinking.type=adaptive` and surface budgeting
				// through `output_config.effort`. Older models (Sonnet 3.5,
				// 3.7) still accept the old shape. Detect by the leading
				// "claude-{opus,sonnet,haiku}-4" segment in the model id.
				if isClaude4Family(string(model)) {
					params.Thinking = anthropic.ThinkingConfigParamUnion{
						OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{
							Display: anthropic.ThinkingConfigAdaptiveDisplaySummarized,
						},
					}
				} else {
					budget := a.thinkingBudget
					if req.ThinkingBudget > 0 {
						budget = int64(req.ThinkingBudget)
					}
					params.Thinking = anthropic.ThinkingConfigParamOfEnabled(budget)
				}
			}
			if len(tools) > 0 {
				params.Tools = tools
			}

			turn, err := a.runStreamTurn(ctx, params, out)
			if err != nil {
				out <- Delta{Type: DeltaError, Err: fmt.Errorf("anthropic stream: %w", err)}
				return
			}
			totalInput += turn.inputTokens
			totalOutput += turn.outputTokens
			totalCacheRead += turn.cacheReadTokens
			totalCacheCreate += turn.cacheCreationTokens

			// Multi-turn only activates when (a) the model wants to call
			// a tool AND (b) the caller wired a dispatcher into the
			// context. Otherwise we stop here — same shape as the legacy
			// behavior, with DeltaToolUse already emitted by runStreamTurn.
			if turn.stopReason != anthropic.StopReasonToolUse || len(turn.toolUses) == 0 {
				break
			}
			if dispatcher == nil {
				// Legacy single-turn path: caller will re-prompt with
				// its own synthetic "tool results" message. We've
				// already streamed the tool_use deltas to them.
				break
			}

			// Canonical multi-turn: append the assistant message
			// verbatim (text + tool_use blocks the model emitted) and
			// build the next user message as an array of tool_result
			// blocks — one per tool the assistant requested.
			messages = append(messages, anthropic.NewAssistantMessage(turn.assistantBlocks...))

			toolResults := make([]anthropic.ContentBlockParamUnion, 0, len(turn.toolUses))
			for _, tu := range turn.toolUses {
				args := joinPartialJSON(tu.fragments)
				body, derr := dispatcher(ctx, tu.id, tu.name, args)
				te := toolUseTrace{Round: iter, ID: tu.id, Name: tu.name, Args: args}
				isError := false
				if derr != nil {
					body = derr.Error()
					isError = true
					te.Err = derr.Error()
				}
				body = truncateToolResult(body)
				trace = append(trace, te)
				block := anthropic.NewToolResultBlock(tu.id, body, isError)
				toolResults = append(toolResults, block)
			}
			messages = append(messages, anthropic.NewUserMessage(toolResults...))

			// Apply rolling cache breakpoints to the new tail so the
			// next round amortizes the now-larger conversation prefix.
			messages = applyMessageCacheBreakpoints(messages)

			// Loop-cap guard: if the *next* iter would exceed the cap,
			// emit the structured error rather than silently dropping
			// the partially-finished work.
			if iter+1 >= maxToolIterations {
				out <- Delta{Type: DeltaError, Err: &toolLoopError{
					Iterations: iter + 1,
					Trace:      trace,
				}}
				return
			}
		}

		out <- Delta{
			Type: DeltaDone, Provider: a.Name(), Model: string(model),
			Usage: &Usage{
				InputTokens:         totalInput,
				OutputTokens:        totalOutput,
				CacheReadTokens:     totalCacheRead,
				CacheCreationTokens: totalCacheCreate,
				CostUSD:             estimateCost(string(model), totalInput, totalOutput, totalCacheRead, totalCacheCreate),
			},
		}
	}()

	return out, nil
}

// streamTurn captures the per-iteration accumulators of a single SSE
// stream. The provider's main loop consumes these to build the next
// request and decide whether to loop again.
type streamTurn struct {
	inputTokens, outputTokens            int
	cacheReadTokens, cacheCreationTokens int
	stopReason                           anthropic.StopReason
	toolUses                             []pendingTool
	assistantBlocks                      []anthropic.ContentBlockParamUnion
}

type pendingTool struct {
	id        string
	name      string
	fragments []string
}

// runStreamTurn drives one SSE stream from the Messages API to completion.
// Text and thinking deltas are forwarded to `out` immediately so the UI
// stays responsive; tool_use blocks are accumulated for the caller to
// dispatch (and also emitted as DeltaToolUse for legacy single-turn
// consumers that read the channel directly). The returned streamTurn
// captures everything the multi-turn loop needs to build its next
// request.
func (a *AnthropicProvider) runStreamTurn(ctx context.Context, params anthropic.MessageNewParams, out chan<- Delta) (streamTurn, error) {
	stream := a.client.Messages.NewStreaming(ctx, params)
	var turn streamTurn

	// Per-index accumulators. Anthropic streams each content block under
	// an `index` integer; we use it to thread text fragments and
	// input_json_delta fragments back to the right block.
	type blockAcc struct {
		kind     string // "text" | "tool_use" | "thinking" | other
		text     string
		thinking string
		// tool_use only
		id       string
		name     string
		fragJSON []string
	}
	blocks := map[int64]*blockAcc{}

	for stream.Next() {
		event := stream.Current()
		switch ev := event.AsAny().(type) {

		case anthropic.MessageStartEvent:
			if ev.Message.Usage.InputTokens > 0 {
				turn.inputTokens = int(ev.Message.Usage.InputTokens)
			}
			if ev.Message.Usage.CacheReadInputTokens > 0 {
				turn.cacheReadTokens = int(ev.Message.Usage.CacheReadInputTokens)
			}
			if ev.Message.Usage.CacheCreationInputTokens > 0 {
				turn.cacheCreationTokens = int(ev.Message.Usage.CacheCreationInputTokens)
			}

		case anthropic.ContentBlockStartEvent:
			acc := &blockAcc{kind: ev.ContentBlock.Type}
			if ev.ContentBlock.Type == "tool_use" {
				acc.id = ev.ContentBlock.ID
				acc.name = ev.ContentBlock.Name
			}
			blocks[ev.Index] = acc

		case anthropic.ContentBlockDeltaEvent:
			acc := blocks[ev.Index]
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Delta.Text != "" {
					if acc != nil {
						acc.text += ev.Delta.Text
					}
					out <- Delta{Type: DeltaText, Text: ev.Delta.Text}
				}
			case "thinking_delta":
				if ev.Delta.Thinking != "" {
					if acc != nil {
						acc.thinking += ev.Delta.Thinking
					}
					out <- Delta{Type: DeltaThinking, Text: ev.Delta.Thinking}
				}
			case "input_json_delta":
				if acc != nil && ev.Delta.PartialJSON != "" {
					acc.fragJSON = append(acc.fragJSON, ev.Delta.PartialJSON)
					out <- Delta{Type: DeltaToolUse, ToolUse: &ToolUseDelta{
						ID: acc.id, Name: acc.name,
						Input: map[string]any{"_partial": ev.Delta.PartialJSON},
					}}
				}
			}

		case anthropic.ContentBlockStopEvent:
			acc := blocks[ev.Index]
			if acc == nil {
				continue
			}
			switch acc.kind {
			case "text":
				if acc.text != "" {
					turn.assistantBlocks = append(turn.assistantBlocks, anthropic.NewTextBlock(acc.text))
				}
			case "tool_use":
				args := joinPartialJSON(acc.fragJSON)
				turn.toolUses = append(turn.toolUses, pendingTool{
					id: acc.id, name: acc.name, fragments: acc.fragJSON,
				})
				turn.assistantBlocks = append(turn.assistantBlocks, anthropic.NewToolUseBlock(acc.id, args, acc.name))
			}
			// Thinking blocks are not echoed back on the next turn: the
			// API recomputes them per request and including stale
			// thinking confuses the model.

		case anthropic.MessageDeltaEvent:
			if ev.Usage.OutputTokens > 0 {
				turn.outputTokens = int(ev.Usage.OutputTokens)
			}
			if ev.Delta.StopReason != "" {
				turn.stopReason = ev.Delta.StopReason
			}

		case anthropic.MessageStopEvent:
			// final — no-op
		}
	}
	if err := stream.Err(); err != nil {
		return turn, err
	}
	return turn, nil
}

// buildAnthropicTools converts the provider-neutral ToolSpec slice into
// the SDK's tagged union, skipping the empty case so we don't even send
// a tools field on text-only calls (which would otherwise cost a few
// extra prefill tokens).
//
// The LAST tool gets a cache_control breakpoint: Anthropic treats one
// breakpoint anywhere in the tools array as "cache the whole tools
// block", and tools rarely change between rounds (the agent role's
// tool surface is stable). This is the 4th and final breakpoint slot
// (system + project_context + rolling-message + tools), used to its
// fullest. The first two cache_control stamps live in
// buildSystemBlocks, the third in applyMessageCacheBreakpoints; this
// one closes the loop so every stable input the model sees is in the
// cache after the first call.
func buildAnthropicTools(req Request) []anthropic.ToolUnionParam {
	if len(req.Tools) == 0 {
		return nil
	}
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
	// Stamp cache_control on the FINAL tool. Anthropic accepts the
	// breakpoint on any tool but the canonical pattern is "last one"
	// so cache hits include every prior tool too.
	if last := &tools[len(tools)-1]; last.OfTool != nil {
		last.OfTool.CacheControl = anthropic.CacheControlEphemeralParam{}
	}
	return tools
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
	// Most calls are text-only: keep that path cheap.
	if len(req.Attachments) == 0 {
		return []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
		}
	}
	// Vision call: emit each attachment as an image block in the same user
	// turn as the prompt. Anthropic accepts base64 image source with the
	// IANA media type — we trust the caller to have validated both before
	// the request landed here.
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(req.Attachments)+1)
	for _, att := range req.Attachments {
		if att.Base64 == "" {
			continue
		}
		mt := att.MediaType
		if mt == "" {
			mt = "image/png"
		}
		blocks = append(blocks, anthropic.NewImageBlockBase64(mt, att.Base64))
	}
	blocks = append(blocks, anthropic.NewTextBlock(req.Prompt))
	return []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)}
}

// applyMessageCacheBreakpoints stamps an ephemeral cache_control marker on
// the last text block of the final message. Anthropic allows up to 4
// cache breakpoints total per request: two are spent on the system blocks
// (system instructions + project context), leaving room here to cache the
// growing conversation prefix. We refresh the breakpoint each round so it
// always sits on the most recent turn — that way the next iteration
// reads from cache for everything up to and including the round we just
// finished, paying full price only for the newest user/tool_result.
//
// We only mark text and tool_result blocks (the bulky ones); image blocks
// already cache fine on their own and tool_use blocks are typically tiny.
func applyMessageCacheBreakpoints(messages []anthropic.MessageParam) []anthropic.MessageParam {
	if len(messages) == 0 {
		return messages
	}
	// First clear any cache_control we previously stamped on prior
	// messages — Anthropic caps the request at 4 breakpoints total, so
	// rolling the marker forward each round (instead of accumulating
	// stale ones) keeps us under the limit no matter how long the
	// conversation grows.
	var zero anthropic.CacheControlEphemeralParam
	for i := range messages {
		for j := range messages[i].Content {
			if t := messages[i].Content[j].OfText; t != nil {
				t.CacheControl = zero
			}
			if r := messages[i].Content[j].OfToolResult; r != nil {
				r.CacheControl = zero
			}
		}
	}
	// Stamp the last cacheable block of the last message.
	last := &messages[len(messages)-1]
	for j := len(last.Content) - 1; j >= 0; j-- {
		if t := last.Content[j].OfText; t != nil {
			t.CacheControl = anthropic.CacheControlEphemeralParam{}
			return messages
		}
		if r := last.Content[j].OfToolResult; r != nil {
			r.CacheControl = anthropic.CacheControlEphemeralParam{}
			return messages
		}
	}
	return messages
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

// isClaude4Family reports whether the given Anthropic model id belongs to the
// 4.x release line. Used by the streaming dispatcher to choose between the
// legacy `thinking.type=enabled` shape (Sonnet 3.5/3.7) and the 4.x-required
// `thinking.type=adaptive` shape. Matches "claude-{opus,sonnet,haiku}-4..."
// prefixes; everything else falls back to legacy.
func isClaude4Family(model string) bool {
	for _, prefix := range []string{"claude-opus-4", "claude-sonnet-4", "claude-haiku-4"} {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

var _ Provider = (*AnthropicProvider)(nil)
