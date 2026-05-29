import { Box, Card, Stack, Typography } from '@mui/material';
import type { Gate, GateStatus } from '../studioData';
import { statusColor } from './statusColor';

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

function CheckGlyph() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

export function DefinitionOfDone({ gates }: { gates: Gate[] }) {
  const items = DOD_ITEMS.map((it) => ({ ...it, ...resolve(gates, it.gateIds) }));
  const done = items.filter((i) => i.status === 'closed').length;
  const blocked = items.find((i) => i.status === 'blocked' || i.status === 'open');

  return (
    <Card sx={{ p: 2.5, mt: 1.5 }}>
      <Stack direction="row" alignItems="baseline" justifyContent="space-between" sx={{ mb: 1.5 }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>
          Definition of Done
        </Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.8rem', color: done === items.length ? 'success.main' : 'text.secondary' })}>
          {done}/{items.length} {done === items.length ? '· shippable' : blocked ? `· blocked on ${blocked.label.toLowerCase()}` : '· in progress'}
        </Typography>
      </Stack>
      <Stack spacing={1}>
        {items.map((it) => {
          const done = it.status === 'closed';
          return (
            <Stack key={it.label} direction="row" alignItems="center" spacing={1.25}>
              <Box
                sx={(t) => ({
                  width: 20, height: 20, borderRadius: 99, flexShrink: 0,
                  display: 'grid', placeItems: 'center',
                  color: done ? t.palette.success.contrastText : statusColor(t, it.status),
                  bgcolor: done ? 'success.main' : `${statusColor(t, it.status)}22`,
                })}
              >
                {done ? <CheckGlyph /> : <Box sx={{ width: 6, height: 6, borderRadius: 99, bgcolor: 'currentColor' }} />}
              </Box>
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Typography sx={{ fontSize: '0.86rem', fontWeight: 600 }} noWrap>{it.label}</Typography>
                <Typography sx={{ fontSize: '0.74rem', color: 'text.secondary' }} noWrap>{it.sub}</Typography>
              </Box>
              <Typography
                sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: done ? 'success.main' : statusColor(t, it.status), textTransform: 'uppercase', flexShrink: 0 })}
              >
                {done ? 'done' : it.reason}
              </Typography>
            </Stack>
          );
        })}
      </Stack>
    </Card>
  );
}
