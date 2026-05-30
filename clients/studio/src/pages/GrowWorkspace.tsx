import { lazy, Suspense, useState } from 'react';
import { Box, CircularProgress, Tab, Tabs } from '@mui/material';

// Business › Grow workspace — two inner tabs consolidate analytics and
// marketing behind a single top-menu entry.
// Each inner pane is code-split so heavy deps only load when the tab opens.
const AnalyticsPane = lazy(() => import('./AnalyticsPane').then((m) => ({ default: m.AnalyticsPane })));
const MarketingPane = lazy(() => import('./MarketingPane').then((m) => ({ default: m.MarketingPane })));

type GTab = 'analytics' | 'marketing';
const TABS: { key: GTab; label: string }[] = [
  { key: 'analytics', label: 'Analytics' },
  { key: 'marketing', label: 'Marketing' },
];

function InnerFallback() {
  return (
    <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', bgcolor: 'background.default' }}>
      <CircularProgress size={24} thickness={5} />
    </Box>
  );
}

export function GrowWorkspace() {
  const [tab, setTab] = useState<GTab>('analytics');

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100%' }}>
      <Box sx={{ px: 3, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as GTab)}
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
          {tab === 'analytics' && <AnalyticsPane />}
          {tab === 'marketing' && <MarketingPane />}
        </Suspense>
      </Box>
    </Box>
  );
}
