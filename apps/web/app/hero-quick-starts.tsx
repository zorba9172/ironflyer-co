'use client';

import { Chip, Stack } from '@mui/material';
import { tokens } from '../../../packages/design-tokens';

interface QuickStart {
  label: string;
  prompt: string;
}

interface Props {
  items: QuickStart[];
}

// HeroQuickStarts is the row of pill chips below the hero PromptBox.
// Each chip seeds the dashboard's pendingIdea bucket (read by /app on
// mount) and routes the visitor straight into the workspace where the
// finisher loop picks the prompt up. Marketing pages are server
// components, so the click handler lives in this client island.
export function HeroQuickStarts({ items }: Props) {
  function seedAndGo(label: string, value: string) {
    if (typeof window === 'undefined') return;
    try {
      window.localStorage.setItem('ironflyer.pendingIdea', value);
      window.localStorage.setItem('ironflyer.pendingIdea.label', label);
    } catch {
      // Private mode / quota — fall through; /app opens empty.
    }
    window.location.href = '/app';
  }

  return (
    <Stack
      direction="row"
      spacing={1}
      flexWrap="wrap"
      justifyContent="center"
      sx={{ mt: 1.8, rowGap: 1 }}
    >
      {items.map((q) => (
        <Chip
          key={q.label}
          label={q.label}
          clickable
          onClick={() => seedAndGo(q.label, q.prompt)}
          sx={{
            bgcolor: 'rgba(244,240,232,0.92)',
            color: '#111',
            border: '1px solid rgba(17,17,17,0.08)',
            fontWeight: 800,
            px: 0.3,
            '& .MuiChip-label': { px: 1.4 },
            '&:hover': { bgcolor: tokens.color.accent.lime, color: '#0d0e0f' },
            transition: `background-color ${tokens.motion.base} ${tokens.motion.curve}`,
          }}
        />
      ))}
    </Stack>
  );
}
