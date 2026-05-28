import { Avatar, Box, Button, Divider, IconButton, Stack, Typography } from '@mui/material';
import { useNavigate, useLocation } from 'react-router-dom';
import { useThemeMode } from '@ironflyer/ui-web';
import { LogoMark } from './LogoMark';
import { useStudio } from '../store';
import { mockProject } from '../studioData';
import type { ReactNode } from 'react';

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
  community: I('M17 21v-2a4 4 0 00-3-3.87M9 21v-2a4 4 0 013-3.87M12 11a4 4 0 100-8 4 4 0 000 8z'),
  agents: I('M12 8V4H8M4 8h16v12H4zM2 14h2M20 14h2M9 13v2M15 13v2'),
};

export function AppSidebar({ onNewProject }: { onNewProject?: () => void }) {
  const { mode, toggle } = useThemeMode();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const openProject = useStudio((s) => s.openProject);
  const go = (to: string) => navigate(to);
  const openRecent = () => { openProject(mockProject); navigate('/build'); };
  return (
    <Box component="aside" sx={{ width: 248, flexShrink: 0, height: '100vh', borderRight: 1, borderColor: 'divider', bgcolor: 'background.paper', display: 'flex', flexDirection: 'column', p: 1.5 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 1, mb: 2 }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <LogoMark size={26} />
          <Typography variant="h6" sx={{ fontSize: '1rem' }}>Ironflyer</Typography>
        </Stack>
        <IconButton size="small" onClick={toggle} aria-label="toggle theme" sx={{ color: 'text.secondary' }}>{mode === 'dark' ? '☼' : '☾'}</IconButton>
      </Stack>

      <Button
        variant="outlined"
        color="inherit"
        onClick={onNewProject}
        sx={{ mb: 2, justifyContent: 'space-between', borderColor: 'divider' }}
        endIcon={<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M8 9l4-4 4 4M8 15l4 4 4-4" /></svg>}
      >
        <Stack direction="row" alignItems="center" spacing={1}>
          <Avatar sx={{ width: 22, height: 22, fontSize: 11, bgcolor: 'primary.main' }}>M</Avatar>
          <Typography sx={{ fontSize: '0.9rem', fontWeight: 600 }}>My Workspace</Typography>
        </Stack>
      </Button>

      <Stack spacing={0.25}>
        <NavItem icon={icons.home} label="Home" active={pathname === '/'} onClick={() => go('/')} />
        <NavItem icon={icons.apps} label="All projects" active={pathname === '/projects'} onClick={() => go('/projects')} />
        <NavItem icon={icons.agents} label="Agents" active={pathname === '/agents'} onClick={() => go('/agents')} />
        <NavItem icon={icons.templates} label="Templates" active={pathname === '/templates'} onClick={() => go('/templates')} />
        <NavItem icon={icons.integrations} label="Integrations" active={pathname === '/integrations'} onClick={() => go('/integrations')} />
      </Stack>

      <Divider sx={{ my: 2 }} />
      <Typography sx={(t) => ({ px: 1.5, fontFamily: t.brand.font.mono, fontSize: '0.68rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Recent</Typography>
      <Stack spacing={0.25} sx={{ mt: 1 }}>
        <NavItem icon={<Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: 'warning.main' }} />} label="Northwind Checkout" onClick={openRecent} />
        <NavItem icon={<Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: 'success.main' }} />} label="MathQuest" onClick={openRecent} />
      </Stack>

      <Box sx={{ flex: 1 }} />

      <Box sx={(t) => ({ p: 2, borderRadius: 3, border: 1, borderColor: 'divider', backgroundImage: t.brand.gradient.signatureSoft, mb: 1.5 })}>
        <Typography sx={{ fontWeight: 600, fontSize: '0.9rem' }}>Upgrade to Pro</Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: '0.8rem', mt: 0.5 }}>Production deploys, mobile, and the spend board.</Typography>
        <Button size="small" variant="contained" sx={{ mt: 1.5 }}>Upgrade</Button>
      </Box>

      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ px: 1 }}>
        <Avatar sx={{ width: 28, height: 28, fontSize: 13, bgcolor: 'action.selected', color: 'text.primary' }}>M</Avatar>
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontSize: '0.85rem', fontWeight: 600 }} noWrap>Moshe</Typography>
          <Typography sx={{ fontSize: '0.72rem', color: 'text.disabled' }} noWrap>Free plan</Typography>
        </Box>
      </Stack>
    </Box>
  );
}
