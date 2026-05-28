// Ironflyer international brand tokens (2026-05-28).
// A deliberate break from the legacy violet/coral identity in ./index.ts.
// See docs/BRAND_SYSTEM_2026-05-28.md. Single source of truth — never
// hardcode these values in app code; consume via the MUI/RN theme or the
// CSS custom properties emitted by `cssVars()`.

export type ThemeMode = 'light' | 'dark';

// Mode-independent brand marks. The cobalt→cyan gradient is the single
// recognizable signature; amber is the warm signal accent.
export const palette = {
  cobalt: '#2F6BFF',
  cobaltDeep: '#1B4DE0',
  cyan: '#18C8E6',
  amber: '#FFB020',
  emerald: '#16B981',
  rose: '#F43F5E',
  ink: '#0A0B0D',
  paper: '#FAF9F6',
} as const;

export const gradient = {
  // Primary CTA / brand signature.
  signature: 'linear-gradient(100deg, #2F6BFF 0%, #18C8E6 100%)',
  signatureSoft: 'linear-gradient(100deg, rgba(47,107,255,0.16), rgba(24,200,230,0.16))',
  // Editorial display text fill.
  ink: 'linear-gradient(180deg, #FFFFFF 0%, #A4A9B3 140%)',
} as const;

type ModeColors = {
  bg: string;
  bgSubtle: string;
  surface: string;
  surfaceRaised: string;
  surfaceHover: string;
  borderSubtle: string;
  borderStrong: string;
  textPrimary: string;
  textSecondary: string;
  textMuted: string;
  textInverse: string;
  overlay: string;
};

export const modes: Record<ThemeMode, ModeColors> = {
  dark: {
    bg: '#0A0B0D',
    bgSubtle: '#0E0F12',
    surface: '#111317',
    surfaceRaised: '#171A1F',
    surfaceHover: '#1D2127',
    borderSubtle: 'rgba(255,255,255,0.08)',
    borderStrong: 'rgba(255,255,255,0.16)',
    textPrimary: '#F4F5F7',
    textSecondary: '#A4A9B3',
    textMuted: '#6B7280',
    textInverse: '#0A0B0D',
    overlay: 'rgba(10,11,13,0.72)',
  },
  light: {
    bg: '#FAF9F6',
    bgSubtle: '#F3F1EC',
    surface: '#FFFFFF',
    surfaceRaised: '#FFFFFF',
    surfaceHover: '#F3F1EC',
    borderSubtle: 'rgba(10,11,13,0.08)',
    borderStrong: 'rgba(10,11,13,0.16)',
    textPrimary: '#0A0B0D',
    textSecondary: '#4A4F58',
    textMuted: '#787E88',
    textInverse: '#FAF9F6',
    overlay: 'rgba(250,249,246,0.72)',
  },
};

// Semantic accents are mode-independent (tuned to read on both canvases).
export const accent = {
  primary: palette.cobalt,
  primaryHover: palette.cobaltDeep,
  secondary: palette.cyan,
  signal: palette.amber,
  success: palette.emerald,
  danger: palette.rose,
  focus: palette.cobalt,
} as const;

export const typography = {
  display: '"Bricolage Grotesque", "Inter", system-ui, sans-serif',
  body: '"Inter", system-ui, -apple-system, sans-serif',
  mono: '"Geist Mono", ui-monospace, "SF Mono", monospace',
  // rem-based scale
  size: {
    displayXl: '4.5rem',
    displayLg: '3.375rem',
    displayMd: '2.5rem',
    h1: '2rem',
    h2: '1.5rem',
    h3: '1.25rem',
    body: '1rem',
    small: '0.875rem',
    label: '0.75rem',
  },
  weight: { regular: 400, medium: 500, semibold: 600, bold: 700 },
  leading: { tight: 1.05, snug: 1.25, normal: 1.5 },
  tracking: { tight: '-0.02em', normal: '0', wide: '0.04em' },
} as const;

export const radius = { xs: 6, sm: 8, md: 12, lg: 16, xl: 24, pill: 999 } as const;

export const space = {
  0: 0, 1: 4, 2: 8, 3: 12, 4: 16, 5: 24, 6: 32, 7: 48, 8: 64, 9: 96, 10: 128,
} as const;

export const shadow = {
  sm: '0 1px 2px rgba(10,11,13,0.24)',
  md: '0 8px 24px rgba(10,11,13,0.28)',
  lg: '0 24px 64px rgba(10,11,13,0.36)',
  glow: '0 0 0 1px rgba(47,107,255,0.4), 0 8px 32px rgba(47,107,255,0.24)',
} as const;

// Dark/light swap timing. See BRAND_SYSTEM §5.
export const motion = {
  themeTransition: '180ms ease-out',
  fast: '120ms ease-out',
  base: '220ms cubic-bezier(0.22, 1, 0.36, 1)',
  slow: '420ms cubic-bezier(0.22, 1, 0.36, 1)',
} as const;

export const brand = {
  palette,
  gradient,
  modes,
  accent,
  typography,
  radius,
  space,
  shadow,
  motion,
} as const;

// Emit theme as CSS custom properties (consumed by Astro/marketing and any
// non-MUI surface). Keys are stable: --if-<group>-<name>.
export function cssVars(mode: ThemeMode): Record<string, string> {
  const m = modes[mode];
  return {
    '--if-bg': m.bg,
    '--if-bg-subtle': m.bgSubtle,
    '--if-surface': m.surface,
    '--if-surface-raised': m.surfaceRaised,
    '--if-surface-hover': m.surfaceHover,
    '--if-border-subtle': m.borderSubtle,
    '--if-border-strong': m.borderStrong,
    '--if-text-primary': m.textPrimary,
    '--if-text-secondary': m.textSecondary,
    '--if-text-muted': m.textMuted,
    '--if-text-inverse': m.textInverse,
    '--if-overlay': m.overlay,
    '--if-accent-primary': accent.primary,
    '--if-accent-primary-hover': accent.primaryHover,
    '--if-accent-secondary': accent.secondary,
    '--if-accent-signal': accent.signal,
    '--if-accent-success': accent.success,
    '--if-accent-danger': accent.danger,
    '--if-gradient-signature': gradient.signature,
    '--if-gradient-signature-soft': gradient.signatureSoft,
    '--if-font-display': typography.display,
    '--if-font-body': typography.body,
    '--if-font-mono': typography.mono,
    '--if-theme-transition': motion.themeTransition,
  };
}

export function cssVarsBlock(mode: ThemeMode): string {
  return Object.entries(cssVars(mode))
    .map(([k, v]) => `  ${k}: ${v};`)
    .join('\n');
}
