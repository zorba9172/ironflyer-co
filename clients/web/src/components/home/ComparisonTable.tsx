"use client";

// ComparisonTable — "vs the AI app builders" 5-row table.
//
// Functional comparison only. Captures the production-discipline gap
// between Ironflyer and the prompt-to-app builders. The caveat row is
// non-negotiable: marketing claims drift, and we say so.

import { CheckRounded, CloseRounded } from "@mui/icons-material";
import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../../../../packages/design-tokens";

interface Row {
  label: string;
  ironflyer: boolean;
  lovable: boolean;
  bolt: boolean;
  replit: boolean;
}

const ROWS: Row[] = [
  {
    label: "Gates block bad code",
    ironflyer: true,
    lovable: false,
    bolt: false,
    replit: false,
  },
  {
    label: "Patches are reviewable",
    ironflyer: true,
    lovable: false,
    bolt: false,
    replit: false,
  },
  {
    label: "Wallet enforced upfront",
    ironflyer: true,
    lovable: false,
    bolt: false,
    replit: false,
  },
  {
    label: "Real Linux workspaces",
    ironflyer: true,
    lovable: false,
    bolt: false,
    replit: false,
  },
  {
    label: "Mobile native builds",
    ironflyer: true,
    lovable: false,
    bolt: false,
    replit: false,
  },
];

const COLUMNS: Array<{ key: keyof Row; label: string; highlight: boolean }> = [
  { key: "ironflyer", label: "Ironflyer", highlight: true },
  { key: "lovable", label: "Lovable", highlight: false },
  { key: "bolt", label: "Bolt", highlight: false },
  { key: "replit", label: "Replit Agent", highlight: false },
];

function Verdict({ pass }: { pass: boolean }) {
  if (pass) {
    return (
      <Box
        sx={{
          display: "inline-grid",
          placeItems: "center",
          width: 28,
          height: 28,
          borderRadius: "50%",
          bgcolor: `${tokens.color.accent.success}1f`,
          color: tokens.color.accent.success,
          border: `1px solid ${tokens.color.accent.success}55`,
        }}
      >
        <CheckRounded sx={{ fontSize: 16 }} />
      </Box>
    );
  }
  return (
    <Box
      sx={{
        display: "inline-grid",
        placeItems: "center",
        width: 28,
        height: 28,
        borderRadius: "50%",
        bgcolor: `${tokens.color.accent.danger}1a`,
        color: tokens.color.accent.danger,
        border: `1px solid ${tokens.color.accent.danger}4d`,
      }}
    >
      <CloseRounded sx={{ fontSize: 16 }} />
    </Box>
  );
}

export function ComparisonTable() {
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
            vs the AI app builders
          </Typography>
          <Typography
            sx={{
              fontSize: { xs: 28, md: 36 },
              fontWeight: 800,
              letterSpacing: -0.5,
              color: tokens.color.text.primary,
            }}
          >
            Production discipline is the moat.
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.secondary,
              fontSize: { xs: 14, md: 15.5 },
              maxWidth: 640,
              mx: "auto",
              lineHeight: 1.6,
            }}
          >
            Prompt-to-app builders ship the demo. Ironflyer ships the
            execution surface that survives a paying customer.
          </Typography>
        </Stack>

        <Box
          sx={{
            borderRadius: `${tokens.radius.md}px`,
            border: `1px solid ${tokens.color.border.subtle}`,
            background: `linear-gradient(160deg, ${tokens.color.bg.surface}ee, ${tokens.color.bg.surfaceRaised}f2)`,
            overflow: "hidden",
            backdropFilter: "blur(8px)",
          }}
        >
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: {
                xs: "1.4fr repeat(4, minmax(0, 1fr))",
                md: "1.6fr repeat(4, minmax(0, 1fr))",
              },
              alignItems: "center",
              px: { xs: 2, md: 3 },
              py: 2,
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: `${tokens.color.bg.inset}80`,
            }}
          >
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11,
                letterSpacing: 1.2,
                textTransform: "uppercase",
                color: tokens.color.text.muted,
              }}
            >
              Capability
            </Typography>
            {COLUMNS.map((c) => (
              <Typography
                key={c.key}
                sx={{
                  textAlign: "center",
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  letterSpacing: 1.2,
                  textTransform: "uppercase",
                  fontWeight: 800,
                  color: c.highlight
                    ? tokens.color.accent.violet
                    : tokens.color.text.muted,
                }}
              >
                {c.label}
              </Typography>
            ))}
          </Box>

          {ROWS.map((row, idx) => (
            <Box
              key={row.label}
              sx={{
                display: "grid",
                gridTemplateColumns: {
                  xs: "1.4fr repeat(4, minmax(0, 1fr))",
                  md: "1.6fr repeat(4, minmax(0, 1fr))",
                },
                alignItems: "center",
                px: { xs: 2, md: 3 },
                py: { xs: 1.75, md: 2 },
                borderBottom:
                  idx < ROWS.length - 1
                    ? `1px solid ${tokens.color.border.subtle}`
                    : "none",
              }}
            >
              <Typography
                sx={{
                  fontSize: { xs: 13.5, md: 14.5 },
                  fontWeight: 700,
                  color: tokens.color.text.primary,
                }}
              >
                {row.label}
              </Typography>
              {COLUMNS.map((c) => (
                <Box
                  key={c.key}
                  sx={{
                    display: "grid",
                    placeItems: "center",
                  }}
                >
                  <Verdict pass={row[c.key] as boolean} />
                </Box>
              ))}
            </Box>
          ))}

          <Box
            sx={{
              px: { xs: 2, md: 3 },
              py: 1.5,
              borderTop: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: `${tokens.color.bg.inset}80`,
            }}
          >
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11,
                color: tokens.color.text.muted,
                letterSpacing: 0.3,
              }}
            >
              Public marketing claims as of 2026-05. Comparison is functional, not legal.
            </Typography>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
