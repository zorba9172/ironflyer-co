// MarketingSection — shared section wrapper for marketing pages.
//
// Constrains content to a 1180px column, applies generous vertical
// rhythm, and optionally renders a section heading block (eyebrow +
// title + subhead). Server component — no client state.

import { Box, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

export interface MarketingSectionProps {
  eyebrow?: string;
  title?: string;
  subhead?: string;
  align?: "left" | "center";
  bgVariant?: "base" | "inset";
  id?: string;
  children: ReactNode;
}

export function MarketingSection({
  eyebrow,
  title,
  subhead,
  align = "left",
  bgVariant = "base",
  id,
  children,
}: MarketingSectionProps) {
  const headerAlign = align === "center" ? "center" : "left";
  const headerMx = align === "center" ? "auto" : undefined;

  return (
    <Box
      component="section"
      id={id}
      sx={{
        width: "100%",
        py: { xs: 7, md: 12 },
        bgcolor:
          bgVariant === "inset" ? tokens.color.bg.inset : "transparent",
      }}
    >
      <Box sx={{ maxWidth: 1180, mx: "auto", width: "100%", minWidth: 0 }}>
        {(eyebrow || title || subhead) && (
          <Stack
            spacing={1.6}
            sx={{
              mb: { xs: 5, md: 7 },
              textAlign: headerAlign,
              maxWidth: align === "center" ? 760 : 820,
              mx: headerMx,
            }}
          >
            {eyebrow && (
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11.5,
                  letterSpacing: 1.4,
                  textTransform: "uppercase",
                  color: tokens.color.accent.violet,
                  fontWeight: 700,
                }}
              >
                {eyebrow}
              </Typography>
            )}
            {title && (
              <Typography
                component="h2"
                sx={{
                  fontSize: { xs: 28, md: 40 },
                  fontWeight: 900,
                  letterSpacing: -0.6,
                  lineHeight: 1.08,
                  color: tokens.color.text.primary,
                }}
              >
                {title}
              </Typography>
            )}
            {subhead && (
              <Typography
                sx={{
                  fontSize: { xs: 15, md: 17 },
                  lineHeight: 1.65,
                  color: tokens.color.text.secondary,
                }}
              >
                {subhead}
              </Typography>
            )}
          </Stack>
        )}
        {children}
      </Box>
    </Box>
  );
}
