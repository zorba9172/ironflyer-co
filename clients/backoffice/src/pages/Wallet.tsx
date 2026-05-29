import { useMemo } from 'react';
import { Box, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { palette } from '@ironflyer/design-tokens/brand';
import { formatUSD, formatRelativeTime } from '@ironflyer/core';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { ledger as SAMPLE, type LedgerRow } from '../data';

function typeColor(t: Theme, type: LedgerRow['type']): string {
  switch (type) {
    case 'topup': return t.palette.success.main;
    case 'refund': return t.brand.accent.secondary;
    case 'payout': return t.palette.warning.main;
    default: return t.palette.text.secondary;
  }
}

// Live ledger entry shape (subset of WalletLedgerEntry from operations.LEDGER).
interface RawLedgerEntry {
  id: string;
  entryType: string;
  direction: string;
  amountUSD: number;
  createdAt: string;
}
interface RawWallet {
  balanceUSD: number;
  lifetimeTopUpUSD: number;
  lifetimeSpendUSD: number;
}

// Collapse the canonical orchestrator entry_type set onto the four buckets
// the operator ledger renders. Anything else falls into 'execution'.
function bucketType(entryType: string): LedgerRow['type'] {
  if (entryType === 'wallet_topup') return 'topup';
  if (entryType === 'refund' || entryType === 'credit_release') return 'refund';
  if (entryType === 'platform_margin') return 'payout';
  return 'execution';
}

export function Wallet() {
  const t = useTheme();
  const axis = t.palette.text.secondary;
  const grid = t.palette.divider;

  // Wallet balance + lifetime credits/debits; live when an orchestrator is
  // reachable, otherwise the seed totals derived below.
  const { data: walletStats, isLive: walletLive } = useGraphQLQuery<RawWallet | null, { wallet: RawWallet }>({
    key: ['bo-wallet'],
    operationName: 'Wallet', query: operations.WALLET,
    fallbackData: null,
    map: (r) => r.wallet ?? null,
  });

  // Ledger entries, newest-first (the server orders created_at DESC). Map the
  // raw WalletLedgerEntry rows into the operator LedgerRow shape with a running
  // balance reconstructed chronologically; fall back to the seed when offline.
  const { data: ledger, isLive: ledgerLive } = useGraphQLQuery<LedgerRow[], { ledger: RawLedgerEntry[] }>({
    key: ['bo-ledger'],
    operationName: 'Ledger', query: operations.LEDGER,
    fallbackData: SAMPLE,
    map: (r) => {
      if (!r.ledger?.length) return SAMPLE;
      // Re-sign amounts: amountUSD is always positive, direction carries sign.
      const signed = r.ledger.map((e) => ({
        id: e.id,
        ts: Date.parse(e.createdAt) || Date.now(),
        type: bucketType(e.entryType),
        amount: e.direction === 'debit' ? -Math.abs(e.amountUSD) : Math.abs(e.amountUSD),
      }));
      // Walk oldest → newest to accumulate the running balance, then present
      // newest-first to match the seed ordering.
      const chrono = [...signed].reverse();
      let bal = 0;
      const withBalance = chrono.map((e) => {
        bal += e.amount;
        return { ...e, balance: bal };
      });
      return withBalance.reverse();
    },
  });

  const isLive = walletLive || ledgerLive;

  // Daily spend = sum of debit magnitudes, charted oldest → newest.
  const spend = useMemo(() => {
    const chrono = [...ledger].reverse();
    return chrono.map((l) => ({ label: formatRelativeTime(l.ts), value: l.amount < 0 ? Math.abs(l.amount) : 0 }));
  }, [ledger]);

  const credits = walletStats?.lifetimeTopUpUSD ?? ledger.filter((l) => l.amount > 0).reduce((s, l) => s + l.amount, 0);
  const debits = walletStats?.lifetimeSpendUSD ?? ledger.filter((l) => l.amount < 0).reduce((s, l) => s + Math.abs(l.amount), 0);
  const balance = walletStats?.balanceUSD ?? ledger[0]?.balance ?? 0;

  const spendOption: EChartsOption = {
    color: [palette.cobalt],
    tooltip: { trigger: 'axis', valueFormatter: (v) => formatUSD(Number(v)) },
    grid: { left: 8, right: 16, top: 16, bottom: 24, containLabel: true },
    xAxis: { type: 'category', data: spend.map((s) => s.label), axisLabel: { color: axis, fontSize: 9, rotate: 30 }, axisLine: { lineStyle: { color: grid } } },
    yAxis: { type: 'value', axisLabel: { color: axis, formatter: (v: number) => `$${v}` }, splitLine: { lineStyle: { color: grid } } },
    series: [{ type: 'bar', data: spend.map((s) => s.value), barWidth: '52%', itemStyle: { color: palette.cobalt, borderRadius: [4, 4, 0, 0] } }],
  };

  const columns = useMemo<DataGridColumn<LedgerRow>[]>(() => [
    { field: 'ts', headerName: 'Date', width: 150, valueFormatter: ({ value }) => formatRelativeTime(Number(value)) },
    { field: 'type', headerName: 'Type', width: 130, cellRenderer: ({ data }: DataGridCellParams<LedgerRow>) => data ? <Chip size="small" label={data.type} sx={{ height: 20, fontSize: '0.62rem', textTransform: 'uppercase', bgcolor: `${typeColor(t, data.type)}22`, color: typeColor(t, data.type) }} /> : null },
    { field: 'amount', headerName: 'Amount', flex: 1, minWidth: 140, cellRenderer: ({ data }: DataGridCellParams<LedgerRow>) => data ? <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.84rem', color: data.amount >= 0 ? 'success.main' : 'text.primary' })}>{data.amount >= 0 ? '+' : '−'}{formatUSD(Math.abs(data.amount))}</Typography> : null },
    { field: 'balance', headerName: 'Balance', width: 150, cellRenderer: ({ data }: DataGridCellParams<LedgerRow>) => data ? <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.84rem' })}>{formatUSD(data.balance)}</Typography> : null },
  ], [t]);

  const stats = [
    { label: 'Wallet balance', value: formatUSD(balance) },
    { label: 'Credits', value: formatUSD(credits) },
    { label: 'Debits', value: formatUSD(debits) },
  ];

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 0.5 }}>
          <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Wallet</Typography>
          <Chip size="small" label={isLive ? 'live' : 'sample'} sx={(th) => ({ height: 20, fontSize: '0.64rem', fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
        </Stack>
        <Typography sx={{ color: 'text.secondary', mb: 3 }}>Prepaid ledger — every paid execution reserves, then debits as cost materializes.</Typography>

        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ mb: 2 }}>
          {stats.map((s) => (
            <Card key={s.label} sx={{ p: 2.5, flex: 1 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>{s.label}</Typography>
              <Typography variant="h4" sx={{ fontSize: '1.8rem', mt: 0.5 }}>{s.value}</Typography>
            </Card>
          ))}
        </Stack>

        <Card sx={{ p: 2.5, mb: 2 }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Spend over time</Typography>
          <Chart option={spendOption} height={240} />
        </Card>

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Ledger</Typography>
        <DataGrid
          rows={ledger}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No ledger entries yet."
          height={440}
          minHeight={260}
        />
      </Box>
    </Box>
  );
}
