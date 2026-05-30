import { Box, Button, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import { LuMoon, LuSun } from 'react-icons/lu';
import { neon } from '../../theme';

// IRONFLYER STUDIO — top navigation bar.
// Pixel-faithful to the locked neon home render: a translucent floating row that
// holds the gate-forward logomark + wordmark on the left, the primary nav links
// in the center-left, and the theme toggle / log-in / "Start a project free" CTA
// on the right. Every color, blur, radius, and motion value is read from the
// studio theme — no inline literals (constitutional law).

const NAV_LINKS = ['Product', 'Templates', 'Solutions', 'Pricing', 'Enterprise'] as const;

// Small neon triangle logomark. Filled by a linearGradient whose stops are the
// brand blue → pink — imported from the theme (legal for SVG fill contexts).
function Logomark() {
  return (
    <Box
      component="svg"
      viewBox="0 0 26 26"
      sx={{ width: 26, height: 26, display: 'block', flexShrink: 0 }}
      aria-hidden
    >
      <defs>
        <linearGradient id="if-topnav-mark" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor={neon.blue} />
          <stop offset="100%" stopColor={neon.pink} />
        </linearGradient>
      </defs>
      <path d="M13 1.6 24.4 22 a1.4 1.4 0 0 1-1.2 2.1 H2.8 A1.4 1.4 0 0 1 1.6 22 Z" fill="url(#if-topnav-mark)" />
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
        px: { xs: 2.5, md: 4 },
        py: 1.5,
        bgcolor: theme.palette.background.paper,
        border: `1px solid ${theme.palette.divider}`,
        borderRadius: theme.studio.radius.lg,
        backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        transition: `background-color ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}`,
      })}
    >
      {/* Left — logomark + wordmark */}
      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ flexShrink: 0 }}>
        <Logomark />
        <Typography
          variant="h6"
          sx={(theme) => ({ fontWeight: 800, letterSpacing: 0, color: theme.palette.text.primary, lineHeight: 1 })}
        >
          Ironflyer
        </Typography>
      </Stack>

      {/* Center-left — primary nav links */}
      <Stack
        direction="row"
        alignItems="center"
        spacing={3}
        sx={{ display: { xs: 'none', md: 'flex' }, ml: 2 }}
      >
        {NAV_LINKS.map((label) => (
          <Typography
            key={label}
            component="a"
            href="#"
            variant="body2"
            sx={(theme) => ({
              color: theme.palette.text.secondary,
              fontWeight: 500,
              textDecoration: 'none',
              cursor: 'pointer',
              transition: `color ${theme.studio.motion.fast}`,
              '&:hover': { color: theme.palette.text.primary },
            })}
          >
            {label}
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

        <Button onClick={onStart} variant="contained" color="primary" sx={{ whiteSpace: 'nowrap', px: 2.5 }}>
          Start a project free
        </Button>
      </Stack>
    </Box>
  );
}
