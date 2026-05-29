import { useMemo, useState } from 'react';
import { Box, Card, Chip, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { FlowCanvas, type FlowNode, type FlowEdge, type HandleSpec } from '@ironflyer/ui-web/fx';
import { DataTable, type DataTableColumn } from '@ironflyer/ui-web/data-table';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';
import { text } from '@ironflyer/design-tokens/brand';

interface AppColumn { name: string; type: string; nullable: boolean; primaryKey: boolean; references: string | null }
interface AppTable { name: string; rowCount: number; columns: AppColumn[] }
interface TableRows { table: string; columns: string[]; rows: Record<string, unknown>[]; total: number }

const SAMPLE: AppTable[] = [
  { name: 'users', rowCount: 1284, columns: [{ name: 'id', type: 'uuid', nullable: false, primaryKey: true, references: null }, { name: 'email', type: 'text', nullable: false, primaryKey: false, references: null }, { name: 'role', type: 'text', nullable: false, primaryKey: false, references: null }, { name: 'verified', type: 'boolean', nullable: false, primaryKey: false, references: null }, { name: 'created_at', type: 'timestamptz', nullable: false, primaryKey: false, references: null }] },
  { name: 'orders', rowCount: 5471, columns: [{ name: 'id', type: 'uuid', nullable: false, primaryKey: true, references: null }, { name: 'user_id', type: 'uuid', nullable: false, primaryKey: false, references: 'users.id' }, { name: 'total_cents', type: 'integer', nullable: false, primaryKey: false, references: null }, { name: 'metadata', type: 'jsonb', nullable: true, primaryKey: false, references: null }, { name: 'placed_at', type: 'timestamptz', nullable: false, primaryKey: false, references: null }] },
  { name: 'products', rowCount: 212, columns: [{ name: 'id', type: 'uuid', nullable: false, primaryKey: true, references: null }, { name: 'title', type: 'text', nullable: false, primaryKey: false, references: null }, { name: 'price_cents', type: 'integer', nullable: false, primaryKey: false, references: null }, { name: 'in_stock', type: 'boolean', nullable: false, primaryKey: false, references: null }] },
];

const SAMPLE_ROWS: Record<string, TableRows> = {
  users: {
    table: 'users', total: 1284,
    columns: ['id', 'email', 'role', 'verified', 'created_at'],
    rows: [
      { id: 'u_1a2b', email: 'ada@example.com', role: 'admin', verified: true, created_at: '2026-01-12T09:24:00Z' },
      { id: 'u_3c4d', email: 'grace@example.com', role: 'member', verified: true, created_at: '2026-02-03T14:10:00Z' },
      { id: 'u_5e6f', email: 'linus@example.com', role: 'member', verified: false, created_at: '2026-03-21T18:45:00Z' },
    ],
  },
  orders: {
    table: 'orders', total: 5471,
    columns: ['id', 'user_id', 'total_cents', 'metadata', 'placed_at'],
    rows: [
      { id: 'o_91', user_id: 'u_1a2b', total_cents: 4200, metadata: { coupon: 'LAUNCH', items: 2 }, placed_at: '2026-04-02T11:00:00Z' },
      { id: 'o_92', user_id: 'u_3c4d', total_cents: 1599, metadata: { coupon: null, items: 1 }, placed_at: '2026-04-05T16:30:00Z' },
    ],
  },
  products: {
    table: 'products', total: 212,
    columns: ['id', 'title', 'price_cents', 'in_stock'],
    rows: [
      { id: 'p_01', title: 'Standard plan', price_cents: 1900, in_stock: true },
      { id: 'p_02', title: 'Pro plan', price_cents: 4900, in_stock: true },
      { id: 'p_03', title: 'Legacy add-on', price_cents: 900, in_stock: false },
    ],
  },
};

const H_LEFT: HandleSpec[] = [{ id: 't', type: 'target', side: 'left' }];
const H_RIGHT: HandleSpec[] = [{ id: 's', type: 'source', side: 'right' }];
const H_BOTH: HandleSpec[] = [{ id: 't', type: 'target', side: 'left' }, { id: 's', type: 'source', side: 'right' }];

// Classify a SQL column type into a render strategy. Keeps the grid type-aware
// (booleans → check/×, numbers → right-aligned, timestamps → formatted dates,
// json → readable, FK → linked) instead of one undifferentiated text column.
type Kind = 'bool' | 'number' | 'datetime' | 'json' | 'text';
function classify(sqlType: string): Kind {
  const t = sqlType.toLowerCase();
  if (/bool/.test(t)) return 'bool';
  if (/(int|serial|numeric|decimal|float|double|real|money|number)/.test(t)) return 'number';
  if (/(timestamp|datetime|^date$|time)/.test(t)) return 'datetime';
  if (/json/.test(t)) return 'json';
  return 'text';
}

function TableCard({ t, accent, muted }: { t: AppTable; accent: string; muted: string }) {
  return (
    <Box sx={{ minWidth: 188 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 0.75 }}>
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s82, fontWeight: 700, color: accent })}>{t.name}</Typography>
        <Typography sx={{ fontSize: text.s66, color: 'text.disabled' }}>{t.rowCount.toLocaleString()} rows</Typography>
      </Stack>
      <Stack spacing={0.25}>
        {t.columns.map((c) => (
          <Stack key={c.name} direction="row" alignItems="center" spacing={0.5} sx={{ justifyContent: 'space-between' }}>
            <Stack direction="row" alignItems="center" spacing={0.5}>
              {c.primaryKey && <Box sx={{ fontSize: text.s60, color: accent }}>◆</Box>}
              {c.references && <Box sx={{ fontSize: text.s60, color: muted }}>↗</Box>}
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70 })}>{c.name}</Typography>
            </Stack>
            <Typography sx={{ fontSize: text.s62, color: 'text.disabled' }}>{c.type}</Typography>
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
    // Live (enabled requires a project) shows real data — empty included; the
    // offline sample is served via fallbackData, never under a "live" chip.
    map: (r) => r.appDataSchema ?? [],
  });

  const activeTable = selected ?? tables[0]?.name ?? null;
  const { data: rowsData } = useGraphQLQuery<TableRows, { appTableRows: TableRows }>({
    key: ['app-table-rows', liveProjectId ?? 'none', activeTable ?? 'none'],
    operationName: 'AppTableRows', query: operations.APP_TABLE_ROWS,
    variables: { projectID: liveProjectId, table: activeTable, limit: 50 },
    fallbackData: (activeTable && SAMPLE_ROWS[activeTable]) || { table: '', columns: [], rows: [], total: 0 },
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

  // Type-aware columns: prefer the table's schema (type/pk/references metadata)
  // so each cell renders by its real type; fall back to bare column names when
  // the rows arrive without a matching schema entry.
  const gridColumns = useMemo<DataTableColumn<Record<string, unknown>>[]>(() => {
    const schema = tables.find((tbl) => tbl.name === activeTable)?.columns ?? [];
    const byName = new Map(schema.map((c) => [c.name, c]));
    const names = rowsData.columns?.length ? rowsData.columns : schema.map((c) => c.name);

    return names.map((name): DataTableColumn<Record<string, unknown>> => {
      const meta = byName.get(name);
      const kind = classify(meta?.type ?? 'text');
      const base: DataTableColumn<Record<string, unknown>> = {
        field: name,
        headerName: name,
        flex: 1,
        minWidth: kind === 'json' ? 200 : 120,
        renderHeader: () => (
          <Stack direction="row" alignItems="center" spacing={0.5}>
            {meta?.primaryKey && <Box component="span" sx={{ fontSize: text.s64, color: accent }}>◆</Box>}
            {meta?.references && <Box component="span" sx={{ fontSize: text.s64, color: muted }}>↗</Box>}
            <Box component="span" sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s74 })}>{name}</Box>
          </Stack>
        ),
      };

      if (kind === 'bool') return { ...base, type: 'boolean', minWidth: 90, flex: 0 };
      if (kind === 'number') return { ...base, type: 'number', minWidth: 110 };
      if (kind === 'datetime') {
        return {
          ...base, type: 'dateTime', minWidth: 170,
          valueGetter: (value) => { if (value == null) return null; const d = new Date(value as string); return Number.isNaN(d.getTime()) ? null : d; },
        };
      }
      if (kind === 'json') {
        return {
          ...base, sortable: false, filterable: false,
          renderCell: (p) => {
            if (p.value == null) return <Box component="span" sx={{ color: 'text.disabled' }}>null</Box>;
            const s = typeof p.value === 'string' ? p.value : JSON.stringify(p.value);
            return (
              <Tooltip title={s} placement="top-start">
                <Box component="span" sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s72, color: 'text.secondary', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' })}>{s}</Box>
              </Tooltip>
            );
          },
        };
      }
      // Foreign keys render as a linked chip so relationships read at a glance.
      if (meta?.references) {
        return {
          ...base, minWidth: 150,
          renderCell: (p) => p.value == null ? <Box component="span" sx={{ color: 'text.disabled' }}>null</Box> : (
            <Chip size="small" label={String(p.value)} onClick={() => { const tgt = meta.references?.split('.')[0]; if (tgt && tables.some((o) => o.name === tgt)) setSelected(tgt); }}
              sx={(th) => ({ height: 20, fontFamily: th.brand.font.mono, fontSize: text.s68, bgcolor: `${accent}1f`, color: accent, cursor: 'pointer' })} />
          ),
        };
      }
      return base;
    });
  }, [tables, activeTable, rowsData.columns, accent, muted]);

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
              sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s74, ...(activeTable === tbl.name ? { bgcolor: `${accent}22`, color: accent } : {}) })} />
          ))}
        </Stack>

        <DataTable
          rows={rowsData.rows ?? []} columns={gridColumns}
          getRowId={(row) => String((row as Record<string, unknown>).id ?? JSON.stringify(row))}
          density="compact" emptyLabel={activeTable ? `No rows in ${activeTable}.` : 'No tables in this app yet.'} height={420} minHeight={240}
        />
        <Typography sx={{ fontSize: text.s76, color: 'text.disabled', mt: 1.5 }}>
          {activeTable ? `${rowsData.total.toLocaleString()} rows in ${activeTable} · showing first ${rowsData.rows?.length ?? 0}` : 'No table selected.'}
        </Typography>
      </Box>
    </Box>
  );
}
