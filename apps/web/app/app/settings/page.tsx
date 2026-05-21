'use client';

import { useEffect, useState } from 'react';
import { Box, Button, Chip, Divider, Stack, Switch, TextField, Typography } from '@mui/material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';

export default function SettingsPage() {
  return (
    <RequireAuth>
      <SettingsInner />
    </RequireAuth>
  );
}

function SettingsInner() {
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
        eyebrow="Workspace settings"
        title="Team, billing, access"
        subtitle="The control center for workspace profile, project access defaults, credits, and privacy."
      />

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.1fr 0.9fr' }, gap: 1.5 }}>
        <Surface sx={{ p: 2 }}>
          <Typography variant="h6">Workspace profile</Typography>
          <Stack spacing={1.5} sx={{ mt: 2 }}>
            <TextField label="Workspace name" defaultValue="Ironflyer workspace" InputLabelProps={{ shrink: true }} />
            <TextField label="Default project visibility" defaultValue="Workspace" InputLabelProps={{ shrink: true }} />
            <TextField label="Primary domain" defaultValue="ironflyer.dev" InputLabelProps={{ shrink: true }} />
            <Button variant="contained" sx={{ alignSelf: 'flex-start' }}>Save changes</Button>
          </Stack>
        </Surface>

        <Surface sx={{ p: 2 }}>
          <Typography variant="h6">Usage and credits</Typography>
          <Stack spacing={1.2} sx={{ mt: 2 }}>
            <SettingMetric label="Plan" value="Free workspace" />
            <SettingMetric label="Projects" value={projects.length.toString()} />
            <SettingMetric label="Finisher gates" value="7 enabled" />
          </Stack>
          <Divider sx={{ my: 2 }} />
          <Button variant="contained" fullWidth>Upgrade credits</Button>
        </Surface>

        <Surface sx={{ p: 2 }}>
          <Typography variant="h6">Access and privacy</Typography>
          <Stack spacing={1} sx={{ mt: 1.5 }}>
            {['Require workspace access by default', 'Allow project remixing', 'Show badge on published apps', 'Use training data opt-out'].map((item, index) => (
              <Stack key={item} direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="body2">{item}</Typography>
                <Switch defaultChecked={index !== 1} />
              </Stack>
            ))}
          </Stack>
        </Surface>

        <Surface sx={{ p: 2 }}>
          <Typography variant="h6">Members</Typography>
          <Stack spacing={1} sx={{ mt: 1.5 }}>
            {[
              ['Demo User', user?.email ?? 'demo@ironflyer.dev', 'Owner'],
              ['Product Team', 'team@ironflyer.dev', 'Editor'],
              ['Agency Partner', 'agency@ironflyer.dev', 'Viewer'],
            ].map(([name, email, role]) => (
              <Stack key={email} direction="row" justifyContent="space-between" alignItems="center">
                <Box>
                  <Typography variant="subtitle2">{name}</Typography>
                  <Typography variant="caption" color="text.secondary">{email}</Typography>
                </Box>
                <Chip label={role} size="small" sx={{ borderRadius: 1 }} />
              </Stack>
            ))}
          </Stack>
          <Button variant="outlined" sx={{ mt: 2 }}>Invite member</Button>
        </Surface>
      </Box>
    </AppShell>
  );
}

function SettingMetric({ label, value }: { label: string; value: string }) {
  return (
    <Stack direction="row" justifyContent="space-between">
      <Typography variant="body2" color="text.secondary">{label}</Typography>
      <Typography variant="body2" sx={{ color: tokens.color.text.primary, fontWeight: 800 }}>{value}</Typography>
    </Stack>
  );
}
