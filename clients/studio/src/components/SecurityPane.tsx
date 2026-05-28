import { Box, Button, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { toast } from '@ironflyer/ui-web/fx';
import { categoryLabel, severityRank, type SecurityState, type Severity } from '../studioData';

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
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Findings</Typography>
        <Stack spacing={1}>
          {findings.length === 0 && <Typography sx={{ color: 'text.disabled' }}>No findings yet — run a scan.</Typography>}
          {findings.map((f) => (
            <Card key={f.id} sx={{ p: 2, display: 'flex', alignItems: 'center', gap: 1.5 }}>
              <Box sx={{ width: 64, flexShrink: 0 }}>
                <Chip size="small" label={f.severity} sx={{ height: 20, fontSize: '0.62rem', fontWeight: 700, textTransform: 'uppercase', bgcolor: `${sevColor(t, f.severity)}22`, color: sevColor(t, f.severity) }} />
              </Box>
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography sx={{ fontSize: '0.9rem' }} noWrap>{f.title}</Typography>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.72rem', color: 'text.disabled' })} noWrap>{f.location} · {categoryLabel[f.category]} · {f.scanner}</Typography>
              </Box>
              <Button size="small" variant="outlined" color="inherit" sx={{ flexShrink: 0 }} onClick={() => toast(`${f.scanner} → drafting a fix for "${f.title}".`, 'info')}>Fix</Button>
            </Card>
          ))}
        </Stack>
      </Box>
    </Box>
  );
}
