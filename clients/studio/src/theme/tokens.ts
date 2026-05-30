// ─────────────────────────────────────────────────────────────────────────
// IRONFLYER STUDIO — PRODUCT WORKSPACE DESIGN TOKENS
// ─────────────────────────────────────────────────────────────────────────
// Single source of truth for the studio surface. Derived verbatim from the
// locked designer handoff:
//   clients/studio/design_refernce/mx.md  (the design system spec)
//   clients/studio/design_refernce/*.png  (the three locked renders)
//
// CONSTITUTIONAL LAW (see clients/studio/DESIGN_CONSTITUTION.md):
//   • This file is the ONLY place raw hex / rgba literals may live for the
//     studio. Components NEVER inline a color, gradient, blur, radius, or
//     motion value — they read it from the MUI theme (theme.palette.*) or by
//     importing `studioTokens` directly (legal for non-sx contexts: keyframes,
//     SVG fills, canvas).
//   • Mode-aware values (canvas / surface / text / border) live in `modes`
//     and are mapped into the MUI palette so they flip on the dark/light
//     toggle automatically.
//   • Mode-INDEPENDENT brand marks (the neon gradient, the accent hues, the
//     ambient glow recipe, the prompt-builder glow, motion) live here and read
//     identically in both schemes — the neon signature never changes.
// ─────────────────────────────────────────────────────────────────────────

export type StudioMode = 'dark' | 'light';

// ── Neon brand marks (mode-independent) ────────────────────────────────────
// mx.md › Accent Colors. The recognizable signature is the blue→violet→pink
// arc; success/warning/danger are tuned to read on both canvases.
export const neon = {
  blue: '#2563EB',
  violet: '#F47A45',
  purple: '#FB8A4C',
  pink: '#FFB088',
  success: '#059669',
  warning: '#D97706',
  danger: '#DC2626',
} as const;

// mx.md › Neon Gradient + CTA Button. Never used on large flat surfaces —
// only CTAs, the final headline phrase, active states, and energy edges.
export const gradient = {
  signature: 'linear-gradient(100deg, #F47A45 0%, #FB8A4C 52%, #FFB088 100%)',
  cta: 'linear-gradient(100deg, #F47A45 0%, #F2672E 100%)',
  soft: 'linear-gradient(180deg, rgba(244,122,69,0.10), rgba(244,122,69,0.02))',
} as const;

// ── Mode canvases (mapped into the MUI palette → flip on toggle) ────────────
// mx.md › Color System. Dark is the primary brand canvas (#050816). Light is
// the daytime counterpart shown in the dashboard render; the prompt builder
// stays dark in BOTH modes (it is the always-neon centerpiece).
type ModeColors = {
  bg: string; // page canvas
  bgRaised: string; // secondary canvas / inset
  surface: string; // floating panel
  surfaceHover: string; // panel hover
  border: string; // strong hairline
  borderSubtle: string; // faint hairline
  cardBg: string; // floating glass card fill
  cardBorder: string; // floating glass card edge
  textPrimary: string;
  textSecondary: string;
  textMuted: string;
};

export const modes: Record<StudioMode, ModeColors> = {
  dark: {
    bg: '#0B1220',
    bgRaised: '#111827',
    surface: '#111827',
    surfaceHover: '#1F2937',
    border: '#374151',
    borderSubtle: '#1F2937',
    cardBg: '#111827',
    cardBorder: '#374151',
    textPrimary: '#F9FAFB',
    textSecondary: '#D1D5DB',
    textMuted: '#9CA3AF',
  },
  light: {
    bg: '#F6F4F1',
    bgRaised: '#FFFFFF',
    surface: '#FFFFFF',
    surfaceHover: '#F1EFEC',
    border: '#DEDAD4',
    borderSubtle: '#E9E5DF',
    cardBg: '#FFFFFF',
    cardBorder: '#E7E2DA',
    textPrimary: '#181614',
    textSecondary: '#5F5B56',
    textMuted: '#9C958D',
  },
};

// ── Effect recipes (mode-independent specs, verbatim from mx.md) ────────────
export const effect = {
  // mx.md › Prompt Builder — the centerpiece. Always dark, in both modes.
  promptBuilder: {
    radius: 12,
    borderColor: '#E7E2DA',
    glow: '0 1px 2px rgba(17,24,39,0.04)',
    bg: '#FFFFFF',
    blur: 0,
  },
  // mx.md › Cards — "cards should not feel like cards."
  card: {
    radius: 8,
    bg: '#FFFFFF',
    border: '#E7E2DA',
    blur: 0,
  },
  // mx.md › CTA Button.
  cta: { height: 44, radius: 8 },
  // mx.md › Ambient Effects — massive radial glows behind the hero, 5–15%
  // opacity, no visible circles, only atmosphere. Top-left blue, center
  // violet, bottom-right pink.
  ambient: {
    dark: 'none',
    light: 'linear-gradient(180deg, #F6F4F1 0%, #F2E8DC 44%, #F79A6E 100%)',
  },
  // Faint engineered grid texture overlaid on the canvas.
  gridLine: 'rgba(24,22,20,0.035)',
} as const;

// ── Charts & data-viz (reference: Performance Review render) ────────────────
// Viz-first law: every chart pulls its colors from here — never an inline hex.
// The categorical series leads with the neon arc (violet → blue → pink) and
// never uses lime as a primary series (mx.md › What To Avoid).
export const chart = {
  // Ordered categorical series for bars, lines, donut slices.
  series: ['#F47A45', '#181614', '#5F5B56', '#9C958D', '#2563EB', '#059669', '#D97706', '#DC2626'] as const,
  // Radial readiness gauge (the 72% dial) — neon arc sweep.
  gauge: {
    arc: `conic-gradient(from 180deg, ${neon.pink}, ${neon.purple} 42%, ${neon.blue} 84%, ${neon.pink})`,
    trackDark: '#374151',
    trackLight: '#E9E5DF',
  },
  // Horizontal meter track (unfilled portion of a progress bar).
  trackDark: '#374151',
  trackLight: '#E9E5DF',
  // Axis / gridline hairlines.
  gridDark: '#1F2937',
  gridLight: '#E9E5DF',
} as const;

// ── Radius scale (mx.md uses 14/18/24/28; pill for chips) ───────────────────
export const radius = { sm: 8, cta: 8, lg: 10, xl: 12, pill: 999 } as const;

// ── Motion (mx.md › Motion System) ──────────────────────────────────────────
// Everything moves slowly. 200/300/500ms. Never bounce/elastic/gaming.
export const motion = {
  easing: 'cubic-bezier(0.22, 0.61, 0.36, 1)',
  fast: '200ms cubic-bezier(0.22, 0.61, 0.36, 1)',
  base: '300ms cubic-bezier(0.22, 0.61, 0.36, 1)',
  slow: '500ms cubic-bezier(0.22, 0.61, 0.36, 1)',
} as const;

// ── Typography (mx.md › Typography: Inter, weights 500–800) ──────────────────
export const font = {
  family: '"Inter Variable", "Inter", system-ui, -apple-system, sans-serif',
  mono: '"Geist Mono", ui-monospace, "SF Mono", monospace',
  weight: { medium: 500, semibold: 600, bold: 700, heavy: 800 },
} as const;

// Aggregate token bag for non-sx contexts (SVG, keyframes, canvas) and for the
// theme builder. Components prefer `theme.palette.*`; this is the escape hatch.
export const studioTokens = {
  neon,
  gradient,
  modes,
  effect,
  chart,
  radius,
  motion,
  font,
} as const;

export type StudioTokens = typeof studioTokens;
