import { Box, Button, CircularProgress, Divider, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import { useNavigate, useLocation } from 'react-router-dom';
import { toast } from '@ironflyer/ui-web/fx';
import { useAuth, useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { LogoMark } from './LogoMark';
import { AccountMenu } from './AccountMenu';
import { useStudio } from '../store';
import { useThemeMode, neon } from '../theme';
import { mockProject, recentProjects, type StudioProject } from '../studioData';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { text } from '@ironflyer/design-tokens/brand';

interface RecentProject { id: string; name: string; status?: string | null; description?: string | null; idea?: string | null; project?: StudioProject }
interface ApiFile { path: string; content?: string | null }

function toneFor(status?: string | null): string {
  const s = (status ?? '').toLowerCase();
  if (s.includes('ship') || s.includes('done') || s.includes('complete')) return 'success.main';
  if (s.includes('error') || s.includes('block') || s.includes('fail')) return 'error.main';
  return 'warning.main';
}

function NavItem({ icon, label, active, onClick }: { icon: ReactNode; label: string; active?: boolean; onClick?: () => void }) {
  return (
    <Button
      onClick={onClick}
      fullWidth
      startIcon={icon}
      sx={(theme) => ({
        minHeight: 44,
        justifyContent: 'flex-start',
        gap: 0.75,
        px: 1.35,
        py: 0.85,
        borderRadius: `${theme.studio.radius.sm}px`,
        color: active ? theme.palette.text.primary : theme.palette.text.secondary,
        bgcolor: active ? theme.palette.surfaceHover : 'transparent',
        border: '1px solid',
        borderColor: active ? theme.palette.cardBorder : 'transparent',
        boxShadow: active ? '0 1px 2px rgba(24,22,20,0.04)' : 'none',
        fontWeight: active ? 700 : 600,
        '&:hover': { bgcolor: theme.palette.surfaceHover, color: theme.palette.text.primary, borderColor: theme.palette.cardBorder },
        '& .MuiButton-startIcon': { color: active ? theme.palette.text.primary : theme.palette.text.secondary, minWidth: 18 },
      })}
    >
      {label}
    </Button>
  );
}

// 16px stroke icons — no icon dep.
const I = (d: string) => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{<path d={d} />}</svg>
);
const icons = {
  home: I('M3 11l9-8 9 8M5 10v10h14V10'),
  apps: I('M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z'),
  templates: I('M4 5h16M4 12h10M4 19h7'),
  integrations: I('M9 7V4h6v3M7 7h10v5a5 5 0 01-10 0z M12 17v3'),
  agents: I('M12 8V4H8M4 8h16v12H4zM2 14h2M20 14h2M9 13v2M15 13v2'),
  search: I('M11 19a8 8 0 100-16 8 8 0 000 16zM21 21l-4.35-4.35'),
  community: I('M16 21v-2a4 4 0 00-4-4H6a4 4 0 00-4 4v2M9 11a4 4 0 100-8 4 4 0 000 8M22 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75'),
  gift: I('M20 12v10H4V12M22 7H2v5h20zM12 22V7M12 7H7.5a2.5 2.5 0 110-5C11 2 12 7 12 7zM12 7h4.5a2.5 2.5 0 100-5C13 2 12 7 12 7z'),
  bell: I('M18 8a6 6 0 10-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M13.73 21a2 2 0 01-3.46 0'),
  sun: I('M12 2v2M12 20v2M4.9 4.9l1.4 1.4M2 12h2M20 12h2M5 19l1.4-1.4M12 8a4 4 0 100 8 4 4 0 000-8z'),
  moon: I('M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z'),
  panel: I('M4 4h7v16H4zM13 4h7v16h-7z'),
  chevron: I('M6 9l6 6 6-6'),
};

function fallbackRecents(): RecentProject[] {
  return recentProjects.map(({ project }) => ({
    id: project.id,
    name: project.name,
    status: project.deploy.status === 'production' ? 'shipped' : project.gates.some((g) => g.status === 'blocked') ? 'blocked' : 'open',
    description: project.source,
    project,
  }));
}

function projectShell(p: RecentProject): StudioProject {
  return {
    ...mockProject,
    id: p.id,
    name: p.name,
    source: p.description || p.idea || 'saved project',
    deploy: { ...mockProject.deploy, status: p.status?.toLowerCase().includes('ship') ? 'production' : 'none' },
  };
}

export function AppSidebar({ onNewProject, newProjectBusy }: { onNewProject?: () => void | Promise<void>; newProjectBusy?: boolean }) {
  const { mode, toggle } = useThemeMode();
  const navigate = useNavigate();
  const { pathname, search } = useLocation();
  const { user, online } = useAuth();
  const request = useRequest();
  const openProject = useStudio((s) => s.openProject);
  const openLiveProject = useStudio((s) => s.openLiveProject);
  const [openingId, setOpeningId] = useState<string | null>(null);
  const go = (to: string) => navigate(to);
  const editorTab = new URLSearchParams(search).get('tab');

  // Recent projects mirror the real Projects query when an orchestrator is
  // connected; offline (sample mode) falls back to the bundled demo projects so
  // the rail is never empty.
  const { data: recent } = useGraphQLQuery<RecentProject[], { projects: RecentProject[] }>({
    key: ['projects'],
    operationName: 'Projects', query: operations.PROJECTS,
    fallbackData: [], map: (r) => r.projects ?? [],
  });
  const recentItems: RecentProject[] = recent.length ? recent.slice(0, 4) : fallbackRecents();
  const openRecent = async (p: RecentProject) => {
    if (p.project || !request) {
      openProject(p.project ?? projectShell(p));
      navigate('/build');
      return;
    }
    setOpeningId(p.id);
    try {
      const d = await request<{ projectFiles: ApiFile[] }>('ProjectFiles', operations.PROJECT_FILES, { id: p.id });
      const files = (d.projectFiles ?? [])
        .filter((f) => typeof f.content === 'string')
        .map((f) => ({ path: f.path, content: f.content as string }));
      openLiveProject(projectShell(p), p.id, files);
      navigate('/build');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not open project.', 'error');
    } finally {
      setOpeningId(null);
    }
  };
  return (
    <Box
      component="aside"
      sx={(theme) => ({
        width: { xs: 300, xl: 332 },
        flexShrink: 0,
        height: '100dvh',
        borderRight: `1px solid ${theme.palette.borderSubtle}`,
        bgcolor: 'background.paper',
        color: 'text.primary',
        display: { xs: 'none', md: 'flex' },
        flexDirection: 'column',
        p: 2,
        position: 'relative',
        overflow: 'hidden',
      })}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 0.25, mb: 1.7 }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <LogoMark size={26} />
        </Stack>
        <Stack direction="row" spacing={0.75}>
          <Tooltip title="Search" arrow>
            <IconButton
              size="small"
              onClick={() => { void onNewProject?.(); }}
              aria-label="Search"
              sx={(theme) => ({ color: 'text.primary', border: `1px solid transparent`, '&:hover': { bgcolor: 'surfaceHover', borderColor: theme.palette.divider } })}
            >
              {icons.search}
            </IconButton>
          </Tooltip>
          <Tooltip title="Collapse sidebar" arrow>
            <IconButton
              size="small"
              aria-label="Collapse sidebar"
              sx={(theme) => ({ color: 'text.primary', border: `1px solid transparent`, '&:hover': { bgcolor: 'surfaceHover', borderColor: theme.palette.divider } })}
            >
              {icons.panel}
            </IconButton>
          </Tooltip>
          <Tooltip title={mode === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'} arrow>
            <IconButton
              size="small"
              onClick={toggle}
              aria-label={mode === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
              sx={(theme) => ({ color: 'text.secondary', border: `1px solid ${theme.palette.divider}`, '&:hover': { bgcolor: 'surfaceHover' } })}
            >
              {mode === 'dark' ? icons.sun : icons.moon}
            </IconButton>
          </Tooltip>
        </Stack>
      </Stack>

      <Button
        fullWidth
        color="inherit"
        endIcon={icons.chevron}
        sx={(theme) => ({
          justifyContent: 'space-between',
          minHeight: 56,
          mb: 1.4,
          px: 1.25,
          borderRadius: `${theme.studio.radius.lg}px`,
          border: `1px solid ${theme.palette.cardBorder}`,
          bgcolor: 'background.paper',
          boxShadow: '0 1px 2px rgba(24,22,20,0.04)',
          '& .MuiButton-endIcon': { color: 'text.primary', ml: 1 },
          '&:hover': { bgcolor: 'surfaceHover' },
        })}
      >
        <Stack direction="row" alignItems="center" spacing={1} sx={{ minWidth: 0 }}>
          <Box
            sx={(theme) => ({
              width: 32,
              height: 32,
              borderRadius: `${theme.studio.radius.sm}px`,
              display: 'grid',
              placeItems: 'center',
              bgcolor: `${theme.palette.primary.main}24`,
              color: 'primary.main',
              fontWeight: 900,
              fontSize: text.s76,
              flexShrink: 0,
            })}
          >
            Mw
          </Box>
          <Box sx={{ minWidth: 0, textAlign: 'left' }}>
            <Typography sx={{ fontWeight: 900, fontSize: text.s92, lineHeight: 1.1 }} noWrap>Moshe's Workspace</Typography>
            <Typography sx={{ color: 'text.secondary', fontSize: text.s68, lineHeight: 1.2 }} noWrap>Ironflyer apps</Typography>
          </Box>
        </Stack>
      </Button>

      <Box sx={(theme) => ({ p: 0.55, border: `1px solid ${theme.palette.cardBorder}`, borderRadius: `${theme.studio.radius.lg}px`, mb: 1.15, bgcolor: 'background.paper' })}>
        <Stack spacing={0.25}>
          <Button fullWidth startIcon={icons.apps} sx={(theme) => ({ justifyContent: 'flex-start', bgcolor: theme.palette.surfaceHover, color: 'text.primary', minHeight: 40, borderRadius: `${theme.studio.radius.sm}px`, fontWeight: 900 })}>Apps</Button>
          <Button fullWidth startIcon={icons.agents} sx={(theme) => ({ justifyContent: 'flex-start', color: 'text.primary', minHeight: 40, borderRadius: `${theme.studio.radius.sm}px`, fontWeight: 800, '&:hover': { bgcolor: 'surfaceHover' } })}>Superagents</Button>
        </Stack>
      </Box>

      <Stack spacing={0.25}>
        <NavItem icon={newProjectBusy ? <CircularProgress color="inherit" size={12} /> : icons.search} label="Search" onClick={() => { void onNewProject?.(); }} />
        <NavItem icon={icons.home} label="Home" active={pathname === '/' || pathname === '/studio'} onClick={() => go('/')} />
        <NavItem icon={icons.apps} label="All apps" active={pathname === '/projects'} onClick={() => go('/projects')} />
        <NavItem icon={icons.templates} label="Templates" active={pathname === '/templates'} onClick={() => go('/templates')} />
        <NavItem icon={icons.integrations} label="Integrations" active={pathname === '/integrations'} onClick={() => go('/integrations')} />
        <NavItem icon={icons.community} label="Community" active={pathname === '/agents'} onClick={() => go('/agents')} />
        <NavItem icon={icons.integrations} label="Deployments" active={pathname === '/build' && editorTab === 'domains'} onClick={() => go('/build?tab=domains')} />
      </Stack>

      <Divider sx={{ my: 2, borderColor: 'borderSubtle' }} />
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 1.2, mb: 1 }}>
        <Typography sx={{ fontSize: text.s82, color: 'text.secondary', fontWeight: 600 }}>Favorite apps</Typography>
        <Typography sx={{ color: 'text.disabled' }}>⌄</Typography>
      </Stack>
      <Box sx={(theme) => ({
        border: `1px dashed ${theme.palette.divider}`,
        borderRadius: `${theme.studio.radius.sm}px`,
        p: 2,
        textAlign: 'center',
        color: 'text.secondary',
        fontSize: text.s78,
        mb: 2,
      })}>
        No favorite apps yet.<br />Add your apps for quick access
      </Box>

      <Typography sx={{ px: 1.2, fontSize: text.s82, color: 'text.secondary', fontWeight: 600 }}>Recents</Typography>
      <Stack spacing={0.25} sx={{ mt: 1 }}>
        {recentItems.map((p) => (
          <NavItem
            key={p.id}
            icon={openingId === p.id ? <CircularProgress size={12} /> : <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: toneFor(p.status) }} />}
            label={p.name}
            onClick={() => { void openRecent(p); }}
          />
        ))}
      </Stack>

      <Box sx={{ flex: 1 }} />

      <Box sx={(theme) => ({ p: 1.6, borderRadius: `${theme.studio.radius.lg}px`, border: `1px solid ${theme.palette.cardBorder}`, bgcolor: 'background.paper', boxShadow: '0 1px 2px rgba(24,22,20,0.04)', mb: 1.5 })}>
        <Stack direction="row" spacing={1} alignItems="center">
          <Box sx={{ color: neon.violet }}>{icons.gift}</Box>
          <Box sx={{ minWidth: 0 }}>
            <Typography sx={{ fontWeight: 800, fontSize: text.s88 }}>Upgrade your plan</Typography>
            <Typography sx={{ color: 'text.secondary', fontSize: text.s76 }}>Get more out of your apps</Typography>
          </Box>
        </Stack>
      </Box>

      <Stack direction="row" alignItems="center" spacing={1.25} sx={(theme) => ({ px: 0.25, py: 0.5, borderRadius: `${theme.studio.radius.sm}px` })}>
        <AccountMenu size={28} />
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography sx={{ fontSize: text.s85, fontWeight: 700 }} noWrap>{user?.email ?? 'Guest'}</Typography>
          <Typography sx={{ fontSize: text.s72, color: online ? (user ? 'success.main' : 'warning.main') : 'text.disabled' }} noWrap>
            {online ? (user ? `${user.plan ?? 'free'} · connected` : 'connected') : 'offline preview'}
          </Typography>
        </Box>
        <IconButton size="small" sx={{ color: 'text.secondary' }}>{icons.bell}</IconButton>
      </Stack>
    </Box>
  );
}
