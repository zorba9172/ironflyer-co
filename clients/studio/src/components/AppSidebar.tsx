import { Box, Button, CircularProgress, Divider, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import { useNavigate, useLocation } from 'react-router-dom';
import { useThemeMode } from '@ironflyer/ui-web';
import { toast } from '@ironflyer/ui-web/fx';
import { useAuth, useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { LogoMark } from './LogoMark';
import { AccountMenu } from './AccountMenu';
import { useStudio } from '../store';
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
      sx={{
        justifyContent: 'flex-start',
        gap: 0.5,
        px: 1.5,
        py: 1,
        color: active ? 'text.primary' : 'text.secondary',
        bgcolor: active ? 'action.selected' : 'transparent',
        fontWeight: active ? 600 : 500,
        '&:hover': { bgcolor: 'action.hover', color: 'text.primary' },
        '& .MuiButton-startIcon': { color: active ? 'primary.main' : 'inherit' },
      }}
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
  sun: I('M12 2v2M12 20v2M4.9 4.9l1.4 1.4M2 12h2M20 12h2M5 19l1.4-1.4M12 8a4 4 0 100 8 4 4 0 000-8z'),
  moon: I('M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z'),
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
  const { pathname } = useLocation();
  const { user, online } = useAuth();
  const request = useRequest();
  const openProject = useStudio((s) => s.openProject);
  const openLiveProject = useStudio((s) => s.openLiveProject);
  const [openingId, setOpeningId] = useState<string | null>(null);
  const go = (to: string) => navigate(to);

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
    <Box component="aside" sx={{ width: 248, flexShrink: 0, height: '100vh', borderRight: 1, borderColor: 'divider', bgcolor: 'background.paper', display: 'flex', flexDirection: 'column', p: 1.5 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 1, mb: 2 }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <LogoMark size={26} />
          <Typography variant="h6" sx={{ fontSize: text.s100 }}>Ironflyer</Typography>
        </Stack>
        <Tooltip title={mode === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'} arrow>
          <IconButton size="small" onClick={toggle} aria-label={mode === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'} sx={{ color: 'text.secondary' }}>{mode === 'dark' ? icons.sun : icons.moon}</IconButton>
        </Tooltip>
      </Stack>

      <Button
        variant="contained"
        onClick={() => { void onNewProject?.(); }}
        disabled={newProjectBusy}
        sx={{ mb: 2, justifyContent: 'flex-start' }}
        startIcon={newProjectBusy ? <CircularProgress color="inherit" size={14} /> : <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 5v14M5 12h14" /></svg>}
      >
        <Typography sx={{ fontSize: text.s90, fontWeight: 600 }}>New project</Typography>
      </Button>

      <Stack spacing={0.25}>
        <NavItem icon={icons.home} label="Home" active={pathname === '/'} onClick={() => go('/')} />
        <NavItem icon={icons.apps} label="All projects" active={pathname === '/projects'} onClick={() => go('/projects')} />
        <NavItem icon={icons.agents} label="Agents" active={pathname === '/agents'} onClick={() => go('/agents')} />
        <NavItem icon={icons.templates} label="Templates" active={pathname === '/templates'} onClick={() => go('/templates')} />
        <NavItem icon={icons.integrations} label="Integrations" active={pathname === '/integrations'} onClick={() => go('/integrations')} />
      </Stack>

      <Divider sx={{ my: 2 }} />
      <Typography sx={(t) => ({ px: 1.5, fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Recent</Typography>
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

      <Box sx={(t) => ({ p: 2, borderRadius: 3, border: 1, borderColor: 'divider', backgroundImage: t.brand.gradient.signatureSoft, mb: 1.5 })}>
        <Typography sx={{ fontWeight: 600, fontSize: text.s90 }}>Upgrade to Pro</Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: text.s80, mt: 0.5 }}>Production deploys, mobile, and the spend board.</Typography>
        <Button size="small" variant="contained" sx={{ mt: 1.5 }} onClick={() => go('/plans')}>Upgrade</Button>
      </Box>

      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ px: 1 }}>
        <AccountMenu size={28} />
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography sx={{ fontSize: text.s85, fontWeight: 600 }} noWrap>{user?.email ?? 'Guest'}</Typography>
          <Typography sx={{ fontSize: text.s72, color: online ? (user ? 'success.main' : 'warning.main') : 'text.disabled' }} noWrap>
            {online ? (user ? `${user.plan ?? 'free'} · connected` : 'connected') : 'offline preview'}
          </Typography>
        </Box>
      </Stack>
    </Box>
  );
}
