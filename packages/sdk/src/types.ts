// Types mirror the Go API shapes (orchestrator + runtime). They are the
// stable contract between server and client — bump the SDK major version
// when a breaking change lands on the wire.

// ---------- Finisher gates ---------------------------------------------------

export type GateName =
  | 'spec' | 'ux' | 'arch' | 'code' | 'lint' | 'test' | 'security' | 'budget' | 'deploy';

export type GateStatus =
  | 'pending' | 'running' | 'passed' | 'failed' | 'blocked' | 'repaired';

export type Severity = 'info' | 'warning' | 'error' | 'critical';

export interface Issue {
  gate: GateName;
  severity: Severity;
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

// ---------- Project ----------------------------------------------------------

export interface UserStory {
  id: string;
  as: string;
  iWant: string;
  soThat: string;
  acceptance: string[];
}

export interface EntityDef {
  name: string;
  fields: string[];
}

export interface StackDecision {
  frontend: string;
  backend: string;
  storage: string;
  auth: string;
}

export interface ProductSpec {
  idea: string;
  userStories?: UserStory[] | null;
  dataModel?: EntityDef[] | null;
  stack: StackDecision;
}

export interface GitHubLink {
  owner: string;
  repo: string;
  fullName: string;
  defaultBranch: string;
  htmlUrl: string;
}

export interface FileNode {
  path: string;
  type: string;
  size?: number;
  content?: string;
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

export interface VisualTarget {
  id: string;
  name?: string;
  routeHint?: string;
  viewportW: number;
  viewportH: number;
  imagePngBase64: string;
  tolerance?: number;
}

export interface Subproject {
  id: string;
  name: string;
  path: string;
  stack?: ProductSpec['stack'];
  role?: 'frontend' | 'backend' | 'worker' | 'mobile' | 'ml' | string;
  createdAt: string;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  status: string;
  ownerId?: string;
  spec: ProductSpec;
  files: FileNode[];
  gates: Record<GateName, GateState>;
  events: ExecutionEvent[];
  github?: GitHubLink;
  visualTargets?: VisualTarget[];
  subprojects?: Subproject[];
  createdAt: string;
  updatedAt: string;
}

// ---------- Budget -----------------------------------------------------------

export type PlanTier = 'free' | 'pro' | 'team' | 'enterprise';

export interface Plan {
  tier: PlanTier;
  name: string;
  monthlyPrice: string;
  costCapUSD: string;
  hardStop: boolean;
  allowList?: string[];
  blockList?: string[];
  stripeId?: string;
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

export interface UserBudget {
  userId: string;
  email?: string;
  tier: PlanTier;
  spent: string;
  entries: LedgerEntry[];
}

// ---------- Agents -----------------------------------------------------------

export type AgentRole =
  | 'planner' | 'uxer' | 'architect' | 'coder'
  | 'reviewer' | 'tester' | 'security' | 'deployer';

export type AgentCapability =
  | 'reasoning' | 'code' | 'json' | 'vision' | 'cheap' | 'fast'
  | 'private' | 'thinking' | 'tools' | 'cache';

export interface Agent {
  role: AgentRole;
  system: string;
  capabilities: AgentCapability[];
  enableThinking?: boolean;
}

// ---------- Auth -------------------------------------------------------------

export interface AuthUser {
  id: string;
  email: string;
  name?: string;
  plan?: PlanTier;
  createdAt?: string;
}

export interface AuthResponse {
  user: AuthUser;
  token: string;
}

// ---------- Brainstorm -------------------------------------------------------

export interface BrainstormPlan {
  mode: 'direct' | 'brainstorm' | 'debate' | 'research';
  roles: string[];
  rounds?: number;
  goal: string;
  reason: string;
}

export interface BrainstormProposal {
  role: string;
  provider: string;
  output: string;
  score: number;
  tokens: number;
  costUSD: number;
}

export interface BrainstormOutcome {
  plan: BrainstormPlan;
  outcome: {
    mode: string;
    winner?: string;
    synthesis: string;
    proposals?: BrainstormProposal[];
    totalCostUSD: number;
    startedAt: string;
    finishedAt: string;
  };
}

// ---------- Run report -------------------------------------------------------

export interface RunReport {
  projectId: string;
  iterations: number;
  gates: GateState[];
  completed: boolean;
  startedAt: string;
  finishedAt: string;
}

// ---------- Workspace runtime ------------------------------------------------

export type WorkspaceStatus = 'creating' | 'running' | 'stopped' | 'error';

export interface Workspace {
  id: string;
  userId: string;
  projectId?: string;
  status: WorkspaceStatus;
  driver: string;
  root: string;
  previewUrl?: string;
  ideUrl?: string;
  createdAt: string;
  updatedAt: string;
}

export interface RuntimeFileEntry {
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

// ---------- Workspace preview (live dev server reverse-proxy) ---------------

export interface DetectedPort {
  port: number;
  source?: string;        // "exec-stdout" | "exec-stderr" | "manual"
  firstSeen: string;
  lastSeen: string;
  previewPath: string;    // e.g. "/preview/ws-abcd1234/3000/"
  allowed: boolean;       // false → port not on the runtime allowlist
}

export interface PreviewTokenResponse {
  /** Public path including the signed `?t=...` token. */
  url: string;
  /** Path without the token, in case the caller wants to compose its own. */
  path: string;
  token: string;
  expiresAt: string;
}

export interface PatchChange {
  path: string;
  kind: 'created' | 'modified' | 'deleted';
  bytes: number;
}

export interface ApplyPatchResponse {
  applied: PatchChange[];
  count: number;
}

// ---------- Streaming chat ---------------------------------------------------

export type ChatDelta =
  | { kind: 'turn'; id: string; role: string }
  | { kind: 'start'; provider: string; model: string; turn: string }
  | { kind: 'text'; text: string; turn: string }
  | { kind: 'thinking'; text: string; turn: string }
  | { kind: 'tool_use'; data: unknown }
  | { kind: 'done'; turn: string; provider: string; model: string; usage?: unknown }
  | { kind: 'error'; error: string };
