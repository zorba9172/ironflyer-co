// GitHub-import client. Wraps the orchestrator's POST /imports endpoint as a
// streaming subscription so the page can render a live log panel, plus a
// polling helper for clients that prefer a single round-trip.
//
// The endpoint understands `Accept: text/event-stream` and falls back to a
// JSON response otherwise. Because EventSource cannot POST, we use
// fetch+ReadableStream and parse SSE frames manually (same pattern as
// `streamChat` in lib/api.ts).

import { auth } from '../auth';

const base = '/api/orchestrator';

export interface ImportRequestBody {
  repoUrl: string;
  branch?: string;
  subdir?: string;
  makePublic?: boolean;
}

export interface StackDecision {
  frontend: string;
  backend: string;
  storage: string;
  auth: string;
}

export interface ImportResult {
  projectId: string;
  workspaceId: string;
  stack: StackDecision;
  files?: { path: string; type: string; size?: number }[];
  warnings?: string[];
}

export interface ImportEvent {
  type:
    | 'import_started'
    | 'project_created'
    | 'cloning'
    | 'cloned'
    | 'detecting_stack'
    | 'stack_detected'
    | 'warning'
    | 'ready'
    | 'failed'
    | 'result'
    | string;
  message?: string;
  projectId?: string;
  workspaceId?: string;
  stack?: StackDecision;
  warning?: string;
  error?: string;
}

export interface ImportStreamHandlers {
  onEvent?: (evt: ImportEvent) => void;
  onResult?: (res: ImportResult) => void;
  onError?: (msg: string) => void;
  onClose?: () => void;
}

/** startImport opens the SSE pipeline against /imports and dispatches each
 *  parsed event to the provided handlers. Returns an AbortController so the
 *  caller can cancel the upload mid-clone. */
export function startImport(
  body: ImportRequestBody,
  handlers: ImportStreamHandlers,
): AbortController {
  const ctrl = new AbortController();

  void (async () => {
    let res: Response;
    try {
      res = await fetch(`${base}/imports`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'text/event-stream',
          ...auth.authHeader(),
        },
        body: JSON.stringify(body),
        signal: ctrl.signal,
      });
    } catch (e) {
      handlers.onError?.(e instanceof Error ? e.message : String(e));
      handlers.onClose?.();
      return;
    }
    if (!res.ok || !res.body) {
      const txt = await res.text().catch(() => '');
      handlers.onError?.(`${res.status}: ${txt || res.statusText}`);
      handlers.onClose?.();
      return;
    }
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = '';
    try {
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        // SSE frames are separated by blank lines.
        let idx = buf.indexOf('\n\n');
        while (idx !== -1) {
          const frame = buf.slice(0, idx);
          buf = buf.slice(idx + 2);
          dispatch(frame, handlers);
          idx = buf.indexOf('\n\n');
        }
      }
    } catch (e) {
      if ((e as DOMException)?.name !== 'AbortError') {
        handlers.onError?.(e instanceof Error ? e.message : String(e));
      }
    } finally {
      handlers.onClose?.();
    }
  })();

  return ctrl;
}

function dispatch(frame: string, handlers: ImportStreamHandlers) {
  let eventName = 'message';
  const dataLines: string[] = [];
  for (const line of frame.split('\n')) {
    if (line.startsWith(':')) continue; // heartbeat
    if (line.startsWith('event:')) eventName = line.slice(6).trim();
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim());
  }
  if (dataLines.length === 0) return;
  const raw = dataLines.join('\n');
  let payload: any;
  try {
    payload = JSON.parse(raw);
  } catch {
    payload = { type: eventName, message: raw };
  }
  if (eventName === 'result' && payload && typeof payload === 'object' && 'projectId' in payload) {
    handlers.onResult?.(payload as ImportResult);
    return;
  }
  const evt: ImportEvent = { type: payload?.type ?? eventName, ...payload };
  handlers.onEvent?.(evt);
  if (evt.type === 'failed' && evt.error) handlers.onError?.(evt.error);
}

/** getImportStatus is the polling fallback. It returns whatever the
 *  orchestrator has on file for the project right now. */
export async function getImportStatus(projectId: string): Promise<{
  projectId: string;
  status: string;
  stack: StackDecision;
  files: { path: string; type: string; size?: number }[];
  github?: { fullName: string; defaultBranch: string; htmlUrl: string };
  updatedAt: string;
}> {
  const res = await fetch(`${base}/imports/${encodeURIComponent(projectId)}/status`, {
    headers: { ...auth.authHeader() },
    cache: 'no-store',
  });
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  return res.json();
}
