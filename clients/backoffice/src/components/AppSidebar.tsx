import { Box, Button, Divider, IconButton, Stack, Typography } from '@mui/material';
import { useNavigate, useLocation } from 'react-router-dom';
import { useThemeMode } from '@ironflyer/ui-web';
import { useAuth } from '@ironflyer/data';
import { LogoMark } from './LogoMark';
import { AccountMenu } from './AccountMenu';
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

// 17px stroke icons — no icon dependency.
const I = (d: string) => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d={d} /></svg>
);
const icons = {
  overview: I('M3 13h8V3H3zM13 21h8V8h-8zM13 3v3h8V3zM3 17h8v4H3z'),
  projects: I('M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z'),
  wallet: I('M3 7h18v12H3zM3 7l2-3h11l2 3M16 13h2'),
  audit: I('M9 11l3 3 7-7M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11'),
};

const NAV = [
  { to: '/', label: 'Overview', icon: icons.overview },
  { to: '/projects', label: 'Projects', icon: icons.projects },
  { to: '/wallet', label: 'Wallet', icon: icons.wallet },
  { to: '/audit', label: 'Audit', icon: icons.audit },
];

export function AppSidebar() {
  const { mode, toggle } = useThemeMode();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { user, online } = useAuth();

  return (
    <Box component="aside" sx={{ width: 248, flexShrink: 0, height: '100vh', borderRight: 1, borderColor: 'divider', bgcolor: 'background.paper', display: 'flex', flexDirection: 'column', p: 1.5 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 1, mb: 2 }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <LogoMark size={26} />
          <Stack spacing={0}>
            <Typography variant="h6" sx={{ fontSize: '1rem', lineHeight: 1.1 }}>Ironflyer</Typography>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.6rem', letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled' })}>Backoffice</Typography>
          </Stack>
        </Stack>
        <IconButton size="small" onClick={toggle} aria-label="toggle theme" sx={{ color: 'text.secondary' }}>{mode === 'dark' ? '☼' : '☾'}</IconButton>
      </Stack>

      <Divider sx={{ mb: 1.5 }} />
      <Typography sx={(t) => ({ px: 1.5, mb: 0.5, fontFamily: t.brand.font.mono, fontSize: '0.62rem', letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled' })}>Operate</Typography>
      <Stack spacing={0.25}>
        {NAV.map((n) => (
          <NavItem key={n.to} icon={n.icon} label={n.label} active={pathname === n.to} onClick={() => navigate(n.to)} />
        ))}
      </Stack>

      <Box sx={{ flex: 1 }} />

      <Box sx={(t) => ({ p: 2, borderRadius: 3, border: 1, borderColor: 'divider', backgroundImage: t.brand.gradient.signatureSoft, mb: 1.5 })}>
        <Typography sx={{ fontWeight: 600, fontSize: '0.9rem' }}>Margin is healthy</Typography>
        <Typography sx={{ color: 'text.secondary', fontSize: '0.8rem', mt: 0.5 }}>Revenue is outrunning provider cost across the last 30 days.</Typography>
      </Box>

      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ px: 1 }}>
        <AccountMenu size={28} />
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography sx={{ fontSize: '0.85rem', fontWeight: 600 }} noWrap>{user?.email ?? 'Operator'}</Typography>
          <Typography sx={() => ({ fontSize: '0.72rem', color: online ? (user ? 'success.main' : 'warning.main') : 'text.disabled' })} noWrap>
            {online ? (user ? 'operator · connected' : 'connected') : 'offline preview'}
          </Typography>
        </Box>
      </Stack>
    </Box>
  );
}
