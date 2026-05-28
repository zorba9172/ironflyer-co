import { useMemo, useState } from 'react';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { useRunProjectFeed } from '@ironflyer/data';
import { formatRelativeTime } from '@ironflyer/core';
import type { ActivityEvent, StudioProject } from '../studioData';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';

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
  const liveProjectId = useLiveProjectId();
  const { dispatch } = useDispatchAgent();
  const { events: liveEvents, isLive } = useRunProjectFeed(liveProjectId);
  const rows = useMemo(
    () => [...liveEvents, ...fallback.activity].sort((a, b) => b.ts - a.ts) as ActivityEvent[],
    [liveEvents, fallback.activity],
  );

  const columns = useMemo<DataGridColumn<ActivityEvent>[]>(() => [
    {
      field: 'kind', headerName: 'Source', width: 132,
      cellRenderer: ({ data }: DataGridCellParams<ActivityEvent>) =>
        data ? <Chip size="small" label={data.kind} sx={{ height: 20, fontSize: '0.64rem', textTransform: 'uppercase', bgcolor: `${kindColor(t, data.kind)}22`, color: kindColor(t, data.kind) }} /> : null,
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

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Logs</Typography>
            <Chip size="small" label={isLive ? 'live' : 'seed'} sx={(th) => ({ height: 20, fontSize: '0.64rem', fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
            <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>
              {rows.length} events{selected.length > 0 ? ` · ${selected.length} selected` : ''}
            </Typography>
          </Stack>
          <Button variant="contained" disabled={rows.length === 0} onClick={fixSelected}>
            {selected.length > 0 ? `Fix selected (${selected.length})` : 'Fix all'}
          </Button>
        </Stack>

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
