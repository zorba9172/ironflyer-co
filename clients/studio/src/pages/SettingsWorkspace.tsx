import { lazy, Suspense, useState } from 'react';
import { Box, CircularProgress, Tab, Tabs } from '@mui/material';

// Operate › Settings workspace — three inner tabs consolidate general config,
// domain management, and automation rules behind a single top-menu entry.
// Each inner pane is code-split so heavy deps only load when the tab opens.
const SettingsPane = lazy(() => import('./SettingsPane').then((m) => ({ default: m.SettingsPane })));
const DomainsPane = lazy(() => import('./DomainsPane').then((m) => ({ default: m.DomainsPane })));
const AutomationsPane = lazy(() => import('./AutomationsPane').then((m) => ({ default: m.AutomationsPane })));

type STab = 'general' | 'domains' | 'automations';
const TABS: { key: STab; label: string }[] = [
  { key: 'general', label: 'General' },
  { key: 'domains', label: 'Domains' },
  { key: 'automations', label: 'Automations' },
];

function InnerFallback() {
  return (
    <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', bgcolor: 'background.default' }}>
      <CircularProgress size={24} thickness={5} />
    </Box>
  );
}

export function SettingsWorkspace() {
  const [tab, setTab] = useState<STab>('general');

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100%' }}>
      <Box sx={{ px: 3, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as STab)}
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
          {tab === 'general' && <SettingsPane />}
          {tab === 'domains' && <DomainsPane />}
          {tab === 'automations' && <AutomationsPane />}
        </Suspense>
      </Box>
    </Box>
  );
}
