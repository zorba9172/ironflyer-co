"use client";

// useRecentExecutions — paged wrapper around useExecutionsQuery used by
// the cockpit Nav notification dropdown and any "latest N runs"
// surface. Skips when unauthenticated and polls every 30s so widgets
// outside the dashboard stay live without each caller re-deriving the
// fetch policy.

import type { ApolloError } from "@apollo/client";
import { useAuth } from "../auth";
import {
  useExecutionsQuery,
  type ExecutionsQuery,
} from "../gql/__generated__";

const POLL_INTERVAL_MS = 30_000;

export type RecentExecution = ExecutionsQuery["executions"][number];

export interface RecentExecutionsResult {
  executions: RecentExecution[];
  loading: boolean;
  error: ApolloError | undefined;
  refetch: () => Promise<unknown>;
}

export function useRecentExecutions(limit = 5): RecentExecutionsResult {
  const { authenticated } = useAuth();
  const { data, loading, error, refetch } = useExecutionsQuery({
    skip: !authenticated,
    variables: { limit, offset: 0 },
    fetchPolicy: "cache-and-network",
    pollInterval: authenticated ? POLL_INTERVAL_MS : 0,
  });

  return {
    executions: data?.executions ?? [],
    loading,
    error,
    refetch,
  };
}
