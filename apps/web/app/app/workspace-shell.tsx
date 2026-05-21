'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
  Add, Apps, CloudQueue, Folder, Home, Hub, Search, Settings, Star, Tune,
  ViewList, Window,
} from '@mui/icons-material';
import {
  Avatar, Box, Button, Divider, IconButton, InputAdornment, Stack,
  TextField, Tooltip, Typography,
} from '@mui/material';
import { Project } from '../../lib/api';
import { tokens } from '../../lib/theme';

const primaryNav = [
  { label: 'Home', href: '/app', icon: <Home fontSize="small" /> },
  { label: 'Search', href: '/app/search', icon: <Search fontSize="small" /> },
  { label: 'Resources', href: '/app/resources', icon: <Apps fontSize="small" /> },
  { label: 'Connectors', href: '/app/connectors', icon: <Hub fontSize="small" /> },
];

const projectNav = [
  { label: 'All projects', href: '/app/projects', icon: <Folder fontSize="small" /> },
  { label: 'Starred', href: '/app/projects?filter=starred', icon: <Star fontSize="small" /> },
  { label: 'Created by me', href: '/app/projects?filter=created', icon: <Folder fontSize="small" /> },
  { label: 'Shared with me', href: '/app/projects?filter=shared', icon: <Folder fontSize="small" /> },
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
      minHeight: '100vh',
      display: 'grid',
      gridTemplateColumns: { xs: '1fr', lg: '248px 1fr' },
      bgcolor: tokens.color.bg.alabaster,
      color: tokens.color.text.inverse,
      overflowX: 'hidden',
      '& .MuiTypography-colorTextSecondary': { color: '#686158' },
    }}>
      <Sidebar userEmail={userEmail} recents={recents} onLogout={onLogout} />
      <Box sx={{ minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <TopBar query={query} setQuery={setQuery} view={view} setView={setView} />
        <Box component="main" sx={{
          flex: 1,
          minWidth: 0,
          overflowX: 'hidden',
          px: { xs: 2, md: 4 },
          py: { xs: 3, md: 5 },
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
            fontSize: { xs: '1.72rem', md: '2.65rem' },
            lineHeight: 0.95,
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
      borderRadius: { xs: 2.2, md: 3.2 },
      bgcolor: '#f8f4ec',
      color: tokens.color.text.inverse,
      boxShadow: 'none',
      ...sx,
    }}>
      {children}
    </Box>
  );
}

function Sidebar({ userEmail, recents, onLogout }: { userEmail: string; recents: Project[]; onLogout?: () => void }) {
  return (
    <Box component="aside" sx={{
      display: { xs: 'none', lg: 'flex' },
      flexDirection: 'column',
      height: '100vh',
      position: 'sticky',
      top: 0,
      borderRight: '1px solid rgba(244,240,232,0.1)',
      bgcolor: tokens.color.bg.inset,
      color: tokens.color.text.primary,
      p: 1.5,
    }}>
      <Link href="/" style={{ color: 'inherit', textDecoration: 'none' }}>
        <Stack direction="row" spacing={1.2} alignItems="center" sx={{ px: 1, py: 0.75 }}>
          <Box sx={{ width: 24, height: 24, borderRadius: 1, bgcolor: tokens.color.accent.lime, boxShadow: `0 0 20px ${tokens.color.accent.lime}` }} />
          <Typography variant="subtitle1" sx={{ fontFamily: tokens.font.display, fontWeight: 400, textTransform: 'uppercase' }}>Ironflyer</Typography>
        </Stack>
      </Link>

      <Button component={Link} href="/app" fullWidth startIcon={<Add />} sx={{
        justifyContent: 'flex-start',
        mt: 2,
        bgcolor: tokens.color.bg.surfaceRaised,
        color: tokens.color.text.primary,
      }}>
        New project
      </Button>

      <NavList items={primaryNav} />

      <Divider sx={{ my: 2 }} />
      <Typography variant="overline" color="text.secondary" sx={{ px: 1 }}>Projects</Typography>
      <NavList items={projectNav} compact />

      <Divider sx={{ my: 2 }} />
      <Typography variant="overline" color="text.secondary" sx={{ px: 1 }}>Recents</Typography>
      <Stack spacing={0.4} sx={{ mt: 0.75, minHeight: 0, overflow: 'auto' }}>
        {recents.length === 0 && <Typography variant="caption" color="text.secondary" sx={{ px: 1 }}>No recent projects yet</Typography>}
        {recents.map((project) => (
          <Button
            key={project.id}
            component={Link}
            href={`/projects/${project.id}`}
            sx={{ justifyContent: 'flex-start', color: tokens.color.text.secondary, borderRadius: 1, minHeight: 32 }}
          >
            <Typography variant="body2" noWrap>{project.name}</Typography>
          </Button>
        ))}
      </Stack>

      <Box sx={{
        mt: 'auto',
        p: 1.6,
        border: '1px solid rgba(244,240,232,0.14)',
        borderRadius: 2,
        bgcolor: tokens.color.bg.surface,
      }}>
        <Stack direction="row" spacing={1} alignItems="center">
          <CloudQueue fontSize="small" sx={{ color: tokens.color.accent.lime }} />
          <Typography variant="subtitle2">Credits</Typography>
        </Stack>
        <Typography variant="caption" color="text.secondary">Runs, previews, deploy gates.</Typography>
        <Button fullWidth variant="contained" size="small" sx={{ mt: 1.2 }}>View plans</Button>
      </Box>

      <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 1.5, color: tokens.color.text.primary }}>
        <Avatar sx={{ width: 30, height: 30, bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse, fontWeight: 900 }}>
          {userEmail[0]?.toUpperCase()}
        </Avatar>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="body2" noWrap>{userEmail}</Typography>
          <Typography variant="caption" color="text.secondary">Free workspace</Typography>
        </Box>
        <Tooltip title="Workspace settings">
          <IconButton component={Link} href="/app/settings" size="small" sx={{ color: 'text.secondary' }}><Settings fontSize="small" /></IconButton>
        </Tooltip>
      </Stack>
      {onLogout && <Button onClick={onLogout} sx={{ mt: 0.6, color: tokens.color.text.secondary }}>Sign out</Button>}
    </Box>
  );
}

function NavList({ items, compact = false }: { items: typeof primaryNav; compact?: boolean }) {
  const pathname = usePathname();
  return (
    <Stack spacing={0.35} sx={{ mt: compact ? 0.75 : 2.5 }}>
      {items.map((item) => {
        const active = pathname === item.href.split('?')[0];
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
              borderRadius: 1.3,
            }}
          >
            {item.label}
          </Button>
        );
      })}
    </Stack>
  );
}

function TopBar({
  query,
  setQuery,
  view,
  setView,
}: {
  query: string;
  setQuery?: (value: string) => void;
  view: 'grid' | 'list';
  setView?: (value: 'grid' | 'list') => void;
}) {
  return (
    <Box sx={{
      minHeight: 56,
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      px: { xs: 1.5, md: 2 },
      borderBottom: '1px solid rgba(17,17,17,0.1)',
      bgcolor: 'rgba(244,240,232,0.9)',
      backdropFilter: 'blur(14px)',
      position: 'sticky',
      top: 0,
      zIndex: 9,
    }}>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ display: { xs: 'flex', lg: 'none' }, minWidth: 0 }}>
        <Box sx={{ width: 21, height: 21, borderRadius: 1, bgcolor: tokens.color.accent.lime }} />
        <Typography variant="subtitle2" sx={{ fontFamily: tokens.font.display, fontWeight: 400, textTransform: 'uppercase', color: tokens.color.text.inverse }}>Ironflyer</Typography>
      </Stack>
      <TextField
        value={query}
        onChange={(event) => setQuery?.(event.target.value)}
        placeholder="Search projects, prompts, folders..."
        size="small"
        sx={{
          display: { xs: 'none', sm: 'block' },
          flex: 1,
          maxWidth: { sm: 360, lg: 520 },
          ml: { xs: 'auto', lg: 0 },
          '& .MuiOutlinedInput-root': {
            bgcolor: '#fffaf1',
            color: tokens.color.text.inverse,
            borderRadius: 999,
            '& fieldset': { borderColor: 'rgba(17,17,17,0.16)' },
            '&:hover fieldset': { borderColor: 'rgba(17,17,17,0.34)' },
            '&.Mui-focused fieldset': { borderColor: tokens.color.accent.lime },
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
        }}
      />
      <Stack direction="row" spacing={0.4} sx={{ ml: 'auto' }}>
        <IconButton onClick={() => setView?.('grid')} sx={{ color: view === 'grid' ? tokens.color.accent.lime : '#3f3b35' }}>
          <Window fontSize="small" />
        </IconButton>
        <IconButton onClick={() => setView?.('list')} sx={{ color: view === 'list' ? tokens.color.accent.lime : '#3f3b35' }}>
          <ViewList fontSize="small" />
        </IconButton>
        <IconButton component={Link} href="/app/settings" sx={{ color: '#3f3b35' }}>
          <Tune fontSize="small" />
        </IconButton>
      </Stack>
    </Box>
  );
}
