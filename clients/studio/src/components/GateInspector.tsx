import { Box, Button, Chip, Divider, Drawer, IconButton, LinearProgress, Stack, Typography } from '@mui/material';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useStudio } from '../store';
import { statusLabel, agentForGate } from '../studioData';
import { statusColor } from './statusColor';

// One-click drill-in from the Map or Dashboard: a gate's status, owning agent,
// what's blocking, findings, and reviewable patches with actions.
export function GateInspector() {
  const project = useStudio((s) => s.current);
  const selectedId = useStudio((s) => s.selectedGateId);
  const selectGate = useStudio((s) => s.selectGate);
  const gate = project.gates.find((g) => g.id === selectedId) ?? null;
  const agent = gate ? agentForGate(gate.id) : undefined;
  const close = () => selectGate(null);

  const applyPatch = async (title: string) => {
    const ok = await confirmAction({ title: 'Apply patch?', text: title, confirmText: 'Apply' });
    if (ok) toast('Patch applied — re-running the gate.', 'success');
  };

  return (
    <Drawer anchor="right" open={!!gate} onClose={close} PaperProps={{ sx: { width: { xs: '100%', sm: 420 }, bgcolor: 'background.paper', backgroundImage: 'none' } }}>
      {gate && (
        <Box sx={{ p: 3, display: 'flex', flexDirection: 'column', height: '100%' }}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
            <Stack direction="row" alignItems="center" spacing={1.25}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, color: 'text.disabled' })}>{gate.no}</Typography>
              <Typography variant="h5" sx={{ fontSize: '1.4rem' }}>{gate.name}</Typography>
            </Stack>
            <IconButton onClick={close} size="small" aria-label="Close" sx={{ color: 'text.secondary' }}>✕</IconButton>
          </Stack>

          <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 2 }}>
            <Chip size="small" label={statusLabel[gate.status]} sx={(t) => ({ bgcolor: `${statusColor(t, gate.status)}22`, color: statusColor(t, gate.status), fontWeight: 600 })} />
            {agent && <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.78rem', color: 'text.secondary' })}>{agent.name}</Typography>}
          </Stack>

          <LinearProgress variant="determinate" value={Math.round(gate.level * 100)} sx={(t) => ({ height: 6, borderRadius: 99, bgcolor: 'action.hover', mb: 2, '& .MuiLinearProgress-bar': { borderRadius: 99, bgcolor: statusColor(t, gate.status) } })} />

          {gate.blocking ? (
            <Box sx={{ p: 1.5, borderRadius: 2, bgcolor: 'action.hover', mb: 2 }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', color: 'text.disabled', mb: 0.5 })}>BLOCKING</Typography>
              <Typography sx={{ fontSize: '0.9rem' }}>{gate.blocking}</Typography>
            </Box>
          ) : (
            <Typography sx={{ color: 'success.main', mb: 2 }}>● Closed end-to-end</Typography>
          )}

          <Box sx={{ flex: 1, overflowY: 'auto' }}>
            {gate.findings.length > 0 && (
              <>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Findings</Typography>
                <Stack spacing={1} sx={{ mb: 3 }}>
                  {gate.findings.map((f) => (
                    <Stack key={f.id} direction="row" spacing={1} alignItems="flex-start">
                      <Box component="span" sx={{ mt: '2px', color: f.severity === 'danger' ? 'error.main' : f.severity === 'warning' ? 'warning.main' : 'text.disabled' }}>●</Box>
                      <Typography sx={{ fontSize: '0.88rem', color: 'text.secondary' }}>{f.text}</Typography>
                    </Stack>
                  ))}
                </Stack>
              </>
            )}

            {gate.patches.length > 0 && (
              <>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Patches</Typography>
                <Stack spacing={1.25}>
                  {gate.patches.map((p) => (
                    <Box key={p.id} sx={{ p: 1.5, border: 1, borderColor: 'divider', borderRadius: 2 }}>
                      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: p.state === 'proposed' ? 1 : 0 }}>
                        <Chip size="small" label={p.state} sx={{ height: 18, fontSize: '0.62rem', bgcolor: 'action.hover' }} />
                        <Typography sx={{ fontSize: '0.86rem', flex: 1 }}>{p.title}</Typography>
                        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled' })}>+{p.lines}</Typography>
                      </Stack>
                      {p.state === 'proposed' && (
                        <Button size="small" variant="contained" onClick={() => applyPatch(p.title)}>Review & apply</Button>
                      )}
                    </Box>
                  ))}
                </Stack>
              </>
            )}
          </Box>

          <Divider sx={{ my: 2 }} />
          <Stack direction="row" spacing={1.5}>
            <Button fullWidth variant="outlined" color="inherit" onClick={() => toast(`${agent?.name ?? 'Agent'} re-running ${gate.name} gate…`, 'info')}>Run agent</Button>
            <Button fullWidth variant="contained" disabled={!!gate.blocking} onClick={() => toast(`${gate.name} gate closed.`, 'success')}>Mark closed</Button>
          </Stack>
        </Box>
      )}
    </Drawer>
  );
}
