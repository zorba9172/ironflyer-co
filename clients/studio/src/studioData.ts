// Mock studio state. No backend yet — clearly fixture data so the cockpit can
// be designed against real shapes. Wire to @ironflyer/sdk + @ironflyer/data later.

export type GateStatus = 'closed' | 'running' | 'open' | 'blocked' | 'unstarted';

export interface Finding {
  id: string;
  severity: 'info' | 'warning' | 'danger';
  text: string;
}

export interface Patch {
  id: string;
  title: string;
  state: 'proposed' | 'applied';
  lines: number;
}

export interface Gate {
  id: string;
  no: string;
  name: string;
  status: GateStatus;
  /** one-line reason the next transition is blocked, '' if none */
  blocking: string;
  level: number; // 0..1 completion of this gate (drives the channel meter)
  costShare: number; // 0..1 share of spend
  findings: Finding[];
  patches: Patch[];
}

export interface ArrangementBlock {
  gateId: string;
  label: string;
  /** start/end on a 0..100 timeline */
  start: number;
  end: number;
  state: GateStatus;
}

export interface ActivityEvent {
  id: string;
  ts: number;
  kind: 'gate' | 'patch' | 'profitguard' | 'deploy' | 'ledger';
  text: string;
}

export interface ProfitGuard {
  reservedUSD: number;
  expectedMarginPct: number;
  // allow = run; hold = needs ROI; block = would push wallet negative (402)
  verdict: 'allow' | 'hold' | 'block';
}

// --- AppSec (mirrors core/orchestrator/internal/appsec) -----------------
export type ScannerStatus = 'clean' | 'findings' | 'not_run';
export type Severity = 'critical' | 'high' | 'medium' | 'low';
export type FindingCategory = 'secret' | 'dependency' | 'code' | 'config' | 'policy';

export interface SecurityScanner {
  id: string;
  name: string;
  status: ScannerStatus;
  count: number;
  source: 'native' | 'oss';
}

export interface SecurityFinding {
  id: string;
  severity: Severity;
  category: FindingCategory;
  title: string;
  location: string;
  scanner: string;
}

// The PDP decision shape from the policy plane (deny by default).
export interface PolicyDecision {
  decisionId: string;
  effect: 'allow' | 'deny';
  risk: 'low' | 'medium' | 'high';
  reason: string;
  obligations: string[];
}

export interface SecurityState {
  riskScore: number; // 0..100 (higher = riskier)
  scanners: SecurityScanner[];
  findings: SecurityFinding[];
  policy: PolicyDecision;
  sbom: { format: 'CycloneDX'; components: number };
}

export interface StudioProject {
  id: string;
  /** orchestrator execution id this project maps to */
  executionId: string;
  name: string;
  source: string;
  completion: number; // 0..1 overall
  deploy: { status: 'none' | 'preview' | 'production' | 'failed'; url?: string };
  gates: Gate[];
  arrangement: ArrangementBlock[];
  meters: {
    walletUsed: number;
    walletBudget: number;
    marginPct: number;
    throughput: number; // runs/min
  };
  profitGuard: ProfitGuard;
  activity: ActivityEvent[];
  security: SecurityState;
}

const SCANNER_NAMES: { id: string; name: string; source: 'native' | 'oss' }[] = [
  { id: 'secrets', name: 'Secrets', source: 'native' },
  { id: 'deps', name: 'Dependencies', source: 'native' },
  { id: 'sbom', name: 'SBOM (CycloneDX)', source: 'native' },
  { id: 'sast', name: 'SAST (Semgrep)', source: 'oss' },
  { id: 'containers', name: 'Containers', source: 'native' },
  { id: 'compose', name: 'Compose', source: 'native' },
  { id: 'actions', name: 'GitHub Actions', source: 'native' },
  { id: 'npm', name: 'npm scripts', source: 'native' },
];

export const severityRank: Record<Severity, number> = { critical: 0, high: 1, medium: 2, low: 3 };
export const categoryLabel: Record<FindingCategory, string> = {
  secret: 'Secret', dependency: 'Dependency', code: 'Code', config: 'Config', policy: 'Policy',
};

export const mockProject: StudioProject = {
  id: 'p_001',
  executionId: 'exec_8f3a91',
  name: 'Northwind Checkout',
  source: 'imported from lovable.dev',
  completion: 0.62,
  deploy: { status: 'preview', url: 'northwind.preview.ironflyer.app' },
  meters: { walletUsed: 18.4, walletBudget: 50, marginPct: 64, throughput: 3.2 },
  profitGuard: { reservedUSD: 2.4, expectedMarginPct: 64, verdict: 'allow' },
  activity: [
    { id: 'e1', ts: Date.now() - 9000, kind: 'profitguard', text: 'ProfitGuard: allow — reserved $2.40, expected margin 64%' },
    { id: 'e2', ts: Date.now() - 24000, kind: 'patch', text: 'Payments agent proposed patch: verify Stripe-Signature header' },
    { id: 'e3', ts: Date.now() - 38000, kind: 'gate', text: 'Data gate → running: applying migration 0007' },
    { id: 'e4', ts: Date.now() - 61000, kind: 'gate', text: 'Security gate → blocked: API key found in src/config.ts' },
    { id: 'e5', ts: Date.now() - 90000, kind: 'ledger', text: 'Ledger: debited $0.31 for sandbox minutes' },
    { id: 'e6', ts: Date.now() - 120000, kind: 'gate', text: 'Identity gate → closed' },
  ],
  security: {
    riskScore: 41,
    sbom: { format: 'CycloneDX', components: 47 },
    scanners: [
      { id: 'secrets', name: 'Secrets', status: 'findings', count: 1, source: 'native' },
      { id: 'deps', name: 'Dependencies', status: 'findings', count: 2, source: 'native' },
      { id: 'sbom', name: 'SBOM (CycloneDX)', status: 'clean', count: 47, source: 'native' },
      { id: 'sast', name: 'SAST (Semgrep)', status: 'findings', count: 1, source: 'oss' },
      { id: 'containers', name: 'Containers', status: 'findings', count: 1, source: 'native' },
      { id: 'compose', name: 'Compose', status: 'clean', count: 0, source: 'native' },
      { id: 'actions', name: 'GitHub Actions', status: 'clean', count: 0, source: 'native' },
      { id: 'npm', name: 'npm scripts', status: 'clean', count: 0, source: 'native' },
    ],
    findings: [
      { id: 's1', severity: 'critical', category: 'secret', title: 'Stripe secret key committed in source', location: 'src/config.ts:14', scanner: 'Secrets' },
      { id: 's2', severity: 'high', category: 'code', title: 'Unsigned webhook handler accepts arbitrary events', location: 'api/webhooks/stripe.ts:8', scanner: 'Semgrep' },
      { id: 's3', severity: 'high', category: 'dependency', title: 'lodash 4.17.19 — prototype pollution (CVE)', location: 'package-lock.json', scanner: 'OSV' },
      { id: 's4', severity: 'medium', category: 'dependency', title: 'axios 0.21.1 — SSRF advisory', location: 'package-lock.json', scanner: 'npm audit' },
      { id: 's5', severity: 'medium', category: 'config', title: 'Dockerfile runs as root user', location: 'Dockerfile:1', scanner: 'Containers' },
    ],
    policy: {
      decisionId: 'pdec_8f3a91',
      effect: 'deny',
      risk: 'high',
      reason: 'secret_in_source_blocks_deploy',
      obligations: ['rotate_secret', 'require_deploy_approval_id', 'audit.high_risk_allow', 'redact_model_context'],
    },
  },
  gates: [
    {
      id: 'identity', no: '01', name: 'Identity', status: 'closed', blocking: '', level: 1, costShare: 0.12,
      findings: [{ id: 'f1', severity: 'info', text: 'Sessions + roles wired to orchestrator auth.' }],
      patches: [{ id: 'pa1', title: 'Add owner check to /projects', state: 'applied', lines: 24 }],
    },
    {
      id: 'money', no: '02', name: 'Money', status: 'open', blocking: 'Stripe webhook signature not verified', level: 0.45, costShare: 0.28,
      findings: [
        { id: 'f2', severity: 'danger', text: 'Webhook handler accepts unsigned events.' },
        { id: 'f3', severity: 'warning', text: 'No reconciliation job for failed charges.' },
      ],
      patches: [{ id: 'pa2', title: 'Verify Stripe-Signature header', state: 'proposed', lines: 41 }],
    },
    {
      id: 'data', no: '03', name: 'Data', status: 'running', blocking: 'migration 0007 pending apply', level: 0.7, costShare: 0.18,
      findings: [{ id: 'f4', severity: 'warning', text: 'orders table has no created_at index.' }],
      patches: [{ id: 'pa3', title: 'Add migration 0007 + index', state: 'proposed', lines: 18 }],
    },
    {
      id: 'security', no: '04', name: 'Security', status: 'blocked', blocking: 'API key committed in src/config.ts', level: 0.2, costShare: 0.1,
      findings: [{ id: 'f5', severity: 'danger', text: 'Secret in source — rotate and move to vault.' }],
      patches: [],
    },
    {
      id: 'deploy', no: '05', name: 'Deploy', status: 'unstarted', blocking: 'blocked by Money + Security', level: 0, costShare: 0.22,
      findings: [], patches: [],
    },
    {
      id: 'signal', no: '06', name: 'Signal', status: 'unstarted', blocking: 'not started', level: 0, costShare: 0.1,
      findings: [], patches: [],
    },
  ],
  arrangement: [
    { gateId: 'identity', label: 'Identity', start: 0, end: 20, state: 'closed' },
    { gateId: 'money', label: 'Verify webhook', start: 18, end: 46, state: 'open' },
    { gateId: 'data', label: 'Migrate + index', start: 22, end: 58, state: 'running' },
    { gateId: 'security', label: 'Rotate secret', start: 30, end: 52, state: 'blocked' },
    { gateId: 'deploy', label: 'Ship to domain', start: 60, end: 84, state: 'unstarted' },
    { gateId: 'signal', label: 'Wire board', start: 84, end: 100, state: 'unstarted' },
  ],
};

const GATE_NAMES: { id: string; no: string; name: string }[] = [
  { id: 'identity', no: '01', name: 'Identity' },
  { id: 'money', no: '02', name: 'Money' },
  { id: 'data', no: '03', name: 'Data' },
  { id: 'security', no: '04', name: 'Security' },
  { id: 'deploy', no: '05', name: 'Deploy' },
  { id: 'signal', no: '06', name: 'Signal' },
];

const URL_RE = /https?:\/\/\S+/i;

// Build a fresh, unstarted project from a composer prompt. Name is derived from
// the prompt; an import URL is recorded as the source.
export function newProjectFromPrompt(prompt: string): StudioProject {
  const url = prompt.match(URL_RE)?.[0];
  const words = prompt.replace(URL_RE, '').trim().split(/\s+/).filter(Boolean).slice(0, 4).join(' ');
  const name = words ? words.replace(/^\w/, (c) => c.toUpperCase()) : 'New project';
  const id = `p_${Date.now().toString(36)}`;
  return {
    id,
    executionId: `exec_${Date.now().toString(36)}`,
    name: name.length > 40 ? `${name.slice(0, 40)}…` : name,
    source: url ? `importing ${new URL(url).host}` : 'from your prompt',
    completion: 0,
    deploy: { status: 'none' },
    meters: { walletUsed: 0, walletBudget: 50, marginPct: 0, throughput: 0 },
    profitGuard: { reservedUSD: 0, expectedMarginPct: 0, verdict: 'allow' },
    activity: [{ id: 'seed', ts: Date.now(), kind: 'gate', text: 'Project created — mapping finisher gates' }],
    security: {
      riskScore: 0,
      sbom: { format: 'CycloneDX', components: 0 },
      scanners: SCANNER_NAMES.map((s) => ({ ...s, status: 'not_run', count: 0 })),
      findings: [],
      policy: { decisionId: 'pdec_pending', effect: 'allow', risk: 'low', reason: 'no_scan_yet', obligations: [] },
    },
    gates: GATE_NAMES.map((g) => ({ ...g, status: 'unstarted', blocking: 'not started', level: 0, costShare: 0, findings: [], patches: [] })),
    arrangement: [],
  };
}

// --- Agents -------------------------------------------------------------
// The orchestrator runs a roster of specialist agents (see core agents.yaml).
// Cross-cutting agents (Orchestrator, Coder) plus one specialist per gate.
export type AgentStatus = 'idle' | 'working' | 'blocked' | 'done';

// How a custom agent is run. Manual = only when dispatched; the rest fire on a
// cadence or a project event (the orchestrator's webhook/trigger model).
export type AgentScheduleMode = 'manual' | 'interval' | 'daily' | 'weekly' | 'on_event';

export type AgentTrigger = 'gate_blocked' | 'patch_proposed' | 'deploy_started' | 'scan_findings';

export interface AgentSchedule {
  mode: AgentScheduleMode;
  /** interval mode — human cadence, e.g. "6h", "30m" */
  every?: string;
  /** daily/weekly mode — "HH:MM" */
  at?: string;
  /** weekly mode — 0 (Sun) … 6 (Sat) */
  weekday?: number;
  /** on_event mode — the project event that fires the agent */
  trigger?: AgentTrigger;
  /** false pauses the schedule without deleting the agent */
  enabled: boolean;
}

// How much rope the agent gets. Mirrors the orchestrator's patch lifecycle:
// suggest = propose only; approval = apply behind a human gate; autonomous =
// apply within its budget + guardrails without waiting.
export type AgentAutonomy = 'suggest' | 'approval' | 'autonomous';

export interface Agent {
  id: string;
  name: string;
  role: string;
  /** one-line routing signal — what this agent is for / when to use it */
  description?: string;
  /** the finisher gate this agent owns, if any */
  gateId?: string;
  /** detailed task definition — what exactly the agent should do */
  instructions?: string;
  /** capabilities the agent may use — ids from SKILL_LIBRARY or free text */
  skills?: string[];
  /** tools / connectors the agent may call — ids from TOOL_LIBRARY */
  tools?: string[];
  /** areas of responsibility — domains or path globs the agent owns */
  responsibilities?: string[];
  /** guardrail rule ids from GUARDRAILS the agent must honor */
  guardrails?: string[];
  /** knowledge sources attached to ground the agent (doc names / refs) */
  knowledge?: string[];
  /** model id from MODEL_OPTIONS the agent reasons with */
  model?: string;
  /** how much autonomy the agent has over applying changes */
  autonomy?: AgentAutonomy;
  /** may hand work off to other agents */
  canDelegate?: boolean;
  /** agent ids this agent may hand off to */
  handoffTo?: string[];
  /** when and how the agent runs (custom agents only) */
  schedule?: AgentSchedule;
  /** true for operator-created agents (vs the built-in orchestrator roster) */
  custom?: boolean;
}

export const AGENTS: Agent[] = [
  { id: 'orchestrator', name: 'Orchestrator', role: 'Plans the run and routes work to specialists', description: 'The conductor — decomposes the goal and dispatches every other agent.', skills: ['planning', 'spec'], tools: ['atlas'], model: 'opus', autonomy: 'autonomous', canDelegate: true },
  { id: 'coder', name: 'Coder', role: 'Writes and applies reviewed patches', description: 'Implements stories end-to-end as small, reviewable patches.', skills: ['patch', 'review', 'refactor'], tools: ['fs_read', 'fs_write', 'atlas'], model: 'sonnet', autonomy: 'approval' },
  { id: 'identity', name: 'Identity agent', role: 'Auth, sessions, roles, ownership', description: 'Owns who-can-do-what: auth wiring and ownership checks.', gateId: 'identity', skills: ['authz', 'backend'], tools: ['fs_write', 'atlas'], model: 'sonnet', autonomy: 'approval' },
  { id: 'payments', name: 'Payments agent', role: 'Stripe/Paddle, webhooks, reconciliation', description: 'Handles money flows and verifies every webhook signature.', gateId: 'money', skills: ['backend', 'sql'], tools: ['stripe', 'http', 'fs_write'], model: 'sonnet', autonomy: 'approval' },
  { id: 'data', name: 'Data agent', role: 'Migrations, indexes, backups', description: 'Evolves the schema safely with reversible migrations.', gateId: 'data', skills: ['migrations', 'indexes', 'sql'], tools: ['postgres', 'fs_write'], model: 'sonnet', autonomy: 'approval' },
  { id: 'security', name: 'Security agent', role: 'Secrets, scoped access, policy', description: 'Audits every patch for secrets, OWASP issues, and policy.', gateId: 'security', skills: ['secrets', 'sast', 'policy', 'threat-model'], tools: ['fs_read', 'atlas'], model: 'opus', autonomy: 'suggest' },
  { id: 'deployer', name: 'Deployer', role: 'Ships to a domain you own, rollbacks', description: 'Ships to production with health checks and a rollback path.', gateId: 'deploy', skills: ['deploy', 'rollback', 'dns', 'observability'], tools: ['shell', 'github'], model: 'sonnet', autonomy: 'approval' },
  { id: 'mobile', name: 'Mobile agent', role: 'iOS + Android build & signing', description: 'Drives Expo/Gradle/Xcode builds and store signing.', gateId: 'signal', skills: ['expo', 'frontend'], tools: ['shell', 'fs_write'], model: 'sonnet', autonomy: 'approval' },
];

export function agentStatus(agent: Agent, gates: Gate[]): AgentStatus {
  if (!agent.gateId) {
    if (agent.id === 'orchestrator' || agent.id === 'coder') return 'working';
    if (agent.custom) return agent.schedule?.enabled ? 'working' : 'idle';
    return 'idle';
  }
  const g = gates.find((x) => x.id === agent.gateId);
  if (!g) return 'idle';
  return g.status === 'closed' ? 'done' : g.status === 'blocked' ? 'blocked' : g.status === 'unstarted' ? 'idle' : 'working';
}

export function agentForGate(gateId: string): Agent | undefined {
  return AGENTS.find((a) => a.gateId === gateId);
}

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const TRIGGER_LABELS: Record<AgentTrigger, string> = {
  gate_blocked: 'a gate is blocked',
  patch_proposed: 'a patch is proposed',
  deploy_started: 'a deploy starts',
  scan_findings: 'a scan has findings',
};

// Human-readable cadence for an agent's schedule — used on cards and map nodes.
export function scheduleLabel(s?: AgentSchedule): string {
  if (!s || s.mode === 'manual') return 'Manual';
  const prefix = s.enabled ? '' : 'Paused · ';
  switch (s.mode) {
    case 'interval': return `${prefix}Every ${s.every || '6h'}`;
    case 'daily': return `${prefix}Daily at ${s.at || '09:00'}`;
    case 'weekly': return `${prefix}${WEEKDAYS[s.weekday ?? 1]} at ${s.at || '09:00'}`;
    case 'on_event': return `${prefix}When ${TRIGGER_LABELS[s.trigger ?? 'gate_blocked']}`;
  }
}

export const SCHEDULE_TRIGGERS: { value: AgentTrigger; label: string }[] = [
  { value: 'gate_blocked', label: 'A gate is blocked' },
  { value: 'patch_proposed', label: 'A patch is proposed' },
  { value: 'deploy_started', label: 'A deploy starts' },
  { value: 'scan_findings', label: 'A scan has findings' },
];

export const WEEKDAY_OPTIONS = WEEKDAYS.map((label, value) => ({ value, label }));

// Factory for a fresh operator-created agent with sane defaults.
export function newAgent(): Agent {
  return {
    id: `agent_${Date.now().toString(36)}`,
    name: '',
    role: '',
    description: '',
    instructions: '',
    skills: [],
    tools: [],
    responsibilities: [],
    guardrails: ['patch_review', 'no_secrets'],
    knowledge: [],
    model: 'sonnet',
    autonomy: 'approval',
    canDelegate: false,
    handoffTo: [],
    custom: true,
    schedule: { mode: 'manual', enabled: true },
  };
}

// --- Crews (multi-agent teams) -----------------------------------------
// A crew runs several agents together toward one goal. The process decides how
// they collaborate: sequential = a chain, parallel = fan-out workers that run
// at once, hierarchical = a manager that plans and delegates to the members.
export type CrewProcess = 'sequential' | 'parallel' | 'hierarchical';

export interface Crew {
  id: string;
  name: string;
  /** the outcome the crew is responsible for, in one line */
  goal: string;
  process: CrewProcess;
  /** agent ids that make up the crew */
  memberIds: string[];
  /** the managing agent for a hierarchical crew (plans + delegates) */
  managerId?: string;
  /** when the crew runs as a unit */
  schedule?: AgentSchedule;
}

export const CREW_PROCESSES: { value: CrewProcess; label: string; desc: string }[] = [
  { value: 'parallel', label: 'Parallel', desc: 'Every member runs at once as a worker. Fastest; best for independent tasks.' },
  { value: 'sequential', label: 'Sequential', desc: 'Members run in order, each handing its output to the next.' },
  { value: 'hierarchical', label: 'Hierarchical', desc: 'A manager agent plans the work and delegates to the members.' },
];

export function crewProcessLabel(p: CrewProcess): string {
  return CREW_PROCESSES.find((x) => x.value === p)?.label ?? p;
}

export function newCrew(): Crew {
  return {
    id: `crew_${Date.now().toString(36)}`,
    name: '',
    goal: '',
    process: 'parallel',
    memberIds: [],
    schedule: { mode: 'manual', enabled: true },
  };
}

export const statusLabel: Record<GateStatus, string> = {
  closed: 'Closed',
  running: 'Running',
  open: 'Open',
  blocked: 'Blocked',
  unstarted: 'Not started',
};

// Sample project list for the Projects page (offline). Replaced by the live
// `Projects` query once the orchestrator is connected.
export const recentProjects: { project: StudioProject; desc: string; tone: string }[] = [
  { project: mockProject, desc: 'Checkout flow imported from Lovable. 2 gates open, deploy blocked on a secret.', tone: 'warning.main' },
  {
    project: {
      ...mockProject,
      id: 'p_math',
      executionId: 'exec_math01',
      name: 'MathQuest',
      source: 'shipped',
      completion: 1,
      deploy: { status: 'production', url: 'mathquest.ironflyer.app' },
      gates: mockProject.gates.map((g) => ({ ...g, status: 'closed', blocking: '', level: 1 })),
    },
    desc: 'Gamified math learning platform. Shipped — all gates closed.',
    tone: 'success.main',
  },
];
