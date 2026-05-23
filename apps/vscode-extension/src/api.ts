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
  files?: ProjectFile[];
  spec?: { idea?: string };
}

export interface ProjectFile {
  path: string;
  type?: string;
  size?: number;
  content?: string;
}

export type GateName =
  | 'spec' | 'ux' | 'arch' | 'code' | 'lint' | 'test' | 'security' | 'budget' | 'deploy';

export type GateStatus =
  | 'pending' | 'running' | 'passed' | 'failed' | 'blocked' | 'repaired';

export type IssueSeverity = 'info' | 'warning' | 'error' | 'critical';

export interface Issue {
  gate: GateName;
  severity: IssueSeverity;
  message: string;
  hint?: string;
  path?: string;
}

export interface GateState {
  name: GateName;
  status: GateStatus;
  issues?: Issue[];
  updatedAt: string;
}

export type PatchOp = 'create' | 'update' | 'delete';

export interface PatchChange {
  op: PatchOp;
  path: string;
  content?: string;
}

export type PatchStatus =
  | 'proposed'
  | 'validated'
  | 'applied'
  | 'rejected'
  | 'rolled-back';

export interface Patch {
  id: string;
  projectId: string;
  author?: string;
  title?: string;
  summary?: string;
  changes: PatchChange[];
  status: PatchStatus;
  createdAt: string;
  appliedAt?: string;
}

export interface BudgetSnapshot {
  tier: string;
  monthSpend: string;
  monthCap: string;
  hardStop: boolean;
}

export type WorkspaceStatus = 'creating' | 'running' | 'stopped' | 'error';

export interface Workspace {
  id: string;
  userId: string;
  projectId?: string;
  status: WorkspaceStatus;
  driver: string;
  root?: string;
  previewUrl?: string;
  ideUrl?: string;
  createdAt: string;
  updatedAt: string;
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

  listFiles(projectId: string): Promise<ProjectFile[]> {
    return this.request<ProjectFile[]>('GET', `/projects/${encodeURIComponent(projectId)}/files`);
  }

  listGates(projectId: string): Promise<GateState[]> {
    return this.request<GateState[]>('GET', `/projects/${encodeURIComponent(projectId)}/gates`);
  }

  listPatches(projectId: string): Promise<Patch[]> {
    return this.request<Patch[]>('GET', `/projects/${encodeURIComponent(projectId)}/patches`);
  }

  applyPatch(patchId: string): Promise<Patch> {
    return this.request<Patch>('POST', `/patches/${encodeURIComponent(patchId)}/apply`, {});
  }

  rejectPatch(patchId: string): Promise<Patch> {
    return this.request<Patch>('POST', `/patches/${encodeURIComponent(patchId)}/reject`, {});
  }

  runFinisher(id: string, gate?: GateName): Promise<unknown> {
    const body = gate ? { gate } : {};
    return this.request<unknown>('POST', `/projects/${encodeURIComponent(id)}/run`, body);
  }

  /** List the caller's workspaces from the runtime API. */
  async listWorkspaces(): Promise<Workspace[]> {
    return this.runtimeRequest<Workspace[]>('GET', '/workspaces/');
  }

  /** Look up the workspace tied to a project (first match wins). */
  async findWorkspaceForProject(projectId: string): Promise<Workspace | undefined> {
    const all = await this.listWorkspaces();
    return all.find((w) => w.projectId === projectId);
  }

  /** Spin up a fresh workspace for a project. */
  async createWorkspace(projectId: string): Promise<Workspace> {
    return this.runtimeRequest<Workspace>('POST', '/workspaces/', { projectId });
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

  /**
   * GETs the project's lifecycle event stream — finisher progress, gate
   * status changes, patch proposals. Long-lived; close by aborting the
   * AbortController.
   */
  async *streamEvents(projectId: string, signal: AbortSignal): AsyncGenerator<SSEEvent> {
    yield* this.streamGet(`/projects/${encodeURIComponent(projectId)}/stream`, signal);
  }

  // ---------- Internals ----------

  private request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const { orchestratorUrl } = readConfig();
    return this.doRequest<T>(orchestratorUrl, method, path, body);
  }

  private runtimeRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
    const { runtimeUrl } = readConfig();
    return this.doRequest<T>(runtimeUrl, method, path, body);
  }

  private async doRequest<T>(
    base: string,
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const token = await this.auth.getToken();
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(base + path, {
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
    yield* this.openStream('POST', path, body, signal);
  }

  private async *streamGet(path: string, signal: AbortSignal): AsyncGenerator<SSEEvent> {
    yield* this.openStream('GET', path, undefined, signal);
  }

  private async *openStream(
    method: 'GET' | 'POST',
    path: string,
    body: unknown,
    signal: AbortSignal,
  ): AsyncGenerator<SSEEvent> {
    const { orchestratorUrl } = readConfig();
    const token = await this.auth.getToken();
    const headers: Record<string, string> = { Accept: 'text/event-stream' };
    if (method === 'POST') headers['Content-Type'] = 'application/json';
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(orchestratorUrl + path, {
      method,
      headers,
      body: method === 'POST' ? JSON.stringify(body ?? {}) : undefined,
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
