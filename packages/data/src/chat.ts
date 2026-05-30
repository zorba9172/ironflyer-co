import { useRef } from 'react';
import { useDataConfig, useRequest } from './provider';
import { CREATE_PAID_EXECUTION } from './operations';

// Discriminated stream events the orchestrator emits over SSE.
export type ChatStreamEvent =
  | { type: 'text'; text: string }
  | { type: 'thinking'; text: string }
  | { type: 'tool'; id: string; name: string; args: unknown }
  | { type: 'finish'; costUSD?: number }
  | { type: 'error'; code?: string; message: string };

// Real chat against the orchestrator:
//   1. createPaidExecution(projectID, budgetUSD) → execution id (reused per project)
//   2. POST /executions/{execId}/chat/stream → SSE `event: delta data: {"text":...}`
// Returns isLive=false when no endpoint is configured.
export function useChatStream() {
  const cfg = useDataConfig();
  const request = useRequest();
  const isLive = !!cfg.endpoint;
  const execByProject = useRef<Record<string, string>>({});

  const EXEC_BUDGET_USD = 2;

  async function createExecution(projectId: string): Promise<string> {
    const d = await request!<{ createPaidExecution: { id: string } }>('CreatePaidExecution', CREATE_PAID_EXECUTION, {
      input: { projectID: projectId, budgetUSD: EXEC_BUDGET_USD, promptSummary: 'studio chat' },
    });
    const id = d.createPaidExecution.id;
    execByProject.current[projectId] = id;
    return id;
  }

  function ensureExecution(projectId: string): Promise<string> {
    const cached = execByProject.current[projectId];
    return cached ? Promise.resolve(cached) : createExecution(projectId);
  }

  // One streaming pass. Returns whether a budget pause hit and whether any
  // assistant text was emitted (so the caller can decide to retry cleanly).
  async function runStream(execId: string, message: string, onEvent: (ev: ChatStreamEvent) => void, signal?: AbortSignal): Promise<{ budgetHit: boolean; textEmitted: boolean }> {
    const base = cfg.endpoint!.replace(/\/graphql\/?$/, '');
    const token = cfg.getToken?.();
    const res = await fetch(`${base}/executions/${encodeURIComponent(execId)}/chat/stream`, {
      method: 'POST',
      headers: { 'content-type': 'application/json', accept: 'text/event-stream', ...(token ? { authorization: `Bearer ${token}` } : {}) },
      // No model/provider hint — the orchestrator owns routing and speaks for
      // every upstream vendor. The client never names or selects a provider.
      body: JSON.stringify({ message }),
      signal,
    });
    if (!res.ok || !res.body) throw new Error(`chat stream failed: ${res.status}`);

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    let textEmitted = false;
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const frames = buf.split('\n\n');
      buf = frames.pop() ?? '';
      for (const frame of frames) {
        const lines = frame.split('\n');
        const event = lines.find((l) => l.startsWith('event:'))?.slice(6).trim();
        const dataLine = lines.find((l) => l.startsWith('data:'))?.slice(5).trim();
        if (!dataLine) continue;
        let json: Record<string, unknown>;
        try {
          json = JSON.parse(dataLine);
        } catch {
          continue;
        }
        switch (event) {
          case 'delta':
            if (typeof json.text === 'string') { textEmitted = true; onEvent({ type: 'text', text: json.text }); }
            break;
          case 'thinking':
            if (typeof json.text === 'string') onEvent({ type: 'thinking', text: json.text });
            break;
          case 'tool_call':
            onEvent({ type: 'tool', id: String(json.id ?? ''), name: String(json.name ?? 'tool'), args: json.args });
            break;
          case 'finish':
            onEvent({ type: 'finish', costUSD: json.costUSD as number | undefined });
            return { budgetHit: false, textEmitted };
          case 'error': {
            // Branch on the orchestrator's safe code, not message text.
            const code = String(json.code ?? '');
            const msg = String(json.message ?? '');
            if (code === 'BUDGET' || code === 'PROFITGUARD') return { budgetHit: true, textEmitted };
            onEvent({ type: 'error', code, message: msg });
            return { budgetHit: false, textEmitted };
          }
          default:
            if (typeof json.text === 'string') { textEmitted = true; onEvent({ type: 'text', text: json.text }); }
        }
      }
    }
    return { budgetHit: false, textEmitted };
  }

  // Streams a reply; if the execution's budget is exhausted before any text,
  // transparently starts a fresh execution and retries once.
  async function send(projectId: string, message: string, onEvent: (ev: ChatStreamEvent) => void, signal?: AbortSignal): Promise<void> {
    if (!cfg.endpoint || !request) throw new Error('offline: no orchestrator endpoint configured');
    const first = await runStream(await ensureExecution(projectId), message, onEvent, signal);
    if (first.budgetHit && !first.textEmitted) {
      delete execByProject.current[projectId];
      const second = await runStream(await createExecution(projectId), message, onEvent, signal);
      if (second.budgetHit) onEvent({ type: 'error', code: 'BUDGET', message: 'Budget reached — top up your wallet to continue.' });
    } else if (first.budgetHit) {
      onEvent({ type: 'error', code: 'BUDGET', message: 'Budget reached mid-reply — top up to continue.' });
      delete execByProject.current[projectId];
    }
  }

  return { isLive, send };
}
