'use client';

import { useEffect, useState } from 'react';
import { Box, Button, Chip, Stack, Switch, Typography } from '@mui/material';
import { Code, DataObject, GitHub, Hub, Storage, TravelExplore } from '@mui/icons-material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';

const connectors = [
  { name: 'Supabase', desc: 'Database, auth, storage, and row-level security.', icon: <Storage /> },
  { name: 'GitHub', desc: 'Repository sync, pull requests, and code collaboration.', icon: <GitHub /> },
  { name: 'Figma', desc: 'Reference frames, screenshots, and design systems.', icon: <DataObject /> },
  { name: 'Web Search', desc: 'Ground research, docs, and current product context.', icon: <TravelExplore /> },
  { name: 'Runtime', desc: 'Preview server, terminal, and code workspace.', icon: <Code /> },
  { name: 'MCP Servers', desc: 'Private context tools for teams and enterprise.', icon: <Hub /> },
];

export default function ConnectorsPage() {
  return (
    <RequireAuth>
      <ConnectorsInner />
    </RequireAuth>
  );
}

function ConnectorsInner() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'grid' | 'list'>('grid');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Connectors"
        title="Context and backend links"
        subtitle="Manage the services the builder can use while planning, coding, previewing, and shipping."
      />

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.4 }}>
        {connectors.map((connector, index) => (
          <Surface key={connector.name} sx={{ p: 1.8, minHeight: 190 }}>
            <Stack direction="row" justifyContent="space-between" alignItems="center">
              <Box sx={{
                width: 38,
                height: 38,
                borderRadius: 1,
                display: 'grid',
                placeItems: 'center',
                bgcolor: index < 2 ? tokens.color.accent.lime : 'rgba(17,17,17,0.08)',
                color: tokens.color.text.inverse,
              }}>
                {connector.icon}
              </Box>
              <Switch defaultChecked={index < 2} />
            </Stack>
            <Typography variant="h6" sx={{ mt: 2 }}>{connector.name}</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.6 }}>{connector.desc}</Typography>
            <Stack direction="row" spacing={0.7} sx={{ mt: 1.5 }}>
              <Chip label={index < 2 ? 'Connected' : 'Available'} size="small" sx={{ borderRadius: 1 }} />
              {index < 2 && <Chip label="Workspace" size="small" sx={{ borderRadius: 1, bgcolor: '#fffaf1', border: '1px solid rgba(17,17,17,0.12)' }} />}
            </Stack>
            <Button variant="outlined" size="small" sx={{ mt: 1.5 }}>{index < 2 ? 'Manage' : 'Connect'}</Button>
          </Surface>
        ))}
      </Box>
    </AppShell>
  );
}
