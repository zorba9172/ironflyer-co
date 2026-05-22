// Typed client over the orchestrator's per-project SSE stream.
//
// The orchestrator publishes ExecutionEvent records on
// GET /api/orchestrator/projects/{id}/stream (SSE, event name "execution").
// Agent A is layering richer event kinds onto the same stream — planner_*,
// gate_*, patch_*, run_complete, run_failed. This module normalises both
// the legacy ExecutionEvent shape and the richer kinds into a single
// RunEvent union the UI can switch on without knowing which orchestrator
// version it is talking to.
//
// EventSource cannot set Authorization headers, so the JWT travels via the
// ?token= query param appended by `api.streamURL(id)`.

import { api, ExecutionEvent, GateName } from '../api';

export type RunEventKind =
  | 'planner_started'
  | 'planner_done'
  | 'agent_thought'
  | 'gate_running'
  | 'gate_passed'
  | 'gate_failed'
  | 'gate_repaired'
  | 'patch_proposed'
  | 'patch_applied'
  | 'patch_rejected'
  | 'run_started'
  | 'run_complete'
  | 'run_failed'
  | 'execution'; // legacy passthrough

export interface RunEvent {
  id: string;
  kind: RunEventKind;
  step?: string;
  gate?: GateName | string;
  agent?: string;
  message: string;
  status?: string;
  detail?: string;
  patchId?: string;
  createdAt: string;
  raw?: ExecutionEvent | Record<string, unknown>;
}

export interface StreamHandle {
  close: () => void;
}

export interface StreamOptions {
  onEvent: (e: RunEvent) => void;
  onError?: (msg: string) => void;
  onOpen?: () => void;
}

// subscribeRunStream wires up the orchestrator SSE stream and translates
// every incoming event into a normalised RunEvent. It listens both for the
// legacy "execution" event name and for any of the new kinds, so the UI
// keeps working whether or not Agent A has shipped the richer events yet.
export function subscribeRunStream(projectId: string, opts: StreamOptions): StreamHandle {
  let es: EventSource | null = null;
  try {
    es = new EventSource(api.streamURL(projectId));
  } catch (err) {
    opts.onError?.(`stream init failed: ${String(err)}`);
    return { close: () => {} };
  }

  const handle = (kind: RunEventKind, raw: string) => {
    try {
      const parsed = JSON.parse(raw) as Record<string, unknown> & Partial<ExecutionEvent>;
      const normalised = normalise(kind, parsed);
      opts.onEvent(normalised);
    } catch {
      // ignore malformed payload — never crash the panel
    }
  };

  // legacy + rich kinds — register the listeners we care about.
  const kinds: RunEventKind[] = [
    'execution',
    'planner_started', 'planner_done', 'agent_thought',
    'gate_running', 'gate_passed', 'gate_failed', 'gate_repaired',
    'patch_proposed', 'patch_applied', 'patch_rejected',
    'run_started', 'run_complete', 'run_failed',
  ];
  for (const kind of kinds) {
    es.addEventListener(kind, (evt) => handle(kind, (evt as MessageEvent).data));
  }
  // also catch unnamed 'message' events as legacy execution payloads.
  es.onmessage = (evt) => handle('execution', evt.data);
  es.onopen = () => opts.onOpen?.();
  es.onerror = () => {
    // EventSource auto-reconnects, so we only surface this as a soft warning.
    opts.onError?.('Live run stream interrupted — retrying.');
  };

  return {
    close: () => {
      es?.close();
      es = null;
    },
  };
}

function normalise(kind: RunEventKind, raw: Record<string, unknown> & Partial<ExecutionEvent>): RunEvent {
  // The legacy ExecutionEvent has step/gate/status/message. Rich events from
  // Agent A may carry { patchId, detail, agent }. Either way we coerce into a
  // single RunEvent the UI renders.
  const idRaw = raw.id ?? raw['eventId'] ?? raw['ID'];
  return {
    id: typeof idRaw === 'string' && idRaw.length > 0
      ? idRaw
      : `${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    kind,
    step: typeof raw.step === 'string' ? raw.step : undefined,
    gate: (raw.gate as GateName | undefined) ?? (raw['gate'] as string | undefined),
    agent: typeof raw.agent === 'string' ? raw.agent : (raw['agent'] as string | undefined),
    message: typeof raw.message === 'string' ? raw.message
      : (raw['detail'] as string | undefined) ?? humanise(kind),
    status: typeof raw.status === 'string' ? raw.status : undefined,
    detail: raw['detail'] as string | undefined,
    patchId: raw['patchId'] as string | undefined,
    createdAt: typeof raw.createdAt === 'string' ? raw.createdAt : new Date().toISOString(),
    raw,
  };
}

function humanise(kind: RunEventKind): string {
  switch (kind) {
    case 'planner_started': return 'Planner started';
    case 'planner_done':    return 'Planner ready';
    case 'gate_running':    return 'Gate running';
    case 'gate_passed':     return 'Gate passed';
    case 'gate_failed':     return 'Gate failed';
    case 'gate_repaired':   return 'Gate repaired';
    case 'patch_proposed':  return 'Patch proposed';
    case 'patch_applied':   return 'Patch applied';
    case 'patch_rejected':  return 'Patch rejected';
    case 'run_started':     return 'Run started';
    case 'run_complete':    return 'Run complete';
    case 'run_failed':      return 'Run failed';
    default:                return 'Event';
  }
}

// Severity bucket the timeline uses for icon + colour selection. Keeping it
// inside the stream module so every consumer agrees on the mapping.
export function eventSeverity(e: RunEvent): 'info' | 'success' | 'danger' | 'progress' {
  if (e.kind.endsWith('_failed') || e.status === 'error' || e.status === 'failed') return 'danger';
  if (e.kind.endsWith('_passed') || e.kind === 'run_complete' || e.kind === 'patch_applied'
      || e.kind === 'planner_done' || e.status === 'done' || e.status === 'passed') return 'success';
  if (e.kind.endsWith('_running') || e.kind === 'run_started' || e.kind === 'planner_started'
      || e.status === 'running') return 'progress';
  return 'info';
}
