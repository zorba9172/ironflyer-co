import { lazy, Suspense, useEffect, useState } from 'react';
import { Box, CircularProgress, Tab, Tabs, Tooltip } from '@mui/material';
import { useStudio } from '../store';

// Engineering-quality cluster, consolidated behind one menu entry with inner
// tabs (the owner asked for naturally-related surfaces to share a pane so the
// top menu stays lean). Each inner pane is code-split so opening a tab is the
// only thing that pulls its heavy deps (echarts, ag-grid, MUI X grid).
const ReviewPane = lazy(() => import('./ReviewPane').then((m) => ({ default: m.ReviewPane })));
const QualityPane = lazy(() => import('./QualityPane').then((m) => ({ default: m.QualityPane })));
const CoveragePane = lazy(() => import('./CoveragePane').then((m) => ({ default: m.CoveragePane })));
const PerformancePane = lazy(() => import('./PerformancePane').then((m) => ({ default: m.PerformancePane })));

type QTab = 'review' | 'health' | 'coverage' | 'performance';
const TABS: { key: QTab; label: string; title: string }[] = [
  { key: 'review', label: 'Review', title: 'AI-guided production review — issues, fixes, and readiness' },
  { key: 'health', label: 'Code health', title: 'Code quality gates — dedup, dead code, complexity, arch boundaries' },
  { key: 'coverage', label: 'Coverage', title: 'Your app\'s test coverage — which files are not closed end-to-end' },
  { key: 'performance', label: 'Performance', title: 'Lighthouse, bundle size, and perf budget audits' },
];

function InnerFallback() {
  return (
    <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', bgcolor: 'background.default' }}>
      <CircularProgress size={24} thickness={5} />
    </Box>
  );
}

export function QualityWorkspace() {
  // Honor a deep-link (e.g. the Map's Performance facet) that asked for a
  // specific inner tab; read it once, then clear it so it doesn't stick.
  const [tab, setTab] = useState<QTab>(() => {
    const req = useStudio.getState().innerTab;
    return TABS.some((x) => x.key === req) ? (req as QTab) : 'review';
  });
  const setInnerTab = useStudio((s) => s.setInnerTab);
  useEffect(() => { setInnerTab(null); }, [setInnerTab]);

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100%' }}>
      <Box sx={{ px: 3, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as QTab)}
          variant="scrollable"
          scrollButtons="auto"
          sx={{
            minHeight: 0,
            '& .MuiTab-root': {
              textTransform: 'none',
              minHeight: 44,
              fontSize: (t) => t.typography.body2.fontSize,
            },
            '& .MuiTab-root.Mui-selected': {
              fontWeight: 700,
            },
          }}
        >
          {TABS.map((x) => (
            <Tooltip key={x.key} title={x.title} arrow placement="bottom">
              <Tab value={x.key} label={x.label} />
            </Tooltip>
          ))}
        </Tabs>
      </Box>
      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Suspense fallback={<InnerFallback />}>
          {tab === 'review' && <ReviewPane />}
          {tab === 'health' && <QualityPane />}
          {tab === 'coverage' && <CoveragePane />}
          {tab === 'performance' && <PerformancePane />}
        </Suspense>
      </Box>
    </Box>
  );
}
