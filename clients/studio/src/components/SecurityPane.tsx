import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { DataGrid, type DataGridCellParams, type DataGridColumn } from '@ironflyer/ui-web/data-grid';
import { toast } from '@ironflyer/ui-web/fx';
import { categoryLabel, severityRank, type SecurityFinding, type SecurityState, type Severity } from '../studioData';

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
function scannerColor(t: Theme, status: string): string {
  return status === 'findings' ? t.palette.warning.main : status === 'clean' ? t.palette.success.main : t.palette.text.disabled;
}

// AppSec surface — mirrors core/orchestrator/internal/appsec: scanner coverage,
// findings, the deny-by-default policy decision, and SBOM/SARIF export. This is
// the capability competitors don't have; it gets a first-class tab.
export function SecurityPane({ security }: { security: SecurityState }) {
  const t = useTheme();
  const findings = [...security.findings].sort((a, b) => severityRank[a.severity] - severityRank[b.severity]);
  const denied = security.policy.effect === 'deny';
  const [selected, setSelected] = useState<SecurityFinding[]>([]);
  const findingColumns = useMemo<DataGridColumn<SecurityFinding>[]>(
    () => [
      {
        field: 'severity',
        headerName: 'Severity',
        width: 118,
        comparator: (a, b) => severityRank[a as Severity] - severityRank[b as Severity],
        cellRenderer: ({ data }: DataGridCellParams<SecurityFinding, Severity>) =>
          data ? (
            <Chip
              size="small"
              label={data.severity}
              sx={{
                height: 20,
                fontSize: '0.62rem',
                fontWeight: 700,
                textTransform: 'uppercase',
                bgcolor: `${sevColor(t, data.severity)}22`,
                color: sevColor(t, data.severity),
              }}
            />
          ) : null,
      },
      {
        field: 'title',
        headerName: 'Finding',
        flex: 1.4,
        minWidth: 260,
        cellRenderer: ({ value }: DataGridCellParams<SecurityFinding, string>) => (
          <Typography sx={{ fontSize: '0.86rem', overflow: 'hidden', textOverflow: 'ellipsis' }} noWrap>
            {value}
          </Typography>
        ),
      },
      {
        field: 'category',
        headerName: 'Category',
        width: 132,
        valueFormatter: ({ value }) => categoryLabel[value as SecurityFinding['category']] ?? String(value ?? ''),
      },
      { field: 'location', headerName: 'Location', flex: 1, minWidth: 190 },
      { field: 'scanner', headerName: 'Scanner', width: 142 },
      {
        colId: 'fix',
        headerName: '',
        width: 96,
        sortable: false,
        filter: false,
        cellRenderer: ({ data }: DataGridCellParams<SecurityFinding>) =>
          data ? (
            <Button
              size="small"
              variant="outlined"
              color="inherit"
              onClick={(event) => {
                event.stopPropagation();
                toast(`${data.scanner} → drafting a fix for "${data.title}".`, 'info');
              }}
            >
              Fix
            </Button>
          ) : null,
      },
    ],
    [t],
  );

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        {/* header */}
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 3 }}>
          <Card sx={{ p: 2.5, flex: 1, display: 'flex', alignItems: 'center', gap: 2.5 }}>
            <Box sx={{ textAlign: 'center' }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.display, fontSize: '2.6rem', fontWeight: 700, lineHeight: 1, color: riskColor(th, security.riskScore) })}>{security.riskScore}</Typography>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.62rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>risk score</Typography>
            </Box>
            <Box sx={{ flex: 1 }}>
              <Typography variant="h6" sx={{ fontSize: '1.1rem' }}>AppSec</Typography>
              <Typography sx={{ color: 'text.secondary', fontSize: '0.88rem' }}>{security.findings.length} open findings · SBOM: {security.sbom.components} components ({security.sbom.format})</Typography>
            </Box>
            <Stack spacing={1}>
              <Button size="small" variant="outlined" color="inherit" onClick={() => toast('SBOM exported (CycloneDX JSON).', 'success')}>Export SBOM</Button>
              <Button size="small" variant="outlined" color="inherit" onClick={() => toast('Findings exported (SARIF).', 'success')}>Export SARIF</Button>
            </Stack>
          </Card>

          {/* policy decision */}
          <Card sx={{ p: 2.5, flex: 1, borderColor: denied ? 'error.main' : 'divider', borderWidth: denied ? 1.5 : 1, borderStyle: 'solid' }}>
            <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Policy plane</Typography>
              <Chip size="small" label={security.policy.effect.toUpperCase()} sx={(th) => ({ height: 18, fontSize: '0.62rem', fontWeight: 700, bgcolor: `${denied ? th.palette.error.main : th.palette.success.main}22`, color: denied ? 'error.main' : 'success.main' })} />
            </Stack>
            <Typography sx={{ fontSize: '0.9rem', mb: 0.5 }}>{security.policy.reason.replace(/_/g, ' ')}</Typography>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled', mb: 1.25 })}>decision {security.policy.decisionId} · risk {security.policy.risk} · deny by default</Typography>
            <Stack direction="row" spacing={0.75} sx={{ flexWrap: 'wrap', gap: 0.75 }}>
              {security.policy.obligations.map((o) => (
                <Chip key={o} size="small" label={o} sx={{ height: 20, fontSize: '0.64rem', bgcolor: 'action.hover', fontFamily: 'var(--if-font-mono)' }} />
              ))}
            </Stack>
          </Card>
        </Stack>

        {/* scanner coverage */}
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Coverage</Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(4, 1fr)' }, gap: 1.5, mb: 4 }}>
          {security.scanners.map((s) => (
            <Card key={s.id} sx={{ p: 2 }}>
              <Stack direction="row" alignItems="center" spacing={1}>
                <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: scannerColor(t, s.status) }} />
                <Typography sx={{ fontSize: '0.9rem', fontWeight: 600, flex: 1 }} noWrap>{s.name}</Typography>
              </Stack>
              <Stack direction="row" alignItems="baseline" justifyContent="space-between" sx={{ mt: 1 }}>
                <Typography sx={{ fontSize: '0.8rem', color: s.status === 'findings' ? 'warning.main' : s.status === 'clean' ? 'success.main' : 'text.disabled' }}>
                  {s.status === 'not_run' ? 'not run' : s.status === 'clean' ? 'clean' : `${s.count} finding${s.count > 1 ? 's' : ''}`}
                </Typography>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.62rem', color: 'text.disabled' })}>{s.source}</Typography>
              </Stack>
            </Card>
          ))}
        </Box>

        {/* findings */}
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>
            Findings{selected.length > 0 ? ` · ${selected.length} selected` : ''}
          </Typography>
          <Button
            size="small"
            variant="contained"
            disabled={selected.length === 0}
            onClick={() => toast(`Dispatching agent to fix ${selected.length} finding${selected.length > 1 ? 's' : ''}.`, 'success')}
          >
            Fix selected{selected.length > 0 ? ` (${selected.length})` : ''}
          </Button>
        </Stack>
        <DataGrid
          rows={findings}
          columns={findingColumns}
          getRowId={(row) => row.id}
          density="compact"
          emptyLabel="No findings yet — run a scan."
          height={findings.length > 6 ? 420 : 280}
          minHeight={240}
          gridOptions={{
            suppressPaginationPanel: findings.length <= 8,
            rowSelection: { mode: 'multiRow' },
            onSelectionChanged: (e) => setSelected(e.api.getSelectedRows()),
          }}
          pagination={findings.length > 8}
          pageSize={8}
        />
      </Box>
    </Box>
  );
}
