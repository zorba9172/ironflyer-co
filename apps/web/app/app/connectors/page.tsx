'use client';

import { useEffect, useMemo, useState } from 'react';
import { Box, Button, Chip, LinearProgress, Stack, Switch, Typography } from '@mui/material';
import {
  Code, DataObject, GitHub, Hub, Key, Lock, Security, Storage, TravelExplore,
} from '@mui/icons-material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';

const connectors = [
  {
    name: 'Supabase',
    group: 'App connectors',
    desc: 'Database, auth, storage, and row-level security.',
    icon: <Storage />,
    status: 'Available',
    scope: 'Workspace',
    security: 'Secrets required',
  },
  {
    name: 'GitHub',
    group: 'App connectors',
    desc: 'Repository sync, pull requests, and code collaboration.',
    icon: <GitHub />,
    status: 'Connected',
    scope: 'Workspace',
    security: 'OAuth',
  },
  {
    name: 'Figma',
    group: 'Chat connectors',
    desc: 'Reference frames, screenshots, and design systems while building.',
    icon: <DataObject />,
    status: 'Available',
    scope: 'Personal',
    security: 'Approval required',
  },
  {
    name: 'Web Search',
    group: 'Chat connectors',
    desc: 'Ground research, docs, and current product context.',
    icon: <TravelExplore />,
    status: 'Connected',
    scope: 'Personal',
    security: 'Cited output',
  },
  {
    name: 'Runtime',
    group: 'Runtime',
    desc: 'Preview server, terminal, file browser, and code workspace.',
    icon: <Code />,
    status: 'Connected',
    scope: 'Project',
    security: 'Sandboxed',
  },
  {
    name: 'MCP Servers',
    group: 'Chat connectors',
    desc: 'Private context tools for teams and enterprise workflows.',
    icon: <Hub />,
    status: 'Available',
    scope: 'Personal',
    security: 'Admin policy',
  },
];

const groups = ['All', 'App connectors', 'Chat connectors', 'Runtime'];

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
  const [group, setGroup] = useState('All');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return connectors.filter((connector) => {
      if (group !== 'All' && connector.group !== group) return false;
      if (!q) return true;
      return `${connector.name} ${connector.group} ${connector.desc} ${connector.status} ${connector.scope}`.toLowerCase().includes(q);
    });
  }, [group, query]);

  const connectedCount = connectors.filter((connector) => connector.status === 'Connected').length;
  const readiness = Math.round((connectedCount / connectors.length) * 100);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Connectors"
        title="Context and backend links"
        subtitle="Manage the services the builder can use while planning, coding, previewing, and shipping."
      />

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.1fr 0.9fr' }, gap: 1.4, mb: 1.4 }}>
        <Surface sx={{ p: 1.8 }}>
          <Stack direction="row" spacing={1.2} alignItems="center">
            <Box sx={{ width: 42, height: 42, borderRadius: '8px', bgcolor: tokens.color.accent.lime, display: 'grid', placeItems: 'center' }}>
              <Security />
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography variant="h6">Connector readiness</Typography>
              <Typography variant="body2" color="text.secondary">
                {connectedCount} of {connectors.length} links are ready for agent use.
              </Typography>
            </Box>
            <Typography variant="h5" sx={{ color: tokens.color.text.inverse }}>{readiness}%</Typography>
          </Stack>
          <LinearProgress variant="determinate" value={readiness} sx={{
            mt: 1.5,
            height: 7,
            borderRadius: '999px',
            bgcolor: 'rgba(17,17,17,0.1)',
            '& .MuiLinearProgress-bar': { bgcolor: tokens.color.accent.lime },
          }} />
        </Surface>
        <Surface sx={{ p: 1.8 }}>
          <Stack direction="row" spacing={1.2} alignItems="flex-start">
            <Lock sx={{ color: tokens.color.accent.coral }} />
            <Box>
              <Typography variant="h6">Safe by default</Typography>
              <Typography variant="body2" color="text.secondary">
                App connectors are reusable workspace capabilities; chat connectors provide build-time context and do not ship in the app.
              </Typography>
            </Box>
          </Stack>
        </Surface>
      </Box>

      <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap" sx={{ mb: 1.4 }}>
        {groups.map((item) => (
          <Chip
            key={item}
            label={item}
            onClick={() => setGroup(item)}
            sx={{
              borderRadius: '8px',
              bgcolor: group === item ? tokens.color.accent.lime : '#fffaf1',
              color: tokens.color.text.inverse,
              border: `1px solid ${group === item ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)'}`,
              fontWeight: 800,
            }}
          />
        ))}
      </Stack>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.4 }}>
        {visible.map((connector, index) => (
          <Surface key={connector.name} sx={{ p: 1.8, minHeight: 190 }}>
            <Stack direction="row" justifyContent="space-between" alignItems="center">
              <Box sx={{
                width: 38,
                height: 38,
                borderRadius: '8px',
                display: 'grid',
                placeItems: 'center',
                bgcolor: connector.status === 'Connected' ? tokens.color.accent.lime : 'rgba(17,17,17,0.08)',
                color: tokens.color.text.inverse,
              }}>
                {connector.icon}
              </Box>
              <Switch defaultChecked={connector.status === 'Connected'} />
            </Stack>
            <Typography variant="h6" sx={{ mt: 2 }}>{connector.name}</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.6 }}>{connector.desc}</Typography>
            <Stack direction="row" spacing={0.7} useFlexGap flexWrap="wrap" sx={{ mt: 1.5 }}>
              <Chip label={connector.status} size="small" sx={{ borderRadius: '6px' }} />
              <Chip label={connector.scope} size="small" sx={lightChipSx} />
              <Chip icon={<Key />} label={connector.security} size="small" sx={lightChipSx} />
            </Stack>
            <Button variant="outlined" size="small" sx={{ mt: 1.5 }}>{connector.status === 'Connected' ? 'Manage' : 'Connect'}</Button>
          </Surface>
        ))}
      </Box>
      {visible.length === 0 && (
        <Surface sx={{ p: 4, mt: 1.4, textAlign: 'center' }}>
          <Typography variant="h6">No connectors match</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            Clear search or switch connector type.
          </Typography>
        </Surface>
      )}
    </AppShell>
  );
}

const lightChipSx = {
  borderRadius: '6px',
  bgcolor: '#fffaf1',
  border: '1px solid rgba(17,17,17,0.12)',
  color: '#514a41',
};
