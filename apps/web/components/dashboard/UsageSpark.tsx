'use client';

import { Box, Stack, Typography } from '@mui/material';
import { tokens } from '../../lib/theme';

export interface UsagePoint { date: string; value: number }

export function bucketByDay(items: { createdAt: string; costUSD: string }[], days = 30): UsagePoint[] {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const buckets: Map<string, number> = new Map();
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(today);
    d.setDate(today.getDate() - i);
    buckets.set(d.toISOString().slice(0, 10), 0);
  }
  for (const entry of items) {
    const dt = Date.parse(entry.createdAt);
    if (Number.isNaN(dt)) continue;
    const key = new Date(dt).toISOString().slice(0, 10);
    if (!buckets.has(key)) continue;
    const cost = Number(entry.costUSD ?? 0);
    if (!Number.isFinite(cost)) continue;
    buckets.set(key, (buckets.get(key) ?? 0) + cost);
  }
  return Array.from(buckets.entries()).map(([date, value]) => ({ date, value }));
}

interface Props {
  points: UsagePoint[];
  height?: number;
  showAxis?: boolean;
  emptyHint?: string;
  caption?: string;
}

export function UsageSpark({ points, height = 96, showAxis = true, emptyHint, caption }: Props) {
  const hasData = points.some((p) => p.value > 0);
  if (!hasData) {
    return (
      <Box sx={{
        height,
        borderRadius: '8px',
        border: '1px dashed rgba(17,17,17,0.18)',
        bgcolor: 'rgba(17,17,17,0.02)',
        display: 'grid',
        placeItems: 'center',
        px: 2,
      }}>
        <Typography variant="caption" sx={{ color: '#686158', textAlign: 'center' }}>
          {emptyHint ?? 'No usage data to show yet'}
        </Typography>
      </Box>
    );
  }

  const w = 600;
  const h = height;
  const padX = 6;
  const padY = 10;
  const max = Math.max(...points.map((p) => p.value), 0.0001);
  const barW = (w - padX * 2) / points.length;
  const peak = points.reduce((acc, p) => (p.value > acc.value ? p : acc), points[0]);

  return (
    <Box sx={{ width: '100%' }}>
      <Box
        component="svg"
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        sx={{ width: '100%', height, display: 'block' }}
        aria-label="Monthly usage chart"
        role="img"
      >
        <defs>
          <linearGradient id="ironflyerUsageGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={tokens.color.accent.lime} stopOpacity="0.95" />
            <stop offset="100%" stopColor={tokens.color.accent.lime} stopOpacity="0.4" />
          </linearGradient>
        </defs>
        {points.map((p, i) => {
          const barH = Math.max(2, ((p.value / max) * (h - padY * 2)));
          const x = padX + i * barW;
          const y = h - padY - barH;
          const isPeak = p.date === peak.date && p.value > 0;
          return (
            <rect
              key={p.date}
              x={x + 1}
              y={y}
              width={Math.max(1, barW - 2)}
              height={barH}
              rx={2}
              ry={2}
              fill={isPeak ? 'url(#ironflyerUsageGrad)' : 'rgba(17,17,17,0.18)'}
            />
          );
        })}
      </Box>
      {showAxis && (
        <Stack direction="row" justifyContent="space-between" sx={{ mt: 0.6 }}>
          <Typography variant="caption" sx={{ color: '#86807a', fontFamily: tokens.font.mono }}>
            {formatDay(points[0]?.date)}
          </Typography>
          {caption && (
            <Typography variant="caption" sx={{ color: '#4a453e', fontWeight: 800 }}>{caption}</Typography>
          )}
          <Typography variant="caption" sx={{ color: '#86807a', fontFamily: tokens.font.mono }}>
            {formatDay(points[points.length - 1]?.date)}
          </Typography>
        </Stack>
      )}
    </Box>
  );
}

function formatDay(iso?: string) {
  if (!iso) return '';
  const [, m, d] = iso.split('-');
  return `${d}/${m}`;
}
