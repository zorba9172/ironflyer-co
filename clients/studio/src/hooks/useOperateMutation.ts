import { useCallback, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useRequest } from '@ironflyer/data';
import { toast } from '@ironflyer/ui-web/fx';

type Requester = NonNullable<ReturnType<typeof useRequest>>;

// Shared busy + toast + invalidate wrapper for Operate mutations. Guards the
// offline case once, surfaces success/failure toasts, and refetches the listed
// query keys on success. Returns false (and toasts) when offline or on error,
// so callers can early-out without their own try/catch.
export function useOperateMutation() {
  const request = useRequest();
  const qc = useQueryClient();
  const [busy, setBusy] = useState(false);

  const run = useCallback(
    async (label: string, fn: (request: Requester) => Promise<unknown>, invalidate: readonly unknown[][] = []): Promise<boolean> => {
      if (!request) {
        toast('Connect the orchestrator to make changes.', 'error');
        return false;
      }
      setBusy(true);
      try {
        await fn(request);
        invalidate.forEach((key) => void qc.invalidateQueries({ queryKey: key }));
        toast(`${label} done.`, 'success');
        return true;
      } catch (e) {
        toast(e instanceof Error ? e.message : `${label} failed.`, 'error');
        return false;
      } finally {
        setBusy(false);
      }
    },
    [request, qc],
  );

  return { busy, run, online: !!request };
}
