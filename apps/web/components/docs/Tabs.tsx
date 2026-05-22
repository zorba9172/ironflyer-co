'use client';

// Tabs — small client-side tab strip for switching between code examples
// (curl / TypeScript / Python). Kept dependency-free so docs pages can
// drop it in without dragging extra MUI tab machinery into the bundle.

import { useState, ReactNode } from 'react';
import { Box, Stack } from '@mui/material';
import { tokens } from '../../../../packages/design-tokens';

export interface TabsProps {
  tabs: Array<{ label: string; content: ReactNode }>;
  defaultIndex?: number;
}

export function Tabs({ tabs, defaultIndex = 0 }: TabsProps) {
  const [index, setIndex] = useState(defaultIndex);

  if (tabs.length === 0) return null;

  return (
    <Box sx={{ my: 3 }}>
      <Stack
        direction="row"
        spacing={0}
        sx={{
          borderBottom: '1px solid rgba(17,17,17,0.10)',
          mb: 1.5,
        }}
        role="tablist"
      >
        {tabs.map((t, i) => {
          const active = i === index;
          return (
            <Box
              key={t.label}
              role="tab"
              aria-selected={active}
              tabIndex={0}
              onClick={() => setIndex(i)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  setIndex(i);
                }
              }}
              sx={{
                px: 2,
                py: 1.1,
                fontFamily: tokens.font.mono,
                fontSize: 12.5,
                fontWeight: 700,
                letterSpacing: '0.04em',
                color: active ? '#111' : '#77736b',
                borderBottom: active ? `2px solid ${tokens.color.accent.lime}` : '2px solid transparent',
                marginBottom: '-1px',
                cursor: 'pointer',
                userSelect: 'none',
                transition: `color ${tokens.motion.fast} ${tokens.motion.curve}, border-color ${tokens.motion.fast} ${tokens.motion.curve}`,
                '&:hover': { color: '#111' },
              }}
            >
              {t.label}
            </Box>
          );
        })}
      </Stack>
      <Box>{tabs[index]?.content}</Box>
    </Box>
  );
}

export default Tabs;
