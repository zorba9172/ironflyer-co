import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createGraphQLClient, type GraphQLRequest } from './client';

interface ProviderConfig {
  /** GraphQL endpoint. When omitted, hooks return their fallback data (offline). */
  endpoint?: string;
  getToken?: () => string | null | undefined;
  persistedQueries?: Record<string, string>;
}

const RequestContext = createContext<GraphQLRequest | null>(null);

export function IronflyerDataProvider({ endpoint, getToken, persistedQueries, children }: ProviderConfig & { children: ReactNode }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { staleTime: 30_000, retry: 1, refetchOnWindowFocus: false } } }),
  );
  const request = useMemo(
    () => (endpoint ? createGraphQLClient({ endpoint, getToken, persistedQueries }) : null),
    [endpoint, getToken, persistedQueries],
  );
  return (
    <QueryClientProvider client={client}>
      <RequestContext.Provider value={request}>{children}</RequestContext.Provider>
    </QueryClientProvider>
  );
}

/** The configured GraphQL request fn, or null when no endpoint is set. */
export function useRequest(): GraphQLRequest | null {
  return useContext(RequestContext);
}
