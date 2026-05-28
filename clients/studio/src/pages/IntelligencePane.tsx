import { useMemo, useState } from 'react';
import { Box, Button, Card, Chip, Dialog, DialogActions, DialogContent, DialogTitle, LinearProgress, Stack, Typography } from '@mui/material';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { toast } from '@ironflyer/ui-web/fx';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import type { StudioProject } from '../studioData';

interface RawGate { gate: string; status: string; issues: { severity: string; message: string }[] }
interface RawFile { path: string; size: number; language: string }

const CATEGORIES: Record<string, string[]> = {
  Core: ['spec', 'ux', 'arch', 'code', 'drift', 'verifier', 'lint', 'test'],
  Security: ['security', 'vuln_scan', 'compliance_soc2', 'compliance_hipaa', 'compliance_pci', 'compliance_gdpr', 'ios_privacy_manifest', 'mobile_security'],
  Quality: ['reuse_check', 'dedup', 'deadcode', 'complexity', 'dep_graph', 'arch_boundary', 'mem_leak'],
  Performance: ['lighthouse', 'bundle_size', 'perf_budget', 'mobile_size', 'mobile_bundle_analyzer'],
  Mobile: ['mobile_build', 'mobile_expo_doctor', 'mobile_push_credentials'],
  Ship: ['deploy', 'budget'],
};
const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
const isClosed = (s: string) => ['pass', 'passed'].includes(s.toLowerCase());
function categoryOf(gate: string): string {
  for (const [cat, gates] of Object.entries(CATEGORIES)) if (gates.includes(gate)) return cat;
  return 'Other';
}

export function IntelligencePane({ fallback }: { fallback: StudioProject }) {
  const liveProjectId = useLiveProjectId();
  const request = useRequest();
  const [open, setOpen] = useState<string | null>(null);

  const fallbackGates: RawGate[] = useMemo(
    () => fallback.gates.map((g) => ({ gate: g.id, status: g.blocking ? 'blocked' : 'pass', issues: g.findings.map((f) => ({ severity: f.severity, message: f.text })) })),
    [fallback],
  );

  const { data: gates, isLive } = useGraphQLQuery<RawGate[], { gates: RawGate[] }>({
    key: ['intel-gates', liveProjectId ?? 'none'], operationName: 'Gates', query: operations.GATES,
    variables: { projectId: liveProjectId }, fallbackData: fallbackGates, enabled: !!liveProjectId, map: (r) => r.gates,
  });

  const { data: files } = useGraphQLQuery<RawFile[], { projectFiles: RawFile[] }>({
    key: ['intel-files', liveProjectId ?? 'none'], operationName: 'ProjectFiles', query: operations.PROJECT_FILES,
    variables: { id: liveProjectId }, fallbackData: [], enabled: !!liveProjectId, map: (r) => r.projectFiles,
  });

  const languages = useMemo(() => {
    const by = new Map<string, number>();
    for (const f of files) {
      const lang = (f.language || '').trim();
      if (!lang || lang === 'file') continue;
      by.set(lang, (by.get(lang) ?? 0) + (f.size || 1));
    }
    const total = [...by.values()].reduce((a, b) => a + b, 0);
    return total === 0 ? [] : [...by.entries()].map(([name, size]) => ({ name, pct: Math.round((size / total) * 100) })).sort((a, b) => b.pct - a.pct).slice(0, 6);
  }, [files]);

  const categories = useMemo(() => {
    const acc: Record<string, RawGate[]> = {};
    for (const g of gates) (acc[categoryOf(g.gate)] ??= []).push(g);
    return Object.entries(acc).map(([name, gs]) => ({ name, gates: gs, total: gs.length, closed: gs.filter((g) => isClosed(g.status)).length }))
      .sort((a, b) => a.closed / a.total - b.closed / b.total);
  }, [gates]);

  const closedTotal = gates.filter((g) => isClosed(g.status)).length;
  const dialogCat = categories.find((c) => c.name === open);

  const dispatchAgent = async (scope: string) => {
    setOpen(null);
    if (!request || !liveProjectId) { toast('Connect to dispatch an agent.', 'info'); return; }
    try {
      await request('RunFinisher', operations.RUN_FINISHER, { id: liveProjectId });
      toast(`Agent dispatched to finish ${scope}.`, 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not dispatch agent.', 'error');
    }
  };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 3, flexWrap: 'wrap', gap: 1 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Intelligence</Typography>
            <Chip size="small" label={isLive ? 'live' : 'sample'} sx={(t) => ({ height: 20, fontSize: '0.64rem', fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
            <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{closedTotal}/{gates.length} checks closed end-to-end</Typography>
          </Stack>
          <Button variant="contained" onClick={() => dispatchAgent('the open work')}>Dispatch agent to finish</Button>
        </Stack>

        {/* Languages */}
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Composition</Typography>
        <Card sx={{ p: 2.5, mb: 3 }}>
          {languages.length === 0 ? (
            <Typography sx={{ color: 'text.disabled', fontSize: '0.9rem' }}>Code not indexed yet — dispatch the agent to map the stack, then language breakdown shows here.</Typography>
          ) : (
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
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.disabled', mt: 0.5 })}>{files.length} files indexed</Typography>
            </Stack>
          )}
        </Card>

        {/* Feature closure */}
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Closed end-to-end?</Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 1.5 }}>
          {categories.map((c) => {
            const pct = Math.round((c.closed / c.total) * 100);
            const done = c.closed === c.total;
            return (
              <Card key={c.name} onClick={() => setOpen(c.name)} sx={{ p: 2.5, cursor: 'pointer', transition: (t) => `border-color ${t.brand.motion.fast}`, '&:hover': { borderColor: 'text.disabled' } }}>
                <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
                  <Typography variant="h6" sx={{ fontSize: '1.05rem' }}>{c.name}</Typography>
                  <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.78rem', color: done ? 'success.main' : 'warning.main' })}>{c.closed}/{c.total}</Typography>
                </Stack>
                <LinearProgress variant="determinate" value={pct} sx={(t) => ({ height: 6, borderRadius: 99, bgcolor: 'action.hover', '& .MuiLinearProgress-bar': { borderRadius: 99, bgcolor: done ? t.palette.success.main : t.palette.warning.main } })} />
                <Typography sx={{ fontSize: '0.78rem', color: 'text.disabled', mt: 1 }}>{done ? 'Closed' : `${c.total - c.closed} open · click for details`}</Typography>
              </Card>
            );
          })}
        </Box>
      </Box>

      <Dialog open={!!dialogCat} onClose={() => setOpen(null)} maxWidth="sm" fullWidth slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}>
        <DialogTitle>{dialogCat?.name} — what's missing</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={1.5}>
            {dialogCat?.gates.map((g) => {
              const closed = isClosed(g.status);
              return (
                <Box key={g.gate}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: closed ? 'success.main' : 'warning.main' }} />
                    <Typography sx={{ fontSize: '0.9rem', fontWeight: 600, flex: 1 }}>{titleCase(g.gate)}</Typography>
                    <Chip size="small" label={g.status} sx={{ height: 18, fontSize: '0.62rem', bgcolor: 'action.hover' }} />
                  </Stack>
                  {!closed && g.issues.length > 0 && (
                    <Stack spacing={0.25} sx={{ pl: 2.25, mt: 0.5 }}>
                      {g.issues.map((iss, i) => (
                        <Typography key={i} sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>
                          <Box component="span" sx={{ color: iss.severity === 'error' ? 'error.main' : 'warning.main' }}>• </Box>{iss.message}
                        </Typography>
                      ))}
                    </Stack>
                  )}
                </Box>
              );
            })}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(null)} color="inherit">Close</Button>
          <Button variant="contained" onClick={() => dispatchAgent(dialogCat?.name ?? 'this')}>Dispatch agent to finish {dialogCat?.name}</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
