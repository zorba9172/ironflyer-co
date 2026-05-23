'use client';

import { useEffect, useState } from 'react';
import dynamic from 'next/dynamic';
import Link from 'next/link';
import {
  AutoAwesome, ChatBubbleOutline, Code, Description, MenuBook, Timeline,
} from '@mui/icons-material';
import { Box, Button, Stack, Typography } from '@mui/material';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';
const TemplateGallery = dynamic(
  () => import('./TemplateGallery').then((m) => m.TemplateGallery),
  {
    ssr: false,
    loading: () => (
      <Surface sx={{ p: 4, mt: 1.5, textAlign: 'center' }}>
        <Typography variant="body2" color="text.secondary">Loading template gallery...</Typography>
      </Surface>
    ),
  },
);

interface ResourceLink {
  title: string;
  desc: string;
  href: string;
  external?: boolean;
  icon: React.ReactNode;
  cta: string;
}

const linkCards: ResourceLink[] = [
  {
    title: 'Docs',
    desc: 'Getting started guides, gate architecture, and provider routing details.',
    href: '/docs',
    icon: <MenuBook />,
    cta: 'Open docs',
  },
  {
    title: 'API Reference',
    desc: 'Orchestrator and runtime endpoints, including the @ironflyer/sdk client.',
    href: '/docs/api',
    icon: <Code />,
    cta: 'Open API',
  },
  {
    title: 'Template gallery',
    desc: 'Curated starter templates that can be pulled into the runtime directly from a prompt.',
    href: '/app/resources#templates',
    icon: <AutoAwesome />,
    cta: 'Explore templates',
  },
  {
    title: 'Status page',
    desc: 'Service availability, open incidents, and response times over the last 30 days.',
    href: 'https://status.ironflyer.dev',
    external: true,
    icon: <Timeline />,
    cta: 'Open status',
  },
  {
    title: 'Community and Discord',
    desc: 'Talk with other builders, share patches, and get fast help from the team.',
    href: 'https://discord.gg/ironflyer',
    external: true,
    icon: <ChatBubbleOutline />,
    cta: 'Join',
  },
  {
    title: 'Changelog',
    desc: 'New releases, newly added gates, and runtime performance improvements.',
    href: '/changelog',
    icon: <Description />,
    cta: 'Read changelog',
  },
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

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Resources"
        title="Everything you need to build"
        subtitle="Real templates, documentation, changelog, and community links in one place."
      />

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)' }, gap: 1.4, mb: 2.4 }}>
        {linkCards.map((card) => <LinkCard key={card.title} card={card} />)}
      </Box>

      <TemplateGallery query={query} view={view} />
    </AppShell>
  );
}

function LinkCard({ card }: { card: ResourceLink }) {
  const inner = (
    <Surface sx={{
      p: 2.2,
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
      '&:hover': { transform: 'translateY(-2px)', borderColor: 'rgba(17,17,17,0.28)' },
    }}>
      <Box sx={{
        width: 42, height: 42, borderRadius: '8px',
        bgcolor: '#fffaf1', border: '1px solid rgba(17,17,17,0.12)',
        display: 'grid', placeItems: 'center',
        color: tokens.color.text.inverse,
      }}>{card.icon}</Box>
      <Typography variant="h6" sx={{ mt: 1.4, fontWeight: 900 }}>{card.title}</Typography>
      <Typography variant="body2" sx={{ color: '#686158', mt: 0.4, flex: 1 }}>{card.desc}</Typography>
      <Button
        component="a"
        href={card.href}
        target={card.external ? '_blank' : undefined}
        rel={card.external ? 'noopener noreferrer' : undefined}
        size="small"
        variant="outlined"
        sx={{ alignSelf: 'flex-start', mt: 1.6 }}
      >
        {card.cta} {card.external ? '↗' : '→'}
      </Button>
    </Surface>
  );
  if (card.external) return inner;
  return <Link href={card.href} style={{ textDecoration: 'none', color: 'inherit' }}>{inner}</Link>;
}
