// App-wide global CSS injected once at the root. Kept minimal — most
// styling goes through MUI `sx` + tokens. Cockpit defaults to a dark
// surface, so the global scrollbar / selection styles assume dark.
//
// 2026-05-25 handoff: deep space base with a subtle violet wash at the
// top of the viewport. Selection is violet; scrollbars are thin violet.

import { tokens } from "../../../packages/design-tokens";

export const globalSx = {
  ":root": {
    "--ifly-violet": tokens.color.accent.violet,
    "--ifly-mint": tokens.color.brand.mint,
    "--ifly-ember": tokens.color.brand.ember,
    "--ifly-amber": tokens.color.brand.amber,
    "--ifly-paper": tokens.color.bg.alabaster,
    "--ifly-ink": tokens.color.text.inverse,
    "--ifly-bg": tokens.color.bg.base,
    "--ifly-surface": tokens.color.bg.surface,
    "--ifly-surface-raised": tokens.color.bg.surfaceRaised,
    "--font-body": "Inter",
    colorScheme: "dark",
  },
  html: {
    scrollBehavior: "smooth" as const,
    backgroundColor: tokens.color.bg.base,
    overflowX: "clip" as const,
  },
  body: {
    minHeight: "100vh",
    margin: 0,
    backgroundColor: tokens.color.bg.base,
    backgroundImage: `radial-gradient(ellipse 1100px 540px at 50% -120px, ${tokens.color.accent.purple}29, transparent 70%), radial-gradient(ellipse 700px 360px at 90% -80px, ${tokens.color.brand.mint}0f, transparent 70%)`,
    backgroundRepeat: "no-repeat",
    backgroundAttachment: "fixed",
    color: tokens.color.text.primary,
    fontFeatureSettings: '"ss01", "cv11", "tnum"',
    textRendering: "optimizeLegibility" as const,
    WebkitFontSmoothing: "antialiased" as const,
    MozOsxFontSmoothing: "grayscale" as const,
    overflowX: "clip" as const,
  },
  "#__next": { minWidth: 0 },
  "*": { boxSizing: "border-box" as const },
  "img, video, canvas, svg": { maxWidth: "100%" },
  a: { color: "inherit", textDecoration: "none" },
  "::selection": {
    background: tokens.color.accent.violet,
    color: tokens.color.text.primary,
  },
  "::-webkit-scrollbar": { width: 10, height: 10 },
  "::-webkit-scrollbar-track": { background: "transparent" },
  "::-webkit-scrollbar-thumb": {
    background: `${tokens.color.accent.purple}47`,
    borderRadius: 8,
    border: "2px solid transparent",
    backgroundClip: "padding-box" as const,
  },
  "::-webkit-scrollbar-thumb:hover": {
    background: `${tokens.color.accent.purple}8c`,
    backgroundClip: "padding-box" as const,
    border: "2px solid transparent",
  },
  // Monospace blocks inherit the inset surface so they read as
  // "evidence" rather than chrome.
  "pre, code, kbd, samp": {
    fontFamily: tokens.font.mono,
  },
  "pre": {
    backgroundColor: tokens.color.bg.inset,
    border: `1px solid ${tokens.color.border.subtle}`,
    borderRadius: tokens.radius.sm,
    padding: 12,
    overflowX: "auto" as const,
  },
  // Focus ring follows the private violet/orange reference.
  "*:focus-visible": {
    outline: `2px solid ${tokens.color.accent.violet}`,
    outlineOffset: 2,
    borderRadius: 4,
  },
} as const;
