import { Stack, Typography } from '@mui/material';

// Warm editorial greeting — the first thing the operator reads. Bricolage
// display weight via the h2 variant (never an inline font); calm muted subline.
export function Hero(props: { name?: string }) {
  const name = props.name?.trim() || 'there';
  return (
    <Stack spacing={0.75}>
      <Typography variant="h2" sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        Good morning, {name}
        <Typography component="span" variant="h2" aria-hidden role="img">
          👋
        </Typography>
      </Typography>
      <Typography variant="body1" color="text.secondary" sx={{ maxWidth: 560 }}>
        Build, ship, and scale your ideas with agents. Describe what you want and
        Ironflyer plans, reviews, and ships it.
      </Typography>
    </Stack>
  );
}
