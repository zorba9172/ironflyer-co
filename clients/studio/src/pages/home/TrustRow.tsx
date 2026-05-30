import { Box, Stack, Typography } from '@mui/material';

const WORDMARKS = ['Google', 'Microsoft', 'Airbnb', 'Amazon', 'Spotify'] as const;

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
        {WORDMARKS.map((label) => (
          <Typography
            key={label}
            variant="h6"
            sx={(theme) => ({
              fontWeight: 700,
              color: 'text.primary',
              opacity: 0.4,
              userSelect: 'none',
              transition: `opacity ${theme.studio.motion.base}`,
              '&:hover': { opacity: 1 },
            })}
          >
            {label}
          </Typography>
        ))}
      </Box>
    </Stack>
  );
}
