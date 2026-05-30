import { useGraphQLQuery, operations } from '@ironflyer/data';

export interface Wallet { balanceUSD: number; holdUSD: number; availableUSD: number; lifetimeTopUpUSD: number; lifetimeSpendUSD: number }

const ZERO_WALLET: Wallet = { balanceUSD: 0, holdUSD: 0, availableUSD: 0, lifetimeTopUpUSD: 0, lifetimeSpendUSD: 0 };

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

export type CostVerdict = 'clear' | 'tight' | 'blocked';
export type CostEstimateSource = 'execution_average' | 'live_burn' | 'held_reservation' | 'fallback';

export interface ActionCostPreview {
  estimateUSD: number;
  headroomBeforeUSD: number;
  headroomAfterUSD: number;
  walletAfterUSD: number;
  coveragePct: number;
  verdict: CostVerdict;
  source: CostEstimateSource;
  label: string;
  detail: string;
  actionText: string;
}

export interface GateSpendInput {
  id: string;
  name: string;
  status?: string;
  costShare?: number;
  level?: number;
  agentName?: string;
}

export interface GateSpendLabel {
  gateId: string;
  gateName: string;
  agentName: string;
  amountUSD: number;
  sharePct: number;
  source: 'live' | 'fallback' | 'allocation';
  status?: string;
}

function clamp(n: number, min: number, max: number) {
  return Math.min(max, Math.max(min, n));
}

function positiveNumbers(values: number[] | undefined): number[] {
  return (values ?? []).filter((v) => Number.isFinite(v) && v > 0);
}

export function buildActionCostPreview(args: {
  wallet: Wallet;
  forecast: SentinelForecast;
  recentExecutionSpendUSD?: number[];
  fallbackEstimateUSD?: number;
}): ActionCostPreview {
  const { wallet, forecast } = args;
  const samples = positiveNumbers(args.recentExecutionSpendUSD);
  const avgExecutionUSD = samples.length > 0 ? samples.reduce((a, b) => a + b, 0) / samples.length : 0;
  const burnEstimateUSD = forecast.burnRatePerHourUSD > 0 ? clamp(forecast.burnRatePerHourUSD / 4, 0.15, 12) : 0;
  const heldEstimateUSD = wallet.holdUSD > 0 ? wallet.holdUSD : 0;
  const fallbackEstimateUSD = args.fallbackEstimateUSD ?? 2.4;

  const estimateUSD = avgExecutionUSD || burnEstimateUSD || heldEstimateUSD || fallbackEstimateUSD;
  const source: CostEstimateSource = avgExecutionUSD
    ? 'execution_average'
    : burnEstimateUSD
      ? 'live_burn'
      : heldEstimateUSD
        ? 'held_reservation'
        : 'fallback';

  const projectHeadroomUSD = forecast.hardCapUSD > 0 || forecast.spentUSD > 0 ? forecast.remainingHeadroomUSD : wallet.availableUSD;
  const headroomBeforeUSD = Math.min(wallet.availableUSD, projectHeadroomUSD);
  const headroomAfterUSD = headroomBeforeUSD - estimateUSD;
  const walletAfterUSD = wallet.availableUSD - estimateUSD;
  const coveragePct = estimateUSD > 0 ? clamp((headroomBeforeUSD / estimateUSD) * 100, 0, 100) : 100;
  const noBudget = wallet.availableUSD <= 0 || headroomBeforeUSD <= 0;
  const tight = !noBudget && (headroomAfterUSD < estimateUSD * 0.25 || forecast.level === 'warn' || forecast.level === 'critical');
  const verdict: CostVerdict = noBudget ? 'blocked' : tight ? 'tight' : 'clear';

  return {
    estimateUSD,
    headroomBeforeUSD,
    headroomAfterUSD,
    walletAfterUSD,
    coveragePct,
    verdict,
    source,
    label: source === 'execution_average' ? 'Avg next action' : source === 'live_burn' ? 'Burn guide' : source === 'held_reservation' ? 'Current hold' : 'Typical action',
    detail: source === 'execution_average'
      ? `Based on ${samples.length} recent execution${samples.length === 1 ? '' : 's'}`
      : source === 'live_burn'
        ? 'Based on the current live burn rate'
        : source === 'held_reservation'
          ? 'Based on funds already reserved'
          : 'Fallback until live run history appears',
    actionText: verdict === 'blocked'
      ? 'Top up before the next paid step'
      : verdict === 'tight'
        ? 'Review before dispatch'
        : 'Cleared for the next paid step',
  };
}

export function buildGateSpendLabels(gates: GateSpendInput[], totalSpendUSD: number, fallbackSpendUSD = 0): GateSpendLabel[] {
  if (gates.length === 0) return [];
  const spendBaseUSD = totalSpendUSD > 0 ? totalSpendUSD : fallbackSpendUSD > 0 ? fallbackSpendUSD : 0;
  const source: GateSpendLabel['source'] = totalSpendUSD > 0 ? 'live' : fallbackSpendUSD > 0 ? 'fallback' : 'allocation';
  const costShareTotal = gates.reduce((a, g) => a + Math.max(0, g.costShare ?? 0), 0);
  const weights = gates.map((g) => {
    if (costShareTotal > 0) return Math.max(0, g.costShare ?? 0) / costShareTotal;
    const progress = clamp(g.level ?? 0, 0, 1);
    const statusWeight = g.status === 'closed' ? 1 : g.status === 'running' ? 0.75 : g.status === 'blocked' || g.status === 'open' ? 0.45 : 0.25;
    return Math.max(0.1, progress || statusWeight);
  });
  const weightTotal = weights.reduce((a, b) => a + b, 0) || gates.length;

  return gates.map((g, i) => {
    const share = (weights[i] ?? 1 / gates.length) / weightTotal;
    return {
      gateId: g.id,
      gateName: g.name,
      agentName: g.agentName ?? 'Unassigned',
      amountUSD: spendBaseUSD * share,
      sharePct: Math.round(share * 100),
      source,
      status: g.status,
    };
  });
}
