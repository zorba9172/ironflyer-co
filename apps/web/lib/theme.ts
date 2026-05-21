'use client';

import { createTheme } from '@mui/material/styles';
import { tokens } from '../../../packages/design-tokens';

export const ironflyerTheme = createTheme({
  palette: {
    mode: 'dark',
    background: {
      default: tokens.color.bg.base,
      paper: tokens.color.bg.surface,
    },
    text: {
      primary: tokens.color.text.primary,
      secondary: tokens.color.text.secondary,
    },
    primary: { main: tokens.color.accent.lime, contrastText: tokens.color.text.inverse },
    secondary: { main: tokens.color.accent.sky },
    success: { main: tokens.color.accent.success },
    warning: { main: tokens.color.accent.warning },
    error: { main: tokens.color.accent.danger },
    divider: tokens.color.border.subtle,
  },
  shape: { borderRadius: tokens.radius.lg },
  typography: {
    fontFamily: tokens.font.family,
    h1: { fontFamily: tokens.font.display, fontWeight: 400, letterSpacing: 0 },
    h2: { fontFamily: tokens.font.display, fontWeight: 400, letterSpacing: 0 },
    h3: { fontFamily: tokens.font.display, fontWeight: 400, letterSpacing: 0 },
    h4: { fontFamily: tokens.font.display, fontWeight: 400, letterSpacing: 0 },
    h5: { fontWeight: 700 },
    h6: { fontWeight: 700 },
    button: { fontWeight: 800, textTransform: 'none', letterSpacing: 0 },
    overline: { fontWeight: 700, letterSpacing: '0.12em' },
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        html: {
          backgroundColor: tokens.color.bg.base,
        },
        body: {
          backgroundColor: tokens.color.bg.base,
          letterSpacing: 0,
        },
        '[data-issues-open]': {
          display: 'none !important',
        },
        'nextjs-portal': {
          display: 'none !important',
        },
      },
    },
    MuiCard: {
      styleOverrides: {
        root: {
          backgroundColor: tokens.color.bg.surface,
          backgroundImage: 'none',
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: tokens.radius.sm,
          boxShadow: 'none',
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          borderRadius: tokens.radius.pill,
          paddingInline: 18,
          paddingBlock: 10,
          transition: `transform ${tokens.motion.fast} ${tokens.motion.curve}, background-color ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
          '&:hover': { transform: 'translateY(-1px)' },
        },
        containedPrimary: {
          backgroundColor: tokens.color.accent.lime,
          color: tokens.color.text.inverse,
          boxShadow: 'none',
          '&:hover': { backgroundColor: '#f0ff36', boxShadow: 'none' },
        },
        outlined: {
          borderColor: tokens.color.border.strong,
          color: tokens.color.text.primary,
          '&:hover': { borderColor: tokens.color.border.accent, backgroundColor: tokens.color.bg.surfaceHover },
        },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: { borderRadius: tokens.radius.pill, fontWeight: 700 },
      },
    },
    MuiTextField: {
      defaultProps: { variant: 'outlined' },
      styleOverrides: {
        root: {
          '& .MuiOutlinedInput-root': {
            backgroundColor: tokens.color.bg.inset,
            borderRadius: tokens.radius.md,
            transition: `border-color ${tokens.motion.base} ${tokens.motion.curve}, background-color ${tokens.motion.base} ${tokens.motion.curve}`,
            '& fieldset': { borderColor: tokens.color.border.subtle },
            '&:hover fieldset': { borderColor: tokens.color.border.strong },
            '&.Mui-focused fieldset': { borderColor: tokens.color.accent.lime },
          },
        },
      },
    },
  },
});

export { tokens };
