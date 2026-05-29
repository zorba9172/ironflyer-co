import type { ReactNode } from 'react';
import { Box, Chip, Stack, Typography } from '@mui/material';

// Shared header for every Operate pane: title + live/sample data-source chip +
// a subtitle, with an optional right-aligned actions slot. Keeps the eight
// panes visually identical and removes the repeated header boilerplate.
export function PaneHeader({ title, isLive, subtitle, actions }: { title: string; isLive: boolean; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
      <Typography variant="h4" sx={{ fontSize: '1.6rem' }}>{title}</Typography>
      <Chip
        size="small"
        label={isLive ? 'live' : 'sample'}
        sx={(t) => ({ height: 20, fontSize: '0.64rem', fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
      />
      {subtitle != null && (typeof subtitle === 'string'
        ? <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem' }}>{subtitle}</Typography>
        : subtitle)}
      {actions != null && <><Box sx={{ flex: 1 }} />{actions}</>}
    </Stack>
  );
}
