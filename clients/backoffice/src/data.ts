// Sample operator data. This is an internal admin surface, so it ships with
// realistic seed numbers; the Overview/Projects panes may overlay live
// GraphQL data when an orchestrator is reachable.

export interface ProjectRow {
  id: string;
  name: string;
  owner: string;
  status: 'shipped' | 'running' | 'blocked' | 'draft';
  gatesOpen: number;
  updatedAt: number;
}

export interface LedgerRow {
  id: string;
  ts: number;
  type: 'topup' | 'execution' | 'refund' | 'payout';
  amount: number; // signed USD; credits positive, debits negative
  balance: number;
}

export interface AuditRow {
  id: string;
  ts: number;
  actor: string;
  action: string;
  decision: 'allow' | 'deny' | 'flag';
}

const HOUR = 3600_000;
const now = Date.now();

export const overview = {
  mrr: 48210,
  activeProjects: 1284,
  providerCost30d: 11930,
  marginPct: 64,
  // 12-month revenue vs provider cost trend.
  months: ['Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec', 'Jan', 'Feb', 'Mar', 'Apr', 'May'],
  revenue: [21000, 24500, 26800, 29100, 31700, 33900, 36200, 38800, 41200, 43900, 46100, 48210],
  cost: [7400, 8200, 8900, 9300, 9800, 10100, 10400, 10700, 11000, 11300, 11600, 11930],
  // Executions grouped by terminal status (last 30 days).
  executions: [
    { name: 'Completed', value: 8420 },
    { name: 'Running', value: 612 },
    { name: 'Blocked', value: 318 },
    { name: 'Refunded', value: 144 },
  ],
};

export const projects: ProjectRow[] = [
  { id: 'p_north', name: 'Northwind Checkout', owner: 'ada@northwind.io', status: 'blocked', gatesOpen: 2, updatedAt: now - 2 * HOUR },
  { id: 'p_math', name: 'MathQuest', owner: 'lin@mathquest.app', status: 'shipped', gatesOpen: 0, updatedAt: now - 26 * HOUR },
  { id: 'p_atlas', name: 'Atlas CRM', owner: 'ravi@atlas.dev', status: 'running', gatesOpen: 1, updatedAt: now - 40 * 60_000 },
  { id: 'p_orbit', name: 'Orbit Analytics', owner: 'mei@orbit.co', status: 'running', gatesOpen: 3, updatedAt: now - 5 * HOUR },
  { id: 'p_pine', name: 'Pinecrest Booking', owner: 'sam@pinecrest.com', status: 'draft', gatesOpen: 0, updatedAt: now - 72 * HOUR },
  { id: 'p_lumen', name: 'Lumen Mobile', owner: 'kai@lumen.app', status: 'blocked', gatesOpen: 4, updatedAt: now - 9 * HOUR },
  { id: 'p_ferro', name: 'Ferro Ledger', owner: 'noa@ferro.bank', status: 'shipped', gatesOpen: 0, updatedAt: now - 14 * HOUR },
  { id: 'p_drift', name: 'Driftwood Store', owner: 'tom@driftwood.shop', status: 'running', gatesOpen: 2, updatedAt: now - 3 * HOUR },
];

function buildLedger(): LedgerRow[] {
  const seed: { type: LedgerRow['type']; amount: number }[] = [
    { type: 'topup', amount: 5000 }, { type: 'execution', amount: -412 },
    { type: 'execution', amount: -188 }, { type: 'topup', amount: 2500 },
    { type: 'execution', amount: -930 }, { type: 'refund', amount: 144 },
    { type: 'execution', amount: -276 }, { type: 'execution', amount: -519 },
    { type: 'payout', amount: -1800 }, { type: 'topup', amount: 5000 },
    { type: 'execution', amount: -640 }, { type: 'execution', amount: -355 },
  ];
  let balance = 0;
  return seed.map((s, i) => {
    balance += s.amount;
    return { id: `l_${i}`, ts: now - (seed.length - i) * 6 * HOUR, type: s.type, amount: s.amount, balance };
  }).reverse();
}
export const ledger: LedgerRow[] = buildLedger();

export const audit: AuditRow[] = [
  { id: 'a_1', ts: now - 12 * 60_000, actor: 'profit-guard', action: 'Reserve $4.20 for opus-4.7 reasoning call', decision: 'allow' },
  { id: 'a_2', ts: now - 38 * 60_000, actor: 'profit-guard', action: 'Block mac workspace allocation — wallet would go negative', decision: 'deny' },
  { id: 'a_3', ts: now - 55 * 60_000, actor: 'ada@northwind.io', action: 'Apply patch checkout-refactor.diff', decision: 'allow' },
  { id: 'a_4', ts: now - 2 * HOUR, actor: 'gate:security', action: 'Secret detected in build artifact — held for review', decision: 'flag' },
  { id: 'a_5', ts: now - 3 * HOUR, actor: 'wallet', action: 'Top-up $5,000 via Stripe (idempotent)', decision: 'allow' },
  { id: 'a_6', ts: now - 4 * HOUR, actor: 'gate:deploy', action: 'Vercel deploy artifact verified', decision: 'allow' },
  { id: 'a_7', ts: now - 6 * HOUR, actor: 'profit-guard', action: 'Deny retry loop #4 — negative expected ROI', decision: 'deny' },
  { id: 'a_8', ts: now - 8 * HOUR, actor: 'operator', action: 'Force-suspend project Lumen Mobile (4 gates open)', decision: 'flag' },
  { id: 'a_9', ts: now - 11 * HOUR, actor: 'gate:budget', action: 'Reservation released on commit — $1.18 returned', decision: 'allow' },
  { id: 'a_10', ts: now - 26 * HOUR, actor: 'lin@mathquest.app', action: 'Promote MathQuest to shipped', decision: 'allow' },
];
