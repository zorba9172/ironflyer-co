import { lazy, Suspense, useEffect, useState } from 'react';
import { Box, CircularProgress, Tab, Tabs, Tooltip } from '@mui/material';
import { useStudio } from '../store';
import type { StudioProject } from '../studioData';

// The execution team in three views behind one menu entry: the flow graph
// (default — the clearest mirror of who routes to which gate), the network
// map (glanceable 2D node graph), and the roster (list + management). Code-split
// so heavy deps only load on demand.
const AgentsManagerPane = lazy(() => import('./AgentsManagerPane').then((m) => ({ default: m.AgentsManagerPane })));
const ExecutionTeamGraph = lazy(() => import('./ExecutionTeamGraph').then((m) => ({ default: m.ExecutionTeamGraph })));

type TTab = 'graph' | 'network' | 'list';
const TABS: { key: TTab; label: string; title: string }[] = [
  { key: 'graph', label: 'Flow', title: 'Execution flow — who routes to which finisher gate' },
  { key: 'network', label: 'Network', title: 'Agent network — 2D mirror of agents, gates, and handoffs' },
  { key: 'list', label: 'Roster', title: 'Agent roster — manage the team' },
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
    return TABS.some((x) => x.key === req) ? (req as TTab) : 'graph';
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
          sx={{
            minHeight: 0,
            '& .MuiTab-root': {
              textTransform: 'none',
              minHeight: 44,
              fontSize: (t) => t.typography.body2.fontSize,
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
          {tab === 'graph' && <ExecutionTeamGraph project={project} onOpenGate={(id) => selectGate(id)} />}
          {tab === 'network' && <ExecutionTeamGraph project={project} onOpenGate={(id) => selectGate(id)} networkMode />}
          {tab === 'list' && <AgentsManagerPane project={project} />}
        </Suspense>
      </Box>
    </Box>
  );
}
