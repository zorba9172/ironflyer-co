// Live data layer: fetches projects + their finisher gates from the
// orchestrator's GraphQL API of record (POST /graphql).
//
// No GraphQL client dependency is installed in clients/mobile, so this uses
// the built-in `fetch`. The orchestrator response is normalized into the same
// Project/Gate shape the screens already render (see lib/sampleData.ts), so the
// UI is identical whether data is live or sample.

import type { Gate, GateStatus, Project } from './sampleData';

// Base URL is configurable via the EXPO_PUBLIC_ORCHESTRATOR_URL env var, which
// Expo inlines into the bundle at build time (no extra dependency needed),
// falling back to a sensible localhost default for `expo start`.
export function orchestratorUrl(): string {
  const fromEnv = process.env.EXPO_PUBLIC_ORCHESTRATOR_URL;
  return (fromEnv || 'http://localhost:8080').replace(/\/+$/, '');
}

const PROJECTS_QUERY = `
  query MobileProjects {
    projects {
      id
      name
      description
      status
      idea
      gates {
        gate
        status
        notes
        issues {
          message
        }
      }
    }
  }
`;

// Shapes of the GraphQL response we read. Kept minimal and local so the screens
// never see raw transport types.
type RawGateIssue = { message?: string | null };
type RawGate = {
  gate: string;
  status: string;
  notes?: string | null;
  issues?: RawGateIssue[] | null;
};
type RawProject = {
  id: string;
  name: string;
  description?: string | null;
  status?: string | null;
  idea?: string | null;
  gates?: RawGate[] | null;
};

// Map the orchestrator GateStatus enum onto the app's three-state model:
// closed = the gate passed, blocked = it failed/was refused, open = anything
// still in flight (pending/running/warn/skipped).
function mapGateStatus(status: string): GateStatus {
  switch (status) {
    case 'PASS':
      return 'closed';
    case 'FAIL':
    case 'BLOCKED':
      return 'blocked';
    default:
      return 'open';
  }
}

// Turn a gate name like "mobile-build" into a readable "Mobile Build".
function humanizeGateName(name: string): string {
  return name
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

function normalizeGate(raw: RawGate): Gate {
  const status = mapGateStatus(raw.status);
  // Surface what is still open end-to-end, per the viz-first contract: prefer
  // the gate notes, then the first issue message.
  const blocking =
    raw.notes?.trim() ||
    raw.issues?.find((i) => i.message?.trim())?.message?.trim() ||
    undefined;
  return {
    id: raw.gate,
    name: humanizeGateName(raw.gate),
    status,
    blocking: status === 'closed' ? undefined : blocking,
  };
}

function normalizeProject(raw: RawProject): Project {
  return {
    id: raw.id,
    name: raw.name,
    source: raw.status ? `Status: ${raw.status}` : 'Orchestrator project',
    summary: raw.description?.trim() || raw.idea?.trim() || 'No summary yet.',
    gates: (raw.gates ?? []).map(normalizeGate),
  };
}

type GraphQLResponse = {
  data?: { projects?: RawProject[] | null };
  errors?: { message: string }[];
};

// Fetch the owner-scoped project list (plus public seeds) with their gates in a
// single round-trip. Throws on transport/GraphQL errors so the hook can fall
// back to sample data.
export async function fetchProjects(signal?: AbortSignal): Promise<Project[]> {
  const res = await fetch(`${orchestratorUrl()}/graphql`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify({ query: PROJECTS_QUERY, operationName: 'MobileProjects' }),
    signal,
  });
  if (!res.ok) {
    throw new Error(`orchestrator returned ${res.status}`);
  }
  const json = (await res.json()) as GraphQLResponse;
  if (json.errors?.length) {
    throw new Error(json.errors.map((e) => e.message).join('; '));
  }
  return (json.data?.projects ?? []).map(normalizeProject);
}
