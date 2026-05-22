'use client';

import { FormatQuote } from '@mui/icons-material';
import { Box, Stack, Typography } from '@mui/material';
import { tokens } from '../../../packages/design-tokens';

interface Testimonial {
  quote: string;
  name: string;
  role: string;
}

// TestimonialMarquee renders a horizontal scroll list of quote cards.
// We double the list and animate the whole rail with a CSS keyframe — gives
// a continuous loop without dragging in a carousel library. Pauses on hover
// so visitors can finish reading.

export function TestimonialMarquee({ items }: { items: Testimonial[] }) {
  const doubled = [...items, ...items];
  return (
    <Box sx={{
      position: 'relative',
      overflow: 'hidden',
      mx: { xs: -2, md: 0 },
      // Edge fade so the rail looks like it disappears into the alabaster.
      maskImage: 'linear-gradient(90deg, transparent 0, #000 6%, #000 94%, transparent 100%)',
      WebkitMaskImage: 'linear-gradient(90deg, transparent 0, #000 6%, #000 94%, transparent 100%)',
    }}>
      <Box sx={{
        display: 'flex',
        gap: 2,
        width: 'max-content',
        animation: 'iflyer-marquee 48s linear infinite',
        '&:hover': { animationPlayState: 'paused' },
        '@keyframes iflyer-marquee': {
          '0%':   { transform: 'translateX(0)' },
          '100%': { transform: 'translateX(-50%)' },
        },
      }}>
        {doubled.map((t, idx) => (
          <Box key={`${t.name}-${idx}`} sx={{
            width: { xs: 320, md: 420 },
            flexShrink: 0,
            bgcolor: '#ece5d4',
            borderRadius: 4,
            p: { xs: 3, md: 3.6 },
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
            border: '1px solid rgba(13,14,15,0.06)',
          }}>
            <FormatQuote sx={{ color: tokens.color.accent.lime, fontSize: 30, transform: 'scaleX(-1)' }} />
            <Typography sx={{
              fontSize: { xs: 15.5, md: 17 },
              fontWeight: 600,
              lineHeight: 1.5,
              color: '#0d0e0f',
              flex: 1,
            }}>
              “{t.quote}”
            </Typography>
            <Stack direction="row" spacing={1.6} alignItems="center" sx={{ pt: 1.2, borderTop: '1px solid rgba(13,14,15,0.08)' }}>
              <Box sx={{
                width: 38, height: 38, borderRadius: '999px',
                background: `linear-gradient(135deg, ${tokens.color.accent.lime}, #0d0e0f)`,
                display: 'grid', placeItems: 'center',
                color: '#0d0e0f', fontWeight: 900,
                fontFamily: tokens.font.display, fontSize: 16,
              }}>
                {t.name.split(' ').map((n) => n[0]).join('').slice(0, 2)}
              </Box>
              <Box>
                <Typography sx={{ fontWeight: 800, fontSize: 14.5, color: '#0d0e0f' }}>{t.name}</Typography>
                <Typography variant="caption" sx={{ color: '#5b554b', fontWeight: 600 }}>{t.role}</Typography>
              </Box>
            </Stack>
          </Box>
        ))}
      </Box>
    </Box>
  );
}
