import { Box, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { GlassPanel, type GlassPanelProps } from './GlassPanel';

export type StatCardProps = {
  label: string;
  value: ReactNode;
  /** signed % or absolute delta vs. a prior period; green up / red down */
  delta?: number;
  /** plain sub-line under the value when there is no numeric delta */
  hint?: string;
  /** semantic accent for the icon tile + focus edge */
  accent?: string;
  icon?: ReactNode;
  /** optional inline visual (sparkline / mini gauge) pinned to the right */
  visual?: ReactNode;
  onClick?: () => void;
  sx?: GlassPanelProps['sx'];
};

// A calm KPI tile for dense product pages.
export function StatCard({ label, value, delta, hint, accent, icon, visual, onClick, sx }: StatCardProps) {
  return (
    <GlassPanel accent={accent} interactive={!!onClick} pad={2.5} onClick={onClick} sx={sx}>
      <Stack direction="row" alignItems="flex-start" justifyContent="space-between" spacing={1.5}>
        <Box sx={{ minWidth: 0 }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
            {icon && (
              <Box
                aria-hidden
                sx={(theme) => {
                  const tone = accent ?? theme.studio.neon.blue;
                  return {
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: 30,
                    height: 30,
                    borderRadius: `${theme.studio.radius.sm}px`,
                    color: tone,
                    fontSize: 16,
                    backgroundColor: theme.palette.surfaceHover,
                    border: `1px solid ${theme.palette.divider}`,
                  };
                }}
              >
                {icon}
              </Box>
            )}
            <Typography
              sx={(theme) => ({
                fontFamily: theme.brand.font.mono,
                fontSize: '0.64rem',
                letterSpacing: '0.12em',
                textTransform: 'uppercase',
                color: 'text.disabled',
              })}
            >
              {label}
            </Typography>
          </Stack>
          <Typography variant="h4" sx={{ fontWeight: 800, letterSpacing: 0, lineHeight: 1.1 }}>
            {value}
          </Typography>
          {delta != null ? (
            <Typography
              variant="caption"
              sx={{ mt: 0.5, display: 'block', fontWeight: 700, color: delta >= 0 ? 'success.main' : 'error.main' }}
            >
              {delta >= 0 ? '▲' : '▼'} {Math.abs(delta)}%
            </Typography>
          ) : hint ? (
            <Typography variant="caption" color="text.secondary" sx={{ mt: 0.5, display: 'block' }}>
              {hint}
            </Typography>
          ) : null}
        </Box>
        {visual && <Box sx={{ flexShrink: 0 }}>{visual}</Box>}
      </Stack>
    </GlassPanel>
  );
}
