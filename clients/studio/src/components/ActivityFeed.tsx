import { useEffect, useState } from 'react';
import { Box, Chip, Stack, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { useEventStream } from '@ironflyer/data';
import { formatRelativeTime } from '@ironflyer/core';
import type { ActivityEvent } from '../studioData';

const SIM_LINES: Pick<ActivityEvent, 'kind' | 'text'>[] = [
  { kind: 'ledger', text: 'Ledger: debited $0.12 for premium model call' },
  { kind: 'profitguard', text: 'ProfitGuard: allow — expected margin 61%' },
  { kind: 'patch', text: 'Coder applied patch: add owner check to /orders' },
  { kind: 'gate', text: 'Data gate → running: rebuilding indexes' },
];

function kindColor(t: Theme, kind: ActivityEvent['kind']): string {
  switch (kind) {
    case 'gate': return t.brand.accent.secondary;
    case 'patch': return t.palette.primary.main;
    case 'profitguard': return t.palette.warning.main;
    case 'deploy': return t.palette.success.main;
    default: return t.palette.text.disabled;
  }
}

// Live orchestration feed — SSE when connected, gently simulated otherwise so
// the operator always sees the system working.
export function ActivityFeed({ projectId, seed }: { projectId: string; seed: ActivityEvent[] }) {
  const theme = useTheme();
  const { events, isLive } = useEventStream<ActivityEvent>({ path: `/executions/${projectId}/feed`, fallback: seed });
  const [extra, setExtra] = useState<ActivityEvent[]>([]);

  useEffect(() => {
    if (isLive) return;
    let i = 0;
    const id = setInterval(() => {
      const line = SIM_LINES[i % SIM_LINES.length]!;
      setExtra((prev) => [{ id: `sim-${Date.now()}`, ts: Date.now(), ...line }, ...prev].slice(0, 8));
      i += 1;
    }, 6000);
    return () => clearInterval(id);
  }, [isLive]);

  const all = [...extra, ...events].slice(0, 14);

  return (
    <Box>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5 }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Activity</Typography>
        <Chip size="small" label={isLive ? 'live' : 'simulated'} sx={(t) => ({ height: 18, fontSize: '0.62rem', fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })} />
      </Stack>
      <Stack spacing={0.5}>
        {all.map((e) => (
          <Stack key={e.id} direction="row" alignItems="center" spacing={1.5} sx={{ py: 0.75, borderBottom: 1, borderColor: 'divider' }}>
            <Box sx={{ width: 7, height: 7, borderRadius: 99, flexShrink: 0, bgcolor: kindColor(theme, e.kind) }} />
            <Typography sx={{ fontSize: '0.84rem', color: 'text.secondary', flex: 1, minWidth: 0 }} noWrap>{e.text}</Typography>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled', flexShrink: 0 })}>{formatRelativeTime(e.ts)}</Typography>
          </Stack>
        ))}
      </Stack>
    </Box>
  );
}
