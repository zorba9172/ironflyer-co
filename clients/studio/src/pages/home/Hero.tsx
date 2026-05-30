import { Stack, Typography } from '@mui/material';

function greeting(): string {
  const h = new Date().getHours();
  if (h < 12) return 'Good morning';
  if (h < 18) return 'Good afternoon';
  return 'Good evening';
}

// Warm editorial greeting — the first thing the operator reads. Bricolage
// display weight via the h2 variant (never an inline font); calm muted subline.
export function Hero(props: { name?: string }) {
  const name = props.name?.trim() || 'there';
  return (
    <Stack spacing={0.5}>
      <Typography variant="h2" sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        {greeting()}, {name}
        <Typography component="span" variant="h2" aria-hidden role="img">
          👋
        </Typography>
      </Typography>
      <Typography variant="body1" color="text.secondary" sx={{ maxWidth: 560 }}>
        Build, ship, and scale your ideas with agents.
      </Typography>
    </Stack>
  );
}
