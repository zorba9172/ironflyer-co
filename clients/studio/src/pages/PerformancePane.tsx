import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useStudio } from '../store';
import { TechIcon } from '../lib/techIcons';

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


interface Forecast { level: string; burnRatePerHourUSD: number; extrapolatedTotalUSD: number; remainingHeadroomUSD: number }

export function PerformancePane() {
  const t = useTheme();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const liveProjectId = storeProjectId ?? firstProjectId;
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

  const { data: forecast } = useGraphQLQuery<Forecast, { sentinelForecast: Forecast }>({
    key: ['sentinel', liveProjectId ?? 'none'], operationName: 'SentinelForecast', query: operations.SENTINEL_FORECAST,
    variables: { projectId: liveProjectId }, fallbackData: { level: 'green', burnRatePerHourUSD: 0, extrapolatedTotalUSD: 0, remainingHeadroomUSD: 0 },
    enabled: !!liveProjectId, map: (r) => r.sentinelForecast,
  });

  // Headline tiles derived from the live perf gates + the Sentinel burn forecast.
  const fe = rows.filter((r) => r.area === 'Frontend');
  const be = rows.filter((r) => r.area === 'Backend');
  const passed = rows.filter((r) => isClosed(r.status)).length;
  const metrics = [
    { label: 'Audits passing', value: `${passed}/${rows.length}`, sub: 'performance gates', area: 'All' },
    { label: 'Frontend', value: `${fe.filter((r) => isClosed(r.status)).length}/${fe.length}`, sub: 'lighthouse · bundle · TBT', area: 'Frontend' },
    { label: 'Backend', value: `${be.filter((r) => isClosed(r.status)).length}/${be.length}`, sub: 'complexity · cycles · leaks', area: 'Backend' },
    { label: 'Burn rate', value: `$${forecast.burnRatePerHourUSD.toFixed(2)}/h`, sub: forecast.level === 'green' ? 'on budget' : `${forecast.level} · ${forecast.remainingHeadroomUSD < 0 ? 'over' : '$' + forecast.remainingHeadroomUSD.toFixed(2) + ' headroom'}`, area: 'Spend' },
  ];

  const columns = useMemo<DataGridColumn<PerfRow>[]>(() => [
    { field: 'area', headerName: 'Area', width: 132, cellRenderer: ({ data }: DataGridCellParams<PerfRow>) => data ? (
      <Stack direction="row" alignItems="center" spacing={0.75}>
        <TechIcon name={data.area} size={15} title={`${data.area} (${data.area === 'Frontend' ? 'React' : 'Go'})`} />
        <Typography sx={{ fontSize: '0.82rem' }}>{data.area}</Typography>
      </Stack>
    ) : null },
    { field: 'name', headerName: 'Audit', width: 190, cellRenderer: ({ data }: DataGridCellParams<PerfRow>) => data ? (
      <Stack direction="row" alignItems="center" spacing={0.85}>
        <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}><TechIcon name={data.id} size={15} title={data.name} /></Box>
        <Typography sx={{ fontSize: '0.86rem' }} noWrap>{data.name}</Typography>
      </Stack>
    ) : null },
    { field: 'status', headerName: 'Status', width: 120, cellRenderer: ({ data }: DataGridCellParams<PerfRow>) => data ? <Chip size="small" label={data.status} sx={{ height: 20, fontSize: '0.62rem', textTransform: 'uppercase', bgcolor: `${statusColor(t, data.status)}22`, color: statusColor(t, data.status) }} /> : null },
    { field: 'detail', headerName: 'Detail', flex: 1, minWidth: 280, cellRenderer: ({ value }: DataGridCellParams<PerfRow, string>) => <Typography sx={{ fontSize: '0.86rem' }} noWrap>{value}</Typography> },
    { colId: 'fix', headerName: '', width: 90, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<PerfRow>) => data && !isClosed(data.status) ? <Button size="small" variant="outlined" color="inherit" onClick={(e) => { e.stopPropagation(); void dispatch(`the ${data.name} audit`); }}>Fix</Button> : null },
  ], [t, dispatch]);

  const open = rows.filter((r) => !isClosed(r.status)).length;
  const fix = () => { const n = selected.length || open; void dispatch(`${n} performance issue${n > 1 ? 's' : ''}`); };

  // Headline visual — passing vs open audits per area, so the operator sees at
  // a glance which layer (frontend / backend) is still blocking the budget.
  const areaBar = useMemo<EChartsOption>(() => {
    const fePass = fe.filter((r) => isClosed(r.status)).length;
    const bePass = be.filter((r) => isClosed(r.status)).length;
    return {
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      legend: { bottom: 0, textStyle: { color: t.palette.text.secondary, fontSize: 11 } },
      grid: { left: 8, right: 16, top: 12, bottom: 28, containLabel: true },
      xAxis: { type: 'value', axisLabel: { color: t.palette.text.secondary }, splitLine: { lineStyle: { color: t.palette.divider } } },
      yAxis: { type: 'category', data: ['Backend', 'Frontend'], axisLabel: { color: t.palette.text.secondary } },
      series: [
        { name: 'Passing', type: 'bar', stack: 'a', itemStyle: { color: t.palette.success.main }, data: [bePass, fePass] },
        { name: 'Needs work', type: 'bar', stack: 'a', itemStyle: { color: t.palette.warning.main }, data: [be.length - bePass, fe.length - fePass] },
      ],
    };
  }, [fe, be, t]);

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

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '340px 1fr' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Audits by layer</Typography>
            <Chart option={areaBar} height={200} />
          </Card>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr' }, gap: 1.5 }}>
            {metrics.map((m) => (
              <Card key={m.label} sx={{ p: 2.5, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <Stack direction="row" justifyContent="space-between" alignItems="baseline">
                  <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                  <Chip size="small" label={m.area} sx={{ height: 16, fontSize: '0.56rem', bgcolor: 'action.hover' }} />
                </Stack>
                <Typography variant="h4" sx={{ fontSize: '1.8rem', mt: 0.5 }}>{m.value}</Typography>
                <Typography sx={{ fontSize: '0.76rem', color: 'text.secondary' }}>{m.sub}</Typography>
              </Card>
            ))}
          </Box>
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
        <Typography sx={{ fontSize: '0.76rem', color: 'text.disabled', mt: 1.5 }}>Tiles summarize the live perf gates and the Sentinel burn forecast; the table reflects each gate verdict.</Typography>
      </Box>
    </Box>
  );
}
