import type { Theme } from '@mui/material/styles';

// ─────────────────────────────────────────────────────────────────────────
// Status taxonomy for the projects portfolio. The orchestrator returns a free
// text status; we fold it into three semantic buckets — Shipped / Building /
// Blocked — each mapped to a single semantic neon mark. Colors are resolved
// from the theme at call sites; this module only classifies + labels.
// ─────────────────────────────────────────────────────────────────────────

export type StatusBucket = 'shipped' | 'building' | 'blocked';

export function bucketFor(status: string): StatusBucket {
  const s = (status || '').toLowerCase();
  if (s.includes('ship') || s.includes('done') || s.includes('complete') || s.includes('live') || s.includes('deploy')) {
    return 'shipped';
  }
  if (s.includes('error') || s.includes('block') || s.includes('fail') || s.includes('stuck')) {
    return 'blocked';
  }
  return 'building';
}

export const BUCKET_LABEL: Record<StatusBucket, string> = {
  shipped: 'Shipped',
  building: 'Building',
  blocked: 'Blocked',
};

// One semantic neon mark per bucket, read from the live theme.
export function bucketColor(theme: Theme, bucket: StatusBucket): string {
  switch (bucket) {
    case 'shipped':
      return theme.studio.neon.success;
    case 'blocked':
      return theme.studio.neon.danger;
    default:
      return theme.studio.neon.blue;
  }
}

export const BUCKET_ORDER: readonly StatusBucket[] = ['building', 'shipped', 'blocked'];
