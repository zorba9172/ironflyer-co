import { useCallback } from 'react';
import { useRequest, operations } from '@ironflyer/data';
import { toast } from '@ironflyer/ui-web/fx';
import { useLiveProjectId } from './useLiveProjectId';

interface Dispatcher {
  online: boolean;
  /** Run the full finisher loop on the live project (Fix all / dispatch). */
  dispatch: (scope?: string) => Promise<void>;
  /** Re-run a single named gate without the full loop (targeted Fix). */
  repairGate: (gate: string, label?: string) => Promise<void>;
}

// Dispatches the finisher on the live project. `dispatch` runs the whole loop
// (Fix all); `repairGate` re-runs one gate via rerunGate (targeted Fix). Both
// trigger the real backend, not a toast. Offline → an honest note.
export function useDispatchAgent(): Dispatcher {
  const request = useRequest();
  const liveProjectId = useLiveProjectId();
  const online = !!request && !!liveProjectId;

  // Stable identity so consumers can keep it out of memo/effect deps — the map
  // builds hundreds of nodes and must not rebuild on every render.
  const dispatch = useCallback(async (scope = 'the open work') => {
    if (!request || !liveProjectId) {
      toast(`Connect the orchestrator to dispatch an agent for ${scope}.`, 'info');
      return;
    }
    try {
      await request('RunFinisher', operations.RUN_FINISHER, { id: liveProjectId });
      toast(`Agent dispatched to finish ${scope}.`, 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not dispatch agent.', 'error');
    }
  }, [request, liveProjectId]);

  const repairGate = useCallback(async (gate: string, label?: string) => {
    const what = label ?? gate;
    if (!request || !liveProjectId) {
      toast(`Connect the orchestrator to re-run the ${what} gate.`, 'info');
      return;
    }
    try {
      await request('RerunGate', operations.RERUN_GATE, { input: { projectId: liveProjectId, gate } });
      toast(`Re-running the ${what} gate…`, 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not re-run the gate.', 'error');
    }
  }, [request, liveProjectId]);

  return { online, dispatch, repairGate };
}
