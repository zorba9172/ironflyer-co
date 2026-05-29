import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { severityRank, type SecurityState, type Severity } from '../studioData';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useStudio } from '../store';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { TechIcon } from '../lib/techIcons';

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
function riskColor(t: Theme, score: number): string {
  return score >= 60 ? t.palette.error.main : score >= 30 ? t.palette.warning.main : t.palette.success.main;
}
const isPass = (s: string) => ['pass', 'passed', 'clean'].includes(s.toLowerCase());

// Trigger a browser download for a generated text artifact (no backend round-trip).
function downloadText(filename: string, text: string, mime = 'application/json') {
  const blob = new Blob([text], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// Render the live findings as a minimal SARIF 2.1.0 document so the operator
// can export exactly what they see into any SARIF-consuming tool (GitHub code
// scanning, etc.). Severity maps onto the SARIF level vocabulary.
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

// AppSec surface — real scanner report for the latest execution (SAST score,
// secrets, dependency advisories, OWASP coverage) plus the project's security
// gates and the deny-by-default deploy decision. AppSec is the headline
// differentiator, so it gets first-class, live treatment.
export function SecurityPane({ fallback }: { fallback: SecurityState }) {
  const t = useTheme();
  const [selected, setSelected] = useState<Row[]>([]);
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const projectId = storeProjectId ?? firstProjectId;
  const { dispatch } = useDispatchAgent();

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

  // The finisher writes a CycloneDX SBOM to .ironflyer/sbom.json on every run
  // (artifact.sbom.published.v1). projectFiles already returns content, so the
  // Export SBOM action downloads the real document with no extra round-trip.
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
    } catch {
      /* malformed artifact — still downloadable as-is */
    }
    return { content: f.content, components };
  }, [files]);

  const live = reportLive && !!latest?.id;

  // Findings: prefer the real scanner report; merge in any security/vuln gate
  // issues; fall back to the offline sample when fully disconnected.
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

  // Coverage cards from real gates + report counts.
  const secGate = gates.find((g) => g.gate === 'security');
  const vulnGate = gates.find((g) => g.gate === 'vuln_scan');
  const coverage = [
    { id: 'sast', icon: 'security', name: 'SAST', status: secGate ? (isPass(secGate.status) ? 'clean' : 'findings') : 'not_run', detail: secGate ? (isPass(secGate.status) ? 'clean' : `${secGate.issues.length} issues`) : 'not run', source: 'security gate' },
    { id: 'vuln', icon: 'vuln_scan', name: 'Dependency scan', status: vulnGate ? (isPass(vulnGate.status) ? 'clean' : 'findings') : 'not_run', detail: vulnGate ? (isPass(vulnGate.status) ? 'clean' : `${vulnGate.issues.length} advisories`) : 'not run', source: 'vuln_scan gate' },
    { id: 'secrets', icon: 'secrets', name: 'Secrets', status: report.secretsFound > 0 ? 'findings' : live ? 'clean' : 'not_run', detail: live ? `${report.secretsFound} found` : 'not run', source: 'scanner report' },
    { id: 'deps', icon: 'bundle_size', name: 'Outdated deps', status: report.outdatedDeps > 0 ? 'findings' : live ? 'clean' : 'not_run', detail: live ? `${report.outdatedDeps} outdated` : 'not run', source: 'scanner report' },
  ];
  const scannerColor = (status: string) => (status === 'findings' ? t.palette.warning.main : status === 'clean' ? t.palette.success.main : t.palette.text.disabled);

  const columns = useMemo<DataGridColumn<Row>[]>(() => [
    {
      field: 'severity', headerName: 'Severity', width: 118,
      comparator: (a, b) => severityRank[a as Severity] - severityRank[b as Severity],
      cellRenderer: ({ data }: DataGridCellParams<Row, Severity>) => data ? (
        <Chip size="small" label={data.severity} sx={{ height: 20, fontSize: '0.62rem', fontWeight: 700, textTransform: 'uppercase', bgcolor: `${sevColor(t, data.severity)}22`, color: sevColor(t, data.severity) }} />
      ) : null,
    },
    { field: 'title', headerName: 'Finding', flex: 1.4, minWidth: 260, cellRenderer: ({ value }: DataGridCellParams<Row, string>) => <Typography sx={{ fontSize: '0.86rem' }} noWrap>{value}</Typography> },
    { field: 'category', headerName: 'Category', width: 150, cellRenderer: ({ data }: DataGridCellParams<Row>) => data ? (
      <Stack direction="row" alignItems="center" spacing={0.75}>
        <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}><TechIcon name={data.category} size={14} title={data.category} /></Box>
        <Typography sx={{ fontSize: '0.82rem' }} noWrap>{data.category}</Typography>
      </Stack>
    ) : null },
    { field: 'location', headerName: 'Location', flex: 1, minWidth: 190 },
    { field: 'scanner', headerName: 'Scanner', width: 150 },
    {
      colId: 'fix', headerName: '', width: 96, sortable: false, filter: false,
      cellRenderer: ({ data }: DataGridCellParams<Row>) => data ? (
        <Button size="small" variant="outlined" color="inherit" onClick={(e) => { e.stopPropagation(); dispatch('security'); toast(`Dispatching agent to fix "${data.title}".`, 'info'); }}>Fix</Button>
      ) : null,
    },
  ], [t, dispatch]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 3 }}>
          <Card sx={{ p: 2.5, flex: 1, display: 'flex', alignItems: 'center', gap: 2.5 }}>
            <Box sx={{ textAlign: 'center' }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.display, fontSize: '2.6rem', fontWeight: 700, lineHeight: 1, color: riskColor(th, riskScore) })}>{riskScore}</Typography>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.62rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>risk score</Typography>
            </Box>
            <Box sx={{ flex: 1 }}>
              <Stack direction="row" alignItems="center" spacing={1}>
                <Typography variant="h6" sx={{ fontSize: '1.1rem' }}>AppSec</Typography>
                <Chip size="small" label={live ? 'live' : 'sample data'} sx={(th) => ({ height: 18, fontSize: '0.6rem', fontFamily: th.brand.font.mono, bgcolor: live ? `${th.palette.success.main}22` : 'action.hover', color: live ? 'success.main' : 'text.disabled' })} />
              </Stack>
              <Typography sx={{ color: 'text.secondary', fontSize: '0.88rem' }}>{rows.length} findings · {report.secretsFound} secrets · {report.outdatedDeps} outdated deps</Typography>
            </Box>
            <Stack spacing={1}>
              <Button
                size="small"
                variant="outlined"
                color="inherit"
                disabled={!sbom}
                onClick={() => { if (!sbom) return; downloadText('sbom.json', sbom.content); toast('SBOM exported (CycloneDX JSON).', 'success'); }}
              >
                {sbom ? `Export SBOM (${sbom.components})` : 'SBOM pending run'}
              </Button>
              <Button
                size="small"
                variant="outlined"
                color="inherit"
                disabled={rows.length === 0}
                onClick={() => { downloadText('findings.sarif', JSON.stringify(buildSarif(rows), null, 2)); toast('Findings exported (SARIF 2.1.0).', 'success'); }}
              >
                Export SARIF
              </Button>
            </Stack>
          </Card>

          <Card sx={{ p: 2.5, flex: 1, borderColor: denied ? 'error.main' : 'divider', borderWidth: denied ? 1.5 : 1, borderStyle: 'solid' }}>
            <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Policy plane</Typography>
              <Chip size="small" label={denied ? 'DENY' : 'ALLOW'} sx={(th) => ({ height: 18, fontSize: '0.62rem', fontWeight: 700, bgcolor: `${denied ? th.palette.error.main : th.palette.success.main}22`, color: denied ? 'error.main' : 'success.main' })} />
            </Stack>
            <Typography sx={{ fontSize: '0.9rem', mb: 0.5 }}>{denied ? 'Deploy blocked — unresolved security findings.' : 'Deploy allowed — security gates satisfied.'}</Typography>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled', mb: 1.25 })}>deny by default · scanner status {report.status || 'n/a'}</Typography>
            <Stack direction="row" spacing={0.75} sx={{ flexWrap: 'wrap', gap: 0.75 }}>
              {owaspPairs.map(([k, v]) => (
                <Chip key={k} size="small" label={`${k}: ${String(v)}`} sx={{ height: 20, fontSize: '0.64rem', bgcolor: 'action.hover', fontFamily: 'var(--if-font-mono)' }} />
              ))}
            </Stack>
          </Card>
        </Stack>

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Coverage</Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(4, 1fr)' }, gap: 1.5, mb: 4 }}>
          {coverage.map((s) => (
            <Card key={s.id} sx={{ p: 2 }}>
              <Stack direction="row" alignItems="center" spacing={1}>
                <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: scannerColor(s.status) }} />
                <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}><TechIcon name={s.icon} size={15} title={s.name} /></Box>
                <Typography sx={{ fontSize: '0.9rem', fontWeight: 600, flex: 1 }} noWrap>{s.name}</Typography>
              </Stack>
              <Stack direction="row" alignItems="baseline" justifyContent="space-between" sx={{ mt: 1 }}>
                <Typography sx={{ fontSize: '0.8rem', color: s.status === 'findings' ? 'warning.main' : s.status === 'clean' ? 'success.main' : 'text.disabled' }}>{s.detail}</Typography>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.62rem', color: 'text.disabled' })}>{s.source}</Typography>
              </Stack>
            </Card>
          ))}
        </Box>

        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>
            Findings{selected.length > 0 ? ` · ${selected.length} selected` : ''}
          </Typography>
          <Button size="small" variant="contained" disabled={selected.length === 0} onClick={() => { dispatch('security'); toast(`Dispatching agent to fix ${selected.length} finding${selected.length > 1 ? 's' : ''}.`, 'success'); }}>
            Fix selected{selected.length > 0 ? ` (${selected.length})` : ''}
          </Button>
        </Stack>
        <DataGrid
          rows={rows}
          columns={columns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel={live ? 'No findings — the latest scan is clean.' : 'Connect the orchestrator to load the live scan.'}
          height={rows.length > 6 ? 420 : 280}
          minHeight={240}
          gridOptions={{ suppressPaginationPanel: rows.length <= 8, rowSelection: { mode: 'multiRow' }, onSelectionChanged: (e) => setSelected(e.api.getSelectedRows()) }}
          pagination={rows.length > 8}
          pageSize={8}
        />
      </Box>
    </Box>
  );
}
