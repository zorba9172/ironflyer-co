import { Box, Typography } from '@mui/material';
import { LuCode, LuShieldCheck, LuRocket, LuActivity } from 'react-icons/lu';
import type { IconType } from 'react-icons';
import { neon } from '../../theme';

// ─────────────────────────────────────────────────────────────────────────
// Feature Cards (mx.md › Feature Cards). Four glass tiles that "should not
// feel like cards": faint fill + 1px hairline + backdrop blur, a soft radial
// wash of the item accent under a rounded icon tile, a 700-weight title, and a
// single sentence. Responsive 1 / 2 / 4 columns at xs / sm / md. Hover lifts
// the tile by 2px on the slow neon easing. No raw color literals — accents come
// from the neon brand marks, surfaces/text from the mode-aware palette.
// ─────────────────────────────────────────────────────────────────────────

type Feature = {
  title: string;
  body: string;
  Icon: IconType;
  accent: string;
};

const FEATURES: readonly Feature[] = [
  {
    title: 'AI generates code',
    body: 'Production-ready code from your prompt.',
    Icon: LuCode,
    accent: neon.blue,
  },
  {
    title: 'Review & test',
    body: 'Preview, test and refine before shipping.',
    Icon: LuShieldCheck,
    accent: neon.purple,
  },
  {
    title: 'Deploy anywhere',
    body: 'One-click deploy to cloud or your infrastructure.',
    Icon: LuRocket,
    accent: neon.pink,
  },
  {
    title: 'Monitor & iterate',
    body: 'Logs, metrics and real-time observability.',
    Icon: LuActivity,
    accent: neon.success,
  },
];

export function FeatureGrid() {
  return (
    <Box
      id="solutions"
      sx={(theme) => ({
        width: '100%',
        maxWidth: 1180,
        display: 'grid',
        gridTemplateColumns: {
          xs: '1fr',
          sm: 'repeat(2, 1fr)',
          md: 'repeat(4, 1fr)',
        },
        backgroundColor: theme.palette.cardBg,
        border: `1px solid ${theme.palette.cardBorder}`,
        borderRadius: `${theme.studio.effect.card.radius}px`,
        backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        overflow: 'hidden',
      })}
    >
      {FEATURES.map(({ title, body, Icon, accent }, index) => (
        <Box
          key={title}
          sx={(theme) => ({
            display: 'flex',
            alignItems: 'center',
            gap: 2,
            p: { xs: 2.5, md: 3 },
            minWidth: 0,
            borderRight: { md: index === FEATURES.length - 1 ? 0 : `1px solid ${theme.palette.borderSubtle}` },
            borderBottom: {
              xs: index === FEATURES.length - 1 ? 0 : `1px solid ${theme.palette.borderSubtle}`,
              sm: index > 1 ? 0 : `1px solid ${theme.palette.borderSubtle}`,
              md: 0,
            },
            transition: `background-color ${theme.studio.motion.base}`,
            '&:hover': {
              backgroundColor: theme.palette.surfaceHover,
            },
          })}
        >
          <Box
            aria-hidden
            sx={(theme) => ({
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 44,
              height: 44,
              flexShrink: 0,
              borderRadius: `${theme.studio.radius.sm}px`,
              color: accent,
              fontSize: 22,
              background: `radial-gradient(120% 120% at 30% 20%, ${accent}33, ${accent}0D 70%)`,
              border: `1px solid ${accent}33`,
            })}
          >
            <Icon strokeWidth={1.5} />
          </Box>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
            <Typography variant="h6" sx={{ fontWeight: 700, fontSize: '1rem' }}>
              {title}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ lineHeight: 1.55 }}>
              {body}
            </Typography>
          </Box>
        </Box>
      ))}
    </Box>
  );
}
