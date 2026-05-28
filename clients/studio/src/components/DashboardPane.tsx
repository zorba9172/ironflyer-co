import { useState } from 'react';
import { Box, Button, Card, Chip, Collapse, LinearProgress, Stack, Typography } from '@mui/material';
import { useGraphQLQuery } from '@ironflyer/data';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { formatUSD } from '@ironflyer/core';
import { statusLabel, agentForGate, type Gate, type StudioProject } from '../studioData';
import { statusColor } from './statusColor';
import { AgentsRail } from './AgentsRail';
import { ActivityFeed } from './ActivityFeed';

const pgLabel: Record<StudioProject['profitGuard']['verdict'], string> = {
  allow: 'ProfitGuard: allow',
  hold: 'ProfitGuard: hold',
  block: 'ProfitGuard: block',
};

// Reads the live finisher snapshot from the orchestrator when a GraphQL
// endpoint is configured; otherwise renders the passed-in project (offline).
// NOTE: field names below must be reconciled with the orchestrator schema.
const FINISHER_QUERY = /* GraphQL */ `
  query FinisherSnapshot($projectId: ID!) {
    project(id: $projectId) {
      id name completion
      meters { walletUsed walletBudget marginPct throughput }
      gates { id no name status blocking level costShare
        findings { id severity text }
        patches { id title state lines }
      }
    }
  }
`;

function GateCard({ g }: { g: Gate }) {
  const [open, setOpen] = useState(false);
  const hasDetail = g.findings.length > 0 || g.patches.length > 0;
  return (
    <Card
      onClick={() => hasDetail && setOpen((o) => !o)}
      sx={{ p: 2.5, display: 'flex', flexDirection: 'column', gap: 1.25, cursor: hasDetail ? 'pointer' : 'default', transition: (t) => `border-color ${t.brand.motion.fast}`, '&:hover': hasDetail ? { borderColor: 'text.disabled' } : undefined }}
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
        {hasDetail && <Typography sx={{ ml: 'auto', fontSize: '0.72rem', color: 'primary.main' }}>{open ? 'Hide' : 'Details'}</Typography>}
      </Stack>

      <Collapse in={open} unmountOnExit>
        <Stack spacing={1} sx={{ pt: 1, mt: 0.5, borderTop: 1, borderColor: 'divider' }}>
          {g.findings.map((f) => (
            <Stack key={f.id} direction="row" spacing={1} alignItems="flex-start">
              <Box component="span" sx={{ color: f.severity === 'danger' ? 'error.main' : f.severity === 'warning' ? 'warning.main' : 'text.disabled', fontSize: '0.8rem', mt: '1px' }}>●</Box>
              <Typography sx={{ fontSize: '0.82rem', color: 'text.secondary' }}>{f.text}</Typography>
            </Stack>
          ))}
          {g.patches.map((p) => (
            <Stack key={p.id} direction="row" alignItems="center" spacing={1}>
              <Chip size="small" label={p.state} sx={{ height: 18, fontSize: '0.62rem', bgcolor: 'action.hover' }} />
              <Typography sx={{ fontSize: '0.82rem', flex: 1, minWidth: 0 }} noWrap>{p.title}</Typography>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled' })}>+{p.lines}</Typography>
              {p.state === 'proposed' && (
                <Button
                  size="small"
                  variant="outlined"
                  color="inherit"
                  onClick={async (e) => {
                    e.stopPropagation();
                    const ok = await confirmAction({ title: 'Apply patch?', text: p.title, confirmText: 'Apply' });
                    if (ok) toast('Patch applied — re-running the gate.', 'success');
                  }}
                  sx={{ py: 0, px: 1, minWidth: 0, fontSize: '0.7rem' }}
                >
                  Apply
                </Button>
              )}
            </Stack>
          ))}
        </Stack>
      </Collapse>
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

export function DashboardPane({ projectId, fallback }: { projectId: string; fallback: StudioProject }) {
  const { data: p, isLive } = useGraphQLQuery<StudioProject, { project: StudioProject }>({
    key: ['finisher', projectId],
    operationName: 'FinisherSnapshot',
    query: FINISHER_QUERY,
    variables: { projectId },
    fallbackData: fallback,
    map: (raw) => raw.project,
  });

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
