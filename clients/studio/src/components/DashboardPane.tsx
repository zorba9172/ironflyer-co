import { useMemo, useState } from 'react';
import { Box, Button, Chip, LinearProgress, Stack, Tab, Tabs, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { formatUSD } from '@ironflyer/core';
import { text } from '@ironflyer/design-tokens/brand';
import { statusLabel, agentForGate, type Gate, type GateStatus, type StudioProject } from '../studioData';
import { mapGate, type GateVerdict } from '../lib/liveGates';
import { statusColor } from './statusColor';
import { AgentsRail } from './AgentsRail';
import { DefinitionOfDone } from './DefinitionOfDone';
import { ActivityFeed } from './ActivityFeed';
import { useStudio } from '../store';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useWallet, useSentinelForecast, buildActionCostPreview, buildGateSpendLabels, type GateSpendLabel } from '../hooks/useEconomics';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { StudioChart, donutOption, gaugeOption, type EChartsOption } from './charts';
import {
  GlassPanel,
  StatCard,
  SectionHeader,
  NeonBars3D,
  NeonConstellation3D,
} from './studio';
import type { Bar3DDatum } from '@ironflyer/ui-web/fx';
import type { Constellation3DNode, Constellation3DLink } from '@ironflyer/ui-web/fx';

const pgLabel: Record<StudioProject['profitGuard']['verdict'], string> = {
  allow: 'ProfitGuard: allow',
  hold: 'ProfitGuard: hold',
  block: 'ProfitGuard: block',
};

interface RawFile { path: string; size: number; language: string }
interface LedgerEntry { executionID?: string | null; entryType: string; direction: string; amountUSD: number }

const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

function icon(paths: string[], size = 16) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      {paths.map((d) => <path key={d} d={d} />)}
    </svg>
  );
}

const icons = {
  wallet: icon(['M3 9h18v10a2 2 0 01-2 2H5a2 2 0 01-2-2V9z', 'M3 9l9-7 9 7']),
  margin: icon(['M12 3v18', 'M3 12h18']),
  throughput: icon(['M3 12h4l2-5 4 10 2-5h6']),
  completion: icon(['M20 6L9 17l-5-5']),
  spark: icon(['M12 3l1.5 5.5L17 10l-3.5 1.5L12 17l-1.5-5.5L7 10l3.5-1.5z']),
};

function GateSpendRow({ g }: { g: GateSpendLabel }) {
  const theme = useTheme();
  const tone = theme.studio.chart.series[0] ?? theme.palette.primary.main;
  return (
    <Box>
      <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 0.5 }}>
        <Typography sx={{ fontSize: text.s82, color: 'text.secondary' }} noWrap>
          {g.agentName} {'·'} {g.gateName}
        </Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s78, color: 'text.primary', flexShrink: 0 })}>
          {formatUSD(g.amountUSD)}
        </Typography>
      </Stack>
      <Box sx={{ height: 5, borderRadius: 99, bgcolor: 'action.hover', overflow: 'hidden' }}>
        <Box sx={{ width: `${g.sharePct}%`, height: '100%', borderRadius: 99, bgcolor: tone }} />
      </Box>
    </Box>
  );
}

function GateCard({ g, spend }: { g: Gate; spend?: GateSpendLabel }) {
  const selectGate = useStudio((s) => s.selectGate);
  const agentName = spend?.agentName ?? agentForGate(g.id)?.name ?? 'Unassigned';
  return (
    <GlassPanel
      interactive
      onClick={() => selectGate(g.id)}
      pad={2.5}
      sx={{ display: 'flex', flexDirection: 'column', gap: 1.25 }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Stack direction="row" alignItems="center" spacing={1.25}>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s78, color: 'text.disabled' })}>{g.no}</Typography>
          <Typography variant="h6" sx={{ fontSize: text.s105 }}>{g.name}</Typography>
        </Stack>
        <Chip
          size="small"
          label={statusLabel[g.status]}
          sx={(t) => ({ bgcolor: `${statusColor(t, g.status)}22`, color: statusColor(t, g.status), fontWeight: 600, fontSize: text.s70 })}
        />
      </Stack>

      <LinearProgress
        variant="determinate"
        value={Math.round(g.level * 100)}
        sx={(t) => ({
          height: 5,
          borderRadius: 99,
          bgcolor: 'action.hover',
          '& .MuiLinearProgress-bar': { borderRadius: 99, bgcolor: statusColor(t, g.status) },
        })}
      />

      {g.blocking ? (
        <Typography sx={{ fontSize: text.s82, color: 'text.secondary' }}>
          <Box component="span" sx={{ color: g.status === 'blocked' ? 'error.main' : 'warning.main' }}>
            {'●'}{' '}
          </Box>
          {g.blocking}
        </Typography>
      ) : (
        <Typography sx={{ fontSize: text.s82, color: 'success.main' }}>{'●'} Closed end-to-end</Typography>
      )}

      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ pt: 0.5 }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s72, color: 'text.secondary' })}>{agentName}</Typography>
        <Box sx={{ width: 3, height: 3, borderRadius: 99, bgcolor: 'text.disabled' }} />
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s72, color: 'text.disabled' })}>
          {spend ? `${formatUSD(spend.amountUSD)} · ${spend.sharePct}%` : `${g.findings.length}f · ${g.patches.length}p`}
        </Typography>
        <Typography sx={{ ml: 'auto', fontSize: text.s72, color: 'primary.main' }}>Open</Typography>
      </Stack>
    </GlassPanel>
  );
}

export function DashboardPane({ fallback }: { projectId: string; fallback: StudioProject }) {
  const theme = useTheme();
  const [tab, setTab] = useState(0);
  const firstProjectId = useLiveProjectId();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const liveProjectId = storeProjectId ?? firstProjectId;
  const { wallet } = useWallet();
  const { executions, economics } = useProjectExecutions(liveProjectId);
  const { forecast } = useSentinelForecast(liveProjectId);
  const { data: liveGates, isLive } = useGraphQLQuery<Gate[], { gates: GateVerdict[] }>({
    key: ['gates', liveProjectId ?? 'none'],
    operationName: 'Gates',
    query: operations.GATES,
    variables: { projectId: liveProjectId },
    fallbackData: [],
    enabled: !!liveProjectId,
    map: (raw) => raw.gates.map(mapGate),
  });

  const { data: files } = useGraphQLQuery<RawFile[], { projectFiles: RawFile[] }>({
    key: ['dash-files', liveProjectId ?? 'none'],
    operationName: 'ProjectFiles', query: operations.PROJECT_FILES,
    variables: { id: liveProjectId }, fallbackData: [], enabled: !!liveProjectId,
    map: (r) => r.projectFiles,
  });
  const { data: ledger } = useGraphQLQuery<LedgerEntry[], { ledger: LedgerEntry[] }>({
    key: ['ledger', 'usage'], operationName: 'Ledger', query: operations.LEDGER,
    variables: { filter: { limit: 200 } }, fallbackData: [], map: (r) => r.ledger ?? [],
  });

  const languages = useMemo(() => {
    const by = new Map<string, number>();
    for (const f of files) {
      const l = (f.language || '').trim();
      if (!l || l === 'file') continue;
      by.set(l, (by.get(l) ?? 0) + (f.size || 1));
    }
    const total = [...by.values()].reduce((a, b) => a + b, 0);
    return total === 0 ? [] : [...by.entries()].map(([name, size]) => ({ name, pct: Math.round((size / total) * 100) })).sort((a, b) => b.pct - a.pct).slice(0, 5);
  }, [files]);

  const gates = isLive && liveGates.length > 0 ? liveGates : fallback.gates;
  const completion = gates.length ? gates.filter((g) => !g.blocking).length / gates.length : 0;
  const p = { ...fallback, gates, completion };

  const open = p.gates.filter((g) => g.blocking).length;
  const displaySpendUSD = economics.spentUSD > 0 ? economics.spentUSD : fallback.meters.walletUsed;
  const displayBudgetUSD = economics.budgetUSD > 0 ? economics.budgetUSD : fallback.meters.walletBudget;
  const displayMarginPct = economics.runs > 0 || economics.revenueUSD > 0 ? economics.marginPct : fallback.meters.marginPct;
  const displayThroughput = fallback.meters.throughput;

  const actionPreview = useMemo(() => buildActionCostPreview({
    wallet,
    forecast,
    recentExecutionSpendUSD: executions.map((e) => e.spentUSD || e.budgetUSD),
    fallbackEstimateUSD: fallback.profitGuard.reservedUSD || 2.4,
  }), [wallet, forecast, executions, fallback.profitGuard.reservedUSD]);

  const gateSpend = useMemo(() => buildGateSpendLabels(
    p.gates.map((g) => ({ ...g, agentName: agentForGate(g.id)?.name })),
    economics.spentUSD,
    fallback.meters.walletUsed,
  ), [p.gates, economics.spentUSD, fallback.meters.walletUsed]);

  const spendByGate = useMemo(() => new Map(gateSpend.map((g) => [g.gateId, g])), [gateSpend]);

  const byEntryType = useMemo(() => {
    const execIds = new Set(executions.map((e) => e.id));
    const scoped = execIds.size > 0 ? ledger.filter((e) => e.executionID && execIds.has(e.executionID)) : ledger;
    const src = scoped.length > 0 ? scoped : ledger;
    const m = new Map<string, number>();
    for (const e of src) {
      if (e.direction !== 'debit' || e.amountUSD <= 0) continue;
      m.set(e.entryType, (m.get(e.entryType) ?? 0) + e.amountUSD);
    }
    return [...m.entries()].map(([name, value]) => ({ name: titleCase(name), value }));
  }, [ledger, executions]);

  const ledgerOption = useMemo<EChartsOption>(() => donutOption(theme, {
    data: byEntryType,
    emptyLabel: 'No ledger entries yet',
    radius: ['52%', '74%'],
  }), [byEntryType, theme]);

  const walletPct = displayBudgetUSD > 0 ? Math.min(100, Math.round((displaySpendUSD / displayBudgetUSD) * 100)) : 0;
  const walletOption = useMemo<EChartsOption>(() => gaugeOption(theme, {
    value: walletPct,
    color: theme.palette.primary.main,
    formatter: `${walletPct}%`,
    radius: '92%',
  }), [walletPct, theme]);

  const gateDonut = useMemo<EChartsOption>(() => {
    const order: GateStatus[] = ['closed', 'running', 'open', 'blocked', 'unstarted'];
    const counts = new Map<GateStatus, number>();
    for (const g of p.gates) counts.set(g.status, (counts.get(g.status) ?? 0) + 1);
    const data = order.filter((s) => (counts.get(s) ?? 0) > 0).map((s) => ({
      name: statusLabel[s],
      value: counts.get(s) ?? 0,
      color: statusColor(theme, s),
    }));
    return donutOption(theme, { data, radius: ['60%', '82%'] });
  }, [p.gates, theme]);

  // Cost-share bars: each bar = one gate's share of total provider cost (2D, data-bound)
  const bars3dData = useMemo<Bar3DDatum[]>(() =>
    gateSpend.map((g, i) => ({
      label: g.gateName,
      value: Math.max(0.01, g.sharePct),
      color: theme.studio.chart.series[i % theme.studio.chart.series.length],
    })),
    [gateSpend, theme.studio.chart.series],
  );

  // Topology network: each node = a real gate or agent, each link = an orchestrator dispatch path (2D)
  const constellationNodes = useMemo<Constellation3DNode[]>(() => {
    const nodes: Constellation3DNode[] = [];
    p.gates.forEach((g, i) => {
      const agent = agentForGate(g.id);
      nodes.push({
        id: `gate_${g.id}`,
        value: Math.round(g.level * 100),
        color: statusColor(theme, g.status),
        x: Math.cos((i / p.gates.length) * Math.PI * 2) * 0.6,
        y: 0,
        z: Math.sin((i / p.gates.length) * Math.PI * 2) * 0.6,
      });
      if (agent) {
        nodes.push({
          id: `agent_${agent.id}`,
          value: 50,
          color: theme.studio.chart.series[1],
          x: Math.cos((i / p.gates.length) * Math.PI * 2) * 1.1,
          y: 0.4,
          z: Math.sin((i / p.gates.length) * Math.PI * 2) * 1.1,
        });
      }
    });
    nodes.push({ id: 'orchestrator', value: 80, color: theme.studio.neon.violet, x: 0, y: 0, z: 0 });
    return nodes;
  }, [p.gates, theme]);

  const constellationLinks = useMemo<Constellation3DLink[]>(() => {
    const links: Constellation3DLink[] = [];
    p.gates.forEach((g) => {
      links.push({ source: 'orchestrator', target: `gate_${g.id}` });
      const agent = agentForGate(g.id);
      if (agent) links.push({ source: `gate_${g.id}`, target: `agent_${agent.id}` });
    });
    return links;
  }, [p.gates]);

  const pgColor = p.profitGuard.verdict === 'block'
    ? theme.palette.error.main
    : p.profitGuard.verdict === 'hold'
      ? theme.palette.warning.main
      : theme.palette.success.main;

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1160, mx: 'auto' }}>
        <SectionHeader
          eyebrow="Finisher workspace"
          title={
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <span>Command Center</span>
              <Chip
                size="small"
                label={isLive ? 'live' : 'sample data'}
                sx={(t) => ({
                  height: 20,
                  fontSize: text.s64,
                  fontFamily: t.brand.font.mono,
                  bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover',
                  color: isLive ? 'success.main' : 'text.disabled',
                })}
              />
              <Chip
                size="small"
                label={pgLabel[p.profitGuard.verdict]}
                sx={{ height: 20, fontSize: text.s64, bgcolor: `${pgColor}22`, color: pgColor }}
              />
            </Stack>
          }
          subtitle={`${open} of ${p.gates.length} gates open · ${Math.round(p.completion * 100)}% to shippable`}
          actions={
            <Button variant="contained" color="primary" size="small" startIcon={icons.spark} sx={{ borderRadius: 999 }}>
              Run Finisher
            </Button>
          }
        />

        <LinearProgress
          variant="determinate"
          value={Math.round(p.completion * 100)}
          sx={{
            height: 6,
            borderRadius: 99,
            bgcolor: 'action.hover',
            mb: 2.5,
            '& .MuiLinearProgress-bar': { borderRadius: 99, backgroundImage: (t) => t.brand.gradient.signature },
          }}
        />

        <Tabs
          value={tab}
          onChange={(_, v: number) => setTab(v)}
          sx={{
            mb: 2,
            minHeight: 0,
            borderBottom: 1,
            borderColor: 'divider',
            '& .MuiTab-root': { textTransform: 'none', minHeight: 40, fontWeight: 600 },
          }}
        >
          <Tab label="Overview" />
          <Tab label={`Gates (${p.gates.length})`} />
          <Tab label="Usage" />
          <Tab label="Activity" />
        </Tabs>

        {tab === 0 && (
          <>
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 1.5 }}>
              <StatCard
                label="Wallet balance"
                value={formatUSD(wallet.availableUSD)}
                hint={`${formatUSD(displaySpendUSD)} spent`}
                accent={theme.studio.neon.blue}
                icon={icons.wallet}
              />
              <StatCard
                label="Margin (project)"
                value={`${displayMarginPct}%`}
                hint="revenue minus provider cost"
                accent={theme.studio.neon.success}
                icon={icons.margin}
              />
              <StatCard
                label="Throughput"
                value={`${displayThroughput} r/min`}
                hint={`${economics.runs} execution${economics.runs === 1 ? '' : 's'}`}
                accent={theme.studio.neon.violet}
                icon={icons.throughput}
              />
              <StatCard
                label="Completion"
                value={`${Math.round(p.completion * 100)}%`}
                hint={`${p.gates.filter((g) => !g.blocking).length}/${p.gates.length} gates closed`}
                accent={theme.studio.neon.pink}
                icon={icons.completion}
              />
            </Box>

            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 320px' }, gap: 1.5, mb: 1.5 }}>
              <GlassPanel pad={2.5}>
                <SectionHeader
                  eyebrow="Cost share by gate"
                  title="Provider spend"
                  subtitle="Each bar is a gate's share of total provider cost, bound to the live ledger"
                />
                {bars3dData.length > 0
                  ? <NeonBars3D data={bars3dData} height={220} />
                  : (
                    <Box sx={{ height: 120, display: 'grid', placeItems: 'center' }}>
                      <Typography sx={{ color: 'text.disabled', fontSize: text.s82 }}>No spend data yet</Typography>
                    </Box>
                  )
                }
              </GlassPanel>

              <GlassPanel pad={2.5}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>
                  Gate status
                </Typography>
                <StudioChart option={gateDonut} height={220} />
              </GlassPanel>
            </Box>

            <GlassPanel pad={2.5} sx={{ mb: 1.5 }}>
              <SectionHeader
                eyebrow="Gate and agent graph"
                title="Execution topology"
                subtitle="Each node is a real gate or agent; links mirror the orchestrator dispatch paths"
              />
              <NeonConstellation3D nodes={constellationNodes} links={constellationLinks} height={240} />
            </GlassPanel>

            <AgentsRail gates={p.gates} />
            <DefinitionOfDone gates={p.gates} />

            {languages.length > 0 && (
              <GlassPanel pad={2.5} sx={{ mt: 2 }}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>
                  Composition
                </Typography>
                <Stack spacing={1.25}>
                  {languages.map((l) => (
                    <Box key={l.name}>
                      <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.5 }}>
                        <Typography sx={{ fontSize: text.s85, fontWeight: 600 }}>{l.name}</Typography>
                        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s80, color: 'text.secondary' })}>{l.pct}%</Typography>
                      </Stack>
                      <LinearProgress
                        variant="determinate"
                        value={l.pct}
                        sx={{ height: 6, borderRadius: 99, bgcolor: 'action.hover', '& .MuiLinearProgress-bar': { borderRadius: 99, backgroundImage: (t) => t.brand.gradient.signature } }}
                      />
                    </Box>
                  ))}
                </Stack>
              </GlassPanel>
            )}
          </>
        )}

        {tab === 1 && (
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 1.5 }}>
            {p.gates.map((g) => <GateCard key={g.id} g={g} spend={spendByGate.get(g.id)} />)}
          </Box>
        )}

        {tab === 2 && (
          <>
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 2 }}>
              <StatCard label="Project spend" value={formatUSD(displaySpendUSD)} hint={`of ${formatUSD(displayBudgetUSD)} reserved`} />
              <StatCard label="Provider cost" value={formatUSD(economics.providerCostUSD)} hint="metered to the ledger" />
              <StatCard label="Next action est." value={formatUSD(actionPreview.estimateUSD)} hint={actionPreview.detail} />
              <StatCard label="Margin (project)" value={`${displayMarginPct}%`} hint={`revenue ${formatUSD(economics.revenueUSD)}`} />
            </Box>

            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 2, mb: 2 }}>
              <GlassPanel pad={2.5}>
                <Stack direction="row" justifyContent="space-between" alignItems="baseline" sx={{ mb: 1 }}>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, textTransform: 'uppercase', color: 'text.disabled' })}>
                    Budget spent
                  </Typography>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled' })}>
                    wallet {formatUSD(wallet.availableUSD)}
                  </Typography>
                </Stack>
                <StudioChart option={walletOption} height={240} />
              </GlassPanel>

              <GlassPanel pad={2.5}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>
                  Spend by ledger type
                </Typography>
                <StudioChart option={ledgerOption} height={240} />
              </GlassPanel>
            </Box>

            <GlassPanel pad={2.5}>
              <Stack direction="row" justifyContent="space-between" alignItems="baseline" sx={{ mb: 1.5 }}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, textTransform: 'uppercase', color: 'text.disabled' })}>
                  Spend by gate and agent
                </Typography>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled' })}>
                  {gateSpend[0]?.source === 'live' ? 'live execution spend' : 'fallback gate allocation'}
                </Typography>
              </Stack>
              <Stack spacing={1.15}>
                {gateSpend.map((g) => <GateSpendRow key={g.gateId} g={g} />)}
              </Stack>
            </GlassPanel>
          </>
        )}

        {tab === 3 && <ActivityFeed projectId={p.id} seed={p.activity} />}
      </Box>
    </Box>
  );
}
