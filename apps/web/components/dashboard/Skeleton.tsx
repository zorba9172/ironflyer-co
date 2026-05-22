'use client';

import { Box, Stack } from '@mui/material';

export function SkeletonBlock({ height = 16, width = '100%', radius = 6 }: { height?: number | string; width?: number | string; radius?: number }) {
  return (
    <Box
      sx={{
        width,
        height,
        borderRadius: `${radius}px`,
        background:
          'linear-gradient(90deg, rgba(17,17,17,0.06) 0%, rgba(17,17,17,0.11) 50%, rgba(17,17,17,0.06) 100%)',
        backgroundSize: '200% 100%',
        animation: 'ironflyerShimmer 1.4s ease-in-out infinite',
        '@keyframes ironflyerShimmer': {
          '0%': { backgroundPosition: '200% 0' },
          '100%': { backgroundPosition: '-200% 0' },
        },
      }}
    />
  );
}

export function SkeletonCard({ lines = 3, minHeight = 150 }: { lines?: number; minHeight?: number }) {
  return (
    <Box
      sx={{
        p: 2,
        minHeight,
        borderRadius: '8px',
        border: '1px solid rgba(17,17,17,0.12)',
        bgcolor: '#f8f4ec',
      }}
    >
      <Stack spacing={1.2}>
        <SkeletonBlock height={18} width="55%" />
        <SkeletonBlock height={12} width="80%" />
        {Array.from({ length: Math.max(lines - 2, 0) }).map((_, i) => (
          <SkeletonBlock key={i} height={12} width={`${60 + i * 10}%`} />
        ))}
      </Stack>
    </Box>
  );
}

export function SkeletonGrid({ columns = 3, count = 6, minHeight = 170 }: { columns?: number; count?: number; minHeight?: number }) {
  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: `repeat(${columns}, 1fr)` },
        gap: 1.4,
      }}
    >
      {Array.from({ length: count }).map((_, i) => (
        <SkeletonCard key={i} minHeight={minHeight} />
      ))}
    </Box>
  );
}
