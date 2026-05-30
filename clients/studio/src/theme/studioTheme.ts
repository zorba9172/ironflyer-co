import { extendTheme } from '@mui/material/styles';
import { brand as sharedBrand } from '@ironflyer/design-tokens/brand';
import type { BrandExtras } from '@ironflyer/ui-web';
import { neon, gradient, modes, effect, chart, radius, motion, font, type StudioMode } from './tokens';

// ─────────────────────────────────────────────────────────────────────────
// Studio MUI theme — the Neon Intelligence reference, expressed as a single
// CSS-variables theme carrying BOTH color schemes. The dark/light toggle
// (useThemeMode) flips `data-if-color-scheme`; every mapped value re-resolves
// with no flash and no component branching.
//
// Two augmentations, both standard MUI, neither colliding with @ironflyer/ui-web:
//   • Palette  — mode-AWARE studio surfaces (flip via CSS vars).
//   • Theme.studio — mode-INDEPENDENT neon marks + effect recipes.
// Components read colors ONLY through these. No inline literals. Ever.
// ─────────────────────────────────────────────────────────────────────────

export interface StudioExtras {
  neon: typeof neon;
  gradient: typeof gradient;
  modes: typeof modes;
  effect: typeof effect;
  chart: typeof chart;
  radius: typeof radius;
  motion: typeof motion;
}

declare module '@mui/material/styles' {
  interface Palette {
    surfaceRaised: string;
    surfaceHover: string;
    borderSubtle: string;
    cardBg: string;
    cardBorder: string;
  }
  interface PaletteOptions {
    surfaceRaised?: string;
    surfaceHover?: string;
    borderSubtle?: string;
    cardBg?: string;
    cardBorder?: string;
  }
  interface Theme {
    studio: StudioExtras;
  }
  interface ThemeOptions {
    studio?: StudioExtras;
  }
}

function paletteFor(mode: StudioMode) {
  const m = modes[mode];
  return {
    primary: { main: '#F2672E', dark: '#D94F18', contrastText: '#FFFFFF' },
    secondary: { main: neon.blue },
    info: { main: neon.blue },
    success: { main: neon.success },
    warning: { main: neon.warning },
    error: { main: neon.danger },
    background: { default: m.bg, paper: m.surface },
    text: { primary: m.textPrimary, secondary: m.textSecondary, disabled: m.textMuted },
    divider: m.border,
    surfaceRaised: m.bgRaised,
    surfaceHover: m.surfaceHover,
    borderSubtle: m.borderSubtle,
    cardBg: m.cardBg,
    cardBorder: m.cardBorder,
  };
}

const brandCompat: BrandExtras = {
  gradient: sharedBrand.gradient,
  accent: sharedBrand.accent,
  font: sharedBrand.typography,
  motion: sharedBrand.motion,
  shadow: sharedBrand.shadow,
};

export const studioTheme = extendTheme({
  cssVarPrefix: 'ifs',
  defaultColorScheme: 'light',
  colorSchemeSelector: 'data',
  colorSchemes: {
    dark: { palette: paletteFor('dark') },
    light: { palette: paletteFor('light') },
  },
  brand: brandCompat,
  studio: { neon, gradient, modes, effect, chart, radius, motion },
  shape: { borderRadius: radius.sm },
  typography: {
    fontFamily: font.family,
    h1: { fontWeight: font.weight.heavy, letterSpacing: 0, lineHeight: 1.04 },
    h2: { fontWeight: font.weight.heavy, letterSpacing: 0, lineHeight: 1.1 },
    h3: { fontWeight: font.weight.bold, lineHeight: 1.2 },
    h4: { fontWeight: font.weight.bold },
    h5: { fontWeight: font.weight.bold },
    h6: { fontWeight: font.weight.bold },
    button: { textTransform: 'none', fontWeight: font.weight.semibold },
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: {
          transition: `background-color ${motion.base}, color ${motion.base}`,
        },
        '@media (prefers-reduced-motion: reduce)': {
          '*, *::before, *::after': {
            animationDuration: '0.01ms !important',
            transitionDuration: '0.01ms !important',
            scrollBehavior: 'auto !important',
          },
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: { borderRadius: radius.cta, fontWeight: font.weight.semibold },
        containedPrimary: {
          backgroundImage: gradient.cta,
          boxShadow: '0 1px 2px rgba(24,22,20,0.12)',
          transition: `background-color ${motion.base}, box-shadow ${motion.base}, transform ${motion.base}`,
          '&:hover': {
            transform: 'translateY(-1px)',
            boxShadow: '0 8px 18px rgba(244,122,69,0.22)',
          },
        },
        outlined: ({ theme }) => ({
          borderColor: theme.palette.divider,
          color: theme.palette.text.primary,
          '&:hover': { borderColor: theme.palette.text.secondary, backgroundColor: theme.palette.surfaceHover },
        }),
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: ({ theme }) => ({ backgroundImage: 'none', border: `1px solid ${theme.palette.divider}` }),
      },
    },
    MuiTooltip: {
      styleOverrides: {
        tooltip: ({ theme }) => ({
          backgroundColor: theme.palette.surfaceRaised,
          border: `1px solid ${theme.palette.divider}`,
          color: theme.palette.text.primary,
          fontSize: '0.75rem',
        }),
      },
    },
  },
});
