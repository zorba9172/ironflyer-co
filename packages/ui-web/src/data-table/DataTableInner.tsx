import { Box, Typography, type SxProps, type Theme } from '@mui/material';
import {
  DataGrid,
  type GridColDef,
  type GridRowId,
  type GridRowsProp,
  type GridValidRowModel,
} from '@mui/x-data-grid';

export type DataTableColumn<TRow extends GridValidRowModel = GridValidRowModel> = GridColDef<TRow>;

export interface DataTableProps<TRow extends GridValidRowModel = GridValidRowModel> {
  rows: readonly TRow[];
  columns: GridColDef<TRow>[];
  getRowId?: (row: TRow) => GridRowId;
  density?: 'compact' | 'standard' | 'comfortable';
  height?: number | string;
  minHeight?: number | string;
  loading?: boolean;
  emptyLabel?: string;
  /** when set, enables client pagination at this page size */
  pageSize?: number;
  dataGridSx?: SxProps<Theme>;
  sx?: SxProps<Theme>;
}

function NoRows({ label }: { label: string }) {
  return (
    <Box sx={{ display: 'grid', placeItems: 'center', height: '100%', px: 2 }}>
      <Typography variant="body2" sx={{ color: 'text.disabled', textAlign: 'center' }}>
        {label}
      </Typography>
    </Box>
  );
}

// The real grid. MUI X DataGrid is heavy, so this module is only ever reached
// through the lazy wrapper in DataTable.tsx — it never lands in the cold
// bundle. Styling is inherited from the MUI theme (constitutional: style maps
// through the theme, never inline literals); only structural tweaks live here.
export default function DataTableInner<TRow extends GridValidRowModel = GridValidRowModel>({
  rows,
  columns,
  getRowId,
  density = 'compact',
  height = 420,
  minHeight = 240,
  loading = false,
  emptyLabel = 'No rows',
  pageSize,
  dataGridSx,
  sx,
}: DataTableProps<TRow>) {
  const rootSx = [
    { width: '100%', height, minHeight },
    ...(Array.isArray(sx) ? sx : sx ? [sx] : []),
  ] as SxProps<Theme>;

  return (
    <Box sx={rootSx}>
      <DataGrid<TRow>
        rows={rows as GridRowsProp<TRow>}
        columns={columns}
        getRowId={getRowId}
        density={density}
        loading={loading}
        disableRowSelectionOnClick
        disableColumnMenu
        pagination={pageSize ? true : undefined}
        initialState={pageSize ? { pagination: { paginationModel: { pageSize } } } : undefined}
        pageSizeOptions={pageSize ? [pageSize] : []}
        hideFooter={!pageSize}
        slots={{ noRowsOverlay: () => <NoRows label={emptyLabel} /> }}
        sx={[
          {
            border: 0,
            '--DataGrid-rowBorderColor': (t) => t.palette.divider,
            '& .MuiDataGrid-columnHeaders': { bgcolor: 'action.hover' },
            '& .MuiDataGrid-columnHeaderTitle': { fontWeight: 600, color: 'text.secondary' },
            '& .MuiDataGrid-cell:focus, & .MuiDataGrid-cell:focus-within': { outline: 'none' },
            '& .MuiDataGrid-columnSeparator': { color: 'divider' },
            '& .MuiDataGrid-footerContainer': { borderColor: 'divider' },
          },
          ...(Array.isArray(dataGridSx) ? dataGridSx : dataGridSx ? [dataGridSx] : []),
        ]}
      />
    </Box>
  );
}
