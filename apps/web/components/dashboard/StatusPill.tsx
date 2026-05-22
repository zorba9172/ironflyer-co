'use client';

import { Chip, ChipProps } from '@mui/material';
import { tokens } from '../../lib/theme';

type StatusKind =
  | 'ready' | 'running' | 'pending' | 'failed' | 'passed' | 'blocked'
  | 'repaired' | 'idle' | 'connected' | 'disconnected' | 'missing'
  | 'available' | 'success' | 'warning' | 'danger' | 'neutral';

const palette: Record<StatusKind, { bg: string; color: string; border: string }> = {
  ready:        { bg: 'rgba(121,224,122,0.22)', color: '#1f5a20', border: 'rgba(31,90,32,0.28)' },
  passed:       { bg: 'rgba(121,224,122,0.22)', color: '#1f5a20', border: 'rgba(31,90,32,0.28)' },
  success:      { bg: 'rgba(121,224,122,0.22)', color: '#1f5a20', border: 'rgba(31,90,32,0.28)' },
  connected:    { bg: 'rgba(121,224,122,0.22)', color: '#1f5a20', border: 'rgba(31,90,32,0.28)' },
  available:    { bg: 'rgba(229,255,0,0.24)', color: '#5a6700', border: 'rgba(17,17,17,0.16)' },
  running:      { bg: 'rgba(120,219,255,0.22)', color: '#0f4f6a', border: 'rgba(15,79,106,0.26)' },
  pending:      { bg: 'rgba(255,196,0,0.22)', color: '#7a5b00', border: 'rgba(122,91,0,0.26)' },
  warning:      { bg: 'rgba(255,196,0,0.22)', color: '#7a5b00', border: 'rgba(122,91,0,0.26)' },
  failed:       { bg: 'rgba(255,24,24,0.16)', color: '#9b1010', border: 'rgba(155,16,16,0.28)' },
  danger:       { bg: 'rgba(255,24,24,0.16)', color: '#9b1010', border: 'rgba(155,16,16,0.28)' },
  blocked:      { bg: 'rgba(255,108,58,0.18)', color: '#8c3914', border: 'rgba(140,57,20,0.28)' },
  repaired:     { bg: 'rgba(139,92,255,0.18)', color: '#3a248a', border: 'rgba(58,36,138,0.26)' },
  idle:         { bg: 'rgba(17,17,17,0.06)', color: '#4a453e', border: 'rgba(17,17,17,0.16)' },
  neutral:      { bg: 'rgba(17,17,17,0.06)', color: '#4a453e', border: 'rgba(17,17,17,0.16)' },
  disconnected: { bg: 'rgba(17,17,17,0.06)', color: '#4a453e', border: 'rgba(17,17,17,0.16)' },
  missing:      { bg: 'rgba(255,108,58,0.18)', color: '#8c3914', border: 'rgba(140,57,20,0.28)' },
};

export function statusKindFromGate(value: string | undefined | null): StatusKind {
  const s = (value ?? '').toLowerCase();
  if (s === 'passed' || s === 'ready' || s === 'success') return 'passed';
  if (s === 'running') return 'running';
  if (s === 'pending') return 'pending';
  if (s === 'failed') return 'failed';
  if (s === 'blocked') return 'blocked';
  if (s === 'repaired') return 'repaired';
  if (s === 'connected') return 'connected';
  return 'idle';
}

interface Props extends Omit<ChipProps, 'color'> {
  kind?: StatusKind;
  label: string;
}

export function StatusPill({ kind = 'neutral', label, sx, ...rest }: Props) {
  const c = palette[kind] ?? palette.neutral;
  return (
    <Chip
      size="small"
      label={label}
      sx={{
        borderRadius: '6px',
        height: 22,
        fontSize: '0.72rem',
        fontWeight: 800,
        letterSpacing: 0.2,
        bgcolor: c.bg,
        color: c.color,
        border: `1px solid ${c.border}`,
        '& .MuiChip-label': { px: 0.85 },
        ...sx,
      }}
      {...rest}
    />
  );
}

// Re-export for callers that want to reference a token color directly.
export const statusPillTokens = { lime: tokens.color.accent.lime };
