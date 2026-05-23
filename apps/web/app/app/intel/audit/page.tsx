'use client';

// Audit log browser with hash-chain verification. Surfaces /api/audit —
// the production-trust moat: every consequential action the orchestrator
// took on behalf of a user, content-addressed so post-hoc tampering is
// detectable. The top banner asks the server to re-walk the chain.

import { useEffect, useMemo, useState } from 'react';
import {
  Box, Button, Chip, MenuItem, Select, Stack, TextField, Typography,
} from '@mui/material';
import { CheckCircle, ErrorOutline, Refresh } from '@mui/icons-material';
import {
  api, AuditEntry, AuditAction, AuditOutcome,
} from '../../../../lib/api';
import { tokens } from '../../../../lib/theme';
import { RequireAuth, useAuth } from '../../../auth-context';
import { AppShell, PageTitle, Surface } from '../../workspace-shell';
import { EmptyState, ErrorBox } from '../../../../components/dashboard';
import { VirtualList } from '../../../../components/performance/VirtualList';

const ACTIONS: { value: AuditAction; label: string }[] = [
  { value: 'patch.proposed',     label: 'Patch proposed' },
  { value: 'patch.applied',      label: 'Patch applied' },
  { value: 'patch.rolled_back',  label: 'Patch rolled back' },
  { value: 'gate.verdict',       label: 'Gate verdict' },
  { value: 'agent.dispatch',     label: 'Agent dispatch' },
  { value: 'secret.written',     label: 'Secret written' },
  { value: 'workspace.exec',     label: 'Workspace exec' },
  { value: 'deploy',             label: 'Deploy' },
  { value: 'memory.record',      label: 'Memory record' },
];

const OUTCOMES: { value: AuditOutcome; label: string }[] = [
  { value: 'success', label: 'Success' },
  { value: 'failure', label: 'Failure' },
  { value: 'blocked', label: 'Blocked' },
];

export default function AuditPage() {
  return (
    <RequireAuth>
      <AuditInner />
    </RequireAuth>
  );
}

function AuditInner() {
  const { user, logout } = useAuth();
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [count, setCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [verify, setVerify] = useState<{ intact: boolean; firstBadIndex: number } | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);

  const [action, setAction] = useState<string>('');
  const [outcome, setOutcome] = useState<string>('');
  const [since, setSince] = useState('');
  const [until, setUntil] = useState('');

  const refresh = useMemo(() => () => {
    setLoading(true);
    setError(null);
    Promise.all([
      api.listAudit({
        action: action || undefined,
        outcome: outcome || undefined,
        since: since ? new Date(since).toISOString() : undefined,
        until: until ? new Date(until).toISOString() : undefined,
        limit: 200,
      }),
      api.verifyAudit().catch(() => null),
    ])
      .then(([r, v]) => {
        setEntries(r.entries);
        setCount(r.count);
        if (v) setVerify(v);
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false));
  }, [action, outcome, since, until]);

  useEffect(() => { refresh(); }, [refresh]);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={[]} onLogout={logout}>
      <PageTitle
        eyebrow="Intelligence"
        title="Audit log"
        subtitle="Append-only, hash-chained log of every consequential action. Verifying the chain detects tampering between the orchestrator and any downstream store."
        action={(
          <Button variant="outlined" startIcon={<Refresh fontSize="small" />} onClick={refresh}>
            Refresh
          </Button>
        )}
      />

      {verify && (
        <Box sx={{
          mb: 2,
          p: 1.6,
          borderRadius: '8px',
          border: `1px solid ${verify.intact ? 'rgba(121,224,122,0.45)' : 'rgba(255,24,24,0.45)'}`,
          bgcolor: verify.intact ? 'rgba(121,224,122,0.12)' : 'rgba(255,24,24,0.10)',
          color: tokens.color.text.inverse,
          display: 'flex',
          alignItems: 'center',
          gap: 1.1,
        }}>
          {verify.intact ? (
            <CheckCircle sx={{ color: '#3f9c40' }} />
          ) : (
            <ErrorOutline sx={{ color: tokens.color.accent.danger }} />
          )}
          <Box>
            <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>
              {verify.intact
                ? `Hash chain intact (${count} entries shown)`
                : `Chain broken at #${verify.firstBadIndex}`}
            </Typography>
            <Typography variant="caption" sx={{ color: '#5b554b' }}>
              {verify.intact
                ? 'Each entry links to the previous content hash. Every replay matches the stored value.'
                : 'A stored entry no longer matches its content hash. Treat this as a critical incident.'}
            </Typography>
          </Box>
        </Box>
      )}

      <Surface sx={{ p: 1.6, mb: 2 }}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.2} alignItems={{ md: 'center' }} useFlexGap flexWrap="wrap">
          <Box sx={{ minWidth: 200 }}>
            <Select value={action} onChange={(e) => setAction(e.target.value as string)} size="small" displayEmpty fullWidth sx={selectSx}>
              <MenuItem value="">All actions</MenuItem>
              {ACTIONS.map((a) => <MenuItem key={a.value} value={a.value}>{a.label}</MenuItem>)}
            </Select>
          </Box>
          <Box sx={{ minWidth: 160 }}>
            <Select value={outcome} onChange={(e) => setOutcome(e.target.value as string)} size="small" displayEmpty fullWidth sx={selectSx}>
              <MenuItem value="">All outcomes</MenuItem>
              {OUTCOMES.map((o) => <MenuItem key={o.value} value={o.value}>{o.label}</MenuItem>)}
            </Select>
          </Box>
          <TextField
            type="datetime-local"
            label="Since"
            value={since}
            onChange={(e) => setSince(e.target.value)}
            size="small"
            InputLabelProps={{ shrink: true }}
          />
          <TextField
            type="datetime-local"
            label="Until"
            value={until}
            onChange={(e) => setUntil(e.target.value)}
            size="small"
            InputLabelProps={{ shrink: true }}
          />
        </Stack>
      </Surface>

      {error && <ErrorBox title="Audit query failed" description={error} onRetry={refresh} />}

      {loading && entries.length === 0 ? (
        <Surface sx={{ p: 4, textAlign: 'center' }}>
          <Typography variant="body2" color="text.secondary">Loading audit entries…</Typography>
        </Surface>
      ) : entries.length === 0 ? (
        <EmptyState
          illustration="grid"
          title="No audit entries match"
          description="Try clearing filters or run a project to generate audit activity."
        />
      ) : (
        <Surface sx={{ overflow: 'hidden' }}>
          <Box sx={{ overflowX: 'auto' }}>
            <Box sx={{ minWidth: 980 }}>
              <Box sx={auditHeaderSx}>
                {['Time', 'Action', 'Outcome', 'User', 'Project', 'Summary', 'Hash'].map((label) => (
                  <Box key={label} sx={thSx}>{label}</Box>
                ))}
              </Box>
              <VirtualList
                items={entries}
                itemHeight={58}
                getItemHeight={(entry) => (expanded === entry.id ? 260 : 58)}
                height={Math.min(620, Math.max(120, entries.length * 58 + (expanded ? 202 : 0)))}
                keyExtractor={(entry) => entry.id}
                ariaLabel="Audit entries"
                renderItem={(entry) => (
                  <AuditEntryRow
                    entry={entry}
                    expanded={expanded === entry.id}
                    onToggle={() => setExpanded(expanded === entry.id ? null : entry.id)}
                  />
                )}
              />
            </Box>
          </Box>
        </Surface>
      )}

      <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: 2 }}>
        <Typography variant="caption" color="text.secondary">
          {count} entr{count === 1 ? 'y' : 'ies'}
        </Typography>
        <Button onClick={refresh} startIcon={<Refresh fontSize="small" />} size="small">Refresh</Button>
      </Stack>
    </AppShell>
  );
}

function AuditEntryRow({
  entry,
  expanded,
  onToggle,
}: {
  entry: AuditEntry;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <Box sx={{ borderBottom: '1px solid rgba(17,17,17,0.08)' }}>
      <Box sx={trSx} onClick={onToggle}>
        <Box sx={tdSx}><Typography variant="caption" sx={{ fontFamily: tokens.font.mono }}>{formatTime(entry.createdAt)}</Typography></Box>
        <Box sx={tdSx}><Chip label={entry.action} size="small" sx={metaChipSx} /></Box>
        <Box sx={tdSx}><Chip label={entry.outcome} size="small" sx={outcomeChipSx(entry.outcome)} /></Box>
        <Box sx={tdSx}><Typography variant="caption" sx={{ fontFamily: tokens.font.mono }}>{short(entry.userId)}</Typography></Box>
        <Box sx={tdSx}><Typography variant="caption" sx={{ fontFamily: tokens.font.mono }}>{short(entry.projectId)}</Typography></Box>
        <Box sx={tdSx}><Typography variant="body2" noWrap title={entry.summary}>{entry.summary}</Typography></Box>
        <Box sx={tdSx}><Typography variant="caption" sx={{ fontFamily: tokens.font.mono }}>{entry.contentHash.slice(0, 10)}</Typography></Box>
      </Box>
      {expanded && (
        <Box sx={{ px: 1.4, py: 1, bgcolor: '#fffaf1', maxHeight: 202, overflow: 'auto' }}>
          <Stack spacing={0.6}>
            <Row label="ID" value={entry.id} mono />
            <Row label="Gate" value={entry.gateName || '-'} />
            <Row label="Agent role" value={entry.agentRole || '-'} />
            <Row label="InputHash" value={entry.inputHash || '-'} mono />
            <Row label="OutputHash" value={entry.outputHash || '-'} mono />
            <Row label="PrevHash" value={entry.prevHash || '-'} mono />
            <Row label="ContentHash" value={entry.contentHash} mono />
            {entry.attrs && Object.keys(entry.attrs).length > 0 && (
              <Box>
                <Typography variant="caption" color="text.secondary">Attrs</Typography>
                <Box component="pre" sx={preSx}>{JSON.stringify(entry.attrs, null, 2)}</Box>
              </Box>
            )}
          </Stack>
        </Box>
      )}
    </Box>
  );
}


function Row({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <Stack direction="row" spacing={1.2}>
      <Typography variant="caption" color="text.secondary" sx={{ minWidth: 110 }}>{label}</Typography>
      <Typography variant="caption" sx={{ fontFamily: mono ? tokens.font.mono : undefined, wordBreak: 'break-all' }}>{value}</Typography>
    </Stack>
  );
}

function short(id?: string) {
  if (!id) return '—';
  return id.length > 10 ? `${id.slice(0, 8)}…` : id;
}

function formatTime(iso: string) {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

const auditHeaderSx = {
  display: 'grid',
  gridTemplateColumns: '150px 170px 110px 110px 110px minmax(240px, 1fr) 90px',
  borderBottom: '1px solid rgba(17,17,17,0.08)',
};

const thSx = {
  textAlign: 'left',
  px: 1.4,
  py: 1,
  fontSize: '0.72rem',
  fontWeight: 800,
  textTransform: 'uppercase',
  color: '#5f5a52',
  bgcolor: '#fffaf1',
};

const tdSx = {
  px: 1.4,
  py: 1,
  minWidth: 0,
  verticalAlign: 'top',
};

const trSx = {
  display: 'grid',
  gridTemplateColumns: '150px 170px 110px 110px 110px minmax(240px, 1fr) 90px',
  cursor: 'pointer',
  '&:hover': { bgcolor: 'rgba(17,17,17,0.04)' },
};

const metaChipSx = {
  borderRadius: '4px',
  bgcolor: 'rgba(244,240,232,0.1)',
  border: '1px solid rgba(244,240,232,0.16)',
  color: tokens.color.text.primary,
  fontSize: '0.7rem',
  '& .MuiChip-label': { color: tokens.color.text.primary },
};

function outcomeChipSx(o: AuditOutcome) {
  const map: Record<AuditOutcome, { bg: string; border: string }> = {
    success: { bg: 'rgba(121,224,122,0.18)', border: 'rgba(121,224,122,0.45)' },
    failure: { bg: 'rgba(255,24,24,0.10)', border: 'rgba(255,24,24,0.45)' },
    blocked: { bg: 'rgba(255,196,0,0.16)', border: 'rgba(255,196,0,0.45)' },
  };
  const c = map[o] ?? { bg: '#fffaf1', border: 'rgba(17,17,17,0.12)' };
  return {
    borderRadius: '4px',
    bgcolor: c.bg,
    border: `1px solid ${c.border}`,
    color: tokens.color.text.primary,
    fontWeight: 700,
    fontSize: '0.7rem',
    '& .MuiChip-label': { color: tokens.color.text.primary },
  };
}

const selectSx = {
  bgcolor: '#fffaf1',
  borderRadius: '8px',
  '& .MuiOutlinedInput-notchedOutline': { borderColor: 'rgba(17,17,17,0.16)' },
};

const preSx = {
  fontFamily: tokens.font.mono,
  fontSize: '0.74rem',
  m: 0,
  mt: 0.4,
  p: 1,
  bgcolor: '#f4f0e8',
  border: '1px solid rgba(17,17,17,0.08)',
  borderRadius: '6px',
  overflowX: 'auto',
};
