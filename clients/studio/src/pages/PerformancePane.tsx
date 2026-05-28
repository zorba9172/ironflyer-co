import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';

interface RawGate { gate: string; status: string; issues: { severity: string; message: string }[] }
interface PerfRow { id: string; area: 'Frontend' | 'Backend'; name: string; status: string; detail: string }

// Which gates are performance audits, and where they run.
const PERF_AREA: Record<string, 'Frontend' | 'Backend'> = {
  lighthouse: 'Frontend', bundle_size: 'Frontend', perf_budget: 'Frontend', mobile_size: 'Frontend', mobile_bundle_analyzer: 'Frontend',
  mem_leak: 'Backend', complexity: 'Backend', dep_graph: 'Backend', arch_boundary: 'Backend',
};
const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
const isClosed = (s: string) => ['pass', 'passed'].includes(s.toLowerCase());
function statusColor(t: Theme, s: string): string {
  if (isClosed(s)) return t.palette.success.main;
  if (['running'].includes(s.toLowerCase())) return t.brand.accent.secondary;
  if (['blocked', 'fail'].includes(s.toLowerCase())) return t.palette.error.main;
  return t.palette.warning.main;
}

const SAMPLE: PerfRow[] = [
  { id: 'lighthouse', area: 'Frontend', name: 'Lighthouse', status: 'blocked', detail: 'Performance score 64 — LCP 3.8s, unused JS 240KB' },
  { id: 'bundle_size', area: 'Frontend', name: 'Bundle Size', status: 'blocked', detail: 'Main chunk 612KB over 400KB budget' },
  { id: 'perf_budget', area: 'Frontend', name: 'Perf Budget', status: 'blocked', detail: 'TBT 410ms over 200ms budget' },
  { id: 'mem_leak', area: 'Backend', name: 'Mem Leak', status: 'blocked', detail: 'Heap grows 8MB/min under load' },
  { id: 'complexity', area: 'Backend', name: 'Complexity', status: 'blocked', detail: 'checkout() cyclomatic complexity 24' },
];

const METRICS = [
  { label: 'Lighthouse', value: '64', sub: 'performance score', area: 'Frontend' },
  { label: 'Bundle', value: '612 KB', sub: 'main chunk', area: 'Frontend' },
  { label: 'p95 latency', value: '380 ms', sub: 'API requests', area: 'Backend' },
  { label: 'Error rate', value: '1.2%', sub: 'last 24h', area: 'Backend' },
];

export function PerformancePane() {
  const t = useTheme();
  const liveProjectId = useLiveProjectId();
  const { dispatch } = useDispatchAgent();
  const [selected, setSelected] = useState<PerfRow[]>([]);

  const { data: rows, isLive } = useGraphQLQuery<PerfRow[], { gates: RawGate[] }>({
    key: ['perf-gates', liveProjectId ?? 'none'],
    operationName: 'Gates', query: operations.GATES,
    variables: { projectId: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    map: (r) => {
      const perf = r.gates.filter((g) => PERF_AREA[g.gate]);
      if (!perf.length) return SAMPLE;
      return perf.map((g) => ({
        id: g.gate, area: PERF_AREA[g.gate]!, name: titleCase(g.gate), status: g.status,
        detail: isClosed(g.status) ? 'Meets budget' : g.issues[0]?.message ?? 'Needs optimization',
      }));
    },
  });

  const columns = useMemo<DataGridColumn<PerfRow>[]>(() => [
    { field: 'area', headerName: 'Area', width: 124, cellRenderer: ({ data }: DataGridCellParams<PerfRow>) => data ? <Chip size="small" label={data.area} sx={{ height: 20, fontSize: '0.64rem', bgcolor: 'action.hover' }} /> : null },
    { field: 'name', headerName: 'Audit', width: 170 },
    { field: 'status', headerName: 'Status', width: 120, cellRenderer: ({ data }: DataGridCellParams<PerfRow>) => data ? <Chip size="small" label={data.status} sx={{ height: 20, fontSize: '0.62rem', textTransform: 'uppercase', bgcolor: `${statusColor(t, data.status)}22`, color: statusColor(t, data.status) }} /> : null },
    { field: 'detail', headerName: 'Detail', flex: 1, minWidth: 280, cellRenderer: ({ value }: DataGridCellParams<PerfRow, string>) => <Typography sx={{ fontSize: '0.86rem' }} noWrap>{value}</Typography> },
    { colId: 'fix', headerName: '', width: 90, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<PerfRow>) => data && !isClosed(data.status) ? <Button size="small" variant="outlined" color="inherit" onClick={(e) => { e.stopPropagation(); void dispatch(`the ${data.name} audit`); }}>Fix</Button> : null },
  ], [t, dispatch]);

  const open = rows.filter((r) => !isClosed(r.status)).length;
  const fix = () => { const n = selected.length || open; void dispatch(`${n} performance issue${n > 1 ? 's' : ''}`); };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Performance</Typography>
            <Chip size="small" label={isLive ? 'live' : 'sample'} sx={(th) => ({ height: 20, fontSize: '0.64rem', fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
            <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{open} audits need work{selected.length > 0 ? ` · ${selected.length} selected` : ''}</Typography>
          </Stack>
          <Button variant="contained" disabled={rows.length === 0} onClick={fix}>{selected.length > 0 ? `Fix selected (${selected.length})` : 'Fix all'}</Button>
        </Stack>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 3 }}>
          {METRICS.map((m) => (
            <Card key={m.label} sx={{ p: 2.5 }}>
              <Stack direction="row" justifyContent="space-between" alignItems="baseline">
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                <Chip size="small" label={m.area} sx={{ height: 16, fontSize: '0.56rem', bgcolor: 'action.hover' }} />
              </Stack>
              <Typography variant="h4" sx={{ fontSize: '1.8rem', mt: 0.5 }}>{m.value}</Typography>
              <Typography sx={{ fontSize: '0.76rem', color: 'text.secondary' }}>{m.sub}</Typography>
            </Card>
          ))}
        </Box>

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Audits</Typography>
        <DataGrid
          rows={rows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No performance audits yet — run the finisher."
          height={420}
          minHeight={260}
          gridOptions={{ rowSelection: { mode: 'multiRow' }, onSelectionChanged: (e) => setSelected(e.api.getSelectedRows()) }}
        />
        <Typography sx={{ fontSize: '0.76rem', color: 'text.disabled', mt: 1.5 }}>Headline metrics are sample until a Lighthouse + load run reports; audits reflect the live perf gates.</Typography>
      </Box>
    </Box>
  );
}
