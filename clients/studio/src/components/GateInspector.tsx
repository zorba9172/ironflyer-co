import { useState } from 'react';
import { Box, Button, Chip, Divider, Drawer, IconButton, LinearProgress, Stack, Typography } from '@mui/material';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useRequest, operations } from '@ironflyer/data';
import { formatUSD } from '@ironflyer/core';
import { useStudio } from '../store';
import { statusLabel, agentForGate } from '../studioData';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useLiveGates } from '../hooks/useLiveGates';
import { useWallet, useSentinelForecast } from '../hooks/useEconomics';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { statusColor } from './statusColor';
import { PatchDiff } from './PatchDiff';
import { text } from '@ironflyer/design-tokens/brand';

// H3 — ProfitGuard verdict derived from the LIVE economic signals the
// orchestrator exposes today (sentinelForecast trajectory + wallet headroom).
// There is no first-class ProfitGuard *decision* field on the wire yet
// (FLAGGED in the report), so we mirror the same allow/hold/block story law 2
// enforces server-side: block when the wallet/headroom is exhausted (the 402
// path), hold when the trajectory is tight, allow otherwise.
type PGVerdict = 'allow' | 'hold' | 'block';
function deriveProfitGuard(
  forecast: { level: string; remainingHeadroomUSD: number; extrapolatedTotalUSD: number; burnRatePerHourUSD: number },
  available: number,
): { verdict: PGVerdict; tone: 'error' | 'warning' | 'success'; reason: string } {
  const exhausted = available <= 0 || forecast.remainingHeadroomUSD < 0;
  const tight = !exhausted && (forecast.level === 'warn' || forecast.level === 'critical' || forecast.remainingHeadroomUSD < 1);
  if (exhausted) {
    return {
      verdict: 'block', tone: 'error',
      reason: `ProfitGuard would block the next paid step — ${formatUSD(available)} available, ${formatUSD(forecast.remainingHeadroomUSD)} headroom. Top up before this gate can proceed (402).`,
    };
  }
  if (tight) {
    return {
      verdict: 'hold', tone: 'warning',
      reason: `ProfitGuard is holding for ROI — projected ${formatUSD(forecast.extrapolatedTotalUSD)} at ${formatUSD(forecast.burnRatePerHourUSD)}/h, only ${formatUSD(forecast.remainingHeadroomUSD)} headroom left.`,
    };
  }
  return {
    verdict: 'allow', tone: 'success',
    reason: `ProfitGuard clears this gate — ${formatUSD(forecast.remainingHeadroomUSD)} headroom, ${formatUSD(available)} available.`,
  };
}

// One-click drill-in from the Map or Dashboard: a gate's status, owning agent,
// what's blocking, findings, and reviewable patches with actions. Reads the
// live gate verdicts when a real project is open (shared cache with the map /
// dashboard) so clicking a live gate node opens the live gate, not the sample.
export function GateInspector() {
  const { gates } = useLiveGates();
  const selectedId = useStudio((s) => s.selectedGateId);
  const selectGate = useStudio((s) => s.selectGate);
  const request = useRequest();
  const { dispatch, repairGate } = useDispatchAgent();
  const [busy, setBusy] = useState(false);
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const projectId = storeProjectId ?? firstProjectId;
  const { wallet } = useWallet();
  const { forecast, isLive: economicsLive } = useSentinelForecast(projectId);
  const pg = deriveProfitGuard(forecast, wallet.availableUSD);
  const gate = gates.find((g) => g.id === selectedId) ?? null;
  const agent = gate ? agentForGate(gate.id) : undefined;
  const close = () => selectGate(null);
  const proposedPatches = gate?.patches.filter((p) => p.state === 'proposed').length ?? 0;
  const findingsCount = gate?.findings.length ?? 0;
  const scannerState = !gate ? 'idle' : gate.status === 'running' ? 'running' : gate.blocking ? 'blocked' : 'clean';
  const nextAction = !gate
    ? ''
    : proposedPatches > 0
      ? 'Review and apply the proposed patch, then re-run the gate.'
      : gate.blocking
        ? 'Dispatch the owning agent to produce a reviewable fix, or re-run the gate after manual changes.'
        : 'Keep this gate closed while the rest of the finisher runs.';

  // Apply a reviewed patch through the real applyPatch mutation (the patch
  // lifecycle re-runs the gate server-side). Offline → honest note.
  const applyPatch = async (patchId: string, title: string) => {
    const ok = await confirmAction({ title: 'Apply reviewed patch?', text: title, confirmText: 'Apply' });
    if (!ok) return;
    if (!request) { toast('Connect the orchestrator to apply patches.', 'info'); return; }
    setBusy(true);
    try {
      await request('ApplyPatch', operations.APPLY_PATCH, { patchId });
      toast('Patch applied — re-running the gate.', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not apply patch.', 'error');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Drawer anchor="right" open={!!gate} onClose={close} PaperProps={{ sx: { width: { xs: '100%', sm: 420 }, bgcolor: 'background.paper', backgroundImage: 'none' } }}>
      {gate && (
        <Box sx={{ p: 3, display: 'flex', flexDirection: 'column', height: '100%' }}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
            <Stack direction="row" alignItems="center" spacing={1.25}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, color: 'text.disabled' })}>{gate.no}</Typography>
              <Typography variant="h5" sx={{ fontSize: text.s140 }}>{gate.name}</Typography>
            </Stack>
            <IconButton onClick={close} size="small" aria-label="Close" sx={{ color: 'text.secondary' }}>✕</IconButton>
          </Stack>

          <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 2 }}>
            <Chip size="small" label={statusLabel[gate.status]} sx={(t) => ({ bgcolor: `${statusColor(t, gate.status)}22`, color: statusColor(t, gate.status), fontWeight: 600 })} />
            {agent && <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s78, color: 'text.secondary' })}>{agent.name}</Typography>}
          </Stack>

          <LinearProgress variant="determinate" value={Math.round(gate.level * 100)} sx={(t) => ({ height: 6, borderRadius: 99, bgcolor: 'action.hover', mb: 2, '& .MuiLinearProgress-bar': { borderRadius: 99, bgcolor: statusColor(t, gate.status) } })} />

          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 1, mb: 2 }}>
            {[
              { label: 'Findings', value: String(findingsCount), tone: findingsCount ? 'warning.main' : 'success.main' },
              { label: 'Patch review', value: proposedPatches ? `${proposedPatches} ready` : 'none', tone: proposedPatches ? 'primary.main' : 'text.disabled' },
              { label: 'Scanner', value: scannerState, tone: scannerState === 'blocked' ? 'error.main' : scannerState === 'running' ? 'primary.main' : 'success.main' },
            ].map((m) => (
              <Box key={m.label} sx={{ p: 1.15, borderRadius: 1.5, bgcolor: 'action.hover', minWidth: 0 }}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s58, textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                <Typography sx={{ fontSize: text.s82, fontWeight: 700, color: m.tone }} noWrap>{m.value}</Typography>
              </Box>
            ))}
          </Box>

          {gate.blocking ? (
            <Box sx={{ p: 1.5, borderRadius: 2, bgcolor: 'action.hover', mb: 2 }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.disabled', mb: 0.5 })}>BLOCKING</Typography>
              <Typography sx={{ fontSize: text.s90 }}>{gate.blocking}</Typography>
            </Box>
          ) : (
            <Typography sx={{ color: 'success.main', mb: 2 }}>● Closed end-to-end</Typography>
          )}

          {/* H3 — ProfitGuard co-located with the gate. Law 2: no expensive
              reasoning runs without expected ROI. When the verdict is block/hold
              this names the *economic* reason this gate can't advance, alongside
              the gate's own blocking line above. */}
          {pg.verdict !== 'allow' && (
            <Box sx={(t) => ({ p: 1.5, borderRadius: 2, mb: 2, border: 1, borderColor: `${t.palette[pg.tone].main}55`, bgcolor: `${t.palette[pg.tone].main}14` })}>
              <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.5 }}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.disabled' })}>
                  {pg.verdict === 'block' ? 'BLOCKED BY BUDGET' : 'HELD FOR ROI'}
                </Typography>
                <Chip
                  size="small"
                  label={`ProfitGuard · ${pg.verdict}`}
                  sx={(t) => ({ height: 18, fontSize: text.s62, fontFamily: t.brand.font.mono, bgcolor: `${t.palette[pg.tone].main}22`, color: t.palette[pg.tone].main, fontWeight: 600 })}
                />
              </Stack>
              <Typography sx={(t) => ({ fontSize: text.s86, color: t.palette[pg.tone].main })}>{pg.reason}</Typography>
              {!economicsLive && (
                <Typography sx={{ fontSize: text.s70, color: 'text.disabled', mt: 0.5 }}>
                  Connect the orchestrator for live ProfitGuard verdicts.
                </Typography>
              )}
            </Box>
          )}

          <Box sx={(t) => ({ p: 1.5, borderRadius: 2, mb: 2, border: 1, borderColor: 'divider', bgcolor: `${t.palette.background.default}aa` })}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Next action</Typography>
            <Typography sx={{ fontSize: text.s86, color: 'text.secondary', mb: 1.25 }}>{nextAction}</Typography>
            <Stack direction="row" spacing={1}>
              {gate.blocking && (
                <Button size="small" variant="contained" disabled={busy} onClick={() => void dispatch(`the ${gate.name} gate blocker`)}>
                  Ask agent to fix
                </Button>
              )}
              <Button size="small" variant={gate.blocking ? 'outlined' : 'contained'} color="inherit" disabled={busy} onClick={() => void repairGate(gate.id, gate.name)}>
                Re-run scanner
              </Button>
            </Stack>
          </Box>

          <Box sx={{ flex: 1, overflowY: 'auto' }}>
            {gate.findings.length > 0 && (
              <>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Findings</Typography>
                <Stack spacing={1} sx={{ mb: 3 }}>
                  {gate.findings.map((f) => (
                    <Stack key={f.id} direction="row" spacing={1} alignItems="flex-start">
                      <Box component="span" sx={{ mt: 0.25, color: f.severity === 'danger' ? 'error.main' : f.severity === 'warning' ? 'warning.main' : 'text.disabled' }}>●</Box>
                      <Typography sx={{ fontSize: text.s88, color: 'text.secondary' }}>{f.text}</Typography>
                    </Stack>
                  ))}
                </Stack>
              </>
            )}

            {gate.findings.length === 0 && (
              <Box sx={{ p: 1.5, borderRadius: 2, bgcolor: 'action.hover', mb: 2 }}>
                <Typography sx={{ fontSize: text.s86, color: 'success.main' }}>No open findings on this gate.</Typography>
              </Box>
            )}

            {gate.patches.length > 0 && (
              <>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Patches</Typography>
                {/* H4 — reviewable diff: each patch expands its hunks inline
                    before the operator applies it through the real applyPatch
                    mutation. */}
                <Stack spacing={1.25}>
                  {gate.patches.map((p) => (
                    <PatchDiff key={p.id} patch={p} busy={busy} onApply={() => void applyPatch(p.id, p.title)} />
                  ))}
                </Stack>
              </>
            )}

            {gate.patches.length === 0 && (
              <Box sx={{ p: 1.5, borderRadius: 2, border: 1, borderColor: 'divider' }}>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Patch review</Typography>
                <Typography sx={{ fontSize: text.s86, color: 'text.secondary', mb: gate.blocking ? 1 : 0 }}>
                  No reviewable patch has been proposed for this gate yet.
                </Typography>
                {gate.blocking && (
                  <Button size="small" variant="outlined" color="inherit" onClick={() => void dispatch(`a reviewable patch for the ${gate.name} gate`)}>
                    Generate patch
                  </Button>
                )}
              </Box>
            )}
          </Box>

          <Divider sx={{ my: 2 }} />
          <Stack direction="row" spacing={1.5}>
            <Button fullWidth variant="contained" disabled={busy} onClick={() => { void repairGate(gate.id, gate.name); close(); }}>Re-run this gate</Button>
          </Stack>
        </Box>
      )}
    </Drawer>
  );
}
