import { DataGrid, type DataGridProps } from '@ironflyer/ui-web/data-grid';
import { useTheme, type Theme } from '@mui/material/styles';
import type { SxProps } from '@mui/material';
import { studioTokens } from '../../theme';

export type { DataGridCellParams, DataGridColumn, DataGridProps } from '@ironflyer/ui-web/data-grid';

function studioGridVars(theme: Theme): NonNullable<DataGridProps<object>['cssVars']> {
  return {
    '--if-grid-bg': theme.palette.cardBg,
    '--if-grid-header-bg': theme.palette.surfaceHover,
    '--if-grid-hover': theme.palette.surfaceHover,
    '--if-grid-selected': `${theme.palette.primary.main}18`,
    '--if-grid-border': theme.palette.cardBorder,
    '--if-grid-font-family': studioTokens.font.family,
  } as NonNullable<DataGridProps<object>['cssVars']>;
}

function studioGridSx<TData extends object>(sx: DataGridProps<TData>['sx']): SxProps<Theme> {
  const base: SxProps<Theme> = (theme) => ({
    '& .ag-root-wrapper': {
      borderRadius: `${theme.studio.radius.sm}px`,
      boxShadow: theme.palette.mode === 'dark' ? '0 18px 60px rgba(0,0,0,0.22)' : '0 18px 50px rgba(17,25,54,0.08)',
      overflow: 'hidden',
    },
    '& .ag-header': {
      borderBottomColor: theme.palette.divider,
    },
    '& .ag-row:hover': {
      boxShadow: `inset 2px 0 0 ${theme.palette.primary.main}`,
    },
    '& .ag-cell': {
      borderColor: theme.palette.borderSubtle,
    },
  });
  return [base, ...(Array.isArray(sx) ? sx : sx ? [sx] : [])] as SxProps<Theme>;
}

export function StudioDataGrid<TData extends object>(props: DataGridProps<TData>) {
  const theme = useTheme();
  return (
    <DataGrid
      {...props}
      cssVars={{ ...studioGridVars(theme), ...props.cssVars }}
      sx={studioGridSx(props.sx)}
    />
  );
}
