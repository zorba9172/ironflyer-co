import { lazy, Suspense, useEffect, useState } from 'react';
import { Box, CircularProgress, Tab, Tabs } from '@mui/material';
import { useStudio } from '../store';
import type { StudioProject } from '../studioData';

// The execution team in two views behind one menu entry: the roster (list +
// management) and the live graph. Code-split so the heavy React Flow graph
// only loads when its tab opens.
const AgentsManagerPane = lazy(() => import('./AgentsManagerPane').then((m) => ({ default: m.AgentsManagerPane })));
const ExecutionTeamGraph = lazy(() => import('./ExecutionTeamGraph').then((m) => ({ default: m.ExecutionTeamGraph })));

type TTab = 'list' | 'graph';
const TABS: { key: TTab; label: string }[] = [
  { key: 'list', label: 'Roster' },
  { key: 'graph', label: 'Graph' },
];

function InnerFallback() {
  return (
    <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', bgcolor: 'background.default' }}>
      <CircularProgress size={24} thickness={5} />
    </Box>
  );
}

export function ExecutionTeamWorkspace({ project }: { project: StudioProject }) {
  const [tab, setTab] = useState<TTab>(() => {
    const req = useStudio.getState().innerTab;
    return TABS.some((x) => x.key === req) ? (req as TTab) : 'list';
  });
  const setInnerTab = useStudio((s) => s.setInnerTab);
  const selectGate = useStudio((s) => s.selectGate);
  useEffect(() => { setInnerTab(null); }, [setInnerTab]);

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, height: '100%' }}>
      <Box sx={{ px: 3, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as TTab)}
          sx={{ minHeight: 0, '& .MuiTab-root': { textTransform: 'none', minHeight: 44, fontSize: (t) => t.typography.body2.fontSize } }}
        >
          {TABS.map((x) => <Tab key={x.key} value={x.key} label={x.label} />)}
        </Tabs>
      </Box>
      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Suspense fallback={<InnerFallback />}>
          {tab === 'list' && <AgentsManagerPane project={project} />}
          {tab === 'graph' && <ExecutionTeamGraph project={project} onOpenGate={(id) => selectGate(id)} />}
        </Suspense>
      </Box>
    </Box>
  );
}
