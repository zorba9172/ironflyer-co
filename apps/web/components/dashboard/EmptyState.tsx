'use client';

import { ReactNode } from 'react';
import { Box, Button, Stack, Typography } from '@mui/material';
import { tokens } from '../../lib/theme';

interface Props {
  title: string;
  description?: string;
  illustration?: 'spark' | 'grid' | 'orbit' | 'empty';
  primaryLabel?: string;
  onPrimary?: () => void;
  secondary?: ReactNode;
}

export function EmptyState({ title, description, illustration = 'spark', primaryLabel, onPrimary, secondary }: Props) {
  return (
    <Box
      sx={{
        textAlign: 'center',
        p: { xs: 3.5, md: 5 },
        border: '1px solid rgba(17,17,17,0.12)',
        borderRadius: '8px',
        bgcolor: '#f8f4ec',
        color: tokens.color.text.inverse,
      }}
    >
      <Box sx={{ display: 'grid', placeItems: 'center', mb: 1.6 }}>
        <Illustration kind={illustration} />
      </Box>
      <Typography variant="h6" sx={{ fontWeight: 900 }}>{title}</Typography>
      {description && (
        <Typography variant="body2" sx={{ mt: 0.7, color: '#686158', maxWidth: 460, mx: 'auto' }}>
          {description}
        </Typography>
      )}
      {(primaryLabel || secondary) && (
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} justifyContent="center" sx={{ mt: 2.2 }}>
          {primaryLabel && (
            <Button variant="contained" onClick={onPrimary}>{primaryLabel}</Button>
          )}
          {secondary}
        </Stack>
      )}
    </Box>
  );
}

function Illustration({ kind }: { kind: 'spark' | 'grid' | 'orbit' | 'empty' }) {
  if (kind === 'grid') {
    return (
      <Box sx={{ position: 'relative', width: 112, height: 84 }}>
        <Box sx={{ position: 'absolute', inset: 0, display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gridTemplateRows: 'repeat(3, 1fr)', gap: '6px' }}>
          {Array.from({ length: 12 }).map((_, i) => (
            <Box key={i} sx={{
              borderRadius: '4px',
              bgcolor: i % 5 === 0 ? tokens.color.accent.lime : 'rgba(17,17,17,0.1)',
              opacity: i % 7 === 0 ? 0.4 : 1,
            }} />
          ))}
        </Box>
      </Box>
    );
  }
  if (kind === 'orbit') {
    return (
      <Box sx={{ position: 'relative', width: 110, height: 110 }}>
        <Box sx={{ position: 'absolute', inset: 0, borderRadius: '50%', border: '2px dashed rgba(17,17,17,0.18)' }} />
        <Box sx={{ position: 'absolute', inset: 16, borderRadius: '50%', border: '2px solid rgba(17,17,17,0.12)' }} />
        <Box sx={{
          position: 'absolute', top: 8, left: 50,
          width: 14, height: 14, borderRadius: '50%',
          bgcolor: tokens.color.accent.lime,
          boxShadow: '0 0 18px rgba(229,255,0,0.7)',
        }} />
        <Box sx={{
          position: 'absolute', top: 46, left: 46,
          width: 18, height: 18, borderRadius: '4px',
          bgcolor: tokens.color.text.inverse,
        }} />
      </Box>
    );
  }
  if (kind === 'empty') {
    return (
      <Box sx={{
        width: 84, height: 84, borderRadius: '14px',
        border: '2px dashed rgba(17,17,17,0.2)',
        display: 'grid', placeItems: 'center',
      }}>
        <Box sx={{ width: 26, height: 26, borderRadius: '6px', bgcolor: 'rgba(17,17,17,0.08)' }} />
      </Box>
    );
  }
  // spark
  return (
    <Box sx={{ position: 'relative', width: 96, height: 96 }}>
      <Box sx={{
        position: 'absolute', inset: 12,
        borderRadius: '14px',
        bgcolor: tokens.color.accent.lime,
        boxShadow: '0 18px 48px rgba(229,255,0,0.32)',
        transform: 'rotate(8deg)',
      }} />
      <Box sx={{
        position: 'absolute', top: 18, left: 18,
        width: 60, height: 60, borderRadius: '14px',
        bgcolor: '#fffaf1',
        border: '1px solid rgba(17,17,17,0.12)',
      }} />
      <Box sx={{
        position: 'absolute', top: 30, left: 32,
        width: 32, height: 4, borderRadius: '4px',
        bgcolor: 'rgba(17,17,17,0.18)',
      }} />
      <Box sx={{
        position: 'absolute', top: 42, left: 32,
        width: 22, height: 4, borderRadius: '4px',
        bgcolor: 'rgba(17,17,17,0.12)',
      }} />
    </Box>
  );
}
