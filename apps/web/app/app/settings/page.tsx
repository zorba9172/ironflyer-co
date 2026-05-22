'use client';

import { useEffect, useState } from 'react';
import { Box, Button, Chip, Divider, LinearProgress, Stack, Switch, TextField, Typography } from '@mui/material';
import { AccountTree, Key, Lock, Speed, TaskAlt } from '@mui/icons-material';
import { api, Plan, Project, UserBudget } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { UpgradeButton } from '../../upgrade-button';
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
  const [budget, setBudget] = useState<UserBudget | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
    void api.myBudget().then(setBudget).catch(() => setBudget(null));
    void api.listPlans().then(setPlans).catch(() => setPlans([]));
  }, []);

  const currentPlan = plans.find((plan) => plan.tier === budget?.tier);
  const spent = Number(budget?.spent ?? 0);
  const cap = Number(currentPlan?.costCapUSD ?? 0.5);
  const spendPercent = cap > 0 ? Math.min(100, (spent / cap) * 100) : 0;

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Workspace settings"
        title="Team, billing, access"
        subtitle="The control center for workspace profile, agent modes, spend guardrails, secrets, and privacy."
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
            <SettingMetric label="Plan" value={currentPlan?.name ?? 'Free workspace'} />
            <SettingMetric label="Projects" value={projects.length.toString()} />
            <SettingMetric label="Provider cost cap" value={`$${cap.toFixed(2)}`} />
            <SettingMetric label="Finisher gates" value="7 enabled" />
          </Stack>
          <Box sx={{ mt: 1.6 }}>
            <Stack direction="row" justifyContent="space-between">
              <Typography variant="caption" color="text.secondary">Monthly guardrail</Typography>
              <Typography variant="caption" sx={{ fontFamily: tokens.font.mono }}>${spent.toFixed(2)} / ${cap.toFixed(2)}</Typography>
            </Stack>
            <LinearProgress variant="determinate" value={spendPercent} sx={{
              mt: 0.7,
              height: 7,
              borderRadius: '999px',
              bgcolor: 'rgba(17,17,17,0.1)',
              '& .MuiLinearProgress-bar': { bgcolor: spendPercent > 82 ? tokens.color.accent.coral : tokens.color.accent.lime },
            }} />
          </Box>
          <Divider sx={{ my: 2 }} />
          <Stack spacing={0.8}>
            <UpgradeButton tier="pro" label="Upgrade to Pro" fullWidth size="small" />
            <UpgradeButton tier="team" label="Create team plan" fullWidth size="small" variant="outlined" />
          </Stack>
        </Surface>

        <Surface sx={{ p: 2 }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Lock sx={{ color: tokens.color.accent.coral }} />
            <Typography variant="h6">Access and privacy</Typography>
          </Stack>
          <Stack spacing={1} sx={{ mt: 1.5 }}>
            {['Require workspace access by default', 'Allow project remixing', 'Show badge on published apps', 'Use training data opt-out', 'Require approval before external tool calls'].map((item, index) => (
              <Stack key={item} direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="body2">{item}</Typography>
                <Switch defaultChecked={index !== 1 && index !== 2} />
              </Stack>
            ))}
          </Stack>
        </Surface>

        <Surface sx={{ p: 2 }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Speed sx={{ color: tokens.color.accent.sky }} />
            <Typography variant="h6">Agent modes</Typography>
          </Stack>
          <Stack spacing={1} sx={{ mt: 1.5 }}>
            {[
              ['Lite', 'Fast visual tweaks and small fixes', true],
              ['Economy', 'Default cost-aware build mode', true],
              ['Power', 'Large changes, migrations, and deep debugging', true],
              ['Turbo', 'Higher-cost accelerated builds', false],
            ].map(([name, desc, enabled]) => (
              <Stack key={name as string} direction="row" justifyContent="space-between" alignItems="center" spacing={1}>
                <Box>
                  <Typography variant="subtitle2">{name}</Typography>
                  <Typography variant="caption" color="text.secondary">{desc}</Typography>
                </Box>
                <Switch defaultChecked={Boolean(enabled)} />
              </Stack>
            ))}
          </Stack>
        </Surface>

        <Surface sx={{ p: 2 }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Key sx={{ color: tokens.color.accent.lime }} />
            <Typography variant="h6">Secrets and connectors</Typography>
          </Stack>
          <Stack spacing={1} sx={{ mt: 1.5 }}>
            {[
              ['DATABASE_URL', 'Project runtime', 'Required'],
              ['GITHUB_TOKEN', 'Git sync', 'Connected'],
              ['STRIPE_SECRET_KEY', 'Payments', 'Missing'],
            ].map(([key, scope, state]) => (
              <Stack key={key} direction="row" justifyContent="space-between" alignItems="center" spacing={1}>
                <Box sx={{ minWidth: 0 }}>
                  <Typography variant="subtitle2" sx={{ fontFamily: tokens.font.mono }} noWrap>{key}</Typography>
                  <Typography variant="caption" color="text.secondary">{scope}</Typography>
                </Box>
                <Chip label={state} size="small" sx={{ borderRadius: '6px', bgcolor: state === 'Missing' ? 'rgba(255,108,58,0.18)' : '#fffaf1' }} />
              </Stack>
            ))}
          </Stack>
          <Button variant="outlined" sx={{ mt: 2 }}>Manage secrets</Button>
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
                <Chip label={role} size="small" sx={{ borderRadius: '6px' }} />
              </Stack>
            ))}
          </Stack>
          <Button variant="outlined" sx={{ mt: 2 }}>Invite member</Button>
        </Surface>

        <Surface sx={{ p: 2 }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <AccountTree sx={{ color: tokens.color.accent.violet }} />
            <Typography variant="h6">Release policy</Typography>
          </Stack>
          <Stack spacing={1} sx={{ mt: 1.5 }}>
            {['Spec approved', 'UX reviewed', 'Tests passing', 'Security gate passed'].map((item, index) => (
              <Stack key={item} direction="row" spacing={1} alignItems="center">
                <TaskAlt fontSize="small" sx={{ color: index < 2 ? tokens.color.accent.lime : '#8b8276' }} />
                <Typography variant="body2">{item}</Typography>
              </Stack>
            ))}
          </Stack>
          <Button variant="outlined" sx={{ mt: 2 }}>Edit policy</Button>
        </Surface>
      </Box>
    </AppShell>
  );
}

function SettingMetric({ label, value }: { label: string; value: string }) {
  return (
    <Stack direction="row" justifyContent="space-between">
      <Typography variant="body2" color="text.secondary">{label}</Typography>
      <Typography variant="body2" sx={{ color: tokens.color.text.inverse, fontWeight: 800 }}>{value}</Typography>
    </Stack>
  );
}
