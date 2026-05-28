import type { Theme } from '@mui/material/styles';
import type { GateStatus } from '../studioData';

// Status → theme color. Never hardcode; always resolve from the theme.
export function statusColor(t: Theme, s: GateStatus): string {
  switch (s) {
    case 'closed': return t.palette.success.main;
    case 'running': return t.brand.accent.secondary;
    case 'open': return t.palette.warning.main;
    case 'blocked': return t.palette.error.main;
    default: return t.palette.text.disabled;
  }
}
