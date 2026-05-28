import { useEffect, useState } from 'react';
import { createClient } from 'graphql-ws';
import { useDataConfig } from './provider';
import { RUN_PROJECT_SUB } from './operations';

export interface RunLogEvent {
  id: string;
  ts: number;
  kind: 'gate' | 'patch' | 'profitguard' | 'deploy' | 'ledger';
  text: string;
}

interface RawRunEvent {
  __typename: string;
  ts?: string;
  gate?: string;
  status?: string;
  message?: string;
  ok?: boolean;
  code?: string;
  payload?: unknown;
}

function mapRunEvent(ev: RawRunEvent): RunLogEvent {
  const ts = ev.ts ? Date.parse(ev.ts) || Date.now() : Date.now();
  const id = `${ev.__typename}-${ts}-${Math.random().toString(36).slice(2, 6)}`;
  switch (ev.__typename) {
    case 'RunGateEvent':
      return { id, ts, kind: 'gate', text: `${ev.gate} → ${ev.status}${ev.message ? `: ${ev.message}` : ''}` };
    case 'RunDoneEvent':
      return { id, ts, kind: 'deploy', text: `Run finished — ok=${ev.ok}` };
    case 'RunErrorEvent':
      return { id, ts, kind: 'profitguard', text: `Error ${ev.code ?? ''}: ${ev.message ?? 'run error'}`.trim() };
    default: {
      const p = typeof ev.payload === 'string' ? ev.payload : JSON.stringify(ev.payload ?? {});
      return { id, ts, kind: 'ledger', text: `execution: ${p.slice(0, 160)}` };
    }
  }
}

// Live project run feed over graphql-ws (the orchestrator streams gate/run
// events on the same /graphql path). Offline → empty + isLive false.
export function useRunProjectFeed(projectId: string | null): { events: RunLogEvent[]; isLive: boolean } {
  const cfg = useDataConfig();
  const [events, setEvents] = useState<RunLogEvent[]>([]);
  const [isLive, setIsLive] = useState(false);

  useEffect(() => {
    if (!cfg.endpoint || !projectId || typeof WebSocket === 'undefined') return;
    const url = cfg.endpoint.replace(/^http/, 'ws');
    const client = createClient({
      url,
      connectionParams: () => {
        const t = cfg.getToken?.();
        return t ? { authorization: `Bearer ${t}` } : {};
      },
    });
    const dispose = client.subscribe<{ runProject: RawRunEvent }>(
      { query: RUN_PROJECT_SUB, variables: { projectId } },
      {
        next: (msg) => {
          const ev = msg.data?.runProject;
          if (ev) {
            setIsLive(true);
            setEvents((prev) => [mapRunEvent(ev), ...prev].slice(0, 200));
          }
        },
        error: () => setIsLive(false),
        complete: () => {},
      },
    );
    return () => {
      dispose();
      void client.dispose();
    };
  }, [cfg.endpoint, projectId]);

  return { events, isLive };
}
