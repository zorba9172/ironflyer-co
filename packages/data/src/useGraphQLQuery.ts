import { useQuery } from '@tanstack/react-query';
import { useRequest } from './provider';

interface Options<T, R> {
  key: readonly unknown[];
  operationName: string;
  query: string;
  variables?: Record<string, unknown>;
  /** Returned while loading, when offline (no endpoint), or on error. */
  fallbackData: T;
  /** Map the raw GraphQL response to T. Defaults to identity. */
  map?: (raw: R) => T;
  enabled?: boolean;
  /** Poll the orchestrator on an interval (ms) so the surface stays live. */
  refetchInterval?: number;
}

interface Result<T> {
  data: T;
  /** true only when data came from the live API (not the fallback). */
  isLive: boolean;
  isLoading: boolean;
  error: unknown;
}

// One query hook for every read: hits the orchestrator when a request fn is
// configured, otherwise resolves to fallbackData so the UI works offline.
export function useGraphQLQuery<T, R = unknown>(opts: Options<T, R>): Result<T> {
  const request = useRequest();
  const enabled = !!request && opts.enabled !== false;

  const q = useQuery({
    queryKey: opts.key,
    enabled,
    refetchInterval: opts.refetchInterval,
    queryFn: async () => {
      const raw = await request!<R>(opts.operationName, opts.query, opts.variables);
      return opts.map ? opts.map(raw) : (raw as unknown as T);
    },
  });

  return {
    data: q.data ?? opts.fallbackData,
    isLive: enabled && q.isSuccess && q.data !== undefined,
    isLoading: enabled && q.isLoading,
    error: q.error,
  };
}
