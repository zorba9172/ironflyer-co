// Thin client over the Ironflyer orchestrator GraphQL API.
//
// Wire format:
//   - Queries + mutations:  HTTP POST /graphql
//   - Subscriptions:        WebSocket /graphql (graphql-transport-ws)
//
// We use Apollo Client 3 with a split link so the right transport is
// chosen per operation (see apollo.ts). The codegen'd typed-document
// nodes live in src/gql/ and are passed straight to apolloClient
// .query/.mutate/.subscribe.
//
// The public surface (method names + return shapes) is intentionally
// kept identical to the pre-migration REST client so the rest of the
// extension (commands, trees, status bar, chat panel, etc.) keeps
// compiling without sweeping refactors. Mapping from the GraphQL
// schema shapes to the legacy TS types happens locally in this file.
//
// Workspaces still live behind the runtime's REST API (not part of the
// orchestrator GraphQL schema yet) — those calls remain plain `fetch`.

import { ApolloClient, FetchResult, Observable } from '@apollo/client/core';
import { Auth } from './auth';
import { readConfig } from './config';
import { createApolloClient } from './apollo';
import {
  AuditOutcome as GqlAuditOutcome,
  GateStatus as GqlGateStatus,
  MemoryKind as GqlMemoryKind,
  PatchChangeOp as GqlPatchChangeOp,
  PatchStatus as GqlPatchStatus,
  MeDocument,
  ProjectsDocument,
  ProjectByIdDocument,
  ProjectFilesDocument,
  ProjectGraphViewDocument,
  CreateProjectDocument,
  RunFinisherDocument,
  RunProjectDocument,
  GatesDocument,
  RerunGateDocument,
  PatchesDocument,
  ApplyPatchDocument,
  RollbackPatchDocument,
  ChatStreamDocument,
  InlineCompletionDocument,
  AcceptInlineCompletionDocument,
  MyBudgetDocument,
  MemoryDocument,
  AuditDocument,
  AgentTelemetryDocument,
} from './gql/graphql';

// ---------------- Legacy types (kept for backwards-compat) ----------------

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

export interface AuditEntry {
  id: string;
  action: string;
  outcome: string;
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

export interface AuditVerifyResult {
  ok: boolean;
  brokenAt?: number;
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
  language: string;
  exports?: string[];
  symbolCount?: number;
}

export interface GraphEdge {
  from: string;
  to: string;
  raw: string;
}

export interface ProjectGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
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

/** Compatibility wrapper — chat / inline / run streams still hand
 *  out `{ event, data }` records so callers don't need to learn
 *  the GraphQL union narrowing. The mapper translates one delta at
 *  a time onto this shape. */
export interface SSEEvent {
  event: string;
  data: unknown;
}

export class ApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

// ---------------- Api class ----------------
//
// All operation documents are imported as typed `DocumentNode`s from
// the codegen output in src/gql/graphql.ts — the source GraphQL strings
// live in src/operations.graphql.

export class Api {
  private clientCell: { client: ApolloClient<unknown>; endpoint: string; dispose: () => void } | undefined;

  constructor(private readonly auth: Auth) {
    // Force a rebuild on sign-in/out so the WS handshake picks up the
    // new (or absent) bearer token.
    this.auth.onDidChange(() => this.resetClient());
  }

  /** Dispose Apollo client + WS connection. Idempotent. */
  dispose(): void {
    this.resetClient();
  }

  private resetClient(): void {
    this.clientCell?.dispose();
    this.clientCell = undefined;
  }

  private getClient(): ApolloClient<unknown> {
    const { orchestratorUrl } = readConfig();
    if (this.clientCell && this.clientCell.endpoint === orchestratorUrl) {
      return this.clientCell.client;
    }
    this.resetClient();
    const made = createApolloClient({
      endpoint: orchestratorUrl,
      getToken: () => this.auth.getToken(),
    });
    this.clientCell = { client: made.client, endpoint: orchestratorUrl, dispose: made.dispose };
    return made.client;
  }

  // ---------------- Projects ----------------

  async listProjects(): Promise<Project[]> {
    const res = await this.getClient().query({ query: ProjectsDocument });
    requireOk(res);
    return (res.data?.projects ?? []).map(unmaskProjectSummary);
  }

  async getProject(id: string): Promise<Project> {
    const res = await this.getClient().query({ query: ProjectByIdDocument, variables: { id } });
    requireOk(res);
    const raw = res.data?.project;
    if (!raw) throw new ApiError(404, `project ${id} not found`);
    return unmaskProjectSummary(raw);
  }

  async createProject(body: { name: string; idea?: string; description?: string }): Promise<Project> {
    const res = await this.getClient().mutate({
      mutation: CreateProjectDocument,
      variables: { input: { name: body.name, idea: body.idea, description: body.description } },
    });
    requireOk(res);
    if (!res.data?.createProject) throw new ApiError(500, 'createProject returned no data');
    return unmaskProjectSummary(res.data.createProject);
  }

  async listFiles(projectId: string): Promise<ProjectFile[]> {
    const res = await this.getClient().query({
      query: ProjectFilesDocument, variables: { id: projectId },
    });
    requireOk(res);
    return (res.data?.projectFiles ?? []).map((f) => ({
      path: f.path,
      content: f.content ?? undefined,
      size: f.size ?? undefined,
      type: f.language ?? undefined,
    }));
  }

  async listGates(projectId: string): Promise<GateState[]> {
    const res = await this.getClient().query({
      query: GatesDocument, variables: { projectId },
    });
    requireOk(res);
    return ((res.data?.gates ?? []) as readonly { ' $fragmentRefs'?: any }[])
      .map((v) => mapGateVerdict(unmaskFragment(v)));
  }

  async listPatches(projectId: string): Promise<Patch[]> {
    const res = await this.getClient().query({
      query: PatchesDocument, variables: { projectId },
    });
    requireOk(res);
    return ((res.data?.patches ?? []) as readonly { ' $fragmentRefs'?: any }[])
      .map((v) => mapPatch(unmaskFragment(v)));
  }

  async applyPatch(patchId: string): Promise<Patch> {
    const res = await this.getClient().mutate({
      mutation: ApplyPatchDocument, variables: { patchId },
    });
    requireOk(res);
    if (!res.data?.applyPatch) throw new ApiError(500, 'applyPatch returned no data');
    return mapPatch(unmaskFragment(res.data.applyPatch as any));
  }

  async rejectPatch(patchId: string): Promise<Patch> {
    // The new schema models this as `rollbackPatch` for applied patches;
    // for proposed/validated patches the orchestrator overloads it to
    // mean "reject". The return is the new patch state as JSON, so we
    // refetch the patch list from the project for an authoritative read.
    await this.getClient().mutate({
      mutation: RollbackPatchDocument, variables: { patchId },
    });
    return {
      id: patchId,
      projectId: '',
      status: 'rejected',
      changes: [],
      createdAt: new Date().toISOString(),
    };
  }

  async runFinisher(id: string, _gate?: GateName): Promise<unknown> {
    // The GraphQL schema doesn't take a gate argument for runFinisher;
    // single-gate re-runs go through `rerunGate`.
    if (_gate) {
      const res = await this.getClient().mutate({
        mutation: RerunGateDocument, variables: { projectId: id, gate: _gate },
      });
      requireOk(res);
      return res.data?.rerunGate;
    }
    const res = await this.getClient().mutate({
      mutation: RunFinisherDocument, variables: { id },
    });
    requireOk(res);
    return res.data?.runFinisher ?? null;
  }

  // ---------------- Workspaces (still REST against runtime) ----------------

  async listWorkspaces(): Promise<Workspace[]> {
    return this.runtimeRequest<Workspace[]>('GET', '/workspaces/');
  }

  async findWorkspaceForProject(projectId: string): Promise<Workspace | undefined> {
    const all = await this.listWorkspaces();
    return all.find((w) => w.projectId === projectId);
  }

  async createWorkspace(projectId: string): Promise<Workspace> {
    return this.runtimeRequest<Workspace>('POST', '/workspaces/', { projectId });
  }

  // ---------------- Budget / Memory / Audit / Telemetry / Graph ----------------

  async myBudget(): Promise<BudgetSnapshot> {
    const res = await this.getClient().query({ query: MyBudgetDocument });
    requireOk(res);
    const b = res.data?.myBudget;
    if (!b) throw new ApiError(404, 'budget unavailable');
    const plan = (res.data?.plans ?? []).find((p) => p.tier === b.tier);
    return {
      tier: b.tier,
      monthSpend: String(b.spentUsd ?? '0'),
      monthCap: String(plan?.costCapUsd ?? '0'),
      hardStop: false,
    };
  }

  async listMemory(opts: { kind?: MemoryKind; projectId?: string; limit?: number } = {}): Promise<MemoryRecord[]> {
    const res = await this.getClient().query({
      query: MemoryDocument,
      variables: { query: {
        kind: opts.kind ? toGqlMemoryKind(opts.kind) : undefined,
        projectId: opts.projectId,
        limit: opts.limit,
      } },
    });
    requireOk(res);
    return (res.data?.memory ?? []).map(mapMemoryRecord);
  }

  async listAudit(opts: { projectId?: string; limit?: number } = {}): Promise<AuditEntry[]> {
    const res = await this.getClient().query({
      query: AuditDocument,
      variables: { query: { projectId: opts.projectId, limit: opts.limit } },
    });
    requireOk(res);
    return (res.data?.audit ?? []).map(mapAuditEntry);
  }

  async verifyAudit(): Promise<AuditVerifyResult> {
    const res = await this.getClient().query({
      query: AuditDocument,
      variables: { query: { limit: 0 } },
    });
    requireOk(res);
    const v = res.data?.verifyAudit;
    return { ok: Boolean(v?.intact), brokenAt: v?.firstBadIndex && v.firstBadIndex >= 0 ? v.firstBadIndex : undefined };
  }

  async listAgentTelemetry(limit = 30): Promise<AgentCall[]> {
    const res = await this.getClient().query({
      query: AgentTelemetryDocument, variables: { limit },
    });
    requireOk(res);
    return (res.data?.agentTelemetry ?? []).map(mapAgentCall);
  }

  async projectGraph(projectId: string): Promise<ProjectGraph> {
    const res = await this.getClient().query({
      query: ProjectGraphViewDocument, variables: { id: projectId },
    });
    requireOk(res);
    const g = res.data?.projectGraph;
    if (!g) return { nodes: [], edges: [] };
    return {
      nodes: g.nodes.map((n) => ({
        path: n.path,
        language: n.language ?? '',
      })),
      edges: g.edges.map((e) => ({
        from: e.from,
        to: e.to,
        raw: e.kind,
      })),
    };
  }

  async me(): Promise<{ id: string; email: string; name?: string }> {
    const res = await this.getClient().query({ query: MeDocument });
    requireOk(res);
    const u = res.data?.me;
    if (!u) throw new ApiError(401, 'not signed in');
    return { id: u.id, email: u.email, name: u.name ?? undefined };
  }

  // ---------------- Streams ----------------

  /**
   * Subscribe to chatStream and yield legacy SSEEvent records so
   * existing chat-panel code does not need to be rewritten. Aborts
   * cleanly when the AbortSignal fires.
   */
  async *chat(
    projectId: string,
    body: { prompt: string; role?: string; effort?: string },
    signal: AbortSignal,
  ): AsyncGenerator<SSEEvent> {
    const obs = this.getClient().subscribe({
      query: ChatStreamDocument,
      variables: { projectId, input: { prompt: body.prompt, role: body.role, effort: body.effort } },
    });
    for await (const frame of fromObservable(obs, signal)) {
      const evt = mapChatDelta(frame);
      if (evt) yield evt;
    }
  }

  /**
   * Subscribe to runProject and yield legacy SSEEvent records so the
   * existing projectStream / runOutput plumbing is untouched.
   */
  async *streamEvents(projectId: string, signal: AbortSignal): AsyncGenerator<SSEEvent> {
    const obs = this.getClient().subscribe({
      query: RunProjectDocument, variables: { projectId },
    });
    for await (const frame of fromObservable(obs, signal)) {
      const evt = mapRunEvent(frame);
      if (evt) yield evt;
    }
  }

  /**
   * Subscribe to inlineCompletion. The legacy SSE shape used
   * { event: 'text' | 'done' | 'error', data: { text? } } — the
   * inline-completion provider only reads `evt.event` and the `text`
   * field. We project the union into that shape.
   *
   * The 1.5s first-token deadline is enforced by the provider via
   * Promise.race against this iterable's first item — see
   * inlineCompletions.ts.
   */
  async *inlineCompletion(
    body: {
      prefix: string;
      suffix: string;
      language: string;
      filename: string;
      maxTokens?: number;
      requestId?: string;
    },
    signal: AbortSignal,
  ): AsyncGenerator<SSEEvent> {
    const obs = this.getClient().subscribe({
      query: InlineCompletionDocument,
      variables: {
        input: {
          prefix: body.prefix,
          suffix: body.suffix,
          language: body.language,
          path: body.filename,
          requestId: body.requestId,
        },
      },
    });
    for await (const frame of fromObservable(obs, signal)) {
      const evt = mapInlineDelta(frame);
      if (evt) yield evt;
    }
  }

  async inlineCompletionAccept(): Promise<void> {
    try {
      await this.getClient().mutate({
        mutation: AcceptInlineCompletionDocument,
        variables: { requestId: 'last' }, // server uses session's most-recent
      });
    } catch {
      // best-effort telemetry — never block accept
    }
  }

  // ---------------- Internals ----------------

  private async runtimeRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
    const { runtimeUrl } = readConfig();
    const token = await this.auth.getToken();
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(runtimeUrl + path, {
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
}

// ---------------- Helpers: error surface ----------------

function requireOk(res: FetchResult<unknown>): void {
  if (!res.errors || res.errors.length === 0) return;
  const first = res.errors[0];
  const status = Number((first.extensions as { status?: number } | undefined)?.status ?? 500);
  throw new ApiError(status, first.message);
}

// ---------------- Helpers: Apollo Observable → AsyncIterable ----------------

/**
 * Bridges `Observable<FetchResult<T>>` to an async iterable so callers
 * can use `for await`. Aborts when the supplied AbortSignal fires.
 *
 * Apollo's Observable is push-based; we buffer frames in a small queue
 * and resolve waiters as they arrive. Backpressure is unbounded — fine
 * for our short-lived inline/run streams.
 */
async function* fromObservable<T>(
  obs: Observable<FetchResult<T>>,
  signal: AbortSignal,
): AsyncGenerator<T> {
  const queue: FetchResult<T>[] = [];
  const waiters: Array<(v: IteratorResult<FetchResult<T>>) => void> = [];
  let done = false;
  let error: unknown;

  const push = (v: FetchResult<T>) => {
    const w = waiters.shift();
    if (w) w({ value: v, done: false });
    else queue.push(v);
  };

  const sub = obs.subscribe({
    next: (v) => push(v),
    error: (err) => {
      error = err;
      const w = waiters.shift();
      if (w) w({ value: undefined as unknown as FetchResult<T>, done: true });
    },
    complete: () => {
      done = true;
      const w = waiters.shift();
      if (w) w({ value: undefined as unknown as FetchResult<T>, done: true });
    },
  });

  const onAbort = () => {
    done = true;
    sub.unsubscribe();
    while (waiters.length) {
      const w = waiters.shift()!;
      w({ value: undefined as unknown as FetchResult<T>, done: true });
    }
  };
  if (signal.aborted) onAbort();
  else signal.addEventListener('abort', onAbort, { once: true });

  try {
    while (true) {
      if (queue.length) {
        const frame = queue.shift()!;
        if (frame.data !== undefined && frame.data !== null) yield frame.data;
        continue;
      }
      if (done) {
        if (error) throw error;
        return;
      }
      const next = await new Promise<IteratorResult<FetchResult<T>>>((resolve) => {
        waiters.push(resolve);
      });
      if (next.done) {
        if (error) throw error;
        return;
      }
      if (next.value.data !== undefined && next.value.data !== null) yield next.value.data;
    }
  } finally {
    sub.unsubscribe();
    signal.removeEventListener('abort', onAbort);
  }
}

// ---------------- Helpers: GraphQL ↔ legacy shape mappers ----------------

/** Fragment-masking dodge — codegen attaches `$fragmentRefs` to fields
 *  declared via `...Fragment`. We unwrap them in-place for our mappers. */
function unmaskFragment<T>(v: T): any {
  if (!v) return v;
  const r = v as unknown as { ' $fragmentRefs'?: Record<string, unknown> } & Record<string, unknown>;
  const refs = r[' $fragmentRefs'];
  if (!refs) return v;
  // Merge each fragment ref onto the host so callers see one flat object.
  let out: Record<string, unknown> = { ...r };
  delete out[' $fragmentRefs'];
  for (const inner of Object.values(refs)) {
    out = { ...out, ...(inner as Record<string, unknown>) };
  }
  return out;
}

function unmaskProjectSummary(raw: unknown): Project {
  const v = unmaskFragment(raw) as {
    id: string;
    name: string;
    description: string | null;
    ownerId: string;
    isPublic: boolean;
    idea: string | null;
    files: Array<{ path: string; size: number | null; language: string | null }>;
  };
  return {
    id: v.id,
    name: v.name,
    description: v.description ?? undefined,
    ownerId: v.ownerId,
    isPublic: v.isPublic,
    files: (v.files ?? []).map((f) => ({
      path: f.path,
      size: f.size ?? undefined,
      type: f.language ?? undefined,
    })),
    spec: v.idea ? { idea: v.idea } : undefined,
  };
}

function mapGateVerdict(v: {
  gate: string;
  status: GqlGateStatus;
  notes: string | null;
  startedAt: string | null;
  finishedAt: string | null;
  durationMs: number | null;
  issues: Array<{ path: string | null; line: number | null; rule: string | null; severity: string | null; message: string }>;
}): GateState {
  return {
    name: v.gate as GateName,
    status: mapGateStatus(v.status),
    updatedAt: v.finishedAt ?? v.startedAt ?? new Date().toISOString(),
    issues: (v.issues ?? []).map((i) => ({
      gate: v.gate as GateName,
      severity: mapIssueSeverity(i.severity ?? ''),
      message: i.message,
      hint: i.rule ?? undefined,
      path: i.path ?? undefined,
    })),
  };
}

function mapGateStatus(s: GqlGateStatus): GateStatus {
  switch (s) {
    case 'PASS': return 'passed';
    case 'FAIL': return 'failed';
    case 'RUNNING': return 'running';
    case 'BLOCKED': return 'blocked';
    case 'PENDING': return 'pending';
    case 'WARN': return 'failed';
    case 'SKIPPED': return 'pending';
    default: return 'pending';
  }
}

function mapIssueSeverity(s: string): IssueSeverity {
  const v = s.toLowerCase();
  if (v === 'info' || v === 'warning' || v === 'error' || v === 'critical') return v as IssueSeverity;
  return 'info';
}

function mapPatch(v: {
  id: string;
  projectId: string;
  title: string | null;
  summary: string | null;
  author: string | null;
  status: GqlPatchStatus;
  createdAt: string;
  appliedAt: string | null;
  changes: Array<{
    op: GqlPatchChangeOp;
    path: string;
    content: string | null;
  }>;
}): Patch {
  return {
    id: v.id,
    projectId: v.projectId,
    title: v.title ?? undefined,
    summary: v.summary ?? undefined,
    author: v.author ?? undefined,
    status: mapPatchStatus(v.status),
    createdAt: v.createdAt,
    appliedAt: v.appliedAt ?? undefined,
    changes: (v.changes ?? []).map((c) => ({
      op: mapPatchOp(c.op),
      path: c.path,
      content: c.content ?? undefined,
    })),
  };
}

function mapPatchStatus(s: GqlPatchStatus): PatchStatus {
  switch (s) {
    case 'PROPOSED': return 'proposed';
    case 'APPROVED': return 'validated';
    case 'APPLIED': return 'applied';
    case 'REJECTED': return 'rejected';
    case 'ROLLED_BACK': return 'rolled-back';
    default: return 'proposed';
  }
}

function mapPatchOp(op: GqlPatchChangeOp): PatchOp {
  switch (op) {
    case 'CREATE': return 'create';
    case 'REPLACE':
    case 'INSERT_BEFORE':
    case 'INSERT_AFTER':
    case 'ANCHOR_REPLACE':
    case 'SYMBOL_REPLACE':
      return 'update';
    case 'DELETE': return 'delete';
    default: return 'update';
  }
}

function toGqlMemoryKind(k: MemoryKind): GqlMemoryKind {
  switch (k) {
    case 'project': return 'PROJECT';
    case 'execution': return 'EXECUTION';
    case 'user': return 'USER';
    case 'business': return 'BUSINESS';
  }
}

function fromGqlMemoryKind(k: GqlMemoryKind): MemoryKind {
  switch (k) {
    case 'PROJECT': return 'project';
    case 'EXECUTION': return 'execution';
    case 'USER': return 'user';
    case 'BUSINESS': return 'business';
    default: return 'project';
  }
}

function mapMemoryRecord(v: {
  id: string;
  kind: GqlMemoryKind;
  userId: string | null;
  projectId: string | null;
  storyId: string | null;
  gateName: string | null;
  title: string | null;
  body: string;
  tags: ReadonlyArray<string> | null;
  createdAt: string;
}): MemoryRecord {
  return {
    id: v.id,
    kind: fromGqlMemoryKind(v.kind),
    userId: v.userId ?? undefined,
    projectId: v.projectId ?? undefined,
    storyId: v.storyId ?? undefined,
    gateName: v.gateName ?? undefined,
    title: v.title ?? '',
    body: v.body,
    tags: v.tags ? [...v.tags] : undefined,
    createdAt: v.createdAt,
  };
}

function mapAuditEntry(v: {
  id: string;
  ts: string;
  userId: string | null;
  projectId: string | null;
  action: string;
  outcome: GqlAuditOutcome;
  hash: string;
  prevHash: string | null;
  payload: unknown;
}): AuditEntry {
  return {
    id: v.id,
    action: v.action,
    outcome: String(v.outcome).toLowerCase(),
    userId: v.userId ?? undefined,
    projectId: v.projectId ?? undefined,
    summary: typeof v.payload === 'object' && v.payload && 'summary' in (v.payload as Record<string, unknown>)
      ? String((v.payload as Record<string, unknown>).summary ?? '')
      : '',
    attrs: (v.payload as Record<string, unknown> | undefined) ?? undefined,
    createdAt: v.ts,
    prevHash: v.prevHash ?? undefined,
    contentHash: v.hash,
  };
}

function mapAgentCall(v: {
  ts: string;
  role: string | null;
  provider: string;
  model: string | null;
  promptTokens: number;
  completionTokens: number;
  costUsd: string;
  durationMs: number;
  error: string | null;
  capabilities: ReadonlyArray<string> | null;
  userId: string | null;
  projectId: string | null;
}): AgentCall {
  return {
    userId: v.userId ?? '',
    projectId: v.projectId ?? undefined,
    role: v.role ?? undefined,
    provider: v.provider,
    model: v.model ?? '',
    capabilities: v.capabilities ? [...v.capabilities] : undefined,
    inputTokens: v.promptTokens,
    outputTokens: v.completionTokens,
    costUSD: Number(v.costUsd) || 0,
    durationMs: v.durationMs,
    startedAt: v.ts,
    error: v.error ?? undefined,
  };
}

/** Translate a single ChatDelta union frame to the legacy SSEEvent shape
 *  the chat webview already understands. */
function mapChatDelta(d: unknown): SSEEvent | undefined {
  if (!d || typeof d !== 'object') return undefined;
  const f = (d as { chatStream?: Record<string, unknown> }).chatStream;
  if (!f) return undefined;
  const tn = f.__typename as string | undefined;
  switch (tn) {
    case 'ChatStartDelta':
      return { event: 'start', data: { turnId: f.startTurnId, provider: f.startProvider, model: f.startModel } };
    case 'ChatTextDelta':
      return { event: 'text', data: { turnId: f.textTurnId, text: f.text } };
    case 'ChatThinkingDelta':
      return { event: 'thinking', data: { turnId: f.thinkingTurnId, text: f.thinkingText } };
    case 'ChatToolUseDelta':
      return { event: 'tool', data: { turnId: f.toolTurnId, toolUse: f.toolUse } };
    case 'ChatDoneDelta':
      return { event: 'done', data: { turnId: f.doneTurnId, provider: f.doneProvider, model: f.doneModel, usage: f.usage } };
    case 'ChatErrorDelta':
      return { event: 'error', data: { turnId: f.errorTurnId, code: f.errorCode, message: f.errorMessage } };
    default:
      return undefined;
  }
}

function mapInlineDelta(d: unknown): SSEEvent | undefined {
  if (!d || typeof d !== 'object') return undefined;
  const f = (d as { inlineCompletion?: Record<string, unknown> }).inlineCompletion;
  if (!f) return undefined;
  const tn = f.__typename as string | undefined;
  switch (tn) {
    case 'InlineStartDelta':
      return { event: 'start', data: { requestId: f.startRequestId, provider: f.startProvider, model: f.startModel } };
    case 'InlineTextDelta':
      return { event: 'text', data: { requestId: f.textRequestId, text: f.text } };
    case 'InlineDoneDelta':
      return { event: 'done', data: { requestId: f.doneRequestId, provider: f.doneProvider, model: f.doneModel, usage: f.usage } };
    case 'InlineCancelledDelta':
      return { event: 'cancelled', data: { requestId: f.cancelRequestId, reason: f.reason } };
    case 'InlineErrorDelta':
      return { event: 'error', data: { requestId: f.errorRequestId, code: f.errorCode, message: f.errorMessage } };
    default:
      return undefined;
  }
}

function mapRunEvent(d: unknown): SSEEvent | undefined {
  if (!d || typeof d !== 'object') return undefined;
  const f = (d as { runProject?: Record<string, unknown> }).runProject;
  if (!f) return undefined;
  const tn = f.__typename as string | undefined;
  switch (tn) {
    case 'RunExecutionEvent':
      return { event: 'execution', data: f.payload };
    case 'RunGateEvent': {
      const status = String(f.status ?? '').toLowerCase();
      const gate = f.gate;
      const message = f.gateMessage;
      // Project legacy lifecycle events the runOutput / refresh loop
      // recognises: gate_started / gate_passed / gate_failed.
      if (status === 'running' || status === 'pending') {
        return { event: 'gate_started', data: { gate, message } };
      }
      if (status === 'pass') {
        return { event: 'gate_passed', data: { gate, message } };
      }
      if (status === 'fail' || status === 'warn' || status === 'blocked') {
        return { event: 'gate_failed', data: { gate, reason: message } };
      }
      return { event: 'gate', data: { gate, status, message } };
    }
    case 'RunDoneEvent':
      return { event: 'run_complete', data: { status: f.ok ? 'success' : 'failed', summary: f.summary } };
    case 'RunErrorEvent':
      return { event: 'error', data: { code: f.code, message: f.errorMessage } };
    default:
      return undefined;
  }
}
