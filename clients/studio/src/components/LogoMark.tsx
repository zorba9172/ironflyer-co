import { useId } from 'react';
import { neon } from '../theme';

// Ironflyer mark — a gate-forward "horizon" symbol in the aurora signature.
// SVG fills are a legal non-sx token context (DESIGN_CONSTITUTION › Tokens Law):
// the gradient stops read straight from the studio palette, never a literal.
export function LogoMark({ size = 24 }: { size?: number }) {
  const id = useId();
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" aria-hidden="true">
      <defs>
        <linearGradient id={id} x1="6" y1="6" x2="34" y2="34" gradientUnits="userSpaceOnUse">
          <stop stopColor={neon.indigo} />
          <stop offset="0.5" stopColor={neon.violet} />
          <stop offset="1" stopColor={neon.pink} />
        </linearGradient>
      </defs>
      <circle cx="20" cy="20" r="16" fill={`url(#${id})`} />
      <path d="M7 22h26M9 26h22M13 30h14" stroke="#FFFFFF" strokeWidth="2.4" strokeLinecap="round" />
    </svg>
  );
}
