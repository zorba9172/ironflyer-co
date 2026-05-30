import { useMemo, useState } from 'react';
import { Box, Chip, FormControlLabel, Stack, Switch, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { useOperateMutation } from '../hooks/useOperateMutation';
import { StudioChart, horizontalBarOption, type EChartsOption } from '../components/charts';
import { StudioDataTable, type DataTableColumn, type StudioTableTab } from '../components/tables';
import { GlassPanel, GaugeRing, SectionHeader, StatCard } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

// Test coverage for the operator's generated project (NOT Ironflyer's own
// repo that stays test-free by constitution). The CoverageGate runs the
// project's suite with coverage when the toggle is on and stores a report;
// this surface mirrors it and names what is not closed (uncovered files).
interface FileCoverage { path: string; linePct: number; uncovered: number }
interface CoverageReport {
  projectID: string;
  enabled: boolean;
  overallPct: number;
  minPct: number;
  tool: string;
  generatedAt: string | null;
  files: FileCoverage[];
}

const DEFAULT_MIN = 80;

const SAMPLE: CoverageReport = {
  projectID: 'sample', enabled: true, overallPct: 78, minPct: DEFAULT_MIN, tool: 'vitest --coverage',
  generatedAt: null,
  files: [
    { path: 'src/server/webhooks/stripe.ts', linePct: 54, uncovered: 23 },
    { path: 'src/components/Checkout.tsx', linePct: 61, uncovered: 18 },
    { path: 'src/components/Cart.tsx', linePct: 73, uncovered: 12 },
    { path: 'src/server/auth.ts', linePct: 88, uncovered: 7 },
    { path: 'src/server/checkout.ts', linePct: 92, uncovered: 4 },
    { path: 'src/lib/format.ts', linePct: 100, uncovered: 0 },
  ],
};

// Coverage is healthy >= 80, watch 60-79, at-risk below 60.
function covColor(t: Theme, pct: number) {
  if (pct >= 80) return t.palette.success.main;
  if (pct >= 60) return t.palette.warning.main;
  return t.palette.error.main;
}

export function CoveragePane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const { busy, run } = useOperateMutation();
  const [tableView, setTableView] = useState('not_closed');
  const [tableSearch, setTableSearch] = useState('');

  const { data: report, isLive } = useGraphQLQuery<CoverageReport, { coverageReport: CoverageReport }>({
    key: ['coverage-report', liveProjectId ?? 'none'],
    operationName: 'CoverageReport', query: operations.COVERAGE_REPORT,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    map: (r) => r.coverageReport,
  });

  const minPct = report.minPct > 0 ? report.minPct : DEFAULT_MIN;
  const files = report.files ?? [];
  const notClosed = useMemo(() => files.filter((f) => f.linePct < 100).sort((a, b) => a.linePct - b.linePct), [files]);
  const belowFloor = notClosed.filter((f) => f.linePct < minPct);
  const tableTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'all', label: 'All', count: files.length },
    { value: 'not_closed', label: 'Not closed', count: notClosed.length, tone: notClosed.length ? 'warning' : 'success' },
    { value: 'below_floor', label: 'Below floor', count: belowFloor.length, tone: belowFloor.length ? 'error' : 'success' },
    { value: 'covered', label: 'Fully covered', count: files.filter((f) => f.linePct >= 100).length, tone: 'success' },
  ], [belowFloor.length, files, notClosed.length]);
  const tableRows = useMemo(() => {
    const q = tableSearch.trim().toLowerCase();
    return files.filter((file) => {
      if (tableView === 'not_closed' && file.linePct >= 100) return false;
      if (tableView === 'below_floor' && file.linePct >= minPct) return false;
      if (tableView === 'covered' && file.linePct < 100) return false;
      return !q || file.path.toLowerCase().includes(q) || String(file.linePct).includes(q) || String(file.uncovered).includes(q);
    });
  }, [files, minPct, tableSearch, tableView]);

  const onToggle = (enabled: boolean) => {
    if (!liveProjectId) return;
    void run(
      enabled ? 'Coverage enabled' : 'Coverage disabled',
      (req) => req('SetCoverageEnabled', operations.SET_COVERAGE_ENABLED, { projectID: liveProjectId, enabled, minPct }),
      [['coverage-report', liveProjectId]],
    );
  };

  const metrics = [
    { label: 'Overall', value: `${report.overallPct}%`, hint: 'line coverage', accent: covColor(t, report.overallPct) },
    { label: 'Files measured', value: String(files.length), hint: 'tracked files', accent: t.palette.primary.main },
    { label: 'Not closed', value: String(notClosed.length), hint: notClosed.length ? 'files below 100%' : 'all files covered', accent: notClosed.length ? t.palette.warning.main : t.palette.success.main },
    { label: `Below ${minPct}%`, value: String(belowFloor.length), hint: `floor is ${minPct}%`, accent: belowFloor.length ? t.palette.error.main : t.palette.success.main },
  ];

  const columns = useMemo<DataTableColumn<FileCoverage>[]>(() => [
    { field: 'path', headerName: 'File', flex: 1, minWidth: 240, renderCell: (p) => <Box component="span" sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s78 })}>{String(p.value)}</Box> },
    {
      field: 'linePct', headerName: 'Coverage', type: 'number', minWidth: 120, flex: 0,
      renderCell: (p) => <Chip size="small" label={`${p.value as number}%`} sx={{ height: 20, fontSize: text.s66, fontWeight: 600, bgcolor: `${covColor(t, p.value as number)}22`, color: covColor(t, p.value as number) }} />,
    },
    { field: 'uncovered', headerName: 'Uncovered lines', type: 'number', minWidth: 150, flex: 0 },
  ], [t]);

  // Horizontal bar chart of the not-closed files (worst first) - data-bound
  // to real coverage values so it names exactly what is not closed end-to-end.
  const barChart = useMemo<EChartsOption>(() => horizontalBarOption(t, {
    labels: notClosed.slice(0, 10).map((f) => f.path.split('/').slice(-1)[0] ?? f.path),
    values: notClosed.slice(0, 10).map((f) => f.linePct),
    colors: notClosed.slice(0, 10).map((f) => covColor(t, f.linePct)),
  }), [notClosed, t]);

  const measured = report.generatedAt ? new Date(report.generatedAt) : null;
  const subtitle = !report.enabled
    ? "Toggle on to measure your app's tests and see what's not closed"
    : measured && !Number.isNaN(measured.getTime())
      ? `${report.tool || 'coverage'} measured ${measured.toLocaleString()}`
      : 'Enabled - runs on the next finisher pass';

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1100, mx: 'auto' }}>
        <SectionHeader
          eyebrow="Coverage gate"
          title="Coverage"
          subtitle={subtitle}
          actions={
            <Stack direction="row" spacing={1.5} alignItems="center">
              <Chip
                size="small"
                label={isLive ? 'live' : 'sample'}
                sx={(th) => ({ height: 20, fontSize: text.s64, fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
              />
              <FormControlLabel
                control={<Switch checked={report.enabled} disabled={busy || !liveProjectId} onChange={(e) => onToggle(e.target.checked)} size="small" />}
                label={<Typography sx={{ fontSize: text.s84 }}>{report.enabled ? 'On' : 'Off'}</Typography>}
                sx={{ mr: 0 }}
              />
            </Stack>
          }
        />

        {!report.enabled ? (
          <GlassPanel pad={4} sx={{ textAlign: 'center' }}>
            <Typography variant="h5" sx={{ fontWeight: 700, mb: 1 }}>Coverage is off</Typography>
            <Typography sx={{ color: 'text.secondary', fontSize: text.s90, maxWidth: 460, mx: 'auto' }}>
              Turn it on and the finisher runs your app&apos;s test suite with coverage each build, then shows your overall percentage and exactly which files are not closed.
            </Typography>
            {!liveProjectId && <Typography sx={{ fontSize: text.s76, color: 'text.disabled', mt: 2 }}>Open a live project to enable coverage.</Typography>}
          </GlassPanel>
        ) : (
          <>
            {/* Visual-first: GaugeRing (readiness dial) + stat cards */}
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '240px 1fr' }, gap: 1.5, mb: 2, alignItems: 'stretch' }}>
              <GlassPanel pad={2} accent={covColor(t, report.overallPct)} sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Overall</Typography>
                <GaugeRing value={report.overallPct} color={covColor(t, report.overallPct)} height={180} />
              </GlassPanel>
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(4, 1fr)' }, gap: 1.5 }}>
                {metrics.map((m) => (
                  <StatCard key={m.label} label={m.label} value={m.value} hint={m.hint} accent={m.accent} />
                ))}
              </Box>
            </Box>

            {/* What's not closed - the headline the operator asked for */}
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>
              What&apos;s not closed {notClosed.length > 0 ? `(${notClosed.length})` : ''}
            </Typography>

            {notClosed.length === 0 ? (
              <GlassPanel pad={3} sx={{ textAlign: 'center', mb: 3 }} accent={t.palette.success.main}>
                <Typography sx={{ color: 'success.main', fontSize: text.s90 }}>Every measured file is fully covered.</Typography>
              </GlassPanel>
            ) : (
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 1.5, mb: 3 }}>
                {/* Horizontal bars: worst-covered files at a glance */}
                <GlassPanel pad={2} accent={t.palette.warning.main}>
                  <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Worst covered (line %)</Typography>
                  <StudioChart option={barChart} height={Math.min(280, notClosed.slice(0, 10).length * 28 + 40)} />
                </GlassPanel>

                {/* File list */}
                <GlassPanel pad={2}>
                  <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Files</Typography>
                  <Stack spacing={0.65}>
                    {notClosed.slice(0, 8).map((f) => (
                      <Stack key={f.path} direction="row" alignItems="center" spacing={1}>
                        <Box sx={{ width: 7, height: 7, borderRadius: 99, bgcolor: covColor(t, f.linePct), flexShrink: 0 }} />
                        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s76, flex: 1, minWidth: 0 })} noWrap>{f.path.split('/').slice(-2).join('/')}</Typography>
                        <Typography sx={{ fontSize: text.s70, color: 'text.disabled' }}>{f.uncovered} lines</Typography>
                        <Chip size="small" label={`${f.linePct}%`} sx={{ height: 18, fontSize: text.s62, fontWeight: 600, bgcolor: `${covColor(t, f.linePct)}22`, color: covColor(t, f.linePct) }} />
                      </Stack>
                    ))}
                    {notClosed.length > 8 && <Typography sx={{ fontSize: text.s72, color: 'text.disabled' }}>+{notClosed.length - 8} more below</Typography>}
                  </Stack>
                </GlassPanel>
              </Box>
            )}

            <StudioDataTable
              title="Coverage files"
              subtitle={`${tableRows.length.toLocaleString()} files in this view · ${minPct}% minimum floor`}
              tabs={tableTabs}
              activeTab={tableView}
              onTabChange={setTableView}
              searchValue={tableSearch}
              onSearchChange={setTableSearch}
              searchPlaceholder="Search files"
              footer="Coverage is measured in the sandbox each finisher pass for the generated app, not Ironflyer's own repo."
              rows={tableRows} columns={columns}
              getRowId={(row) => row.path}
              density="compact" emptyLabel="No coverage report yet - run the finisher." height={360} minHeight={240}
            />
          </>
        )}
      </Box>
    </Box>
  );
}
