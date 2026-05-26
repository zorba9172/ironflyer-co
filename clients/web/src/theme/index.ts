// MUI v6 theme built from packages/design-tokens.
//
// We ship two themes — dark (default cockpit chrome) and light (kept
// available for auth + marketing-style surfaces). Both pull palette,
// typography, and radii from the single source of truth so design
// changes are made in one place.
//
// Private 2026-05-25 reference: severe, engineered, legible.
// Deep space base (#050612), violet-tinted surfaces, violet product
// glow, and orange-to-magenta CTA gradients. Flat geometry, tight
// grids, restrained gradients. Cards stay at 8px radius.

import { createTheme, type Theme, type Components } from "@mui/material/styles";
import { tokens } from "../../../../packages/design-tokens";

declare module "@mui/material/styles" {
  interface Palette {
    accent: {
      lime: string;
      yellow: string;
      sky: string;
      coral: string;
      success: string;
      warning: string;
      danger: string;
      purple: string;
      violet: string;
    };
    surface: {
      base: string;
      raised: string;
      hover: string;
      inset: string;
    };
  }
  interface PaletteOptions {
    accent?: Partial<Palette["accent"]>;
    surface?: Partial<Palette["surface"]>;
  }
  interface ButtonVariants {
    cta: React.CSSProperties;
  }
  interface ButtonVariantsOptions {
    cta?: React.CSSProperties;
  }
}

declare module "@mui/material/Button" {
  interface ButtonPropsVariantOverrides {
    cta: true;
  }
}

const accentPalette = {
  lime: tokens.color.accent.lime,
  yellow: tokens.color.accent.yellow,
  sky: tokens.color.accent.sky,
  coral: tokens.color.accent.coral,
  success: tokens.color.accent.success,
  warning: tokens.color.accent.warning,
  danger: tokens.color.accent.danger,
  purple: tokens.color.accent.purple,
  violet: tokens.color.accent.violet,
};

// Brand surfaces only. Primary CTAs use the locked coral-magenta-violet
// sequence from the private 2026-05-25 reference.
export const gradients = {
  brandGlow: `linear-gradient(180deg, ${tokens.color.bg.surface} 0%, ${tokens.color.bg.surfaceRaised} 100%)`,
  violetWash: `radial-gradient(ellipse at top, rgba(143, 77, 255, 0.18), transparent 60%)`,
  mintWash: `radial-gradient(ellipse at top, rgba(83, 255, 189, 0.10), transparent 60%)`,
} as const;

const sharedTypography = {
  fontFamily: tokens.font.family,
  fontWeightRegular: tokens.font.weight.regular,
  fontWeightMedium: tokens.font.weight.medium,
  fontWeightBold: tokens.font.weight.bold,
  h1: { fontWeight: 750, letterSpacing: -0.02, lineHeight: 0.96 },
  h2: { fontWeight: 750, letterSpacing: -0.015, lineHeight: 1.04 },
  h3: { fontWeight: 720, letterSpacing: -0.01, lineHeight: 1.1 },
  h4: { fontWeight: 700, letterSpacing: 0, lineHeight: 1.18 },
  h5: { fontWeight: 700, letterSpacing: 0, lineHeight: 1.2 },
  h6: { fontWeight: 700, letterSpacing: 0, lineHeight: 1.25 },
  body1: { fontSize: 14.5, lineHeight: 1.5 },
  body2: { fontSize: 13.5, lineHeight: 1.5 },
  caption: { fontSize: 12, letterSpacing: 0.2 },
  overline: {
    fontFamily: tokens.font.mono,
    fontSize: 11,
    fontWeight: 600,
    letterSpacing: 1.2,
    textTransform: "uppercase" as const,
  },
  button: { textTransform: "none" as const, fontWeight: 700, letterSpacing: 0 },
};

function sharedComponents(mode: "light" | "dark"): Components<Theme> {
  const isDark = mode === "dark";
  return {
    MuiCssBaseline: {
      styleOverrides: {
        ":root": { colorScheme: mode },
        body: {
          backgroundColor: isDark ? tokens.color.bg.base : tokens.color.bg.alabaster,
          color: isDark ? tokens.color.text.primary : tokens.color.text.inverse,
          fontFeatureSettings: '"ss01", "cv11", "tnum"',
        },
        // Global keyboard focus ring. Mouse interactions don't trip
        // `:focus-visible`, so this only lights up for keyboard users —
        // exactly the Base44 accessibility behaviour we want.
        "a:focus-visible, button:focus-visible, [role='button']:focus-visible, [tabindex]:focus-visible":
          {
            outline: `2px solid ${tokens.color.accent.violet}`,
            outlineOffset: 2,
            borderRadius: tokens.radius.sm,
          },
        // Honour reduced-motion: kill non-essential transitions for
        // users who opt out.
        "@media (prefers-reduced-motion: reduce)": {
          "*, *::before, *::after": {
            animationDuration: "0ms !important",
            transitionDuration: "0ms !important",
          },
        },
      },
    },
    MuiButton: {
      defaultProps: { disableElevation: true },
      styleOverrides: {
        root: {
          minHeight: 40,
          borderRadius: tokens.radius.sm,
          boxShadow: "none",
          paddingInline: 18,
          textTransform: "none" as const,
          fontWeight: 700,
          transition: `transform ${tokens.motion.fast} ${tokens.motion.snap}, box-shadow ${tokens.motion.fast} ${tokens.motion.snap}, background-color ${tokens.motion.fast} ${tokens.motion.snap}, border-color ${tokens.motion.fast} ${tokens.motion.snap}, filter ${tokens.motion.fast} ${tokens.motion.snap}`,
          "&:focus-visible": {
            outline: `2px solid ${tokens.color.accent.violet}`,
            outlineOffset: 2,
          },
        },
        // REFERENCE — DO NOT CHANGE.
        // The primary CTA gradient is the brand's contractual stop
        // sequence (coral → magenta → purple) per the design-tokens
        // handoff comment ("orange-to-magenta CTAs"). Any agent or
        // linter that flattens this to a single colour is in violation
        // of CLAUDE.md "Design reference is law".
        containedPrimary: {
          color: "#ffffff",
          background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
          "&:hover": {
            background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
            filter: "brightness(1.06)",
            transform: "translateY(-1px)",
            boxShadow: `0 6px 18px ${tokens.color.accent.purple}3d`,
          },
          "&:active": {
            transform: "translateY(0)",
          },
          "&.Mui-disabled": {
            background: tokens.color.bg.surfaceRaised,
            color: tokens.color.text.muted,
            boxShadow: "none",
          },
        },
        outlined: {
          borderColor: tokens.color.border.strong,
          color: tokens.color.text.primary,
          "&:hover": {
            borderColor: tokens.color.border.accent,
            backgroundColor: tokens.color.bg.surfaceHover,
          },
        },
      },
      variants: [
        {
          props: { variant: "cta" },
          style: {
            minHeight: 52,
            paddingInline: 28,
            fontSize: 15,
            fontWeight: 800,
            letterSpacing: 0.2,
            borderRadius: tokens.radius.md,
            color: tokens.color.text.primary,
            background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
            transition: `transform ${tokens.motion.fast} ${tokens.motion.snap}, box-shadow ${tokens.motion.base} ${tokens.motion.snap}`,
            "&:hover": {
              background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
              filter: "brightness(1.06)",
              transform: "translateY(-1px)",
            },
            "&.Mui-disabled": {
              backgroundColor: tokens.color.bg.surfaceRaised,
              color: tokens.color.text.muted,
              boxShadow: "none",
            },
          },
        },
      ],
    },
    MuiIconButton: {
      styleOverrides: {
        root: {
          borderRadius: tokens.radius.sm,
          transition: `background-color ${tokens.motion.fast} ${tokens.motion.snap}, color ${tokens.motion.fast} ${tokens.motion.snap}, box-shadow ${tokens.motion.fast} ${tokens.motion.snap}`,
          "&:focus-visible": {
            outline: `2px solid ${tokens.color.accent.violet}`,
            outlineOffset: 2,
          },
        },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: { borderRadius: tokens.radius.pill, fontWeight: 700 },
      },
    },
    MuiCard: {
      styleOverrides: {
        root: {
          borderRadius: tokens.radius.sm,
          backgroundColor: isDark ? tokens.color.bg.surface : "#ffffff",
          border: `1px solid ${isDark ? tokens.color.border.subtle : "rgba(7,16,17,0.08)"}`,
          boxShadow: "none",
          backgroundImage: "none",
          transition: `border-color ${tokens.motion.fast} ${tokens.motion.curve}, box-shadow ${tokens.motion.fast} ${tokens.motion.curve}, transform ${tokens.motion.fast} ${tokens.motion.snap}`,
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundImage: "none",
          borderRadius: tokens.radius.sm,
        },
      },
    },
    MuiAppBar: {
      defaultProps: { color: "transparent", elevation: 0 },
      styleOverrides: {
        root: {
          backgroundColor: isDark ? tokens.color.bg.surface : "#ffffff",
          borderBottom: `1px solid ${isDark ? tokens.color.border.subtle : "rgba(7,16,17,0.08)"}`,
        },
      },
    },
    MuiToolbar: {
      styleOverrides: { root: { minHeight: 60 } },
    },
    MuiDivider: {
      styleOverrides: {
        root: { borderColor: isDark ? tokens.color.border.subtle : "rgba(7,16,17,0.08)" },
      },
    },
    MuiTooltip: {
      styleOverrides: {
        tooltip: {
          backgroundColor: tokens.color.bg.surfaceRaised,
          color: tokens.color.text.primary,
          fontSize: 12,
          borderRadius: tokens.radius.sm,
          border: `1px solid ${tokens.color.border.subtle}`,
        },
      },
    },
    MuiTextField: {
      defaultProps: { variant: "outlined" as const, size: "small" as const },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          borderRadius: tokens.radius.sm,
        },
      },
    },
    MuiLink: {
      styleOverrides: {
        root: {
          color: tokens.color.accent.violet,
          textDecorationColor: "transparent",
          "&:hover": { textDecorationColor: tokens.color.accent.violet },
        },
      },
    },
  };
}

export const darkTheme: Theme = createTheme({
  palette: {
    mode: "dark",
    primary: { main: tokens.color.accent.violet, contrastText: tokens.color.text.primary },
    secondary: { main: tokens.color.accent.coral, contrastText: tokens.color.text.primary },
    background: {
      default: tokens.color.bg.base,
      paper: tokens.color.bg.surface,
    },
    text: {
      primary: tokens.color.text.primary,
      secondary: tokens.color.text.secondary,
      disabled: tokens.color.text.muted,
    },
    divider: tokens.color.border.subtle,
    error: { main: tokens.color.accent.danger },
    warning: { main: tokens.color.accent.warning },
    success: { main: tokens.color.accent.success },
    info: { main: tokens.color.accent.sky },
    accent: accentPalette,
    surface: {
      base: tokens.color.bg.base,
      raised: tokens.color.bg.surfaceRaised,
      hover: tokens.color.bg.surfaceHover,
      inset: tokens.color.bg.inset,
    },
  },
  typography: sharedTypography,
  shape: { borderRadius: tokens.radius.sm },
  components: sharedComponents("dark"),
});

export const lightTheme: Theme = createTheme({
  palette: {
    mode: "light",
    primary: { main: tokens.color.accent.violet, contrastText: tokens.color.text.primary },
    secondary: { main: tokens.color.accent.coral, contrastText: tokens.color.text.primary },
    background: {
      default: tokens.color.bg.alabaster,
      paper: "#ffffff",
    },
    text: {
      primary: tokens.color.text.inverse,
      secondary: "#5b6266",
      disabled: "#9aa0a4",
    },
    divider: "rgba(7,16,17,0.08)",
    error: { main: tokens.color.accent.danger },
    warning: { main: tokens.color.accent.warning },
    success: { main: tokens.color.accent.success },
    info: { main: tokens.color.accent.sky },
    accent: accentPalette,
    surface: {
      base: tokens.color.bg.alabaster,
      raised: "#ffffff",
      hover: tokens.color.bg.alabasterDeep,
      inset: tokens.color.bg.alabasterDeep,
    },
  },
  typography: sharedTypography,
  shape: { borderRadius: tokens.radius.sm },
  components: sharedComponents("light"),
});

export type ThemeModeName = "light" | "dark";

export { tokens };
