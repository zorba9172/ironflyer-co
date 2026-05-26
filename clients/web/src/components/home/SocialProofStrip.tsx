"use client";

// SocialProofStrip — 3 testimonial cards.
//
// Quotes reference concrete Ironflyer mechanics (ProfitGuard,
// GateBuild, patch review). Roles + stacks only; no fabricated
// company names.

import { FormatQuoteRounded } from "@mui/icons-material";
import { Box, Card, Stack, Typography } from "@mui/material";
import { tokens } from "../../../../../packages/design-tokens";

interface Quote {
  body: string;
  role: string;
  stack: string;
}

const QUOTES: Quote[] = [
  {
    body:
      "ProfitGuard blocked a $40 Opus call I didn't budget for. The wallet refused the reservation, surfaced the shortfall in the ledger, and I topped up before anything ran.",
    role: "Solo founder",
    stack: "React + Postgres",
  },
  {
    body:
      "GateBuild caught the missing env var before my Vercel deploy. The verdict pointed at the exact key and the build never burned a deploy minute on a guaranteed failure.",
    role: "Indie dev",
    stack: "Next.js + Stripe",
  },
  {
    body:
      "Patch review let me reject 3 of 17 file changes without losing the rest. The remaining patches applied, the gates re-ran on the smaller diff, and the execution still committed clean.",
    role: "Tech lead",
    stack: "Expo + Supabase",
  },
];

export function SocialProofStrip() {
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
        <Stack
          spacing={1.5}
          sx={{ textAlign: "center", mb: { xs: 5, md: 6 } }}
        >
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
              letterSpacing: 1.4,
              textTransform: "uppercase",
              color: tokens.color.accent.violet,
            }}
          >
            Stop shipping broken AI code
          </Typography>
          <Typography
            sx={{
              fontSize: { xs: 28, md: 36 },
              fontWeight: 800,
              letterSpacing: -0.5,
              color: tokens.color.text.primary,
            }}
          >
            Builders who run paid executions.
          </Typography>
        </Stack>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(3, minmax(0, 1fr))",
            },
            gap: 2.5,
          }}
        >
          {QUOTES.map((q) => (
            <Card
              key={q.body}
              sx={{
                p: 3,
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: `${tokens.radius.md}px`,
                display: "flex",
                flexDirection: "column",
                gap: 2.5,
                transition: `border-color ${tokens.motion.fast} ease`,
                "&:hover": {
                  borderColor: tokens.color.border.strong,
                },
              }}
            >
              <FormatQuoteRounded
                sx={{
                  fontSize: 28,
                  color: tokens.color.accent.violet,
                  alignSelf: "flex-start",
                  opacity: 0.7,
                }}
              />
              <Typography
                sx={{
                  fontSize: 14.5,
                  lineHeight: 1.6,
                  color: tokens.color.text.primary,
                  flex: 1,
                }}
              >
                {q.body}
              </Typography>
              <Box
                sx={{
                  pt: 2,
                  borderTop: `1px solid ${tokens.color.border.subtle}`,
                }}
              >
                <Typography
                  sx={{
                    fontSize: 13,
                    fontWeight: 800,
                    color: tokens.color.text.primary,
                  }}
                >
                  {q.role}
                </Typography>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 11.5,
                    color: tokens.color.text.muted,
                    letterSpacing: 0.3,
                  }}
                >
                  {q.stack}
                </Typography>
              </Box>
            </Card>
          ))}
        </Box>
      </Box>
    </Box>
  );
}
