// SSE wire-format helpers, factored out of the API client so they can be
// unit-tested in isolation. Speaks the framing used by the orchestrator:
//
//   event: <name>\n
//   data: <json or text>\n
//   data: <continuation>\n
//   \n
//
// Lines starting with ":" are comments (heartbeats) and are dropped.

export interface SSEEvent {
  event: string;
  data: unknown;
}

export function parseFrame(frame: string): SSEEvent | undefined {
  let eventName = 'message';
  const dataLines: string[] = [];
  for (const line of frame.split('\n')) {
    if (!line || line.startsWith(':')) continue;
    if (line.startsWith('event:')) {
      eventName = line.slice(6).trim();
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trim());
    }
  }
  if (dataLines.length === 0) return undefined;
  const raw = dataLines.join('\n');
  try {
    return { event: eventName, data: JSON.parse(raw) };
  } catch {
    return { event: eventName, data: raw };
  }
}

/**
 * Splits a buffer into complete frames separated by blank lines. Returns
 * the parsed frames and the remainder that should stay in the read buffer
 * for the next chunk.
 */
export function drainFrames(buf: string): { events: SSEEvent[]; rest: string } {
  const events: SSEEvent[] = [];
  let rest = buf;
  let idx;
  while ((idx = rest.indexOf('\n\n')) >= 0) {
    const frame = rest.slice(0, idx);
    rest = rest.slice(idx + 2);
    const evt = parseFrame(frame);
    if (evt) events.push(evt);
  }
  return { events, rest };
}
