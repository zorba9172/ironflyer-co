// ─────────────────────────────────────────────────────────────────────────
// IRONFLYER STUDIO — NEON INTELLIGENCE DESIGN TOKENS
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
  blue: '#00D4FF', // AI Blue            — links, active, info
  violet: '#6B5CFF', // brand violet      — gradient mid-stop, primary solid
  purple: '#8B5CF6', // Intelligence Purple
  pink: '#FF4FD8', // Neon Pink          — gradient end-stop, energy
  success: '#00E6A7',
  warning: '#FFB84D',
  danger: '#FF5D73',
} as const;

// mx.md › Neon Gradient + CTA Button. Never used on large flat surfaces —
// only CTAs, the final headline phrase, active states, and energy edges.
export const gradient = {
  // Primary brand signature (blue → violet → pink).
  signature: `linear-gradient(100deg, ${neon.blue} 0%, ${neon.violet} 48%, ${neon.pink} 100%)`,
  // CTA button fill (blue → intelligence purple → pink) per mx.md › CTA Button.
  cta: `linear-gradient(100deg, ${neon.blue} 0%, ${neon.purple} 50%, ${neon.pink} 100%)`,
  // Subtle wash for hover/active glass fills (never opaque).
  soft: 'linear-gradient(135deg, rgba(0,212,255,0.10), rgba(255,79,216,0.12))',
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
    bg: '#050816',
    bgRaised: '#0A1024',
    surface: '#101936',
    surfaceHover: '#152149',
    border: 'rgba(255,255,255,0.12)',
    borderSubtle: 'rgba(255,255,255,0.06)',
    cardBg: 'rgba(255,255,255,0.03)',
    cardBorder: 'rgba(255,255,255,0.06)',
    textPrimary: '#F4F7FF',
    textSecondary: '#B8C2E6',
    textMuted: '#7C88B0',
  },
  light: {
    bg: '#F7F9FF',
    bgRaised: '#FFFFFF',
    surface: '#FFFFFF',
    surfaceHover: '#EEF2FF',
    border: 'rgba(17,25,54,0.12)',
    borderSubtle: 'rgba(17,25,54,0.07)',
    cardBg: 'rgba(255,255,255,0.78)',
    cardBorder: 'rgba(17,25,54,0.08)',
    textPrimary: '#080D27',
    textSecondary: '#58627D',
    textMuted: '#8A93AE',
  },
};

// ── Effect recipes (mode-independent specs, verbatim from mx.md) ────────────
export const effect = {
  // mx.md › Prompt Builder — the centerpiece. Always dark, in both modes.
  promptBuilder: {
    radius: 28,
    borderColor: 'rgba(255,255,255,0.12)',
    glow: '0 0 40px rgba(139,92,246,0.35)',
    bg: 'rgba(8,12,30,0.85)',
    blur: 24,
  },
  // mx.md › Cards — "cards should not feel like cards."
  card: {
    radius: 24,
    bg: 'rgba(255,255,255,0.03)',
    border: 'rgba(255,255,255,0.06)',
    blur: 20,
  },
  // mx.md › CTA Button.
  cta: { height: 56, radius: 18 },
  // mx.md › Ambient Effects — massive radial glows behind the hero, 5–15%
  // opacity, no visible circles, only atmosphere. Top-left blue, center
  // violet, bottom-right pink.
  ambient: {
    dark: 'radial-gradient(900px 620px at 12% 4%, rgba(0,212,255,0.16), transparent 66%), radial-gradient(1000px 760px at 50% 42%, rgba(107,92,255,0.14), transparent 70%), radial-gradient(900px 640px at 88% 96%, rgba(255,79,216,0.16), transparent 68%)',
    light:
      'radial-gradient(900px 600px at 12% 0%, rgba(107,92,255,0.12), transparent 64%), radial-gradient(960px 720px at 52% 40%, rgba(0,212,255,0.10), transparent 70%), radial-gradient(900px 620px at 88% 98%, rgba(255,79,216,0.10), transparent 66%)',
  },
  // Faint engineered grid texture overlaid on the canvas.
  gridLine: 'rgba(255,255,255,0.05)',
} as const;

// ── Charts & data-viz (reference: Performance Review render) ────────────────
// Viz-first law: every chart pulls its colors from here — never an inline hex.
// The categorical series leads with the neon arc (violet → blue → pink) and
// never uses lime as a primary series (mx.md › What To Avoid).
export const chart = {
  // Ordered categorical series for bars, lines, donut slices.
  series: [neon.violet, neon.blue, neon.pink, neon.purple, neon.success, neon.warning, neon.danger] as const,
  // Radial readiness gauge (the 72% dial) — neon arc sweep.
  gauge: {
    arc: `conic-gradient(from 180deg, ${neon.pink}, ${neon.purple} 42%, ${neon.blue} 84%, ${neon.pink})`,
    trackDark: 'rgba(255,255,255,0.08)',
    trackLight: 'rgba(17,25,54,0.08)',
  },
  // Horizontal meter track (unfilled portion of a progress bar).
  trackDark: 'rgba(255,255,255,0.08)',
  trackLight: 'rgba(17,25,54,0.07)',
  // Axis / gridline hairlines.
  gridDark: 'rgba(255,255,255,0.06)',
  gridLight: 'rgba(17,25,54,0.06)',
} as const;

// ── Radius scale (mx.md uses 14/18/24/28; pill for chips) ───────────────────
export const radius = { sm: 14, cta: 18, lg: 24, xl: 28, pill: 999 } as const;

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
