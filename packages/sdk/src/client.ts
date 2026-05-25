/**
 * `@ironflyer/sdk` — class-shaped public client for the Ironflyer
 * orchestrator GraphQL API.
 *
 * Two transports work together:
 *
 *   - `graphql-request` for queries + mutations (typed via the codegen
 *     SDK in `./gql`).
 *   - `graphql-ws` for subscriptions, which graphql-request does not
 *     handle. Each subscription method returns an `AsyncIterable` so
 *     callers can drive it with a plain `for await`.
 *
 * The constructor takes an `endpoint` (HTTP) plus an optional bearer
 * `token`. `setToken(...)` swaps the token at runtime so a refresh
 * loop can re-authenticate without rebuilding the client. The WS
 * connection uses `connectionParams: () => ({...})` so token updates
 * take effect on the next reconnect.
 *
 * The class wraps the generated graphql-request SDK and exposes one
 * method per public operation. Subscription methods are hand-written
 * shells around the codegen-emitted typed `Document` + result types so
 * type safety carries through to the consumer.
 */

import { GraphQLClient } from 'graphql-request';
import {
  createClient as createWSClient,
  type Client as WSClient,
} from 'graphql-ws';
import type { DocumentNode } from 'graphql';

import {
  getSdk,
  type Sdk,
  type SignInInput,
  type SignUpInput,
  type CreateProjectInput,
  type UpdateProjectInput,
  type RerunGateInput,
  type ProposePatchInput,
  type StartCheckoutInput,
  type CancelSubscriptionInput,
  type AddMemoryInput,
  type MemoryQueryInput,
  type AuditQueryInput,
  type CreateWebhookInput,
  type StartDeployInput,
  type CreateChatInput,
  type ForkChatInput,
  type ChatInput,
  type InlineInput,
  RunProjectDocument,
  ChatStreamDocument,
  InlineCompletionDocument,
  DeployStreamDocument,
  WorkspacePtyDocument,
  CostStreamDocument,
  type RunProjectSubscription,
  type ChatStreamSubscription,
  type InlineCompletionSubscription,
  type DeployStreamSubscription,
  type WorkspacePtySubscription,
  type CostStreamSubscription,
} from './gql';

/** Minimal WebSocket constructor shape compatible with the `ws` package. */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WebSocketImpl = any;

export interface IronflyerOptions {
  /**
   * HTTP endpoint for the orchestrator's GraphQL API.
   * Example: `https://api.ironflyer.dev/graphql`.
   * Trailing `/graphql` is added if missing.
   */
  endpoint: string;
  /**
   * Bearer JWT. Optional — public ops (signIn, plans) work anonymously.
   * Swap with `setToken(...)` after sign-in.
   */
  token?: string;
  /**
   * WebSocket endpoint for subscriptions. Defaults to the HTTP endpoint
   * with `http(s)` → `ws(s)`.
   */
  wsEndpoint?: string;
  /**
   * Custom `fetch` implementation. Useful for proxies, test harnesses,
   * or runtimes without global fetch.
   */
  fetch?: typeof globalThis.fetch;
  /**
   * WebSocket constructor. Required on Node < 22 — pass `import('ws')`.
   * In browsers and modern Node, native `WebSocket` is used automatically.
   */
  webSocketImpl?: WebSocketImpl;
  /**
   * Extra headers attached to every HTTP request (e.g. tracing).
   */
  headers?: Record<string, string>;
}

/**
 * Error thrown when an operation fails. Wraps the raw GraphQL response
 * so callers can dig into `extensions` / `path` if they need to.
 */
export class IronflyerError extends Error {
  public override readonly cause: unknown;
  constructor(message: string, cause: unknown) {
    super(message);
    this.name = 'IronflyerError';
    this.cause = cause;
  }
}

function normalizeEndpoint(endpoint: string): string {
  const trimmed = endpoint.replace(/\/+$/, '');
  return trimmed.endsWith('/graphql') ? trimmed : `${trimmed}/graphql`;
}

function deriveWsEndpoint(http: string): string {
  return http.replace(/^http/, 'ws');
}

/**
 * Public Ironflyer client. Construct once per session.
 *
 * ```ts
 * const ifr = new Ironflyer({ endpoint: 'https://api.ironflyer.dev' });
 * const { token, user } = await ifr.signIn({ email, password });
 * ifr.setToken(token);
 * const projects = await ifr.projects();
 * for await (const evt of ifr.runProject(projects[0].id)) {
 *   console.log(evt);
 * }
 * ifr.dispose();
 * ```
 */
export class Ironflyer {
  private readonly httpEndpoint: string;
  private readonly wsEndpoint: string;
  private readonly webSocketImpl: WebSocketImpl | undefined;
  private readonly extraHeaders: Record<string, string>;
  private readonly client: GraphQLClient;
  private readonly sdk: Sdk;
  private ws: WSClient | null = null;
  private token: string | undefined;
  private disposed = false;

  constructor(opts: IronflyerOptions) {
    if (!opts || typeof opts.endpoint !== 'string' || opts.endpoint.length === 0) {
      throw new IronflyerError('Ironflyer: `endpoint` is required', null);
    }
    this.httpEndpoint = normalizeEndpoint(opts.endpoint);
    this.wsEndpoint = opts.wsEndpoint
      ? normalizeEndpoint(opts.wsEndpoint).replace(/^http/, 'ws')
      : deriveWsEndpoint(this.httpEndpoint);
    this.webSocketImpl = opts.webSocketImpl;
    this.extraHeaders = { ...(opts.headers ?? {}) };
    this.token = opts.token;

    const requestInit: { fetch?: typeof globalThis.fetch } = {};
    if (opts.fetch) requestInit.fetch = opts.fetch;

    this.client = new GraphQLClient(this.httpEndpoint, {
      ...requestInit,
      headers: () => this.authHeaders(),
    });
    this.sdk = getSdk(this.client);
  }

  /** Update or clear the bearer token. Affects subsequent HTTP requests + WS reconnects. */
  setToken(token: string | undefined): void {
    this.token = token;
  }

  /** Current bearer token, if any. */
  getToken(): string | undefined {
    return this.token;
  }

  /** Close the WebSocket connection (if open). The HTTP client has no state to clean up. */
  dispose(): void {
    this.disposed = true;
    if (this.ws) {
      try {
        void this.ws.dispose();
      } catch {
        // ignore — best effort.
      }
      this.ws = null;
    }
  }

  // ── HTTP request helper ─────────────────────────────────────────

  private authHeaders(): Record<string, string> {
    const headers: Record<string, string> = { ...this.extraHeaders };
    if (this.token) headers.authorization = `Bearer ${this.token}`;
    return headers;
  }

  private async call<T>(label: string, fn: () => Promise<T>): Promise<T> {
    try {
      return await fn();
    } catch (err) {
      const message =
        err instanceof Error
          ? `Ironflyer.${label} failed: ${err.message}`
          : `Ironflyer.${label} failed`;
      throw new IronflyerError(message, err);
    }
  }

  // ── Auth ────────────────────────────────────────────────────────

  async me() {
    return (await this.call('me', () => this.sdk.Me())).me;
  }

  async signIn(input: SignInInput) {
    const data = await this.call('signIn', () => this.sdk.SignIn({ input }));
    // Convenience: auto-attach the token to this client so subsequent
    // calls authenticate without an extra `setToken` step.
    if (data.signIn?.token) this.setToken(data.signIn.token);
    return data.signIn;
  }

  async signUp(input: SignUpInput) {
    const data = await this.call('signUp', () => this.sdk.SignUp({ input }));
    if (data.signUp?.token) this.setToken(data.signUp.token);
    return data.signUp;
  }

  async signOut() {
    const data = await this.call('signOut', () => this.sdk.SignOut());
    this.setToken(undefined);
    return data.signOut;
  }

  // ── Projects ────────────────────────────────────────────────────

  async projects() {
    return (await this.call('projects', () => this.sdk.Projects())).projects;
  }

  async project(id: string) {
    return (await this.call('project', () => this.sdk.Project({ id }))).project;
  }

  async projectFiles(id: string) {
    return (await this.call('projectFiles', () => this.sdk.ProjectFiles({ id }))).projectFiles;
  }

  async projectGraph(id: string) {
    return (await this.call('projectGraph', () => this.sdk.ProjectGraph({ id }))).projectGraph;
  }

  async projectSnapshot(id: string) {
    return (await this.call('projectSnapshot', () => this.sdk.ProjectSnapshot({ id }))).projectSnapshot;
  }

  async searchProjectCode(id: string, q: string, opts?: { k?: number; maxKb?: number }) {
    return (
      await this.call('searchProjectCode', () =>
        this.sdk.SearchProjectCode({ id, q, k: opts?.k ?? null, maxKb: opts?.maxKb ?? null }),
      )
    ).searchProjectCode;
  }

  async createProject(input: CreateProjectInput) {
    return (await this.call('createProject', () => this.sdk.CreateProject({ input }))).createProject;
  }

  async updateProject(id: string, input: UpdateProjectInput) {
    return (await this.call('updateProject', () => this.sdk.UpdateProject({ id, input }))).updateProject;
  }

  async deleteProject(id: string) {
    return (await this.call('deleteProject', () => this.sdk.DeleteProject({ id }))).deleteProject;
  }

  async bulkDeleteProjects(ids: string[]) {
    return (await this.call('bulkDeleteProjects', () => this.sdk.BulkDeleteProjects({ ids }))).bulkDeleteProjects;
  }

  // ── Gates ───────────────────────────────────────────────────────

  async gates(projectId: string, sub?: string) {
    return (await this.call('gates', () => this.sdk.Gates({ projectId, sub: sub ?? null }))).gates;
  }

  async rerunGate(input: RerunGateInput) {
    return (await this.call('rerunGate', () => this.sdk.RerunGate({ input }))).rerunGate;
  }

  async runFinisher(id: string) {
    return (await this.call('runFinisher', () => this.sdk.RunFinisher({ id }))).runFinisher;
  }

  // ── Patches ─────────────────────────────────────────────────────

  async patches(projectId: string) {
    return (await this.call('patches', () => this.sdk.Patches({ projectId }))).patches;
  }

  async proposePatch(input: ProposePatchInput) {
    return (await this.call('proposePatch', () => this.sdk.ProposePatch({ input }))).proposePatch;
  }

  async applyPatch(patchId: string) {
    return (await this.call('applyPatch', () => this.sdk.ApplyPatch({ patchId }))).applyPatch;
  }

  async rollbackPatch(patchId: string) {
    return (await this.call('rollbackPatch', () => this.sdk.RollbackPatch({ patchId }))).rollbackPatch;
  }

  // ── Budget + billing ────────────────────────────────────────────

  async myBudget() {
    return (await this.call('myBudget', () => this.sdk.MyBudget())).myBudget;
  }

  async plans() {
    return (await this.call('plans', () => this.sdk.Plans())).plans;
  }

  async rates() {
    return (await this.call('rates', () => this.sdk.Rates())).rates;
  }

  async vault() {
    return (await this.call('vault', () => this.sdk.Vault())).vault;
  }

  async mySubscription() {
    return (await this.call('mySubscription', () => this.sdk.MySubscription())).mySubscription;
  }

  async startCheckout(input: StartCheckoutInput) {
    return (await this.call('startCheckout', () => this.sdk.StartCheckout({ input }))).startCheckout;
  }

  async cancelSubscription(input: CancelSubscriptionInput) {
    return (await this.call('cancelSubscription', () => this.sdk.CancelSubscription({ input }))).cancelSubscription;
  }

  // ── Memory ──────────────────────────────────────────────────────

  async memory(query?: MemoryQueryInput) {
    return (await this.call('memory', () => this.sdk.Memory({ query: query ?? null }))).memory;
  }

  async addMemory(input: AddMemoryInput) {
    return (await this.call('addMemory', () => this.sdk.AddMemory({ input }))).addMemory;
  }

  async deleteMemory(id: string) {
    return (await this.call('deleteMemory', () => this.sdk.DeleteMemory({ id }))).deleteMemory;
  }

  // ── Audit ───────────────────────────────────────────────────────

  async audit(query?: AuditQueryInput) {
    return (await this.call('audit', () => this.sdk.Audit({ query: query ?? null }))).audit;
  }

  async verifyAudit() {
    return (await this.call('verifyAudit', () => this.sdk.VerifyAudit())).verifyAudit;
  }

  // ── Webhooks ────────────────────────────────────────────────────

  async webhooks() {
    return (await this.call('webhooks', () => this.sdk.Webhooks())).webhooks;
  }

  async createWebhook(input: CreateWebhookInput) {
    return (await this.call('createWebhook', () => this.sdk.CreateWebhook({ input }))).createWebhook;
  }

  async deleteWebhook(id: string) {
    return (await this.call('deleteWebhook', () => this.sdk.DeleteWebhook({ id }))).deleteWebhook;
  }

  async testWebhook(id: string) {
    return (await this.call('testWebhook', () => this.sdk.TestWebhook({ id }))).testWebhook;
  }

  // ── Deploys ─────────────────────────────────────────────────────

  async deploys(projectId: string) {
    return (await this.call('deploys', () => this.sdk.Deploys({ projectId }))).deploys;
  }

  async startDeploy(input: StartDeployInput) {
    return (await this.call('startDeploy', () => this.sdk.StartDeploy({ input }))).startDeploy;
  }

  // ── Inline completion ───────────────────────────────────────────

  async acceptInlineCompletion(requestId: string) {
    return (
      await this.call('acceptInlineCompletion', () =>
        this.sdk.AcceptInlineCompletion({ requestId }),
      )
    ).acceptInlineCompletion;
  }

  // ── Chats ───────────────────────────────────────────────────────

  async chats(projectId: string) {
    return (await this.call('chats', () => this.sdk.Chats({ projectId }))).chats;
  }

  async chatMessages(chatId: string) {
    return (await this.call('chatMessages', () => this.sdk.ChatMessages({ chatId }))).chatMessages;
  }

  async createChat(input: CreateChatInput) {
    return (await this.call('createChat', () => this.sdk.CreateChat({ input }))).createChat;
  }

  async forkChat(input: ForkChatInput) {
    return (await this.call('forkChat', () => this.sdk.ForkChat({ input }))).forkChat;
  }

  // ── Workspaces ──────────────────────────────────────────────────

  async workspaces(projectId: string) {
    return (await this.call('workspaces', () => this.sdk.Workspaces({ projectId }))).workspaces;
  }

  async workspace(id: string) {
    return (await this.call('workspace', () => this.sdk.Workspace({ id }))).workspace;
  }

  async workspaceFiles(workspaceId: string, path?: string) {
    return (
      await this.call('workspaceFiles', () =>
        this.sdk.WorkspaceFiles({ workspaceId, path: path ?? null }),
      )
    ).workspaceFiles;
  }

  async workspaceFile(workspaceId: string, path: string) {
    return (await this.call('workspaceFile', () => this.sdk.WorkspaceFile({ workspaceId, path }))).workspaceFile;
  }

  async createWorkspace(projectId: string, driver?: string) {
    return (
      await this.call('createWorkspace', () =>
        this.sdk.CreateWorkspace({ projectId, driver: driver ?? null }),
      )
    ).createWorkspace;
  }

  async startWorkspace(id: string) {
    return (await this.call('startWorkspace', () => this.sdk.StartWorkspace({ id }))).startWorkspace;
  }

  async stopWorkspace(id: string) {
    return (await this.call('stopWorkspace', () => this.sdk.StopWorkspace({ id }))).stopWorkspace;
  }

  async writeWorkspaceFile(workspaceId: string, path: string, content: string) {
    return (
      await this.call('writeWorkspaceFile', () =>
        this.sdk.WriteWorkspaceFile({ workspaceId, path, content }),
      )
    ).writeWorkspaceFile;
  }

  async execInWorkspace(workspaceId: string, command: string, timeoutSec?: number) {
    return (
      await this.call('execInWorkspace', () =>
        this.sdk.ExecInWorkspace({
          workspaceId,
          command,
          timeoutSec: timeoutSec ?? null,
        }),
      )
    ).execInWorkspace;
  }

  // ── Subscriptions (AsyncIterable) ───────────────────────────────

  /** Live finisher run for a project. */
  runProject(projectId: string): AsyncIterable<RunProjectSubscription['runProject']> {
    return this.subscribe<RunProjectSubscription>(RunProjectDocument as DocumentNode, { projectId }, (d) => d.runProject);
  }

  /** Live chat stream (replaces the legacy SSE chat endpoint). */
  chatStream(projectId: string, input: ChatInput): AsyncIterable<ChatStreamSubscription['chatStream']> {
    return this.subscribe<ChatStreamSubscription>(ChatStreamDocument as DocumentNode, { projectId, input }, (d) => d.chatStream);
  }

  /** Cursor-style inline completion stream. */
  inlineCompletion(input: InlineInput): AsyncIterable<InlineCompletionSubscription['inlineCompletion']> {
    return this.subscribe<InlineCompletionSubscription>(InlineCompletionDocument as DocumentNode, { input }, (d) => d.inlineCompletion);
  }

  /** Deploy progress events. */
  deployStream(deployId: string): AsyncIterable<DeployStreamSubscription['deployStream']> {
    return this.subscribe<DeployStreamSubscription>(DeployStreamDocument as DocumentNode, { deployId }, (d) => d.deployStream);
  }

  /** PTY output for a workspace. Use `execInWorkspace` for one-shot commands. */
  workspacePty(workspaceId: string): AsyncIterable<WorkspacePtySubscription['workspacePty']> {
    return this.subscribe<WorkspacePtySubscription>(WorkspacePtyDocument as DocumentNode, { workspaceId }, (d) => d.workspacePty);
  }

  /** Per-call cost deltas for the authenticated user. */
  costStream(): AsyncIterable<CostStreamSubscription['costStream']> {
    return this.subscribe<CostStreamSubscription>(CostStreamDocument as DocumentNode, {}, (d) => d.costStream);
  }

  // ── WS plumbing ─────────────────────────────────────────────────

  private ensureWS(): WSClient {
    if (this.disposed) {
      throw new IronflyerError('Ironflyer client has been disposed', null);
    }
    if (this.ws) return this.ws;

    const opts: Parameters<typeof createWSClient>[0] = {
      url: this.wsEndpoint,
      lazy: true,
      keepAlive: 30_000,
      retryAttempts: 5,
      connectionParams: () =>
        this.token ? { authorization: `Bearer ${this.token}` } : {},
    };
    if (this.webSocketImpl) {
      opts.webSocketImpl = this.webSocketImpl;
    }
    this.ws = createWSClient(opts);
    return this.ws;
  }

  /**
   * Lift a graphql-ws subscription into an AsyncIterable.
   *
   * Lifecycle:
   *   - Iterator pulls (`next`) hand back a buffered payload, or wait
   *     for the WS `next` callback to push one.
   *   - WS `complete` resolves the iterator with `done: true`.
   *   - WS `error` rejects pending pulls + closes the iterator. The
   *     thrown error is an `IronflyerError` wrapping the raw cause.
   *   - Iterator `return()` (e.g. `break` inside a `for await`) calls
   *     `unsubscribe()` on the WS subscription so the server stops
   *     producing.
   */
  private subscribe<TData>(
    query: DocumentNode,
    variables: Record<string, unknown>,
    pick: (data: TData) => unknown,
  ): AsyncIterable<never> {
    const client = this.ensureWS();
    type Pulled = unknown;

    // Buffer of payloads delivered before a consumer pulls; mirror
    // queue of pending pulls waiting for the next payload.
    const buffer: Pulled[] = [];
    type Pending = {
      resolve: (r: IteratorResult<Pulled>) => void;
      reject: (e: unknown) => void;
    };
    const pending: Pending[] = [];
    let closed = false;
    let closeError: unknown = null;

    const flushClose = () => {
      while (pending.length > 0) {
        const p = pending.shift();
        if (!p) break;
        if (closeError) p.reject(closeError);
        else p.resolve({ value: undefined, done: true });
      }
    };

    const unsubscribe = client.subscribe<TData>(
      { query: query as unknown as string, variables },
      {
        next: (msg) => {
          if (closed) return;
          if (msg.errors && msg.errors.length > 0) {
            closeError = new IronflyerError(
              `subscription error: ${msg.errors.map((e) => e.message).join('; ')}`,
              msg.errors,
            );
            closed = true;
            flushClose();
            return;
          }
          if (msg.data === null || msg.data === undefined) return;
          const value = pick(msg.data);
          if (pending.length > 0) {
            const p = pending.shift();
            p?.resolve({ value, done: false });
          } else {
            buffer.push(value);
          }
        },
        error: (err) => {
          closeError = new IronflyerError('subscription transport error', err);
          closed = true;
          flushClose();
        },
        complete: () => {
          closed = true;
          flushClose();
        },
      },
    );

    const iterator: AsyncIterator<Pulled> = {
      next: () =>
        new Promise<IteratorResult<Pulled>>((resolve, reject) => {
          if (buffer.length > 0) {
            const value = buffer.shift();
            resolve({ value, done: false });
            return;
          }
          if (closed) {
            if (closeError) reject(closeError);
            else resolve({ value: undefined, done: true });
            return;
          }
          pending.push({ resolve, reject });
        }),
      return: async () => {
        closed = true;
        try {
          unsubscribe();
        } catch {
          // ignore — best effort.
        }
        flushClose();
        return { value: undefined, done: true };
      },
      throw: async (err) => {
        closed = true;
        closeError = err;
        try {
          unsubscribe();
        } catch {
          // ignore.
        }
        flushClose();
        return { value: undefined, done: true };
      },
    };

    return {
      [Symbol.asyncIterator]: () => iterator as AsyncIterator<never>,
    };
  }
}
