import { lazy, Suspense, type ComponentType } from 'react';
import { Box } from '@mui/material';
import type { DataGridProps } from './DataGridInner';

export type { DataGridColumn, DataGridCellParams, DataGridProps } from './DataGridInner';

// Lazy boundary: ag-grid (and its CSS) load only when a grid actually mounts,
// so surfaces that never open a table never pay for it (constitutional:
// heavy libs never land in the cold bundle).
const Inner = lazy(() => import('./DataGridInner')) as unknown as ComponentType<DataGridProps<object>>;

export function DataGrid<TData extends object>(props: DataGridProps<TData>) {
  const { height = 360, minHeight = 220 } = props;
  return (
    <Suspense fallback={<Box sx={{ width: '100%', height, minHeight, borderRadius: 2, bgcolor: 'action.hover' }} />}>
      <Inner {...(props as unknown as DataGridProps<object>)} />
    </Suspense>
  );
}
