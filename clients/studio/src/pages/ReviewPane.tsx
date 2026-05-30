import { useMemo, useState } from 'react';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useRequest, operations } from '@ironflyer/data';
import { PatchDiff } from '../components/PatchDiff';
import { useLiveGates } from '../hooks/useLiveGates';
import { agentForGate } from '../studioData';
import type { Finding, Patch } from '../studioData';
import { Icon } from '../icons';
import { StudioChart, donutOption, barOption, type EChartsOption } from '../components/charts';
import { GlassPanel, SectionHeader, StatCard } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

interface PatchRow extends Patch { gateId: string; gateName: string }
interface FindingRow extends Finding { gateId: string; gateName: string }

const sevColor = (t: Theme, s: Finding['severity']) =>
  s === 'danger' ? t.palette.error.main : s === 'warning' ? t.palette.warning.main : t.palette.info.main;

function FindingRow({ f }: { f: FindingRow }) {
  const t = useTheme();
  const color = sevColor(t, f.severity);
  const label = f.severity === 'danger' ? 'Blocking' : f.severity === 'warning' ? 'Warning' : 'Info';
  return (
    <Stack
      direction="row"
      alignItems="flex-start"
      spacing={1.5}
      sx={{
        px: 2,
        py: 1.5,
        borderTop: '1px solid',
        borderColor: 'divider',
        '&:hover': { bgcolor: 'action.hover' },
      }}
    >
      <Box sx={{ width: 7, height: 7, mt: 0.9, borderRadius: 99, bgcolor: color, flexShrink: 0 }} />
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography sx={{ fontSize: text.s86, fontWeight: 600 }}>{f.text}</Typography>
        <Typography sx={(theme) => ({ fontFamily: theme.brand.font.mono, fontSize: text.s72, color: 'text.disabled', mt: 0.4 })}>
          {f.gateName}
        </Typography>
      </Box>
      <Chip
        size="small"
        label={label}
        sx={{ height: 20, fontSize: text.s62, color, bgcolor: `${color}18`, fontWeight: 700 }}
      />
    </Stack>
  );
}

// PR-style review surface: every reviewable patch the finisher proposed, across
// all gates, in one place. Mirrors live gate verdicts when a project is open;
// the session sample otherwise.
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
      { value: proposed.length, name: 'To review', color: t.palette.warning.main },
      { value: applied.length, name: 'Applied', color: t.palette.success.main },
    ].filter((d) => d.value > 0);
    return donutOption(t, {
      data,
      centerLabel: proposed.length > 0 ? `${proposed.length}\nto review` : 'all\nreviewed',
      centerColor: proposed.length > 0 ? t.palette.warning.main : t.palette.success.main,
      emptyLabel: 'No patches',
      radius: ['60%', '82%'],
    });
  }, [proposed.length, applied.length, t]);

  // Multi-color bar: how many reviewable patches each gate produced — bound to
  // the real gate verdicts so the operator sees where review effort concentrates.
  const byGate = useMemo<EChartsOption>(() => {
    const counts = new Map<string, number>();
    for (const p of patches) counts.set(p.gateName, (counts.get(p.gateName) ?? 0) + 1);
    const entries = [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 7);
    return barOption(t, {
      categories: entries.map(([name]) => name),
      series: [{ name: 'Patches', data: entries.map(([, n]) => n) }],
      multicolor: true,
    });
  }, [patches, t]);

  const ordered = [...proposed, ...applied];

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <SectionHeader
          eyebrow="Code review"
          title={
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <span>Review</span>
              <Chip
                size="small"
                label={isLive ? 'live' : 'sample'}
                sx={(th) => ({
                  height: 20,
                  fontSize: text.s64,
                  fontFamily: th.brand.font.mono,
                  bgcolor: isLive ? `${th.palette.success.main}22` : 'action.hover',
                  color: isLive ? 'success.main' : 'text.disabled',
                })}
              />
            </Stack>
          }
          subtitle={
            proposed.length > 0
              ? `${proposed.length} patch${proposed.length > 1 ? 'es' : ''} awaiting review`
              : 'No patches awaiting review'
          }
          actions={
            proposed.length > 0 ? (
              <Button
                variant="contained"
                color="primary"
                size="small"
                startIcon={<Icon name="sparkles" size={15} />}
                sx={{ borderRadius: 999 }}
              >
                Apply all {proposed.length} patches
              </Button>
            ) : undefined
          }
        />

        {/* Visual-first lead: only mount the donut + per-gate bar when there are
            actually patches — an empty project would otherwise render two giant
            faint boxes. When empty, the KPI rail below carries the state. */}
        {patches.length > 0 && (
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '220px 1fr' }, gap: 1.5, mb: 1.5, alignItems: 'stretch' }}>
            <GlassPanel pad={2} accent={proposed.length > 0 ? t.studio.neon.warning : t.studio.neon.success}>
              <Typography
                sx={(th) => ({
                  fontFamily: th.brand.font.mono,
                  fontSize: text.s66,
                  textTransform: 'uppercase',
                  color: 'text.disabled',
                  mb: 0.5,
                })}
              >
                Patch states
              </Typography>
              <StudioChart option={donut} height={200} />
            </GlassPanel>

            <GlassPanel pad={2} accent={t.studio.neon.violet}>
              <Typography
                sx={(th) => ({
                  fontFamily: th.brand.font.mono,
                  fontSize: text.s66,
                  textTransform: 'uppercase',
                  color: 'text.disabled',
                  mb: 0.5,
                })}
              >
                Review queue by gate
              </Typography>
              <StudioChart option={byGate} height={200} />
            </GlassPanel>
          </Box>
        )}

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(4, 1fr)' }, gap: 1.5, mb: 3 }}>
          <StatCard
            label="To review"
            value={String(proposed.length)}
            hint="proposed patches"
            accent={t.palette.warning.main}
            icon={<Icon name="pullRequest" size={16} />}
          />
          <StatCard
            label="Applied"
            value={String(applied.length)}
            hint="through the gates"
            accent={t.palette.success.main}
            icon={<Icon name="checkAll" size={16} />}
          />
          <StatCard
            label="Blocking findings"
            value={String(dangers)}
            hint="must close to ship"
            accent={t.palette.error.main}
            icon={<Icon name="alert" size={16} />}
          />
          <StatCard
            label="Warnings"
            value={String(warnings)}
            hint="review recommended"
            accent={t.palette.warning.main}
            icon={<Icon name="info" size={16} />}
          />
        </Box>

        <Typography
          sx={(th) => ({
            fontFamily: th.brand.font.mono,
            fontSize: text.s70,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: 'text.disabled',
            mb: 1.5,
          })}
        >
          Patches
        </Typography>

        {ordered.length === 0 ? (
          <GlassPanel pad={2.5}>
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <Box
                sx={(th) => ({
                  width: 34,
                  height: 34,
                  borderRadius: `${th.studio.radius.sm}px`,
                  display: 'grid',
                  placeItems: 'center',
                  color: 'text.disabled',
                  bgcolor: 'action.hover',
                  flexShrink: 0,
                })}
              >
                <Icon name="pullRequest" size={17} />
              </Box>
              <Box sx={{ minWidth: 0 }}>
                <Typography sx={{ fontWeight: 700, fontSize: text.s88 }}>No patches to review yet</Typography>
                <Typography sx={{ color: 'text.secondary', fontSize: text.s80 }}>
                  Run the finisher — reviewable changes from every gate land here, ready to inspect and apply.
                </Typography>
              </Box>
            </Stack>
          </GlassPanel>
        ) : (
          <Stack spacing={1.25}>
            {ordered.map((p) => {
              const agent = agentForGate(p.gateId);
              return (
                <Box key={`${p.gateId}-${p.id}`}>
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.5 }}>
                    <Chip
                      size="small"
                      label={p.gateName}
                      sx={(th) => ({
                        height: 18,
                        fontFamily: th.brand.font.mono,
                        fontSize: text.s62,
                        bgcolor: 'action.hover',
                        color: 'text.secondary',
                      })}
                    />
                    {agent && (
                      <Typography sx={{ fontSize: text.s70, color: 'text.disabled' }}>{agent.name}</Typography>
                    )}
                    {p.state === 'proposed' && (
                      <Chip
                        size="small"
                        label="Awaiting review"
                        sx={(th) => ({
                          height: 18,
                          fontSize: text.s62,
                          color: th.palette.warning.main,
                          bgcolor: `${th.palette.warning.main}18`,
                          fontWeight: 700,
                        })}
                      />
                    )}
                    {p.state === 'applied' && (
                      <Chip
                        size="small"
                        label="Applied"
                        sx={(th) => ({
                          height: 18,
                          fontSize: text.s62,
                          color: th.palette.success.main,
                          bgcolor: `${th.palette.success.main}18`,
                          fontWeight: 700,
                        })}
                      />
                    )}
                  </Stack>
                  <PatchDiff patch={p} busy={busy === p.id} onApply={() => void applyPatch(p)} />
                </Box>
              );
            })}
          </Stack>
        )}

        {findings.length > 0 && (
          <>
            <Typography
              sx={(th) => ({
                fontFamily: th.brand.font.mono,
                fontSize: text.s70,
                letterSpacing: '0.1em',
                textTransform: 'uppercase',
                color: 'text.disabled',
                mt: 3,
                mb: 1.5,
              })}
            >
              Findings
            </Typography>
            <GlassPanel pad={0} sx={{ overflow: 'hidden' }}>
              {findings.map((f) => <FindingRow key={`${f.gateId}-${f.id}`} f={f} />)}
            </GlassPanel>
          </>
        )}
      </Box>
    </Box>
  );
}
