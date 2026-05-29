import { lazy, Suspense, type ComponentType } from 'react';
import { Box } from '@mui/material';
import type { GridValidRowModel } from '@mui/x-data-grid';
import type { DataTableProps } from './DataTableInner';

export type { DataTableColumn, DataTableProps } from './DataTableInner';

// Lazy boundary: MUI X DataGrid loads only when a table actually mounts, so
// surfaces that never open one never pay for it (constitutional: heavy libs
// never land in the cold bundle).
const Inner = lazy(() => import('./DataTableInner')) as unknown as ComponentType<DataTableProps<GridValidRowModel>>;

export function DataTable<TRow extends GridValidRowModel = GridValidRowModel>(props: DataTableProps<TRow>) {
  const { height = 420, minHeight = 240 } = props;
  return (
    <Suspense fallback={<Box sx={{ width: '100%', height, minHeight, borderRadius: 2, bgcolor: 'action.hover' }} />}>
      <Inner {...(props as unknown as DataTableProps<GridValidRowModel>)} />
    </Suspense>
  );
}
