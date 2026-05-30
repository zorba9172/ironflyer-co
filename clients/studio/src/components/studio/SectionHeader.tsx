import { Box, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';

export type SectionHeaderProps = {
  /** short tracked uppercase label above the title (mx.md › section labels) */
  eyebrow?: string;
  title: ReactNode;
  subtitle?: ReactNode;
  /** trailing controls (toggles, buttons, range pickers) */
  actions?: ReactNode;
  /** small leading mark (icon tile / status dot) */
  lead?: ReactNode;
};

// One consistent section header across every pane.
export function SectionHeader({ eyebrow, title, subtitle, actions, lead }: SectionHeaderProps) {
  return (
    <Stack
      direction="row"
      alignItems={subtitle ? 'flex-start' : 'center'}
      justifyContent="space-between"
      flexWrap="wrap"
      useFlexGap
      sx={{ gap: 2, mb: 2 }}
    >
      <Stack direction="row" alignItems="flex-start" spacing={1.5} sx={{ minWidth: 0 }}>
        {lead}
        <Box sx={{ minWidth: 0 }}>
          {eyebrow && (
            <Typography
              sx={(theme) => ({
                fontFamily: theme.studio ? theme.brand.font.mono : undefined,
                fontSize: '0.66rem',
                letterSpacing: '0.14em',
                textTransform: 'uppercase',
                color: 'text.disabled',
                mb: 0.5,
              })}
            >
              {eyebrow}
            </Typography>
          )}
          <Typography variant="h5" sx={{ fontWeight: 800, letterSpacing: 0, lineHeight: 1.15 }}>
            {title}
          </Typography>
          {subtitle && (
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5, maxWidth: 640 }}>
              {subtitle}
            </Typography>
          )}
        </Box>
      </Stack>
      {actions && (
        <Stack direction="row" alignItems="center" spacing={1} sx={{ flexShrink: 0 }}>
          {actions}
        </Stack>
      )}
    </Stack>
  );
}
