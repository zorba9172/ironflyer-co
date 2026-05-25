// motion — reusable motion tokens for cockpit microinteractions.
//
// These re-export the durations + curves declared in
// packages/design-tokens so component code can lean on a single import
// (`motion.fast`, `motion.curve`, …) instead of reaching into the
// tokens tree for every transition string. They are kept in sync with
// `tokens.motion` — if a value diverges, the tokens module wins.
//
// Usage examples:
//
//   transition: `border-color ${motion.fast} ${motion.curve}`,
//   animation: `${pulse} ${motion.slow} ${motion.curve} infinite`,
//
// The values mirror the Base44 microinteraction grammar: fast (160ms)
// for hover state changes, base (220ms) for collapse/expand panels,
// slow (420ms) for staged reveals, and the two cubic-bezier curves —
// `curve` for elastic reveals and `snap` for crisp transforms.

import { tokens } from "../../../../../packages/design-tokens";

export const motion = {
  fast: tokens.motion.fast,
  base: "220ms",
  slow: "420ms",
  curve: tokens.motion.curve,
  snap: tokens.motion.snap,
} as const;

// Common composed transition strings — saves repeating the pattern in
// every component that wants a Base44-grade hover.
export const transitions = {
  hover: `border-color ${motion.fast} ${motion.curve}, box-shadow ${motion.fast} ${motion.curve}, transform ${motion.fast} ${motion.snap}`,
  collapse: `width ${motion.base} ${motion.curve}, flex-basis ${motion.base} ${motion.curve}`,
  fade: `opacity ${motion.base} ${motion.curve}`,
} as const;

// Reusable keyframes (as object literals — keep MUI-friendly).
export const keyframes = {
  // Gentle dot pulse for live/running status indicators.
  livePulse: {
    "0%, 100%": { opacity: 1, transform: "scale(1)" },
    "50%": { opacity: 0.55, transform: "scale(0.85)" },
  },
  // Shimmer for skeleton bars.
  shimmer: {
    "0%": { backgroundPosition: "-200% 0" },
    "100%": { backgroundPosition: "200% 0" },
  },
} as const;

export type Motion = typeof motion;
