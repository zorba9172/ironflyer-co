// chat_stream.go — POST /executions/{id}/chat/stream.
//
// Dedicated Server-Sent-Events endpoint for raw LLM chat token
// streams. Lives outside GraphQL so that:
//
//  1. gqlgen middleware (CSRF, persisted-queries, complexity, rate
//     limit) does not run on every assistant token.
//  2. The transport stays free of the graphql-transport-ws envelope
//     so high-frequency deltas can flush straight to the wire.
//  3. The schema does not pin assistant payload shape — providers
//     emit free-form tool_call / tool_result / delta frames.
//
// GraphQL is still the API of record for structured orchestration
// events (executionFeed) and for chat mutations (describeIdea,
// refineIdea). Only the assistant's streaming response moves off
// GraphQL.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
	"ironflyer/core/orchestrator/internal/ai/providers"
	"ironflyer/core/orchestrator/internal/operations/sentryext"
)

// chatStreamRequest is the POST body for /executions/{id}/chat/stream.
// Both fields are optional — `message` defaults to the execution's
// prompt summary, `model` is a soft preference passed through to the
// provider router.
type chatStreamRequest struct {
	Message string `json:"message"`
	Model   string `json:"model"`
}

// chatStream serves the SSE endpoint. The route is mounted under the
// strict auth.Middleware so anonymous requests get 401 before this
// handler runs.
func (a *API) chatStream(w http.ResponseWriter, r *http.Request) {
	executionID := strings.TrimSpace(chi.URLParam(r, "id"))
	if executionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing execution id"})
		return
	}

	u, ok := auth.FromContext(r.Context())
	if !ok || u.ID == "" {
		// auth.Middleware should have already blocked anonymous
		// requests, but be defensive.
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
		return
	}

	if a.d.Guard == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "billing guard not configured"})
		return
	}

	// Tenant for ledger / owner checks — orgID when present, else user.ID.
	tenant := u.OrgID
	if tenant == "" {
		tenant = u.ID
	}

	// Free-chat mode: executionID == "_" means the caller wants a
	// general copilot reply (chat + product-building guidance) before
	// any execution has been admitted. Skip the execution lookup and
	// ProfitGuard entirely — BillingGuard still debits the user's
	// wallet so cost stays attributed.
	freeChat := executionID == "_"

	var execPromptSummary string
	if !freeChat {
		if a.d.Execution == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "execution service not configured"})
			return
		}
		e, err := a.d.Execution.Get(r.Context(), executionID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "execution not found"})
			return
		}
		if e.TenantID != tenant {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		execPromptSummary = e.PromptSummary
	}

	// Decode body (best-effort — empty body is allowed when an
	// execution is attached, since we fall back to its prompt summary).
	var body chatStreamRequest
	if r.ContentLength != 0 {
		limited := io.LimitReader(r.Body, 1<<20) // 1 MiB cap
		if err := json.NewDecoder(limited).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body: " + err.Error()})
			return
		}
	}
	prompt := strings.TrimSpace(body.Message)
	if prompt == "" {
		prompt = execPromptSummary
	}
	if prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message required"})
		return
	}

	// SSE headers + flusher.
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: don't buffer SSE
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Attach execution + tenant to ctx so BillingGuard's per-token
	// cost attribution lands on the right execution / ledger row.
	// In free-chat mode no execution exists yet — ProfitGuard is
	// skipped, BillingGuard still debits the wallet via TenantID below.
	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if !freeChat {
		streamCtx = profitguardctx.WithExecution(streamCtx, executionID, tenant)
	}

	system := "You are Ironflyer, a senior AI builder copilot. " +
		"Hold a natural conversation with the user: answer questions, brainstorm, " +
		"plan product work, and propose concrete next steps. When the user signals " +
		"intent to build, outline what you would ship, in what order, and what gates " +
		"or budget implications matter. Be precise, direct, and useful — no hype."
	if !freeChat {
		system += " An execution is currently running; keep replies grounded in it when relevant."
	}

	req := providers.Request{
		System:       system,
		Prompt:       prompt,
		TenantID:     tenant,
		Capabilities: []providers.Capability{providers.CapCode, providers.CapFast},
		MaxTokens:    2048,
	}
	if m := strings.TrimSpace(body.Model); m != "" {
		req.PreferredProvider = m
	}

	in, err := a.d.Guard.CompleteStream(streamCtx, req)
	if err != nil {
		writeSSE(w, flusher, "error", map[string]any{
			"code":    classifyErrorCode(err),
			"message": err.Error(),
		})
		a.d.Logger.Warn().Err(err).
			Str("execution_id", executionID).
			Str("tenant_id", tenant).
			Msg("chat stream: provider start failed")
		sentryext.CaptureError(streamCtx, err, map[string]string{
			"execution_id": executionID,
			"tenant_id":    tenant,
			"surface":      "chat_stream",
		})
		return
	}

	a.d.Logger.Info().
		Str("execution_id", executionID).
		Str("tenant_id", tenant).
		Int("prompt_chars", len(prompt)).
		Msg("chat stream: opened")

	started := time.Now()
	var inTokens, outTokens int
	var costUSD float64
	clientGone := r.Context().Done()

	for {
		select {
		case <-clientGone:
			// Client hung up — cancel upstream and exit.
			cancel()
			a.d.Logger.Info().
				Str("execution_id", executionID).
				Dur("elapsed", time.Since(started)).
				Msg("chat stream: client disconnect")
			return
		case d, ok := <-in:
			if !ok {
				// Provider channel closed without a DeltaDone /
				// DeltaError frame. Emit a synthetic finish so the
				// client UI flips out of "streaming" state.
				writeSSE(w, flusher, "finish", map[string]any{
					"reason":   "stop",
					"tokenIn":  inTokens,
					"tokenOut": outTokens,
					"costUSD":  costUSD,
				})
				return
			}
			switch d.Type {
			case providers.DeltaText:
				writeSSE(w, flusher, "delta", map[string]any{"text": d.Text})
			case providers.DeltaThinking:
				writeSSE(w, flusher, "thinking", map[string]any{"text": d.Text})
			case providers.DeltaToolUse:
				if d.ToolUse != nil {
					writeSSE(w, flusher, "tool_call", map[string]any{
						"id":   d.ToolUse.ID,
						"name": d.ToolUse.Name,
						"args": d.ToolUse.Input,
					})
				}
			case providers.DeltaDone:
				if d.Usage != nil {
					inTokens = d.Usage.InputTokens
					outTokens = d.Usage.OutputTokens
					costUSD = d.Usage.CostUSD
				}
				writeSSE(w, flusher, "finish", map[string]any{
					"reason":   "stop",
					"tokenIn":  inTokens,
					"tokenOut": outTokens,
					"costUSD":  costUSD,
					"provider": d.Provider,
					"model":    d.Model,
				})
				return
			case providers.DeltaError:
				msg := "stream error"
				if d.Err != nil {
					msg = d.Err.Error()
				}
				writeSSE(w, flusher, "error", map[string]any{
					"code":    classifyErrorCode(d.Err),
					"message": msg,
				})
				if d.Err != nil {
					sentryext.CaptureError(streamCtx, d.Err, map[string]string{
						"execution_id": executionID,
						"tenant_id":    tenant,
						"surface":      "chat_stream",
					})
				}
				return
			}
		}
	}
}

// writeSSE emits one Server-Sent-Events frame and flushes immediately
// so the client sees the chunk on the wire without buffering. Frames
// follow the spec: `event: <type>\ndata: <json>\n\n`.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		// Best-effort — fall back to an error payload that is
		// guaranteed to marshal.
		data = []byte(fmt.Sprintf(`{"code":"INTERNAL","message":%q}`, err.Error()))
		event = "error"
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

// classifyErrorCode picks a short error code for the SSE `error`
// payload so the client can branch on machine-readable codes without
// string-matching the human message.
func classifyErrorCode(err error) string {
	if err == nil {
		return "INTERNAL"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "budget"):
		return "BUDGET"
	case strings.Contains(msg, "profitguard"):
		return "PROFITGUARD"
	case strings.Contains(msg, "no provider"):
		return "NO_PROVIDER"
	case strings.Contains(msg, "context canceled"), strings.Contains(msg, "deadline exceeded"):
		return "CANCELLED"
	default:
		return "INTERNAL"
	}
}
