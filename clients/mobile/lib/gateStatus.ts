import type { NativeTheme } from '@ironflyer/ui-native';
import type { GateStatus } from './sampleData';

// Map a gate status to a brand color + readable label. Colors come only from
// the native theme (which derives from @ironflyer/design-tokens), never raw hex.
export function gateStatusColor(theme: NativeTheme, status: GateStatus): string {
  switch (status) {
    case 'closed':
      return theme.color.success;
    case 'open':
      return theme.color.signal;
    case 'blocked':
      return theme.color.danger;
  }
}

export function gateStatusLabel(status: GateStatus): string {
  switch (status) {
    case 'closed':
      return 'Closed';
    case 'open':
      return 'Open';
    case 'blocked':
      return 'Blocked';
  }
}
