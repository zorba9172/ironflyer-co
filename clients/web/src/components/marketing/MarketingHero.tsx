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
import { BrandBackdrop } from "./BrandBackdrop";
import { ProductTheater } from "./ProductTheater";

export interface MarketingHeroProps {
  eyebrow?: string;
  title: string;
  subhead: string;
  primary?: { href: string; label: string };
  secondary?: { href: string; label: string };
  proofChips?: string[];
  aside?: ReactNode;
  accentText?: string;
}

export function MarketingHero({
  eyebrow,
  title,
  subhead,
  primary,
  secondary,
  proofChips,
  aside,
  accentText,
}: MarketingHeroProps) {
  const visual = aside ?? <ProductTheater />;

  return (
    <Box
      component="section"
      sx={{
        position: "relative",
        px: { xs: 2, md: 4 },
        py: { xs: 8, md: 12 },
        overflow: "hidden",
        minHeight: { md: "calc(100vh - 70px)" },
        display: "grid",
        alignItems: "center",
      }}
    >
      <BrandBackdrop />
      <Box
        sx={{
          position: "relative",
          maxWidth: 1280,
          mx: "auto",
          width: "100%",
          minWidth: 0,
          display: "grid",
          gap: { xs: 4, md: 6 },
          gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 0.95fr) minmax(420px, 0.85fr)" },
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
              letterSpacing: 0,
              lineHeight: 1.04,
              color: tokens.color.text.primary,
            }}
          >
            {title}{" "}
            {accentText && (
              <Box
                component="span"
                sx={{
                  backgroundImage: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 50%, ${tokens.color.accent.violet})`,
                  WebkitBackgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                }}
              >
                {accentText}
              </Box>
            )}
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
        <Box sx={{ minWidth: 0 }}>{visual}</Box>
      </Box>
    </Box>
  );
}
