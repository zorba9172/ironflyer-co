import { lazy, Suspense, useState } from 'react';
import { Box, CircularProgress, Tab, Tabs } from '@mui/material';

// Operate › Data workspace — two inner tabs keep the top menu lean while
// grouping naturally-related surfaces: the live database record browser
// (Records) and the project's public API explorer (API).
// Each inner pane is code-split so heavy deps (ag-grid, echarts) only load
// when their tab is opened.
const DataPane = lazy(() => import('./DataPane').then((m) => ({ default: m.DataPane })));
const ApiPane = lazy(() => import('./ApiPane').then((m) => ({ default: m.ApiPane })));

type DTab = 'records' | 'api';
const TABS: { key: DTab; label: string }[] = [
  { key: 'records', label: 'Records' },
  { key: 'api', label: 'API' },
];

function InnerFallback() {
  return (
    <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', bgcolor: 'background.default' }}>
      <CircularProgress size={24} thickness={5} />
    </Box>
  );
}

export function DataWorkspace() {
  const [tab, setTab] = useState<DTab>('records');

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100%' }}>
      <Box sx={{ px: 3, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as DTab)}
          variant="scrollable"
          scrollButtons="auto"
          sx={{
            minHeight: 0,
            '& .MuiTab-root': {
              textTransform: 'none',
              minHeight: 44,
              fontSize: (t) => t.typography.body2.fontSize,
            },
          }}
        >
          {TABS.map((x) => <Tab key={x.key} value={x.key} label={x.label} />)}
        </Tabs>
      </Box>
      <Box sx={{ flex: 1, minHeight: 0, display: 'flex', overflow: 'auto' }}>
        <Suspense fallback={<InnerFallback />}>
          {tab === 'records' && <DataPane />}
          {tab === 'api' && <ApiPane />}
        </Suspense>
      </Box>
    </Box>
  );
}
