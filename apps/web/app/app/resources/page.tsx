'use client';

import { useEffect, useState } from 'react';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { AutoAwesome, Dashboard, Language, Storefront, Terminal } from '@mui/icons-material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';

const categories = [
  { label: 'All', icon: <Dashboard fontSize="small" /> },
  { label: 'Websites', icon: <Language fontSize="small" /> },
  { label: 'Apps', icon: <AutoAwesome fontSize="small" /> },
  { label: 'Internal tools', icon: <Terminal fontSize="small" /> },
  { label: 'Commerce', icon: <Storefront fontSize="small" /> },
];

const resources = [
  { title: 'SaaS dashboard', type: 'Apps', img: '/marketplace/output-ref/hooked.png', bg: '#e9ae91', desc: 'Auth, billing, chart cards, admin settings.' },
  { title: 'Internal ops tool', type: 'Internal tools', img: '/marketplace/output-ref/fx.png', bg: '#191919', desc: 'Approvals, roles, reports, audit history.' },
  { title: 'Client portal', type: 'Websites', img: '/marketplace/output-ref/pack-generator.png', bg: '#001f1c', desc: 'Documents, status, comments, and project updates.' },
  { title: 'Launch site', type: 'Websites', img: '/marketplace/output-ref/gear.png', bg: '#e8e2d2', desc: 'Hero, pricing, waitlist, FAQs, and conversion sections.' },
  { title: 'Analytics room', type: 'Apps', img: '/marketplace/output-ref/hero.jpg', bg: '#121212', desc: 'Metrics, filters, cohorts, and operations dashboards.' },
  { title: 'Commerce storefront', type: 'Commerce', img: '/marketplace/output-ref/hero.jpg', bg: '#121212', desc: 'Catalog, cart, checkout, orders, and CMS-ready pages.' },
];

export default function ResourcesPage() {
  return (
    <RequireAuth>
      <ResourcesInner />
    </RequireAuth>
  );
}

function ResourcesInner() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'grid' | 'list'>('grid');
  const [category, setCategory] = useState('All');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const visible = resources.filter((item) => category === 'All' || item.type === category);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Resources"
        title="Templates and remix starts"
        subtitle="A Lovable-style resource library: pick a foundation, remix it, then continue in the project editor."
      />

      <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap" sx={{ mb: 2.2 }}>
        {categories.map((item) => (
          <Chip
            key={item.label}
            icon={item.icon}
            label={item.label}
            onClick={() => setCategory(item.label)}
            sx={{
              borderRadius: 1,
              bgcolor: category === item.label ? tokens.color.accent.lime : '#fffaf1',
              color: tokens.color.text.inverse,
              border: `1px solid ${category === item.label ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)'}`,
            }}
          />
        ))}
      </Stack>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' }, gap: 1.5 }}>
        {visible.map((item) => (
          <Surface key={item.title} sx={{ overflow: 'hidden' }}>
            <Box sx={{ height: { xs: 170, md: 190 }, bgcolor: item.bg, overflow: 'hidden' }}>
              <Box component="img" src={item.img} alt="" sx={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
            </Box>
            <Box sx={{ p: 1.8 }}>
              <Stack direction="row" justifyContent="space-between" alignItems="center" spacing={1}>
                <Typography variant="h6">{item.title}</Typography>
                <Chip label={item.type} size="small" sx={{ borderRadius: 1 }} />
              </Stack>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.6 }}>{item.desc}</Typography>
              <Button variant="contained" size="small" sx={{ mt: 1.5 }}>Use template</Button>
            </Box>
          </Surface>
        ))}
      </Box>
    </AppShell>
  );
}
