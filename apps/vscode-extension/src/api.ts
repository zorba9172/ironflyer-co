// Thin client over the Ironflyer orchestrator HTTP API.
//
// One pattern: every call goes through `request()`, which attaches the
// Authorization header from Auth and surfaces structured errors. We use
// the runtime's built-in `fetch` (Node 18+ / VSCode bundles Node 20).
//
// SSE is handled by `stream()` — chat and event streams both speak the
// same `event: <name>\ndata: <json>\n\n` protocol on the orchestrator
// side, so we parse it in one place.

import { Auth } from './auth';
import { readConfig } from './config';
import { drainFrames, SSEEvent } from './sse';

export { SSEEvent };

export interface Project {
  id: string;
  name: string;
  description?: string;
  ownerId?: string;
  isPublic?: boolean;
  files?: unknown[];
  spec?: { idea?: string };
}

export interface BudgetSnapshot {
  tier: string;
  monthSpend: string;
  monthCap: string;
  hardStop: boolean;
}

export class ApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export class Api {
  constructor(private readonly auth: Auth) {}

  // ---------- REST ----------

  listProjects(): Promise<Project[]> {
    return this.request<Project[]>('GET', '/projects/');
  }

  getProject(id: string): Promise<Project> {
    return this.request<Project>('GET', `/projects/${encodeURIComponent(id)}`);
  }

  createProject(body: { name: string; idea?: string; description?: string }): Promise<Project> {
    return this.request<Project>('POST', '/projects/', {
      name: body.name,
      idea: body.idea ?? '',
      description: body.description ?? '',
    });
  }

  runFinisher(id: string): Promise<unknown> {
    return this.request<unknown>('POST', `/projects/${encodeURIComponent(id)}/run`, {});
  }

  myBudget(): Promise<BudgetSnapshot> {
    return this.request<BudgetSnapshot>('GET', '/budget/users/me');
  }

  me(): Promise<{ id: string; email: string; name?: string }> {
    return this.request('GET', '/auth/me');
  }

  // ---------- Streaming ----------

  /**
   * POSTs a chat prompt and yields parsed SSE events until the stream
   * closes or the AbortSignal fires.
   */
  async *chat(
    projectId: string,
    body: { prompt: string; role?: string; effort?: string },
    signal: AbortSignal,
  ): AsyncGenerator<SSEEvent> {
    yield* this.stream(`/projects/${encodeURIComponent(projectId)}/chat`, body, signal);
  }

  // ---------- Internals ----------

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const { orchestratorUrl } = readConfig();
    const token = await this.auth.getToken();
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(orchestratorUrl + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new ApiError(res.status, text || `${method} ${path} → ${res.status}`);
    }
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  private async *stream(
    path: string,
    body: unknown,
    signal: AbortSignal,
  ): AsyncGenerator<SSEEvent> {
    const { orchestratorUrl } = readConfig();
    const token = await this.auth.getToken();
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    };
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(orchestratorUrl + path, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal,
    });
    if (!res.ok || !res.body) {
      const text = res.body ? await res.text() : '';
      throw new ApiError(res.status, text || `stream ${path} → ${res.status}`);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const { events, rest } = drainFrames(buf);
      buf = rest;
      for (const evt of events) yield evt;
    }
  }
}
