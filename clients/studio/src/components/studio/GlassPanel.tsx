import { Box, type BoxProps } from '@mui/material';
import { forwardRef } from 'react';
import { studioTokens } from '../../theme';

export type GlassPanelProps = BoxProps & {
  /** neon accent (theme.studio.neon.*) — adds a soft rim glow + tinted edge */
  accent?: string;
  /** lift + brighten on hover; use for clickable tiles */
  interactive?: boolean;
  /** keep the always-dark neon treatment in both theme modes (e.g. anchor panels) */
  dark?: boolean;
  /** padding on the theme spacing scale (default 3) */
  pad?: number;
};

// The canonical Studio surface — the "card that doesn't feel like a card"
// (mx.md › Cards): faint fill, 1px hairline, backdrop blur, soft 24px radius.
// Every pane composes from this instead of styling raw Boxes, so the whole
// studio shares one floating-glass language. Accent adds the neon rim glow the
// constitution reserves for live/active surfaces. Zero raw color literals.
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
            backgroundColor: dark ? theme.studio.effect.promptBuilder.bg : theme.palette.cardBg,
            border: `1px solid ${dark ? theme.studio.effect.card.border : theme.palette.cardBorder}`,
            backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
            WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
            color: dark ? studioTokens.modes.dark.textPrimary : theme.palette.text.primary,
            transition: `transform ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}, box-shadow ${theme.studio.motion.base}`,
            ...(accent && {
              boxShadow: `0 0 0 1px ${tone}22, 0 18px 48px ${tone}1f`,
            }),
            ...(interactive && {
              cursor: 'pointer',
              '&:hover': {
                transform: 'translateY(-2px)',
                borderColor: `${tone}55`,
                boxShadow: `0 0 0 1px ${tone}33, 0 22px 60px ${tone}26`,
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
