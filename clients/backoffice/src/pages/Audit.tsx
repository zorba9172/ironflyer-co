import { useMemo, useState } from 'react';
import { Box, Chip, InputBase, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { formatRelativeTime } from '@ironflyer/core';
import { audit, type AuditRow } from '../data';

function decisionColor(t: Theme, d: AuditRow['decision']): string {
  switch (d) {
    case 'allow': return t.palette.success.main;
    case 'deny': return t.palette.error.main;
    default: return t.palette.warning.main;
  }
}

export function Audit() {
  const t = useTheme();
  const [q, setQ] = useState('');

  const rows = useMemo(
    () => audit.filter((a) => `${a.actor} ${a.action} ${a.decision}`.toLowerCase().includes(q.toLowerCase())),
    [q],
  );

  const columns = useMemo<DataGridColumn<AuditRow>[]>(() => [
    { field: 'ts', headerName: 'Time', width: 140, valueFormatter: ({ value }) => formatRelativeTime(Number(value)) },
    { field: 'actor', headerName: 'Actor', width: 180, cellRenderer: ({ value }: DataGridCellParams<AuditRow, string>) => <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.78rem', color: 'text.secondary' })} noWrap>{value}</Typography> },
    { field: 'action', headerName: 'Action', flex: 1, minWidth: 320, cellRenderer: ({ value }: DataGridCellParams<AuditRow, string>) => <Typography sx={{ fontSize: '0.86rem' }} noWrap>{value}</Typography> },
    { field: 'decision', headerName: 'Decision', width: 120, cellRenderer: ({ data }: DataGridCellParams<AuditRow>) => data ? <Chip size="small" label={data.decision} sx={{ height: 20, fontSize: '0.62rem', textTransform: 'uppercase', bgcolor: `${decisionColor(t, data.decision)}22`, color: decisionColor(t, data.decision) }} /> : null },
  ], [t]);

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Audit</Typography>
          <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{rows.length} events · ProfitGuard, gate verdicts, and operator actions</Typography>
        </Stack>

        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, border: 1, borderColor: 'divider', borderRadius: 2, px: 2, py: 1, mb: 2, maxWidth: 420, bgcolor: 'background.paper' }}>
          <Box component="span" sx={{ color: 'text.disabled' }}>⌕</Box>
          <InputBase fullWidth placeholder="Search actor, action, or decision" value={q} onChange={(e) => setQ(e.target.value)} sx={{ fontSize: '0.9rem' }} />
        </Box>

        <DataGrid
          rows={rows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No audit events match your search."
          height={520}
          minHeight={300}
          pagination={rows.length > 12}
          pageSize={12}
          gridOptions={{ suppressPaginationPanel: rows.length <= 12 }}
        />
      </Box>
    </Box>
  );
}
