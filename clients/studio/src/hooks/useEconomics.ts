import { useMemo } from 'react';
import { useGraphQLQuery, operations } from '@ironflyer/data';

export interface Wallet { balanceUSD: number; holdUSD: number; availableUSD: number; lifetimeTopUpUSD: number; lifetimeSpendUSD: number }
export interface Rollup { revenueUSD: number; providerCostUSD: number; sandboxCostUSD: number; storageCostUSD: number; deploymentCostUSD: number; premiumReasoningCostUSD: number; refundsUSD: number; platformMarginUSD: number; grossMarginPct: number }

const ZERO_WALLET: Wallet = { balanceUSD: 0, holdUSD: 0, availableUSD: 0, lifetimeTopUpUSD: 0, lifetimeSpendUSD: 0 };
const ZERO_ROLLUP: Rollup = { revenueUSD: 0, providerCostUSD: 0, sandboxCostUSD: 0, storageCostUSD: 0, deploymentCostUSD: 0, premiumReasoningCostUSD: 0, refundsUSD: 0, platformMarginUSD: 0, grossMarginPct: 0 };

// Tenant wallet — the real prepaid balance, hold, and lifetime spend.
export function useWallet() {
  const { data, isLive } = useGraphQLQuery<Wallet, { wallet: Wallet }>({
    key: ['wallet'], operationName: 'Wallet', query: operations.WALLET,
    fallbackData: ZERO_WALLET, map: (r) => r.wallet ?? ZERO_WALLET,
  });
  return { wallet: data, isLive };
}

export interface SentinelForecast {
  level: string;
  spentUSD: number;
  hardCapUSD: number;
  burnRatePerHourUSD: number;
  extrapolatedTotalUSD: number;
  remainingHeadroomUSD: number;
  etaCompletionAt?: string | null;
}

const ZERO_FORECAST: SentinelForecast = {
  level: 'ok', spentUSD: 0, hardCapUSD: 0, burnRatePerHourUSD: 0,
  extrapolatedTotalUSD: 0, remainingHeadroomUSD: 0, etaCompletionAt: null,
};

// Live trajectory for a project: burn rate, projected total, ETA to completion.
// Polled so the map's header reads as a real-time instrument.
export function useSentinelForecast(projectId: string | null) {
  const { data, isLive } = useGraphQLQuery<SentinelForecast, { sentinelForecast: SentinelForecast }>({
    key: ['sentinel-forecast', projectId ?? 'none'],
    operationName: 'SentinelForecast', query: operations.SENTINEL_FORECAST,
    variables: { projectId }, fallbackData: ZERO_FORECAST, enabled: !!projectId,
    refetchInterval: 10000, map: (r) => r.sentinelForecast ?? ZERO_FORECAST,
  });
  return { forecast: data, isLive };
}

// Margin rollup over a trailing window (default 30d): revenue − provider cost.
export function useLedgerRollup(days = 30) {
  const { since, until } = useMemo(() => {
    const now = new Date();
    const start = new Date(now.getTime() - days * 24 * 60 * 60 * 1000);
    return { since: start.toISOString(), until: now.toISOString() };
  }, [days]);
  const { data, isLive } = useGraphQLQuery<Rollup, { ledgerRollup: Rollup }>({
    key: ['ledger-rollup', days], operationName: 'LedgerRollup', query: operations.LEDGER_ROLLUP,
    variables: { since, until }, fallbackData: ZERO_ROLLUP, map: (r) => r.ledgerRollup ?? ZERO_ROLLUP,
  });
  return { rollup: data, isLive };
}
