import { useMemo, useState } from 'react';
import { Box, Card, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { Chart, type EChartsOption, confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useRequest, operations } from '@ironflyer/data';
import { PatchDiff } from '../components/PatchDiff';
import { useLiveGates } from '../hooks/useLiveGates';
import { agentForGate } from '../studioData';
import type { Finding, Patch } from '../studioData';
import { text } from '@ironflyer/design-tokens/brand';

interface PatchRow extends Patch { gateId: string; gateName: string }
interface FindingRow extends Finding { gateId: string; gateName: string }

const sevColor = (t: Theme, s: Finding['severity']) =>
  s === 'danger' ? t.palette.error.main : s === 'warning' ? t.palette.warning.main : t.brand.accent.secondary;

// PR-style review surface: every reviewable patch the finisher proposed, across
// all gates, in one place — review the diff, then apply through the real
// patch lifecycle (which re-runs the gate server-side). Findings the gates
// surfaced sit alongside as the "why" behind the work. Mirrors the live gate
// verdicts when a project is open; the session sample otherwise.
export function ReviewPane() {
  const t = useTheme();
  const { gates, isLive } = useLiveGates();
  const request = useRequest();
  const [busy, setBusy] = useState<string | null>(null);

  const patches = useMemo<PatchRow[]>(
    () => gates.flatMap((g) => g.patches.map((p) => ({ ...p, gateId: g.id, gateName: g.name }))),
    [gates],
  );
  const findings = useMemo<FindingRow[]>(
    () => gates.flatMap((g) => g.findings.map((f) => ({ ...f, gateId: g.id, gateName: g.name }))),
    [gates],
  );

  const proposed = patches.filter((p) => p.state === 'proposed');
  const applied = patches.filter((p) => p.state === 'applied');
  const dangers = findings.filter((f) => f.severity === 'danger').length;
  const warnings = findings.filter((f) => f.severity === 'warning').length;

  // Apply a reviewed patch through the real applyPatch mutation. Offline → note.
  const applyPatch = async (patch: PatchRow) => {
    const ok = await confirmAction({ title: 'Apply reviewed patch?', text: patch.title, confirmText: 'Apply' });
    if (!ok) return;
    if (!request) { toast('Connect the orchestrator to apply patches.', 'info'); return; }
    setBusy(patch.id);
    try {
      await request('ApplyPatch', operations.APPLY_PATCH, { patchId: patch.id });
      toast('Patch applied — re-running the gate.', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not apply patch.', 'error');
    } finally {
      setBusy(null);
    }
  };

  const donut = useMemo<EChartsOption>(() => {
    const data = [
      { value: proposed.length, name: 'To review', itemStyle: { color: t.palette.warning.main } },
      { value: applied.length, name: 'Applied', itemStyle: { color: t.palette.success.main } },
    ].filter((d) => d.value > 0);
    return {
      tooltip: { trigger: 'item' },
      legend: { bottom: 0, textStyle: { color: t.palette.text.secondary, fontSize: 11 } },
      series: [{
        type: 'pie', radius: ['58%', '80%'], avoidLabelOverlap: true,
        itemStyle: { borderColor: t.palette.background.paper, borderWidth: 2 },
        label: { show: true, position: 'center', formatter: proposed.length > 0 ? `${proposed.length}\nto review` : 'all\nreviewed', color: proposed.length > 0 ? t.palette.warning.main : t.palette.success.main, fontSize: 22, lineHeight: 22 },
        data: data.length ? data : [{ value: 1, name: 'No patches', itemStyle: { color: t.palette.action.hover } }],
      }],
    };
  }, [proposed.length, applied.length, t]);

  const metrics = [
    { label: 'To review', value: String(proposed.length), sub: 'proposed patches' },
    { label: 'Applied', value: String(applied.length), sub: 'through the gates' },
    { label: 'Blocking findings', value: String(dangers), sub: 'must close to ship' },
    { label: 'Warnings', value: String(warnings), sub: 'review recommended' },
  ];

  const ordered = [...proposed, ...applied];

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
          <Typography variant="h4" sx={{ fontSize: text.s160 }}>Review</Typography>
          <Chip size="small" label={isLive ? 'live' : 'sample'} sx={(th) => ({ height: 20, fontSize: text.s64, fontFamily: th.brand.font.mono, bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
          <Typography sx={{ color: 'text.secondary', fontSize: text.s90 }}>{proposed.length > 0 ? `${proposed.length} patch${proposed.length > 1 ? 'es' : ''} awaiting review` : 'No patches awaiting review'}</Typography>
        </Stack>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '300px 1fr' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Patch states</Typography>
            <Chart option={donut} height={200} />
          </Card>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr' }, gap: 1.5 }}>
            {metrics.map((m) => (
              <Card key={m.label} sx={{ p: 2.5, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
                <Typography variant="h4" sx={{ fontSize: text.s180, mt: 0.5 }}>{m.value}</Typography>
                <Typography sx={{ fontSize: text.s76, color: 'text.secondary' }}>{m.sub}</Typography>
              </Card>
            ))}
          </Box>
        </Box>

        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Patches</Typography>
        {ordered.length === 0 ? (
          <Card sx={{ p: 3, textAlign: 'center' }}>
            <Typography sx={{ color: 'text.disabled', fontSize: text.s90 }}>No patches yet — run the finisher to generate reviewable changes.</Typography>
          </Card>
        ) : (
          <Stack spacing={1.25}>
            {ordered.map((p) => {
              const agent = agentForGate(p.gateId);
              return (
                <Box key={`${p.gateId}-${p.id}`}>
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.5 }}>
                    <Chip size="small" label={p.gateName} sx={(th) => ({ height: 18, fontFamily: th.brand.font.mono, fontSize: text.s62, bgcolor: 'action.hover', color: 'text.secondary' })} />
                    {agent && <Typography sx={{ fontSize: text.s70, color: 'text.disabled' }}>{agent.name}</Typography>}
                  </Stack>
                  <PatchDiff patch={p} busy={busy === p.id} onApply={() => void applyPatch(p)} />
                </Box>
              );
            })}
          </Stack>
        )}

        {findings.length > 0 && (
          <>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mt: 3, mb: 1.5 })}>Findings</Typography>
            <Stack spacing={0.75}>
              {findings.map((f) => (
                <Card key={`${f.gateId}-${f.id}`} sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1.25 }}>
                  <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: sevColor(t, f.severity), flexShrink: 0 }} />
                  <Typography sx={{ fontSize: text.s86, flex: 1, minWidth: 0 }}>{f.text}</Typography>
                  <Chip size="small" label={f.gateName} sx={(th) => ({ height: 18, fontFamily: th.brand.font.mono, fontSize: text.s62, bgcolor: 'action.hover', color: 'text.secondary' })} />
                </Card>
              ))}
            </Stack>
          </>
        )}
      </Box>
    </Box>
  );
}
