import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { useRunProjectFeed, useGraphQLQuery, operations } from '@ironflyer/data';
import { formatRelativeTime } from '@ironflyer/core';
import type { ActivityEvent, StudioProject } from '../studioData';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useStudio } from '../store';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { TechIcon } from '../lib/techIcons';

interface LedgerEntry { id: string; executionID?: string | null; entryType: string; direction: string; amountUSD: number; provider?: string | null; createdAt: string }
const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

function ledgerKind(entryType: string): ActivityEvent['kind'] {
  const e = entryType.toLowerCase();
  if (e.includes('profit') || e.includes('budget') || e.includes('guard') || e.includes('reserv')) return 'profitguard';
  if (e.includes('deploy')) return 'deploy';
  return 'ledger';
}

function kindColor(t: Theme, kind: ActivityEvent['kind']): string {
  switch (kind) {
    case 'gate': return t.brand.accent.secondary;
    case 'patch': return t.palette.primary.main;
    case 'profitguard': return t.palette.warning.main;
    case 'deploy': return t.palette.success.main;
    default: return t.palette.text.disabled;
  }
}

// One table for everything the orchestrator emitted on this project: gate
// transitions, patches, ProfitGuard verdicts, ledger debits, deploys. Select
// rows (or all) and dispatch an agent to fix them.
export function LogsPane({ fallback }: { fallback: StudioProject }) {
  const t = useTheme();
  const [selected, setSelected] = useState<ActivityEvent[]>([]);
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const liveProjectId = storeProjectId ?? firstProjectId;
  const { dispatch } = useDispatchAgent();
  const { events: liveEvents, isLive } = useRunProjectFeed(liveProjectId);
  const { executions } = useProjectExecutions(liveProjectId);

  // Real historical activity from the ledger, scoped to this project's
  // executions when we can map them, else the tenant-wide feed.
  const { data: ledger, isLive: ledgerLive } = useGraphQLQuery<LedgerEntry[], { ledger: LedgerEntry[] }>({
    key: ['ledger', 'logs'], operationName: 'Ledger', query: operations.LEDGER,
    variables: { filter: { limit: 200 } }, fallbackData: [], map: (r) => r.ledger ?? [],
  });

  const ledgerEvents = useMemo<ActivityEvent[]>(() => {
    const execIds = new Set(executions.map((e) => e.id));
    const scoped = execIds.size > 0 ? ledger.filter((l) => l.executionID && execIds.has(l.executionID)) : ledger;
    const src = scoped.length > 0 ? scoped : ledger;
    return src.map((l) => ({
      id: l.id,
      ts: Date.parse(l.createdAt) || Date.now(),
      kind: ledgerKind(l.entryType),
      text: `${l.direction === 'debit' ? '−' : '+'}$${l.amountUSD.toFixed(4)} · ${titleCase(l.entryType)}${l.provider ? ` (${l.provider})` : ''}`,
    }));
  }, [ledger, executions]);

  const hasReal = liveEvents.length > 0 || ledgerEvents.length > 0;
  const rows = useMemo(
    () => [...liveEvents, ...ledgerEvents, ...(hasReal ? [] : fallback.activity)].sort((a, b) => b.ts - a.ts) as ActivityEvent[],
    [liveEvents, ledgerEvents, hasReal, fallback.activity],
  );
  const live = isLive || ledgerLive;

  const columns = useMemo<DataGridColumn<ActivityEvent>[]>(() => [
    {
      field: 'kind', headerName: 'Source', width: 148,
      cellRenderer: ({ data }: DataGridCellParams<ActivityEvent>) =>
        data ? (
          <Stack direction="row" alignItems="center" spacing={0.85}>
            <Box component="span" sx={{ color: kindColor(t, data.kind), display: 'inline-flex' }}><TechIcon name={data.kind} size={15} title={data.kind} /></Box>
            <Typography sx={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.04em', color: kindColor(t, data.kind) }}>{data.kind}</Typography>
          </Stack>
        ) : null,
    },
    {
      field: 'text', headerName: 'Message', flex: 1, minWidth: 320,
      cellRenderer: ({ value }: DataGridCellParams<ActivityEvent, string>) => <Typography sx={{ fontSize: '0.86rem' }} noWrap>{value}</Typography>,
    },
    {
      field: 'ts', headerName: 'When', width: 130,
      valueFormatter: ({ value }) => formatRelativeTime(Number(value)),
    },
    {
      colId: 'fix', headerName: '', width: 90, sortable: false, filter: false,
      cellRenderer: ({ data }: DataGridCellParams<ActivityEvent>) =>
        data ? <Button size="small" variant="outlined" color="inherit" onClick={(e) => { e.stopPropagation(); void dispatch('this log'); }}>Fix</Button> : null,
    },
  ], [t, dispatch]);

  const fixSelected = () => {
    const n = selected.length || rows.length;
    void dispatch(`${n} log${n > 1 ? 's' : ''}`);
  };

  // Headline visual — event volume by source, mirroring exactly what the
  // orchestrator emitted on this project (gate / patch / ProfitGuard / deploy /
  // ledger). Reads in one glance before the operator scans the row detail.
  const sourceBar = useMemo<EChartsOption>(() => {
    const order: ActivityEvent['kind'][] = ['gate', 'patch', 'profitguard', 'deploy', 'ledger'];
    const counts = order.map((k) => rows.filter((r) => r.kind === k).length);
    const labels = ['Gate', 'Patch', 'ProfitGuard', 'Deploy', 'Ledger'];
    return {
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      grid: { left: 8, right: 16, top: 12, bottom: 8, containLabel: true },
      xAxis: { type: 'value', axisLabel: { color: t.palette.text.secondary }, splitLine: { lineStyle: { color: t.palette.divider } } },
      yAxis: { type: 'category', data: labels, axisLabel: { color: t.palette.text.secondary } },
      series: [{
        type: 'bar', barWidth: '56%', data: counts.map((value, i) => ({ value, itemStyle: { color: kindColor(t, order[i]!) } })),
        label: { show: true, position: 'right', color: t.palette.text.secondary, fontSize: 11 },
      }],
    };
  }, [rows, t]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Logs</Typography>
            <Chip size="small" label={live ? 'live' : 'seed'} sx={(th) => ({ height: 20, fontSize: '0.64rem', fontFamily: th.brand.font.mono, bgcolor: live ? `${th.palette.success.main}22` : 'action.hover', color: live ? 'success.main' : 'text.disabled' })} />
            <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>
              {rows.length} events{selected.length > 0 ? ` · ${selected.length} selected` : ''}
            </Typography>
          </Stack>
          <Button variant="contained" disabled={rows.length === 0} onClick={fixSelected}>
            {selected.length > 0 ? `Fix selected (${selected.length})` : 'Fix all'}
          </Button>
        </Stack>

        {rows.length > 0 && (
          <Card sx={{ p: 2, mb: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Events by source</Typography>
            <Chart option={sourceBar} height={170} />
          </Card>
        )}

        <DataGrid
          rows={rows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No logs yet — run the finisher to generate activity."
          height={520}
          minHeight={300}
          pagination={rows.length > 12}
          pageSize={12}
          gridOptions={{
            suppressPaginationPanel: rows.length <= 12,
            rowSelection: { mode: 'multiRow' },
            onSelectionChanged: (e) => setSelected(e.api.getSelectedRows()),
          }}
        />
      </Box>
    </Box>
  );
}
