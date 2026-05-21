// Thin client for the Ironflyer orchestrator. All requests go through the
// Next.js rewrite at /api/orchestrator/* so the browser never needs to know
// the upstream URL.

export type GateName =
  | 'spec' | 'ux' | 'arch' | 'code' | 'lint' | 'test' | 'security' | 'deploy';

export type GateStatus =
  | 'pending' | 'running' | 'passed' | 'failed' | 'blocked' | 'repaired';

export interface GateState {
  name: GateName;
  status: GateStatus;
  issues?: Issue[];
  updatedAt: string;
}

export interface Issue {
  gate: GateName;
  severity: 'info' | 'warning' | 'error' | 'critical';
  message: string;
  hint?: string;
  path?: string;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  status: string;
  spec: {
    idea: string;
    userStories?: any[] | null;
    dataModel?: any[] | null;
    stack: { frontend: string; backend: string; storage: string; auth: string };
  };
  files: { path: string; type: string; size?: number; content?: string }[];
  gates: Record<GateName, GateState>;
  events: ExecutionEvent[];
  github?: { fullName: string; defaultBranch: string; htmlUrl: string };
  ownerId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ExecutionEvent {
  id: string;
  step: string;
  agent?: string;
  gate?: GateName;
  message: string;
  status: string;
  createdAt: string;
}

export interface Plan {
  tier: string;
  name: string;
  monthlyPrice: string;
  costCapUSD: string;
  hardStop: boolean;
  allowList?: string[];
}

export interface Rate {
  provider: string;
  model: string;
  inputUSD: string;
  outputUSD: string;
  cacheReadUSD: string;
  cacheCreateUSD: string;
  capability?: string[];
}

export interface VaultSnapshot {
  revenue: string;
  providerCost: string;
  refunds: string;
  adjustments: string;
  margin: string;
}

export interface UserBudget {
  userId: string;
  tier: string;
  spent: string;
  entries: LedgerEntry[];
}

export interface LedgerEntry {
  id: string;
  userId: string;
  projectId?: string;
  provider: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  cacheRead: number;
  cacheCreate: number;
  costUSD: string;
  createdAt: string;
}

export interface BrainstormOutcome {
  plan: {
    mode: 'direct' | 'brainstorm' | 'debate' | 'research';
    roles: string[];
    rounds?: number;
    goal: string;
    reason: string;
  };
  outcome: {
    mode: string;
    winner?: string;
    synthesis: string;
    proposals?: { role: string; provider: string; output: string; score: number; tokens: number; costUSD: number }[];
    totalCostUSD: number;
    startedAt: string;
    finishedAt: string;
  };
}

import { auth } from './auth';

const base = '/api/orchestrator';

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
  if (res.status === 401) {
    auth.clear();
    if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')) {
      window.location.href = '/login';
    }
  }
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

export const api = {
  health: () => jsonFetch<{ ok: boolean; service: string; version: string }>('/health'),
  listProjects: () => jsonFetch<Project[]>('/projects'),
  getProject: (id: string) => jsonFetch<Project>(`/projects/${id}`),
  createProject: (body: { id?: string; name: string; description?: string; idea?: string }) =>
    jsonFetch<Project>('/projects', { method: 'POST', body: JSON.stringify(body) }),
  listGates: (id: string) => jsonFetch<GateState[]>(`/projects/${id}/gates`),
  runFinisher: (id: string) =>
    jsonFetch<{ projectId: string; iterations: number; gates: GateState[]; completed: boolean }>(
      `/projects/${id}/run`, { method: 'POST' },
    ),
  brainstorm: (id: string, body: { goal: string; role?: string }) =>
    jsonFetch<BrainstormOutcome>(`/projects/${id}/brainstorm`,
      { method: 'POST', body: JSON.stringify(body) }),
  // SSE: EventSource can't set headers so we append ?token=<jwt>.
  streamURL: (id: string) => auth.appendTokenParam(`${base}/projects/${id}/stream`),
  chatURL:  (id: string) => `${base}/projects/${id}/chat`,
  // budget
  listPlans: () => jsonFetch<Plan[]>('/budget/plans'),
  listRates: () => jsonFetch<Rate[]>('/budget/rates'),
  vault: () => jsonFetch<VaultSnapshot>('/budget/vault'),
  // Per-user budget is the authenticated user's; orchestrator infers from JWT.
  myBudget: () => jsonFetch<UserBudget>(`/budget/users/me`),
  // Stripe checkout — returns the URL to redirect the browser to.
  startCheckout: (tier: string) =>
    jsonFetch<{ url: string }>(`/budget/checkout`, {
      method: 'POST', body: JSON.stringify({ tier }),
    }),
};

// streamChat opens a POST SSE stream against /chat. Browsers do not allow
// EventSource for POST, so we use fetch+ReadableStream and parse SSE manually.
export type ChatDelta =
  | { kind: 'turn'; id: string; role: string }
  | { kind: 'start'; provider: string; model: string; turn: string }
  | { kind: 'text'; text: string; turn: string }
  | { kind: 'thinking'; text: string; turn: string }
  | { kind: 'tool_use'; data: unknown }
  | { kind: 'done'; turn: string; provider: string; model: string; usage?: any }
  | { kind: 'error'; error: string };

export type Effort = 'lite' | 'economy' | 'power';

export async function streamChat(
  projectId: string,
  body: { prompt: string; role?: string; effort?: Effort },
  onDelta: (d: ChatDelta) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(api.chatURL(projectId), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...auth.authHeader() },
    body: JSON.stringify(body),
    signal,
  });
  if (!res.ok || !res.body) {
    onDelta({ kind: 'error', error: `${res.status}: ${await res.text()}` });
    return;
  }
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = '';
  while (true) {
    const { value, done } = await reader.read();
    if (done) return;
    buf += dec.decode(value, { stream: true });
    // SSE separator: blank line (\n\n)
    let idx: number;
    while ((idx = buf.indexOf('\n\n')) >= 0) {
      const block = buf.slice(0, idx);
      buf = buf.slice(idx + 2);
      let event = 'message';
      let data = '';
      for (const line of block.split('\n')) {
        if (line.startsWith('event:')) event = line.slice(6).trim();
        else if (line.startsWith('data:')) data += line.slice(5).trim();
      }
      if (!data) continue;
      try {
        const parsed = JSON.parse(data);
        switch (event) {
          case 'turn':  onDelta({ kind: 'turn', id: parsed.id, role: parsed.role }); break;
          case 'start': onDelta({ kind: 'start', provider: parsed.provider, model: parsed.model, turn: parsed.turn }); break;
          case 'text':  onDelta({ kind: 'text', text: parsed.text, turn: parsed.turn }); break;
          case 'thinking': onDelta({ kind: 'thinking', text: parsed.text, turn: parsed.turn }); break;
          case 'tool_use': onDelta({ kind: 'tool_use', data: parsed }); break;
          case 'done':  onDelta({ kind: 'done', turn: parsed.turn, provider: parsed.provider, model: parsed.model, usage: parsed.usage }); break;
          case 'error': onDelta({ kind: 'error', error: parsed.error }); break;
        }
      } catch {
        // swallow malformed event
      }
    }
  }
}
