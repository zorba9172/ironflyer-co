import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useStudio } from '../store';
import { TechIcon } from '../lib/techIcons';
import { StudioChart, donutOption, type EChartsOption } from '../components/charts';
import { StudioDataGrid, type DataGridCellParams, type DataGridColumn } from '../components/tables';
import { text } from '@ironflyer/design-tokens/brand';

interface RawGate { gate: string; status: string; issues: { severity: string; message: string }[] }
interface QualityRow { id: string; name: string; tool: string; status: string; detail: string }
interface Health { reuseRate: number; dedupRate: number; deadCodeCount: number; dependencyCycles: number; locPerCapability: number; atlasCapabilityCount: number }

// Code-quality gates and the OSS library each runs in the orchestrator.
const QUALITY_TOOL: Record<string, string> = {
  dedup: 'jscpd',
  reuse_check: 'jscpd',
  deadcode: 'knip',
  complexity: 'ts-morph',
  dep_graph: 'madge',
  arch_boundary: 'dependency-cruiser',
  lint: 'eslint',
};
const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
const isClosed = (s: string) => ['pass', 'passed'].includes(s.toLowerCase());
function statusColor(t: Theme, s: string): string {
  if (isClosed(s)) return t.palette.success.main;
  if (s.toLowerCase() === 'running') return t.brand.accent.secondary;
  if (['blocked', 'fail'].includes(s.toLowerCase())) return t.palette.error.main;
  return t.palette.warning.main;
}

const SAMPLE: QualityRow[] = [
  { id: 'dedup', name: 'Dedup', tool: 'jscpd', status: 'blocked', detail: '4.2% duplicated lines (budget 1%)' },
  { id: 'deadcode', name: 'Deadcode', tool: 'knip', status: 'blocked', detail: '12 unused exports across 6 files' },
  { id: 'complexity', name: 'Complexity', tool: 'ts-morph', status: 'blocked', detail: 'checkout() cyclomatic complexity 24' },
  { id: 'dep_graph', name: 'Dep Graph', tool: 'madge', status: 'blocked', detail: '3 circular dependencies' },
  { id: 'arch_boundary', name: 'Arch Boundary', tool: 'dependency-cruiser', status: 'blocked', detail: 'ui imports from server layer' },
  { id: 'reuse_check', name: 'Reuse Check', tool: 'jscpd', status: 'passed', detail: 'No copy-paste blocks found' },
];

export function QualityPane() {
  const t = useTheme();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const liveProjectId = storeProjectId ?? firstProjectId;
  const { dispatch, repairGate } = useDispatchAgent();
  const [selected, setSelected] = useState<QualityRow[]>([]);

  const { data: rows, isLive } = useGraphQLQuery<QualityRow[], { gates: RawGate[] }>({
    key: ['quality-gates', liveProjectId ?? 'none'],
    operationName: 'Gates', query: operations.GATES,
    variables: { projectId: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    map: (r) => {
      const q = r.gates.filter((g) => QUALITY_TOOL[g.gate]);
      if (!q.length) return SAMPLE;
      return q.map((g) => ({
        id: g.gate, name: titleCase(g.gate), tool: QUALITY_TOOL[g.gate]!, status: g.status,
        detail: isClosed(g.status) ? 'Clean' : g.issues[0]?.message ?? 'Needs cleanup',
      }));
    },
  });

  // Real Code-Health metrics when the anti-bloat report exists (values ≥ 0);
  // otherwise derive each tile from the live gate verdict so it's never faked.
  const { data: health } = useGraphQLQuery<Health, { healthDashboard: Health }>({
    key: ['health-dashboard'], operationName: 'HealthDashboard', query: operations.HEALTH_DASHBOARD,
    fallbackData: { reuseRate: -1, dedupRate: -1, deadCodeCount: -1, dependencyCycles: -1, locPerCapability: 0, atlasCapabilityCount: 0 },
    map: (r) => r.healthDashboard,
  });
  const gateClean = (id: string) => { const g = rows.find((r) => r.id === id); return g ? (isClosed(g.status) ? 'clean' : 'review') : 'n/a'; };
  const passedChecks = rows.filter((r) => isClosed(r.status)).length;
  const blockedChecks = rows.filter((r) => ['blocked', 'fail'].includes(r.status.toLowerCase())).length;
  const runningChecks = rows.filter((r) => r.status.toLowerCase() === 'running').length;
  const open = rows.length - passedChecks;
  const priorityRows = rows.filter((r) => !isClosed(r.status)).slice(0, 4);
  const scannerState = runningChecks > 0 ? `${runningChecks} running` : open > 0 ? `${open} need review` : 'clean';
  const metrics = [
    { label: 'Duplication', value: health.dedupRate >= 0 ? `${(health.dedupRate * 100).toFixed(1)}%` : gateClean('dedup'), sub: 'jscpd' },
    { label: 'Dead code', value: health.deadCodeCount >= 0 ? String(health.deadCodeCount) : gateClean('deadcode'), sub: 'unused exports (knip)' },
    { label: 'Dep cycles', value: health.dependencyCycles >= 0 ? String(health.dependencyCycles) : gateClean('dep_graph'), sub: 'circular (madge)' },
    { label: 'Checks passing', value: `${passedChecks}/${rows.length}`, sub: 'quality gates' },
  ];

  // Headline visual — mirrors the live quality-gate verdicts. The center
  // names what is still open end-to-end so the operator reads it in one glance.
  const statusDonut = useMemo<EChartsOption>(() => {
    const review = Math.max(0, rows.length - passedChecks - blockedChecks);
    const data = [
      { value: passedChecks, name: 'Clean', itemStyle: { color: t.palette.success.main } },
      { value: review, name: 'Review', itemStyle: { color: t.palette.warning.main } },
      { value: blockedChecks, name: 'Blocked', itemStyle: { color: t.palette.error.main } },
    ].filter((d) => d.value > 0);
    return donutOption(t, {
      data: data.map((d) => ({ value: d.value, name: d.name, color: d.itemStyle.color })),
      centerLabel: open > 0 ? `${open}\nopen` : 'all\nclean',
      centerColor: open > 0 ? t.palette.warning.main : t.palette.success.main,
    });
  }, [rows.length, passedChecks, blockedChecks, open, t]);

  const columns = useMemo<DataGridColumn<QualityRow>[]>(() => [
    { field: 'name', headerName: 'Check', width: 184, cellRenderer: ({ data }: DataGridCellParams<QualityRow>) => data ? (
      <Stack direction="row" alignItems="center" spacing={0.85}>
        <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}><TechIcon name={data.id} size={15} title={data.name} /></Box>
        <Typography sx={{ fontSize: text.s86 }} noWrap>{data.name}</Typography>
      </Stack>
    ) : null },
    { field: 'tool', headerName: 'Tool', width: 168, cellRenderer: ({ data }: DataGridCellParams<QualityRow>) => data ? (
      <Stack direction="row" alignItems="center" spacing={0.75}>
        <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}><TechIcon name={data.tool} size={14} title={data.tool} /></Box>
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s74, color: 'text.secondary' })}>{data.tool}</Typography>
      </Stack>
    ) : null },
    { field: 'status', headerName: 'Status', width: 116, cellRenderer: ({ data }: DataGridCellParams<QualityRow>) => data ? <Chip size="small" label={data.status} sx={{ height: 20, fontSize: text.s62, textTransform: 'uppercase', bgcolor: `${statusColor(t, data.status)}22`, color: statusColor(t, data.status) }} /> : null },
    { field: 'detail', headerName: 'Detail', flex: 1, minWidth: 260, cellRenderer: ({ value }: DataGridCellParams<QualityRow, string>) => <Typography sx={{ fontSize: text.s86 }} noWrap>{value}</Typography> },
    { colId: 'fix', headerName: '', width: 104, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<QualityRow>) => data && !isClosed(data.status) ? <Button size="small" variant="outlined" color="inherit" onClick={(e) => { e.stopPropagation(); void repairGate(data.id, data.name); }}>Re-run</Button> : null },
  ], [t, repairGate]);

  const fix = () => {
    const scopeRows = selected.length > 0 ? selected : priorityRows;
    const names = scopeRows.map((r) => r.name).join(', ');
    const n = scopeRows.length || open;
    void dispatch(names ? `quality checks: ${names}` : `${n} quality issue${n > 1 ? 's' : ''}`);
  };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: text.s160 }}>Code quality</Typography>
            <Chip size="small" label={isLive ? 'live' : 'sample'} sx={(th) => ({ height: 20, fontSize: text.s64, fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
            <Chip size="small" label={`scanner: ${scannerState}`} sx={(th) => ({ height: 20, fontSize: text.s64, fontFamily: th.brand.font.mono, bgcolor: open ? `${th.palette.warning.main}22` : `${th.palette.success.main}22`, color: open ? 'warning.main' : 'success.main' })} />
            <Typography sx={{ color: 'text.secondary', fontSize: text.s90 }}>{open} checks need work{selected.length > 0 ? ` · ${selected.length} selected` : ''}</Typography>
          </Stack>
          <Stack direction="row" spacing={1}>
            {open > 0 && <Button variant="outlined" color="inherit" onClick={() => void repairGate(priorityRows[0]?.id ?? 'lint', priorityRows[0]?.name ?? 'Quality')}>Re-run next</Button>}
            <Button variant="contained" disabled={rows.length === 0 || open === 0} onClick={fix}>{selected.length > 0 ? `Fix selected (${selected.length})` : 'Fix open'}</Button>
          </Stack>
        </Stack>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '300px 1fr' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Check verdicts</Typography>
            <StudioChart option={statusDonut} height={200} />
          </Card>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr' }, gap: 1.5 }}>
            {metrics.map((m) => (
              <Card key={m.label} sx={{ p: 2.5, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                <Typography variant="h4" sx={{ fontSize: text.s180, mt: 0.5 }}>{m.value}</Typography>
                <Typography sx={{ fontSize: text.s76, color: 'text.secondary' }}>{m.sub}</Typography>
              </Card>
            ))}
          </Box>
        </Box>

        {priorityRows.length > 0 && (
          <Card sx={{ p: 2, mb: 3 }}>
            <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Open review queue</Typography>
              <Chip size="small" label={priorityRows.length} sx={(th) => ({ height: 18, fontSize: text.s62, bgcolor: `${th.palette.warning.main}22`, color: 'warning.main' })} />
            </Stack>
            <Stack spacing={0.75}>
              {priorityRows.map((r) => (
                <Stack key={r.id} direction="row" alignItems="center" spacing={1}>
                  <Chip size="small" label={r.tool} sx={(th) => ({ height: 20, fontSize: text.s62, fontFamily: th.brand.font.mono, bgcolor: 'action.hover' })} />
                  <Typography sx={{ fontSize: text.s86, fontWeight: 600, minWidth: 110 }} noWrap>{r.name}</Typography>
                  <Typography sx={{ fontSize: text.s82, color: 'text.secondary', flex: 1 }} noWrap>{r.detail}</Typography>
                  <Button size="small" variant="outlined" color="inherit" onClick={() => void repairGate(r.id, r.name)}>Re-run</Button>
                  <Button size="small" variant="contained" onClick={() => void dispatch(`quality check ${r.name}: ${r.detail}`)}>Fix</Button>
                </Stack>
              ))}
            </Stack>
          </Card>
        )}

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Checks</Typography>
        <StudioDataGrid
          rows={rows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No quality checks yet — run the finisher."
          height={420}
          minHeight={260}
          gridOptions={{ rowSelection: { mode: 'multiRow' }, onSelectionChanged: (e) => setSelected(e.api.getSelectedRows()) }}
        />
        <Typography sx={{ fontSize: text.s76, color: 'text.disabled', mt: 1.5 }}>Each check runs its OSS tool in the orchestrator sandbox (jscpd, knip, madge, dependency-cruiser, eslint); the table reflects the live quality gates.</Typography>
      </Box>
    </Box>
  );
}
