import type { Gate, GateStatus } from '../studioData';

// The live GateVerdict shape returned by the orchestrator's `gates` query.
export interface GateVerdict {
  gate: string;
  status: string;
  durationMs?: number | null;
  notes?: string | null;
  issues: { severity: string; message: string; path?: string | null; line?: number | null }[];
}

const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

export function mapStatus(s: string): GateStatus {
  switch (s.toLowerCase()) {
    case 'pass': case 'passed': return 'closed';
    case 'running': return 'running';
    case 'warn': return 'open';
    case 'blocked': case 'fail': return 'blocked';
    default: return 'unstarted';
  }
}

// Map a live GateVerdict to the studio Gate shape used across the cockpit.
export function mapGate(v: GateVerdict, i: number): Gate {
  const status = mapStatus(v.status);
  const err = v.issues.find((x) => x.severity === 'error');
  return {
    id: v.gate,
    no: String(i + 1).padStart(2, '0'),
    name: titleCase(v.gate),
    status,
    blocking: status === 'closed' ? '' : err?.message ?? v.issues[0]?.message ?? (v.notes || (status === 'blocked' ? 'blocked' : 'pending')),
    level: status === 'closed' ? 1 : status === 'running' ? 0.5 : status === 'open' ? 0.6 : status === 'blocked' ? 0.25 : 0,
    costShare: 0,
    findings: v.issues.map((x, j) => ({ id: `${v.gate}-${j}`, severity: x.severity === 'error' ? 'danger' : x.severity === 'warning' ? 'warning' : 'info', text: x.message })),
    patches: [],
  };
}
