'use client';

import { Box, Button, Stack, Typography } from '@mui/material';
import { ErrorOutline, Refresh } from '@mui/icons-material';
import { tokens } from '../../lib/theme';

export function ErrorBox({
  title = 'Something went wrong',
  description,
  onRetry,
  retryLabel = 'Try again',
}: {
  title?: string;
  description?: string;
  onRetry?: () => void;
  retryLabel?: string;
}) {
  return (
    <Box
      sx={{
        p: { xs: 2, md: 2.4 },
        border: '1px solid rgba(255,108,58,0.32)',
        borderRadius: '8px',
        bgcolor: 'rgba(255,108,58,0.08)',
        color: tokens.color.text.inverse,
      }}
    >
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.2} alignItems={{ xs: 'flex-start', sm: 'center' }} justifyContent="space-between">
        <Stack direction="row" spacing={1.2} alignItems="flex-start">
          <ErrorOutline sx={{ color: tokens.color.accent.coral, mt: 0.2 }} />
          <Box>
            <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>{title}</Typography>
            {description && (
              <Typography variant="body2" sx={{ mt: 0.3, color: '#5b554b' }}>{description}</Typography>
            )}
          </Box>
        </Stack>
        {onRetry && (
          <Button size="small" variant="outlined" startIcon={<Refresh fontSize="small" />} onClick={onRetry}>
            {retryLabel}
          </Button>
        )}
      </Stack>
    </Box>
  );
}
