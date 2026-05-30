import { Box, Button, CircularProgress, Divider, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import { useNavigate, useLocation } from 'react-router-dom';
import { toast } from '@ironflyer/ui-web/fx';
import { useAuth, useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { LogoMark } from './LogoMark';
import { AccountMenu } from './AccountMenu';
import { Icon, type IconName } from '../icons';
import { useStudio } from '../store';
import { useThemeMode } from '../theme';
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
      disableRipple
      sx={(theme) => ({
        minHeight: 42,
        justifyContent: 'flex-start',
        gap: 0.75,
        px: 1.35,
        py: 0.8,
        borderRadius: `${theme.studio.radius.sm}px`,
        // Active nav reads in the signature indigo: tinted fill + indigo ink.
        color: active ? theme.palette.primary.main : theme.palette.text.secondary,
        bgcolor: active ? `${theme.palette.primary.main}14` : 'transparent',
        fontWeight: active ? theme.typography.fontWeightBold : theme.typography.fontWeightMedium,
        transition: `background-color ${theme.studio.motion.fast}, color ${theme.studio.motion.fast}`,
        '&:hover': {
          bgcolor: active ? `${theme.palette.primary.main}1F` : theme.palette.surfaceHover,
          color: active ? theme.palette.primary.main : theme.palette.text.primary,
        },
        '& .MuiButton-startIcon': {
          color: active ? theme.palette.primary.main : theme.palette.text.secondary,
          minWidth: 18,
        },
      })}
    >
      {label}
    </Button>
  );
}

// Sidebar glyphs route through the one Icon barrel (Lucide) — no inline SVG.
const glyph = (name: IconName) => <Icon name={name} size={17} strokeWidth={1.8} />;

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
              sx={(theme) => ({ color: 'text.secondary', border: `1px solid transparent`, '&:hover': { bgcolor: 'surfaceHover', color: 'text.primary', borderColor: theme.palette.divider } })}
            >
              <Icon name="search" size={17} />
            </IconButton>
          </Tooltip>
          <Tooltip title="Collapse sidebar" arrow>
            <IconButton
              size="small"
              aria-label="Collapse sidebar"
              sx={(theme) => ({ color: 'text.secondary', border: `1px solid transparent`, '&:hover': { bgcolor: 'surfaceHover', color: 'text.primary', borderColor: theme.palette.divider } })}
            >
              <Icon name="collapse" size={17} />
            </IconButton>
          </Tooltip>
          <Tooltip title={mode === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'} arrow>
            <IconButton
              size="small"
              onClick={toggle}
              aria-label={mode === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
              sx={(theme) => ({ color: 'text.secondary', border: `1px solid ${theme.palette.divider}`, '&:hover': { bgcolor: 'surfaceHover', color: 'text.primary' } })}
            >
              <Icon name={mode === 'dark' ? 'sun' : 'moon'} size={17} />
            </IconButton>
          </Tooltip>
        </Stack>
      </Stack>

      <Button
        fullWidth
        color="inherit"
        endIcon={<Icon name="chevronDown" size={16} />}
        sx={(theme) => ({
          justifyContent: 'space-between',
          minHeight: 56,
          mb: 1.4,
          px: 1.25,
          borderRadius: `${theme.studio.radius.lg}px`,
          border: `1px solid ${theme.palette.cardBorder}`,
          bgcolor: 'background.paper',
          boxShadow: 1,
          '& .MuiButton-endIcon': { color: 'text.secondary', ml: 1 },
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
              bgcolor: `${theme.palette.primary.main}1F`,
              color: 'primary.main',
              fontWeight: theme.typography.fontWeightBold,
              fontSize: text.s76,
              flexShrink: 0,
            })}
          >
            Mw
          </Box>
          <Box sx={{ minWidth: 0, textAlign: 'left' }}>
            <Typography sx={(theme) => ({ fontWeight: theme.typography.fontWeightBold, fontSize: text.s92, lineHeight: 1.1 })} noWrap>Moshe's Workspace</Typography>
            <Typography sx={{ color: 'text.secondary', fontSize: text.s68, lineHeight: 1.2 }} noWrap>Ironflyer apps</Typography>
          </Box>
        </Stack>
      </Button>

      <Box sx={(theme) => ({ p: 0.55, border: `1px solid ${theme.palette.cardBorder}`, borderRadius: `${theme.studio.radius.lg}px`, mb: 1.15, bgcolor: 'background.paper' })}>
        <Stack spacing={0.25}>
          <Button fullWidth disableRipple startIcon={<Icon name="projects" size={17} />} sx={(theme) => ({ justifyContent: 'flex-start', bgcolor: theme.palette.surfaceHover, color: 'text.primary', minHeight: 40, borderRadius: `${theme.studio.radius.sm}px`, fontWeight: theme.typography.fontWeightBold })}>Apps</Button>
          <Button fullWidth disableRipple startIcon={<Icon name="bot" size={17} />} sx={(theme) => ({ justifyContent: 'flex-start', color: 'text.secondary', minHeight: 40, borderRadius: `${theme.studio.radius.sm}px`, fontWeight: theme.typography.fontWeightMedium, '&:hover': { bgcolor: 'surfaceHover', color: 'text.primary' } })}>Superagents</Button>
        </Stack>
      </Box>

      <Stack spacing={0.25}>
        <NavItem icon={newProjectBusy ? <CircularProgress color="inherit" size={12} /> : glyph('search')} label="Search" onClick={() => { void onNewProject?.(); }} />
        <NavItem icon={glyph('home')} label="Home" active={pathname === '/' || pathname === '/studio'} onClick={() => go('/')} />
        <NavItem icon={glyph('projects')} label="All apps" active={pathname === '/projects'} onClick={() => go('/projects')} />
        <NavItem icon={glyph('templates')} label="Templates" active={pathname === '/templates'} onClick={() => go('/templates')} />
        <NavItem icon={glyph('integrations')} label="Integrations" active={pathname === '/integrations'} onClick={() => go('/integrations')} />
        <NavItem icon={glyph('users')} label="Community" active={pathname === '/agents'} onClick={() => go('/agents')} />
        <NavItem icon={glyph('deployments')} label="Deployments" active={pathname === '/build' && editorTab === 'domains'} onClick={() => go('/build?tab=domains')} />
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

      <Box
        role="button"
        tabIndex={0}
        onClick={() => go('/plans')}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); go('/plans'); } }}
        sx={(theme) => ({
          p: 1.6,
          borderRadius: `${theme.studio.radius.lg}px`,
          border: `1px solid ${theme.palette.cardBorder}`,
          bgcolor: 'background.paper',
          boxShadow: 1,
          mb: 1.5,
          cursor: 'pointer',
          outline: 'none',
          transition: `border-color ${theme.studio.motion.fast}, background-color ${theme.studio.motion.fast}`,
          '&:hover, &:focus-visible': { borderColor: `${theme.palette.primary.main}66`, bgcolor: 'surfaceHover' },
        })}
      >
        <Stack direction="row" spacing={1.25} alignItems="center">
          <Box
            sx={(theme) => ({
              display: 'grid',
              placeItems: 'center',
              width: 32,
              height: 32,
              borderRadius: `${theme.studio.radius.sm}px`,
              color: theme.studio.neon.violet,
              bgcolor: `${theme.studio.neon.violet}1F`,
              flexShrink: 0,
            })}
          >
            <Icon name="sparkles" size={17} />
          </Box>
          <Box sx={{ minWidth: 0 }}>
            <Typography sx={(theme) => ({ fontWeight: theme.typography.fontWeightBold, fontSize: text.s88 })}>Upgrade your plan</Typography>
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
        <Tooltip title="Notifications" arrow>
          <IconButton size="small" aria-label="Notifications" sx={{ color: 'text.secondary', '&:hover': { color: 'text.primary' } }}>
            <Icon name="bell" size={17} />
          </IconButton>
        </Tooltip>
      </Stack>
    </Box>
  );
}
