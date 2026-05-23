// Patches client — thin wrapper around the orchestrator patch API. The
// orchestrator validates and applies every change through patch.Engine so
// the UI never writes to disk directly.

import { auth } from '../auth';
import { GateName } from '../api';

const base = '/api/orchestrator';

export type PatchStatus =
  | 'proposed' | 'validated' | 'applied' | 'rejected' | 'rolled-back';

export interface FileChange {
  op: 'create' | 'update' | 'delete' | 'replace' | 'insert_after';
  path: string;
  content?: string;
  // anchor + replacement are used by the partial-rewrite ops (replace,
  // insert_after). The orchestrator validates that anchor occurs exactly
  // once in the current file body before applying.
  anchor?: string;
  replacement?: string;
}

export interface PatchIssue {
  gate: GateName;
  severity: 'info' | 'warning' | 'error' | 'critical';
  message: string;
  hint?: string;
  path?: string;
}

export interface Patch {
  id: string;
  projectId: string;
  author: string;
  title: string;
  summary: string;
  changes: FileChange[];
  issues?: PatchIssue[];
  status: PatchStatus;
  createdAt: string;
  appliedAt?: string;
}

async function jsonFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${base}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...auth.authHeader(),
      ...(init?.headers ?? {}),
    },
    cache: 'no-store',
  });
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

export interface Snapshot {
  id: string;
  projectId: string;
  patchId?: string;
  title?: string;
  createdAt: string;
}

export const patches = {
  list: (projectId: string) => jsonFetch<Patch[]>(`/projects/${projectId}/patches`),
  propose: (projectId: string, body: Partial<Patch>) =>
    jsonFetch<Patch>(`/projects/${projectId}/patches`, {
      method: 'POST', body: JSON.stringify(body),
    }),
  apply: (projectId: string, patchId: string) =>
    jsonFetch<Patch>(`/projects/${projectId}/patches/${patchId}/apply`, { method: 'POST' }),
  // Rollback restores the snapshot taken immediately before this patch was
  // applied. Returns 409 if no snapshot exists (e.g. the patch was never
  // applied) so the caller can render a clear "nothing to undo" message.
  rollback: (patchId: string) =>
    jsonFetch<Snapshot>(`/patches/${patchId}/rollback`, { method: 'POST' }),
};
