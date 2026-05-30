import { Box, LinearProgress, Stack, Typography } from '@mui/material';
import type { Gate, GateStatus } from '../studioData';
import { statusColor } from './statusColor';
import { GlassPanel } from './studio';
import { Icon } from '../icons';
import { text } from '@ironflyer/design-tokens/brand';

// The vision's "Definition of Done" made legible: each item is backed by a real
// finisher gate verdict, so "done" is never a vibe — it's a passing gate. An
// item with no matching gate yet reads as "not started" rather than faking green.
const DOD_ITEMS: { label: string; gateIds: string[]; sub: string }[] = [
  { label: 'Spec & acceptance criteria', gateIds: ['spec', 'verifier'], sub: 'stories proven against the build' },
  { label: 'Builds & compiles', gateIds: ['code'], sub: 'deps resolve · build succeeds' },
  { label: 'Tests pass', gateIds: ['test'], sub: 'suite runs in the sandbox' },
  { label: 'Security clean', gateIds: ['security'], sub: 'AppSec scan, no ship-stoppers' },
  { label: 'Code quality', gateIds: ['lint'], sub: 'lint / vet clean' },
  { label: 'Deployment path ready', gateIds: ['deploy'], sub: 'Dockerfile + build check' },
];

// Worst status wins: a DoD item is only "done" when every backing gate passes.
const SEVERITY: GateStatus[] = ['blocked', 'open', 'running', 'unstarted', 'closed'];

function resolve(gates: Gate[], ids: string[]): { status: GateStatus; reason: string } {
  const found = ids.map((id) => gates.find((g) => g.id === id)).filter((g): g is Gate => !!g);
  if (found.length === 0) return { status: 'unstarted', reason: 'not started' };
  found.sort((a, b) => SEVERITY.indexOf(a.status) - SEVERITY.indexOf(b.status));
  const worst = found[0]!;
  return { status: worst.status, reason: worst.status === 'closed' ? 'done' : worst.blocking || worst.status };
}

// H3 — ProfitGuard as a Definition-of-Done concern. "Done" is never just the
// finisher gates: a paid execution is only shippable if it clears the economic
// gate too (law 2). The verdict is derived live in PreviewWorkspace from the
// orchestrator's trajectory + wallet headroom and passed in here so the DoD
// names the budget/ROI block alongside the build blocks. Optional so callers
// that have no economic context (e.g. the sample map) render gates only.
export interface ProfitGuardItem {
  /** allow → done, hold → open (warning), block → blocked (error) */
  verdict: 'allow' | 'hold' | 'block';
  reason: string;
}

function pgStatus(v: ProfitGuardItem['verdict']): GateStatus {
  return v === 'allow' ? 'closed' : v === 'hold' ? 'open' : 'blocked';
}

export function DefinitionOfDone({ gates, profitGuard }: { gates: Gate[]; profitGuard?: ProfitGuardItem }) {
  const gateItems = DOD_ITEMS.map((it) => ({ ...it, ...resolve(gates, it.gateIds) }));
  const items = profitGuard
    ? [
        ...gateItems,
        {
          label: 'Within budget & ROI',
          sub: 'ProfitGuard clears the next paid step',
          gateIds: [] as string[],
          status: pgStatus(profitGuard.verdict),
          reason: profitGuard.verdict === 'allow' ? 'done' : profitGuard.reason,
        },
      ]
    : gateItems;
  const done = items.filter((i) => i.status === 'closed').length;
  const blocked = items.find((i) => i.status === 'blocked' || i.status === 'open');
  const progressPct = items.length > 0 ? Math.round((done / items.length) * 100) : 0;
  const allDone = done === items.length;

  return (
    <GlassPanel pad={2.5}>
      {/* Header row */}
      <Stack
        direction="row"
        alignItems="baseline"
        justifyContent="space-between"
        sx={{ mb: 1.5 }}
      >
        <Typography
          sx={(t) => ({
            fontFamily: t.brand.font.mono,
            fontSize: text.s68,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: 'text.disabled',
          })}
        >
          Definition of Done
        </Typography>
        <Typography
          sx={(t) => ({
            fontFamily: t.brand.font.mono,
            fontSize: text.s74,
            fontWeight: 700,
            color: allDone ? 'success.main' : blocked ? 'warning.main' : 'text.secondary',
          })}
        >
          {done}/{items.length}
        </Typography>
      </Stack>

      {/* Progress bar */}
      <Box sx={{ mb: 2 }}>
        <LinearProgress
          variant="determinate"
          value={progressPct}
          color={allDone ? 'success' : blocked?.status === 'blocked' ? 'error' : 'warning'}
          sx={{ height: 4, borderRadius: 99, bgcolor: 'action.hover' }}
        />
        <Typography
          sx={(t) => ({
            fontFamily: t.brand.font.mono,
            fontSize: text.s66,
            color: allDone ? 'success.main' : blocked ? 'text.secondary' : 'text.disabled',
            mt: 0.75,
          })}
        >
          {allDone
            ? '· shippable'
            : blocked
            ? `· blocked on ${blocked.label.toLowerCase()}`
            : '· in progress'}
        </Typography>
      </Box>

      {/* Gate items */}
      <Stack spacing={0.75}>
        {items.map((it) => {
          const isDone = it.status === 'closed';
          return (
            <Stack key={it.label} direction="row" alignItems="center" spacing={1.25}>
              {/* Status glyph */}
              <Box
                sx={(t) => ({
                  width: 20,
                  height: 20,
                  borderRadius: 99,
                  flexShrink: 0,
                  display: 'grid',
                  placeItems: 'center',
                  color: isDone ? t.palette.success.contrastText : statusColor(t, it.status),
                  bgcolor: isDone ? 'success.main' : `${statusColor(t, it.status)}22`,
                  transition: `background-color ${t.studio?.motion?.base ?? '300ms'}`,
                })}
              >
                {isDone ? (
                  <Icon name="check" size={12} strokeWidth={3} />
                ) : (
                  <Box sx={{ width: 5, height: 5, borderRadius: 99, bgcolor: 'currentColor' }} />
                )}
              </Box>

              {/* Label + sub */}
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Typography sx={{ fontSize: text.s84, fontWeight: 600 }} noWrap>
                  {it.label}
                </Typography>
                <Typography sx={{ fontSize: text.s72, color: 'text.secondary' }} noWrap>
                  {it.sub}
                </Typography>
              </Box>

              {/* Status badge */}
              <Typography
                sx={(t) => ({
                  fontFamily: t.brand.font.mono,
                  fontSize: text.s70,
                  color: isDone ? 'success.main' : statusColor(t, it.status),
                  textTransform: 'uppercase',
                  letterSpacing: '0.06em',
                  flexShrink: 0,
                })}
              >
                {isDone ? 'done' : it.reason}
              </Typography>
            </Stack>
          );
        })}
      </Stack>
    </GlassPanel>
  );
}
