'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import {
  Add, Apps, Close, CloudQueue, Folder, Home, Hub, Logout, MenuBook,
  Notifications, Person, Search, Settings, Star, Tune, ViewList, Window,
} from '@mui/icons-material';
import {
  Avatar, Badge, Box, Button, Divider, IconButton, InputAdornment, LinearProgress,
  ListItemIcon, ListItemText, Menu, MenuItem, Stack, TextField, Tooltip, Typography,
} from '@mui/material';
import { IronflyerLogo } from '../../components/brand/IronflyerLogo';
import { api, Plan, Project, UserBudget } from '../../lib/api';
import { tokens } from '../../lib/theme';

const primaryNav = [
  { label: 'Home', href: '/app', icon: <Home fontSize="small" /> },
  { label: 'Projects', href: '/app/projects', icon: <Folder fontSize="small" /> },
  { label: 'Templates', href: '/app/resources', icon: <MenuBook fontSize="small" /> },
  { label: 'Connectors', href: '/app/connectors', icon: <Hub fontSize="small" /> },
  { label: 'Search', href: '/app/search', icon: <Search fontSize="small" /> },
  { label: 'Settings', href: '/app/settings', icon: <Settings fontSize="small" /> },
];

const mobileNav = [
  { label: 'Home', href: '/app', icon: <Home fontSize="small" /> },
  { label: 'Projects', href: '/app/projects', icon: <Folder fontSize="small" /> },
  { label: 'Search', href: '/app/search', icon: <Search fontSize="small" /> },
  { label: 'Connectors', href: '/app/connectors', icon: <Hub fontSize="small" /> },
  { label: 'Settings', href: '/app/settings', icon: <Settings fontSize="small" /> },
];

const projectNav = [
  { label: 'All projects', href: '/app/projects', icon: <Folder fontSize="small" /> },
  { label: 'Ready', href: '/app/projects?filter=ready', icon: <Star fontSize="small" /> },
  { label: 'Running', href: '/app/projects?filter=running', icon: <CloudQueue fontSize="small" /> },
  { label: 'Failed', href: '/app/projects?filter=failed', icon: <Folder fontSize="small" /> },
];

export function AppShell({
  children,
  userEmail = 'workspace',
  recents = [],
  query = '',
  setQuery,
  view = 'grid',
  setView,
  onLogout,
}: {
  children: React.ReactNode;
  userEmail?: string;
  recents?: Project[];
  query?: string;
  setQuery?: (value: string) => void;
  view?: 'grid' | 'list';
  setView?: (value: 'grid' | 'list') => void;
  onLogout?: () => void;
}) {
  return (
    <Box sx={{
      height: '100vh',
      minHeight: '100vh',
      display: 'grid',
      gridTemplateColumns: { xs: '1fr', lg: '248px 1fr' },
      bgcolor: tokens.color.bg.alabaster,
      color: tokens.color.text.inverse,
      overflow: 'hidden',
      '& .MuiTypography-colorTextSecondary': { color: '#686158' },
    }}>
      <Sidebar userEmail={userEmail} recents={recents} onLogout={onLogout} />
      <Box sx={{ minWidth: 0, minHeight: 0, height: '100vh', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <TopBar
          query={query}
          setQuery={setQuery}
          view={view}
          setView={setView}
          userEmail={userEmail}
          onLogout={onLogout}
        />
        <Box component="main" sx={{
          flex: 1,
          minHeight: 0,
          minWidth: 0,
          mb: { xs: 9.5, lg: 0 },
          overflowY: 'auto',
          overflowX: 'hidden',
          WebkitOverflowScrolling: 'touch',
          scrollbarGutter: { md: 'stable' },
          px: { xs: 1.5, sm: 2.5, md: 4 },
          pt: { xs: 2.2, md: 4 },
          pb: { xs: 11, lg: 5 },
          bgcolor: tokens.color.bg.alabaster,
          backgroundImage: 'linear-gradient(180deg, rgba(229,255,0,0.16), rgba(244,240,232,0) 260px)',
          '& .MuiButton-outlined': {
            color: tokens.color.text.inverse,
            borderColor: 'rgba(17,17,17,0.24)',
          },
        }}>
          <Box sx={{ width: '100%', maxWidth: 1120, minWidth: 0, mx: 'auto' }}>
            {children}
          </Box>
        </Box>
      </Box>
      <MobileNav />
    </Box>
  );
}

export function PageTitle({
  eyebrow,
  title,
  subtitle,
  action,
}: {
  eyebrow?: string;
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
}) {
  return (
    <Stack
      direction={{ xs: 'column', md: 'row' }}
      justifyContent="space-between"
      alignItems={{ xs: 'flex-start', md: 'flex-end' }}
      spacing={2}
      sx={{ mb: 3 }}
    >
      <Box sx={{ maxWidth: 720 }}>
        {eyebrow && (
          <Typography variant="overline" sx={{ color: '#9fb500' }}>
            {eyebrow}
          </Typography>
        )}
        <Typography
          component="h1"
          sx={{
            mt: 0.4,
            fontFamily: tokens.font.display,
            fontSize: { xs: '1.65rem', md: '2.35rem' },
            lineHeight: 1,
            textTransform: 'uppercase',
            textWrap: 'balance',
          }}
        >
          {title}
        </Typography>
        {subtitle && (
          <Typography variant="body1" sx={{ mt: 1, maxWidth: 620, color: '#686158' }}>
            {subtitle}
          </Typography>
        )}
      </Box>
      {action}
    </Stack>
  );
}

export function Surface({
  children,
  sx,
}: {
  children: React.ReactNode;
  sx?: Record<string, unknown>;
}) {
  return (
    <Box sx={{
      border: '1px solid rgba(17,17,17,0.12)',
      borderRadius: '8px',
      bgcolor: '#f8f4ec',
      color: tokens.color.text.inverse,
      boxShadow: 'none',
      '& .MuiTypography-root': { color: 'inherit' },
      '& .MuiTypography-colorTextSecondary': { color: '#686158' },
      '& .MuiChip-root': {
        color: tokens.color.text.inverse,
        borderColor: 'rgba(17,17,17,0.14)',
      },
      '& .MuiChip-icon': { color: 'inherit' },
      '&& .MuiOutlinedInput-root': {
        bgcolor: '#fffaf1',
        color: tokens.color.text.inverse,
        borderRadius: '8px',
      },
      '&& .MuiInputBase-input': {
        color: tokens.color.text.inverse,
      },
      '&& .MuiInputBase-input::placeholder': {
        color: '#6b645b',
        opacity: 1,
      },
      '& .MuiInputLabel-root': { color: '#686158' },
      '& .MuiInputLabel-root.Mui-focused': { color: '#6f7e00' },
      '& .MuiSwitch-track': { bgcolor: 'rgba(17,17,17,0.22)' },
      ...sx,
    }}>
      {children}
    </Box>
  );
}

function Sidebar({ userEmail, recents, onLogout }: { userEmail: string; recents: Project[]; onLogout?: () => void }) {
  const [budget, setBudget] = useState<UserBudget | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);

  useEffect(() => {
    let alive = true;
    void Promise.all([
      api.myBudget().catch(() => null),
      api.listPlans().catch(() => []),
    ]).then(([nextBudget, nextPlans]) => {
      if (!alive) return;
      setBudget(nextBudget);
      setPlans(nextPlans);
    });
    return () => { alive = false; };
  }, []);

  const usage = useMemo(() => usageSnapshot(budget, plans), [budget, plans]);

  return (
    <Box component="aside" sx={{
      display: { xs: 'none', lg: 'flex' },
      flexDirection: 'column',
      height: '100vh',
      minHeight: 0,
      position: 'relative',
      top: 0,
      borderRight: '1px solid rgba(244,240,232,0.1)',
      bgcolor: tokens.color.bg.inset,
      color: tokens.color.text.primary,
      p: 1.5,
      overflow: 'hidden',
    }}>
      <Link href="/" style={{ color: 'inherit', textDecoration: 'none' }}>
        <Box sx={{ px: 1, py: 0.75 }}>
          <IronflyerLogo size={28} tone="dark" />
        </Box>
      </Link>

      <Button component={Link} href="/app" fullWidth startIcon={<Add />} sx={{
        justifyContent: 'flex-start',
        mt: 2,
        bgcolor: tokens.color.bg.surfaceRaised,
        color: tokens.color.text.primary,
        borderRadius: '8px',
      }}>
        New project
      </Button>

      <NavList items={primaryNav} />

      <Divider sx={{ my: 2 }} />
      <Typography variant="overline" color="text.secondary" sx={{ px: 1 }}>Status</Typography>
      <NavList items={projectNav} compact />

      <Divider sx={{ my: 2 }} />
      <Typography variant="overline" color="text.secondary" sx={{ px: 1 }}>Recent</Typography>
      <Stack spacing={0.4} sx={{
        mt: 0.75,
        minHeight: 0,
        overflowY: 'auto',
        overflowX: 'hidden',
        pr: 0.2,
        scrollbarWidth: 'thin',
      }}>
        {recents.length === 0 && <Typography variant="caption" color="text.secondary" sx={{ px: 1 }}>No active projects yet</Typography>}
        {recents.map((project) => (
          <Button
            key={project.id}
            component={Link}
            href={`/projects/${project.id}`}
            sx={{ justifyContent: 'flex-start', color: tokens.color.text.secondary, borderRadius: '8px', minHeight: 32 }}
          >
            <Typography variant="body2" noWrap>{project.name}</Typography>
          </Button>
        ))}
      </Stack>

      <Button
        component={Link}
        href="/app/settings?tab=billing"
        fullWidth
        sx={{
          mt: 'auto',
          p: 1.6,
          border: '1px solid rgba(244,240,232,0.14)',
          borderRadius: '8px',
          bgcolor: tokens.color.bg.surface,
          textAlign: 'left',
          justifyContent: 'stretch',
          flexDirection: 'column',
          alignItems: 'stretch',
          '&:hover': { bgcolor: tokens.color.bg.surfaceHover, transform: 'none' },
        }}
      >
        <Stack direction="row" spacing={1} alignItems="center" sx={{ width: '100%' }}>
          <CloudQueue fontSize="small" sx={{ color: tokens.color.accent.lime }} />
          <Typography variant="subtitle2" sx={{ color: tokens.color.text.primary }}>{usage.planName}</Typography>
        </Stack>
        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'flex-start', mt: 0.2 }}>{usage.label}</Typography>
        <Box sx={{ mt: 1.2, width: '100%' }}>
          <Stack direction="row" justifyContent="space-between">
            <Typography variant="caption" color="text.secondary">Monthly budget</Typography>
            <Typography variant="caption" sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary }}>
              ${usage.spent.toFixed(2)} / ${usage.cap.toFixed(2)}
            </Typography>
          </Stack>
          <LinearProgress variant="determinate" value={usage.percent} sx={{
            mt: 0.65,
            height: 7,
            borderRadius: '999px',
            bgcolor: 'rgba(244,240,232,0.12)',
            '& .MuiLinearProgress-bar': { bgcolor: usage.percent > 82 ? tokens.color.accent.coral : tokens.color.accent.lime },
          }} />
        </Box>
        <Typography
          variant="caption"
          sx={{
            mt: 1.1,
            alignSelf: 'flex-start',
            color: tokens.color.accent.lime,
            fontWeight: 800,
            letterSpacing: 0.3,
          }}
        >
          {usage.tier === 'free' ? 'Upgrade plan ->' : 'Manage plan ->'}
        </Typography>
      </Button>

      <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 1.5, color: tokens.color.text.primary }}>
        <Avatar sx={{ width: 30, height: 30, bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse, fontWeight: 900 }}>
          {userEmail[0]?.toUpperCase()}
        </Avatar>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="body2" noWrap>{userEmail}</Typography>
          <Typography variant="caption" color="text.secondary">{usage.tier === 'free' ? 'Free account' : `${usage.planName} plan`}</Typography>
        </Box>
        <Tooltip title="Settings">
          <IconButton component={Link} href="/app/settings" size="small" sx={{ color: 'text.secondary' }}><Settings fontSize="small" /></IconButton>
        </Tooltip>
      </Stack>
      {onLogout && <Button onClick={onLogout} sx={{ mt: 0.6, color: tokens.color.text.secondary }} startIcon={<Logout fontSize="small" />}>Log out</Button>}
    </Box>
  );
}

function usageSnapshot(budget: UserBudget | null, plans: Plan[]) {
  const tier = budget?.tier ?? 'free';
  const plan = plans.find((item) => item.tier === tier);
  const cap = Number(plan?.costCapUSD ?? (tier === 'team' ? 32 : tier === 'pro' ? 8 : tier === 'enterprise' ? 180 : 0.5));
  const spent = Number(budget?.spent ?? 0);
  const percent = cap > 0 ? Math.min(100, Math.max(0, (spent / cap) * 100)) : 0;
  const planName = plan?.name ? `${plan.name}` : tier === 'free' ? 'Free' : `${tier[0]?.toUpperCase() ?? 'F'}${tier.slice(1)}`;
  const label = tier === 'free'
    ? 'Basic usage with a hard cap'
    : 'Usage stays under the monthly cap';
  return { tier, planName, label, spent, cap: Math.max(cap, 0.5), percent };
}

function NavList({ items, compact = false }: { items: typeof primaryNav; compact?: boolean }) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const currentQuery = searchParams.toString();
  return (
    <Stack spacing={0.35} sx={{ mt: compact ? 0.75 : 2.5 }}>
      {items.map((item) => {
        const active = isActiveHref(item.href, pathname, currentQuery, compact);
        return (
          <Button
            key={item.label}
            component={Link}
            href={item.href}
            startIcon={item.icon}
            sx={{
              minHeight: compact ? 34 : 42,
              justifyContent: 'flex-start',
              color: active ? tokens.color.text.primary : tokens.color.text.secondary,
              bgcolor: active ? tokens.color.bg.surfaceHover : 'transparent',
              borderRadius: '8px',
              '&:hover': {
                bgcolor: tokens.color.bg.surfaceHover,
                color: tokens.color.text.primary,
              },
            }}
          >
            {item.label}
          </Button>
        );
      })}
    </Stack>
  );
}

function MobileNav() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const currentQuery = searchParams.toString();

  return (
    <Box component="nav" sx={{
      display: { xs: 'block', lg: 'none' },
      position: 'fixed',
      left: 12,
      right: 12,
      bottom: 12,
      zIndex: 20,
      border: '1px solid rgba(17,17,17,0.14)',
      borderRadius: '8px',
      bgcolor: 'rgba(248,244,236,0.94)',
      backdropFilter: 'blur(16px)',
      boxShadow: '0 18px 48px rgba(17,17,17,0.18)',
    }}>
      <Stack direction="row" spacing={0.5} sx={{ p: 0.5 }}>
        {mobileNav.map((item) => {
          const active = isActiveHref(item.href, pathname, currentQuery);
          return (
            <Button
              key={item.label}
              component={Link}
              href={item.href}
              startIcon={item.icon}
              aria-label={item.label}
              sx={{
                flex: 1,
                minWidth: 0,
                minHeight: 44,
                px: 0.8,
                borderRadius: '8px',
                color: active ? tokens.color.text.inverse : '#4f4941',
                bgcolor: active ? tokens.color.accent.lime : 'transparent',
                '& .MuiButton-startIcon': { mr: { xs: 0, sm: 0.7 } },
                '& .MuiButton-startIcon svg': { fontSize: 19 },
                '&:hover': {
                  bgcolor: active ? tokens.color.accent.lime : 'rgba(17,17,17,0.06)',
                },
              }}
            >
              <Typography component="span" variant="caption" sx={{ display: { xs: 'none', sm: 'inline' }, fontWeight: 900 }}>
                {item.label}
              </Typography>
            </Button>
          );
        })}
      </Stack>
    </Box>
  );
}

function isActiveHref(href: string, pathname: string, currentQuery: string, exactQuery = false) {
  const [itemPath, itemQuery = ''] = href.split('?');
  if (itemQuery) return pathname === itemPath && currentQuery === itemQuery;
  return pathname === itemPath && (!exactQuery || !currentQuery);
}

function TopBar({
  query,
  setQuery,
  view,
  setView,
  userEmail,
  onLogout,
}: {
  query: string;
  setQuery?: (value: string) => void;
  view: 'grid' | 'list';
  setView?: (value: 'grid' | 'list') => void;
  userEmail: string;
  onLogout?: () => void;
}) {
  const router = useRouter();
  const [internalQuery, setInternalQuery] = useState(query);
  const [menuEl, setMenuEl] = useState<HTMLElement | null>(null);
  const [notifEl, setNotifEl] = useState<HTMLElement | null>(null);

  useEffect(() => setInternalQuery(query), [query]);

  function commitGlobal(value: string) {
    setInternalQuery(value);
    if (setQuery) {
      setQuery(value);
      return;
    }
    if (!value) return;
  }

  function submitToSearch(value: string) {
    if (setQuery) return; // page handles search locally
    if (!value.trim()) return;
    router.push(`/app/search?q=${encodeURIComponent(value)}`);
  }

  return (
    <Box sx={{
      minHeight: { xs: 94, sm: 58 },
      flex: '0 0 auto',
      display: 'flex',
      alignItems: 'center',
      flexWrap: 'wrap',
      gap: 1,
      px: { xs: 1.5, md: 2 },
      py: { xs: 1, sm: 0 },
      borderBottom: '1px solid rgba(17,17,17,0.1)',
      bgcolor: 'rgba(244,240,232,0.9)',
      backdropFilter: 'blur(14px)',
      position: 'sticky',
      top: 0,
      zIndex: 9,
    }}>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ display: { xs: 'flex', lg: 'none' }, minWidth: 0 }}>
        <IronflyerLogo size={24} tone="light" />
      </Stack>
      <TextField
        value={internalQuery}
        onChange={(event) => commitGlobal(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') submitToSearch(internalQuery);
        }}
        placeholder={setQuery ? 'Search this page...' : 'Search all projects...'}
        size="small"
        sx={{
          display: 'block',
          order: { xs: 3, sm: 0 },
          flex: 1,
          flexBasis: { xs: '100%', sm: 'auto' },
          maxWidth: { sm: 360, lg: 520 },
          ml: { xs: 'auto', lg: 0 },
          '& .MuiOutlinedInput-root': {
            bgcolor: '#fffaf1',
            color: tokens.color.text.inverse,
            borderRadius: '8px',
            '& fieldset': { borderColor: 'rgba(17,17,17,0.16)' },
            '&:hover fieldset': { borderColor: 'rgba(17,17,17,0.34)' },
            '&.Mui-focused fieldset': { borderColor: tokens.color.accent.lime },
          },
          '& .MuiInputBase-input': {
            fontSize: { xs: '0.95rem', sm: '0.92rem' },
            py: { xs: 1.1, sm: 0.9 },
          },
          '& .MuiInputBase-input::placeholder': { color: '#5f5a52', opacity: 1 },
          '& .MuiSvgIcon-root': { color: '#5f5a52' },
        }}
        InputProps={{
          startAdornment: (
            <InputAdornment position="start">
              <Search fontSize="small" />
            </InputAdornment>
          ),
          endAdornment: internalQuery ? (
            <InputAdornment position="end">
              <Tooltip title="Clear search">
                <IconButton aria-label="Clear search" edge="end" size="small" onClick={() => commitGlobal('')}>
                  <Close fontSize="small" />
                </IconButton>
              </Tooltip>
            </InputAdornment>
          ) : undefined,
        }}
      />
      <Stack direction="row" spacing={0.35} sx={{ ml: 'auto' }}>
        {setView && (
          <>
            <Tooltip title="Grid view">
              <IconButton aria-label="Grid view" onClick={() => setView('grid')} sx={topIconButtonSx(view === 'grid')}>
                <Window fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title="List view">
              <IconButton aria-label="List view" onClick={() => setView('list')} sx={topIconButtonSx(view === 'list')}>
                <ViewList fontSize="small" />
              </IconButton>
            </Tooltip>
          </>
        )}
        <Tooltip title="Notifications">
          <IconButton aria-label="Notifications" onClick={(e) => setNotifEl(e.currentTarget)} sx={topIconButtonSx(false)}>
            <Badge color="primary" variant="dot" overlap="circular" invisible>
              <Notifications fontSize="small" />
            </Badge>
          </IconButton>
        </Tooltip>
        <Tooltip title="Workspace settings">
          <IconButton aria-label="Workspace settings" component={Link} href="/app/settings" sx={topIconButtonSx(false)}>
            <Tune fontSize="small" />
          </IconButton>
        </Tooltip>
        <Tooltip title="Account">
          <IconButton aria-label="Account" onClick={(e) => setMenuEl(e.currentTarget)} sx={{ p: 0.4 }}>
            <Avatar sx={{ width: 30, height: 30, bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse, fontWeight: 900, fontSize: '0.85rem' }}>
              {userEmail[0]?.toUpperCase()}
            </Avatar>
          </IconButton>
        </Tooltip>
      </Stack>

      <Menu
        anchorEl={notifEl}
        open={Boolean(notifEl)}
        onClose={() => setNotifEl(null)}
        slotProps={{
          paper: {
            sx: { mt: 1, minWidth: 280, borderRadius: '10px', border: '1px solid rgba(17,17,17,0.1)' },
          },
        }}
      >
        <Box sx={{ px: 2, py: 1.4 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 900 }}>No new notifications</Typography>
          <Typography variant="caption" sx={{ color: '#686158' }}>
            Run, gate, patch, and deployment alerts will appear here.
          </Typography>
        </Box>
      </Menu>

      <Menu
        anchorEl={menuEl}
        open={Boolean(menuEl)}
        onClose={() => setMenuEl(null)}
        slotProps={{
          paper: {
            sx: { mt: 1, minWidth: 220, borderRadius: '10px', border: '1px solid rgba(17,17,17,0.1)' },
          },
        }}
      >
        <Box sx={{ px: 2, py: 1.2 }}>
          <Typography variant="caption" sx={{ color: '#86807a' }}>Signed in as</Typography>
          <Typography variant="body2" sx={{ fontWeight: 900 }} noWrap>{userEmail}</Typography>
        </Box>
        <Divider />
        <MenuItem component={Link} href="/app/settings?tab=account" onClick={() => setMenuEl(null)}>
          <ListItemIcon><Person fontSize="small" /></ListItemIcon>
          <ListItemText>Account</ListItemText>
        </MenuItem>
        <MenuItem component={Link} href="/app/settings?tab=billing" onClick={() => setMenuEl(null)}>
          <ListItemIcon><CloudQueue fontSize="small" /></ListItemIcon>
          <ListItemText>Plan and billing</ListItemText>
        </MenuItem>
        <MenuItem component={Link} href="/app/connectors" onClick={() => setMenuEl(null)}>
          <ListItemIcon><Hub fontSize="small" /></ListItemIcon>
          <ListItemText>Connectors</ListItemText>
        </MenuItem>
        <Divider />
        {onLogout && (
          <MenuItem onClick={() => { setMenuEl(null); onLogout(); }}>
            <ListItemIcon><Logout fontSize="small" /></ListItemIcon>
            <ListItemText>Log out</ListItemText>
          </MenuItem>
        )}
      </Menu>
    </Box>
  );
}

function topIconButtonSx(active: boolean) {
  return {
    width: 36,
    height: 36,
    borderRadius: '8px',
    color: active ? tokens.color.text.inverse : '#3f3b35',
    bgcolor: active ? tokens.color.accent.lime : 'transparent',
    '&:hover': {
      bgcolor: active ? tokens.color.accent.lime : 'rgba(17,17,17,0.08)',
    },
  };
}
