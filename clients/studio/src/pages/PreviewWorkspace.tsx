import { Box, Chip, Stack, Tooltip, Typography } from '@mui/material';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { formatUSD } from '@ironflyer/core';
import { mapGate, type GateVerdict } from '../lib/liveGates';
import { type Gate, type StudioProject } from '../studioData';
import { useStudio } from '../store';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useWallet, useSentinelForecast } from '../hooks/useEconomics';
import { PreviewPane } from '../components/PreviewPane';
import { AgentsRail } from '../components/AgentsRail';
import { DefinitionOfDone } from '../components/DefinitionOfDone';
import { text } from '@ironflyer/design-tokens/brand';

// Derive the visible ProfitGuard state from the economics signals available
// today until the API exposes a first-class decision field.
function deriveProfitGuard(
  forecast: { level: string; remainingHeadroomUSD: number; extrapolatedTotalUSD: number; burnRatePerHourUSD: number },
  available: number,
): { verdict: 'allow' | 'hold' | 'block'; tone: 'error' | 'warning' | 'success'; tip: string } {
  const exhausted = available <= 0 || forecast.remainingHeadroomUSD < 0;
  const tight = !exhausted && (forecast.level === 'warn' || forecast.level === 'critical' || forecast.remainingHeadroomUSD < 1);
  if (exhausted) {
    return { verdict: 'block', tone: 'error', tip: `Blocked — ${formatUSD(available)} available, ${formatUSD(forecast.remainingHeadroomUSD)} headroom. Top up before the next paid step (402).` };
  }
  if (tight) {
    return { verdict: 'hold', tone: 'warning', tip: `Held for ROI — projected ${formatUSD(forecast.extrapolatedTotalUSD)} at ${formatUSD(forecast.burnRatePerHourUSD)}/h, ${formatUSD(forecast.remainingHeadroomUSD)} headroom left.` };
  }
  return { verdict: 'allow', tone: 'success', tip: `Clears the next step — ${formatUSD(forecast.remainingHeadroomUSD)} headroom, ${formatUSD(available)} available.` };
}

export function PreviewWorkspace({ project }: { project: StudioProject }) {
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const liveProjectId = storeProjectId ?? firstProjectId;

  const { data: liveGates, isLive } = useGraphQLQuery<Gate[], { gates: GateVerdict[] }>({
    key: ['preview-gates', liveProjectId ?? 'none'],
    operationName: 'Gates', query: operations.GATES,
    variables: { projectId: liveProjectId }, fallbackData: [], enabled: !!liveProjectId,
    refetchInterval: 6000, map: (r) => r.gates.map(mapGate),
  });
  const gates = isLive && liveGates.length > 0 ? liveGates : project.gates;
  const open = gates.filter((g) => g.blocking).length;

  const { wallet } = useWallet();
  const { forecast, isLive: economicsLive } = useSentinelForecast(liveProjectId);
  const pg = deriveProfitGuard(forecast, wallet.availableUSD);

  return (
    <Box sx={{ flex: 1, display: 'flex', minWidth: 0 }}>
      <Box sx={{ flex: 1.5, minWidth: 0, display: 'flex' }}>
        <PreviewPane />
      </Box>

      <Box sx={{ width: 400, flexShrink: 0, borderLeft: 1, borderColor: 'divider', bgcolor: 'background.default', overflowY: 'auto', p: 2.5 }}>
        <Stack direction="row" alignItems="center" spacing={1.25} sx={{ mb: 2 }}>
          <Typography variant="h6" sx={{ fontSize: text.s105 }}>Live build</Typography>
          <Chip
            size="small"
            label={isLive ? 'live' : 'sample'}
            sx={(t) => ({ height: 20, fontSize: text.s62, fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
          />
          <Typography sx={{ ml: 'auto', fontSize: text.s82, color: open > 0 ? 'warning.main' : 'success.main' }}>
            {open > 0 ? `${open} open` : 'all closed'}
          </Typography>
        </Stack>

        <Tooltip title={economicsLive ? pg.tip : 'Connect the orchestrator for live ProfitGuard verdicts'} arrow>
          <Stack
            direction="row"
            alignItems="center"
            spacing={1}
            sx={(t) => ({
              px: 1.25, py: 0.75, mb: 2, borderRadius: 2,
              border: 1, borderColor: `${t.palette[pg.tone].main}55`, bgcolor: `${t.palette[pg.tone].main}14`,
              cursor: 'default', userSelect: 'none',
            })}
          >
            <Box sx={(t) => ({ width: 8, height: 8, borderRadius: 99, flexShrink: 0, bgcolor: t.palette[pg.tone].main })} />
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'text.disabled' })}>
              ProfitGuard
            </Typography>
            <Chip
              size="small"
              label={pg.verdict}
              sx={(t) => ({ height: 18, fontSize: text.s62, fontFamily: t.brand.font.mono, bgcolor: `${t.palette[pg.tone].main}22`, color: t.palette[pg.tone].main, fontWeight: 600 })}
            />
            <Typography sx={(t) => ({ ml: 'auto', fontFamily: t.brand.font.mono, fontSize: text.s70, color: t.palette[pg.tone].main })}>
              {formatUSD(forecast.remainingHeadroomUSD)} headroom
            </Typography>
          </Stack>
        </Tooltip>

        <AgentsRail gates={gates} />
        <DefinitionOfDone
          gates={gates}
          profitGuard={economicsLive ? { verdict: pg.verdict, reason: pg.verdict === 'block' ? 'over budget' : 'tight ROI' } : undefined}
        />
      </Box>
    </Box>
  );
}
