import { Box, Chip, Stack, Typography } from '@mui/material';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { mapGate, type GateVerdict } from '../lib/liveGates';
import { type Gate, type StudioProject } from '../studioData';
import { useStudio } from '../store';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { PreviewPane } from '../components/PreviewPane';
import { AgentsRail } from '../components/AgentsRail';
import { DefinitionOfDone } from '../components/DefinitionOfDone';

// Build theater: the running app and the live build state, side by side. This
// is the moat made visceral — Base44-speed (the app renders instantly on the
// left) AND the discipline layer no fast studio has (the AI team + Definition
// of Done on the right, updating live). One glance: it runs, and here's exactly
// what's still not closed end-to-end before it can ship.
export function TheaterPane({ project }: { project: StudioProject }) {
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const liveProjectId = storeProjectId ?? firstProjectId;

  const { data: liveGates, isLive } = useGraphQLQuery<Gate[], { gates: GateVerdict[] }>({
    key: ['theater-gates', liveProjectId ?? 'none'],
    operationName: 'Gates', query: operations.GATES,
    variables: { projectId: liveProjectId }, fallbackData: [], enabled: !!liveProjectId,
    refetchInterval: 6000, map: (r) => r.gates.map(mapGate),
  });
  const gates = isLive && liveGates.length > 0 ? liveGates : project.gates;
  const open = gates.filter((g) => g.blocking).length;

  return (
    <Box sx={{ flex: 1, display: 'flex', minWidth: 0 }}>
      {/* Left: the running app, instant. */}
      <Box sx={{ flex: 1.5, minWidth: 0, display: 'flex' }}>
        <PreviewPane />
      </Box>

      {/* Right: live build rail — the AI team + Definition of Done. */}
      <Box sx={{ width: 400, flexShrink: 0, borderLeft: 1, borderColor: 'divider', bgcolor: 'background.default', overflowY: 'auto', p: 2.5 }}>
        <Stack direction="row" alignItems="center" spacing={1.25} sx={{ mb: 2 }}>
          <Typography variant="h6" sx={{ fontSize: '1.05rem' }}>Live build</Typography>
          <Chip
            size="small"
            label={isLive ? 'live' : 'sample'}
            sx={(t) => ({ height: 20, fontSize: '0.62rem', fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
          />
          <Typography sx={{ ml: 'auto', fontSize: '0.82rem', color: open > 0 ? 'warning.main' : 'success.main' }}>
            {open > 0 ? `${open} open` : 'all closed'}
          </Typography>
        </Stack>

        <AgentsRail gates={gates} />
        <DefinitionOfDone gates={gates} />
      </Box>
    </Box>
  );
}
