'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { Folder, Home, Hub, Search, Settings } from '@mui/icons-material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';

const quickLinks = [
  { label: 'Dashboard', href: '/app', icon: <Home fontSize="small" /> },
  { label: 'Workspace settings', href: '/app/settings', icon: <Settings fontSize="small" /> },
  { label: 'Connectors', href: '/app/connectors', icon: <Hub fontSize="small" /> },
  { label: 'All projects', href: '/app/projects', icon: <Folder fontSize="small" /> },
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
  const [projects, setProjects] = useState<Project[]>([]);
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'grid' | 'list'>('grid');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return projects.slice(0, 6);
    return projects.filter((project) => `${project.name} ${project.description} ${project.spec.idea}`.toLowerCase().includes(q));
  }, [projects, query]);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Command palette"
        title="Search anything"
        subtitle="A focused command surface for projects, folders, settings, connectors, and workspace actions."
      />

      <Surface sx={{ p: { xs: 1.6, md: 2.2 }, mb: 2 }}>
        <Stack direction="row" spacing={1.3} alignItems="center">
          <Search sx={{ color: tokens.color.accent.lime }} />
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="overline" color="text.secondary">Type in the top search field</Typography>
            <Typography variant="body2">Results update live and stay grouped by project or navigation target.</Typography>
          </Box>
          <Chip label="Cmd K" sx={{ borderRadius: '6px', bgcolor: '#fffaf1', border: '1px solid rgba(17,17,17,0.12)' }} />
        </Stack>
      </Surface>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 360px' }, gap: 1.5 }}>
        <Surface sx={{ p: 1.2 }}>
          <Typography variant="overline" color="text.secondary" sx={{ px: 1 }}>Projects</Typography>
          {results.length === 0 ? (
            <Box sx={{ px: 1, py: 3 }}>
              <Typography variant="subtitle2">No project results</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.4 }}>
                Try a different project name, prompt, or status.
              </Typography>
            </Box>
          ) : (
            <Stack spacing={0.5} sx={{ mt: 0.8 }}>
              {results.map((project) => (
                <Button key={project.id} component={Link} href={`/projects/${project.id}`} sx={resultButtonSx}>
                  <Box sx={{ flex: 1, minWidth: 0, textAlign: 'left' }}>
                    <Typography variant="subtitle2" noWrap>{project.name}</Typography>
                    <Typography variant="caption" color="text.secondary" noWrap>{project.description || project.spec.idea}</Typography>
                  </Box>
                  <Chip label={project.status} size="small" sx={{ borderRadius: '6px' }} />
                </Button>
              ))}
            </Stack>
          )}
        </Surface>

        <Surface sx={{ p: 1.2 }}>
          <Typography variant="overline" color="text.secondary" sx={{ px: 1 }}>Quick navigation</Typography>
          <Stack spacing={0.5} sx={{ mt: 0.8 }}>
            {quickLinks.map((item) => (
              <Button key={item.label} component={Link} href={item.href} startIcon={item.icon} sx={resultButtonSx}>
                {item.label}
              </Button>
            ))}
          </Stack>
        </Surface>
      </Box>
    </AppShell>
  );
}

const resultButtonSx = {
  width: '100%',
  justifyContent: 'flex-start',
  color: tokens.color.text.primary,
  bgcolor: 'transparent',
  borderRadius: '8px',
  px: 1,
  py: 1,
  '&:hover': { bgcolor: 'rgba(17,17,17,0.06)' },
};
