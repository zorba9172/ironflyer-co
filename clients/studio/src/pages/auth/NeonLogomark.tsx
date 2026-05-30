import { Box } from '@mui/material';
import { useId } from 'react';
import { neon } from '../../theme';

// The neon Ironflyer gate-forward triangle, filled with the brand blue→pink
// arc. Mirrors the landing TopNav logomark so the auth surface reads as the
// same world. Gradient stops come from the theme (legal for SVG fill).
export function NeonLogomark({ size = 44 }: { size?: number }) {
  const id = useId();
  return (
    <Box
      component="svg"
      viewBox="0 0 26 26"
      sx={{ width: size, height: size, display: 'block', flexShrink: 0 }}
      aria-hidden
    >
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor={neon.blue} />
          <stop offset="55%" stopColor={neon.purple} />
          <stop offset="100%" stopColor={neon.pink} />
        </linearGradient>
      </defs>
      <path
        d="M13 1.6 24.4 22 a1.4 1.4 0 0 1-1.2 2.1 H2.8 A1.4 1.4 0 0 1 1.6 22 Z"
        fill={`url(#${id})`}
      />
    </Box>
  );
}
