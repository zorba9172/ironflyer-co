// Thin client for the Ironflyer workspace runtime. All requests go through
// the Next.js rewrite at /api/runtime/* so the browser doesn't need the
// upstream URL. The terminal WebSocket bypasses the rewrite — it connects
// directly because Next's dev rewriter doesn't proxy WS.

export interface Workspace {
  id: string;
  userId: string;
  projectId?: string;
  status: 'creating' | 'running' | 'stopped' | 'error';
  driver: string;
  root: string;
  previewUrl?: string;
  ideUrl?: string;
  createdAt: string;
  updatedAt: string;
}

export interface FileEntry {
  path: string;
  size: number;
  isDir: boolean;
}

export interface ExecRequest {
  shell?: string;
  cmd?: string[];
  cwd?: string;
  env?: string[];
  timeoutSeconds?: number;
}

export interface ExecResult {
  stdout: string;
  stderr: string;
  exitCode: number;
  durationMs: number;
  timedOut?: boolean;
  truncatedAt?: number;
}

const base = '/api/runtime';

async function jsonFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${base}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
    cache: 'no-store',
  });
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

export const runtime = {
  health: () => jsonFetch<{ ok: boolean; service: string; driver: string }>('/health'),
  list: () => jsonFetch<Workspace[]>('/workspaces'),
  create: (body: { userId: string; projectId?: string }) =>
    jsonFetch<Workspace>('/workspaces', { method: 'POST', body: JSON.stringify(body) }),
  get: (id: string) => jsonFetch<Workspace>(`/workspaces/${id}`),
  destroy: (id: string) => fetch(`${base}/workspaces/${id}`, { method: 'DELETE' }),
  listFiles: (id: string) => jsonFetch<FileEntry[]>(`/workspaces/${id}/files`),
  readFile: async (id: string, path: string) => {
    const res = await fetch(`${base}/workspaces/${id}/files/${encodeURI(path)}`);
    if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
    return res.text();
  },
  writeFile: (id: string, path: string, data: string) =>
    fetch(`${base}/workspaces/${id}/files/${encodeURI(path)}`, {
      method: 'PUT', body: data,
    }),
  exec: (id: string, body: ExecRequest) =>
    jsonFetch<ExecResult>(`/workspaces/${id}/exec`, {
      method: 'POST', body: JSON.stringify(body),
    }),
  // The terminal WS goes directly to the runtime, not via /api/runtime
  // (Next.js dev server doesn't proxy WebSockets).
  terminalURL: (id: string, runtimeBase = `ws://${typeof window !== 'undefined' ? window.location.hostname : 'localhost'}:8090`) =>
    `${runtimeBase}/workspaces/${id}/terminal`,
};
