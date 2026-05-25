"use client";

// MobilePreviewFrame — wraps the live PreviewPane (or any preview slot)
// in a centred phone-sized viewport so the operator can sanity-check
// responsive layout without opening dev tools.
//
// The frame is a 390x844 (iPhone 14) box on md+ viewports and falls
// back to the full preview surface on xs / sm since the device is
// already a mobile width. A subtle device chrome (rounded corners +
// notch hint) sells the framing without imitating a real phone.

import { Box, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

export interface MobilePreviewFrameProps {
  children: ReactNode;
  // Optional caption rendered above the device (e.g. "iPhone 14 — 390x844").
  caption?: string;
}

const DEVICE_WIDTH = 390;
const DEVICE_HEIGHT = 844;

export function MobilePreviewFrame({
  children,
  caption = "iPhone 14 · 390 × 844",
}: MobilePreviewFrameProps) {
  return (
    <Box
      sx={{
        alignItems: "center",
        bgcolor: tokens.color.bg.base,
        display: "flex",
        flex: 1,
        flexDirection: "column",
        gap: 1.2,
        justifyContent: "center",
        minHeight: 0,
        minWidth: 0,
        overflow: "auto",
        p: { xs: 1, md: 2 },
      }}
    >
      <Stack
        direction="row"
        spacing={0.6}
        sx={{ alignItems: "center", color: tokens.color.text.muted }}
      >
        <Box
          sx={{
            bgcolor: tokens.color.accent.lime,
            borderRadius: "50%",
            height: 6,
            width: 6,
          }}
        />
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 0.6,
            textTransform: "uppercase",
          }}
        >
          {caption}
        </Typography>
      </Stack>
      <Box
        sx={{
          bgcolor: tokens.color.brand.graphite,
          border: `2px solid ${tokens.color.bg.surfaceRaised}`,
          borderRadius: 4,
          boxShadow: tokens.shadow.lg,
          flex: "0 0 auto",
          height: { xs: "70vh", md: DEVICE_HEIGHT },
          maxHeight: "calc(100% - 60px)",
          maxWidth: "100%",
          overflow: "hidden",
          p: 0.6,
          width: { xs: "100%", md: DEVICE_WIDTH },
        }}
      >
        <Box
          sx={{
            bgcolor: tokens.color.bg.alabaster,
            borderRadius: 3.2,
            height: "100%",
            overflow: "hidden",
            position: "relative",
            width: "100%",
          }}
        >
          {children}
        </Box>
      </Box>
    </Box>
  );
}
