import { useEffect, useState } from 'react';
import { useDataConfig } from './provider';

export interface FeedEvent {
  id: string;
  ts: number;
  kind: string;
  text: string;
}

// Subscribes to a Server-Sent Events stream (the orchestrator exposes execution
// feeds over SSE, authed via ?token=). Offline, it returns the fallback list.
export function useEventStream<T extends FeedEvent>(opts: {
  path?: string;
  fallback: T[];
  max?: number;
}): { events: T[]; isLive: boolean } {
  const cfg = useDataConfig();
  const [events, setEvents] = useState<T[]>(opts.fallback);
  const [isLive, setIsLive] = useState(false);
  const max = opts.max ?? 50;

  useEffect(() => {
    if (!cfg.endpoint || !opts.path || typeof EventSource === 'undefined') return;
    const base = cfg.endpoint.replace(/\/graphql\/?$/, '');
    const token = cfg.getToken?.();
    const url = `${base}${opts.path}${token ? `?token=${encodeURIComponent(token)}` : ''}`;
    const es = new EventSource(url);
    es.onopen = () => setIsLive(true);
    es.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data) as T;
        setEvents((prev) => [ev, ...prev].slice(0, max));
      } catch {
        /* ignore malformed frames */
      }
    };
    es.onerror = () => {
      setIsLive(false);
      es.close();
    };
    return () => es.close();
  }, [cfg.endpoint, opts.path, max]);

  return { events, isLive };
}
