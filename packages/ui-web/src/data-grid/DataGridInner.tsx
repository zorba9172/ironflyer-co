import { useMemo, type CSSProperties } from 'react';
import { Box, type SxProps, type Theme, useTheme } from '@mui/material';
import { AgGridReact, type AgGridReactProps } from 'ag-grid-react';
import {
  AllCommunityModule,
  type ColDef,
  type GetRowIdParams,
  type GridReadyEvent,
  type ICellRendererParams,
  type RowClickedEvent,
} from 'ag-grid-community';
import './DataGrid.css';

const communityModules = [AllCommunityModule];

export type DataGridColumn<TData extends object> = ColDef<TData>;
export type DataGridCellParams<TData extends object, TValue = unknown> = ICellRendererParams<TData, TValue>;

export interface DataGridProps<TData extends object> {
  rows: TData[];
  columns: DataGridColumn<TData>[];
  getRowId?: (row: TData) => string;
  height?: number | string;
  minHeight?: number | string;
  density?: 'standard' | 'compact';
  emptyLabel?: string;
  quickFilterText?: string;
  pagination?: boolean;
  pageSize?: number;
  onReady?: (event: GridReadyEvent<TData>) => void;
  onRowClick?: (row: TData, event: RowClickedEvent<TData>) => void;
  gridOptions?: Partial<AgGridReactProps<TData>>;
  cssVars?: CSSProperties;
  sx?: SxProps<Theme>;
}

// The real grid. ag-grid is heavy, so this module is only ever reached through
// the lazy wrapper in DataGrid.tsx — it never lands in the cold bundle.
export default function DataGridInner<TData extends object>({
  rows,
  columns,
  getRowId,
  height = 360,
  minHeight = 220,
  density = 'standard',
  emptyLabel = 'No rows',
  quickFilterText,
  pagination = false,
  pageSize = 10,
  onReady,
  onRowClick,
  gridOptions,
  cssVars,
  sx,
}: DataGridProps<TData>) {
  const theme = useTheme();
  const defaultColDef = useMemo<ColDef<TData>>(
    () => ({
      filter: true,
      resizable: true,
      sortable: true,
      suppressHeaderMenuButton: true,
    }),
    [],
  );
  const gridGetRowId = useMemo(
    () =>
      getRowId
        ? (params: GetRowIdParams<TData>) => getRowId(params.data)
        : undefined,
    [getRowId],
  );

  const vars = {
    '--if-grid-bg': theme.palette.background.paper,
    '--if-grid-fg': theme.palette.text.primary,
    '--if-grid-muted': theme.palette.text.secondary,
    '--if-grid-border': theme.palette.divider,
    '--if-grid-header-bg': theme.palette.action.hover,
    '--if-grid-hover': theme.palette.action.hover,
    '--if-grid-selected': theme.palette.action.selected,
    '--if-grid-font-family': theme.typography.fontFamily,
  } as CSSProperties;
  const rootSx = [
    { width: '100%', height, minHeight },
    ...(Array.isArray(sx) ? sx : sx ? [sx] : []),
  ] as SxProps<Theme>;

  return (
    <Box
      className={`if-data-grid ag-theme-quartz${density === 'compact' ? ' if-data-grid--compact' : ''}`}
      sx={rootSx}
      style={{ ...vars, ...cssVars }}
    >
      <AgGridReact<TData>
        modules={communityModules}
        rowData={rows}
        columnDefs={columns}
        defaultColDef={defaultColDef}
        getRowId={gridGetRowId}
        quickFilterText={quickFilterText}
        pagination={pagination}
        paginationPageSize={pageSize}
        paginationPageSizeSelector={false}
        rowHeight={density === 'compact' ? 36 : 42}
        headerHeight={density === 'compact' ? 34 : 38}
        animateRows={false}
        suppressCellFocus
        suppressMovableColumns
        overlayNoRowsTemplate={emptyLabel}
        onGridReady={onReady}
        onRowClicked={(event) => {
          if (event.data) onRowClick?.(event.data, event);
        }}
        {...gridOptions}
      />
    </Box>
  );
}
