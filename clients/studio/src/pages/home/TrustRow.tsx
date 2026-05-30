import { Box, Stack, Typography } from '@mui/material';
import { FaAirbnb, FaAmazon, FaGoogle, FaMicrosoft, FaSpotify } from 'react-icons/fa';
import type { IconType } from 'react-icons';

// Trust strip at the foot of the neon Home hero (mx.md › Trust Section).
// Monochrome wordmarks at 40% opacity, lifting to 100% on hover. No logos,
// no color — only typographic marks so the neon hero stays the hero.
const WORDMARKS: readonly { label: string; Icon: IconType }[] = [
  { label: 'Google', Icon: FaGoogle },
  { label: 'Microsoft', Icon: FaMicrosoft },
  { label: 'airbnb', Icon: FaAirbnb },
  { label: 'amazon', Icon: FaAmazon },
  { label: 'Spotify', Icon: FaSpotify },
];

export function TrustRow() {
  return (
    <Stack alignItems="center" spacing={3} sx={{ width: '100%', py: 4 }}>
      <Typography
        variant="body2"
        color="text.secondary"
        sx={{ textAlign: 'center', letterSpacing: '0.04em' }}
      >
        Trusted by modern teams worldwide
      </Typography>

      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'center',
          columnGap: { xs: 4, sm: 6, md: 7 },
          rowGap: 2.5,
        }}
      >
        {WORDMARKS.map(({ label, Icon }) => (
          <Box
            key={label}
            component="span"
            sx={(theme) => ({
              display: 'inline-flex',
              alignItems: 'center',
              gap: 0.8,
              fontWeight: 700,
              color: 'text.primary',
              fontSize: { xs: '1rem', sm: '1.2rem' },
              opacity: 0.36,
              userSelect: 'none',
              transition: `opacity ${theme.studio.motion.base}`,
              '&:hover': { opacity: 1 },
            })}
          >
            <Icon aria-hidden />
            <Typography component="span" sx={{ font: 'inherit', fontWeight: 'inherit' }}>{label}</Typography>
          </Box>
        ))}
      </Box>
    </Stack>
  );
}
