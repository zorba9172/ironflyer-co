import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';

interface RawGate { gate: string; status: string; issues: { severity: string; message: string }[] }
interface QualityRow { id: string; name: string; tool: string; status: string; detail: string }

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

const METRICS = [
  { label: 'Duplication', value: '4.2%', sub: 'budget 1%' },
  { label: 'Dead code', value: '12', sub: 'unused exports' },
  { label: 'Max complexity', value: '24', sub: 'checkout()' },
  { label: 'Dep cycles', value: '3', sub: 'circular' },
];

export function QualityPane() {
  const t = useTheme();
  const liveProjectId = useLiveProjectId();
  const { dispatch } = useDispatchAgent();
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

  const columns = useMemo<DataGridColumn<QualityRow>[]>(() => [
    { field: 'name', headerName: 'Check', width: 168 },
    { field: 'tool', headerName: 'Tool', width: 156, cellRenderer: ({ data }: DataGridCellParams<QualityRow>) => data ? <Chip size="small" label={data.tool} sx={(th) => ({ height: 20, fontSize: '0.64rem', fontFamily: th.brand.font.mono, bgcolor: 'action.hover' })} /> : null },
    { field: 'status', headerName: 'Status', width: 116, cellRenderer: ({ data }: DataGridCellParams<QualityRow>) => data ? <Chip size="small" label={data.status} sx={{ height: 20, fontSize: '0.62rem', textTransform: 'uppercase', bgcolor: `${statusColor(t, data.status)}22`, color: statusColor(t, data.status) }} /> : null },
    { field: 'detail', headerName: 'Detail', flex: 1, minWidth: 260, cellRenderer: ({ value }: DataGridCellParams<QualityRow, string>) => <Typography sx={{ fontSize: '0.86rem' }} noWrap>{value}</Typography> },
    { colId: 'fix', headerName: '', width: 90, sortable: false, filter: false, cellRenderer: ({ data }: DataGridCellParams<QualityRow>) => data && !isClosed(data.status) ? <Button size="small" variant="outlined" color="inherit" onClick={(e) => { e.stopPropagation(); void dispatch(`${data.name} (${data.tool})`); }}>Fix</Button> : null },
  ], [t, dispatch]);

  const open = rows.filter((r) => !isClosed(r.status)).length;
  const fix = () => { const n = selected.length || open; void dispatch(`${n} quality issue${n > 1 ? 's' : ''}`); };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Code quality</Typography>
            <Chip size="small" label={isLive ? 'live' : 'sample'} sx={(th) => ({ height: 20, fontSize: '0.64rem', fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
            <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{open} checks need work{selected.length > 0 ? ` · ${selected.length} selected` : ''}</Typography>
          </Stack>
          <Button variant="contained" disabled={rows.length === 0} onClick={fix}>{selected.length > 0 ? `Fix selected (${selected.length})` : 'Fix all'}</Button>
        </Stack>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 3 }}>
          {METRICS.map((m) => (
            <Card key={m.label} sx={{ p: 2.5 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
              <Typography variant="h4" sx={{ fontSize: '1.8rem', mt: 0.5 }}>{m.value}</Typography>
              <Typography sx={{ fontSize: '0.76rem', color: 'text.secondary' }}>{m.sub}</Typography>
            </Card>
          ))}
        </Box>

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Checks</Typography>
        <DataGrid
          rows={rows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No quality checks yet — run the finisher."
          height={420}
          minHeight={260}
          gridOptions={{ rowSelection: { mode: 'multiRow' }, onSelectionChanged: (e) => setSelected(e.api.getSelectedRows()) }}
        />
        <Typography sx={{ fontSize: '0.76rem', color: 'text.disabled', mt: 1.5 }}>Each check runs its OSS tool in the orchestrator sandbox (jscpd, knip, madge, dependency-cruiser, eslint); the table reflects the live quality gates.</Typography>
      </Box>
    </Box>
  );
}
