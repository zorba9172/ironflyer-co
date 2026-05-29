import { extendTheme } from '@mui/material/styles';
import { brand, modes } from '@ironflyer/design-tokens/brand';

// Mode-independent brand extras carried on the theme so components map
// gradients / accents / fonts / motion through `theme.brand.*` — never inline.
export interface BrandExtras {
  gradient: typeof brand.gradient;
  accent: typeof brand.accent;
  font: typeof brand.typography;
  motion: typeof brand.motion;
  shadow: typeof brand.shadow;
}

declare module '@mui/material/styles' {
  interface Theme {
    brand: BrandExtras;
  }
  interface ThemeOptions {
    brand?: BrandExtras;
  }
}

const { accent, typography, radius, motion, gradient, a11y } = brand;

function paletteFor(mode: 'dark' | 'light') {
  const m = modes[mode];
  return {
    primary: { main: accent.primary, dark: accent.primaryHover, contrastText: '#FFFFFF' },
    secondary: { main: accent.secondary },
    success: { main: accent.success },
    error: { main: accent.danger },
    warning: { main: accent.signal },
    background: { default: m.bg, paper: m.surface },
    text: { primary: m.textPrimary, secondary: m.textSecondary, disabled: m.textMuted },
    divider: m.borderSubtle,
  };
}

// Single CSS-variables theme with both color schemes. SSR-safe (no flash) and
// the only legal source of color/typography for every MUI surface.
export const theme = extendTheme({
  cssVarPrefix: 'if',
  colorSchemes: {
    dark: { palette: paletteFor('dark') },
    light: { palette: paletteFor('light') },
  },
  brand: { gradient, accent, font: typography, motion, shadow: brand.shadow },
  shape: { borderRadius: radius.md },
  typography: {
    fontFamily: typography.body,
    h1: { fontFamily: typography.display, fontWeight: 700, letterSpacing: typography.tracking.tight, lineHeight: typography.leading.tight },
    h2: { fontFamily: typography.display, fontWeight: 700, letterSpacing: typography.tracking.tight, lineHeight: typography.leading.snug },
    h3: { fontFamily: typography.display, fontWeight: 600, lineHeight: typography.leading.snug },
    h4: { fontFamily: typography.display, fontWeight: 700 },
    h5: { fontFamily: typography.display, fontWeight: 700 },
    h6: { fontFamily: typography.display, fontWeight: 700 },
    button: { textTransform: 'none', fontWeight: 600 },
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: { transition: `background-color ${motion.themeTransition}, color ${motion.themeTransition}` },
        '*:focus-visible': {
          outline: `2px solid ${accent.focus}`,
          outlineOffset: a11y.focusRingOffset,
        },
        '@media (prefers-reduced-motion: reduce)': {
          '*, *::before, *::after': {
            animationDuration: '0.01ms !important',
            animationIterationCount: '1 !important',
            scrollBehavior: 'auto !important',
            transitionDuration: '0.01ms !important',
          },
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          borderRadius: radius.sm,
          minHeight: a11y.minTarget,
          '&.MuiButton-sizeSmall': { minHeight: a11y.minCompactTarget },
          '&.Mui-focusVisible': { boxShadow: a11y.focusRing },
        },
        containedPrimary: ({ theme: t }) => ({
          backgroundImage: t.brand.gradient.signature,
          boxShadow: 'none',
          '&:hover': { boxShadow: t.brand.shadow.glow },
        }),
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: ({ theme: t }) => ({ backgroundImage: 'none', border: `1px solid ${t.palette.divider}` }),
      },
    },
    MuiIconButton: {
      styleOverrides: {
        root: {
          minWidth: a11y.minCompactTarget,
          minHeight: a11y.minCompactTarget,
          '&.Mui-focusVisible': { boxShadow: a11y.focusRing },
        },
      },
    },
  },
});

export { theme as appTheme };
