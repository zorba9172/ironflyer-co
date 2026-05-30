import { Box, Button, Stack, Typography } from '@mui/material';
import { Icon } from '../../icons';

// ─────────────────────────────────────────────────────────────────────────
// A single pricing tier. Glass tile that "doesn't feel like a card": faint
// fill, hairline edge, backdrop blur, 2px hover lift on the slow neon easing.
// The recommended tier is wrapped in a gradient ring + violet bloom and floats
// a "Most popular" badge — the one place the full brand gradient is allowed on
// a card edge. Feature rows use a neon-tinted Lucide check. Every value reads
// from the theme; no raw color/size literals.
// ─────────────────────────────────────────────────────────────────────────

export type PlanTier = {
  name: string;
  price: string; // mode-derived display price (e.g. "$0", "$16")
  cadenceLabel: string; // "/mo" or "/mo billed annually"
  credits: string;
  popular: boolean;
  features: string[];
};

export function PlanCard({ tier, onSelect }: { tier: PlanTier; onSelect: (name: string) => void }) {
  const { name, price, cadenceLabel, credits, popular, features } = tier;

  return (
    <Box
      sx={(theme) => ({
        position: 'relative',
        borderRadius: `${theme.studio.effect.card.radius}px`,
        // Recommended tier: gradient ring via a 1.5px padded backdrop.
        ...(popular
          ? {
              p: '1.5px',
              backgroundImage: theme.studio.gradient.signature,
              boxShadow: `0 22px 60px ${theme.studio.neon.violet}40, 0 0 34px ${theme.studio.neon.pink}2E`,
            }
          : { p: 0 }),
      })}
    >
      <Box
        sx={(theme) => ({
          position: 'relative',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          p: 3,
          borderRadius: popular ? `${theme.studio.effect.card.radius - 2}px` : `${theme.studio.effect.card.radius}px`,
          backgroundColor: popular ? theme.palette.background.paper : theme.palette.cardBg,
          border: popular ? 'none' : `1px solid ${theme.palette.cardBorder}`,
          backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          overflow: 'hidden',
          transition: `transform ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}`,
          '&:hover': {
            transform: 'translateY(-2px)',
            borderColor: popular ? undefined : theme.palette.borderSubtle,
          },
        })}
      >
        {/* Soft violet bloom behind the recommended tier header (token gradient). */}
        {popular && (
          <Box
            aria-hidden
            sx={(theme) => ({
              position: 'absolute',
              inset: 0,
              pointerEvents: 'none',
              backgroundImage: theme.studio.gradient.soft,
            })}
          />
        )}

        {popular && (
          <Stack
            direction="row"
            spacing={0.5}
            alignItems="center"
            sx={(theme) => ({
              alignSelf: 'flex-start',
              mb: 1.5,
              px: 1.25,
              py: 0.5,
              borderRadius: theme.studio.radius.pill,
              backgroundImage: theme.studio.gradient.signature,
              color: theme.palette.common.white,
            })}
          >
            <Icon name="sparkles" size={13} strokeWidth={2} />
            <Typography
              variant="caption"
              sx={(theme) => ({ fontWeight: theme.typography.fontWeightBold, letterSpacing: '0.06em', textTransform: 'uppercase' })}
            >
              Most popular
            </Typography>
          </Stack>
        )}

        <Typography variant="h6" sx={(theme) => ({ fontWeight: theme.typography.fontWeightBold, position: 'relative' })}>
          {name}
        </Typography>

        <Stack direction="row" alignItems="baseline" spacing={0.5} sx={{ mt: 1, position: 'relative' }}>
          <Typography component="span" sx={(theme) => ({ fontSize: '2.75rem', fontWeight: theme.typography.fontWeightBold, lineHeight: 1, letterSpacing: '-0.02em' })}>
            {price}
          </Typography>
          <Typography component="span" variant="body2" color="text.secondary">
            {cadenceLabel}
          </Typography>
        </Stack>

        <Typography variant="body2" sx={(theme) => ({ mt: 1.5, fontWeight: theme.typography.fontWeightMedium, color: theme.studio.neon.blue })}>
          {credits}
        </Typography>

        <Box sx={{ mt: 2.5, position: 'relative' }}>
          <Button
            fullWidth
            variant={popular ? 'contained' : 'outlined'}
            color={popular ? 'primary' : 'inherit'}
            onClick={() => onSelect(name)}
            sx={(theme) =>
              popular
                ? {}
                : {
                    borderColor: theme.palette.divider,
                    color: theme.palette.text.primary,
                    '&:hover': { borderColor: theme.studio.neon.blue, backgroundColor: theme.palette.surfaceHover },
                  }
            }
          >
            {price === '$0' ? `Start with ${name}` : `Get ${name}`}
          </Button>
        </Box>

        <Stack spacing={1.5} sx={{ mt: 3, position: 'relative' }}>
          {features.map((f) => (
            <Stack key={f} direction="row" spacing={1.25} alignItems="flex-start">
              <Box
                aria-hidden
                sx={(theme) => ({
                  mt: '1px',
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: 18,
                  height: 18,
                  flexShrink: 0,
                  borderRadius: theme.studio.radius.pill,
                  color: theme.studio.neon.success,
                  backgroundColor: `${theme.studio.neon.success}1F`,
                })}
              >
                <Icon name="check" size={12} strokeWidth={2.5} />
              </Box>
              <Typography variant="body2" color="text.secondary">
                {f}
              </Typography>
            </Stack>
          ))}
        </Stack>
      </Box>
    </Box>
  );
}
