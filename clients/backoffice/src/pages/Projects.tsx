import { useMemo, useState } from 'react';
import { Box, Button, Chip, InputBase, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { toast } from '@ironflyer/ui-web/fx';
import { formatRelativeTime } from '@ironflyer/core';
import { projects as SAMPLE, type ProjectRow } from '../data';

interface RawProject { id: string; name: string; status: string; updatedAt: string }

function statusColor(t: Theme, s: ProjectRow['status']): string {
  switch (s) {
    case 'shipped': return t.palette.success.main;
    case 'running': return t.brand.accent.secondary;
    case 'blocked': return t.palette.error.main;
    default: return t.palette.text.disabled;
  }
}

export function Projects() {
  const t = useTheme();
  const [q, setQ] = useState('');
  const [selected, setSelected] = useState<ProjectRow[]>([]);

  // Overlay live projects when an orchestrator is reachable; fall back to seed.
  const { data: all, isLive } = useGraphQLQuery<ProjectRow[], { projects: RawProject[] }>({
    key: ['bo-projects'],
    operationName: 'Projects', query: operations.PROJECTS,
    fallbackData: SAMPLE,
    map: (r) => {
      if (!r.projects?.length) return SAMPLE;
      return r.projects.map((p, i) => ({
        id: p.id,
        name: p.name,
        owner: SAMPLE[i % SAMPLE.length]!.owner,
        status: (['shipped', 'running', 'blocked', 'draft'].includes(p.status) ? p.status : 'running') as ProjectRow['status'],
        gatesOpen: SAMPLE[i % SAMPLE.length]!.gatesOpen,
        updatedAt: Date.parse(p.updatedAt) || Date.now(),
      }));
    },
  });

  const rows = useMemo(
    () => all.filter((p) => `${p.name} ${p.owner}`.toLowerCase().includes(q.toLowerCase())),
    [all, q],
  );

  const columns = useMemo<DataGridColumn<ProjectRow>[]>(() => [
    { field: 'name', headerName: 'Project', flex: 1, minWidth: 200, cellRenderer: ({ value }: DataGridCellParams<ProjectRow, string>) => <Typography sx={{ fontSize: '0.88rem', fontWeight: 600 }} noWrap>{value}</Typography> },
    { field: 'owner', headerName: 'Owner', width: 220, cellRenderer: ({ value }: DataGridCellParams<ProjectRow, string>) => <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.78rem', color: 'text.secondary' })} noWrap>{value}</Typography> },
    { field: 'status', headerName: 'Status', width: 120, cellRenderer: ({ data }: DataGridCellParams<ProjectRow>) => data ? <Chip size="small" label={data.status} sx={{ height: 20, fontSize: '0.62rem', textTransform: 'uppercase', bgcolor: `${statusColor(t, data.status)}22`, color: statusColor(t, data.status) }} /> : null },
    { field: 'gatesOpen', headerName: 'Gates open', width: 120, cellRenderer: ({ data }: DataGridCellParams<ProjectRow>) => data ? <Typography sx={{ fontSize: '0.86rem', color: data.gatesOpen > 0 ? 'warning.main' : 'text.disabled' }}>{data.gatesOpen}</Typography> : null },
    { field: 'updatedAt', headerName: 'Updated', width: 140, valueFormatter: ({ value }) => formatRelativeTime(Number(value)) },
  ], [t]);

  const bulk = () => {
    const n = selected.length;
    toast(n ? `Queued re-run for ${n} project${n > 1 ? 's' : ''}.` : 'Select projects to act on.', n ? 'success' : 'info');
  };

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Projects</Typography>
            <Chip size="small" label={isLive ? 'live' : 'sample'} sx={(th) => ({ height: 20, fontSize: '0.64rem', fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
            <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{rows.length} projects{selected.length > 0 ? ` · ${selected.length} selected` : ''}</Typography>
          </Stack>
          <Button variant="contained" disabled={selected.length === 0} onClick={bulk}>Re-run selected{selected.length > 0 ? ` (${selected.length})` : ''}</Button>
        </Stack>

        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, border: 1, borderColor: 'divider', borderRadius: 2, px: 2, py: 1, mb: 2, maxWidth: 420, bgcolor: 'background.paper' }}>
          <Box component="span" sx={{ color: 'text.disabled' }}>⌕</Box>
          <InputBase fullWidth placeholder="Search projects or owners" value={q} onChange={(e) => setQ(e.target.value)} sx={{ fontSize: '0.9rem' }} />
        </Box>

        <DataGrid
          rows={rows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No projects match your search."
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
