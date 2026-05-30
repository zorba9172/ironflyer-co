// Runtime service client — talks to the per-user workspace runtime (File API,
// PTY, and the embedded web IDE) rather than the GraphQL orchestrator. The
// runtime is a separate Go service; in dev the studio Vite proxy maps
// `/api/runtime` → http://localhost:8090.
//
// The orchestrator stays GraphQL-only (see CLAUDE.md). The runtime is REST by
// design and owns these workspace-local resources.

import { useQuery } from '@tanstack/react-query';
import { useDataConfig } from './provider';

// `import.meta.env` is injected by Vite in the consuming apps; the cast keeps
// this package type-checkable on its own (its tsconfig has no vite/client types).
const viteEnv = (import.meta as unknown as { env?: Record<string, string | undefined> }).env;
export const RUNTIME_BASE = viteEnv?.VITE_RUNTIME_URL ?? '/api/runtime';

export class RuntimeError extends Error {
  constructor(message: string, public readonly status: number) {
    super(message);
    this.name = 'RuntimeError';
  }
}

/**
 * Thin fetch wrapper for the runtime REST API. Returns the parsed JSON body and
 * the HTTP status so callers can distinguish "ready" (200) from "still starting"
 * (202) — both are valid, non-error responses. Only 4xx/5xx throw.
 */
export async function runtimeFetch<T>(
  path: string,
  init?: RequestInit & { token?: string | null },
): Promise<{ data: T; status: number }> {
  const { token, headers, ...rest } = init ?? {};
  const res = await fetch(`${RUNTIME_BASE}${path}`, {
    ...rest,
    headers: {
      accept: 'application/json',
      ...(token ? { authorization: `Bearer ${token}` } : {}),
      ...headers,
    },
  });

  // 202 Accepted is a valid "not ready yet" response for long-running
  // provisioning (e.g. the IDE backend spinning up). Do NOT treat it as an
  // error — surface the status so the caller can decide to keep polling.
  if (!res.ok && res.status !== 202) {
    let detail = '';
    try {
      detail = await res.text();
    } catch {
      /* ignore */
    }
    throw new RuntimeError(detail || `Runtime request failed (${res.status})`, res.status);
  }

  let data: T;
  try {
    data = (await res.json()) as T;
  } catch {
    data = {} as T;
  }
  return { data, status: res.status };
}

// ── Workspace files ───────────────────────────────────────────────────────

/**
 * Write a single file into the runtime workspace for a project. Used to push
 * studio-generated code into the workspace the embedded IDE opens, so chat
 * output shows up in the editor. The workspace must already exist (the IDE
 * endpoint provisions it); writes resolve by project id via the runtime.
 *
 * Backend: `PUT /workspaces/{projectId}/files/{path}` with a raw body.
 */
export async function writeWorkspaceFile(
  projectId: string,
  path: string,
  content: string,
  token?: string | null,
): Promise<void> {
  const encodedPath = path
    .split('/')
    .filter(Boolean)
    .map(encodeURIComponent)
    .join('/');
  await runtimeFetch(`/workspaces/${encodeURIComponent(projectId)}/files/${encodedPath}`, {
    method: 'PUT',
    body: content,
    headers: { 'content-type': 'application/octet-stream' },
    token,
  });
}

// ── Workspace IDE ─────────────────────────────────────────────────────────

export interface WorkspaceIde {
  /** Browser-reachable URL to iframe once `ready` is true; empty while starting. */
  url: string;
  /** True when the IDE backend is up and the URL can be loaded. */
  ready: boolean;
}

/**
 * Resolves the embedded web IDE (Eclipse Theia) for a workspace/project.
 *
 * Backend contract: `GET /workspaces/{id}/ide`
 *   - 200 `{ url, ready: true }`  → IDE backend is up; iframe `url`.
 *   - 202 `{ url: "", ready: false }` → still starting; client should poll.
 *
 * Polls every 2s while `ready` is false and stops once ready. Disabled until a
 * project id is provided.
 */
export function useWorkspaceIde(projectId?: string) {
  // The runtime's /workspaces tree sits behind JWT auth (it trusts the
  // orchestrator's signature), so the IDE lookup MUST carry the same bearer
  // token the GraphQL client uses — otherwise the runtime answers 401
  // {"error":"missing token"} and the IDE pane reads as "offline".
  const { getToken } = useDataConfig();
  const query = useQuery<WorkspaceIde>({
    queryKey: ['workspace-ide', projectId ?? 'none'],
    enabled: !!projectId,
    queryFn: async () => {
      const { data, status } = await runtimeFetch<Partial<WorkspaceIde>>(
        `/workspaces/${encodeURIComponent(projectId as string)}/ide`,
        { token: getToken?.() },
      );
      const ready = status === 200 && !!data.ready;
      return { url: ready ? data.url ?? '' : '', ready };
    },
    // Poll while starting; stop once the IDE reports ready.
    refetchInterval: (q) => (q.state.data?.ready ? false : 2000),
    refetchOnWindowFocus: false,
    staleTime: 0,
  });

  return {
    url: query.data?.url ?? '',
    ready: query.data?.ready ?? false,
    isLoading: query.isPending && !!projectId,
    isError: query.isError,
    error: query.error as Error | null,
  };
}
