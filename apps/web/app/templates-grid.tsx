'use client';

import { ArrowForward, AutoAwesome } from '@mui/icons-material';
import { Box, Chip, Stack, Typography } from '@mui/material';
import { tokens } from '../../../packages/design-tokens';

export interface TemplateItem {
  title: string;
  desc: string;
  tag: string;
  prompt: string;
  accent: string;
}

// TemplatesGrid is the client island that handles clicks: we seed the
// localStorage bucket /app reads on mount with the template's prompt, then
// hard-redirect to the workspace. Same pattern as HeroQuickStarts.

export function TemplatesGrid({ items }: { items: TemplateItem[] }) {
  function seedAndGo(t: TemplateItem) {
    if (typeof window === 'undefined') return;
    try {
      window.localStorage.setItem('ironflyer.pendingIdea', t.prompt);
      window.localStorage.setItem('ironflyer.pendingIdea.label', t.title);
    } catch {
      // private mode / quota — fall through
    }
    window.location.href = '/app';
  }

  return (
    <Box sx={{
      display: 'grid',
      gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(3, 1fr)' },
      gap: { xs: 2, md: 2.5 },
    }}>
      {items.map((t) => (
        <Box
          key={t.title}
          role="button"
          tabIndex={0}
          onClick={() => seedAndGo(t)}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); seedAndGo(t); } }}
          sx={{
            display: 'flex',
            flexDirection: 'column',
            cursor: 'pointer',
            color: '#0d0e0f',
            borderRadius: 4,
            overflow: 'hidden',
            bgcolor: '#ece5d4',
            transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, box-shadow ${tokens.motion.base} ${tokens.motion.curve}`,
            outline: 'none',
            '&:hover, &:focus-visible': {
              transform: 'translateY(-4px)',
              boxShadow: '0 24px 56px rgba(13,14,15,0.16)',
            },
            '&:focus-visible': {
              outline: `2px solid ${tokens.color.accent.lime}`,
              outlineOffset: 2,
            },
          }}
        >
          <Box sx={{
            position: 'relative',
            height: 180,
            background: `linear-gradient(135deg, ${t.accent}, #0d0e0f 180%)`,
            overflow: 'hidden',
          }}>
            <Box sx={{
              position: 'absolute', inset: 0,
              backgroundImage: 'radial-gradient(circle at 20% 20%, rgba(255,255,255,0.12), transparent 50%)',
            }} />
            <Stack direction="row" justifyContent="space-between" alignItems="flex-start" sx={{ p: 2.4, position: 'relative' }}>
              <Chip
                label={t.tag}
                size="small"
                sx={{
                  bgcolor: 'rgba(13,14,15,0.7)',
                  color: '#f4f0e8',
                  borderRadius: '999px',
                  fontWeight: 800,
                  fontSize: 11,
                  backdropFilter: 'blur(6px)',
                }}
              />
              <AutoAwesome sx={{ color: 'rgba(13,14,15,0.7)', fontSize: 22 }} />
            </Stack>
          </Box>
          <Box sx={{ p: 2.6, flex: 1, display: 'flex', flexDirection: 'column' }}>
            <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: 24, lineHeight: 1, letterSpacing: 0 }}>
              {t.title}
            </Typography>
            <Typography variant="body2" sx={{ mt: 1.4, color: '#5b554b', lineHeight: 1.5, flex: 1 }}>
              {t.desc}
            </Typography>
            <Stack direction="row" alignItems="center" spacing={0.8} sx={{ mt: 2.4, color: '#0d0e0f', fontWeight: 800, fontSize: 14 }}>
              <Typography variant="body2" sx={{ fontWeight: 800 }}>Open in prompt</Typography>
              <ArrowForward sx={{ fontSize: 16 }} />
            </Stack>
          </Box>
        </Box>
      ))}
    </Box>
  );
}
