'use client';

import { useSearchParams } from 'next/navigation';
import { Box, Button, Container, Stack, Typography } from '@mui/material';
import Link from 'next/link';
import { CheckCircle, ErrorOutline } from '@mui/icons-material';
import { tokens } from '../lib/theme';

export function BillingStatusBanner({ compact = false }: { compact?: boolean }) {
  const searchParams = useSearchParams();
  const status = searchParams.get('stripe');
  if (status !== 'success' && status !== 'cancel') return null;

  const success = status === 'success';
  const content = (
    <Box sx={{
      borderRadius: '8px',
      bgcolor: success ? 'rgba(229,255,0,0.2)' : 'rgba(255,108,58,0.14)',
      border: `1px solid ${success ? 'rgba(17,17,17,0.14)' : 'rgba(255,108,58,0.35)'}`,
      color: '#111',
      p: { xs: 1.4, md: 1.8 },
    }}>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.2} alignItems={{ xs: 'flex-start', sm: 'center' }} justifyContent="space-between">
        <Stack direction="row" spacing={1} alignItems="flex-start">
          {success ? <CheckCircle sx={{ color: '#718000' }} /> : <ErrorOutline sx={{ color: tokens.color.accent.coral }} />}
          <Box>
            <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>
              {success ? 'Plan updated' : 'Checkout was not completed'}
            </Typography>
            <Typography variant="body2" sx={{ color: '#5b554b', fontWeight: 600 }}>
              {success
                ? 'Your workspace can now use the upgraded credits and governance controls.'
                : 'No payment was captured. You can choose a plan again when ready.'}
            </Typography>
          </Box>
        </Stack>
        <Button component={Link} href={success ? '/app/settings' : '/pricing'} variant="contained" size="small">
          {success ? 'Open billing' : 'View plans'}
        </Button>
      </Stack>
    </Box>
  );

  if (compact) return content;

  return (
    <Box sx={{ bgcolor: tokens.color.bg.alabaster, pt: { xs: 2, md: 3 } }}>
      <Container maxWidth="xl">{content}</Container>
    </Box>
  );
}
