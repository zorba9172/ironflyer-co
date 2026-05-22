'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { Apps, Bolt, CheckCircle, PriceCheck, TrendingUp } from '@mui/icons-material';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { A11y, Keyboard, Navigation, Pagination } from 'swiper/modules';
import { Swiper, SwiperSlide } from 'swiper/react';
import { api, Project } from '../../lib/api';
import { tokens } from '../../lib/theme';
import { RequireAuth, useAuth } from '../auth-context';
import { PromptBox } from '../prompt-box';
import { AppShell } from './workspace-shell';

const templateCards = [
  {
    title: 'Aiforge AI SaaS',
    desc: 'AI landing, integrations, pricing, blog, onboarding',
    tag: 'Apps',
    img: '/templates/aiforge-hero.jpg',
    source: 'templates/aiforge',
    prompt: 'Use the local Aiforge template as the visual foundation for an AI SaaS app with landing pages, integrations, pricing, blog, onboarding, and production launch checks.',
  },
  {
    title: 'Allstore Commerce',
    desc: 'Catalog, sale hero, product grids, checkout-ready flows',
    tag: 'Commerce',
    img: '/templates/allstore-slide.jpg',
    source: 'templates/allstore-html-template/html',
    prompt: 'Use the local Allstore HTML template as the foundation for a commerce storefront with catalog pages, product detail, cart, checkout, order states, and CMS-ready content.',
  },
  {
    title: 'Davies Agency System',
    desc: 'Portfolio demos, services, process, pricing, contact flow',
    tag: 'Websites',
    img: '/templates/davies-demo.jpg',
    source: 'templates/davies-mainfiles/davies',
    prompt: 'Use the local Davies template as the base for a premium agency website with portfolio demos, service sections, process, pricing, contact flows, analytics, and SEO checks.',
  },
  {
    title: 'Codec Mobile Kit',
    desc: 'Mobile-first screens, app navigation, onboarding, PWA',
    tag: 'Mobile/PWA',
    img: '/templates/codec-mobile.png',
    source: 'templates/codec-mobile/codec',
    prompt: 'Use the local Codec mobile template as the foundation for a mobile-first PWA with onboarding, app navigation, profile screens, push-ready flows, and touch ergonomics.',
  },
  {
    title: 'Blix Portfolio Mobile',
    desc: 'Swipe-first portfolio shell with themed variants',
    tag: 'Mobile/PWA',
    img: '/templates/blix-mobile.png',
    source: 'templates/blix/blix',
    prompt: 'Use the local Blix mobile template as the foundation for a mobile portfolio PWA with themed variants, project pages, contact flows, and CMS-ready portfolio content.',
  },
];

const quickPrompts = [
  'Build a full-stack SaaS with auth, billing, teams, and admin settings.',
  'Plan an internal tool for operations with approvals, roles, and reports.',
  'Create a landing page and waitlist for a new AI product.',
  'Turn a rough product idea into a spec, UX plan, and implementation roadmap.',
];

const statusFilters = ['All', 'Ready', 'Running', 'Pending', 'Failed'];

export default function AppHome() {
  return (
    <RequireAuth>
      <AppHomeInner />
    </RequireAuth>
  );
}

function AppHomeInner() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [idea, setIdea] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'grid' | 'list'>('grid');
  const [statusFilter, setStatusFilter] = useState('All');

  useEffect(() => {
    void refresh();
    const pending = window.localStorage.getItem('ironflyer.pendingIdea');
    if (pending) {
      setIdea(pending);
      window.localStorage.removeItem('ironflyer.pendingIdea');
    }
  }, []);

  async function refresh() {
    try {
      setProjects(await api.listProjects());
    } catch (e) {
      setError(String(e));
    }
  }

  async function createFromIdea(nextIdea = idea) {
    if (!nextIdea.trim()) return;
    setBusy(true); setError(null);
    try {
      const name = nextIdea.split('\n')[0].slice(0, 60);
      const p = await api.createProject({ name, description: 'Created from prompt', idea: nextIdea });
      setIdea('');
      await refresh();
      window.location.href = `/projects/${p.id}`;
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  const filteredProjects = useMemo(() => {
    const q = query.trim().toLowerCase();
    return projects.filter((project) => {
      if (statusFilter !== 'All' && project.status.toLowerCase() !== statusFilter.toLowerCase()) {
        return false;
      }
      if (!q) return true;
      const haystack = `${project.name} ${project.description} ${project.spec.idea} ${project.status}`.toLowerCase();
      return haystack.includes(q);
    });
  }, [projects, query, statusFilter]);

  const recents = projects.slice(0, 5);
  const runningProjects = projects.filter((project) => project.status.toLowerCase() === 'running').length;

  return (
    <AppShell
      userEmail={user?.email ?? 'workspace'}
      recents={recents}
      onLogout={logout}
      query={query}
      setQuery={setQuery}
      view={view}
      setView={setView}
    >
      <Stack spacing={{ xs: 2.6, md: 3 }}>
            <Stack alignItems="center" spacing={2} sx={{
              textAlign: 'center',
              pt: { xs: 0.4, md: 1.2 },
              pb: { xs: 0.2, md: 0.4 },
            }}>
              <Chip label="Ironflyer workspace" sx={{
                bgcolor: 'rgba(229,255,0,0.14)',
                color: '#6f7e00',
                border: '1px solid rgba(17,17,17,0.12)',
                borderRadius: '8px',
                fontWeight: 900,
              }} />
              <Typography variant="h2" sx={{
                maxWidth: 760,
                fontSize: { xs: '1.75rem', sm: '2.25rem', md: '2.7rem' },
                lineHeight: 0.98,
                textTransform: 'uppercase',
                textWrap: 'balance',
              }}>
                What do you want to build?
              </Typography>
              <Typography variant="body1" sx={{ maxWidth: 620, fontWeight: 500, color: '#686158' }}>
                Start from a prompt, attach context, choose Build or Plan, and Ironflyer will create the project workspace.
              </Typography>
              <Box sx={{ width: '100%', maxWidth: 860 }}>
                <PromptBox
                  value={idea}
                  onChange={setIdea}
                  onSubmit={(value) => void createFromIdea(value)}
                  busy={busy}
                  error={error}
                  size="dashboard"
                  cta="Build"
                  placeholder="Ask Ironflyer to build a SaaS app, internal tool, dashboard, portal, or website..."
                />
              </Box>
              <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap" justifyContent="center" sx={{ width: '100%', maxWidth: 940, overflow: 'hidden' }}>
                {quickPrompts.map((prompt) => (
                  <Chip
                    key={prompt}
                    label={prompt}
                    onClick={() => setIdea(prompt)}
                    sx={{
                      width: { xs: '100%', sm: 'auto' },
                      maxWidth: { xs: '100%', sm: 420 },
                      borderRadius: '8px',
                      bgcolor: '#fffaf1',
                      color: '#4a453e',
                      border: '1px solid rgba(17,17,17,0.12)',
                      '& .MuiChip-label': {
                        display: 'block',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                      },
                      '&:hover': {
                        bgcolor: 'rgba(229,255,0,0.36)',
                        color: tokens.color.text.inverse,
                      },
                    }}
                  />
                ))}
              </Stack>
            </Stack>

            <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.2}>
              <MetricCard label="Projects" value={projects.length.toString()} accent={tokens.color.accent.lime} />
              <MetricCard label="Running now" value={runningProjects.toString()} accent={tokens.color.accent.sky} />
              <MetricCard label="Agent roles" value="8" accent={tokens.color.accent.coral} />
            </Stack>

            <RevenueReadinessPanel />

            <SectionHeader title="Start with a template" action={<Button component={Link} href="/app/resources" variant="outlined">Browse all</Button>} />
            <Box sx={homeSwiperSx}>
              <Swiper
                modules={[Navigation, Pagination, Keyboard, A11y]}
                navigation
                pagination={{ clickable: true }}
                keyboard={{ enabled: true }}
                spaceBetween={12}
                slidesPerView={1.05}
                breakpoints={{
                  720: { slidesPerView: 2.15, spaceBetween: 14 },
                  1120: { slidesPerView: 3.15, spaceBetween: 14 },
                }}
              >
                {templateCards.map((item) => (
                  <SwiperSlide key={item.title}>
                    <TemplateCard item={item} onUse={() => setIdea(item.prompt)} />
                  </SwiperSlide>
                ))}
              </Swiper>
            </Box>

            <SectionHeader
              title="Your projects"
              action={<Typography variant="body2" color="text.secondary">{filteredProjects.length} visible</Typography>}
            />
            <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap">
              {statusFilters.map((status) => (
                <Chip
                  key={status}
                  label={status}
                  onClick={() => setStatusFilter(status)}
                  sx={{
                    borderRadius: '8px',
                    bgcolor: statusFilter === status ? tokens.color.accent.lime : tokens.color.bg.surface,
                    color: statusFilter === status ? tokens.color.text.inverse : tokens.color.text.secondary,
                    border: `1px solid ${statusFilter === status ? tokens.color.accent.lime : tokens.color.border.subtle}`,
                    fontWeight: 900,
                  }}
                />
              ))}
            </Stack>
            {filteredProjects.length === 0 ? (
              <EmptyProjects query={query} />
            ) : view === 'grid' ? (
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.3 }}>
                {filteredProjects.map((project) => <ProjectGridCard key={project.id} project={project} />)}
              </Box>
            ) : (
              <Stack spacing={1}>
                {filteredProjects.map((project) => <ProjectListRow key={project.id} project={project} />)}
              </Stack>
            )}
          </Stack>
    </AppShell>
  );
}

function MetricCard({ label, value, accent }: { label: string; value: string; accent: string }) {
  return (
    <Box sx={{
      flex: 1,
      p: { xs: 1.7, md: 2.2 },
      border: '1px solid rgba(17,17,17,0.12)',
      borderRadius: '8px',
      bgcolor: '#f8f4ec',
      color: tokens.color.text.inverse,
      transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
      '&:hover': { transform: 'translateY(-2px)', borderColor: 'rgba(17,17,17,0.28)' },
    }}>
      <Typography variant="overline" sx={{ color: '#716a60' }}>{label}</Typography>
      <Typography variant="h2" sx={{ color: accent, lineHeight: 0.95, fontSize: { xs: '2.35rem', md: '2.8rem' } }}>{value}</Typography>
    </Box>
  );
}

function SectionHeader({ title, action }: { title: string; action?: React.ReactNode }) {
  return (
    <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" alignItems={{ xs: 'flex-start', sm: 'center' }} spacing={1}>
      <Typography variant="h4" sx={{ color: tokens.color.text.inverse, fontSize: { xs: '1.65rem', md: '2.2rem' } }}>{title}</Typography>
      {action}
    </Stack>
  );
}

function TemplateCard({ item, onUse }: { item: typeof templateCards[number]; onUse: () => void }) {
  return (
    <Box sx={{
      height: '100%',
      border: '1px solid rgba(17,17,17,0.12)',
      borderRadius: '8px',
      bgcolor: '#f8f4ec',
      color: tokens.color.text.inverse,
      overflow: 'hidden',
      display: 'flex',
      flexDirection: 'column',
      transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
      '&:hover': { transform: 'translateY(-3px)', borderColor: 'rgba(17,17,17,0.28)' },
    }}>
      <Box component="img" src={item.img} alt="" sx={{ width: '100%', height: { xs: 170, md: 154 }, objectFit: 'cover', display: 'block', bgcolor: '#111' }} />
      <Box sx={{ p: 2, display: 'flex', flexDirection: 'column', flex: 1 }}>
        <Stack direction="row" justifyContent="space-between" spacing={1}>
          <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>{item.title}</Typography>
          <Chip label={item.tag} size="small" sx={{ borderRadius: '6px', bgcolor: 'rgba(17,17,17,0.12)', color: tokens.color.text.inverse, fontWeight: 800 }} />
        </Stack>
        <Typography variant="body2" sx={{ mt: 0.75, minHeight: 42, color: '#686158' }}>{item.desc}</Typography>
        <Typography variant="caption" sx={{ mt: 0.9, color: '#686158', wordBreak: 'break-word' }}>Source: {item.source}</Typography>
        <Button onClick={onUse} startIcon={<Bolt />} sx={{ mt: 'auto', px: 0, color: tokens.color.text.inverse, alignSelf: 'flex-start' }}>Use blueprint</Button>
      </Box>
    </Box>
  );
}

function RevenueReadinessPanel() {
  const items = [
    { icon: <TrendingUp fontSize="small" />, title: 'Revenue path', text: 'Free builders convert into Pro, teams expand seats, Enterprise adds SSO and procurement.' },
    { icon: <PriceCheck fontSize="small" />, title: 'Cost control', text: 'Credits, gates, and budget visibility make paid AI usage easier to trust.' },
    { icon: <CheckCircle fontSize="small" />, title: 'Launch proof', text: 'Templates start with production checks so projects feel closer to paid outcomes.' },
  ];

  return (
    <Box sx={{
      p: { xs: 1.5, md: 1.8 },
      border: '1px solid rgba(17,17,17,0.12)',
      borderRadius: '8px',
      bgcolor: '#111',
      color: tokens.color.bg.alabaster,
    }}>
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.6} alignItems={{ xs: 'stretch', md: 'center' }}>
        <Box sx={{ minWidth: { md: 250 } }}>
          <Typography variant="overline" sx={{ color: tokens.color.accent.lime }}>Revenue engine</Typography>
          <Typography variant="h5" sx={{ mt: 0.3, lineHeight: 1.05 }}>Build the app around paid outcomes.</Typography>
        </Box>
        <Box sx={{ flex: 1, display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1 }}>
          {items.map((item) => (
            <Stack key={item.title} direction="row" spacing={1} sx={{ p: 1, borderRadius: '8px', bgcolor: 'rgba(244,240,232,0.07)' }}>
              <Box sx={{ width: 30, height: 30, flex: '0 0 auto', borderRadius: '8px', display: 'grid', placeItems: 'center', bgcolor: 'rgba(229,255,0,0.14)', color: tokens.color.accent.lime }}>
                {item.icon}
              </Box>
              <Box>
                <Typography variant="subtitle2" sx={{ color: tokens.color.bg.alabaster }}>{item.title}</Typography>
                <Typography variant="caption" sx={{ color: '#c9c0b0' }}>{item.text}</Typography>
              </Box>
            </Stack>
          ))}
        </Box>
        <Stack spacing={0.8} sx={{ minWidth: { md: 136 } }}>
          <Button component={Link} href="/pricing" variant="contained" size="small">View plans</Button>
          <Button component={Link} href="/app/resources" variant="outlined" size="small" sx={{ color: tokens.color.bg.alabaster, borderColor: 'rgba(244,240,232,0.28)' }}>Templates</Button>
        </Stack>
      </Stack>
    </Box>
  );
}

function EmptyProjects({ query }: { query: string }) {
  return (
    <Box sx={{
      p: 5,
      textAlign: 'center',
      border: '1px solid rgba(17,17,17,0.12)',
      borderRadius: '8px',
      bgcolor: '#f8f4ec',
      color: tokens.color.text.inverse,
    }}>
      <Typography variant="h5">{query ? 'No projects match that search' : 'No projects yet'}</Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
        {query ? 'Clear the search or create a new project from the prompt above.' : 'Describe what you want to build and Ironflyer will create the workspace.'}
      </Typography>
    </Box>
  );
}

function ProjectGridCard({ project }: { project: Project }) {
  const passed = Object.values(project.gates).filter((gate) => gate.status === 'passed').length;
  const total = Object.keys(project.gates).length;
  return (
    <Link href={`/projects/${project.id}`} style={{ color: 'inherit', textDecoration: 'none' }}>
      <Box sx={{
        p: 2.5,
        minHeight: 190,
        border: '1px solid rgba(17,17,17,0.12)',
        borderRadius: '8px',
        bgcolor: '#f8f4ec',
        color: tokens.color.text.inverse,
        transition: `background-color ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}, transform ${tokens.motion.base} ${tokens.motion.curve}`,
        '&:hover': { borderColor: 'rgba(17,17,17,0.28)', bgcolor: '#fffaf1' },
      }}>
        <Stack direction="row" justifyContent="space-between" spacing={1}>
          <Typography variant="h6" noWrap>{project.name}</Typography>
          <Chip label={project.status} size="small" sx={{ borderRadius: '6px', bgcolor: 'rgba(17,17,17,0.12)', color: tokens.color.text.inverse, fontWeight: 800 }} />
        </Stack>
        <Typography variant="body2" sx={{
          mt: 1,
          color: '#686158',
          display: '-webkit-box',
          WebkitLineClamp: 3,
          WebkitBoxOrient: 'vertical',
          overflow: 'hidden',
        }}>
          {project.description || project.spec.idea || 'No description'}
        </Typography>
        <Box sx={{ mt: 3, height: 7, borderRadius: '999px', bgcolor: tokens.color.bg.inset }}>
          <Box sx={{ width: `${(passed / Math.max(total, 1)) * 100}%`, height: '100%', borderRadius: '999px', bgcolor: tokens.color.accent.lime }} />
        </Box>
        <Stack direction="row" justifyContent="space-between" sx={{ mt: 1 }}>
          <Typography variant="caption" sx={{ color: '#686158' }}>Gates {passed}/{total}</Typography>
          <Typography variant="caption" sx={{ color: '#686158' }}>{new Date(project.updatedAt).toLocaleDateString()}</Typography>
        </Stack>
      </Box>
    </Link>
  );
}

function ProjectListRow({ project }: { project: Project }) {
  const passed = Object.values(project.gates).filter((gate) => gate.status === 'passed').length;
  const total = Object.keys(project.gates).length;
  return (
    <Link href={`/projects/${project.id}`} style={{ color: 'inherit', textDecoration: 'none' }}>
      <Stack direction={{ xs: 'column', sm: 'row' }} alignItems={{ xs: 'stretch', sm: 'center' }} spacing={1.2} sx={{
        p: 1.5,
        border: '1px solid rgba(17,17,17,0.12)',
        borderRadius: '8px',
        bgcolor: '#f8f4ec',
        color: tokens.color.text.inverse,
        transition: `background-color ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
        '&:hover': { borderColor: 'rgba(17,17,17,0.28)', bgcolor: '#fffaf1' },
      }}>
        <Stack direction="row" spacing={1.2} alignItems="center" sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ width: 42, height: 42, flex: '0 0 auto', borderRadius: '8px', bgcolor: tokens.color.bg.surfaceHover, display: 'grid', placeItems: 'center' }}>
            <Apps fontSize="small" />
          </Box>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography variant="subtitle1" noWrap>{project.name}</Typography>
            <Typography variant="caption" color="text.secondary" noWrap>{project.description || project.spec.idea}</Typography>
          </Box>
        </Stack>
        <Stack direction="row" spacing={1} alignItems="center" justifyContent={{ xs: 'space-between', sm: 'flex-end' }}>
          <Chip label={`${passed}/${total} gates`} size="small" sx={{ borderRadius: '6px' }} />
          <Typography variant="caption" color="text.secondary">{new Date(project.updatedAt).toLocaleDateString()}</Typography>
        </Stack>
      </Stack>
    </Link>
  );
}

const homeSwiperSx = {
  mx: { xs: -0.2, md: 0 },
  '& .swiper': {
    pb: 4.4,
    px: { xs: 0.2, md: 0.1 },
  },
  '& .swiper-slide': {
    height: 'auto',
    display: 'flex',
  },
  '& .swiper-button-prev, & .swiper-button-next': {
    width: 32,
    height: 32,
    top: { xs: 100, md: 92 },
    display: { xs: 'none', md: 'flex' },
    borderRadius: '8px',
    bgcolor: '#111',
    color: tokens.color.accent.lime,
    boxShadow: '0 10px 28px rgba(17,17,17,0.22)',
    '&:after': { fontSize: 13, fontWeight: 900 },
    '&.swiper-button-disabled': { opacity: 0, pointerEvents: 'none' },
  },
  '& .swiper-pagination-bullet': {
    width: 8,
    height: 8,
    bgcolor: 'rgba(17,17,17,0.35)',
    opacity: 1,
  },
  '& .swiper-pagination-bullet-active': {
    width: 24,
    borderRadius: '999px',
    bgcolor: tokens.color.accent.lime,
  },
};
