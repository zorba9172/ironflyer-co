import { Box, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { GlassPanel } from './GlassPanel';

export type StatCardProps = {
  label: string;
  value: ReactNode;
  /** signed % or absolute delta vs. a prior period; green up / red down */
  delta?: number;
  /** plain sub-line under the value when there is no numeric delta */
  hint?: string;
  /** neon accent for the icon tile + rim */
  accent?: string;
  icon?: ReactNode;
  /** optional inline visual (sparkline / mini gauge) pinned to the right */
  visual?: ReactNode;
  onClick?: () => void;
};

// A KPI tile: tracked label, large value, signed delta or hint, optional neon
// icon tile and a right-aligned mini visual. The repeated atom for every
// metrics strip in the studio — built on GlassPanel so it shares the surface
// language and accepts an accent rim.
export function StatCard({ label, value, delta, hint, accent, icon, visual, onClick }: StatCardProps) {
  return (
    <GlassPanel accent={accent} interactive={!!onClick} pad={2.5} onClick={onClick}>
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
                    background: `radial-gradient(120% 120% at 30% 20%, ${tone}33, ${tone}0D 70%)`,
                    border: `1px solid ${tone}33`,
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
          <Typography variant="h4" sx={{ fontWeight: 800, letterSpacing: '-0.015em', lineHeight: 1.1 }}>
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
