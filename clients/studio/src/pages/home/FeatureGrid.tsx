import { Box, Stack, Typography } from '@mui/material';
import { LuCode, LuShieldCheck, LuRocket, LuActivity } from 'react-icons/lu';
import type { IconType } from 'react-icons';

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
  tone: 'primary' | 'success' | 'warning' | 'neutral';
};

const FEATURES: readonly Feature[] = [
  {
    title: 'AI generates code',
    body: 'Production-ready code written before it ships.',
    Icon: LuCode,
    tone: 'primary',
  },
  {
    title: 'Review & test',
    body: 'Gates check quality and security before deploy.',
    Icon: LuShieldCheck,
    tone: 'warning',
  },
  {
    title: 'Deploy anywhere',
    body: 'One-click deploy to prod or your own cloud.',
    Icon: LuRocket,
    tone: 'primary',
  },
  {
    title: 'Monitor & iterate',
    body: 'Live cost, gates and metrics with full observability.',
    Icon: LuActivity,
    tone: 'success',
  },
];

export function FeatureGrid() {
  return (
    <Box
      sx={{
        display: 'grid',
        gap: 2.5,
        gridTemplateColumns: {
          xs: '1fr',
          sm: 'repeat(2, 1fr)',
          md: 'repeat(4, 1fr)',
        },
      }}
    >
      {FEATURES.map(({ title, body, Icon, tone }) => (
        <Box
          key={title}
          sx={(theme) => ({
            '--feature-accent':
              tone === 'success'
                ? theme.palette.success.main
                : tone === 'warning'
                  ? theme.palette.warning.main
                  : tone === 'neutral'
                    ? theme.palette.text.secondary
                    : theme.palette.primary.main,
            display: 'flex',
            alignItems: 'center',
            gap: 1.25,
            p: 2,
            minWidth: 0,
            backgroundColor: theme.palette.cardBg,
            border: `1px solid ${theme.palette.cardBorder}`,
            borderRadius: `${theme.studio.radius.lg}px`,
            transition: `transform ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}`,
            '&:hover': {
              transform: 'translateY(-1px)',
              borderColor: theme.palette.divider,
            },
          })}
        >
          <Box
            aria-hidden
            sx={(theme) => ({
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 38,
              height: 38,
              flexShrink: 0,
              borderRadius: `${theme.studio.radius.sm}px`,
              color: 'var(--feature-accent)',
              fontSize: 22,
              backgroundColor: `${theme.palette.primary.main}0a`,
              border: `1px solid ${theme.palette.divider}`,
            })}
          >
            <Icon strokeWidth={1.5} />
          </Box>

          <Stack sx={{ minWidth: 0 }}>
            <Typography variant="h6" sx={{ fontWeight: 800, fontSize: '0.95rem' }} noWrap>
              {title}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.8rem' }} noWrap>
              {body}
            </Typography>
          </Stack>
        </Box>
      ))}
    </Box>
  );
}
