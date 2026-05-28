// Local sample data standing in for the orchestrator's GraphQL feed. Mirrors
// the V22 framing: each project moves toward "shippable" by closing finisher
// gates, and ProfitGuard/budget can block a gate until something is resolved.

export type GateStatus = 'closed' | 'open' | 'blocked';

export type Gate = {
  id: string;
  name: string;
  status: GateStatus;
  // What is still open end-to-end — mandatory per the viz-first contract.
  blocking?: string;
};

export type Project = {
  id: string;
  name: string;
  // Where the code came from (prompt, repo import, blueprint...).
  source: string;
  // Short one-line of what it is.
  summary: string;
  gates: Gate[];
};

export const projects: Project[] = [
  {
    id: 'aurora-checkout',
    name: 'Aurora Checkout',
    source: 'Prompt → Next.js blueprint',
    summary: 'Prepaid checkout flow with Stripe wallet top-up.',
    gates: [
      { id: 'budget', name: 'Budget', status: 'closed' },
      { id: 'plan', name: 'Plan', status: 'closed' },
      { id: 'build', name: 'Build', status: 'closed' },
      {
        id: 'verify',
        name: 'Verify',
        status: 'open',
        blocking: 'Waiting on integration run against the sandbox.',
      },
      {
        id: 'deploy',
        name: 'Deploy',
        status: 'blocked',
        blocking: 'Vercel deploy artifact missing; Verify must pass first.',
      },
    ],
  },
  {
    id: 'lumen-mobile',
    name: 'Lumen Mobile',
    source: 'Repo import → Expo',
    summary: 'Field-ops companion app with offline gate sync.',
    gates: [
      { id: 'budget', name: 'Budget', status: 'closed' },
      { id: 'plan', name: 'Plan', status: 'closed' },
      {
        id: 'mobile-build',
        name: 'Mobile Build',
        status: 'open',
        blocking: 'EAS build queued; signing secrets verified.',
      },
      { id: 'verify', name: 'Verify', status: 'open' },
      { id: 'deploy', name: 'Deploy', status: 'blocked', blocking: 'Mobile Build not yet green.' },
    ],
  },
  {
    id: 'ledger-pulse',
    name: 'Ledger Pulse',
    source: 'Prompt → Go service',
    summary: 'Append-only ledger dashboard with margin alerts.',
    gates: [
      { id: 'budget', name: 'Budget', status: 'closed' },
      { id: 'plan', name: 'Plan', status: 'closed' },
      { id: 'build', name: 'Build', status: 'closed' },
      { id: 'verify', name: 'Verify', status: 'closed' },
      { id: 'deploy', name: 'Deploy', status: 'closed' },
    ],
  },
  {
    id: 'beacon-crm',
    name: 'Beacon CRM',
    source: 'Blueprint → React + Postgres',
    summary: 'Lightweight CRM with owner-isolated workspaces.',
    gates: [
      { id: 'budget', name: 'Budget', status: 'closed' },
      {
        id: 'plan',
        name: 'Plan',
        status: 'blocked',
        blocking: 'ProfitGuard refused the premium model call: wallet below reservation.',
      },
      { id: 'build', name: 'Build', status: 'open' },
      { id: 'verify', name: 'Verify', status: 'open' },
      { id: 'deploy', name: 'Deploy', status: 'open' },
    ],
  },
];

export function getProject(id: string): Project | undefined {
  return projects.find((p) => p.id === id);
}

export function openGateCount(project: Project): number {
  return project.gates.filter((g) => g.status !== 'closed').length;
}

// Percentage of gates closed — a project is "shippable" at 100%.
export function shippablePercent(project: Project): number {
  if (project.gates.length === 0) return 0;
  const closed = project.gates.filter((g) => g.status === 'closed').length;
  return Math.round((closed / project.gates.length) * 100);
}
