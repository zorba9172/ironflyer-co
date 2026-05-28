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

export interface StudioProject {
  id: string;
  name: string;
  source: string;
  completion: number; // 0..1 overall
  gates: Gate[];
  arrangement: ArrangementBlock[];
  meters: {
    walletUsed: number;
    walletBudget: number;
    marginPct: number;
    throughput: number; // runs/min
  };
}

export const mockProject: StudioProject = {
  id: 'p_001',
  name: 'Northwind Checkout',
  source: 'imported from lovable.dev',
  completion: 0.62,
  meters: { walletUsed: 18.4, walletBudget: 50, marginPct: 64, throughput: 3.2 },
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
  return {
    id: `p_${Date.now().toString(36)}`,
    name: name.length > 40 ? `${name.slice(0, 40)}…` : name,
    source: url ? `importing ${new URL(url).host}` : 'from your prompt',
    completion: 0,
    meters: { walletUsed: 0, walletBudget: 50, marginPct: 0, throughput: 0 },
    gates: GATE_NAMES.map((g) => ({ ...g, status: 'unstarted', blocking: 'not started', level: 0, costShare: 0, findings: [], patches: [] })),
    arrangement: [],
  };
}

export const statusLabel: Record<GateStatus, string> = {
  closed: 'Closed',
  running: 'Running',
  open: 'Open',
  blocked: 'Blocked',
  unstarted: 'Not started',
};
