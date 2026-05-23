'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Box, Button, InputAdornment, Stack, TextField, Typography } from '@mui/material';
import {
  AutoAwesome, BoltOutlined, Folder, GavelOutlined, Hub, Search,
} from '@mui/icons-material';
import { api, ExecutionEvent, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';
import { EmptyState, ErrorBox, SkeletonGrid, StatusPill, statusKindFromGate } from '../../../components/dashboard';
import { VirtualList } from '../../../components/performance/VirtualList';

type ResultKind = 'project' | 'patch' | 'gate' | 'nav';

interface SearchHit {
  id: string;
  kind: ResultKind;
  title: string;
  subtitle?: string;
  href: string;
  status?: string;
}

const quickLinks: SearchHit[] = [
  { id: 'nav-home',       kind: 'nav', title: 'Dashboard',       href: '/app',                    subtitle: 'Home screen' },
  { id: 'nav-projects',   kind: 'nav', title: 'My projects',     href: '/app/projects',           subtitle: 'All projects' },
  { id: 'nav-resources',  kind: 'nav', title: 'Templates and links', href: '/app/resources',      subtitle: 'Resource center' },
  { id: 'nav-connectors', kind: 'nav', title: 'Connectors',      href: '/app/connectors',         subtitle: 'Integrations' },
  { id: 'nav-settings',   kind: 'nav', title: 'Account settings', href: '/app/settings?tab=account', subtitle: 'Profile' },
  { id: 'nav-billing',    kind: 'nav', title: 'Billing and budget', href: '/app/settings?tab=billing', subtitle: 'Plans' },
  { id: 'nav-vault',      kind: 'nav', title: 'Vault',           href: '/app/settings?tab=vault',   subtitle: 'Margin ledger' },
];

export default function SearchPage() {
  return (
    <RequireAuth>
      <SearchInner />
    </RequireAuth>
  );
}

function SearchInner() {
  const { user, logout } = useAuth();
  const searchParams = useSearchParams();
  const router = useRouter();
  const initialQ = searchParams.get('q') ?? '';

  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState(initialQ);
  const [debouncedQuery, setDebouncedQuery] = useState(initialQ.trim().toLowerCase());

  useEffect(() => {
    let alive = true;
    setLoading(true);
    api.listProjects()
      .then((next) => { if (alive) setProjects(next); })
      .catch((e) => { if (alive) setError(e instanceof Error ? e.message : String(e)); })
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, []);

  useEffect(() => {
    const handle = setTimeout(() => {
      setDebouncedQuery(query.trim().toLowerCase());
      const params = new URLSearchParams(searchParams.toString());
      if (query.trim()) params.set('q', query.trim()); else params.delete('q');
      router.replace(`/app/search${params.toString() ? `?${params.toString()}` : ''}`);
    }, 220);
    return () => clearTimeout(handle);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query]);

  const results = useMemo(() => searchAll(projects, debouncedQuery), [projects, debouncedQuery]);

  const grouped = useMemo(() => ({
    projects: results.filter((r) => r.kind === 'project'),
    patches:  results.filter((r) => r.kind === 'patch'),
    gates:    results.filter((r) => r.kind === 'gate'),
    nav:      results.filter((r) => r.kind === 'nav'),
  }), [results]);

  const hasAny = results.length > 0;

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout}>
      <PageTitle
        eyebrow="Search"
        title="Search the workspace"
        subtitle="Search projects, patches, gates, and navigation. Results are local until a full search backend is connected."
      />

      <Surface sx={{ p: 1.4, mb: 1.8 }}>
        <TextField
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Type a project, patch, gate, or navigation target..."
          fullWidth
          autoFocus
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <Search />
              </InputAdornment>
            ),
          }}
          sx={{
            '& .MuiOutlinedInput-root': {
              bgcolor: '#fffaf1',
              borderRadius: '10px',
              fontSize: '1.05rem',
              py: 0.3,
            },
          }}
        />
        <Typography variant="caption" sx={{ color: '#86807a', display: 'block', mt: 1, px: 0.4 }}>
          Search runs locally across your projects, activity history, and navigation. Results are grouped by type.
        </Typography>
      </Surface>

      {error && (
        <Box sx={{ mb: 1.4 }}>
          <ErrorBox title="Could not load search results" description={error} onRetry={() => window.location.reload()} />
        </Box>
      )}

      {loading ? (
        <SkeletonGrid columns={1} count={4} minHeight={70} />
      ) : !debouncedQuery ? (
        <ResultGroup
          title="Quick navigation"
          icon={<Hub fontSize="small" />}
          hits={quickLinks}
        />
      ) : !hasAny ? (
        <EmptyState
          illustration="empty"
          title="No results"
          description={`No matches found for "${query}". Try another search or open the project list.`}
          primaryLabel="All projects"
          onPrimary={() => router.push('/app/projects')}
        />
      ) : (
        <Stack spacing={1.4}>
          <ResultGroup title="Projects" icon={<Folder fontSize="small" />} hits={grouped.projects} />
          <ResultGroup title="Patches" icon={<BoltOutlined fontSize="small" />} hits={grouped.patches} />
          <ResultGroup title="Gates" icon={<GavelOutlined fontSize="small" />} hits={grouped.gates} />
          <ResultGroup title="Navigation" icon={<Hub fontSize="small" />} hits={grouped.nav} />
        </Stack>
      )}
    </AppShell>
  );
}

function ResultGroup({ title, icon, hits }: { title: string; icon: React.ReactNode; hits: SearchHit[] }) {
  if (hits.length === 0) return null;
  return (
    <Surface sx={{ p: 0, overflow: 'hidden' }}>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ px: 1.8, py: 1.1, borderBottom: '1px solid rgba(17,17,17,0.08)' }}>
        {icon}
        <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>{title}</Typography>
        <Typography variant="caption" sx={{ color: '#86807a' }}>· {hits.length}</Typography>
      </Stack>
      <VirtualList
        items={hits}
        itemHeight={66}
        height={Math.min(420, Math.max(72, hits.length * 66))}
        keyExtractor={(hit) => hit.id}
        ariaLabel={`${title} search results`}
        renderItem={(hit) => <ResultRow hit={hit} />}
      />
    </Surface>
  );
}

function ResultRow({ hit }: { hit: SearchHit }) {
  return (
    <Stack
      component={Link}
      href={hit.href}
      direction="row"
      spacing={1.4}
      alignItems="center"
      sx={{
        px: 1.8, py: 1.2,
        minHeight: 66,
        color: 'inherit',
        textDecoration: 'none',
        transition: 'background-color 160ms',
        '&:hover': { bgcolor: 'rgba(229,255,0,0.12)' },
      }}
    >
      <Box sx={{ width: 30, height: 30, borderRadius: '8px', bgcolor: '#fffaf1', border: '1px solid rgba(17,17,17,0.12)', display: 'grid', placeItems: 'center' }}>
        {iconFor(hit.kind)}
      </Box>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="subtitle2" noWrap sx={{ fontWeight: 800 }}>{hit.title}</Typography>
        {hit.subtitle && <Typography variant="caption" sx={{ color: '#86807a' }} noWrap>{hit.subtitle}</Typography>}
      </Box>
      {hit.status && (
        <StatusPill kind={statusKindFromGate(hit.status)} label={hit.status} />
      )}
    </Stack>
  );
}

function iconFor(kind: ResultKind) {
  if (kind === 'project') return <Folder fontSize="small" />;
  if (kind === 'patch')   return <BoltOutlined fontSize="small" />;
  if (kind === 'gate')    return <GavelOutlined fontSize="small" />;
  return <AutoAwesome fontSize="small" />;
}

function searchAll(projects: Project[], q: string): SearchHit[] {
  if (!q) return [];
  const hits: SearchHit[] = [];

  for (const project of projects) {
    const hay = `${project.name} ${project.description} ${project.spec?.idea ?? ''} ${project.status}`.toLowerCase();
    if (hay.includes(q)) {
      hits.push({
        id: `p:${project.id}`,
        kind: 'project',
        title: project.name,
        subtitle: project.description || project.spec?.idea || 'Project',
        href: `/projects/${project.id}`,
        status: project.status,
      });
    }
    const events: ExecutionEvent[] = Array.isArray(project.events) ? project.events : [];
    for (const ev of events) {
      const evHay = `${ev.message} ${ev.step} ${ev.agent ?? ''} ${ev.status}`.toLowerCase();
      if (!evHay.includes(q)) continue;
      const isGate = ev.gate || (ev.step ?? '').toLowerCase().includes('gate');
      hits.push({
        id: `e:${project.id}:${ev.id}`,
        kind: isGate ? 'gate' : 'patch',
        title: ev.message.slice(0, 90),
        subtitle: `${project.name} · ${ev.agent || ev.step}`,
        href: `/projects/${project.id}`,
        status: ev.status,
      });
    }
    const gates = project.gates ?? ({} as Project['gates']);
    for (const gateName of Object.keys(gates) as Array<keyof typeof gates>) {
      const gate = gates[gateName];
      if (!gate) continue;
      const gateHay = `${gateName} ${gate.status}`.toLowerCase();
      if (!gateHay.includes(q)) continue;
      hits.push({
        id: `g:${project.id}:${gateName}`,
        kind: 'gate',
        title: `Gate ${gateName} — ${gate.status}`,
        subtitle: `${project.name}`,
        href: `/projects/${project.id}`,
        status: gate.status,
      });
    }
  }

  for (const link of quickLinks) {
    if (`${link.title} ${link.subtitle ?? ''}`.toLowerCase().includes(q)) {
      hits.push(link);
    }
  }

  return hits.slice(0, 80);
}
