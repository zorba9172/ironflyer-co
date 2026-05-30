import { Box } from '@mui/material';
import type { SxProps } from '@mui/material';
import { useThemeMode } from '../../theme';

// Fixed atmospheric layer behind the entire hero. Two stacked, blended layers:
//   1. the massive radial neon glows (top-left blue, center violet,
//      bottom-right pink) at 5–15% opacity — no visible circles, only
//      atmosphere (mx.md › Ambient Effects).
//   2. a faint engineered 1px grid, masked to fade out at the top and bottom
//      edges so it reads as depth, not a table.
// Both pull every value from the theme; zero raw color/size literals here.
export function AmbientBackdrop(props?: { sx?: SxProps }) {
  const { mode } = useThemeMode();

  return (
    <Box
      aria-hidden
      sx={[
        (theme) => ({
          position: 'absolute',
          inset: 0,
          zIndex: 0,
          pointerEvents: 'none',
          overflow: 'hidden',
          // Smooth atmosphere shift on the dark/light toggle.
          transition: `background ${theme.studio.motion.base}`,
          // The radial neon glows — mode-aware recipe straight from the theme.
          background: theme.studio.effect.ambient[mode],
          '&::before': {
            content: '""',
            position: 'absolute',
            inset: 0,
            backgroundImage: [
              `linear-gradient(148deg, transparent 0 56%, ${theme.studio.neon.blue}33 57%, transparent 59%)`,
              `linear-gradient(150deg, transparent 0 64%, ${theme.studio.neon.pink}3D 65%, transparent 67%)`,
              `linear-gradient(32deg, transparent 0 71%, ${theme.studio.neon.violet}30 72%, transparent 74%)`,
            ].join(', '),
            opacity: mode === 'light' ? 0.22 : 0.42,
            maskImage: `linear-gradient(to bottom, transparent 0%, ${theme.palette.common.black} 18%, ${theme.palette.common.black} 74%, transparent 100%)`,
            WebkitMaskImage: `linear-gradient(to bottom, transparent 0%, ${theme.palette.common.black} 18%, ${theme.palette.common.black} 74%, transparent 100%)`,
            transition: `opacity ${theme.studio.motion.base}`,
          },
          '&::after': {
            // Engineered grid texture overlaid on the glows.
            content: '""',
            position: 'absolute',
            inset: 0,
            backgroundImage: `linear-gradient(to right, ${theme.studio.effect.gridLine} 1px, transparent 1px), linear-gradient(to bottom, ${theme.studio.effect.gridLine} 1px, transparent 1px)`,
            backgroundSize: '88px 88px',
            // Dimmer grid in light mode so it never overpowers the canvas.
            opacity: mode === 'light' ? 0.4 : 1,
            // Fade the grid out at the top + bottom edges.
            maskImage: `linear-gradient(to bottom, transparent 0%, ${theme.palette.common.black} 18%, ${theme.palette.common.black} 78%, transparent 100%)`,
            WebkitMaskImage: `linear-gradient(to bottom, transparent 0%, ${theme.palette.common.black} 18%, ${theme.palette.common.black} 78%, transparent 100%)`,
            transition: `opacity ${theme.studio.motion.base}`,
          },
        }),
        ...(Array.isArray(props?.sx) ? props!.sx : props?.sx ? [props.sx] : []),
      ]}
    >
      <Box
        className="if-ambient-wave"
        sx={(theme) => ({
          position: 'absolute',
          left: '-8%',
          right: '-8%',
          bottom: { xs: '-8%', md: '-14%' },
          height: { xs: 260, md: 360 },
          backgroundImage: [
            `radial-gradient(ellipse at 18% 42%, ${theme.studio.neon.blue}26 0%, transparent 54%)`,
            `radial-gradient(ellipse at 84% 46%, ${theme.studio.neon.pink}2B 0%, transparent 58%)`,
            `linear-gradient(178deg, transparent 0%, ${theme.studio.neon.violet}1F 42%, transparent 72%)`,
          ].join(', '),
          opacity: mode === 'light' ? 0.28 : 0.85,
          clipPath: 'polygon(0 42%, 10% 38%, 20% 44%, 32% 34%, 46% 43%, 58% 36%, 72% 45%, 86% 34%, 100% 43%, 100% 100%, 0 100%)',
          filter: 'blur(1px)',
          transition: `opacity ${theme.studio.motion.base}`,
        })}
      />
    </Box>
  );
}
