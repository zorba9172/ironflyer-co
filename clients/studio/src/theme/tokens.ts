// ─────────────────────────────────────────────────────────────────────────
// IRONFLYER STUDIO — PRODUCT WORKSPACE DESIGN TOKENS  ·  "AURORA" SYSTEM
// ─────────────────────────────────────────────────────────────────────────
// Single source of truth for the studio surface. Derived verbatim from the
// locked designer references:
//   clients/studio/design_refernce/image.png          (light Home — newest)
//   clients/studio/design_refernce/image copy.png      (light Review — newest)
//   clients/studio/design_refernce/ChatGPT…01_32_05…   (workbench / IDE shell)
//   clients/studio/design_refernce/ChatGPT…01_48_34…   (Performance Review)
//   clients/studio/design_refernce/ChatGPT…01_57_49…   (Preview / live build)
//
// DESIGN DNA (see clients/studio/DESIGN_CONSTITUTION.md):
//   Clean, light, expensive, intelligent. Linear × Lovable × Figma × Vercel.
//   The product is LIGHT-FIRST with an indigo→violet signature; dark mode is a
//   first-class peer (the toggle). Functional zones (sidebar, workbench, code,
//   and the dark prompt variant) may go dark inside light mode for focus.
//
// CONSTITUTIONAL LAW:
//   • This file is the ONLY place raw hex / rgba literals may live for the
//     studio. Components NEVER inline a color, gradient, blur, radius, or
//     motion value — they read it from the MUI theme (theme.palette.* /
//     theme.studio.*) or by importing `studioTokens` (legal only for non-sx
//     contexts: keyframes, SVG fills, canvas).
//   • Mode-aware values (canvas / surface / text / border) live in `modes` and
//     are mapped into the MUI palette so they flip on the dark/light toggle.
//   • Mode-INDEPENDENT brand marks (the aurora gradient, accent hues, glow
//     recipes, motion) live here and read identically in both schemes — the
//     indigo→violet signature never changes.
// ─────────────────────────────────────────────────────────────────────────

export type StudioMode = 'dark' | 'light';

// ── Brand marks (mode-independent) ──────────────────────────────────────────
// The recognizable signature is the indigo → violet → pink arc. Semantic
// success/warning/danger/info are tuned to read on both canvases. Key names are
// semantic (a `violet` is violet) — never let a name lie about its value.
export const neon = {
  indigo: '#6366F1', // primary brand hue (CTA, active nav, focus)
  blue: '#3B82F6',
  violet: '#8B5CF6',
  purple: '#7C5CF6', // gradient mid-stop (indigo↔violet blend)
  pink: '#EC4899',
  cyan: '#22CCEE', // donut / data accent
  success: '#16B364',
  warning: '#F79009',
  danger: '#F04438',
  info: '#2E90FA',
} as const;

// Aurora gradient. Never on large flat surfaces — only CTAs, the final headline
// phrase, active states, focus rims, and thin energy edges.
export const gradient = {
  signature: 'linear-gradient(100deg, #4F6BF5 0%, #7C5CF6 50%, #D96BD8 100%)',
  cta: 'linear-gradient(100deg, #5B6CF6 0%, #7C5CF6 100%)',
  soft: 'linear-gradient(180deg, rgba(99,102,241,0.10), rgba(124,92,246,0.02))',
} as const;

// ── Mode canvases (mapped into the MUI palette → flip on toggle) ────────────
// Light is the daytime product canvas (the newest references). Dark is the
// cinematic peer. Greys follow a calibrated cool-neutral ramp.
type ModeColors = {
  bg: string; // page canvas
  bgRaised: string; // secondary canvas / inset
  surface: string; // floating panel
  surfaceHover: string; // panel hover
  border: string; // strong hairline
  borderSubtle: string; // faint hairline
  cardBg: string; // floating card fill
  cardBorder: string; // floating card edge
  textPrimary: string;
  textSecondary: string;
  textMuted: string;
};

export const modes: Record<StudioMode, ModeColors> = {
  light: {
    bg: '#F7F8FA',
    bgRaised: '#FFFFFF',
    surface: '#FFFFFF',
    surfaceHover: '#F2F4F7',
    border: '#EAECF0',
    borderSubtle: '#F2F4F7',
    cardBg: '#FFFFFF',
    cardBorder: '#EAECF0',
    textPrimary: '#101828',
    textSecondary: '#475467',
    textMuted: '#98A2B3',
  },
  dark: {
    bg: '#0A0F1C',
    bgRaised: '#111827',
    surface: '#111827',
    surfaceHover: '#1C2536',
    border: '#283142',
    borderSubtle: '#1A2233',
    cardBg: '#111827',
    cardBorder: '#283142',
    textPrimary: '#F9FAFB',
    textSecondary: '#CBD5E1',
    textMuted: '#8B96A8',
  },
};

// ── Effect recipes (mode-independent specs) ─────────────────────────────────
export const effect = {
  // The prompt builder — the hero. In light mode it is a clean white card with
  // a soft violet focus glow; the dark variant uses dark glass. Components read
  // these as the resting spec and overlay the active scheme.
  promptBuilder: {
    radius: 16,
    borderColor: '#EAECF0',
    glow: '0 12px 32px rgba(99,102,241,0.14)',
    bg: '#FFFFFF',
    blur: 0,
  },
  // Cards — soft, bordered, never bubbly. "Calm, not boxed."
  card: {
    radius: 12,
    bg: '#FFFFFF',
    border: '#EAECF0',
    blur: 0,
  },
  // CTA button geometry.
  cta: { height: 44, radius: 10 },
  // Ambient atmosphere behind heroes — radial aurora glows at 5–12% opacity,
  // no visible circles, only light. Light mode keeps a barely-there wash.
  ambient: {
    dark: 'radial-gradient(60% 70% at 18% 12%, rgba(99,102,241,0.16), transparent 60%), radial-gradient(50% 60% at 82% 8%, rgba(217,107,216,0.10), transparent 60%)',
    light:
      'radial-gradient(60% 60% at 16% 0%, rgba(99,102,241,0.07), transparent 60%), radial-gradient(50% 50% at 88% 4%, rgba(217,107,216,0.05), transparent 60%)',
  },
  // Faint engineered grid texture overlaid on the canvas.
  gridLine: 'rgba(16,24,40,0.04)',
} as const;

// ── Charts & data-viz (reference: the rainbow "Provider spend" bar + donut) ──
// Viz-first law: every chart pulls its colors from here — never an inline hex.
// The categorical series is a friendly, soft rainbow led by indigo; lime is
// never a primary series. Use the FULL range — pie, bar, donut, line, gauge,
// radar — not one chart type everywhere.
export const chart = {
  // Ordered categorical series for bars, lines, donut & pie slices.
  series: ['#6366F1', '#22CCEE', '#16B364', '#F79009', '#F04438', '#8B5CF6', '#EC4899', '#3B82F6'] as const,
  // Radial readiness gauge (the 72% dial) — aurora arc sweep.
  gauge: {
    arc: `conic-gradient(from 180deg, ${neon.pink}, ${neon.violet} 38%, ${neon.indigo} 70%, ${neon.cyan})`,
    trackDark: '#283142',
    trackLight: '#EAECF0',
  },
  // Horizontal meter track (unfilled portion of a progress bar).
  trackDark: '#283142',
  trackLight: '#EAECF0',
  // Axis / gridline hairlines.
  gridDark: '#1A2233',
  gridLight: '#F2F4F7',
} as const;

// ── Radius scale ────────────────────────────────────────────────────────────
// Compact controls 8–10, cards 12–14, panels 16–20, pills fully round.
export const radius = { sm: 8, md: 10, cta: 10, lg: 14, xl: 18, pill: 999 } as const;

// ── Motion & Timing ─────────────────────────────────────────────────────────
// A deliberate motion system, synchronized with the type/space scale and living
// in this same single source of truth. Everything moves calmly — never bounce,
// elastic, or gaming. Four intents, one duration ramp:
//   • standard   — symmetric in/out (most state changes)
//   • decelerate — enter: fast-in, soft-settle (elements arriving)
//   • accelerate — exit: ease-out of view (elements leaving)
//   • emphasized — expressive enter for hero/CTA moments
// Duration ramp mirrors the type scale's discipline: instant→fast→base→slow.
const E_STANDARD = 'cubic-bezier(0.22, 0.61, 0.36, 1)';
const E_DECEL = 'cubic-bezier(0, 0, 0.2, 1)';
const E_ACCEL = 'cubic-bezier(0.4, 0, 1, 1)';
const E_EMPH = 'cubic-bezier(0.2, 0, 0, 1)';

export const motion = {
  // Back-compat string shorthands (consumed widely as `transition: motion.fast`).
  easing: E_STANDARD,
  fast: `180ms ${E_STANDARD}`,
  base: `280ms ${E_STANDARD}`,
  slow: `460ms ${E_STANDARD}`,
  // Intent-named transitions — prefer these for new work.
  hover: `180ms ${E_STANDARD}`, // micro-interactions, hover lift, focus rim
  enter: `280ms ${E_DECEL}`, // mount / reveal
  exit: `180ms ${E_ACCEL}`, // unmount / dismiss
  emphasis: `420ms ${E_EMPH}`, // hero / CTA expressive moment
  theme: `420ms ${E_EMPH}`, // dark/light canvas swap
  // Raw axes for custom keyframes / staggering.
  ease: { standard: E_STANDARD, decelerate: E_DECEL, accelerate: E_ACCEL, emphasized: E_EMPH },
  duration: { instant: 80, fast: 180, base: 280, slow: 460, hero: 620 },
  stagger: 56, // ms between successive items in a list/grid reveal
} as const;

// ── Typography ──────────────────────────────────────────────────────────────
// A two-typeface editorial system — the differentiator from generic bold-Inter
// SaaS (Base44 et al.):
//   • DISPLAY → Bricolage Grotesque: characterful, editorial, young. Used for
//     h1–h3 with tight negative tracking. This carries the brand voice.
//   • TEXT    → Inter: calm, neutral UI/body at comfortable line-height.
//   • MONO    → Geist Mono: code, metrics, tiny operational labels.
// Inspired by output.com's refined editorial hierarchy — on WHITE, never beige.
export const font = {
  display: '"Bricolage Grotesque Variable", "Inter Variable", system-ui, sans-serif',
  family: '"Inter Variable", "Inter", system-ui, -apple-system, sans-serif',
  mono: '"Geist Mono", ui-monospace, "SF Mono", monospace',
  weight: { regular: 400, medium: 500, semibold: 600, bold: 700, heavy: 800 },
  // Precise tracking — tight on display, neutral on text, open on labels.
  tracking: { display: '-0.022em', tight: '-0.014em', normal: '0', label: '0.08em' },
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
