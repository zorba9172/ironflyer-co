'use client';

// Project memory browser. Surfaces /api/memory — the four-store
// intelligence index (project / execution / user / business). Builders
// see what the orchestrator has learned across runs and can search /
// slice by tag without round-tripping through the chat agent.

import { useEffect, useMemo, useState } from 'react';
import {
  Box, Button, Chip, MenuItem, Select, Stack, TextField, Typography,
} from '@mui/material';
import { Refresh } from '@mui/icons-material';
import {
  api, MemoryRecord, MemoryKind, Project,
} from '../../../../lib/api';
import { tokens } from '../../../../lib/theme';
import { RequireAuth, useAuth } from '../../../auth-context';
import { AppShell, PageTitle, Surface } from '../../workspace-shell';
import { EmptyState, ErrorBox } from '../../../../components/dashboard';

const KINDS: { value: MemoryKind; label: string }[] = [
  { value: 'project',   label: 'Project' },
  { value: 'execution', label: 'Execution' },
  { value: 'user',      label: 'User' },
  { value: 'business',  label: 'Business' },
];

export default function MemoryPage() {
  return (
    <RequireAuth>
      <MemoryInner />
    </RequireAuth>
  );
}

function MemoryInner() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [records, setRecords] = useState<MemoryRecord[]>([]);
  const [count, setCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);

  const [kind, setKind] = useState<MemoryKind | ''>('project');
  const [projectId, setProjectId] = useState<string>('');
  const [tag, setTag] = useState('');
  const [query, setQuery] = useState('');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const refresh = useMemo(() => () => {
    if (!kind && !projectId) {
      // backend rejects firehose reads; default to "project" kind.
      setRecords([]);
      setCount(0);
      return;
    }
    setLoading(true);
    setError(null);
    api.listMemory({
      kind: kind || undefined,
      projectId: projectId || undefined,
      tag: tag || undefined,
      q: query || undefined,
      limit: 100,
    })
      .then((r) => { setRecords(r.records); setCount(r.count); })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false));
  }, [kind, projectId, tag, query]);

  useEffect(() => { refresh(); }, [refresh]);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout}>
      <PageTitle
        eyebrow="Intelligence"
        title="Project memory"
        subtitle="The four-store intelligence index the orchestrator builds while it runs. Browse what the system has learned about this project, this user, and the business goal."
        action={(
          <Button variant="outlined" startIcon={<Refresh fontSize="small" />} onClick={refresh}>
            Refresh
          </Button>
        )}
      />

      <Surface sx={{ p: 1.6, mb: 2 }}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.2} alignItems={{ md: 'center' }} useFlexGap flexWrap="wrap">
          <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap">
            {KINDS.map((k) => (
              <Chip
                key={k.value}
                label={k.label}
                onClick={() => setKind(kind === k.value ? '' : k.value)}
                size="small"
                sx={chipFilterSx(kind === k.value)}
              />
            ))}
          </Stack>
          <Box sx={{ minWidth: 200 }}>
            <Select
              value={projectId}
              onChange={(e) => setProjectId(e.target.value as string)}
              size="small"
              displayEmpty
              fullWidth
              sx={selectSx}
            >
              <MenuItem value="">All projects</MenuItem>
              {projects.map((p) => (
                <MenuItem key={p.id} value={p.id}>{p.name}</MenuItem>
              ))}
            </Select>
          </Box>
          <TextField
            value={tag}
            onChange={(e) => setTag(e.target.value)}
            placeholder="Tag (e.g. decision, fix)"
            size="small"
            sx={{ minWidth: 200 }}
          />
          <TextField
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search title / body"
            size="small"
            sx={{ minWidth: 240, flex: 1 }}
          />
        </Stack>
      </Surface>

      {error && <ErrorBox title="Memory query failed" description={error} onRetry={refresh} />}

      {loading && records.length === 0 ? (
        <Surface sx={{ p: 4, textAlign: 'center' }}>
          <Typography variant="body2" color="text.secondary">Loading memory…</Typography>
        </Surface>
      ) : records.length === 0 ? (
        <EmptyState
          illustration="orbit"
          title="No memory recorded yet"
          description="Run a project to start accumulating intelligence. The orchestrator records decisions, failures, fixes, and user preferences automatically."
        />
      ) : (
        <Stack spacing={1.2}>
          {records.map((rec) => (
            <Surface key={rec.id} sx={{
              p: 2,
              cursor: 'pointer',
              transition: `border-color ${tokens.motion.base} ${tokens.motion.curve}`,
              '&:hover': { borderColor: 'rgba(17,17,17,0.28)' },
            }}>
              <Box onClick={() => setExpanded(expanded === rec.id ? null : rec.id)}>
                <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={1}>
                  <Box sx={{ minWidth: 0, flex: 1 }}>
                    <Stack direction="row" spacing={0.6} alignItems="center" sx={{ mb: 0.4 }}>
                      <Chip label={rec.kind} size="small" sx={kindChipSx(rec.kind)} />
                      {(rec.tags ?? []).map((t) => (
                        <Chip key={t} label={t} size="small" sx={metaChipSx} />
                      ))}
                    </Stack>
                    <Typography variant="subtitle1" sx={{ fontWeight: 800 }}>{rec.title || '(untitled)'}</Typography>
                    <Typography
                      variant="body2"
                      sx={{
                        color: '#514a41',
                        mt: 0.4,
                        whiteSpace: 'pre-wrap',
                        display: expanded === rec.id ? 'block' : '-webkit-box',
                        WebkitLineClamp: 3,
                        WebkitBoxOrient: 'vertical',
                        overflow: 'hidden',
                      }}
                    >
                      {rec.body}
                    </Typography>
                  </Box>
                  <Typography variant="caption" sx={{ color: '#86807a', fontFamily: tokens.font.mono, whiteSpace: 'nowrap' }}>
                    {formatTime(rec.createdAt)}
                  </Typography>
                </Stack>
              </Box>
            </Surface>
          ))}
        </Stack>
      )}

      <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: 2 }}>
        <Typography variant="caption" color="text.secondary">
          {count} record{count === 1 ? '' : 's'}
        </Typography>
        <Button onClick={refresh} startIcon={<Refresh fontSize="small" />} size="small">
          Refresh
        </Button>
      </Stack>
    </AppShell>
  );
}

function chipFilterSx(active: boolean) {
  return {
    borderRadius: '6px',
    bgcolor: active ? tokens.color.accent.lime : '#fffaf1',
    border: `1px solid ${active ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)'}`,
    color: tokens.color.text.inverse,
    fontWeight: 700,
  };
}

function kindChipSx(kind: MemoryKind) {
  const map: Record<MemoryKind, string> = {
    project:   '#dfe9c6',
    execution: '#f6dccb',
    user:      '#dde7f1',
    business:  '#efe1cb',
  };
  return {
    borderRadius: '4px',
    bgcolor: map[kind] ?? '#fffaf1',
    border: '1px solid rgba(17,17,17,0.12)',
    color: tokens.color.text.inverse,
    fontWeight: 700,
    textTransform: 'uppercase',
    fontSize: '0.66rem',
  };
}

const metaChipSx = {
  borderRadius: '4px',
  bgcolor: '#fffaf1',
  border: '1px solid rgba(17,17,17,0.12)',
  color: '#514a41',
  fontSize: '0.7rem',
};

const selectSx = {
  bgcolor: '#fffaf1',
  borderRadius: '8px',
  '& .MuiOutlinedInput-notchedOutline': { borderColor: 'rgba(17,17,17,0.16)' },
};

function formatTime(iso: string) {
  try {
    const d = new Date(iso);
    return d.toLocaleString();
  } catch {
    return iso;
  }
}
