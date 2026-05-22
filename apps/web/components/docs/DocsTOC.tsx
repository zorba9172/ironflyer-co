'use client';

// DocsTOC — right-rail "On this page" navigation. Uses an IntersectionObserver
// to highlight the section currently in view. Kept as a separate client
// component so DocPage itself can stay a server component.

import { useEffect, useState } from 'react';
import { Box, Stack, Typography } from '@mui/material';
import { tokens } from '../../../../packages/design-tokens';
import type { DocTOCEntry } from './DocPage';

export interface DocsTOCProps {
  entries: DocTOCEntry[];
}

export function DocsTOC({ entries }: DocsTOCProps) {
  const [activeId, setActiveId] = useState<string>(entries[0]?.id ?? '');

  useEffect(() => {
    if (entries.length === 0) return;
    const nodes = entries
      .map((e) => document.getElementById(e.id))
      .filter((n): n is HTMLElement => Boolean(n));
    if (nodes.length === 0) return;

    const obs = new IntersectionObserver(
      (records) => {
        const visible = records
          .filter((r) => r.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible.length > 0) {
          setActiveId(visible[0].target.id);
        }
      },
      { rootMargin: '-88px 0px -65% 0px', threshold: 0 }
    );
    nodes.forEach((n) => obs.observe(n));
    return () => obs.disconnect();
  }, [entries]);

  return (
    <Box
      component="aside"
      aria-label="On this page"
      sx={{
        display: { xs: 'none', lg: 'block' },
        position: 'sticky',
        top: 88,
        alignSelf: 'flex-start',
        maxHeight: 'calc(100vh - 100px)',
        overflowY: 'auto',
      }}
    >
      <Typography
        variant="overline"
        sx={{
          color: '#5c5750',
          fontSize: 11,
          letterSpacing: '0.16em',
          fontWeight: 800,
          mb: 1.4,
          display: 'block',
        }}
      >
        On this page
      </Typography>
      <Stack spacing={0.4}>
        {entries.map((e) => {
          const active = activeId === e.id;
          return (
            <a
              key={e.id}
              href={`#${e.id}`}
              style={{ textDecoration: 'none' }}
            >
              <Typography
                sx={{
                  fontSize: 13,
                  color: active ? '#111' : '#77736b',
                  fontWeight: active ? 700 : 500,
                  borderLeft: active ? `2px solid ${tokens.color.accent.lime}` : '2px solid transparent',
                  py: 0.4,
                  pl: e.depth === 3 ? '22px' : '12px',
                  transition: `color ${tokens.motion.fast} ${tokens.motion.curve}, border-color ${tokens.motion.fast} ${tokens.motion.curve}`,
                  '&:hover': { color: '#111' },
                }}
              >
                {e.label}
              </Typography>
            </a>
          );
        })}
      </Stack>
    </Box>
  );
}

export default DocsTOC;
