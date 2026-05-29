import { useCallback } from 'react';
import { useRequest, operations } from '@ironflyer/data';
import { toast } from '@ironflyer/ui-web/fx';
import { useLiveProjectId } from './useLiveProjectId';

// Dispatches the finisher on the live project (runFinisher). Used by every
// "Fix / Fix all / Dispatch agent" action so they trigger the real loop, not a
// toast. Offline → an honest note.
export function useDispatchAgent(): { online: boolean; dispatch: (scope?: string) => Promise<void> } {
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

  return { online, dispatch };
}
