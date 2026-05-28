import { Box, Typography, type TypographyProps } from '@mui/material';
import type { ReactNode } from 'react';

// Mono uppercase label. Color/typography mapped through the theme.
export function Eyebrow({ children }: { children: ReactNode }) {
  return (
    <Typography
      component="p"
      sx={(t) => ({
        fontFamily: t.brand.font.mono,
        fontSize: '0.75rem',
        letterSpacing: '0.12em',
        textTransform: 'uppercase',
        color: 'text.disabled',
      })}
    >
      {children}
    </Typography>
  );
}

// Heading filled with the brand signature gradient. Caller sx is merged.
export function GradientText({ children, sx, ...props }: TypographyProps & { component?: React.ElementType }) {
  return (
    <Box
      component="span"
      sx={[
        (t) => ({
          backgroundImage: t.brand.gradient.signature,
          WebkitBackgroundClip: 'text',
          backgroundClip: 'text',
          color: 'transparent',
        }),
        ...(Array.isArray(sx) ? sx : [sx]),
      ]}
      {...props}
    >
      {children}
    </Box>
  );
}
