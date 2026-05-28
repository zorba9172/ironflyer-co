import { useDataConfig } from './provider';

// Streams assistant deltas from the orchestrator. Per the core's API contract
// this is REST + Server-Sent Events (POST /executions/{id}/chat/stream), not
// GraphQL. Returns isLive=false when no endpoint is configured.
export function useChatStream() {
  const cfg = useDataConfig();
  const isLive = !!cfg.endpoint;

  async function send(projectId: string, message: string, onDelta: (text: string) => void): Promise<void> {
    if (!cfg.endpoint) throw new Error('offline: no orchestrator endpoint configured');
    const base = cfg.endpoint.replace(/\/graphql\/?$/, '');
    const token = cfg.getToken?.();
    const res = await fetch(`${base}/executions/${encodeURIComponent(projectId)}/chat/stream`, {
      method: 'POST',
      headers: { 'content-type': 'application/json', accept: 'text/event-stream', ...(token ? { authorization: `Bearer ${token}` } : {}) },
      body: JSON.stringify({ message }),
    });
    if (!res.ok || !res.body) throw new Error(`chat stream failed: ${res.status}`);

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const frames = buf.split('\n\n');
      buf = frames.pop() ?? '';
      for (const frame of frames) {
        const line = frame.split('\n').find((l) => l.startsWith('data:'));
        if (!line) continue;
        const payload = line.slice(5).trim();
        if (payload === '[DONE]') return;
        try {
          const json = JSON.parse(payload) as { delta?: string; text?: string };
          const delta = json.delta ?? json.text;
          if (delta) onDelta(delta);
        } catch {
          onDelta(payload);
        }
      }
    }
  }

  return { isLive, send };
}
