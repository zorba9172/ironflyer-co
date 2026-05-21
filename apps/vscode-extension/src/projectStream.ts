// Multiplexes /projects/{id}/stream subscriptions across UI surfaces.
//
// Why this exists: each chat panel + tree refresh path wants to know when
// the project's lifecycle changes. Without multiplexing we'd open one
// long-lived HTTP connection per surface. With refcounting we open one
// per active projectId, no matter how many subscribers are listening.
//
// Lifecycle: the first subscribe() for a projectId opens the SSE stream
// and starts pumping events into the registered callbacks. The last
// dispose() aborts the connection. Reconnects on transport error use
// exponential backoff capped at 30s.

import { Api, SSEEvent } from './api';

export type ProjectEventListener = (event: SSEEvent) => void;

interface Subscription {
  listeners: Set<ProjectEventListener>;
  controller: AbortController;
  retry: number;
}

const BACKOFF_BASE_MS = 1_000;
const BACKOFF_MAX_MS = 30_000;

export class ProjectStream {
  private readonly subs = new Map<string, Subscription>();
  private readonly logger: (msg: string, err?: unknown) => void;

  constructor(
    private readonly api: Api,
    logger?: (msg: string, err?: unknown) => void,
  ) {
    this.logger = logger ?? (() => {});
  }

  subscribe(projectId: string, listener: ProjectEventListener): { dispose: () => void } {
    let sub = this.subs.get(projectId);
    if (!sub) {
      sub = { listeners: new Set(), controller: new AbortController(), retry: 0 };
      this.subs.set(projectId, sub);
      void this.pump(projectId, sub);
    }
    sub.listeners.add(listener);
    return {
      dispose: () => {
        const s = this.subs.get(projectId);
        if (!s) return;
        s.listeners.delete(listener);
        if (s.listeners.size === 0) {
          s.controller.abort();
          this.subs.delete(projectId);
        }
      },
    };
  }

  disposeAll(): void {
    for (const sub of this.subs.values()) sub.controller.abort();
    this.subs.clear();
  }

  private async pump(projectId: string, sub: Subscription): Promise<void> {
    while (this.subs.get(projectId) === sub && !sub.controller.signal.aborted) {
      try {
        for await (const evt of this.api.streamEvents(projectId, sub.controller.signal)) {
          sub.retry = 0;
          for (const l of sub.listeners) {
            try { l(evt); } catch (err) { this.logger('listener threw', err); }
          }
        }
        // Stream closed cleanly — orchestrator restarted, or auth expired.
        if (sub.controller.signal.aborted) return;
      } catch (err) {
        if (sub.controller.signal.aborted) return;
        this.logger(`stream ${projectId} error`, err);
      }
      const delay = Math.min(BACKOFF_BASE_MS * Math.pow(2, sub.retry), BACKOFF_MAX_MS);
      sub.retry++;
      await sleep(delay, sub.controller.signal);
    }
  }
}

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const t = setTimeout(resolve, ms);
    signal.addEventListener(
      'abort',
      () => { clearTimeout(t); resolve(); },
      { once: true },
    );
  });
}
