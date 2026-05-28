import { palette } from '@ironflyer/design-tokens/brand';
import { useId } from 'react';

// The mark fill is the brand signature gradient; stops come from tokens.
export function LogoMark({ size = 26 }: { size?: number }) {
  const id = useId();
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" aria-hidden="true">
      <defs>
        <linearGradient id={id} x1="6" y1="34" x2="34" y2="6" gradientUnits="userSpaceOnUse">
          <stop stopColor={palette.cobalt} />
          <stop offset="1" stopColor={palette.cyan} />
        </linearGradient>
      </defs>
      <path d="M6 34 L20 6 L34 34 L20 26 Z" fill={`url(#${id})`} />
    </svg>
  );
}
