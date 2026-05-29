import { useMemo } from 'react';
import { useGraphQLQuery, operations } from '@ironflyer/data';

export interface ProjectExecution {
  id: string;
  status: string;
  budgetUSD: number;
  spentUSD: number;
  reservedUSD: number;
  refundedUSD: number;
  revenueUSD: number;
  providerCostUSD: number;
  sandboxCostUSD: number;
  storageCostUSD: number;
  deploymentCostUSD: number;
  completionScore: number;
  grossMarginPct?: number | null;
  riskScore?: number | null;
  createdAt: string;
  startedAt?: string | null;
  promptSummary?: string | null;
}

export interface ProjectEconomics {
  spentUSD: number;
  budgetUSD: number;
  providerCostUSD: number;
  sandboxCostUSD: number;
  revenueUSD: number;
  refundedUSD: number;
  marginUSD: number;
  marginPct: number;
  runs: number;
}

// Executions for a project, newest first. The first entry is the run whose
// scanner report + completion drive the live Security/Dashboard surfaces.
export function useProjectExecutions(projectId: string | null) {
  const { data, isLive } = useGraphQLQuery<ProjectExecution[], { projectExecutions: ProjectExecution[] }>({
    key: ['project-executions', projectId ?? 'none'],
    operationName: 'ProjectExecutions', query: operations.PROJECT_EXECUTIONS,
    variables: { projectId, limit: 50 }, fallbackData: [], enabled: !!projectId,
    map: (r) => r.projectExecutions ?? [],
  });

  // Per-project economics: summed across this project's executions, so each
  // project shows its own spend/margin instead of the tenant-wide wallet.
  const economics = useMemo<ProjectEconomics>(() => {
    const sum = (f: (e: ProjectExecution) => number) => data.reduce((a, e) => a + (f(e) || 0), 0);
    const revenueUSD = sum((e) => e.revenueUSD);
    const providerCostUSD = sum((e) => e.providerCostUSD);
    const sandboxCostUSD = sum((e) => e.sandboxCostUSD);
    const marginUSD = revenueUSD - providerCostUSD - sandboxCostUSD;
    return {
      spentUSD: sum((e) => e.spentUSD),
      budgetUSD: sum((e) => e.budgetUSD),
      providerCostUSD, sandboxCostUSD, revenueUSD,
      refundedUSD: sum((e) => e.refundedUSD),
      marginUSD,
      marginPct: revenueUSD > 0 ? Math.round((marginUSD / revenueUSD) * 100) : 0,
      runs: data.length,
    };
  }, [data]);

  return { executions: data, latest: data[0] ?? null, economics, isLive };
}
