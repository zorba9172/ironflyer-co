// Ironflyer design tokens — output.com-inspired aesthetic.
// Warm near-black foundations, alabaster contrast, blunt display typography,
// and a single electric CTA accent used with restraint.

export const tokens = {
  color: {
    brand: {
      graphite: '#07090d',
      slate: '#10151d',
      paper: '#f5f2ea',
      mint: '#53ffbd',
      cyan: '#5bc8ff',
      amber: '#ffd166',
      ember: '#ff7a45',
    },
    // backgrounds
    bg: {
      base: '#0d0e0f',
      surface: '#151513',
      surfaceRaised: '#1d1d1a',
      surfaceHover: '#262620',
      inset: '#070807',
      alabaster: '#f4f0e8',
      alabasterDeep: '#e7dfd2',
      overlay: 'rgba(13, 14, 15, 0.74)',
    },
    // text
    text: {
      primary: '#f7f3ea',
      secondary: '#b9b3a8',
      muted: '#77736b',
      inverse: '#111111',
    },
    // borders / dividers
    border: {
      subtle: 'rgba(244, 240, 232, 0.12)',
      strong: 'rgba(244, 240, 232, 0.24)',
      accent: 'rgba(229, 255, 0, 0.55)',
    },
    // accents — Output-like loud colors, led by electric lime
    accent: {
      lime: '#e5ff00',
      yellow: '#ffc400',
      red: '#ff1818',
      purple: '#671dfc',
      sky: '#78dbff',
      violet: '#8b5cff',
      coral: '#ff6c3a',
      success: '#79e07a',
      warning: '#ffc400',
      danger: '#ff1818',
    },
  },
  radius: {
    sm: 8,
    md: 12,
    lg: 18,
    xl: 24,
    pill: 999,
  },
  spacing: {
    xs: 4,
    sm: 8,
    md: 16,
    lg: 24,
    xl: 32,
    xxl: 48,
  },
  font: {
    family: 'var(--font-body), "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    display: 'var(--font-display), "Arial Black", "Inter", -apple-system, BlinkMacSystemFont, sans-serif',
    mono: '"Geist Mono", "JetBrains Mono", ui-monospace, SFMono-Regular, monospace',
    weight: {
      regular: 400,
      medium: 500,
      semibold: 600,
      bold: 700,
      black: 900,
    },
    size: {
      xs: '0.75rem',
      sm: '0.875rem',
      md: '1rem',
      lg: '1.125rem',
      xl: '1.5rem',
      xxl: '2.25rem',
      display: '3rem',
    },
  },
  shadow: {
    sm: '0 1px 2px rgba(0,0,0,0.28)',
    md: '0 14px 36px rgba(0,0,0,0.38)',
    lg: '0 32px 90px rgba(0,0,0,0.52)',
  },
  motion: {
    fast: '160ms',
    base: '260ms',
    slow: '520ms',
    curve: 'cubic-bezier(0.16, 1, 0.3, 1)',
    snap: 'cubic-bezier(0.22, 1, 0.36, 1)',
  },
} as const;

export type Tokens = typeof tokens;
