import { useMemo, useState } from 'react';
import { Box, Card, Chip, LinearProgress, Stack, Tab, Tabs, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { formatUSD } from '@ironflyer/core';
import { palette, modes } from '@ironflyer/design-tokens/brand';
import { statusLabel, agentForGate, type Gate, type GateStatus, type StudioProject } from '../studioData';
import { mapGate, type GateVerdict } from '../lib/liveGates';
import { statusColor } from './statusColor';
import { AgentsRail } from './AgentsRail';
import { DefinitionOfDone } from './DefinitionOfDone';
import { ActivityFeed } from './ActivityFeed';
import { useStudio } from '../store';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useWallet } from '../hooks/useEconomics';
import { useProjectExecutions } from '../hooks/useLatestExecution';

const pgLabel: Record<StudioProject['profitGuard']['verdict'], string> = {
  allow: 'ProfitGuard: allow',
  hold: 'ProfitGuard: hold',
  block: 'ProfitGuard: block',
};

interface RawFile { path: string; size: number; language: string }
interface LedgerEntry { executionID?: string | null; entryType: string; direction: string; amountUSD: number }

const SERIES_COLORS = [palette.cobalt, palette.cyan, palette.amber, palette.emerald, palette.rose, modes.dark.textMuted];
const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

function GateCard({ g }: { g: Gate }) {
  const selectGate = useStudio((s) => s.selectGate);
  return (
    <Card
      onClick={() => selectGate(g.id)}
      sx={{ p: 2.5, display: 'flex', flexDirection: 'column', gap: 1.25, cursor: 'pointer', transition: (t) => `border-color ${t.brand.motion.fast}`, '&:hover': { borderColor: 'text.disabled' } }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Stack direction="row" alignItems="center" spacing={1.25}>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.78rem', color: 'text.disabled' })}>{g.no}</Typography>
          <Typography variant="h6" sx={{ fontSize: '1.05rem' }}>{g.name}</Typography>
        </Stack>
        <Chip size="small" label={statusLabel[g.status]} sx={(t) => ({ bgcolor: `${statusColor(t, g.status)}22`, color: statusColor(t, g.status), fontWeight: 600, fontSize: '0.7rem' })} />
      </Stack>

      <LinearProgress variant="determinate" value={Math.round(g.level * 100)} sx={(t) => ({ height: 5, borderRadius: 99, bgcolor: 'action.hover', '& .MuiLinearProgress-bar': { borderRadius: 99, bgcolor: statusColor(t, g.status) } })} />

      {g.blocking ? (
        <Typography sx={{ fontSize: '0.82rem', color: 'text.secondary' }}>
          <Box component="span" sx={{ color: g.status === 'blocked' ? 'error.main' : 'warning.main' }}>● </Box>{g.blocking}
        </Typography>
      ) : (
        <Typography sx={{ fontSize: '0.82rem', color: 'success.main' }}>● Closed end-to-end</Typography>
      )}

      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ pt: 0.5 }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.secondary' })}>{agentForGate(g.id)?.name ?? 'Unassigned'}</Typography>
        <Box sx={{ width: 3, height: 3, borderRadius: 99, bgcolor: 'text.disabled' }} />
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.disabled' })}>{g.findings.length}f · {g.patches.length}p</Typography>
        <Typography sx={{ ml: 'auto', fontSize: '0.72rem', color: 'primary.main' }}>Open</Typography>
      </Stack>
    </Card>
  );
}

function Meter({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <Card sx={{ p: 2.5, flex: 1 }}>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>{label}</Typography>
      <Typography variant="h4" sx={{ fontSize: '1.8rem', mt: 0.5 }}>{value}</Typography>
      {sub && <Typography sx={{ fontSize: '0.78rem', color: 'text.secondary' }}>{sub}</Typography>}
    </Card>
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
  const { data: liveGates, isLive } = useGraphQLQuery<Gate[], { gates: GateVerdict[] }>({
    key: ['gates', liveProjectId ?? 'none'],
    operationName: 'Gates',
    query: operations.GATES,
    variables: { projectId: liveProjectId },
    fallbackData: [],
    enabled: !!liveProjectId,
    map: (raw) => raw.gates.map(mapGate),
  });

  // Language composition (merged in from the former Intelligence tab).
  const { data: files } = useGraphQLQuery<RawFile[], { projectFiles: RawFile[] }>({
    key: ['dash-files', liveProjectId ?? 'none'],
    operationName: 'ProjectFiles', query: operations.PROJECT_FILES,
    variables: { id: liveProjectId }, fallbackData: [], enabled: !!liveProjectId,
    map: (r) => r.projectFiles,
  });
  // Ledger feed for the Usage tab's spend breakdown (moved here from the
  // former standalone Usage pane — usage now lives inside the dashboard).
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

  // Live gates when connected; otherwise the sample project. Meters/activity
  // stay sample until the snapshot mapping lands.
  const gates = isLive && liveGates.length > 0 ? liveGates : fallback.gates;
  const completion = gates.length ? gates.filter((g) => !g.blocking).length / gates.length : 0;
  const p = { ...fallback, gates, completion };

  const open = p.gates.filter((g) => g.blocking).length;
  const gatesClosed = p.gates.filter((g) => !g.blocking).length;

  // --- Usage tab: spend breakdown + budget gauge (ported from UsagePane) ---
  const axis = theme.palette.text.secondary;
  const grid = theme.palette.divider;

  const byEntryType = useMemo(() => {
    const execIds = new Set(executions.map((e) => e.id));
    const scoped = execIds.size > 0 ? ledger.filter((e) => e.executionID && execIds.has(e.executionID)) : ledger;
    const src = scoped.length > 0 ? scoped : ledger;
    const m = new Map<string, number>();
    for (const e of src) m.set(e.entryType, (m.get(e.entryType) ?? 0) + 1);
    return [...m.entries()].map(([name, value]) => ({ name: titleCase(name), value }));
  }, [ledger, executions]);

  const ledgerOption = useMemo<EChartsOption>(() => ({
    color: SERIES_COLORS,
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, type: 'scroll', textStyle: { color: axis, fontSize: 10 } },
    series: [{
      type: 'pie', radius: ['52%', '74%'], avoidLabelOverlap: true,
      itemStyle: { borderColor: theme.palette.background.paper, borderWidth: 2 },
      label: { show: false }, data: byEntryType.length ? byEntryType : [{ name: 'No ledger entries yet', value: 1 }],
    }],
  }), [byEntryType, axis, theme]);

  const walletPct = economics.budgetUSD > 0 ? Math.min(100, Math.round((economics.spentUSD / economics.budgetUSD) * 100)) : 0;
  const walletOption = useMemo<EChartsOption>(() => ({
    series: [{
      type: 'gauge', startAngle: 210, endAngle: -30, min: 0, max: 100, radius: '92%',
      progress: { show: true, width: 14, itemStyle: { color: palette.cobalt } },
      axisLine: { lineStyle: { width: 14, color: [[1, grid]] } },
      axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false }, pointer: { show: false },
      anchor: { show: false },
      detail: { valueAnimation: true, formatter: `${walletPct}%`, color: theme.palette.text.primary, fontSize: 26, offsetCenter: [0, 0] },
      data: [{ value: walletPct }],
    }],
  }), [walletPct, grid, theme]);

  // Visual summary: gate status distribution donut.
  const gateDonut = useMemo<EChartsOption>(() => {
    const order: GateStatus[] = ['closed', 'running', 'open', 'blocked', 'unstarted'];
    const counts = new Map<GateStatus, number>();
    for (const g of p.gates) counts.set(g.status, (counts.get(g.status) ?? 0) + 1);
    const data = order.filter((s) => (counts.get(s) ?? 0) > 0).map((s) => ({ name: statusLabel[s], value: counts.get(s) ?? 0, itemStyle: { color: statusColor(theme, s) } }));
    return {
      tooltip: { trigger: 'item' },
      legend: { bottom: 0, textStyle: { color: theme.palette.text.secondary, fontSize: 11 } },
      series: [{ type: 'pie', radius: ['55%', '78%'], avoidLabelOverlap: true, itemStyle: { borderColor: theme.palette.background.paper, borderWidth: 2 }, label: { show: false }, data }],
    };
  }, [p.gates, theme]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 0.5 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Finisher</Typography>
            <Chip
              size="small"
              label={isLive ? 'live' : 'sample data'}
              sx={(t) => ({ height: 20, fontSize: '0.64rem', fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
            />
            <Chip
              size="small"
              label={pgLabel[p.profitGuard.verdict]}
              sx={(t) => {
                const c = p.profitGuard.verdict === 'block' ? t.palette.error.main : p.profitGuard.verdict === 'hold' ? t.palette.warning.main : t.palette.success.main;
                return { height: 20, fontSize: '0.64rem', fontFamily: t.brand.font.mono, bgcolor: `${c}22`, color: c };
              }}
            />
          </Stack>
          <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{open} of {p.gates.length} gates open · {Math.round(p.completion * 100)}% to shippable</Typography>
        </Stack>
        <LinearProgress variant="determinate" value={Math.round(p.completion * 100)} sx={{ height: 6, borderRadius: 99, bgcolor: 'action.hover', mb: 2, '& .MuiLinearProgress-bar': { borderRadius: 99, backgroundImage: (t) => t.brand.gradient.signature } }} />

        <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 3, minHeight: 0, borderBottom: 1, borderColor: 'divider', '& .MuiTab-root': { textTransform: 'none', minHeight: 40 } }}>
          <Tab label="Summary" />
          <Tab label={`Gates (${p.gates.length})`} />
          <Tab label="Usage" />
          <Tab label="Activity" />
        </Tabs>

        {/* TAB 1 — visual summary */}
        {tab === 0 && (
          <>
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 1.5, mb: 1.5 }}>
              <Card sx={{ p: 2.5 }}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled' })}>Gate status</Typography>
                <Chart option={gateDonut} height={240} />
              </Card>
              <Stack spacing={1.5}>
                <Meter label="Project spend" value={formatUSD(economics.spentUSD)} sub={`of ${formatUSD(economics.budgetUSD)} reserved · wallet ${formatUSD(wallet.availableUSD)}`} />
                <Meter label="Margin (project)" value={`${economics.marginPct}%`} sub="revenue − provider cost" />
                <Meter label="Provider cost" value={formatUSD(economics.providerCostUSD)} sub={`${economics.runs} execution${economics.runs === 1 ? '' : 's'}`} />
              </Stack>
            </Box>

            <AgentsRail gates={p.gates} />

            <DefinitionOfDone gates={p.gates} />

            {languages.length > 0 && (
              <Card sx={{ p: 2.5, mt: 1.5 }}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Composition</Typography>
                <Stack spacing={1.25}>
                  {languages.map((l) => (
                    <Box key={l.name}>
                      <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.5 }}>
                        <Typography sx={{ fontSize: '0.85rem', fontWeight: 600 }}>{l.name}</Typography>
                        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.8rem', color: 'text.secondary' })}>{l.pct}%</Typography>
                      </Stack>
                      <LinearProgress variant="determinate" value={l.pct} sx={{ height: 6, borderRadius: 99, bgcolor: 'action.hover', '& .MuiLinearProgress-bar': { borderRadius: 99, backgroundImage: (t) => t.brand.gradient.signature } }} />
                    </Box>
                  ))}
                </Stack>
              </Card>
            )}
          </>
        )}

        {/* TAB 2 — gates */}
        {tab === 1 && (
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 1.5 }}>
            {p.gates.map((g) => <GateCard key={g.id} g={g} />)}
          </Box>
        )}

        {/* TAB 3 — usage: spend, margin, and gate progress */}
        {tab === 2 && (
          <>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ mb: 1.5 }}>
              <Meter label="Project spend" value={formatUSD(economics.spentUSD)} sub={`of ${formatUSD(economics.budgetUSD)} reserved`} />
              <Meter label="Provider cost" value={formatUSD(economics.providerCostUSD)} sub="metered to the ledger" />
              <Meter label="Margin (project)" value={`${economics.marginPct}%`} sub={`revenue ${formatUSD(economics.revenueUSD)}`} />
              <Meter label="Gates closed" value={`${gatesClosed}/${p.gates.length}`} sub="end-to-end" />
            </Stack>
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 1.5 }}>
              <Card sx={{ p: 2.5 }}>
                <Stack direction="row" justifyContent="space-between" alignItems="baseline" sx={{ mb: 1 }}>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled' })}>Budget spent</Typography>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.62rem', color: 'text.disabled' })}>account wallet {formatUSD(wallet.availableUSD)}</Typography>
                </Stack>
                <Chart option={walletOption} height={240} />
              </Card>
              <Card sx={{ p: 2.5 }}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Spend by ledger type</Typography>
                <Chart option={ledgerOption} height={240} />
              </Card>
            </Box>
          </>
        )}

        {/* TAB 4 — activity */}
        {tab === 3 && <ActivityFeed projectId={p.id} seed={p.activity} />}
      </Box>
    </Box>
  );
}
