// streamChat — fetch-based client for the orchestrator's dedicated
// SSE chat endpoint (POST /executions/{id}/chat/stream).
//
// Why fetch + ReadableStream instead of EventSource:
//   - EventSource only supports GET, but the orchestrator wants a
//     POST body ({ message, model? }).
//   - EventSource cannot set Authorization headers; the workaround
//     (?token=) leaks the JWT through proxy access logs. fetch lets
//     us send `Authorization: Bearer ...` natively.
//
// The endpoint emits the SSE frames documented in
// core/orchestrator/internal/httpapi/chat_stream.go:
//   event: delta        -> { text: string }
//   event: thinking     -> { text: string }
//   event: tool_call    -> { id, name, args }
//   event: tool_result  -> { id, ok, summary? }
//   event: finish       -> { reason, tokenIn, tokenOut, costUSD, provider?, model? }
//   event: error        -> { code, message }
//
// onEvent receives every frame; the caller decides how to render.
// The returned controller exposes abort() so the caller can cancel
// mid-stream (e.g. user clicked Stop) — aborting closes the fetch,
// which in turn cancels the upstream provider call on the server.

const DEFAULT_API_BASE = "http://localhost:8080";

export interface StreamChatInput {
  executionID: string;
  message: string;
  model?: string;
  token: string;
  // Optional AbortSignal for external cancellation (e.g. tied to an
  // unmount effect). When supplied, the helper still creates its own
  // internal AbortController so the returned cancel() works.
  signal?: AbortSignal;
}

export interface StreamChatEvent {
  type:
    | "delta"
    | "thinking"
    | "tool_call"
    | "tool_result"
    | "finish"
    | "error";
  data: Record<string, unknown>;
}

export interface StreamChatHandle {
  // Promise that resolves once the stream has been fully drained (or
  // rejected with the underlying network/abort error).
  done: Promise<void>;
  // Cancel mid-stream. Triggers an HTTP abort which cascades into the
  // server-side provider stream cancellation.
  abort(): void;
}

function apiBase(): string {
  const base = process.env.NEXT_PUBLIC_IRONFLYER_API_URL;
  if (base && base.length > 0) return base.replace(/\/+$/, "");
  return DEFAULT_API_BASE;
}

// parseFrames splits a buffer into complete SSE frames (terminated
// by a blank line). Returns the parsed events plus the unconsumed
// tail to feed into the next chunk.
function parseFrames(buffer: string): {
  events: StreamChatEvent[];
  rest: string;
} {
  const events: StreamChatEvent[] = [];
  let rest = buffer;
  // SSE frames are separated by "\n\n" (or "\r\n\r\n").
  // Normalise CRLF first so a single split rule covers both.
  rest = rest.replace(/\r\n/g, "\n");
  let idx = rest.indexOf("\n\n");
  while (idx !== -1) {
    const raw = rest.slice(0, idx);
    rest = rest.slice(idx + 2);
    const parsed = parseSingleFrame(raw);
    if (parsed) events.push(parsed);
    idx = rest.indexOf("\n\n");
  }
  return { events, rest };
}

function parseSingleFrame(raw: string): StreamChatEvent | null {
  let event = "delta";
  const dataLines: string[] = [];
  for (const line of raw.split("\n")) {
    if (!line || line.startsWith(":")) continue;
    if (line.startsWith("event:")) {
      event = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      dataLines.push(line.slice(5).trim());
    }
  }
  if (dataLines.length === 0) return null;
  const dataStr = dataLines.join("\n");
  let parsed: Record<string, unknown> = {};
  try {
    parsed = JSON.parse(dataStr) as Record<string, unknown>;
  } catch {
    parsed = { raw: dataStr };
  }
  return {
    type: event as StreamChatEvent["type"],
    data: parsed,
  };
}

export function streamChat(
  input: StreamChatInput,
  onEvent: (ev: StreamChatEvent) => void,
): StreamChatHandle {
  const controller = new AbortController();
  if (input.signal) {
    if (input.signal.aborted) controller.abort();
    else
      input.signal.addEventListener("abort", () => controller.abort(), {
        once: true,
      });
  }

  const url = `${apiBase()}/executions/${encodeURIComponent(input.executionID)}/chat/stream`;
  const body: Record<string, unknown> = { message: input.message };
  if (input.model) body.model = input.model;

  const done = (async () => {
    let res: Response;
    try {
      res = await fetch(url, {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "text/event-stream",
          authorization: `Bearer ${input.token}`,
        },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
    } catch (err) {
      if (controller.signal.aborted) return;
      onEvent({
        type: "error",
        data: {
          code: "NETWORK",
          message: err instanceof Error ? err.message : String(err),
        },
      });
      return;
    }

    if (!res.ok || !res.body) {
      let detail = `HTTP ${res.status}`;
      try {
        const text = await res.text();
        if (text) detail = text;
      } catch {
        // Ignore — fall back to status code.
      }
      onEvent({
        type: "error",
        data: { code: `HTTP_${res.status}`, message: detail },
      });
      return;
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    try {
      // eslint-disable-next-line no-constant-condition
      while (true) {
        const { value, done: streamDone } = await reader.read();
        if (streamDone) break;
        buffer += decoder.decode(value, { stream: true });
        const { events, rest } = parseFrames(buffer);
        buffer = rest;
        for (const ev of events) onEvent(ev);
      }
      // Flush any trailing complete frame in the buffer.
      buffer += decoder.decode();
      if (buffer.trim().length > 0) {
        const { events } = parseFrames(buffer + "\n\n");
        for (const ev of events) onEvent(ev);
      }
    } catch (err) {
      if (controller.signal.aborted) return;
      onEvent({
        type: "error",
        data: {
          code: "STREAM",
          message: err instanceof Error ? err.message : String(err),
        },
      });
    }
  })();

  return {
    done,
    abort: () => controller.abort(),
  };
}
