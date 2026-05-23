'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Add, Apps, FilterList, Sort } from '@mui/icons-material';
import { Box, Button, Chip, InputAdornment, Stack, TextField, Typography } from '@mui/material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';
import {
  EmptyState, ErrorBox, ProjectGridCard, SkeletonGrid, StatusPill, statusKindFromGate,
} from '../../../components/dashboard';

const filters = [
  { value: 'all', label: 'All' },
  { value: 'ready', label: 'Ready' },
  { value: 'running', label: 'Running' },
  { value: 'pending', label: 'Pending' },
  { value: 'failed', label: 'Failed' },
];

const sorts = [
  { value: 'updated', label: 'Recently updated' },
  { value: 'created', label: 'Recently created' },
  { value: 'name', label: 'Name' },
];

const PAGE_SIZE = 24;

export default function ProjectsPage() {
  return (
    <RequireAuth>
      <ProjectsInner />
    </RequireAuth>
  );
}

function ProjectsInner() {
  const { user, logout } = useAuth();
  const searchParams = useSearchParams();
  const urlFilter = searchParams.get('filter');
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pageQuery, setPageQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [view, setView] = useState<'grid' | 'list'>('grid');
  const [filter, setFilter] = useState('all');
  const [sort, setSort] = useState('updated');
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);

  useEffect(() => {
    void refresh();
  }, []);

  async function refresh() {
    setLoading(true);
    setError(null);
    try {
      const list = await api.listProjects();
      setProjects(list);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    const next = (urlFilter ?? 'all').toLowerCase();
    setFilter(filters.some((f) => f.value === next) ? next : 'all');
  }, [urlFilter]);

  // Debounce search input.
  useEffect(() => {
    const handle = setTimeout(() => setDebouncedQuery(pageQuery.trim().toLowerCase()), 220);
    return () => clearTimeout(handle);
  }, [pageQuery]);

  // Reset pagination when filters/search change.
  useEffect(() => {
    setVisibleCount(PAGE_SIZE);
  }, [debouncedQuery, filter, sort]);

  const filtered = useMemo(() => {
    const list = projects.filter((project) => {
      const status = (project.status ?? '').toLowerCase();
      if (filter !== 'all' && status !== filter) return false;
      if (!debouncedQuery) return true;
      return `${project.name} ${project.description} ${project.status} ${project.spec?.idea ?? ''}`
        .toLowerCase()
        .includes(debouncedQuery);
    });
    if (sort === 'name') return [...list].sort((a, b) => a.name.localeCompare(b.name, 'en'));
    if (sort === 'created') return [...list].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
    return [...list].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
  }, [filter, projects, debouncedQuery, sort]);

  const visible = filtered.slice(0, visibleCount);
  const hasMore = visible.length < filtered.length;
  const statusCounts = useMemo(() => countByStatus(projects), [projects]);

  return (
    <AppShell
      userEmail={user?.email ?? 'workspace'}
      recents={projects.slice(0, 5)}
      onLogout={logout}
      query={pageQuery}
      setQuery={setPageQuery}
      view={view}
      setView={setView}
    >
      <PageTitle
        eyebrow="Projects"
        title="Your projects"
        subtitle="Every app you are finishing in one place. Filter by status, search the workspace, and open the next project that needs action."
        action={
          <Stack direction="row" spacing={1}>
            <Button component={Link} href="/app/resources" variant="outlined">Templates</Button>
            <Button component={Link} href="/app" variant="contained" startIcon={<Add />}>New project</Button>
          </Stack>
        }
      />

      {error && (
        <Box sx={{ mb: 1.6 }}>
          <ErrorBox
            title="Could not load projects"
            description={error}
            onRetry={() => void refresh()}
          />
        </Box>
      )}

      <Surface sx={{ p: 1.2, mb: 1.8 }}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.2} alignItems={{ xs: 'stretch', md: 'center' }}>
          <TextField
            placeholder="Search by name, description, or status..."
            value={pageQuery}
            onChange={(e) => setPageQuery(e.target.value)}
            size="small"
            sx={{ flex: 1, minWidth: 240 }}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <FilterList fontSize="small" />
                </InputAdornment>
              ),
            }}
          />
          <Stack direction="row" spacing={0.7} useFlexGap flexWrap="wrap" alignItems="center">
            {filters.map((item) => (
              <Chip
                key={item.value}
                label={item.value === 'all' ? `${item.label} (${projects.length})` : `${item.label} (${statusCounts[item.value] ?? 0})`}
                onClick={() => setFilter(item.value)}
                sx={filterChipSx(filter === item.value)}
              />
            ))}
          </Stack>
          <Stack direction="row" spacing={0.7} useFlexGap flexWrap="wrap" alignItems="center">
            <Chip icon={<Sort fontSize="small" />} label="Sort" sx={{ borderRadius: '6px', bgcolor: 'transparent', color: '#686158' }} />
            {sorts.map((item) => (
              <Chip key={item.value} label={item.label} onClick={() => setSort(item.value)} sx={filterChipSx(sort === item.value)} />
            ))}
          </Stack>
        </Stack>
      </Surface>

      {loading ? (
        <SkeletonGrid columns={view === 'grid' ? 3 : 1} count={6} minHeight={180} />
      ) : projects.length === 0 ? (
        <EmptyState
          illustration="grid"
          title="No projects yet. Describe the first product you want to ship."
          description="Start from a template or a plain-English idea. Ironflyer will create the workspace and move the project through the finisher gates."
          primaryLabel="Open prompt box"
          onPrimary={() => { window.location.href = '/app'; }}
          secondary={
            <Button component={Link} href="/app/resources" variant="outlined">Explore templates</Button>
          }
        />
      ) : filtered.length === 0 ? (
        <EmptyState
          illustration="empty"
          title="No projects match this filter"
          description="Clear the search or choose another status."
          primaryLabel="Clear filters"
          onPrimary={() => { setPageQuery(''); setFilter('all'); }}
        />
      ) : view === 'grid' ? (
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.5 }}>
          {visible.map((project) => <ProjectGridCard key={project.id} project={project} />)}
        </Box>
      ) : (
        <Stack spacing={1}>
          {visible.map((project) => <ProjectRow key={project.id} project={project} />)}
        </Stack>
      )}

      {hasMore && (
        <Box sx={{ display: 'flex', justifyContent: 'center', mt: 2.4 }}>
          <Button variant="outlined" onClick={() => setVisibleCount((n) => n + PAGE_SIZE)}>
            Show more ({filtered.length - visible.length})
          </Button>
        </Box>
      )}
    </AppShell>
  );
}

function ProjectRow({ project }: { project: Project }) {
  return (
    <Link href={`/projects/${project.id}`} style={{ color: 'inherit', textDecoration: 'none' }}>
      <Surface sx={{ p: 1.4, transition: 'border-color 180ms, background-color 180ms', '&:hover': { borderColor: 'rgba(17,17,17,0.28)', bgcolor: '#fffaf1' } }}>
        <Stack direction={{ xs: 'column', sm: 'row' }} alignItems={{ xs: 'stretch', sm: 'center' }} spacing={1.2}>
          <Stack direction="row" alignItems="center" spacing={1.4} sx={{ minWidth: 0, flex: 1 }}>
            <Box sx={{ width: 42, height: 42, flex: '0 0 auto', borderRadius: '8px', bgcolor: 'rgba(17,17,17,0.08)', display: 'grid', placeItems: 'center' }}>
              <Apps fontSize="small" />
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography variant="subtitle2" noWrap sx={{ fontWeight: 900 }}>{project.name}</Typography>
              <Typography variant="caption" color="text.secondary" noWrap>{project.description || project.spec?.idea}</Typography>
            </Box>
          </Stack>
          <Stack direction="row" spacing={1} alignItems="center" justifyContent={{ xs: 'space-between', sm: 'flex-end' }}>
            <StatusPill kind={statusKindFromGate(project.status)} label={project.status || 'idle'} />
            <Typography variant="caption" sx={{ color: '#86807a', fontFamily: tokens.font.mono }}>
              {new Date(project.updatedAt).toLocaleDateString('en-US', { day: '2-digit', month: 'short' })}
            </Typography>
          </Stack>
        </Stack>
      </Surface>
    </Link>
  );
}

function countByStatus(projects: Project[]) {
  const map: Record<string, number> = {};
  for (const project of projects) {
    const key = (project.status ?? 'idle').toLowerCase();
    map[key] = (map[key] ?? 0) + 1;
  }
  return map;
}

function filterChipSx(active: boolean) {
  return {
    borderRadius: '8px',
    bgcolor: active ? tokens.color.accent.lime : '#fffaf1',
    color: tokens.color.text.inverse,
    border: `1px solid ${active ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)'}`,
    fontWeight: 800,
    cursor: 'pointer',
  };
}
