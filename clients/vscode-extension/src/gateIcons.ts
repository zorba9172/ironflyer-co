// Pure mappers from gate status / issue severity to codicon names and
// theme colors. Kept out of gatesTree.ts so they stay unit-testable
// without spinning up a VSCode test host.

import type { GateStatus, IssueSeverity } from './api';

export function gateIcon(status: GateStatus): string {
  switch (status) {
    case 'passed':   return 'pass-filled';
    case 'failed':   return 'error';
    case 'running':  return 'loading~spin';
    case 'repaired': return 'wrench';
    case 'blocked':  return 'circle-slash';
    case 'pending':  return 'circle-outline';
    default:         return 'circle-outline';
  }
}

export function gateColorId(status: GateStatus): string | undefined {
  switch (status) {
    case 'passed':   return 'charts.green';
    case 'failed':   return 'charts.red';
    case 'blocked':  return 'charts.orange';
    case 'repaired': return 'charts.blue';
    default:         return undefined;
  }
}

export function issueIcon(severity: IssueSeverity): string {
  switch (severity) {
    case 'critical': return 'error';
    case 'error':    return 'error';
    case 'warning':  return 'warning';
    case 'info':     return 'info';
    default:         return 'info';
  }
}

export function issueColorId(severity: IssueSeverity): string | undefined {
  switch (severity) {
    case 'critical': return 'charts.red';
    case 'error':    return 'charts.red';
    case 'warning':  return 'charts.orange';
    case 'info':     return 'charts.blue';
    default:         return undefined;
  }
}

/**
 * Returns a one-line summary like "5 / 8 passed · 2 failed".
 * Useful for the project node description and the status bar.
 */
export function summarizeGates(gates: { status: GateStatus }[]): string {
  if (gates.length === 0) return 'no gates';
  const counts: Record<string, number> = {};
  for (const g of gates) counts[g.status] = (counts[g.status] ?? 0) + 1;
  const parts: string[] = [];
  parts.push(`${counts.passed ?? 0} / ${gates.length} passed`);
  if (counts.failed) parts.push(`${counts.failed} failed`);
  if (counts.blocked) parts.push(`${counts.blocked} blocked`);
  if (counts.running) parts.push(`${counts.running} running`);
  if (counts.repaired) parts.push(`${counts.repaired} repaired`);
  return parts.join(' · ');
}
