import { lazy, Suspense, useEffect, useState } from 'react';
import { Box, CircularProgress, Tab, Tabs, Tooltip } from '@mui/material';
import { useStudio } from '../store';
import type { StudioProject } from '../studioData';

// The execution team in three views behind one menu entry: the constellation
// (default glanceable 3D living graph), the roster (list + management), and
// the legacy flow graph. Code-split so heavy deps only load on demand.
const AgentsManagerPane = lazy(() => import('./AgentsManagerPane').then((m) => ({ default: m.AgentsManagerPane })));
const ExecutionTeamGraph = lazy(() => import('./ExecutionTeamGraph').then((m) => ({ default: m.ExecutionTeamGraph })));

type TTab = 'constellation' | 'list' | 'graph';
const TABS: { key: TTab; label: string; title: string }[] = [
  { key: 'constellation', label: 'Live map', title: 'Agent constellation — living 3D mirror of the execution team' },
  { key: 'list', label: 'Roster', title: 'Agent roster — manage the team' },
  { key: 'graph', label: 'Flow', title: 'Execution flow — finisher gate tethers' },
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
    return TABS.some((x) => x.key === req) ? (req as TTab) : 'constellation';
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
          {tab === 'constellation' && <ExecutionTeamGraph project={project} onOpenGate={(id) => selectGate(id)} constellationMode />}
          {tab === 'list' && <AgentsManagerPane project={project} />}
          {tab === 'graph' && <ExecutionTeamGraph project={project} onOpenGate={(id) => selectGate(id)} />}
        </Suspense>
      </Box>
    </Box>
  );
}
