// IronFlyer visual reference tokens.
// Locked to the private 2026-05-25 dark SaaS handoff: deep space base,
// violet surfaces, orange-to-magenta CTAs, tight 8px radii.

export const tokens = {
  color: {
    brand: {
      graphite: '#050612',
      slate: '#0b0d1d',
      paper: '#f7f5ff',
      mint: '#53ffbd',
      cyan: '#8fc7ff',
      amber: '#ff9f43',
      ember: '#ff6f3c',
      // Magenta — the middle stop in the primary CTA gradient (coral
      // → magenta → purple). Lives on brand so MarketingLandingPage
      // and theme.containedPrimary can both reference it without
      // hardcoding the hex inline.
      magenta: '#e149c9',
    },
    // backgrounds
    bg: {
      base: '#050612',
      surface: '#0c0d20',
      surfaceRaised: '#11132a',
      surfaceHover: '#191538',
      inset: '#080918',
      alabaster: '#f4f0e8',
      alabasterDeep: '#e7dfd2',
      overlay: 'rgba(5, 6, 18, 0.78)',
    },
    // text
    text: {
      primary: '#f7f4ff',
      secondary: '#b9b2d3',
      muted: '#777096',
      inverse: '#090816',
    },
    // borders / dividers
    border: {
      subtle: 'rgba(178, 133, 255, 0.16)',
      strong: 'rgba(187, 147, 255, 0.34)',
      accent: 'rgba(154, 86, 255, 0.72)',
    },
    // accents — private handoff palette: violet product glow + warm CTA.
    accent: {
      lime: '#9dff7a',
      yellow: '#ffb457',
      red: '#ff4f6d',
      purple: '#8f4dff',
      sky: '#7eb7ff',
      violet: '#b56cff',
      coral: '#ff7848',
      success: '#7fe28a',
      warning: '#ffb457',
      danger: '#ff4f6d',
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
    display: 'var(--font-body), "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
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
    sm: '0 1px 2px rgba(0,0,0,0.32)',
    md: '0 18px 44px rgba(0,0,0,0.42)',
    lg: '0 36px 110px rgba(54,18,116,0.46)',
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
