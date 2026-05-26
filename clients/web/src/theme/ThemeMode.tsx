"use client";

// ThemeMode — page-scoped opt-in to a non-default palette.
//
// The root <Providers> mounts the dark theme by default (cockpit
// chrome). Auth routes mount <ThemeMode mode="light"> when they want
// the alabaster surface back; MUI nests ThemeProvider safely.

import { CssBaseline, ThemeProvider } from "@mui/material";
import type { ReactNode } from "react";
import { darkTheme, lightTheme, type ThemeModeName } from "./index";

export function ThemeMode({
  mode,
  children,
}: {
  mode: ThemeModeName;
  children: ReactNode;
}) {
  const theme = mode === "dark" ? darkTheme : lightTheme;
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      {children}
    </ThemeProvider>
  );
}
