import { DataTable, type DataTableProps } from '@ironflyer/ui-web/data-table';
import type { GridValidRowModel } from '@mui/x-data-grid';
import type { Theme } from '@mui/material/styles';
import type { SxProps } from '@mui/material';
import { StudioTableShell, type StudioTableShellProps } from './StudioTableShell';

export type { DataTableColumn, DataTableProps } from '@ironflyer/ui-web/data-table';

type StudioDataTableChromeProps = Omit<StudioTableShellProps, 'children'>;

export type StudioDataTableProps<TRow extends GridValidRowModel = GridValidRowModel> = DataTableProps<TRow> & StudioDataTableChromeProps;

function hasTableChrome(props: StudioDataTableChromeProps) {
  return props.title != null || props.subtitle != null || !!props.tabs?.length || props.actions != null || props.showDefaultActions || props.onSearchChange != null || props.footer != null;
}

function studioTableSx<TRow extends GridValidRowModel>(sx: DataTableProps<TRow>['sx'], wrapped: boolean): SxProps<Theme> {
  const base: SxProps<Theme> = (theme) => ({
    borderRadius: wrapped ? 0 : `${theme.studio.radius.sm}px`,
    border: wrapped ? 0 : `1px solid ${theme.palette.cardBorder}`,
    bgcolor: 'cardBg',
    boxShadow: wrapped ? 'none' : '0 1px 2px rgba(17,24,39,0.04)',
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

export function StudioDataTable<TRow extends GridValidRowModel = GridValidRowModel>(props: StudioDataTableProps<TRow>) {
  const {
    title,
    subtitle,
    tabs,
    activeTab,
    onTabChange,
    actions,
    footer,
    searchValue,
    onSearchChange,
    searchPlaceholder,
    showDefaultActions,
    ...tableProps
  } = props;
  const chromeProps = { title, subtitle, tabs, activeTab, onTabChange, actions, showDefaultActions, footer, searchValue, onSearchChange, searchPlaceholder };
  const wrapped = hasTableChrome(chromeProps);
  return (
    <StudioTableShell
      title={title}
      subtitle={subtitle}
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={onTabChange}
      actions={actions}
      showDefaultActions={showDefaultActions}
      footer={footer}
      searchValue={searchValue}
      onSearchChange={onSearchChange}
      searchPlaceholder={searchPlaceholder}
    >
      <DataTable {...tableProps} sx={studioTableSx(tableProps.sx, wrapped)} dataGridSx={studioInnerTableSx(tableProps.dataGridSx)} />
    </StudioTableShell>
  );
}
