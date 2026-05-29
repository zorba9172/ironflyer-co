import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useStudio } from '../store';
import { useLiveProjectId } from './useLiveProjectId';
import { mapGate, type GateVerdict } from '../lib/liveGates';
import type { Gate } from '../studioData';

// The canonical gate list for surfaces that read gates outside the map/dashboard
// (e.g. the GateInspector drawer). When a real project is open it returns the
// orchestrator's live verdicts; offline it falls back to the session's gates.
// Shares the `['gates', projectId]` query cache with DashboardPane so opening
// the inspector adds no extra network round-trip.
export function useLiveGates(): { gates: Gate[]; isLive: boolean } {
  const firstProjectId = useLiveProjectId();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const liveProjectId = storeProjectId ?? firstProjectId;
  const fallback = useStudio((s) => s.current.gates);

  // queryFn matches DashboardPane's `['gates', id]` exactly so the shared cache
  // never sees two divergent producers; the empty→fallback choice is applied on
  // read, not inside the cached value.
  const { data, isLive } = useGraphQLQuery<Gate[], { gates: GateVerdict[] }>({
    key: ['gates', liveProjectId ?? 'none'],
    operationName: 'Gates', query: operations.GATES,
    variables: { projectId: liveProjectId }, fallbackData: [], enabled: !!liveProjectId,
    map: (raw) => raw.gates.map(mapGate),
  });
  return { gates: data.length > 0 ? data : fallback, isLive: isLive && data.length > 0 };
}
