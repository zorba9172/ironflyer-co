import { useMemo, useState } from 'react';
import { Box, Button, Chip, Divider, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { severityRank, type SecurityState, type Severity } from '../studioData';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useStudio } from '../store';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { TechIcon } from '../lib/techIcons';
import { Icon, BrandAsset } from '../icons';
import { StudioDataGrid, type DataGridCellParams, type DataGridColumn, type StudioTableTab } from './tables';
import { text } from '@ironflyer/design-tokens/brand';
import { GlassPanel, SectionHeader, GaugeRing } from './studio';
import { StudioChart, donutOption, horizontalBarOption, type EChartsOption } from './charts';

interface Row { id: string; severity: Severity; title: string; category: string; location: string; scanner: string; remediation: string }
interface RawFile { path: string; content?: string | null; size?: number | null }
interface GateVerdict { gate: string; status: string; issues: { severity: string; message: string; path?: string | null; line?: number | null }[] }
interface Report {
  status: string; overallScore: number; secretsFound: number; outdatedDeps: number; blockedDeploy: boolean;
  owaspCoverage: Record<string, unknown>; generatedAt: string;
  findings: { id: string; severity: string; ruleID: string; category: string; path: string; line: number; summary: string; remediation: string }[];
}
const EMPTY_REPORT: Report = { status: '', overallScore: 1, secretsFound: 0, outdatedDeps: 0, blockedDeploy: false, owaspCoverage: {}, generatedAt: '', findings: [] };

function normSev(s: string): Severity {
  const l = s.toLowerCase();
  if (l.startsWith('crit')) return 'critical';
  if (l.startsWith('high')) return 'high';
  if (l.startsWith('med') || l === 'moderate' || l === 'warning') return 'medium';
  return 'low';
}
function sevColor(t: Theme, s: Severity): string {
  switch (s) {
    case 'critical': return t.palette.error.main;
    case 'high': return t.palette.error.main;
    case 'medium': return t.palette.warning.main;
    default: return t.palette.text.disabled;
  }
}
const isPass = (s: string) => ['pass', 'passed', 'clean'].includes(s.toLowerCase());
const scannerLabel = (status: string, live: boolean) => {
  if (!live) return 'not connected';
  if (!status) return 'waiting';
  return status.replace(/_/g, ' ');
};

// Trigger a browser download for a generated text artifact (no backend round-trip).
function downloadText(filename: string, content: string, mime = 'application/json') {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// Render the live findings as a minimal SARIF 2.1.0 document.
function buildSarif(rows: Row[]) {
  const level = (s: Severity) => (s === 'critical' || s === 'high' ? 'error' : s === 'medium' ? 'warning' : 'note');
  return {
    $schema: 'https://json.schemastore.org/sarif-2.1.0.json',
    version: '2.1.0',
    runs: [
      {
        tool: { driver: { name: 'Ironflyer AppSec', informationUri: 'https://ironflyer.dev', rules: [] } },
        results: rows.map((r) => ({
          ruleId: r.scanner,
          level: level(r.severity),
          message: { text: r.title },
          locations:
            r.location && r.location !== '—'
              ? [{ physicalLocation: { artifactLocation: { uri: r.location.split(':')[0] } } }]
              : [],
        })),
      },
    ],
  };
}

// A single scanner coverage tile — GlassPanel with live status dot + detail.
function ScannerTile({
  name, detail, status, source, iconName,
}: {
  name: string; detail: string; status: string; source: string; iconName: string;
}) {
  const t = useTheme();
  const tone =
    status === 'findings' ? t.palette.warning.main
    : status === 'clean' ? t.palette.success.main
    : t.palette.text.disabled;
  const accent = status === 'findings' ? t.palette.warning.main : status === 'clean' ? t.palette.success.main : undefined;
  return (
    <GlassPanel accent={accent} pad={2} sx={{ minWidth: 0 }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: tone, flexShrink: 0, boxShadow: `0 0 6px ${tone}88` }} />
        <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}>
          <TechIcon name={iconName} size={14} title={name} />
        </Box>
        <Typography sx={{ fontSize: text.s88, fontWeight: 700, flex: 1 }} noWrap>{name}</Typography>
      </Stack>
      <Typography sx={{ fontSize: text.s80, color: tone, fontWeight: 600 }}>{detail}</Typography>
      <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s62, color: 'text.disabled', mt: 0.5 })}>{source}</Typography>
    </GlassPanel>
  );
}

// Policy decision panel — deny-by-default, visually tinted on block.
function PolicyPanel({
  denied, criticalCount, report, owaspPairs, onDispatch,
}: {
  denied: boolean; criticalCount: number; report: Report; owaspPairs: [string, unknown][]; onDispatch: (msg: string) => void;
}) {
  const t = useTheme();
  const accent = denied ? t.palette.error.main : t.palette.success.main;
  return (
    <GlassPanel accent={accent} pad={2.5} sx={{ flex: 1 }}>
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 1.5 }}>
        <Box sx={{ color: accent, display: 'inline-flex' }}>
          <Icon name="shield" size={20} />
        </Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Typography sx={{ fontWeight: 800, fontSize: text.s100 }}>Policy Plane</Typography>
            <Chip
              size="small"
              label={denied ? 'DENY' : 'ALLOW'}
              sx={{
                height: 20, fontSize: text.s62, fontWeight: 800,
                bgcolor: `${accent}22`, color: accent,
              }}
            />
          </Stack>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, color: 'text.disabled', mt: 0.25 })}>
            deny-by-default · scanner {report.status || 'n/a'}
          </Typography>
        </Box>
      </Stack>

      <Typography sx={{ fontSize: text.s88, color: denied ? 'error.main' : 'success.main', fontWeight: 600, mb: 1.5 }}>
        {denied ? 'Deploy blocked — unresolved security findings.' : 'Deploy allowed — security gates satisfied.'}
      </Typography>

      {owaspPairs.length > 0 && (
        <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75, mb: 1.5 }}>
          {owaspPairs.map(([k, v]) => (
            <Chip key={k} size="small" label={`${k}: ${String(v)}`} sx={(th) => ({ height: 20, fontSize: text.s62, bgcolor: 'action.hover', fontFamily: th.brand.font.mono })} />
          ))}
        </Stack>
      )}

      <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', gap: 1 }}>
        {criticalCount > 0 && (
          <Button size="small" variant="contained" startIcon={<Icon name="sparkles" size={16} />} onClick={() => onDispatch(`${criticalCount} critical security finding${criticalCount === 1 ? '' : 's'}`)}>
            Fix critical first
          </Button>
        )}
        {denied && (
          <Button size="small" variant="outlined" color="inherit" onClick={() => onDispatch('the deploy-blocking security policy')}>
            Clear deploy block
          </Button>
        )}
      </Stack>
    </GlassPanel>
  );
}

// AppSec surface — real scanner report for the latest execution (SAST score,
// secrets, dependency advisories, OWASP coverage) plus the project's security
// gates and the deny-by-default deploy decision. AppSec leads with the risk
// dial and findings distribution before showing the grid detail.
export function SecurityPane({ fallback }: { fallback: SecurityState }) {
  const t = useTheme();
  const [selected, setSelected] = useState<Row[]>([]);
  const [tableView, setTableView] = useState('all');
  const [tableSearch, setTableSearch] = useState('');
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const projectId = storeProjectId ?? firstProjectId;
  const { dispatch, repairGate } = useDispatchAgent();

  const { latest } = useProjectExecutions(projectId);
  const { data: report, isLive: reportLive } = useGraphQLQuery<Report, { executionSecurityReport: Report }>({
    key: ['security-report', latest?.id ?? 'none'],
    operationName: 'ExecutionSecurityReport', query: operations.EXECUTION_SECURITY_REPORT,
    variables: { executionID: latest?.id }, fallbackData: EMPTY_REPORT, enabled: !!latest?.id,
    map: (r) => r.executionSecurityReport ?? EMPTY_REPORT,
  });

  const { data: gates } = useGraphQLQuery<GateVerdict[], { gates: GateVerdict[] }>({
    key: ['security-gates', projectId ?? 'none'], operationName: 'Gates', query: operations.GATES,
    variables: { projectId }, fallbackData: [], enabled: !!projectId, map: (r) => r.gates ?? [],
  });

  const { data: files } = useGraphQLQuery<RawFile[], { projectFiles: RawFile[] }>({
    key: ['security-files', projectId ?? 'none'], operationName: 'ProjectFiles', query: operations.PROJECT_FILES,
    variables: { id: projectId }, fallbackData: [], enabled: !!projectId, map: (r) => r.projectFiles ?? [],
  });
  const sbom = useMemo(() => {
    const f = files.find((x) => x.path === '.ironflyer/sbom.json');
    if (!f?.content) return null;
    let components = 0;
    try {
      const doc = JSON.parse(f.content) as { components?: unknown[] };
      components = Array.isArray(doc.components) ? doc.components.length : 0;
    } catch { /* malformed artifact — still downloadable as-is */ }
    return { content: f.content, components };
  }, [files]);

  const live = reportLive && !!latest?.id;

  const rows = useMemo<Row[]>(() => {
    if (!live) return fallback.findings.map((f) => ({ id: f.id, severity: f.severity, title: f.title, category: f.category, location: f.location, scanner: f.scanner, remediation: '' }));
    const out: Row[] = report.findings.map((f) => ({
      id: f.id, severity: normSev(f.severity), title: f.summary, category: f.category,
      location: f.line ? `${f.path}:${f.line}` : f.path, scanner: f.ruleID, remediation: f.remediation,
    }));
    for (const g of gates.filter((x) => ['security', 'vuln_scan'].includes(x.gate))) {
      g.issues.forEach((iss, i) => out.push({
        id: `${g.gate}-${i}`, severity: normSev(iss.severity || 'medium'), title: iss.message,
        category: g.gate === 'vuln_scan' ? 'vulnerability' : 'sast',
        location: iss.path ? `${iss.path}${iss.line ? `:${iss.line}` : ''}` : '—', scanner: g.gate, remediation: '',
      }));
    }
    return out.sort((a, b) => severityRank[a.severity] - severityRank[b.severity]);
  }, [live, report, gates, fallback]);

  const riskScore = live ? Math.round((1 - report.overallScore) * 100) : fallback.riskScore;
  const denied = live ? report.blockedDeploy : fallback.policy.effect === 'deny';
  const owaspPairs = useMemo(() => Object.entries(report.owaspCoverage ?? {}).slice(0, 8), [report.owaspCoverage]);
  const criticalRows = rows.filter((r) => r.severity === 'critical' || r.severity === 'high');
  const tableTabs = useMemo<StudioTableTab[]>(() => [
    { value: 'all', label: 'All', count: rows.length },
    { value: 'critical', label: 'Critical/High', count: criticalRows.length, tone: criticalRows.length ? 'error' : 'success' },
    { value: 'medium', label: 'Medium', count: rows.filter((r) => r.severity === 'medium').length, tone: 'warning' },
    { value: 'low', label: 'Low', count: rows.filter((r) => r.severity === 'low').length },
  ], [criticalRows.length, rows]);
  const visibleRows = useMemo(() => {
    const q = tableSearch.trim().toLowerCase();
    return rows.filter((row) => {
      if (tableView === 'critical' && !['critical', 'high'].includes(row.severity)) return false;
      if (tableView === 'medium' && row.severity !== 'medium') return false;
      if (tableView === 'low' && row.severity !== 'low') return false;
      return !q || [row.title, row.category, row.location, row.scanner, row.remediation, row.severity].some((value) => value.toLowerCase().includes(q));
    });
  }, [rows, tableSearch, tableView]);
  const scannerState = scannerLabel(report.status, live);
  const generated = report.generatedAt ? new Date(report.generatedAt).toLocaleString() : 'pending';

  const secGate = gates.find((g) => g.gate === 'security');
  const vulnGate = gates.find((g) => g.gate === 'vuln_scan');
  const coverage = [
    { id: 'sast', icon: 'security', name: 'SAST', status: secGate ? (isPass(secGate.status) ? 'clean' : 'findings') : 'not_run', detail: secGate ? (isPass(secGate.status) ? 'Clean' : `${secGate.issues.length} issues`) : 'Not run', source: 'security gate' },
    { id: 'vuln', icon: 'vuln_scan', name: 'Dep scan', status: vulnGate ? (isPass(vulnGate.status) ? 'clean' : 'findings') : 'not_run', detail: vulnGate ? (isPass(vulnGate.status) ? 'Clean' : `${vulnGate.issues.length} advisories`) : 'Not run', source: 'vuln_scan gate' },
    { id: 'secrets', icon: 'secrets', name: 'Secrets', status: report.secretsFound > 0 ? 'findings' : live ? 'clean' : 'not_run', detail: live ? `${report.secretsFound} found` : 'Not run', source: 'scanner report' },
    { id: 'deps', icon: 'bundle_size', name: 'Outdated deps', status: report.outdatedDeps > 0 ? 'findings' : live ? 'clean' : 'not_run', detail: live ? `${report.outdatedDeps} outdated` : 'Not run', source: 'scanner report' },
  ];

  // Risk gauge color — derived from palette, never raw hex.
  const riskGaugeColor =
    riskScore >= 60 ? t.palette.error.main
    : riskScore >= 30 ? t.palette.warning.main
    : t.palette.success.main;

  const severityChart = useMemo<EChartsOption>(() => {
    const counts: Record<Severity, number> = { critical: 0, high: 0, medium: 0, low: 0 };
    for (const r of rows) counts[r.severity]++;
    return horizontalBarOption(t, {
      labels: ['Critical', 'High', 'Medium', 'Low'],
      values: [counts.critical, counts.high, counts.medium, counts.low],
      colors: [t.palette.error.main, t.palette.warning.main, t.palette.primary.main, t.palette.text.disabled],
    });
  }, [rows, t]);

  // Donut by category — mirrors scanner breakdown.
  const categoryDonut = useMemo<EChartsOption>(() => {
    const byCategory: Record<string, number> = {};
    for (const r of rows) byCategory[r.category] = (byCategory[r.category] ?? 0) + 1;
    const data = Object.entries(byCategory).map(([name, value]) => ({ name, value }));
    return donutOption(t, {
      data,
      centerLabel: `${rows.length}\nfindings`,
      centerColor: rows.length > 0 ? t.palette.error.main : t.palette.success.main,
      emptyLabel: 'No findings',
    });
  }, [rows, t]);

  const columns = useMemo<DataGridColumn<Row>[]>(() => [
    {
      field: 'severity', headerName: 'Severity', width: 118,
      comparator: (a, b) => severityRank[a as Severity] - severityRank[b as Severity],
      cellRenderer: ({ data }: DataGridCellParams<Row, Severity>) => data ? (
        <Chip size="small" label={data.severity}
          sx={{ height: 20, fontSize: text.s62, fontWeight: 700, textTransform: 'uppercase',
            bgcolor: `${sevColor(t, data.severity)}22`, color: sevColor(t, data.severity) }} />
      ) : null,
    },
    {
      field: 'title', headerName: 'Finding', flex: 1.4, minWidth: 260,
      cellRenderer: ({ value }: DataGridCellParams<Row, string>) =>
        <Typography sx={{ fontSize: text.s86 }} noWrap>{value}</Typography>,
    },
    {
      field: 'category', headerName: 'Category', width: 150,
      cellRenderer: ({ data }: DataGridCellParams<Row>) => data ? (
        <Stack direction="row" alignItems="center" spacing={0.75}>
          <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}>
            <TechIcon name={data.category} size={14} title={data.category} />
          </Box>
          <Typography sx={{ fontSize: text.s82 }} noWrap>{data.category}</Typography>
        </Stack>
      ) : null,
    },
    { field: 'location', headerName: 'Location', flex: 1, minWidth: 190 },
    { field: 'scanner', headerName: 'Scanner', width: 150 },
    {
      colId: 'fix', headerName: '', width: 96, sortable: false, filter: false,
      cellRenderer: ({ data }: DataGridCellParams<Row>) => data ? (
        <Button size="small" variant="outlined" color="inherit"
          onClick={(e) => { e.stopPropagation(); void dispatch(`security finding ${data.scanner}: ${data.title}`); toast(`Dispatching agent to fix "${data.title}".`, 'info'); }}>
          Fix
        </Button>
      ) : null,
    },
  ], [t, dispatch]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: { xs: 2, md: 3 } }}>
      <Box sx={{ maxWidth: 1160, mx: 'auto' }}>

        {/* ── Header ─────────────────────────────────────────────────────── */}
        <SectionHeader
          eyebrow="AppSec"
          title={
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <Typography variant="h5" sx={{ fontWeight: 800 }}>Security Analysis</Typography>
              <Chip size="small" label={live ? 'live' : 'sample data'}
                sx={(th) => ({ height: 20, fontSize: text.s62, fontFamily: th.brand.font.mono,
                  bgcolor: live ? `${th.palette.success.main}22` : 'action.hover',
                  color: live ? 'success.main' : 'text.disabled' })} />
            </Stack>
          }
          subtitle={`${rows.length} findings · ${report.secretsFound} secrets · ${report.outdatedDeps} outdated deps · scanner: ${scannerState} · generated: ${generated}`}
          actions={
            <Stack direction="row" spacing={1}>
              <Tooltip title={sbom ? `Export SBOM (${sbom.components} components)` : 'SBOM pending run'}>
                <span>
                  <Button size="small" variant="outlined" color="inherit" disabled={!sbom}
                    onClick={() => { if (!sbom) return; downloadText('sbom.json', sbom.content); toast('SBOM exported (CycloneDX JSON).', 'success'); }}>
                    Export SBOM
                  </Button>
                </span>
              </Tooltip>
              <Button size="small" variant="outlined" color="inherit" disabled={rows.length === 0}
                onClick={() => { downloadText('findings.sarif', JSON.stringify(buildSarif(rows), null, 2)); toast('Findings exported (SARIF 2.1.0).', 'success'); }}>
                Export SARIF
              </Button>
              <Button size="small" variant="contained" startIcon={<Icon name="sparkles" size={16} />} onClick={() => void repairGate('security', 'Security')}>
                Re-run security
              </Button>
            </Stack>
          }
        />

        {/* ── Lead visual row: Risk Gauge + Severity bars + Category donut ── */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '220px 1fr 240px' }, gap: 2, mb: 3 }}>
          {/* Risk gauge — the primary risk read at a glance */}
          <GlassPanel
            accent={riskGaugeColor}
            pad={2.5}
            sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}
          >
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s64, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>
              Risk Score
            </Typography>
            <GaugeRing
              value={riskScore}
              color={riskGaugeColor}
              formatter="{value}"
              height={160}
            />
            <Stack direction="row" spacing={1} sx={{ mt: 0.5 }}>
              {[
                { label: `${criticalRows.length} critical`, color: t.palette.error.main },
                { label: `${rows.length - criticalRows.length} other`, color: t.palette.text.disabled },
              ].map((item) => (
                <Typography key={item.label} sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s62, color: item.color })}>
                  {item.label}
                </Typography>
              ))}
            </Stack>
          </GlassPanel>

          <GlassPanel pad={2.5}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s64, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>
              Findings by Severity
            </Typography>
            <StudioChart option={severityChart} height={200} />
          </GlassPanel>

          {/* Donut — findings by category */}
          <GlassPanel pad={2.5}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s64, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>
              By Category
            </Typography>
            <StudioChart option={categoryDonut} height={200} />
          </GlassPanel>
        </Box>

        {/* ── Scanner coverage tiles + Policy plane ──────────────────────── */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 2, mb: 3 }}>
          {/* Coverage scanners */}
          <Box>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>
              Scanner Coverage
            </Typography>
            <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1.5 }}>
              {coverage.map((s) => (
                <ScannerTile key={s.id} name={s.name} detail={s.detail} status={s.status} source={s.source} iconName={s.icon} />
              ))}
            </Box>
          </Box>

          {/* Policy plane */}
          <Box>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>
              Deploy Decision
            </Typography>
            <PolicyPanel
              denied={denied}
              criticalCount={criticalRows.length}
              report={report}
              owaspPairs={owaspPairs}
              onDispatch={(msg) => void dispatch(msg)}
            />
          </Box>
        </Box>

        {/* ── Priority blockers callout ───────────────────────────────────── */}
        {criticalRows.length > 0 && (
          <GlassPanel accent={t.palette.error.main} pad={2} sx={{ mb: 3 }}>
            <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'error.main' })}>
                Priority Blockers
              </Typography>
              <Chip size="small" label={criticalRows.length}
                sx={(th) => ({ height: 18, fontSize: text.s62, bgcolor: `${th.palette.error.main}22`, color: 'error.main' })} />
            </Stack>
            <Stack spacing={1}>
              {criticalRows.slice(0, 3).map((r) => (
                <Box key={r.id}
                  sx={{ display: 'flex', alignItems: 'center', gap: 1.5, p: 1.5, borderRadius: (th) => `${th.studio.radius.sm}px`,
                    bgcolor: `${t.palette.error.main}0A`, border: '1px solid', borderColor: `${t.palette.error.main}22` }}>
                  <Chip size="small" label={r.severity}
                    sx={{ height: 20, fontSize: text.s62, fontWeight: 700,
                      bgcolor: `${sevColor(t, r.severity)}22`, color: sevColor(t, r.severity) }} />
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography sx={{ fontSize: text.s86, fontWeight: 600 }} noWrap>{r.title}</Typography>
                    {r.location && r.location !== '—' && (
                      <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s68, color: 'text.disabled' })} noWrap>
                        {r.location}
                      </Typography>
                    )}
                  </Box>
                  <Button size="small" variant="outlined" color="error"
                    onClick={() => void dispatch(`security finding ${r.scanner}: ${r.title}`)}>
                    Fix
                  </Button>
                </Box>
              ))}
            </Stack>
          </GlassPanel>
        )}

        <StudioDataGrid
          title="Security findings"
          subtitle={`${visibleRows.length.toLocaleString()} visible · ${selected.length.toLocaleString()} selected · scanner ${scannerState}`}
          tabs={tableTabs}
          activeTab={tableView}
          onTabChange={setTableView}
          searchValue={tableSearch}
          onSearchChange={setTableSearch}
          searchPlaceholder="Search findings"
          actions={
            <Button size="small" variant="contained" disabled={selected.length === 0}
              startIcon={<Icon name="sparkles" size={16} />}
              onClick={() => {
                void dispatch(`${selected.length} selected security finding${selected.length > 1 ? 's' : ''}`);
                toast(`Dispatching agent to fix ${selected.length} finding${selected.length > 1 ? 's' : ''}.`, 'success');
              }}>
              Fix selected{selected.length > 0 ? ` (${selected.length})` : ''}
            </Button>
          }
          footer="Findings can be exported as SARIF; selected rows can be dispatched directly to the AppSec agent."
          rows={visibleRows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel={live ? 'No findings — the latest scan is clean.' : 'Connect the orchestrator to load the live scan.'}
          height={visibleRows.length > 6 ? 420 : 280}
          minHeight={240}
          gridOptions={{
            suppressPaginationPanel: visibleRows.length <= 8,
            rowSelection: { mode: 'multiRow' },
            onSelectionChanged: (e) => setSelected(e.api.getSelectedRows()),
          }}
          pagination={visibleRows.length > 8}
          pageSize={8}
        />

        {/* ── Agent checklist ────────────────────────────────────────────── */}
        <GlassPanel pad={2.5} sx={{ mt: 3 }}>
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1.5 }}>
            <Stack direction="row" spacing={1.5} alignItems="center">
              <Box
                sx={(th) => ({
                  width: 44,
                  height: 44,
                  borderRadius: `${th.studio.radius.md}px`,
                  display: 'grid',
                  placeItems: 'center',
                  background: `linear-gradient(160deg, ${th.palette.primary.main}14, ${th.studio.neon.violet}0F)`,
                  border: `1px solid ${th.palette.borderSubtle}`,
                  flexShrink: 0,
                })}
              >
                <BrandAsset name="security.hero" size={32} />
              </Box>
              <Box>
                <Typography sx={{ fontWeight: 800, fontSize: text.s95 }}>AppSec Agent</Typography>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s64, color: 'text.disabled' })}>
                  deny-by-default · zero-trust
                </Typography>
              </Box>
            </Stack>
            <Chip size="small" label={live ? 'Live' : 'Standby'}
              sx={(th) => ({ height: 20, fontSize: text.s62, color: live ? 'success.main' : 'text.disabled',
                bgcolor: live ? `${th.palette.success.main}22` : 'action.hover' })} />
          </Stack>
          <Divider sx={{ mb: 1.5 }} />
          <Stack spacing={0.85}>
            {[
              { label: 'SAST scanner connected', done: live },
              { label: 'Dependency advisories pulled', done: live },
              { label: 'Secrets scan complete', done: live && report.secretsFound === 0 },
              { label: 'Policy plane evaluated', done: true },
              { label: 'SARIF export ready', done: rows.length >= 0 },
            ].map((item) => (
              <Stack key={item.label} direction="row" spacing={1.2} alignItems="center">
                <Box sx={(th) => ({
                  width: 16, height: 16, borderRadius: '50%', flexShrink: 0, display: 'grid', placeItems: 'center',
                  bgcolor: item.done ? th.palette.success.main : 'transparent',
                  border: item.done ? 'none' : `2px solid ${th.palette.primary.main}`,
                  color: item.done ? th.palette.background.paper : 'transparent',
                })}>
                  {item.done && <Icon name="check" size={10} />}
                </Box>
                <Typography sx={{ fontSize: text.s84, color: item.done ? 'text.primary' : 'text.secondary' }}>
                  {item.label}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </GlassPanel>

      </Box>
    </Box>
  );
}
