import { DataGrid, type DataGridProps } from '@ironflyer/ui-web/data-grid';
import { useTheme, type Theme } from '@mui/material/styles';
import type { SxProps } from '@mui/material';
import { studioTokens } from '../../theme';
import { StudioTableShell, type StudioTableShellProps } from './StudioTableShell';

export type { DataGridCellParams, DataGridColumn, DataGridProps } from '@ironflyer/ui-web/data-grid';

type StudioDataGridChromeProps = Omit<StudioTableShellProps, 'children'>;

export type StudioDataGridProps<TData extends object> = DataGridProps<TData> & StudioDataGridChromeProps;

function hasTableChrome(props: StudioDataGridChromeProps) {
  return props.title != null || props.subtitle != null || !!props.tabs?.length || props.actions != null || props.showDefaultActions || props.onSearchChange != null || props.footer != null;
}

function studioGridVars(theme: Theme, wrapped: boolean): NonNullable<DataGridProps<object>['cssVars']> {
  return {
    '--if-grid-bg': theme.palette.cardBg,
    '--if-grid-fg': theme.palette.text.primary,
    '--if-grid-muted': theme.palette.text.secondary,
    '--if-grid-header-bg': theme.palette.surfaceHover,
    '--if-grid-hover': theme.palette.surfaceHover,
    '--if-grid-selected': `${theme.palette.primary.main}12`,
    '--if-grid-accent': theme.palette.primary.main,
    '--if-grid-accent-soft': `${theme.palette.primary.main}12`,
    '--if-grid-row-alt': theme.palette.surfaceHover,
    '--if-grid-panel-bg': theme.palette.background.default,
    '--if-grid-border': theme.palette.cardBorder,
    '--if-grid-font-family': studioTokens.font.family,
    '--if-grid-radius': wrapped ? '0px' : `${theme.studio.radius.sm}px`,
  } as NonNullable<DataGridProps<object>['cssVars']>;
}

function studioGridSx<TData extends object>(sx: DataGridProps<TData>['sx'], wrapped: boolean): SxProps<Theme> {
  const base: SxProps<Theme> = (theme) => ({
    '& .ag-root-wrapper': {
      border: wrapped ? 0 : `1px solid ${theme.palette.cardBorder}`,
      borderRadius: wrapped ? 0 : `${theme.studio.radius.sm}px`,
      boxShadow: wrapped ? 'none' : '0 1px 2px rgba(17,24,39,0.04)',
      overflow: 'hidden',
    },
    '& .ag-header': {
      borderBottomColor: theme.palette.divider,
    },
    '& .ag-header-cell-label': {
      fontFamily: theme.brand.font.mono,
    },
    '& .ag-cell': {
      borderColor: theme.palette.borderSubtle,
    },
  });
  return [base, ...(Array.isArray(sx) ? sx : sx ? [sx] : [])] as SxProps<Theme>;
}

export function StudioDataGrid<TData extends object>(props: StudioDataGridProps<TData>) {
  const theme = useTheme();
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
    ...gridProps
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
      <DataGrid
        {...gridProps}
        density={gridProps.density ?? 'compact'}
        cssVars={{ ...studioGridVars(theme, wrapped), ...gridProps.cssVars }}
        sx={studioGridSx(gridProps.sx, wrapped)}
      />
    </StudioTableShell>
  );
}
