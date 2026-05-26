"use client";

// PanelFrame — shared frame for the six Code Health panels.
//
// Provides the consistent chrome each panel renders inside:
//   - title row with optional eyebrow + drill link
//   - body slot for the chart / list / gauge
//   - footer slot for the "what's not closed" hint
//
// Colors and surfaces come exclusively from `tokens.color.*` —
// design-reference law (no raw hex / rgba). The chrome here is
// intentionally minimal so each panel's visualization stays the
// glanceable focus.

import { Box, Card, Stack, Typography } from "@mui/material";
import Link from "next/link";
import type { ReactNode } from "react";
import { tokens } from "../../../theme";

export interface PanelFrameProps {
  title: string;
  eyebrow?: string;
  hint?: ReactNode;
  drill?: { label: string; href: string };
  children: ReactNode;
}

export function PanelFrame({ title, eyebrow, hint, drill, children }: PanelFrameProps) {
  return (
    <Card
      sx={{
        p: { xs: 2, md: 2.5 },
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        height: "100%",
        display: "flex",
        flexDirection: "column",
        minWidth: 0,
      }}
    >
      <Stack
        direction="row"
        alignItems="baseline"
        justifyContent="space-between"
        spacing={1}
        sx={{ mb: 1.25 }}
      >
        <Box sx={{ minWidth: 0 }}>
          {eyebrow && (
            <Typography
              variant="overline"
              sx={{ color: tokens.color.accent.violet, letterSpacing: 1.2, display: "block" }}
            >
              {eyebrow}
            </Typography>
          )}
          <Typography
            sx={{
              fontSize: 16,
              fontWeight: 700,
              color: tokens.color.text.primary,
              letterSpacing: -0.1,
            }}
          >
            {title}
          </Typography>
        </Box>
        {drill && (
          <Typography
            component={Link}
            href={drill.href}
            sx={{
              fontSize: 12,
              fontWeight: 700,
              color: tokens.color.text.secondary,
              textDecoration: "none",
              whiteSpace: "nowrap",
              "&:hover": { color: tokens.color.text.primary },
            }}
          >
            {drill.label} →
          </Typography>
        )}
      </Stack>

      <Box sx={{ flex: 1, minHeight: 0 }}>{children}</Box>

      {hint && (
        <Box
          sx={{
            mt: 1.5,
            pt: 1.25,
            borderTop: `1px solid ${tokens.color.border.subtle}`,
          }}
        >
          <Typography sx={{ fontSize: 12, color: tokens.color.text.muted }}>
            {hint}
          </Typography>
        </Box>
      )}
    </Card>
  );
}

export function PanelSkeleton({ height = 220 }: { height?: number }) {
  return (
    <Card
      sx={{
        p: { xs: 2, md: 2.5 },
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        height: "100%",
        minHeight: height + 64,
      }}
    >
      <Box
        sx={{
          height: 16,
          width: 140,
          borderRadius: 0.5,
          bgcolor: tokens.color.bg.surfaceRaised,
          mb: 2,
          animation: "ironflyerPulse 1.4s ease-in-out infinite",
          "@keyframes ironflyerPulse": {
            "0%, 100%": { opacity: 0.55 },
            "50%": { opacity: 1 },
          },
        }}
      />
      <Box
        sx={{
          height,
          borderRadius: 1,
          bgcolor: tokens.color.bg.surfaceRaised,
          animation: "ironflyerPulse 1.4s ease-in-out infinite",
        }}
      />
    </Card>
  );
}

export function PanelStubEmpty({ children }: { children: ReactNode }) {
  return (
    <Stack
      sx={{
        py: 3,
        px: 2,
        border: `1px dashed ${tokens.color.border.subtle}`,
        borderRadius: 1,
        bgcolor: tokens.color.bg.inset,
        alignItems: "flex-start",
        gap: 0.75,
      }}
    >
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
      >
        Report not wired
      </Typography>
      <Typography sx={{ fontSize: 12.5, color: tokens.color.text.secondary, lineHeight: 1.5 }}>
        {children}
      </Typography>
    </Stack>
  );
}
