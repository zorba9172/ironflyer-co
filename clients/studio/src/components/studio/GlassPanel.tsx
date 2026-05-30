import { Box, type BoxProps } from '@mui/material';
import { forwardRef } from 'react';
import { studioTokens } from '../../theme';

export type GlassPanelProps = BoxProps & {
  /** semantic accent (theme.studio.neon.*) for a restrained focus edge */
  accent?: string;
  /** lift + brighten on hover; use for clickable tiles */
  interactive?: boolean;
  /** keep a dark treatment when a surface explicitly needs it */
  dark?: boolean;
  /** padding on the theme spacing scale (default 3) */
  pad?: number;
};

// Canonical Studio surface: plain white/gray product card with a hairline
// border. The old glass/neon language has been flattened here so every pane
// inherits the cleaner workspace look.
export const GlassPanel = forwardRef<HTMLDivElement, GlassPanelProps>(function GlassPanel(
  { accent, interactive, dark, pad = 3, sx, children, ...rest },
  ref,
) {
  return (
    <Box
      ref={ref}
      sx={[
        (theme) => {
          const tone = accent ?? theme.studio.neon.violet;
          return {
            position: 'relative',
            p: pad,
            borderRadius: `${theme.studio.effect.card.radius}px`,
            backgroundColor: dark ? theme.palette.surfaceRaised : theme.palette.cardBg,
            border: `1px solid ${dark ? theme.studio.effect.card.border : theme.palette.cardBorder}`,
            color: dark ? studioTokens.modes.dark.textPrimary : theme.palette.text.primary,
            transition: `transform ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}, box-shadow ${theme.studio.motion.base}`,
            ...(accent && {
              boxShadow: `inset 3px 0 0 ${tone}`,
            }),
            ...(interactive && {
              cursor: 'pointer',
              '&:hover': {
                transform: 'translateY(-1px)',
                borderColor: tone,
                boxShadow: `0 8px 20px rgba(17,24,39,0.08)`,
              },
            }),
          };
        },
        ...(Array.isArray(sx) ? sx : sx ? [sx] : []),
      ]}
      {...rest}
    >
      {children}
    </Box>
  );
});
