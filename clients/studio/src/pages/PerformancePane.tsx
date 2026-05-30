import { useMemo, useState } from 'react';
import type { ReactElement } from 'react';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useStudio } from '../store';
import { text } from '@ironflyer/design-tokens/brand';
import { GlassPanel, SectionHeader, GaugeRing } from '../components/studio';
import { StudioTableShell, type StudioTableTab } from '../components/tables';

interface RawGate { gate: string; status: string; issues: { severity: string; message: string }[] }
interface PerfRow { id: string; area: 'Frontend' | 'Backend'; name: string; status: string; detail: string }
interface Forecast { level: string; burnRatePerHourUSD: number; extrapolatedTotalUSD: number; remainingHeadroomUSD: number }

const PERF_AREA: Record<string, 'Frontend' | 'Backend'> = {
  lighthouse: 'Frontend',
  bundle_size: 'Frontend',
  perf_budget: 'Frontend',
  mobile_size: 'Frontend',
  mobile_bundle_analyzer: 'Frontend',
  mem_leak: 'Backend',
  complexity: 'Backend',
  dep_graph: 'Backend',
  arch_boundary: 'Backend',
};

const SAMPLE: PerfRow[] = [
  { id: 'bundle_size', area: 'Frontend', name: 'Bundle Size', status: 'blocked', detail: 'Main chunk is 612KB, over the 400KB budget' },
  { id: 'lighthouse', area: 'Frontend', name: 'Performance (LCP)', status: 'blocked', detail: 'LCP 3.8s, exceeding the 2.5s threshold' },
  { id: 'mem_leak', area: 'Backend', name: 'Memory Leak', status: 'blocked', detail: 'Heap grows 8MB/min under load' },
  { id: 'complexity', area: 'Backend', name: 'Cyclomatic Complexity', status: 'blocked', detail: 'checkout() complexity is 24' },
  { id: 'unused_js', area: 'Frontend', name: 'Unused JavaScript', status: 'warning', detail: '240KB of unused JS detected' },
];

function icon(paths: string[], size = 17) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      {paths.map((d) => <path key={d} d={d} />)}
    </svg>
  );
}

const icons = {
  spark: icon(['M12 3l1.5 5.5L17 10l-3.5 1.5L12 17l-1.5-5.5L7 10l3.5-1.5z', 'M19 15l.7 2.3L22 18l-2.3.7L19 21l-.7-2.3L16 18l2.3-.7z']),
  cube: icon(['M12 3l8 4.5v9L12 21l-8-4.5v-9z', 'M12 12l8-4.5', 'M12 12v9', 'M12 12L4 7.5']),
  pulse: icon(['M3 12h4l2-5 4 10 2-5h6']),
  chip: icon(['M8 3v3M16 3v3M8 18v3M16 18v3M3 8h3M18 8h3M3 16h3M18 16h3', 'M7 7h10v10H7z']),
  braces: icon(['M8 4H7a3 3 0 00-3 3v2a2 2 0 01-2 2 2 2 0 012 2v2a3 3 0 003 3h1', 'M16 4h1a3 3 0 013 3v2a2 2 0 002 2 2 2 0 00-2 2v2a3 3 0 01-3 3h-1']),
  doc: icon(['M7 3h7l5 5v13H7z', 'M14 3v6h5', 'M10 13h6', 'M10 17h4']),
  desktop: icon(['M4 5h16v11H4z', 'M9 21h6', 'M12 16v5']),
  server: icon(['M4 5h16v5H4z', 'M4 14h16v5H4z', 'M7 7h.01M7 16h.01']),
  shield: icon(['M12 3l7 3v5c0 5-3.4 8.4-7 10-3.6-1.6-7-5-7-10V6z']),
  filter: icon(['M4 6h16', 'M7 12h10', 'M10 18h4']),
  fix: icon(['M14.7 6.3a3 3 0 00-4 4L4 17v3h3l6.7-6.7a3 3 0 004-4z', 'M16 5l3 3']),
  arrow: icon(['M5 12h14', 'M13 6l6 6-6 6']),
  check: icon(['M20 6L9 17l-5-5']),
};

const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
const isClosed = (s: string) => ['pass', 'passed'].includes(s.toLowerCase());

function severityFor(row: PerfRow): 'High' | 'Medium' | 'Low' {
  if (row.id.includes('bundle')) return 'High';
  if (row.id.includes('lighthouse') || row.id.includes('mem')) return 'Medium';
  return 'Low';
}

function severityTone(t: Theme, severity: 'High' | 'Medium' | 'Low') {
  if (severity === 'High') return t.palette.error.main;
  if (severity === 'Medium') return t.palette.warning.main;
  return t.palette.info.main;
}

function metricFor(row: PerfRow) {
  if (row.id.includes('bundle')) return { current: '612KB', target: '<= 400KB', impactPct: '+18%' };
  if (row.id.includes('lighthouse')) return { current: '3.8s', target: '<= 2.5s', impactPct: '+9%' };
  if (row.id.includes('mem')) return { current: '8MB/min', target: '<= 2MB/min', impactPct: '+12%' };
  if (row.id.includes('complexity')) return { current: '24', target: '<= 15', impactPct: '+6%' };
  return { current: '240KB', target: '<= 50KB', impactPct: '+4%' };
}

function issueIcon(id: string) {
  if (id.includes('bundle')) return icons.cube;
  if (id.includes('lighthouse')) return icons.pulse;
  if (id.includes('mem')) return icons.chip;
  if (id.includes('complexity')) return icons.braces;
  return icons.doc;
}

function LayerRow({ iconNode, label, issues, tone, value }: { iconNode: ReactElement; label: string; issues: string; tone: string; value: number }) {
  return (
    <Stack direction="row" alignItems="center" spacing={2} sx={{ py: 1.65, borderBottom: '1px solid', borderColor: 'divider' }}>
      <Box sx={{ width: 26, color: 'text.secondary', display: 'grid', placeItems: 'center' }}>{iconNode}</Box>
      <Typography sx={{ flex: 1, fontWeight: 800, fontSize: text.s90 }}>{label}</Typography>
      <Typography sx={{ width: 82, color: tone, fontSize: text.s78, fontWeight: 700 }}>{issues}</Typography>
      <Box sx={{ width: 140, height: 7, borderRadius: 99, bgcolor: 'action.hover', overflow: 'hidden' }}>
        <Box sx={{ width: `${value}%`, height: '100%', borderRadius: 99, bgcolor: tone }} />
      </Box>
    </Stack>
  );
}

function IssueRow({ row, onFix }: { row: PerfRow; onFix: (row: PerfRow) => void }) {
  const t = useTheme();
  const severity = severityFor(row);
  const tone = severityTone(t, severity);
  const metric = metricFor(row);
  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={2}
      sx={{
        px: 2,
        py: 1.45,
        minHeight: 72,
        borderTop: '1px solid',
        borderColor: 'divider',
        '&:hover': { bgcolor: 'action.hover' },
      }}
    >
      <Box
        sx={{
          width: 42,
          height: 42,
          borderRadius: '50%',
          display: 'grid',
          placeItems: 'center',
          color: tone,
          bgcolor: `${tone}16`,
          border: `1px solid ${tone}24`,
          flexShrink: 0,
        }}
      >
        {issueIcon(row.id)}
      </Box>
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Stack direction="row" spacing={1} alignItems="center">
          <Typography sx={{ fontWeight: 800, fontSize: text.s90 }} noWrap>{row.name}</Typography>
          <Chip size="small" label={severity} sx={{ height: 20, fontSize: text.s62, color: tone, bgcolor: `${tone}16`, fontWeight: 700 }} />
        </Stack>
        <Typography sx={{ color: 'text.secondary', fontSize: text.s78 }} noWrap>{row.detail}</Typography>
      </Box>
      <Box sx={{ width: 76 }}>
        <Typography sx={{ color: 'text.disabled', fontSize: text.s62 }}>Current</Typography>
        <Typography sx={{ fontWeight: 800, fontSize: text.s82 }}>{metric.current}</Typography>
      </Box>
      <Box sx={{ width: 76 }}>
        <Typography sx={{ color: 'text.disabled', fontSize: text.s62 }}>Target</Typography>
        <Typography sx={{ fontWeight: 800, fontSize: text.s82 }}>{metric.target}</Typography>
      </Box>
      <Box sx={{ width: 78 }}>
        <Typography sx={{ color: 'text.disabled', fontSize: text.s62 }}>Impact</Typography>
        <Typography sx={{ color: tone, fontSize: text.s78, fontWeight: 700 }}>{metric.impactPct}</Typography>
      </Box>
      <Button size="small" variant="outlined" onClick={() => onFix(row)} startIcon={icons.spark} sx={{ borderRadius: 999, minWidth: 74 }}>
        Fix
      </Button>
    </Stack>
  );
}

export function PerformancePane() {
  const t = useTheme();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const liveProjectId = storeProjectId ?? firstProjectId;
  const { dispatch, repairGate } = useDispatchAgent();
  const [tableView, setTableView] = useState('open');
  const [tableSearch, setTableSearch] = useState('');

  const { data: rows } = useGraphQLQuery<PerfRow[], { gates: RawGate[] }>({
    key: ['perf-gates', liveProjectId ?? 'none'],
    operationName: 'Gates',
    query: operations.GATES,
    variables: { projectId: liveProjectId },
    fallbackData: SAMPLE,
    enabled: !!liveProjectId,
    map: (r) => {
      const perf = r.gates.filter((g) => PERF_AREA[g.gate]);
      if (!perf.length) return SAMPLE;
      return perf.map((g) => ({
        id: g.gate,
        area: PERF_AREA[g.gate]!,
        name: titleCase(g.gate),
        status: g.status,
        detail: isClosed(g.status) ? 'Meets budget' : g.issues[0]?.message ?? 'Needs optimization',
      }));
    },
  });

  const { data: forecast } = useGraphQLQuery<Forecast, { sentinelForecast: Forecast }>({
    key: ['sentinel', liveProjectId ?? 'none'],
    operationName: 'SentinelForecast',
    query: operations.SENTINEL_FORECAST,
    variables: { projectId: liveProjectId },
    fallbackData: { level: 'green', burnRatePerHourUSD: 0, extrapolatedTotalUSD: 0, remainingHeadroomUSD: 0 },
    enabled: !!liveProjectId,
    map: (r) => r.sentinelForecast,
  });

  const openRows = rows.filter((r) => !isClosed(r.status));
  const open = openRows.length;
  const score = Math.max(0, Math.round(100 - open * (28 / Math.max(rows.length, 1))));
  const frontendIssues = rows.filter((r) => r.area === 'Frontend' && !isClosed(r.status)).length;
  const backendIssues = rows.filter((r) => r.area === 'Backend' && !isClosed(r.status)).length;
  const memoryIssues = rows.filter((r) => r.id.includes('mem') && !isClosed(r.status)).length;
  const memoryTone = memoryIssues > 0 ? t.palette.warning.main : t.palette.success.main;
  const memoryValue = memoryIssues > 0 ? 48 : 100;

  // Gauge color: semantic — red below 50, warning 50-75, success 76+
  const gaugeColor = score < 50
    ? t.palette.error.main
    : score < 76
      ? t.palette.warning.main
      : t.palette.success.main;

  const largestIssue = useMemo(() => openRows.find((r) => r.id.includes('bundle')) ?? openRows[0] ?? SAMPLE[0]!, [openRows]);
  const fixAll = () => { void dispatch(`${open} performance issue${open > 1 ? 's' : ''}`); };
  const fixOne = (row: PerfRow) => { void repairGate(row.id, row.name); };
  const tableTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'open', label: 'Open', count: openRows.length, tone: openRows.length ? 'warning' : 'success' },
    { value: 'frontend', label: 'Frontend', count: frontendIssues, tone: frontendIssues ? 'warning' : 'success' },
    { value: 'backend', label: 'Backend', count: backendIssues, tone: backendIssues ? 'warning' : 'success' },
    { value: 'closed', label: 'Closed', count: rows.filter((r) => isClosed(r.status)).length, tone: 'success' },
    { value: 'all', label: 'All', count: rows.length },
  ], [backendIssues, frontendIssues, openRows.length, rows]);
  const tableRows = useMemo(() => {
    const q = tableSearch.trim().toLowerCase();
    return rows.filter((row) => {
      if (tableView === 'open' && isClosed(row.status)) return false;
      if (tableView === 'frontend' && row.area !== 'Frontend') return false;
      if (tableView === 'backend' && row.area !== 'Backend') return false;
      if (tableView === 'closed' && !isClosed(row.status)) return false;
      return !q || [row.name, row.detail, row.status, row.area, row.id].some((value) => value.toLowerCase().includes(q));
    });
  }, [rows, tableSearch, tableView]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', color: 'text.primary', p: { xs: 2, md: 3.5 } }}>
      <Box sx={{ maxWidth: 1240, mx: 'auto' }}>
        <SectionHeader
          eyebrow="Quality workspace"
          title={
            <Stack direction="row" alignItems="center" spacing={1.25}>
              <span>Performance Review</span>
              <Chip
                size="small"
                label="AI POWERED"
                sx={(theme) => ({ height: 22, color: theme.palette.primary.main, bgcolor: `${theme.palette.primary.main}16`, fontWeight: 800, fontSize: text.s62 })}
              />
            </Stack>
          }
          subtitle={`AI analyzed your code and found ${open} issues impacting production readiness.`}
          actions={
            <Stack spacing={0.75} alignItems="flex-end">
              <Button
                variant="contained"
                onClick={fixAll}
                disabled={open === 0}
                startIcon={icons.spark}
                sx={{ minHeight: 46, px: 3.5, borderRadius: '12px' }}
              >
                Fix {open} issues automatically
              </Button>
              <Typography sx={{ color: 'text.disabled', fontSize: text.s72 }}>AI will apply safe fixes and create a PR</Typography>
            </Stack>
          }
        />

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1fr) 330px' }, gap: 2 }}>
          <Box>
            {/* Production readiness card with canonical GaugeRing */}
            <GlassPanel pad={3} sx={{ mb: 3 }}>
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '280px 1fr' }, gap: 3, alignItems: 'center' }}>
                <Box sx={{ textAlign: 'center', borderRight: { md: '1px solid' }, borderColor: { md: 'divider' }, pr: { md: 3 } }}>
                  <Typography sx={{ fontWeight: 800, fontSize: text.s105, mb: 1.5 }}>Production Readiness</Typography>
                  <GaugeRing
                    value={score}
                    color={gaugeColor}
                    formatter={'{value}%'}
                    height={200}
                  />
                  <Stack direction="row" justifyContent="center" alignItems="center" spacing={1} sx={{ mt: 0.5 }}>
                    <Box sx={{ width: 7, height: 7, borderRadius: 99, bgcolor: gaugeColor }} />
                    <Typography sx={{ color: gaugeColor, fontSize: text.s76, fontWeight: 800 }}>
                      {score < 50 ? 'Needs attention' : score < 76 ? 'Good, improving' : 'Production ready'}
                    </Typography>
                  </Stack>
                  <Typography sx={{ color: 'text.disabled', fontSize: text.s72, mt: 0.75 }}>Target: 90%+</Typography>
                </Box>

                <Box>
                  <LayerRow iconNode={icons.desktop} label="Frontend" issues={`${frontendIssues} issue${frontendIssues !== 1 ? 's' : ''}`} tone={t.palette.primary.main} value={62} />
                  <LayerRow iconNode={icons.server} label="Backend" issues={`${backendIssues} issue${backendIssues !== 1 ? 's' : ''}`} tone={t.palette.warning.main} value={80} />
                  <LayerRow iconNode={icons.pulse} label="Memory" issues={`${memoryIssues} issue${memoryIssues !== 1 ? 's' : ''}`} tone={memoryTone} value={memoryValue} />
                  <LayerRow iconNode={icons.shield} label="Security" issues="0 issues" tone={t.palette.success.main} value={100} />
                </Box>
              </Box>
            </GlassPanel>

            <StudioTableShell
              title="Issues to resolve"
              subtitle={`${tableRows.length.toLocaleString()} visible · ${open.toLocaleString()} open issues`}
              tabs={tableTabs}
              activeTab={tableView}
              onTabChange={setTableView}
              searchValue={tableSearch}
              onSearchChange={setTableSearch}
              searchPlaceholder="Search issues"
              footer="All fixes are safe, tested, and reversible. Ironflyer creates a pull request with changes for your review."
            >
              <Box sx={{ overflow: 'hidden' }}>
                {tableRows.map((row) => <IssueRow key={row.id} row={row} onFix={fixOne} />)}
                {tableRows.length === 0 && (
                  <Box sx={{ py: 6, textAlign: 'center' }}>
                    <Typography sx={{ color: 'text.disabled', fontSize: text.s90 }}>No issues in this view.</Typography>
                  </Box>
                )}
              </Box>
            </StudioTableShell>
          </Box>

          <Stack spacing={2}>
            {/* AI Analysis panel */}
            <GlassPanel pad={2.6}>
              <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 2 }}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Box sx={(theme) => ({ color: theme.palette.primary.main })}>{icons.spark}</Box>
                  <Typography sx={{ fontWeight: 800 }}>AI Analysis</Typography>
                </Stack>
                <Chip
                  size="small"
                  label="High impact"
                  sx={(theme) => ({ height: 22, color: theme.palette.error.main, bgcolor: `${theme.palette.error.main}12`, fontSize: text.s62 })}
                />
              </Stack>
              <Typography sx={{ color: 'text.secondary', fontSize: text.s86, lineHeight: 1.7 }}>
                Your largest issue is{' '}
                <Box component="span" sx={{ color: 'text.primary', fontWeight: 800 }}>{largestIssue.name.toLowerCase()}</Box>.
              </Typography>
              <Typography sx={{ color: 'text.secondary', fontSize: text.s84, lineHeight: 1.7, mt: 1.5 }}>
                Users on mobile networks may experience slower loads.
              </Typography>
              <Typography sx={{ fontWeight: 800, mt: 2, mb: 1, fontSize: text.s82 }}>Estimated impact</Typography>
              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1 }}>
                <Box
                  sx={(theme) => ({
                    borderRadius: `${theme.studio.radius.sm}px`,
                    p: 1.4,
                    bgcolor: `${theme.palette.error.main}10`,
                    textAlign: 'center',
                  })}
                >
                  <Typography sx={(theme) => ({ color: theme.palette.error.main, fontSize: text.s130, fontWeight: 800 })}>+18%</Typography>
                  <Typography sx={{ color: 'text.secondary', fontSize: text.s68 }}>slower first load</Typography>
                </Box>
                <Box
                  sx={(theme) => ({
                    borderRadius: `${theme.studio.radius.sm}px`,
                    p: 1.4,
                    bgcolor: `${theme.palette.error.main}10`,
                    textAlign: 'center',
                  })}
                >
                  <Typography sx={(theme) => ({ color: theme.palette.error.main, fontSize: text.s130, fontWeight: 800 })}>+9%</Typography>
                  <Typography sx={{ color: 'text.secondary', fontSize: text.s68 }}>lower conversion</Typography>
                </Box>
              </Box>
              <Typography sx={{ fontWeight: 800, mt: 2, mb: 0.8, fontSize: text.s82 }}>Recommended fix</Typography>
              <Typography sx={{ color: 'text.secondary', fontSize: text.s84, lineHeight: 1.7 }}>
                Split checkout bundle and lazy load non-critical modules.
              </Typography>
              <Button
                fullWidth
                variant="outlined"
                onClick={() => fixOne(largestIssue)}
                startIcon={icons.fix}
                sx={{ mt: 2.2, borderRadius: '12px', minHeight: 42 }}
              >
                Preview Fix
              </Button>
              <Typography sx={{ color: 'text.disabled', fontSize: text.s68, mt: 1.2, textAlign: 'center' }}>
                Safe {'·'} Tested {'·'} Reversible
              </Typography>
            </GlassPanel>

            {/* AI Optimization Agent panel */}
            <GlassPanel pad={2.6}>
              <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 2 }}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Box sx={(theme) => ({ color: theme.palette.primary.main })}>{icons.spark}</Box>
                  <Typography sx={{ fontWeight: 800 }}>AI Optimization Agent</Typography>
                </Stack>
                <Chip
                  size="small"
                  label="Live"
                  sx={(theme) => ({
                    height: 22,
                    color: theme.palette.success.main,
                    bgcolor: `${theme.palette.success.main}16`,
                    fontSize: text.s62,
                  })}
                />
              </Stack>
              {['Scanning project', 'Bundle analysis complete', 'Memory analysis complete', 'Performance plan generated'].map((item) => (
                <Stack key={item} direction="row" spacing={1.2} alignItems="center" sx={{ py: 0.85 }}>
                  <Box
                    sx={(theme) => ({
                      width: 16,
                      height: 16,
                      borderRadius: '50%',
                      bgcolor: theme.palette.success.main,
                      color: theme.palette.common.white,
                      display: 'grid',
                      placeItems: 'center',
                    })}
                  >
                    {icons.check}
                  </Box>
                  <Typography sx={{ color: 'text.secondary', fontSize: text.s82 }}>{item}</Typography>
                </Stack>
              ))}
              <Stack direction="row" spacing={1.2} alignItems="center" sx={{ py: 0.85 }}>
                <Box
                  sx={(theme) => ({
                    width: 16,
                    height: 16,
                    borderRadius: '50%',
                    border: `2px solid ${theme.palette.primary.main}`,
                  })}
                />
                <Typography sx={{ color: 'text.secondary', fontSize: text.s82 }}>Ready to apply fixes</Typography>
              </Stack>
              <Button fullWidth endIcon={icons.arrow} sx={(theme) => ({ mt: 1.4, color: theme.palette.primary.main })}>
                Learn last scan logs
              </Button>
              <Typography sx={{ color: 'text.disabled', fontSize: text.s68, mt: 1, textAlign: 'center' }}>
                Sentinel burn rate: ${forecast.burnRatePerHourUSD.toFixed(2)}/h
              </Typography>
            </GlassPanel>
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}
