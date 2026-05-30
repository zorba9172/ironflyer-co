import { useMemo } from 'react';
import { Box, Chip, Divider, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
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
import { GlassPanel, SectionHeader, GaugeRing } from '../components/studio';
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

// Readiness score: fraction of non-blocking gates closed as a 0–100 integer.
function readinessPct(gates: Gate[]): number {
  if (gates.length === 0) return 0;
  const closed = gates.filter((g) => g.status === 'closed').length;
  return Math.round((closed / gates.length) * 100);
}

function ProfitGuardPanel({
  pg,
  forecast,
  economicsLive,
}: {
  pg: ReturnType<typeof deriveProfitGuard>;
  forecast: { remainingHeadroomUSD: number; burnRatePerHourUSD: number };
  economicsLive: boolean;
}) {
  const theme = useTheme();
  const accentColor =
    pg.tone === 'success'
      ? theme.studio.neon.success
      : pg.tone === 'warning'
      ? theme.studio.neon.warning
      : theme.studio.neon.danger;

  return (
    <Tooltip
      title={economicsLive ? pg.tip : 'Connect the orchestrator for live ProfitGuard verdicts'}
      arrow
    >
      <GlassPanel
        accent={accentColor}
        pad={2}
        sx={{ mb: 2, cursor: 'default', userSelect: 'none' }}
      >
        <Stack direction="row" alignItems="center" spacing={1.25} sx={{ mb: 1 }}>
          {/* live pulse dot */}
          <Box sx={{ position: 'relative', width: 10, height: 10, flexShrink: 0 }}>
            <Box
              sx={(t) => ({
                position: 'absolute', inset: 0, borderRadius: 99,
                bgcolor: t.palette[pg.tone].main,
              })}
            />
          </Box>
          <Typography
            sx={(t) => ({
              fontFamily: t.brand.font.mono,
              fontSize: text.s68,
              letterSpacing: '0.1em',
              textTransform: 'uppercase',
              color: 'text.disabled',
              flex: 1,
            })}
          >
            ProfitGuard
          </Typography>
          <Chip
            size="small"
            label={pg.verdict}
            sx={(t) => ({
              height: 20,
              fontSize: text.s62,
              fontFamily: t.brand.font.mono,
              bgcolor: `${t.palette[pg.tone].main}22`,
              color: t.palette[pg.tone].main,
              fontWeight: 700,
              textTransform: 'uppercase',
              letterSpacing: '0.06em',
            })}
          />
        </Stack>
        <Stack direction="row" alignItems="baseline" justifyContent="space-between" spacing={1}>
          <Stack spacing={0.25}>
            <Typography
              sx={(t) => ({
                fontFamily: t.brand.font.mono,
                fontSize: text.s95,
                fontWeight: 700,
                color: t.palette[pg.tone].main,
              })}
            >
              {formatUSD(forecast.remainingHeadroomUSD)}
            </Typography>
            <Typography sx={{ fontSize: text.s74, color: 'text.secondary' }}>
              remaining headroom
            </Typography>
          </Stack>
          {forecast.burnRatePerHourUSD > 0 && (
            <Stack spacing={0.25} alignItems="flex-end">
              <Typography
                sx={(t) => ({
                  fontFamily: t.brand.font.mono,
                  fontSize: text.s80,
                  fontWeight: 600,
                  color: 'text.primary',
                })}
              >
                {formatUSD(forecast.burnRatePerHourUSD)}/h
              </Typography>
              <Typography sx={{ fontSize: text.s68, color: 'text.secondary' }}>
                burn rate
              </Typography>
            </Stack>
          )}
        </Stack>
      </GlassPanel>
    </Tooltip>
  );
}

export function PreviewWorkspace({ project }: { project: StudioProject }) {
  const theme = useTheme();
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
  const readiness = readinessPct(gates);

  const { wallet } = useWallet();
  const { forecast, isLive: economicsLive } = useSentinelForecast(liveProjectId);
  const pg = deriveProfitGuard(forecast, wallet.availableUSD);
  const selectGate = useStudio((s) => s.selectGate);

  // "View all gates" drills into the most urgent gate's inspector: the first
  // blocking gate, else the first gate — a real reviewable path, not a no-op.
  const viewAllGates = () => {
    const target = gates.find((g) => g.status === 'blocked')
      ?? gates.find((g) => g.blocking)
      ?? gates[0];
    if (target) selectGate(target.id);
  };

  // Readiness arc color: full green when everything closed, orange when open, red when blocked.
  const readinessColor = useMemo(() => {
    const hasBlocked = gates.some((g) => g.status === 'blocked');
    const hasOpen = gates.some((g) => g.status === 'open' || g.status === 'running');
    if (hasBlocked) return theme.studio.neon.danger;
    if (hasOpen) return theme.studio.neon.warning;
    return theme.studio.neon.success;
  }, [gates, theme]);

  return (
    <Box sx={{ flex: 1, display: 'flex', minWidth: 0 }}>
      {/* ── Central preview canvas ── */}
      <Box sx={{ flex: 1.5, minWidth: 0, display: 'flex' }}>
        <PreviewPane gates={gates} />
      </Box>

      {/* ── Right live-build column ── */}
      <Box
        sx={(t) => ({
          width: 400,
          flexShrink: 0,
          borderLeft: `1px solid ${t.palette.divider}`,
          bgcolor: 'background.default',
          overflowY: 'auto',
          p: 2,
          display: 'flex',
          flexDirection: 'column',
          gap: 0,
        })}
      >
        {/* Column header */}
        <SectionHeader
          eyebrow="Live build"
          title={
            <Stack direction="row" alignItems="center" spacing={1.25}>
              <Box component="span">Live build</Box>
              <Chip
                size="small"
                label={isLive ? 'live' : 'sample'}
                sx={(t) => ({
                  height: 20,
                  fontSize: text.s62,
                  fontFamily: t.brand.font.mono,
                  bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover',
                  color: isLive ? 'success.main' : 'text.disabled',
                })}
              />
            </Stack>
          }
          subtitle={
            open > 0
              ? `${open} gate${open > 1 ? 's' : ''} open — blocking ship`
              : 'All gates closed — ready to ship'
          }
        />

        {/* Readiness gauge — the % lives inside the ring; the copy beside it
            names what's still open rather than repeating the number. */}
        <GlassPanel pad={2} sx={{ mb: 2 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Box sx={{ flexShrink: 0 }}>
              <GaugeRing
                value={readiness}
                color={readinessColor}
                formatter="{value}%"
                height={104}
              />
            </Box>
            <Stack spacing={0.5} sx={{ flex: 1, minWidth: 0 }}>
              <Typography
                sx={(t) => ({
                  fontFamily: t.brand.font.mono,
                  fontSize: text.s66,
                  letterSpacing: '0.1em',
                  textTransform: 'uppercase',
                  color: 'text.disabled',
                })}
              >
                Readiness
              </Typography>
              <Typography variant="subtitle2" sx={{ color: readinessColor, lineHeight: 1.2 }}>
                {open > 0 ? 'Blocking ship' : 'Ready to ship'}
              </Typography>
              <Typography sx={{ fontSize: text.s74, color: 'text.secondary' }}>
                {open > 0
                  ? `${open} gate${open > 1 ? 's' : ''} still open`
                  : 'All gates closed'}
              </Typography>
            </Stack>
          </Stack>
        </GlassPanel>

        {/* ProfitGuard status */}
        <ProfitGuardPanel pg={pg} forecast={forecast} economicsLive={economicsLive} />

        <Divider sx={{ mb: 2, opacity: 0.5 }} />

        {/* Agents rail */}
        <AgentsRail gates={gates} />

        <Divider sx={{ mb: 2, opacity: 0.5 }} />

        {/* Definition of Done */}
        <DefinitionOfDone
          gates={gates}
          onViewAll={viewAllGates}
          profitGuard={
            economicsLive
              ? {
                  verdict: pg.verdict,
                  reason:
                    pg.verdict === 'block'
                      ? 'over budget'
                      : 'tight ROI',
                }
              : undefined
          }
        />
      </Box>
    </Box>
  );
}
