import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createGraphQLClient, type GraphQLRequest } from './client';

interface ProviderConfig {
  /** GraphQL endpoint. When omitted, hooks return their fallback data (offline). */
  endpoint?: string;
  getToken?: () => string | null | undefined;
  persistedQueries?: Record<string, string>;
}

export interface DataConfig {
  request: GraphQLRequest | null;
  endpoint?: string;
  getToken?: () => string | null | undefined;
}

const DataContext = createContext<DataConfig>({ request: null });

export function IronflyerDataProvider({ endpoint, getToken, persistedQueries, children }: ProviderConfig & { children: ReactNode }) {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { staleTime: 30_000, retry: 1, refetchOnWindowFocus: false } } }),
  );
  const value = useMemo<DataConfig>(
    () => ({ request: endpoint ? createGraphQLClient({ endpoint, getToken, persistedQueries }) : null, endpoint, getToken }),
    [endpoint, getToken, persistedQueries],
  );
  return (
    <QueryClientProvider client={client}>
      <DataContext.Provider value={value}>{children}</DataContext.Provider>
    </QueryClientProvider>
  );
}

/** The configured GraphQL request fn, or null when no endpoint is set. */
export function useRequest(): GraphQLRequest | null {
  return useContext(DataContext).request;
}

/** Endpoint + token for non-GraphQL transports (SSE). */
export function useDataConfig(): DataConfig {
  return useContext(DataContext);
}
