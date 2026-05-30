import { Stack, Typography } from '@mui/material';

export function Hero() {
  return (
    <Stack alignItems="center" textAlign="center" spacing={{ xs: 1.05, md: 1.35 }} sx={{ px: { xs: 1, md: 2 } }}>
      <Typography
        variant="h1"
        sx={{
          fontWeight: 900,
          fontSize: { xs: '2rem', sm: '2.8rem', md: '3.55rem' },
          lineHeight: { xs: 1.04, md: 1.02 },
          maxWidth: 760,
          letterSpacing: 0,
        }}
      >
        What will you build next?
      </Typography>

      <Typography
        color="text.secondary"
        sx={{ maxWidth: 610, mx: 'auto', fontSize: { xs: '0.94rem', md: '1.02rem' }, lineHeight: 1.5 }}
      >
        Describe the app you want to create. Ironflyer plans, builds, reviews, and ships it with your agent team.
      </Typography>
    </Stack>
  );
}
