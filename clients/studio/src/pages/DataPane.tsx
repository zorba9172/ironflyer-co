import { useMemo, useState } from 'react';
import { Box, Chip, Stack, Tooltip } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';
import { StudioChart, horizontalBarOption, type EChartsOption } from '../components/charts';
import { StudioDataTable, type DataTableColumn, type StudioTableTab } from '../components/tables';
import { GlassPanel, SectionHeader, StatCard } from '../components/studio';
import { Icon } from '../icons';
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

// Classify a SQL column type into a render strategy.
type Kind = 'bool' | 'number' | 'datetime' | 'json' | 'text';
function classify(sqlType: string): Kind {
  const t = sqlType.toLowerCase();
  if (/bool/.test(t)) return 'bool';
  if (/(int|serial|numeric|decimal|float|double|real|money|number)/.test(t)) return 'number';
  if (/(timestamp|datetime|^date$|time)/.test(t)) return 'datetime';
  if (/json/.test(t)) return 'json';
  return 'text';
}

export function DataPane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const [selected, setSelected] = useState<string | null>(null);
  const [tableSearch, setTableSearch] = useState('');

  const { data: tables, isLive } = useGraphQLQuery<AppTable[], { appDataSchema: AppTable[] }>({
    key: ['app-data-schema', liveProjectId ?? 'none'],
    operationName: 'AppDataSchema', query: operations.APP_DATA_SCHEMA,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    map: (r) => r.appDataSchema ?? [],
  });

  const activeTable = selected ?? tables[0]?.name ?? null;
  const tableTabs = useMemo<StudioTableTab[]>(() => tables.map((tbl) => ({
    value: tbl.name,
    label: tbl.name,
    count: tbl.rowCount,
    tone: tbl.rowCount > 1000 ? 'info' : 'default',
  })), [tables]);
  const { data: rowsData } = useGraphQLQuery<TableRows, { appTableRows: TableRows }>({
    key: ['app-table-rows', liveProjectId ?? 'none', activeTable ?? 'none'],
    operationName: 'AppTableRows', query: operations.APP_TABLE_ROWS,
    variables: { projectID: liveProjectId, table: activeTable, limit: 50 },
    fallbackData: (activeTable && SAMPLE_ROWS[activeTable]) || { table: '', columns: [], rows: [], total: 0 },
    enabled: !!liveProjectId && !!activeTable,
    map: (r) => r.appTableRows,
  });

  const accent = t.palette.primary.main;
  const muted = t.palette.text.secondary;

  // One hue per table row from the categorical Aurora palette — never flat grey.
  const volumeChart = useMemo<EChartsOption>(() => horizontalBarOption(t, {
    labels: tables.map((tbl) => tbl.name),
    values: tables.map((tbl) => tbl.rowCount),
  }), [tables, t]);

  // Compact operator summary: counts that frame the chart below it.
  const summary = useMemo(() => {
    const totalRows = tables.reduce((sum, tbl) => sum + tbl.rowCount, 0);
    const totalCols = tables.reduce((sum, tbl) => sum + tbl.columns.length, 0);
    const largest = tables.reduce<AppTable | null>((top, tbl) => (!top || tbl.rowCount > top.rowCount ? tbl : top), null);
    return { totalRows, totalCols, largest };
  }, [tables]);

  // Right-size the bar chart to its row count so it never floats in empty space:
  // ~38px per bar, clamped to a tight band rather than a fixed 280px void.
  const chartHeight = Math.min(260, Math.max(132, tables.length * 38 + 28));

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
      // Foreign key cells render as accent chips — relationships read at a glance
      if (meta?.references) {
        return {
          ...base, minWidth: 150,
          renderCell: (p) => p.value == null ? <Box component="span" sx={{ color: 'text.disabled' }}>null</Box> : (
            <Chip
              size="small"
              label={String(p.value)}
              onClick={() => { const tgt = meta.references?.split('.')[0]; if (tgt && tables.some((o) => o.name === tgt)) setSelected(tgt); }}
              sx={(th) => ({ height: 20, fontFamily: th.brand.font.mono, fontSize: text.s68, bgcolor: `${accent}1f`, color: accent, cursor: 'pointer' })}
            />
          ),
        };
      }
      return base;
    });
  }, [tables, activeTable, rowsData.columns, accent, muted]);

  const visibleRows = useMemo(() => {
    const q = tableSearch.trim().toLowerCase();
    const rows = rowsData.rows ?? [];
    if (!q) return rows;
    return rows.filter((row) => Object.values(row).some((value) => {
      if (value == null) return false;
      const textValue = typeof value === 'object' ? JSON.stringify(value) : String(value);
      return textValue.toLowerCase().includes(q);
    }));
  }, [rowsData.rows, tableSearch]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1180, mx: 'auto' }}>
        <PaneHeader
          title="Data"
          isLive={isLive}
          subtitle={`${tables.length} tables · schema changes flow through the finisher, never ad-hoc DDL`}
        />

        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: 'repeat(3, 1fr)' },
            gap: 1.5,
            mb: 1.5,
          }}
        >
          <StatCard
            label="Tables"
            value={tables.length}
            hint="live in this app"
            accent={t.studio.neon.indigo}
            icon={<Icon name="data" size={16} />}
          />
          <StatCard
            label="Total rows"
            value={summary.totalRows.toLocaleString()}
            hint={`across ${summary.totalCols} columns`}
            accent={t.studio.neon.cyan}
            icon={<Icon name="layers" size={16} />}
          />
          <StatCard
            label="Largest table"
            value={summary.largest?.name ?? '—'}
            hint={summary.largest ? `${summary.largest.rowCount.toLocaleString()} rows` : 'no tables yet'}
            accent={t.studio.neon.violet}
            icon={<Icon name="chartBar" size={16} />}
          />
        </Box>

        <GlassPanel accent={t.palette.primary.main} pad={2} sx={{ mb: 2 }}>
          <SectionHeader
            eyebrow="Row counts by table"
            title="Data volume"
            subtitle="Each bar maps to a real table — one hue per table, width = row count."
          />
          <StudioChart option={volumeChart} height={chartHeight} />
        </GlassPanel>

        <StudioDataTable
          title={activeTable ? `${activeTable} rows` : 'Rows'}
          subtitle={activeTable ? `${rowsData.total.toLocaleString()} total rows · showing ${visibleRows.length.toLocaleString()} in the current view` : 'No table selected'}
          tabs={tableTabs}
          activeTab={activeTable ?? undefined}
          onTabChange={(value) => setSelected(value)}
          searchValue={tableSearch}
          onSearchChange={setTableSearch}
          searchPlaceholder={activeTable ? `Search ${activeTable}` : 'Search rows'}
          footer={activeTable
            ? `${rowsData.total.toLocaleString()} rows in ${activeTable} · schema metadata stays available through the table tabs.`
            : 'No table selected.'}
          rows={visibleRows}
          columns={gridColumns}
          getRowId={(row) => String((row as Record<string, unknown>).id ?? JSON.stringify(row))}
          density="compact"
          emptyLabel={activeTable ? `No rows in ${activeTable}.` : 'No tables in this app yet.'}
          height={420}
          minHeight={240}
        />
      </Box>
    </Box>
  );
}
