import { useMemo, useState } from 'react';
import { Box, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { FlowCanvas, type FlowNode, type FlowEdge, type HandleSpec } from '@ironflyer/ui-web/fx';
import { DataGrid, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';

interface AppColumn { name: string; type: string; nullable: boolean; primaryKey: boolean; references: string | null }
interface AppTable { name: string; rowCount: number; columns: AppColumn[] }
interface TableRows { table: string; columns: string[]; rows: Record<string, unknown>[]; total: number }

const SAMPLE: AppTable[] = [
  { name: 'users', rowCount: 1284, columns: [{ name: 'id', type: 'uuid', nullable: false, primaryKey: true, references: null }, { name: 'email', type: 'text', nullable: false, primaryKey: false, references: null }, { name: 'role', type: 'text', nullable: false, primaryKey: false, references: null }] },
  { name: 'orders', rowCount: 5471, columns: [{ name: 'id', type: 'uuid', nullable: false, primaryKey: true, references: null }, { name: 'user_id', type: 'uuid', nullable: false, primaryKey: false, references: 'users.id' }, { name: 'total_cents', type: 'integer', nullable: false, primaryKey: false, references: null }] },
  { name: 'products', rowCount: 212, columns: [{ name: 'id', type: 'uuid', nullable: false, primaryKey: true, references: null }, { name: 'title', type: 'text', nullable: false, primaryKey: false, references: null }] },
];

const H_LEFT: HandleSpec[] = [{ id: 't', type: 'target', side: 'left' }];
const H_RIGHT: HandleSpec[] = [{ id: 's', type: 'source', side: 'right' }];
const H_BOTH: HandleSpec[] = [{ id: 't', type: 'target', side: 'left' }, { id: 's', type: 'source', side: 'right' }];

function TableCard({ t, accent, muted }: { t: AppTable; accent: string; muted: string }) {
  return (
    <Box sx={{ minWidth: 188 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 0.75 }}>
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.82rem', fontWeight: 700, color: accent })}>{t.name}</Typography>
        <Typography sx={{ fontSize: '0.66rem', color: 'text.disabled' }}>{t.rowCount.toLocaleString()} rows</Typography>
      </Stack>
      <Stack spacing={0.25}>
        {t.columns.map((c) => (
          <Stack key={c.name} direction="row" alignItems="center" spacing={0.5} sx={{ justifyContent: 'space-between' }}>
            <Stack direction="row" alignItems="center" spacing={0.5}>
              {c.primaryKey && <Box sx={{ fontSize: '0.6rem', color: accent }}>◆</Box>}
              {c.references && <Box sx={{ fontSize: '0.6rem', color: muted }}>↗</Box>}
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem' })}>{c.name}</Typography>
            </Stack>
            <Typography sx={{ fontSize: '0.62rem', color: 'text.disabled' }}>{c.type}</Typography>
          </Stack>
        ))}
      </Stack>
    </Box>
  );
}

export function DataPane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const [selected, setSelected] = useState<string | null>(null);

  const { data: tables, isLive } = useGraphQLQuery<AppTable[], { appDataSchema: AppTable[] }>({
    key: ['app-data-schema', liveProjectId ?? 'none'],
    operationName: 'AppDataSchema', query: operations.APP_DATA_SCHEMA,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    map: (r) => (r.appDataSchema?.length ? r.appDataSchema : SAMPLE),
  });

  const activeTable = selected ?? tables[0]?.name ?? null;
  const { data: rowsData } = useGraphQLQuery<TableRows, { appTableRows: TableRows }>({
    key: ['app-table-rows', liveProjectId ?? 'none', activeTable ?? 'none'],
    operationName: 'AppTableRows', query: operations.APP_TABLE_ROWS,
    variables: { projectID: liveProjectId, table: activeTable, limit: 50 },
    fallbackData: { table: '', columns: [], rows: [], total: 0 },
    enabled: !!liveProjectId && !!activeTable,
    map: (r) => r.appTableRows,
  });

  const accent = t.brand.accent.primary;
  const muted = t.palette.text.secondary;

  const { nodes, edges } = useMemo(() => {
    const ns: FlowNode[] = tables.map((tbl, i) => {
      const hasRef = tbl.columns.some((c) => c.references);
      const isRefd = tables.some((o) => o.columns.some((c) => c.references?.startsWith(tbl.name + '.')));
      const handles = hasRef && isRefd ? H_BOTH : hasRef ? H_RIGHT : H_LEFT;
      return {
        id: tbl.name, type: 'card',
        position: { x: (i % 2) * 320, y: Math.floor(i / 2) * 220 },
        data: { label: <TableCard t={tbl} accent={accent} muted={muted} />, handles, tone: accent },
      };
    });
    const es: FlowEdge[] = [];
    tables.forEach((tbl) => {
      tbl.columns.forEach((c) => {
        if (!c.references) return;
        const target = c.references.split('.')[0] ?? '';
        if (target && tables.some((o) => o.name === target)) {
          es.push({ id: `${tbl.name}-${c.name}-${target}`, source: tbl.name, target, sourceHandle: 's', targetHandle: 't', animated: false, style: { stroke: muted } });
        }
      });
    });
    return { nodes: ns, edges: es };
  }, [tables, accent, muted]);

  const gridColumns = useMemo<DataGridColumn<Record<string, unknown>>[]>(
    () => (rowsData.columns ?? []).map((c) => ({ field: c, headerName: c, flex: 1, minWidth: 120 })),
    [rowsData.columns],
  );

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1180, mx: 'auto' }}>
        <PaneHeader title="Data" isLive={isLive} subtitle={`${tables.length} tables · schema changes flow through the finisher, never ad-hoc DDL`} />

        <Card sx={{ p: 1, mb: 2, height: 360 }}>
          <FlowCanvas nodes={nodes} edges={edges} horizontal minimap fitViewPadding={0.2} />
        </Card>

        <Stack direction="row" spacing={1} sx={{ mb: 1.5, flexWrap: 'wrap', gap: 1 }}>
          {tables.map((tbl) => (
            <Chip key={tbl.name} label={tbl.name} onClick={() => setSelected(tbl.name)} variant={activeTable === tbl.name ? 'filled' : 'outlined'}
              sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.74rem', ...(activeTable === tbl.name ? { bgcolor: `${accent}22`, color: accent } : {}) })} />
          ))}
        </Stack>

        <DataGrid
          rows={rowsData.rows ?? []} columns={gridColumns}
          getRowId={(row) => String((row as Record<string, unknown>).id ?? JSON.stringify(row))}
          density="compact" emptyLabel="Select a table to browse rows." height={420} minHeight={240}
        />
        <Typography sx={{ fontSize: '0.76rem', color: 'text.disabled', mt: 1.5 }}>
          {activeTable ? `${rowsData.total.toLocaleString()} rows in ${activeTable} · showing first ${rowsData.rows?.length ?? 0}` : 'No table selected.'}
        </Typography>
      </Box>
    </Box>
  );
}
