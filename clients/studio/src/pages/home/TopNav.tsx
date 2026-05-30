import { Box, Button, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import { LuChevronDown, LuMoon, LuSun } from 'react-icons/lu';
import { neon } from '../../theme';

// IRONFLYER STUDIO — top navigation bar.
// Pixel-faithful to the locked neon home render: a translucent floating row that
// holds the gate-forward logomark + wordmark on the left, the primary nav links
// in the center-left, and the theme toggle / log-in / "Start a project free" CTA
// on the right. Every color, blur, radius, and motion value is read from the
// studio theme — no inline literals (constitutional law).

type NavLink = {
  label: string;
  href: string;
  active?: boolean;
  menu?: boolean;
};

const NAV_LINKS: readonly NavLink[] = [
  { label: 'Product', href: '#product', active: true },
  { label: 'Templates', href: '/templates' },
  { label: 'Solutions', href: '#solutions', menu: true },
  { label: 'Pricing', href: '/plans' },
  { label: 'Resources', href: '#resources', menu: true },
  { label: 'Enterprise', href: '#enterprise' },
] as const;

// Small neon triangle logomark. Filled by a linearGradient whose stops are the
// brand blue → pink — imported from the theme (legal for SVG fill contexts).
function Logomark() {
  return (
    <Box
      component="svg"
      viewBox="0 0 42 28"
      sx={{ width: 36, height: 24, display: 'block', flexShrink: 0 }}
      aria-hidden
    >
      <defs>
        <linearGradient id="if-topnav-mark" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor={neon.blue} />
          <stop offset="45%" stopColor={neon.violet} />
          <stop offset="100%" stopColor={neon.pink} />
        </linearGradient>
      </defs>
      <path d="M4 23 15 6" stroke="url(#if-topnav-mark)" strokeWidth="7" strokeLinecap="round" />
      <path d="M17 23 28 6" stroke="url(#if-topnav-mark)" strokeWidth="7" strokeLinecap="round" opacity="0.92" />
      <path d="M30 23 38 10" stroke="url(#if-topnav-mark)" strokeWidth="7" strokeLinecap="round" opacity="0.82" />
    </Box>
  );
}

export function TopNav(props: { onThemeToggle: () => void; mode: 'light' | 'dark'; onLogin?: () => void; onStart?: () => void }) {
  const { onThemeToggle, mode, onLogin, onStart } = props;
  const isDark = mode === 'dark';

  return (
    <Box
      component="nav"
      sx={(theme) => ({
        display: 'flex',
        alignItems: 'center',
        gap: 2,
        width: '100%',
        minWidth: 0,
        px: { xs: 0, md: 0.5 },
        py: 0.25,
        transition: `color ${theme.studio.motion.base}`,
      })}
    >
      {/* Left — logomark + wordmark */}
      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ flexShrink: 0 }}>
        <Logomark />
        <Typography
          variant="h6"
          sx={(theme) => ({
            display: { xs: 'none', sm: 'block' },
            fontWeight: 800,
            letterSpacing: 0,
            color: theme.palette.text.primary,
            lineHeight: 1,
          })}
        >
          IronFlyer
        </Typography>
      </Stack>

      {/* Center-left — primary nav links */}
      <Stack
        direction="row"
        alignItems="center"
        spacing={1.25}
        sx={{ display: { xs: 'none', md: 'flex' }, ml: 2 }}
      >
        {NAV_LINKS.map(({ label, href, active, menu }) => (
          <Typography
            key={label}
            component="a"
            href={href}
            variant="body2"
            sx={(theme) => ({
              display: 'inline-flex',
              alignItems: 'center',
              gap: 0.45,
              height: 38,
              px: active ? 2 : 1.15,
              borderRadius: theme.studio.radius.pill,
              color: active ? theme.palette.text.primary : theme.palette.text.secondary,
              fontWeight: active ? 700 : 600,
              textDecoration: 'none',
              cursor: 'pointer',
              border: active ? `1px solid ${theme.palette.divider}` : '1px solid transparent',
              backgroundColor: active ? theme.palette.cardBg : 'transparent',
              boxShadow: active ? `0 0 24px ${theme.studio.neon.violet}33` : 'none',
              backdropFilter: active ? `blur(${theme.studio.effect.card.blur}px)` : 'none',
              transition: `color ${theme.studio.motion.fast}, background-color ${theme.studio.motion.fast}, border-color ${theme.studio.motion.fast}, box-shadow ${theme.studio.motion.fast}`,
              '&:hover': {
                color: theme.palette.text.primary,
                borderColor: theme.palette.borderSubtle,
                backgroundColor: theme.palette.cardBg,
              },
            })}
          >
            {label}
            {menu && <LuChevronDown size={13} />}
          </Typography>
        ))}
      </Stack>

      {/* Right — theme toggle, log in, primary CTA */}
      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ ml: 'auto', flexShrink: 0 }}>
        <Tooltip title={isDark ? 'Switch to light mode' : 'Switch to dark mode'}>
          <IconButton
            onClick={onThemeToggle}
            aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
            sx={(theme) => ({
              color: theme.palette.text.secondary,
              border: `1px solid ${theme.palette.divider}`,
              width: 40,
              height: 40,
              bgcolor: theme.palette.cardBg,
              backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
              transition: `color ${theme.studio.motion.fast}, background-color ${theme.studio.motion.fast}, border-color ${theme.studio.motion.fast}`,
              '&:hover': { color: theme.palette.text.primary, bgcolor: theme.palette.surfaceHover, borderColor: theme.palette.divider },
            })}
          >
            {isDark ? <LuSun size={18} /> : <LuMoon size={18} />}
          </IconButton>
        </Tooltip>

        <Button
          onClick={onLogin}
          variant="text"
          sx={(theme) => ({
            display: { xs: 'none', sm: 'inline-flex' },
            color: theme.palette.text.secondary,
            fontWeight: 600,
            '&:hover': { color: theme.palette.text.primary, bgcolor: theme.palette.surfaceHover },
          })}
        >
          Log in
        </Button>

        <Button onClick={onStart} variant="contained" color="primary" sx={{ whiteSpace: 'nowrap', px: { xs: 1.75, sm: 2.5 }, minWidth: 0 }}>
          <Box component="span" sx={{ display: { xs: 'none', sm: 'inline' } }}>Start a project free</Box>
          <Box component="span" sx={{ display: { xs: 'inline', sm: 'none' } }}>Start free</Box>
        </Button>
      </Stack>
    </Box>
  );
}
