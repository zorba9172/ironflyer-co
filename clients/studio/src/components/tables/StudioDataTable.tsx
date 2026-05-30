import { DataTable, type DataTableProps } from '@ironflyer/ui-web/data-table';
import type { GridValidRowModel } from '@mui/x-data-grid';
import type { Theme } from '@mui/material/styles';
import type { SxProps } from '@mui/material';

export type { DataTableColumn, DataTableProps } from '@ironflyer/ui-web/data-table';

function studioTableSx<TRow extends GridValidRowModel>(sx: DataTableProps<TRow>['sx']): SxProps<Theme> {
  const base: SxProps<Theme> = (theme) => ({
    borderRadius: `${theme.studio.radius.sm}px`,
    border: `1px solid ${theme.palette.cardBorder}`,
    bgcolor: 'cardBg',
    boxShadow: theme.palette.mode === 'dark' ? '0 18px 60px rgba(0,0,0,0.18)' : '0 18px 50px rgba(17,25,54,0.08)',
    overflow: 'hidden',
    '& .MuiDataGrid-root': {
      bgcolor: 'transparent',
    },
    '& .MuiDataGrid-columnHeaders': {
      bgcolor: 'surfaceHover',
      borderColor: 'divider',
    },
    '& .MuiDataGrid-row:hover': {
      bgcolor: 'surfaceHover',
    },
    '& .MuiDataGrid-cell': {
      borderColor: 'borderSubtle',
    },
  });
  return [base, ...(Array.isArray(sx) ? sx : sx ? [sx] : [])] as SxProps<Theme>;
}

function studioInnerTableSx<TRow extends GridValidRowModel>(sx: DataTableProps<TRow>['dataGridSx']): SxProps<Theme> {
  const base: SxProps<Theme> = (theme) => ({
    '--DataGrid-rowBorderColor': theme.palette.borderSubtle,
    '& .MuiDataGrid-columnHeaders': {
      bgcolor: 'surfaceHover',
      borderColor: 'divider',
    },
    '& .MuiDataGrid-columnHeaderTitle': {
      fontWeight: 700,
      color: 'text.secondary',
      textTransform: 'uppercase',
      fontSize: 11,
    },
    '& .MuiDataGrid-row:hover': {
      bgcolor: 'surfaceHover',
    },
    '& .MuiDataGrid-cell': {
      borderColor: 'borderSubtle',
    },
  });
  return [base, ...(Array.isArray(sx) ? sx : sx ? [sx] : [])] as SxProps<Theme>;
}

export function StudioDataTable<TRow extends GridValidRowModel = GridValidRowModel>(props: DataTableProps<TRow>) {
  return <DataTable {...props} sx={studioTableSx(props.sx)} dataGridSx={studioInnerTableSx(props.dataGridSx)} />;
}
