package resolver

// Wired by Closure Agent P. Cursor-style inline completions backed by
// the BillingGuard so every token still passes budget admission /
// telemetry / ledger attribution. The mutation just records the
// acceptance via the existing metric counter (the SSE handler is gone;
// this is the GraphQL replacement).

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"ironflyer/apps/orchestrator/internal/graph/model"
	"ironflyer/apps/orchestrator/internal/metrics"
	"ironflyer/apps/orchestrator/internal/providers"
)

// AcceptInlineCompletion bumps the accept counter so the provider
// telemetry can compute acceptance rate. The schema returns a
// generic OperationResult — `ok=true` means the count landed; we
// never error because losing the metric is a soft failure.
func (r *mutationResolver) AcceptInlineCompletion(ctx context.Context, requestID string) (*model.OperationResult, error) {
	metrics.ObserveInlineCompletionAccept()
	msg := "accepted " + requestID
	return &model.OperationResult{Ok: true, Message: &msg}, nil
}

// InlineCompletion streams a middle-fill-in completion. Drives the
// BillingGuard with CapInline + CapFast so the bandit picks the
// cheapest fast provider (Haiku / 4o-mini / Gemini Flash). Emits the
// schema's InlineDelta union: Start → Text… → Done, or Error.
func (r *subscriptionResolver) InlineCompletion(ctx context.Context, input model.InlineInput) (<-chan model.InlineDelta, error) {
	if r.Guard == nil {
		return nil, gqlNotConfigured("billing-guard")
	}
	reqID := ""
	if input.RequestID != nil {
		reqID = strings.TrimSpace(*input.RequestID)
	}
	if reqID == "" {
		reqID = uuid.NewString()
	}
	tenantID := ""
	if u, err := currentUser(ctx); err == nil {
		tenantID = tenantFor(u)
	}

	prompt := buildInlinePrompt(input)
	req := providers.Request{
		System:       inlineSystemPrompt,
		Prompt:       prompt,
		Capabilities: []providers.Capability{providers.CapInline, providers.CapFast, providers.CapCheap},
		MaxTokens:    inlineMaxTokens(input.Effort),
		Temperature:  0.2,
		TenantID:     tenantID,
	}

	out := make(chan model.InlineDelta, 32)
	started := time.Now()
	metrics.ObserveInlineCompletionRequest("started")

	in, err := r.Guard.CompleteStream(ctx, req)
	if err != nil {
		go func() {
			defer close(out)
			out <- model.InlineErrorDelta{
				RequestID: stringPtr(reqID),
				Code:      "STREAM_FAILED",
				Message:   err.Error(),
			}
		}()
		metrics.ObserveInlineCompletionRequest("error")
		return out, nil
	}

	go func() {
		defer close(out)
		var (
			startedSent bool
			provider    string
			modelName   string
			doneSent    bool
			firstToken  bool
		)
		for {
			select {
			case <-ctx.Done():
				out <- model.InlineCancelledDelta{
					RequestID: reqID,
					Reason:    stringPtr("context cancelled"),
				}
				metrics.ObserveInlineCompletionRequest("cancelled")
				return
			case d, ok := <-in:
				if !ok {
					if !doneSent {
						out <- model.InlineDoneDelta{
							RequestID: reqID,
							Provider:  stringPtr(provider),
							Model:     stringPtr(modelName),
						}
					}
					if firstToken {
						metrics.ObserveInlineCompletionLatency(time.Since(started))
					}
					return
				}
				switch d.Type {
				case providers.DeltaStart:
					if !startedSent {
						provider = d.Provider
						modelName = d.Model
						out <- model.InlineStartDelta{
							RequestID: reqID,
							Provider:  d.Provider,
							Model:     d.Model,
						}
						startedSent = true
					}
				case providers.DeltaText:
					if !startedSent {
						out <- model.InlineStartDelta{
							RequestID: reqID,
							Provider:  d.Provider,
							Model:     d.Model,
						}
						startedSent = true
						provider = d.Provider
						modelName = d.Model
					}
					if !firstToken {
						metrics.ObserveInlineCompletionLatency(time.Since(started))
						firstToken = true
					}
					out <- model.InlineTextDelta{RequestID: reqID, Text: d.Text}
				case providers.DeltaDone:
					done := model.InlineDoneDelta{
						RequestID: reqID,
						Provider:  stringPtr(d.Provider),
						Model:     stringPtr(d.Model),
					}
					if d.Usage != nil {
						done.Usage = model.JSON{
							"inputTokens":  d.Usage.InputTokens,
							"outputTokens": d.Usage.OutputTokens,
							"costUsd":      d.Usage.CostUSD,
						}
					}
					out <- done
					doneSent = true
					metrics.ObserveInlineCompletionRequest("done")
				case providers.DeltaError:
					msg := "unknown error"
					if d.Err != nil {
						msg = d.Err.Error()
					}
					out <- model.InlineErrorDelta{
						RequestID: stringPtr(reqID),
						Code:      "PROVIDER_ERROR",
						Message:   msg,
					}
					metrics.ObserveInlineCompletionRequest("error")
					doneSent = true
				}
			}
		}
	}()
	return out, nil
}

// inlineSystemPrompt is the system prompt the model sees for every
// inline call. Tight + senior so the model doesn't go off-script
// and offer paragraphs of explanation in the middle of a function.
const inlineSystemPrompt = `You are an in-editor code completion engine.
Continue the user's code at the cursor. Output ONLY the inserted
code, no explanation, no markdown fences, no commentary. Match the
surrounding language, indentation, and style. Keep the suggestion
short — at most ~6 lines unless the prefix clearly requires more.`

// buildInlinePrompt assembles the prefix/suffix block the provider
// sees. We use markers so the model knows exactly where to fill in
// without having to guess from natural language.
func buildInlinePrompt(in model.InlineInput) string {
	var b strings.Builder
	if in.Language != nil && *in.Language != "" {
		fmt.Fprintf(&b, "Language: %s\n", *in.Language)
	}
	if in.Path != nil && *in.Path != "" {
		fmt.Fprintf(&b, "Path: %s\n", *in.Path)
	}
	b.WriteString("\n<<<PREFIX>>>\n")
	b.WriteString(in.Prefix)
	b.WriteString("\n<<<CURSOR>>>\n")
	if in.Suffix != nil {
		b.WriteString(*in.Suffix)
	}
	b.WriteString("\n<<<SUFFIX_END>>>\n")
	return b.String()
}

// inlineMaxTokens caps the completion length per `effort` hint so a
// noisy provider can't run away with the user's wallet on every
// keystroke.
func inlineMaxTokens(effort *string) int {
	if effort == nil {
		return 96
	}
	switch strings.ToLower(*effort) {
	case "low", "fast":
		return 48
	case "high", "extended":
		return 256
	}
	return 96
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}
