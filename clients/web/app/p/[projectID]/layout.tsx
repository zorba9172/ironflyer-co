"use client";

// /p/[projectID] layout — the Studio surface is a full-bleed
// split-view. The root <CockpitFrame /> in app/layout.tsx still
// renders (we can't remove a parent layout from a child segment in
// Next 15), so this layout neutralises the cockpit chrome:
//
//   1. Wraps children in a fixed-position container that covers the
//      entire viewport, sitting above the cockpit Nav + main padding.
//   2. Locks body scroll while the studio is mounted so the iframe
//      and chat scrollers own their own overflow.
//
// Auth is no longer enforced at this layout level, so demo and review
// workspaces can be inspected freely while authenticated sessions still
// sync real project data.

import { Box } from "@mui/material";
import { useEffect, type ReactNode } from "react";
import { tokens } from "../../../src/theme";

export default function StudioLayout({ children }: { children: ReactNode }) {
  useEffect(() => {
    if (typeof document === "undefined") return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, []);

  return (
    <Box
      sx={{
        position: "fixed",
        inset: 0,
        zIndex: 1100,
        bgcolor: tokens.color.bg.base,
        color: tokens.color.text.primary,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
      }}
    >
      {children}
    </Box>
  );
}
