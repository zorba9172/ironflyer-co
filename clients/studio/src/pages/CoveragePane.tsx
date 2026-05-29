import { useMemo } from 'react';
import { Box, Card, Chip, FormControlLabel, Stack, Switch, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { DataTable, type DataTableColumn } from '@ironflyer/ui-web/data-table';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { useOperateMutation } from '../hooks/useOperateMutation';
import { text } from '@ironflyer/design-tokens/brand';

// Test coverage for the operator's generated project (NOT Ironflyer's own
// repo — that stays test-free by constitution). The CoverageGate runs the
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

// Coverage is healthy ≥80, watch 60–79, at-risk below 60 — resolved from the
// theme so the thresholds read in the locked palette.
function covColor(t: Theme, pct: number) {
  if (pct >= 80) return t.palette.success.main;
  if (pct >= 60) return t.palette.warning.main;
  return t.palette.error.main;
}

export function CoveragePane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const { busy, run } = useOperateMutation();

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

  const onToggle = (enabled: boolean) => {
    if (!liveProjectId) return;
    void run(
      enabled ? 'Coverage enabled' : 'Coverage disabled',
      (req) => req('SetCoverageEnabled', operations.SET_COVERAGE_ENABLED, { projectID: liveProjectId, enabled, minPct }),
      [['coverage-report', liveProjectId]],
    );
  };

  const gauge = useMemo<EChartsOption>(() => ({
    series: [{
      type: 'gauge', startAngle: 210, endAngle: -30, min: 0, max: 100,
      progress: { show: true, width: 14, itemStyle: { color: covColor(t, report.overallPct) } },
      axisLine: { lineStyle: { width: 14, color: [[1, t.palette.action.hover]] } },
      axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false }, pointer: { show: false },
      anchor: { show: false },
      detail: { valueAnimation: true, formatter: '{value}%', color: covColor(t, report.overallPct), fontSize: 30, offsetCenter: [0, 0] },
      data: [{ value: report.overallPct }],
    }],
  }), [report.overallPct, t]);

  const metrics = [
    { label: 'Overall', value: `${report.overallPct}%`, color: covColor(t, report.overallPct) },
    { label: 'Files measured', value: String(files.length), color: t.palette.text.primary },
    { label: 'Not closed', value: String(notClosed.length), color: notClosed.length ? t.palette.warning.main : t.palette.success.main },
    { label: `Below ${minPct}%`, value: String(belowFloor.length), color: belowFloor.length ? t.palette.error.main : t.palette.success.main },
  ];

  const columns = useMemo<DataTableColumn<FileCoverage>[]>(() => [
    { field: 'path', headerName: 'File', flex: 1, minWidth: 240, renderCell: (p) => <Box component="span" sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s78 })}>{String(p.value)}</Box> },
    {
      field: 'linePct', headerName: 'Coverage', type: 'number', minWidth: 120, flex: 0,
      renderCell: (p) => <Chip size="small" label={`${p.value as number}%`} sx={{ height: 20, fontSize: text.s66, fontWeight: 600, bgcolor: `${covColor(t, p.value as number)}22`, color: covColor(t, p.value as number) }} />,
    },
    { field: 'uncovered', headerName: 'Uncovered lines', type: 'number', minWidth: 150, flex: 0 },
  ], [t]);

  const measured = report.generatedAt ? new Date(report.generatedAt) : null;
  const subtitle = !report.enabled
    ? 'Toggle on to measure your app’s tests and see what’s not closed'
    : measured && !Number.isNaN(measured.getTime())
      ? `${report.tool || 'coverage'} · measured ${measured.toLocaleString()}`
      : 'Enabled — runs on the next finisher pass';

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Typography variant="h4" sx={{ fontSize: text.s160 }}>Coverage</Typography>
          <Chip size="small" label={isLive ? 'live' : 'sample'} sx={(th) => ({ height: 20, fontSize: text.s64, fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
          <Typography sx={{ color: 'text.secondary', fontSize: text.s90 }}>{subtitle}</Typography>
          <Box sx={{ flex: 1 }} />
          <FormControlLabel
            control={<Switch checked={report.enabled} disabled={busy || !liveProjectId} onChange={(e) => onToggle(e.target.checked)} />}
            label={<Typography sx={{ fontSize: text.s84 }}>{report.enabled ? 'Coverage on' : 'Coverage off'}</Typography>}
            sx={{ mr: 0 }}
          />
        </Stack>

        {!report.enabled ? (
          <Card sx={{ p: 4, textAlign: 'center' }}>
            <Typography sx={{ fontSize: text.s120, fontWeight: 600, mb: 1 }}>Test coverage is off</Typography>
            <Typography sx={{ color: 'text.secondary', fontSize: text.s90, maxWidth: 460, mx: 'auto' }}>
              Turn it on and the finisher runs your app&apos;s test suite with coverage each build, then shows your overall percentage and exactly which files are not closed.
            </Typography>
            {!liveProjectId && <Typography sx={{ fontSize: text.s76, color: 'text.disabled', mt: 2 }}>Open a live project to enable coverage.</Typography>}
          </Card>
        ) : (
          <>
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '300px 1fr' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
              <Card sx={{ p: 2 }}>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Overall coverage</Typography>
                <Chart option={gauge} height={200} />
              </Card>
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr' }, gap: 1.5 }}>
                {metrics.map((m) => (
                  <Card key={m.label} sx={{ p: 2.5, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                    <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                    <Typography variant="h4" sx={{ fontSize: text.s180, mt: 0.5, color: m.color }}>{m.value}</Typography>
                  </Card>
                ))}
              </Box>
            </Box>

            {/* What's not closed — the headline the operator asked for. */}
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>
              What&apos;s not closed {notClosed.length > 0 ? `(${notClosed.length})` : ''}
            </Typography>
            {notClosed.length === 0 ? (
              <Card sx={{ p: 3, textAlign: 'center', mb: 3 }}>
                <Typography sx={{ color: 'success.main', fontSize: text.s90 }}>Every measured file is fully covered. ✓</Typography>
              </Card>
            ) : (
              <Stack spacing={0.75} sx={{ mb: 3 }}>
                {notClosed.slice(0, 8).map((f) => (
                  <Card key={f.path} sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1.25 }}>
                    <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: covColor(t, f.linePct), flexShrink: 0 }} />
                    <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s80, flex: 1, minWidth: 0 })} noWrap>{f.path}</Typography>
                    <Typography sx={{ fontSize: text.s74, color: 'text.disabled' }}>{f.uncovered} uncovered</Typography>
                    <Chip size="small" label={`${f.linePct}%`} sx={{ height: 20, fontSize: text.s66, fontWeight: 600, bgcolor: `${covColor(t, f.linePct)}22`, color: covColor(t, f.linePct) }} />
                  </Card>
                ))}
                {notClosed.length > 8 && <Typography sx={{ fontSize: text.s76, color: 'text.disabled', pl: 0.5 }}>+{notClosed.length - 8} more below the table</Typography>}
              </Stack>
            )}

            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>By file</Typography>
            <DataTable
              rows={files} columns={columns}
              getRowId={(row) => row.path}
              density="compact" emptyLabel="No coverage report yet — run the finisher." height={360} minHeight={240}
            />
            <Typography sx={{ fontSize: text.s76, color: 'text.disabled', mt: 1.5 }}>
              Coverage of your generated app, measured in the sandbox each finisher pass — your project&apos;s tests, never Ironflyer&apos;s.
            </Typography>
          </>
        )}
      </Box>
    </Box>
  );
}
