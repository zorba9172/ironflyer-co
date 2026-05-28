import { Box, Card, Chip, LinearProgress, Stack, Typography } from '@mui/material';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { formatUSD } from '@ironflyer/core';
import { statusLabel, agentForGate, type Gate, type GateStatus, type StudioProject } from '../studioData';
import { statusColor } from './statusColor';
import { AgentsRail } from './AgentsRail';
import { ActivityFeed } from './ActivityFeed';
import { useStudio } from '../store';
import { useLiveProjectId } from '../hooks/useLiveProjectId';

const pgLabel: Record<StudioProject['profitGuard']['verdict'], string> = {
  allow: 'ProfitGuard: allow',
  hold: 'ProfitGuard: hold',
  block: 'ProfitGuard: block',
};

interface GateVerdict {
  gate: string;
  status: string;
  notes?: string | null;
  issues: { severity: string; message: string; path?: string | null; line?: number | null }[];
}

const titleCase = (s: string) => s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

function mapStatus(s: string): GateStatus {
  switch (s.toLowerCase()) {
    case 'pass': case 'passed': return 'closed';
    case 'running': return 'running';
    case 'warn': return 'open';
    case 'blocked': case 'fail': return 'blocked';
    default: return 'unstarted';
  }
}

// Map a live GateVerdict to the studio Gate shape.
function mapGate(v: GateVerdict, i: number): Gate {
  const status = mapStatus(v.status);
  const err = v.issues.find((x) => x.severity === 'error');
  return {
    id: v.gate,
    no: String(i + 1).padStart(2, '0'),
    name: titleCase(v.gate),
    status,
    blocking: status === 'closed' ? '' : err?.message ?? v.issues[0]?.message ?? (v.notes || (status === 'blocked' ? 'blocked' : 'pending')),
    level: status === 'closed' ? 1 : status === 'running' ? 0.5 : status === 'open' ? 0.6 : status === 'blocked' ? 0.25 : 0,
    costShare: 0,
    findings: v.issues.map((x, j) => ({ id: `${v.gate}-${j}`, severity: x.severity === 'error' ? 'danger' : x.severity === 'warning' ? 'warning' : 'info', text: x.message })),
    patches: [],
  };
}

function GateCard({ g }: { g: Gate }) {
  const selectGate = useStudio((s) => s.selectGate);
  return (
    <Card
      onClick={() => selectGate(g.id)}
      sx={{ p: 2.5, display: 'flex', flexDirection: 'column', gap: 1.25, cursor: 'pointer', transition: (t) => `border-color ${t.brand.motion.fast}`, '&:hover': { borderColor: 'text.disabled' } }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Stack direction="row" alignItems="center" spacing={1.25}>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.78rem', color: 'text.disabled' })}>{g.no}</Typography>
          <Typography variant="h6" sx={{ fontSize: '1.05rem' }}>{g.name}</Typography>
        </Stack>
        <Chip size="small" label={statusLabel[g.status]} sx={(t) => ({ bgcolor: `${statusColor(t, g.status)}22`, color: statusColor(t, g.status), fontWeight: 600, fontSize: '0.7rem' })} />
      </Stack>

      <LinearProgress variant="determinate" value={Math.round(g.level * 100)} sx={(t) => ({ height: 5, borderRadius: 99, bgcolor: 'action.hover', '& .MuiLinearProgress-bar': { borderRadius: 99, bgcolor: statusColor(t, g.status) } })} />

      {g.blocking ? (
        <Typography sx={{ fontSize: '0.82rem', color: 'text.secondary' }}>
          <Box component="span" sx={{ color: g.status === 'blocked' ? 'error.main' : 'warning.main' }}>● </Box>{g.blocking}
        </Typography>
      ) : (
        <Typography sx={{ fontSize: '0.82rem', color: 'success.main' }}>● Closed end-to-end</Typography>
      )}

      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ pt: 0.5 }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.secondary' })}>{agentForGate(g.id)?.name ?? 'Unassigned'}</Typography>
        <Box sx={{ width: 3, height: 3, borderRadius: 99, bgcolor: 'text.disabled' }} />
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.disabled' })}>{g.findings.length}f · {g.patches.length}p</Typography>
        <Typography sx={{ ml: 'auto', fontSize: '0.72rem', color: 'primary.main' }}>Open</Typography>
      </Stack>
    </Card>
  );
}

function Meter({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <Card sx={{ p: 2.5, flex: 1 }}>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>{label}</Typography>
      <Typography variant="h4" sx={{ fontSize: '1.8rem', mt: 0.5 }}>{value}</Typography>
      {sub && <Typography sx={{ fontSize: '0.78rem', color: 'text.secondary' }}>{sub}</Typography>}
    </Card>
  );
}

export function DashboardPane({ fallback }: { projectId: string; fallback: StudioProject }) {
  const liveProjectId = useLiveProjectId();
  const { data: liveGates, isLive } = useGraphQLQuery<Gate[], { gates: GateVerdict[] }>({
    key: ['gates', liveProjectId ?? 'none'],
    operationName: 'Gates',
    query: operations.GATES,
    variables: { projectId: liveProjectId },
    fallbackData: [],
    enabled: !!liveProjectId,
    map: (raw) => raw.gates.map(mapGate),
  });

  // Live gates when connected; otherwise the sample project. Meters/activity
  // stay sample until the snapshot mapping lands.
  const gates = isLive && liveGates.length > 0 ? liveGates : fallback.gates;
  const completion = gates.length ? gates.filter((g) => !g.blocking).length / gates.length : 0;
  const p = { ...fallback, gates, completion };

  const open = p.gates.filter((g) => g.blocking).length;
  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 0.5 }}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>Finisher</Typography>
            <Chip
              size="small"
              label={isLive ? 'live' : 'sample data'}
              sx={(t) => ({ height: 20, fontSize: '0.64rem', fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
            />
            <Chip
              size="small"
              label={pgLabel[p.profitGuard.verdict]}
              sx={(t) => {
                const c = p.profitGuard.verdict === 'block' ? t.palette.error.main : p.profitGuard.verdict === 'hold' ? t.palette.warning.main : t.palette.success.main;
                return { height: 20, fontSize: '0.64rem', fontFamily: t.brand.font.mono, bgcolor: `${c}22`, color: c };
              }}
            />
          </Stack>
          <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{open} of {p.gates.length} gates open · {Math.round(p.completion * 100)}% to shippable</Typography>
        </Stack>
        <LinearProgress variant="determinate" value={Math.round(p.completion * 100)} sx={{ height: 6, borderRadius: 99, bgcolor: 'action.hover', mb: 3, '& .MuiLinearProgress-bar': { borderRadius: 99, backgroundImage: (t) => t.brand.gradient.signature } }} />

        <AgentsRail gates={p.gates} />

        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ mb: 3 }}>
          <Meter label="Wallet" value={formatUSD(p.meters.walletUsed)} sub={`of ${formatUSD(p.meters.walletBudget)} budget`} />
          <Meter label="Margin (30d)" value={`${p.meters.marginPct}%`} sub="revenue − provider cost" />
          <Meter label="Throughput" value={`${p.meters.throughput}/min`} sub="agent runs" />
        </Stack>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 1.5 }}>
          {p.gates.map((g) => <GateCard key={g.id} g={g} />)}
        </Box>

        <Box sx={{ mt: 4 }}>
          <ActivityFeed projectId={p.id} seed={p.activity} />
        </Box>
      </Box>
    </Box>
  );
}
