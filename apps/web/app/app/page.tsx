'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import {
  Add, Apps, Bolt, Folder, Home, Hub, Search, Settings, Star, ViewList, Window,
} from '@mui/icons-material';
import {
  Avatar, Box, Button, Chip, Divider, IconButton, InputAdornment, Stack,
  TextField, Tooltip, Typography,
} from '@mui/material';
import { api, Project } from '../../lib/api';
import { tokens } from '../../lib/theme';
import { RequireAuth, useAuth } from '../auth-context';
import { PromptBox } from '../prompt-box';
import { AppShell } from './workspace-shell';

const templateCards = [
  { title: 'SaaS dashboard', desc: 'Auth, billing, charts, admin settings', tag: 'Apps', img: '/marketplace/output-ref/hooked.png' },
  { title: 'Internal ops tool', desc: 'Approvals, roles, reports, audit trail', tag: 'Internal Tools', img: '/marketplace/output-ref/fx.png' },
  { title: 'Client portal', desc: 'Documents, messages, project status', tag: 'Portals', img: '/marketplace/output-ref/pack-generator.png' },
  { title: 'Launch site', desc: 'Hero, pricing, waitlist, CMS-ready pages', tag: 'Websites', img: '/marketplace/output-ref/gear.png' },
];

const navItems = [
  { label: 'Home', icon: <Home fontSize="small" /> },
  { label: 'Search', icon: <Search fontSize="small" /> },
  { label: 'Templates', icon: <Apps fontSize="small" /> },
  { label: 'Connectors', icon: <Hub fontSize="small" /> },
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
      <Stack spacing={3.5}>
            <Stack alignItems="center" spacing={2} sx={{ textAlign: 'center', pt: { xs: 0.5, md: 2 } }}>
              <Chip label="Ironflyer workspace" sx={{
                bgcolor: 'rgba(229,255,0,0.14)',
                color: '#6f7e00',
                border: '1px solid rgba(17,17,17,0.12)',
                borderRadius: 1,
                fontWeight: 900,
              }} />
              <Typography variant="h2" sx={{ maxWidth: 760, fontSize: { xs: '1.8rem', md: '3.15rem' }, lineHeight: 0.94, textTransform: 'uppercase', textWrap: 'balance' }}>
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
                      borderRadius: 1.5,
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
              <MetricCard label="Finisher gates" value="7" accent={tokens.color.accent.sky} />
              <MetricCard label="Agent roles" value="8" accent={tokens.color.accent.coral} />
            </Stack>

            <SectionHeader title="Start with a template" action={<Button component={Link} href="/app/resources" variant="outlined">Browse all</Button>} />
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(4, 1fr)' }, gap: 1.5 }}>
              {templateCards.map((item) => (
                <TemplateCard key={item.title} item={item} onUse={() => setIdea(`Build a ${item.title.toLowerCase()} with ${item.desc.toLowerCase()}.`)} />
              ))}
            </Box>

            <SectionHeader
              title="Your projects"
              action={<Typography variant="body2" color="text.secondary">{filteredProjects.length} visible</Typography>}
            />
            <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap">
              {statusFilters.map((status) => (
                <Chip
                  key={status}
                  label={status}
                  onClick={() => setStatusFilter(status)}
                  sx={{
                    borderRadius: 1.5,
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
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.5 }}>
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

function Sidebar({ userEmail, recents, onLogout }: { userEmail: string; recents: Project[]; onLogout: () => void }) {
  return (
    <Box component="aside" sx={{
      display: { xs: 'none', lg: 'flex' },
      flexDirection: 'column',
      height: '100vh',
      position: 'sticky',
      top: 0,
      borderRight: `1px solid ${tokens.color.border.subtle}`,
      bgcolor: tokens.color.bg.inset,
      p: 2,
    }}>
      <Link href="/" style={{ color: 'inherit', textDecoration: 'none' }}>
        <Stack direction="row" spacing={1.4} alignItems="center" sx={{ px: 1, py: 1 }}>
          <Box sx={{ width: 28, height: 28, borderRadius: 1, bgcolor: tokens.color.accent.lime, boxShadow: `0 0 24px ${tokens.color.accent.lime}` }} />
          <Typography variant="h6" sx={{ fontFamily: tokens.font.display, fontWeight: 400, textTransform: 'uppercase' }}>Ironflyer</Typography>
        </Stack>
      </Link>

      <Button
        fullWidth
        startIcon={<Add />}
        sx={{ justifyContent: 'flex-start', mt: 2, bgcolor: tokens.color.bg.surfaceRaised, color: tokens.color.text.primary }}
      >
        New project
      </Button>

      <Stack spacing={0.5} sx={{ mt: 3 }}>
        {navItems.map((item, index) => (
          <Button
            key={item.label}
            startIcon={item.icon}
            sx={{
              justifyContent: 'flex-start',
              color: index === 0 ? tokens.color.text.primary : tokens.color.text.secondary,
              bgcolor: index === 0 ? tokens.color.bg.surfaceHover : 'transparent',
              borderRadius: 1.5,
            }}
          >
            {item.label}
          </Button>
        ))}
      </Stack>

      <Divider sx={{ my: 2.5 }} />
      <Typography variant="overline" color="text.secondary">Projects</Typography>
      <Stack spacing={0.5} sx={{ mt: 1 }}>
        {['All projects', 'Starred', 'Created by me', 'Shared with me'].map((label, index) => (
          <Button
            key={label}
            startIcon={index === 1 ? <Star fontSize="small" /> : <Folder fontSize="small" />}
            sx={{ justifyContent: 'flex-start', color: tokens.color.text.secondary, borderRadius: 1.5 }}
          >
            {label}
          </Button>
        ))}
      </Stack>

      <Divider sx={{ my: 2.5 }} />
      <Typography variant="overline" color="text.secondary">Recents</Typography>
      <Stack spacing={0.75} sx={{ mt: 1, minHeight: 0, overflow: 'auto' }}>
        {recents.length === 0 && <Typography variant="caption" color="text.secondary">No recent projects yet</Typography>}
        {recents.map((project) => (
          <Link key={project.id} href={`/projects/${project.id}`} style={{ color: 'inherit', textDecoration: 'none' }}>
            <Typography variant="body2" noWrap sx={{ color: tokens.color.text.secondary, py: 0.5 }}>
              {project.name}
            </Typography>
          </Link>
        ))}
      </Stack>

      <Box sx={{
        mt: 'auto',
        p: 2,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 2,
        bgcolor: tokens.color.bg.surface,
      }}>
        <Typography variant="subtitle2">Upgrade credits</Typography>
        <Typography variant="caption" color="text.secondary">More runs, shared gates, and private workspaces.</Typography>
        <Button fullWidth variant="contained" size="small" sx={{ mt: 1.5 }}>View plans</Button>
      </Box>

      <Stack direction="row" spacing={1.2} alignItems="center" sx={{ mt: 2 }}>
        <Avatar sx={{ width: 34, height: 34, bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse, fontWeight: 900 }}>
          {userEmail[0]?.toUpperCase()}
        </Avatar>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="body2" noWrap>{userEmail}</Typography>
          <Typography variant="caption" color="text.secondary">Free workspace</Typography>
        </Box>
        <Tooltip title="Settings">
          <IconButton size="small" sx={{ color: 'text.secondary' }}><Settings fontSize="small" /></IconButton>
        </Tooltip>
      </Stack>
      <Button onClick={onLogout} sx={{ mt: 1, color: tokens.color.text.secondary }}>Sign out</Button>
    </Box>
  );
}

function TopBar({
  query, setQuery, view, setView,
}: {
  query: string;
  setQuery: (value: string) => void;
  view: 'grid' | 'list';
  setView: (value: 'grid' | 'list') => void;
}) {
  return (
    <Box sx={{
      minHeight: 68,
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      px: { xs: 2, md: 3 },
      borderBottom: `1px solid ${tokens.color.border.subtle}`,
      bgcolor: 'rgba(13,14,15,0.82)',
      backdropFilter: 'blur(14px)',
      position: 'sticky',
      top: 0,
      zIndex: 9,
    }}>
      <Stack direction="row" spacing={1.3} alignItems="center" sx={{ display: { xs: 'flex', lg: 'none' }, minWidth: 0 }}>
        <Box sx={{ width: 28, height: 28, borderRadius: 1, bgcolor: tokens.color.accent.lime }} />
        <Typography variant="h6" sx={{ fontFamily: tokens.font.display, fontWeight: 400, textTransform: 'uppercase' }}>Ironflyer</Typography>
      </Stack>
      <TextField
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="Search projects, prompts, gates..."
        size="small"
        sx={{ display: { xs: 'none', sm: 'block' }, flex: 1, maxWidth: { sm: 360, lg: 560 }, ml: { xs: 'auto', lg: 0 } }}
        InputProps={{
          startAdornment: (
            <InputAdornment position="start">
              <Search fontSize="small" />
            </InputAdornment>
          ),
        }}
      />
      <Stack direction="row" spacing={0.75}>
        <IconButton onClick={() => setView('grid')} sx={{ color: view === 'grid' ? tokens.color.accent.lime : tokens.color.text.secondary }}>
          <Window fontSize="small" />
        </IconButton>
        <IconButton onClick={() => setView('list')} sx={{ color: view === 'list' ? tokens.color.accent.lime : tokens.color.text.secondary }}>
          <ViewList fontSize="small" />
        </IconButton>
      </Stack>
    </Box>
  );
}

function MetricCard({ label, value, accent }: { label: string; value: string; accent: string }) {
  return (
    <Box sx={{
      flex: 1,
      p: { xs: 2.2, md: 3 },
      border: '1px solid rgba(17,17,17,0.12)',
      borderRadius: { xs: 2.2, md: 3 },
      bgcolor: '#f8f4ec',
      color: tokens.color.text.inverse,
      transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
      '&:hover': { transform: 'translateY(-2px)', borderColor: 'rgba(17,17,17,0.28)' },
    }}>
      <Typography variant="overline" sx={{ color: '#716a60' }}>{label}</Typography>
      <Typography variant="h2" sx={{ color: accent, lineHeight: 0.95 }}>{value}</Typography>
    </Box>
  );
}

function SectionHeader({ title, action }: { title: string; action?: React.ReactNode }) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="center" spacing={2}>
      <Typography variant="h4" sx={{ color: tokens.color.text.inverse, fontSize: { xs: '1.65rem', md: '2.2rem' } }}>{title}</Typography>
      {action}
    </Stack>
  );
}

function TemplateCard({ item, onUse }: { item: typeof templateCards[number]; onUse: () => void }) {
  return (
    <Box sx={{
      border: '1px solid rgba(17,17,17,0.12)',
      borderRadius: { xs: 2.2, md: 3 },
      bgcolor: '#f8f4ec',
      color: tokens.color.text.inverse,
      overflow: 'hidden',
      transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
      '&:hover': { transform: 'translateY(-3px)', borderColor: 'rgba(17,17,17,0.28)' },
    }}>
      <Box component="img" src={item.img} alt="" sx={{ width: '100%', height: 150, objectFit: 'cover', display: 'block' }} />
      <Box sx={{ p: 2 }}>
        <Stack direction="row" justifyContent="space-between" spacing={1}>
          <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>{item.title}</Typography>
          <Chip label={item.tag} size="small" sx={{ borderRadius: 1, bgcolor: 'rgba(17,17,17,0.12)', color: tokens.color.text.inverse, fontWeight: 800 }} />
        </Stack>
        <Typography variant="body2" sx={{ mt: 0.75, minHeight: 42, color: '#686158' }}>{item.desc}</Typography>
        <Button onClick={onUse} startIcon={<Bolt />} sx={{ mt: 1, px: 0, color: tokens.color.text.inverse }}>Use blueprint</Button>
      </Box>
    </Box>
  );
}

function EmptyProjects({ query }: { query: string }) {
  return (
    <Box sx={{
      p: 5,
      textAlign: 'center',
      border: '1px solid rgba(17,17,17,0.12)',
      borderRadius: 3,
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
        borderRadius: 3,
        bgcolor: '#f8f4ec',
        color: tokens.color.text.inverse,
        '&:hover': { borderColor: 'rgba(17,17,17,0.28)', bgcolor: '#fffaf1' },
      }}>
        <Stack direction="row" justifyContent="space-between" spacing={1}>
          <Typography variant="h6" noWrap>{project.name}</Typography>
          <Chip label={project.status} size="small" sx={{ borderRadius: 1, bgcolor: 'rgba(17,17,17,0.12)', color: tokens.color.text.inverse, fontWeight: 800 }} />
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
        <Box sx={{ mt: 3, height: 7, borderRadius: 1, bgcolor: tokens.color.bg.inset }}>
          <Box sx={{ width: `${(passed / Math.max(total, 1)) * 100}%`, height: '100%', borderRadius: 1, bgcolor: tokens.color.accent.lime }} />
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
      <Stack direction="row" alignItems="center" spacing={2} sx={{
        p: 1.5,
        border: '1px solid rgba(17,17,17,0.12)',
        borderRadius: 2,
        bgcolor: '#f8f4ec',
        color: tokens.color.text.inverse,
      }}>
        <Box sx={{ width: 42, height: 42, borderRadius: 1, bgcolor: tokens.color.bg.surfaceHover, display: 'grid', placeItems: 'center' }}>
          <Apps fontSize="small" />
        </Box>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="subtitle1" noWrap>{project.name}</Typography>
          <Typography variant="caption" color="text.secondary" noWrap>{project.description || project.spec.idea}</Typography>
        </Box>
        <Chip label={`${passed}/${total} gates`} size="small" sx={{ borderRadius: 1 }} />
        <Typography variant="caption" color="text.secondary">{new Date(project.updatedAt).toLocaleDateString()}</Typography>
      </Stack>
    </Link>
  );
}
