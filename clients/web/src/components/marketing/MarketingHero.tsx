// MarketingHero — hero block used by /pricing, /product, /solutions,
// /enterprise.
//
// Optional eyebrow chip, large title, subhead, and CTA stack. Server
// component — no client state. CTAs are server-rendered Links wrapped
// in MUI Buttons; primary uses the locked containedPrimary gradient.

import { ArrowForwardRounded } from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

export interface MarketingHeroProps {
  eyebrow?: string;
  title: string;
  subhead: string;
  primary?: { href: string; label: string };
  secondary?: { href: string; label: string };
  proofChips?: string[];
  aside?: ReactNode;
}

export function MarketingHero({
  eyebrow,
  title,
  subhead,
  primary,
  secondary,
  proofChips,
  aside,
}: MarketingHeroProps) {
  return (
    <Box
      component="section"
      sx={{
        position: "relative",
        py: { xs: 8, md: 14 },
        overflow: "hidden",
      }}
    >
      <Box
        aria-hidden
        sx={{
          position: "absolute",
          inset: 0,
          background: `radial-gradient(circle at 78% 8%, ${tokens.color.accent.violet}26, transparent 38%), radial-gradient(circle at 8% 92%, ${tokens.color.accent.coral}1a, transparent 42%)`,
          pointerEvents: "none",
        }}
      />
      <Box
        sx={{
          position: "relative",
          maxWidth: 1180,
          mx: "auto",
          width: "100%",
          minWidth: 0,
          display: "grid",
          gap: { xs: 4, md: 6 },
          gridTemplateColumns: aside
            ? { xs: "1fr", md: "minmax(0, 1.1fr) minmax(0, 0.9fr)" }
            : "1fr",
          alignItems: "center",
        }}
      >
        <Stack spacing={2.6} sx={{ minWidth: 0 }}>
          {eyebrow && (
            <Box
              sx={{
                alignSelf: "flex-start",
                display: "inline-flex",
                alignItems: "center",
                gap: 1,
                px: 1.4,
                py: 0.6,
                borderRadius: 999,
                border: `1px solid ${tokens.color.border.accent}`,
                bgcolor: `${tokens.color.accent.violet}1a`,
              }}
            >
              <Box
                sx={{
                  width: 7,
                  height: 7,
                  borderRadius: "50%",
                  bgcolor: tokens.color.accent.violet,
                }}
              />
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11.5,
                  letterSpacing: 1.2,
                  textTransform: "uppercase",
                  color: tokens.color.accent.violet,
                  fontWeight: 700,
                }}
              >
                {eyebrow}
              </Typography>
            </Box>
          )}
          <Typography
            component="h1"
            sx={{
              fontSize: { xs: 38, md: 60 },
              fontWeight: 900,
              letterSpacing: -1,
              lineHeight: 1.04,
              color: tokens.color.text.primary,
            }}
          >
            {title}
          </Typography>
          <Typography
            sx={{
              fontSize: { xs: 16, md: 19 },
              lineHeight: 1.6,
              color: tokens.color.text.secondary,
              maxWidth: 640,
            }}
          >
            {subhead}
          </Typography>
          {(primary || secondary) && (
            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={1.5}
              sx={{ pt: 1 }}
            >
              {primary && (
                <Button
                  component={Link}
                  href={primary.href}
                  variant="contained"
                  color="primary"
                  size="large"
                  endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
                >
                  {primary.label}
                </Button>
              )}
              {secondary && (
                <Button
                  component={Link}
                  href={secondary.href}
                  variant="text"
                  size="large"
                  sx={{ color: tokens.color.accent.violet }}
                >
                  {secondary.label}
                </Button>
              )}
            </Stack>
          )}
          {proofChips && proofChips.length > 0 && (
            <Stack
              direction="row"
              spacing={1}
              flexWrap="wrap"
              useFlexGap
              sx={{ pt: 1.5 }}
            >
              {proofChips.map((chip) => (
                <Box
                  key={chip}
                  sx={{
                    px: 1.2,
                    py: 0.5,
                    borderRadius: 999,
                    border: `1px solid ${tokens.color.border.subtle}`,
                    bgcolor: `${tokens.color.bg.surfaceRaised}b3`,
                    fontFamily: tokens.font.mono,
                    fontSize: 11.5,
                    color: tokens.color.text.secondary,
                    letterSpacing: 0.3,
                  }}
                >
                  {chip}
                </Box>
              ))}
            </Stack>
          )}
        </Stack>
        {aside && <Box sx={{ minWidth: 0 }}>{aside}</Box>}
      </Box>
    </Box>
  );
}
