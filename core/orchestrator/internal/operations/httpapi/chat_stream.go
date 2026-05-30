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

	"github.com/go-chi/chi/v5"

	"ironflyer/core/orchestrator/internal/ai/providers"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/operations/sentryext"
)

// ironflyerChatVision is the constant Ironflyer copilot identity baked into
// every chat stream. It fixes who the assistant is (the Ironflyer finisher,
// not a generic coder), how it works (action-first, gate-aware, cost-aware),
// and what it ships (real, reviewable, production-grade output). Keep it tight
// — it is prepended to every chat request.
const ironflyerChatVision = "You are Ironflyer — a senior AI builder that ships finished, production-grade products end to end, not demos. " +
	"Your edge over generic 'describe-an-idea' tools is production discipline: finisher GATES that block unfinished work, reviewable PATCHES (never silent edits), a prepaid WALLET with ProfitGuard before every expensive call, real Linux WORKSPACES, per-user isolation, and first-class AppSec (secrets, SAST, dependency and SBOM scanning). Carry this vision in how you talk and what you build. " +
	"Default to ACTION, not interrogation. When asked to build, do not stall with rounds of clarifying questions — make reasonable, briefly-stated assumptions and immediately produce the real implementation: a short plan, the file tree, then the full code for every file in fenced code blocks with the file path on the opening fence (e.g. ```tsx src/App.tsx), then the exact run steps. If the user says 'you choose' or 'don't ask', pick sensible defaults (Vite + React + TypeScript) and build. Ask at most ONE question, and only if truly blocked. " +
	"Be efficient and human: lead with the work, keep prose tight, no hype, no filler caveats. Prefer concrete nouns — gate verdict, patch, wallet, ledger entry, deploy artifact, completion score. " +
	"Think visually: when a diagram, mockup, icon, or sample image would help, include it — emit an image with markdown ![alt](url) when you have a real URL, and otherwise describe the visual precisely so it can be rendered. " +
	"You are mid-conversation: the message may include prior turns as context — honor them and continue the thread rather than restarting. " +
	"LANGUAGE: reply in the SAME language the user writes in. Detect the language of the user's latest message and respond entirely in it (Hebrew → Hebrew, Spanish → Spanish, Arabic → Arabic, etc.); if they switch languages, switch with them. Keep code, file paths, identifiers, commands, and technical keywords in their canonical form (normally English) — translate only the prose around them."

// sseFrameBufPool reuses the per-frame scratch buffer used by writeSSE.
// Every SSE frame (delta, thinking, tool_call) goes through this pool so
// the hot path stops allocating a fresh json.Marshal []byte + Sprintf
// string for every assistant token.
var sseFrameBufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

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

	system := ironflyerChatVision
	if !freeChat {
		system += " An execution is currently running; keep replies grounded in it and reference its gates, patches, and ledger entries when relevant."
	}

	req := providers.Request{
		System:       system,
		Prompt:       prompt,
		TenantID:     tenant,
		Capabilities: []providers.Capability{providers.CapCode, providers.CapFast},
		MaxTokens:    2048,
	}
	// The client does NOT choose the provider/model. The orchestrator owns
	// routing by capability so vendor selection never leaks to (or is driven
	// by) the user; body.Model is accepted for wire-compat but ignored.
	_ = body.Model

	in, err := a.d.Guard.CompleteStreamWithFailover(streamCtx, req)
	if err != nil {
		code := classifyErrorCode(err)
		writeSSE(w, flusher, "error", map[string]any{
			"code":    code,
			"message": safeChatErrorMessage(code),
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
				// NOTE: provider + model are deliberately NOT emitted. The
				// orchestrator speaks for every upstream vendor; the client
				// only ever sees tokens + cost, never which provider ran.
				writeSSE(w, flusher, "finish", map[string]any{
					"reason":   "stop",
					"tokenIn":  inTokens,
					"tokenOut": outTokens,
					"costUSD":  costUSD,
				})
				return
			case providers.DeltaError:
				code := classifyErrorCode(d.Err)
				writeSSE(w, flusher, "error", map[string]any{
					"code":    code,
					"message": safeChatErrorMessage(code),
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
//
// The scratch buffer is pooled because chat streams emit one frame per
// assistant token. With Sonnet/Haiku that's 30–80 frames per second per
// active subscriber, so a per-frame []byte allocation showed up in pprof.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload any) {
	bb := sseFrameBufPool.Get().(*bytes.Buffer)
	bb.Reset()

	bb.WriteString("event: ")
	bb.WriteString(event)
	bb.WriteString("\ndata: ")
	if err := json.NewEncoder(bb).Encode(payload); err != nil {
		// Reset and emit a fallback error frame that is guaranteed to
		// marshal — match the legacy behavior.
		bb.Reset()
		fmt.Fprintf(bb, "event: error\ndata: {\"code\":\"INTERNAL\",\"message\":%q}\n\n", err.Error())
	} else {
		// json.Encoder appends a trailing newline that completes the
		// `data:` line; we still need the blank line that terminates an
		// SSE frame.
		bb.WriteByte('\n')
	}
	_, _ = w.Write(bb.Bytes())
	flusher.Flush()

	// Reclaim the buffer, capped to keep one runaway payload from
	// anchoring a huge allocation in the pool forever.
	if bb.Cap() <= 64<<10 {
		sseFrameBufPool.Put(bb)
	}
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
	// Upstream credential / auth / rate problems are mapped to a single
	// provider-blind "temporarily unavailable" code. The raw error (which may
	// name a vendor or model) is logged server-side only — never surfaced.
	case strings.Contains(msg, "api key"), strings.Contains(msg, "api_key"),
		strings.Contains(msg, "unauthorized"), strings.Contains(msg, "unauthenticated"),
		strings.Contains(msg, "401"), strings.Contains(msg, "403"), strings.Contains(msg, "expired"),
		strings.Contains(msg, "permission"), strings.Contains(msg, "quota"),
		strings.Contains(msg, "rate limit"), strings.Contains(msg, "429"),
		strings.Contains(msg, "overloaded"), strings.Contains(msg, "503"):
		return "UNAVAILABLE"
	default:
		return "INTERNAL"
	}
}

// safeChatErrorMessage returns a fixed, provider-blind message for a classified
// error code. The orchestrator speaks for every upstream provider, so the chat
// stream NEVER forwards a raw provider error (which can name a vendor, a model,
// or leak internal detail) to the client — only one of these safe strings.
func safeChatErrorMessage(code string) string {
	switch code {
	case "BUDGET":
		return "This run is out of budget. Top up the wallet to continue."
	case "PROFITGUARD":
		return "ProfitGuard paused this step to protect the run's margin."
	case "NO_PROVIDER":
		return "The assistant is temporarily unavailable. Please try again shortly."
	case "CANCELLED":
		return "The request was cancelled."
	case "UNAVAILABLE":
		return "The assistant is temporarily unavailable. Please try again shortly."
	default:
		return "Something went wrong handling that message. Please try again."
	}
}
