import { Box, Chip, Stack, Typography } from '@mui/material';
import { LuSparkles } from 'react-icons/lu';
import { studioTokens } from '../../theme';

// Centered hero text block above the prompt builder (mx.md › Hero Section,
// Headline Formula). AI badge → headline (final phrase gradient) → subheading.
// Pixel-faithful to the locked render: small glass pill, big 800-weight title,
// muted single-line-ish description.
export function Hero() {
  return (
    <Stack id="product" alignItems="center" textAlign="center" spacing={2.75} sx={{ px: 2 }}>
      <Chip
        icon={<LuSparkles size={16} />}
        label={
          <Box component="span">
            AI-powered{' '}
            <Box component="span" sx={(theme) => ({ color: theme.studio.neon.blue })}>
              product builder
            </Box>
          </Box>
        }
        sx={(theme) => ({
          height: 34,
          px: 1.25,
          borderRadius: theme.studio.radius.pill,
          border: `1px solid ${theme.palette.divider}`,
          backgroundColor: theme.palette.cardBg,
          backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          color: theme.palette.text.secondary,
          fontWeight: theme.typography.fontWeightMedium,
          letterSpacing: '0.01em',
          '& .MuiChip-icon': { color: theme.studio.neon.pink, ml: 0.25 },
          '& .MuiChip-label': { px: 1 },
        })}
      />

      <Typography
        variant="h1"
        sx={{
          fontWeight: 800,
          fontSize: { xs: '2.28rem', sm: '3.35rem', md: '4.5rem' },
          lineHeight: { xs: 1.08, md: 1.02 },
          maxWidth: 930,
          overflowWrap: 'break-word',
          textWrap: 'balance',
          textShadow: (theme) => theme.palette.mode === 'dark' ? `0 12px 46px ${studioTokens.modes.dark.textPrimary}24` : 'none',
        }}
      >
        <Box component="span" sx={{ display: { xs: 'inline', md: 'block' } }}>Build, review and ship{' '}</Box>
        <Box component="span" sx={{ display: { xs: 'inline', md: 'block' } }}>production apps{' '}</Box>
        <Box
          component="span"
          sx={(theme) => ({
            display: { md: 'block' },
            backgroundImage: theme.studio.gradient.signature,
            WebkitBackgroundClip: 'text',
            backgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            color: 'transparent',
          })}
        >
          from a single prompt.
        </Box>
      </Typography>

      <Typography
        color="text.secondary"
        sx={{ maxWidth: 720, mx: 'auto', fontSize: { xs: '1rem', md: '1.125rem' }, lineHeight: 1.5 }}
      >
        Ironflyer turns a plain-language idea into screens, real data, code, tests and deployments — in minutes.
      </Typography>
    </Stack>
  );
}
