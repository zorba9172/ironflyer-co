'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { Apps, FilterList, Lock, Public, Sort } from '@mui/icons-material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';

const filters = ['All', 'Ready', 'Running', 'Pending', 'Failed'];
const sorts = ['Last edited', 'Last viewed', 'Created', 'Name'];

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
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'grid' | 'list'>('grid');
  const [filter, setFilter] = useState('All');
  const [sort, setSort] = useState('Last edited');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  useEffect(() => {
    const nextFilter = normalizeFilter(urlFilter);
    setFilter(nextFilter);
  }, [urlFilter]);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = projects.filter((project) => {
      if (filter !== 'All' && project.status.toLowerCase() !== filter.toLowerCase()) {
        return false;
      }
      if (!q) return true;
      return `${project.name} ${project.description} ${project.status} ${project.spec.idea}`.toLowerCase().includes(q);
    });
    if (sort === 'Name') return [...list].sort((a, b) => a.name.localeCompare(b.name));
    if (sort === 'Created') return [...list].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
    return [...list].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
  }, [filter, projects, query, sort]);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Projects"
        title="Everything in this workspace"
        subtitle="Search, filter, sort, and jump back into any build without losing context."
        action={<Button component={Link} href="/app" variant="contained">New project</Button>}
      />

      <Stack spacing={2.2}>
        <Surface sx={{ p: 1.2 }}>
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1} justifyContent="space-between">
            <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap">
              <Chip icon={<FilterList />} label="Status" sx={labelChipSx} />
              {filters.map((item) => (
                <Chip key={item} label={item} onClick={() => setFilter(item)} sx={filterChipSx(filter === item)} />
              ))}
            </Stack>
            <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap">
              <Chip icon={<Sort />} label="Sort" sx={labelChipSx} />
              {sorts.map((item) => (
                <Chip key={item} label={item} onClick={() => setSort(item)} sx={filterChipSx(sort === item)} />
              ))}
            </Stack>
          </Stack>
        </Surface>

        {visible.length === 0 ? (
          <Surface sx={{ p: 4, textAlign: 'center' }}>
            <Typography variant="h6">No projects match this view</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.6 }}>
              Clear the search or switch status filters to see more projects.
            </Typography>
          </Surface>
        ) : view === 'grid' ? (
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.4 }}>
            {visible.map((project) => <ProjectCard key={project.id} project={project} />)}
          </Box>
        ) : (
          <Stack spacing={1}>
            {visible.map((project) => <ProjectRow key={project.id} project={project} />)}
          </Stack>
        )}
      </Stack>
    </AppShell>
  );
}

function ProjectCard({ project }: { project: Project }) {
  const passed = Object.values(project.gates).filter((gate) => gate.status === 'passed').length;
  const total = Object.keys(project.gates).length || 7;
  return (
    <Link href={`/projects/${project.id}`} style={{ color: 'inherit', textDecoration: 'none' }}>
      <Surface sx={{ overflow: 'hidden', minHeight: 238, '&:hover': { borderColor: tokens.color.border.strong } }}>
        <Box sx={{
          height: 118,
          backgroundImage: `linear-gradient(135deg, rgba(229,255,0,0.2), rgba(112,214,255,0.08)), url('/marketplace/output-ref/hero.jpg')`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          borderBottom: '1px solid rgba(17,17,17,0.1)',
        }} />
        <Box sx={{ p: 1.8 }}>
          <Stack direction="row" justifyContent="space-between" spacing={1}>
            <Typography variant="subtitle1" sx={{ fontWeight: 900 }} noWrap>{project.name}</Typography>
            <Chip label={project.status} size="small" sx={{ borderRadius: '6px' }} />
          </Stack>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.8, minHeight: 42 }}>
            {project.description || project.spec.idea || 'Prompt-to-product workspace'}
          </Typography>
          <Box sx={{ mt: 2, height: 6, bgcolor: 'rgba(17,17,17,0.1)', borderRadius: '999px', overflow: 'hidden' }}>
            <Box sx={{ width: `${(passed / total) * 100}%`, height: '100%', bgcolor: tokens.color.accent.lime }} />
          </Box>
          <Stack direction="row" justifyContent="space-between" sx={{ mt: 1 }}>
            <Typography variant="caption" color="text.secondary">Gates {passed}/{total}</Typography>
            <Typography variant="caption" color="text.secondary">{new Date(project.updatedAt).toLocaleDateString()}</Typography>
          </Stack>
        </Box>
      </Surface>
    </Link>
  );
}

function ProjectRow({ project }: { project: Project }) {
  return (
    <Link href={`/projects/${project.id}`} style={{ color: 'inherit', textDecoration: 'none' }}>
      <Surface sx={{ p: 1.4, '&:hover': { borderColor: tokens.color.border.strong } }}>
        <Stack direction={{ xs: 'column', sm: 'row' }} alignItems={{ xs: 'stretch', sm: 'center' }} spacing={1.2}>
          <Stack direction="row" alignItems="center" spacing={1.4} sx={{ minWidth: 0, flex: 1 }}>
            <Box sx={{ width: 42, height: 42, flex: '0 0 auto', borderRadius: '8px', bgcolor: 'rgba(17,17,17,0.08)', display: 'grid', placeItems: 'center' }}>
              <Apps fontSize="small" />
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography variant="subtitle2" noWrap>{project.name}</Typography>
              <Typography variant="caption" color="text.secondary" noWrap>{project.description || project.spec.idea}</Typography>
            </Box>
          </Stack>
          <Box sx={{ display: 'flex', justifyContent: { xs: 'flex-start', sm: 'flex-end' } }}>
            <Chip icon={project.status === 'ready' ? <Lock /> : <Public />} label={project.status} size="small" sx={{ borderRadius: '6px' }} />
          </Box>
        </Stack>
      </Surface>
    </Link>
  );
}

const labelChipSx = {
  bgcolor: 'transparent',
  color: '#686158',
  borderRadius: '6px',
};

function filterChipSx(active: boolean) {
  return {
    borderRadius: '8px',
    bgcolor: active ? tokens.color.accent.lime : '#fffaf1',
    color: tokens.color.text.inverse,
    border: `1px solid ${active ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)'}`,
    fontWeight: 800,
  };
}

function normalizeFilter(value: string | null) {
  const match = filters.find((item) => item.toLowerCase() === value?.toLowerCase());
  return match ?? 'All';
}
