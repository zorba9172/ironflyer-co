"use client";

// FinalCTABand — closing violet-glow CTA card.
//
// Locked coral-magenta-violet gradient comes through the theme on the
// primary <Button>. The card glow is built from tokens.* radial layers
// — no raw hex or rgba.

import {
  ArrowForwardRounded,
  CheckCircleRounded,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { tokens } from "../../../../../packages/design-tokens";

const TRUST: string[] = [
  "Prepaid wallet · no surprises",
  "Cancel anytime",
  "Export your code",
];

export function FinalCTABand() {
  return (
    <Box
      sx={{
        width: "100%",
        minWidth: 0,
        px: { xs: 2, md: 4 },
        py: { xs: 8, md: 12 },
      }}
    >
      <Box sx={{ maxWidth: 1180, mx: "auto", minWidth: 0 }}>
        <Box
          sx={{
            position: "relative",
            overflow: "hidden",
            p: { xs: 4, md: 7 },
            borderRadius: `${tokens.radius.lg}px`,
            border: `1px solid ${tokens.color.border.strong}`,
            background: `
              radial-gradient(60% 80% at 18% 12%, ${tokens.color.accent.violet}3d, transparent 70%),
              radial-gradient(55% 75% at 85% 90%, ${tokens.color.accent.purple}33, transparent 70%),
              linear-gradient(140deg, ${tokens.color.bg.surfaceRaised}f5, ${tokens.color.bg.surface}f0)
            `,
            boxShadow: tokens.shadow.lg,
            textAlign: "center",
          }}
        >
          <Stack
            spacing={3}
            alignItems="center"
            sx={{ position: "relative", zIndex: 1 }}
          >
            <Typography
              component="h2"
              sx={{
                fontSize: { xs: 32, md: 48 },
                fontWeight: 800,
                letterSpacing: -0.8,
                lineHeight: 1.05,
                color: tokens.color.text.primary,
                maxWidth: 760,
              }}
            >
              Ship the product, not the{" "}
              <Box
                component="span"
                sx={{
                  backgroundImage: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
                  WebkitBackgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                }}
              >
                receipt.
              </Box>
            </Typography>
            <Typography
              sx={{
                fontSize: { xs: 15, md: 17 },
                color: tokens.color.text.secondary,
                maxWidth: 580,
                lineHeight: 1.55,
              }}
            >
              Prepaid wallet, gates that block, patches you can read,
              ProfitGuard before every expensive call. Describe the
              product — Ironflyer takes it through Studio, review and
              deploy.
            </Typography>

            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={1.5}
              sx={{ mt: 1 }}
            >
              <Button
                component={Link}
                href="/signup"
                variant="contained"
                color="primary"
                size="large"
                endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
              >
                Start building free
              </Button>
              <Button
                component={Link}
                href="/templates"
                variant="outlined"
                size="large"
              >
                See it run
              </Button>
            </Stack>

            <Stack
              direction="row"
              useFlexGap
              flexWrap="wrap"
              spacing={1.5}
              justifyContent="center"
              sx={{ mt: 2 }}
            >
              {TRUST.map((t) => (
                <Stack
                  key={t}
                  direction="row"
                  spacing={0.75}
                  alignItems="center"
                  sx={{
                    px: 1.5,
                    py: 0.6,
                    borderRadius: `${tokens.radius.pill}px`,
                    border: `1px solid ${tokens.color.border.subtle}`,
                    bgcolor: `${tokens.color.bg.surface}b8`,
                  }}
                >
                  <CheckCircleRounded
                    sx={{
                      fontSize: 14,
                      color: tokens.color.brand.mint,
                    }}
                  />
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      fontSize: 11.5,
                      fontWeight: 700,
                      color: tokens.color.text.secondary,
                      letterSpacing: 0.2,
                    }}
                  >
                    {t}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}
