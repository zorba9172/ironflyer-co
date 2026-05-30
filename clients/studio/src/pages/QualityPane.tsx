import { useMemo, useState } from 'react';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useStudio } from '../store';
import { TechIcon } from '../lib/techIcons';
import { StudioChart, donutOption, horizontalBarOption, type EChartsOption } from '../components/charts';
import { StudioDataGrid, type DataGridCellParams, type DataGridColumn, type StudioTableTab } from '../components/tables';
import { GlassPanel, SectionHeader } from '../components/studio';
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
  const [tableView, setTableView] = useState('open');
  const [tableSearch, setTableSearch] = useState('');

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
  const tableTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'open', label: 'Open', count: open, tone: open > 0 ? 'warning' : 'success' },
    { value: 'blocked', label: 'Blocked', count: blockedChecks, tone: blockedChecks > 0 ? 'error' : 'default' },
    { value: 'passed', label: 'Passed', count: passedChecks, tone: 'success' },
    { value: 'all', label: 'All', count: rows.length },
  ], [open, blockedChecks, passedChecks, rows.length]);
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

  const tableRows = useMemo(() => {
    const q = tableSearch.trim().toLowerCase();
    return rows.filter((row) => {
      if (tableView === 'open' && isClosed(row.status)) return false;
      if (tableView === 'blocked' && !['blocked', 'fail'].includes(row.status.toLowerCase())) return false;
      if (tableView === 'passed' && !isClosed(row.status)) return false;
      if (!q) return true;
      return [row.name, row.tool, row.status, row.detail].some((value) => value.toLowerCase().includes(q));
    });
  }, [rows, tableView, tableSearch]);

  const fix = () => {
    const scopeRows = selected.length > 0 ? selected : priorityRows;
    const names = scopeRows.map((r) => r.name).join(', ');
    const n = scopeRows.length || open;
    void dispatch(names ? `quality checks: ${names}` : `${n} quality issue${n > 1 ? 's' : ''}`);
  };

  const barsData = useMemo<EChartsOption>(() => horizontalBarOption(t, {
    labels: rows.map((r) => r.name),
    values: rows.map((r) => isClosed(r.status) ? 20 : r.status.toLowerCase() === 'running' ? 60 : 100),
    colors: rows.map((r) => isClosed(r.status) ? t.palette.success.main : r.status.toLowerCase() === 'blocked' ? t.palette.error.main : t.palette.warning.main),
  }), [rows, t]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1100, mx: 'auto' }}>
        <SectionHeader
          eyebrow="Quality gates"
          title="Code quality"
          subtitle={`${open} checks need work · scanner: ${scannerState}`}
          actions={
            <Stack direction="row" spacing={1}>
              <Chip
                size="small"
                label={isLive ? 'live' : 'sample'}
                sx={(th) => ({ height: 20, fontSize: text.s64, fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
              />
              {open > 0 && <Button variant="outlined" color="inherit" onClick={() => void repairGate(priorityRows[0]?.id ?? 'lint', priorityRows[0]?.name ?? 'Quality')}>Re-run next</Button>}
              <Button variant="contained" disabled={rows.length === 0 || open === 0} onClick={fix}>{selected.length > 0 ? `Fix selected (${selected.length})` : 'Fix open'}</Button>
            </Stack>
          }
        />

        {/* Visual-first band: donut (verdicts) + flat severity bars + metrics */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '260px 1fr 240px' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
          <GlassPanel pad={2} accent={open > 0 ? t.studio.neon.warning : t.studio.neon.success}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Check verdicts</Typography>
            <StudioChart option={statusDonut} height={200} />
          </GlassPanel>

          <GlassPanel pad={2} accent={t.studio.neon.violet}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Gate severity</Typography>
            <StudioChart option={barsData} height={220} />
          </GlassPanel>

          <Box sx={{ display: 'grid', gridTemplateRows: 'repeat(2, 1fr)', gap: 1.5 }}>
            {metrics.slice(0, 2).map((m) => (
              <GlassPanel key={m.label} pad={2} sx={{ display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                <Typography variant="h4" sx={{ fontSize: text.s160, mt: 0.5 }}>{m.value}</Typography>
                <Typography sx={{ fontSize: text.s76, color: 'text.secondary' }}>{m.sub}</Typography>
              </GlassPanel>
            ))}
          </Box>
        </Box>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr' }, gap: 1.5, mb: 3 }}>
          {metrics.slice(2).map((m) => (
            <GlassPanel key={m.label} pad={2} sx={{ display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
              <Typography variant="h4" sx={{ fontSize: text.s160, mt: 0.5 }}>{m.value}</Typography>
              <Typography sx={{ fontSize: text.s76, color: 'text.secondary' }}>{m.sub}</Typography>
            </GlassPanel>
          ))}
        </Box>

        {priorityRows.length > 0 && (
          <GlassPanel pad={2} accent={t.studio.neon.warning} sx={{ mb: 3 }}>
            <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Open review queue</Typography>
              <Chip size="small" label={priorityRows.length} sx={(th) => ({ height: 18, fontSize: text.s62, bgcolor: `${th.palette.warning.main}22`, color: 'warning.main' })} />
            </Stack>
            <Stack spacing={0.75}>
              {priorityRows.map((r) => (
                <Stack key={r.id} direction="row" alignItems="center" spacing={1} sx={{ p: 1, borderRadius: 1.5, bgcolor: 'action.hover' }}>
                  <Chip size="small" label={r.tool} sx={(th) => ({ height: 20, fontSize: text.s62, fontFamily: th.brand.font.mono, bgcolor: 'background.paper' })} />
                  <Typography sx={{ fontSize: text.s86, fontWeight: 600, minWidth: 110 }} noWrap>{r.name}</Typography>
                  <Typography sx={{ fontSize: text.s82, color: 'text.secondary', flex: 1 }} noWrap>{r.detail}</Typography>
                  <Button size="small" variant="outlined" color="inherit" onClick={() => void repairGate(r.id, r.name)}>Re-run</Button>
                  <Button size="small" variant="contained" onClick={() => void dispatch(`quality check ${r.name}: ${r.detail}`)}>Fix</Button>
                </Stack>
              ))}
            </Stack>
          </GlassPanel>
        )}

        <StudioDataGrid
          title="Quality checks"
          subtitle="Grouped by gate state with row selection for batch repairs."
          tabs={tableTabs}
          activeTab={tableView}
          onTabChange={setTableView}
          searchValue={tableSearch}
          onSearchChange={setTableSearch}
          searchPlaceholder="Search checks"
          footer="Each check runs its OSS tool in the orchestrator sandbox; the table reflects live quality gates."
          rows={tableRows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No quality checks yet — run the finisher."
          height={420}
          minHeight={260}
          gridOptions={{ rowSelection: { mode: 'multiRow' }, onSelectionChanged: (e) => setSelected(e.api.getSelectedRows()) }}
        />
      </Box>
    </Box>
  );
}
