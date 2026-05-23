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

export interface EnterpriseLead {
  name?: string;
  email: string;
  company: string;
  teamSize?: string;
  useCase?: string;
  budget?: string;
  timeline?: string;
  source?: string;
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

// VisualTarget is a pixel-perfect reference screenshot the UX gate diffs
// the live preview against. `imagePngBase64` is the raw PNG bytes
// base64-encoded (no `data:` prefix) — the orchestrator caps uploads at
// 4 MiB.
export interface VisualTarget {
  id: string;
  name?: string;
  routeHint?: string;
  viewportW: number;
  viewportH: number;
  imagePngBase64: string;
  tolerance?: number;
}

// Subproject models one service inside a monorepo project. `path` is what
// the file claimer uses to route generated files to the right directory.
export interface Subproject {
  id: string;
  name: string;
  path: string;
  stack?: {
    frontend?: string;
    backend?: string;
    storage?: string;
    auth?: string;
  };
  role?: 'frontend' | 'backend' | 'worker' | 'mobile' | 'ml' | 'other' | string;
  createdAt: string;
}

// ---------- Intelligence surfaces (memory / audit / telemetry / graph) -------

export type MemoryKind = 'project' | 'execution' | 'user' | 'business';

export interface MemoryRecord {
  id: string;
  kind: MemoryKind;
  projectId?: string;
  userId?: string;
  storyId?: string;
  gateName?: string;
  title: string;
  body: string;
  tags?: string[];
  confidence?: number;
  createdAt: string;
}

export type AuditAction =
  | 'patch.proposed'
  | 'patch.applied'
  | 'patch.rolled_back'
  | 'gate.verdict'
  | 'agent.dispatch'
  | 'secret.written'
  | 'workspace.exec'
  | 'deploy'
  | 'memory.record';

export type AuditOutcome = 'success' | 'failure' | 'blocked';

export interface AuditEntry {
  id: string;
  action: AuditAction;
  outcome: AuditOutcome;
  userId?: string;
  projectId?: string;
  storyId?: string;
  gateName?: string;
  agentRole?: string;
  summary: string;
  inputHash?: string;
  outputHash?: string;
  attrs?: Record<string, unknown>;
  createdAt: string;
  prevHash?: string;
  contentHash: string;
}

export interface AgentCall {
  userId: string;
  projectId?: string;
  role?: string;
  provider: string;
  model: string;
  capabilities?: string[];
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens?: number;
  cacheNewTokens?: number;
  costUSD: number;
  durationMs: number;
  startedAt: string;
  error?: string;
}

export interface GraphNode {
  path: string;
  language: 'ts' | 'go' | 'py' | 'other' | string;
  exports?: string[];
  symbolCount?: number;
}

export interface GraphEdge {
  from: string;
  to: string;
  raw: string;
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
import type { Patch } from './api/patches';

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
  // Visual-edit hands a selector + instruction (+ optional screenshot) to
  // the Coder agent. The orchestrator parses the response into a Patch and
  // returns it ready for the existing PatchDrawer Apply flow.
  visualEdit: (id: string, body: {
    selector: string;
    instruction: string;
    screenshot?: string;
    screenshotMediaType?: 'image/png' | 'image/jpeg' | 'image/webp';
    path?: string;
  }) =>
    jsonFetch<Patch>(`/projects/${id}/visual-edit`, {
      method: 'POST', body: JSON.stringify(body),
    }),
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
  submitEnterpriseLead: (body: EnterpriseLead) =>
    jsonFetch<{ id: string; status: string; createdAt: string }>(
      '/leads/enterprise',
      { method: 'POST', body: JSON.stringify(body) },
    ),
  // ---- subprojects --------------------------------------------------
  // Monorepo modeling: one entry per app/service inside a project.
  listSubprojects: async (id: string) => {
    const r = await jsonFetch<{ subprojects: Subproject[] | null; count: number }>(
      `/projects/${id}/subprojects`,
    );
    return r.subprojects ?? [];
  },
  addSubproject: (
    id: string,
    body: {
      name: string;
      path: string;
      role?: string;
      stack?: { frontend?: string; backend?: string; storage?: string; auth?: string };
    },
  ) =>
    jsonFetch<{ subproject: Subproject; count: number }>(
      `/projects/${id}/subprojects`,
      { method: 'POST', body: JSON.stringify(body) },
    ),
  deleteSubproject: (id: string, subId: string) =>
    jsonFetch<{ ok: true }>(
      `/projects/${id}/subprojects/${subId}`,
      { method: 'DELETE' },
    ),
  // ---- visual targets -----------------------------------------------
  // Pixel-perfect references the UX gate diffs against the live preview.
  listVisualTargets: async (id: string) => {
    const r = await jsonFetch<{ targets: VisualTarget[] | null; count: number }>(
      `/projects/${id}/visual-targets`,
    );
    return r.targets ?? [];
  },
  addVisualTarget: (
    id: string,
    body: {
      name?: string;
      routeHint?: string;
      viewportW: number;
      viewportH: number;
      imagePngBase64: string;
      tolerance?: number;
    },
  ) =>
    jsonFetch<{ target: VisualTarget; count: number }>(
      `/projects/${id}/visual-targets`,
      { method: 'POST', body: JSON.stringify(body) },
    ),
  deleteVisualTarget: (id: string, targetId: string) =>
    jsonFetch<{ ok: true }>(
      `/projects/${id}/visual-targets/${targetId}`,
      { method: 'DELETE' },
    ),
  // ---- intelligence: memory / audit / telemetry / graph ----------------
  // Persistent project intelligence — newest-first. At least one of kind /
  // projectId is required so the response is scoped, not a firehose.
  listMemory: (opts: {
    kind?: MemoryKind;
    projectId?: string;
    q?: string;
    tag?: string;
    limit?: number;
  } = {}) => {
    const params = new URLSearchParams();
    if (opts.kind) params.set('kind', opts.kind);
    if (opts.projectId) params.set('projectId', opts.projectId);
    if (opts.q) params.set('q', opts.q);
    if (opts.tag) params.set('tag', opts.tag);
    if (opts.limit) params.set('limit', String(opts.limit));
    const qs = params.toString();
    return jsonFetch<{ records: MemoryRecord[] | null; count: number }>(
      `/memory${qs ? `?${qs}` : ''}`,
    ).then((r) => ({ records: r.records ?? [], count: r.count }));
  },
  // Immutable audit log — append-only hash chain. Filter by action /
  // outcome / time window; capped at 1000 entries per call by the server.
  listAudit: (opts: {
    projectId?: string;
    action?: string;
    outcome?: string;
    since?: string;
    until?: string;
    limit?: number;
  } = {}) => {
    const params = new URLSearchParams();
    if (opts.projectId) params.set('projectId', opts.projectId);
    if (opts.action) params.set('action', opts.action);
    if (opts.outcome) params.set('outcome', opts.outcome);
    if (opts.since) params.set('since', opts.since);
    if (opts.until) params.set('until', opts.until);
    if (opts.limit) params.set('limit', String(opts.limit));
    const qs = params.toString();
    return jsonFetch<{ entries: AuditEntry[] | null; count: number }>(
      `/audit${qs ? `?${qs}` : ''}`,
    ).then((r) => ({ entries: r.entries ?? [], count: r.count }));
  },
  // Compliance attestation: walks the chain and returns whether tampering
  // has been detected. firstBadIndex is -1 when the chain is intact.
  verifyAudit: () =>
    jsonFetch<{ intact: boolean; firstBadIndex: number }>(`/audit/verify`),
  // Per-call agent telemetry — provider/model/tokens/cost/duration.
  listTelemetry: (opts: {
    limit?: number;
    role?: string;
    provider?: string;
    model?: string;
  } = {}) => {
    const params = new URLSearchParams();
    if (opts.limit) params.set('limit', String(opts.limit));
    if (opts.role) params.set('role', opts.role);
    if (opts.provider) params.set('provider', opts.provider);
    if (opts.model) params.set('model', opts.model);
    const qs = params.toString();
    return jsonFetch<{ calls: AgentCall[] | null; count: number }>(
      `/telemetry/agents${qs ? `?${qs}` : ''}`,
    ).then((r) => ({ calls: r.calls ?? [], count: r.count }));
  },
  // Derived dependency graph for a project — nodes + edges across the
  // project's in-memory file tree.
  projectGraph: (id: string) =>
    jsonFetch<{ nodes: GraphNode[] | null; edges: GraphEdge[] | null }>(
      `/projects/${id}/graph`,
    ).then((r) => ({ nodes: r.nodes ?? [], edges: r.edges ?? [] })),
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

// ChatAttachment is one user-supplied image carried inline with the prompt.
// `base64` is the raw image bytes, base64-encoded with no `data:` prefix —
// the orchestrator rejects values that look like data URLs.
export interface ChatAttachment {
  mediaType: 'image/png' | 'image/jpeg' | 'image/webp' | 'image/gif';
  base64: string;
}

export async function streamChat(
  projectId: string,
  body: { prompt: string; role?: string; effort?: Effort; attachments?: ChatAttachment[] },
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
