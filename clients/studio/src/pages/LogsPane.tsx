import { useCallback, useMemo, useRef, useState } from 'react';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useRunProjectFeed, useGraphQLQuery, operations } from '@ironflyer/data';
import { formatRelativeTime } from '@ironflyer/core';
import type { ActivityEvent, StudioProject } from '../studioData';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useStudio } from '../store';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { TechIcon } from '../lib/techIcons';
import { PaneHeader } from '../components/operate/PaneHeader';
import { StudioChart, horizontalBarOption, type EChartsOption } from '../components/charts';
import { StudioTableShell, type StudioTableTab } from '../components/tables';
import { text } from '@ironflyer/design-tokens/brand';
import { GlassPanel, StatCard } from '../components/studio';
import { toast } from '@ironflyer/ui-web/fx';

interface LedgerEntry { id: string; executionID?: string | null; entryType: string; direction: string; amountUSD: number; createdAt: string }
const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

function ledgerKind(entryType: string): ActivityEvent['kind'] {
  const e = entryType.toLowerCase();
  if (e.includes('profit') || e.includes('budget') || e.includes('guard') || e.includes('reserv')) return 'profitguard';
  if (e.includes('deploy')) return 'deploy';
  return 'ledger';
}

function kindColor(t: Theme, kind: ActivityEvent['kind']): string {
  switch (kind) {
    case 'gate': return t.palette.primary.main;
    case 'patch': return t.palette.primary.main;
    case 'profitguard': return t.palette.warning.main;
    case 'deploy': return t.palette.success.main;
    default: return t.palette.text.disabled;
  }
}

// Row height for the virtualizer — compact rows, themed via spacing scale.
const ROW_HEIGHT = 44;

// Compact, themed icon for the kind badge.
function KindIcon({ kind }: { kind: ActivityEvent['kind'] }) {
  return (
    <Box component="span" sx={{ color: 'inherit', display: 'inline-flex' }}>
      <TechIcon name={kind} size={13} title={kind} />
    </Box>
  );
}

// Single virtualized log row.
function LogRow({
  event,
  onFix,
  style,
}: {
  event: ActivityEvent;
  onFix: (ev: ActivityEvent) => void;
  style: React.CSSProperties;
}) {
  const t = useTheme();
  const tone = kindColor(t, event.kind);
  return (
    <Box
      style={style}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.5,
        px: 2,
        borderBottom: '1px solid',
        borderColor: 'borderSubtle',
        transition: (th) => `background-color ${th.studio.motion.fast}`,
        '&:hover': { bgcolor: 'surfaceHover' },
      }}
    >
      {/* severity bar — tinted left edge */}
      <Box sx={{ width: 3, height: 28, borderRadius: 99, bgcolor: tone, flexShrink: 0, opacity: 0.85 }} />

      {/* kind badge */}
      <Stack direction="row" alignItems="center" spacing={0.6} sx={{ width: 120, flexShrink: 0 }}>
        <Box sx={{ color: tone }}><KindIcon kind={event.kind} /></Box>
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, textTransform: 'uppercase', letterSpacing: '0.05em', color: tone, fontWeight: 700 })}>
          {event.kind}
        </Typography>
      </Stack>

      {/* message */}
      <Typography sx={{ flex: 1, fontSize: text.s86, color: 'text.primary', minWidth: 0 }} noWrap>
        {event.text}
      </Typography>

      {/* timestamp */}
      <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, color: 'text.disabled', flexShrink: 0, width: 96, textAlign: 'right' })}>
        {formatRelativeTime(event.ts)}
      </Typography>

      {/* fix action */}
      <Button
        size="small"
        variant="text"
        color="inherit"
        sx={{ flexShrink: 0, minWidth: 48, fontSize: text.s74, color: 'text.disabled', '&:hover': { color: 'text.primary' } }}
        onClick={() => onFix(event)}
      >
        Fix
      </Button>
    </Box>
  );
}

// One place for all orchestrator-emitted activity: gate transitions, patches,
// ProfitGuard verdicts, ledger debits, deploys.
// The list is virtualized with @tanstack/react-virtual so it handles thousands
// of events without DOM overhead. The headline visual (event volume by source)
// mirrors exactly what the orchestrator emitted — viz-first before the log detail.
export function LogsPane({ fallback }: { fallback: StudioProject }) {
  const t = useTheme();
  const [kindFilter, setKindFilter] = useState<ActivityEvent['kind'] | 'all'>('all');
  const [search, setSearch] = useState('');
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const liveProjectId = storeProjectId ?? firstProjectId;
  const { dispatch } = useDispatchAgent();
  const { events: liveEvents, isLive } = useRunProjectFeed(liveProjectId);
  const { executions } = useProjectExecutions(liveProjectId);

  const { data: ledger, isLive: ledgerLive } = useGraphQLQuery<LedgerEntry[], { ledger: LedgerEntry[] }>({
    key: ['ledger', 'logs'], operationName: 'Ledger', query: operations.LEDGER,
    variables: { filter: { limit: 200 } }, fallbackData: [], map: (r) => r.ledger ?? [],
  });

  const ledgerEvents = useMemo<ActivityEvent[]>(() => {
    const execIds = new Set(executions.map((e) => e.id));
    const scoped = execIds.size > 0 ? ledger.filter((l) => l.executionID && execIds.has(l.executionID)) : ledger;
    const src = scoped.length > 0 ? scoped : ledger;
    return src.map((l) => ({
      id: l.id,
      ts: Date.parse(l.createdAt) || Date.now(),
      kind: ledgerKind(l.entryType),
      text: `${l.direction === 'debit' ? '−' : '+'}$${l.amountUSD.toFixed(4)} · ${titleCase(l.entryType)}`,
    }));
  }, [ledger, executions]);

  const hasReal = liveEvents.length > 0 || ledgerEvents.length > 0;
  const allRows = useMemo(
    () => [...liveEvents, ...ledgerEvents, ...(hasReal ? [] : fallback.activity)].sort((a, b) => b.ts - a.ts) as ActivityEvent[],
    [liveEvents, ledgerEvents, hasReal, fallback.activity],
  );
  const live = isLive || ledgerLive;

  // Filtered rows — kind + free-text search.
  const rows = useMemo(() => {
    let r = allRows;
    if (kindFilter !== 'all') r = r.filter((ev) => ev.kind === kindFilter);
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      r = r.filter((ev) => ev.text.toLowerCase().includes(q) || ev.kind.includes(q));
    }
    return r;
  }, [allRows, kindFilter, search]);
  const logTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'all', label: 'All', count: allRows.length },
    { value: 'gate', label: 'Gate', count: allRows.filter((ev) => ev.kind === 'gate').length, tone: 'info' },
    { value: 'patch', label: 'Patch', count: allRows.filter((ev) => ev.kind === 'patch').length },
    { value: 'profitguard', label: 'ProfitGuard', count: allRows.filter((ev) => ev.kind === 'profitguard').length, tone: 'warning' },
    { value: 'deploy', label: 'Deploy', count: allRows.filter((ev) => ev.kind === 'deploy').length, tone: 'success' },
    { value: 'ledger', label: 'Ledger', count: allRows.filter((ev) => ev.kind === 'ledger').length },
  ], [allRows]);

  // Source volume bar — mirrors what the orchestrator emitted on this project.
  const sourceBar = useMemo<EChartsOption>(() => {
    const order: ActivityEvent['kind'][] = ['gate', 'patch', 'profitguard', 'deploy', 'ledger'];
    const counts = order.map((k) => allRows.filter((r) => r.kind === k).length);
    return horizontalBarOption(t, {
      labels: ['Gate', 'Patch', 'ProfitGuard', 'Deploy', 'Ledger'],
      values: counts,
      colors: order.map((kind) => kindColor(t, kind)),
    });
  }, [allRows, t]);

  // Stats derived from allRows.
  const gateCount = allRows.filter((r) => r.kind === 'gate').length;
  const patchCount = allRows.filter((r) => r.kind === 'patch').length;
  const deployCount = allRows.filter((r) => r.kind === 'deploy').length;

  // Stable fix callback to avoid re-renders.
  const handleFix = useCallback((ev: ActivityEvent) => {
    void dispatch(`this ${ev.kind} event`);
    toast(`Dispatching agent for ${ev.kind} event.`, 'info');
  }, [dispatch]);

  const fixAll = () => {
    const n = rows.length;
    void dispatch(`${n} log${n > 1 ? 's' : ''}`);
    toast(`Dispatching agent for ${n} log event${n > 1 ? 's' : ''}.`, 'success');
  };

  // Virtualizer — @tanstack/react-virtual, fixed row height.
  const scrollRef = useRef<HTMLDivElement>(null);
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  });

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: { xs: 2, md: 3 } }}>
      <Box sx={{ maxWidth: 1160, mx: 'auto' }}>

        {/* ── Header ─────────────────────────────────────────────────────── */}
        <PaneHeader
          title="Logs"
          isLive={live}
          subtitle={`${allRows.length.toLocaleString()} events · gate transitions, patches, ProfitGuard verdicts, ledger debits, deploys`}
        />

        {/* ── Stats strip ────────────────────────────────────────────────── */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 3 }}>
          <StatCard label="Total events" value={allRows.length} hint={live ? 'live feed' : 'seed data'} accent={t.palette.primary.main} />
          <StatCard label="Gate events" value={gateCount} hint="transitions" accent={t.palette.primary.main} />
          <StatCard label="Patches" value={patchCount} hint="applied" accent={t.palette.primary.main} />
          <StatCard label="Deploys" value={deployCount} hint="completed" accent={t.palette.success.main} />
        </Box>

        {/* ── Source volume bar ───────────────────────────────────────────── */}
        {allRows.length > 0 && (
          <GlassPanel pad={2.5} sx={{ mb: 3 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s64, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>
              Events by Source
            </Typography>
            <StudioChart option={sourceBar} height={160} />
          </GlassPanel>
        )}

        {/* ── Virtualized log list ────────────────────────────────────────── */}
        <StudioTableShell
          title="Event stream"
          subtitle={`${rows.length.toLocaleString()} of ${allRows.length.toLocaleString()} events`}
          tabs={logTabs}
          activeTab={kindFilter}
          onTabChange={(value) => setKindFilter(value as ActivityEvent['kind'] | 'all')}
          searchValue={search}
          onSearchChange={setSearch}
          searchPlaceholder="Search messages"
          actions={
            <Button variant="contained" disabled={rows.length === 0} onClick={fixAll}>
              Fix all
            </Button>
          }
          footer={rows.length > 0 ? `Showing ${rows.length.toLocaleString()} event${rows.length !== 1 ? 's' : ''} - virtualized` : 'No events in the current view.'}
        >
          {/* Column headers */}
          <Box sx={{
            display: 'flex', alignItems: 'center', gap: 1.5, px: 2, py: 1,
            borderBottom: '1px solid', borderColor: 'divider',
            bgcolor: 'action.hover',
          }}>
            <Box sx={{ width: 3, flexShrink: 0 }} />
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s62, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', width: 120, flexShrink: 0 })}>
              Source
            </Typography>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s62, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', flex: 1 })}>
              Message
            </Typography>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s62, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', width: 96, textAlign: 'right', flexShrink: 0 })}>
              When
            </Typography>
            <Box sx={{ width: 48, flexShrink: 0 }} />
          </Box>

          {rows.length === 0 ? (
            <Box sx={{ py: 8, textAlign: 'center' }}>
              <Typography sx={{ color: 'text.disabled', fontSize: text.s90 }}>
                {allRows.length === 0 ? 'No logs yet — run the finisher to generate activity.' : 'No events match the current filter.'}
              </Typography>
            </Box>
          ) : (
            // Scroll container — virtualizer owns the inner div height.
            <Box
              ref={scrollRef}
              sx={{ height: Math.min(rows.length * ROW_HEIGHT, 540), overflowY: 'auto' }}
            >
              <Box sx={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
                {virtualizer.getVirtualItems().map((vRow) => {
                  const ev = rows[vRow.index]!;
                  return (
                    <LogRow
                      key={ev.id}
                      event={ev}
                      onFix={handleFix}
                      style={{
                        position: 'absolute',
                        top: 0,
                        left: 0,
                        width: '100%',
                        height: ROW_HEIGHT,
                        transform: `translateY(${vRow.start}px)`,
                      }}
                    />
                  );
                })}
              </Box>
            </Box>
          )}
        </StudioTableShell>
      </Box>
    </Box>
  );
}
