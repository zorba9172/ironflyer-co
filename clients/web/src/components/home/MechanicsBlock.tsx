"use client";

// MechanicsBlock — "What you actually get" 6-card grid.
//
// Names six concrete orchestrator mechanics that competitors don't
// ship as first-class primitives. Each card pairs a mechanic noun
// with a plain-language promise. All colors come from tokens.* per
// the constitutional design rule.

import {
  AccountBalanceWalletOutlined,
  PhoneIphoneRounded,
  ReceiptLongRounded,
  RuleFolderRounded,
  ShieldOutlined,
  TerminalRounded,
} from "@mui/icons-material";
import { Box, Card, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../../../../packages/design-tokens";

interface Mechanic {
  name: string;
  promise: string;
  icon: ReactNode;
}

const MECHANICS: Mechanic[] = [
  {
    name: "GateBudget",
    promise:
      "Holds funds before any expensive call runs. 402 Payment Required beats a surprise bill.",
    icon: <ShieldOutlined sx={{ fontSize: 20 }} />,
  },
  {
    name: "GatePatchReview",
    promise:
      "Every file change lands as a reviewable patch. Accept, reject or edit before it touches main.",
    icon: <RuleFolderRounded sx={{ fontSize: 20 }} />,
  },
  {
    name: "GateMobileBuild",
    promise:
      "Validates Expo / Gradle / xcodebuild manifests and runs the real build before a deploy attempt.",
    icon: <PhoneIphoneRounded sx={{ fontSize: 20 }} />,
  },
  {
    name: "ProfitGuard reservation",
    promise:
      "Refuses premium model calls and Mac workspaces that would push the wallet negative.",
    icon: <AccountBalanceWalletOutlined sx={{ fontSize: 20 }} />,
  },
  {
    name: "Append-only ledger",
    promise:
      "Revenue minus provider cost per execution. Margin is a column, not a guess.",
    icon: <ReceiptLongRounded sx={{ fontSize: 20 }} />,
  },
  {
    name: "Real Docker workspace",
    promise:
      "Per-user Linux sandbox with PTY and File API. Not a browser shim pretending to be a shell.",
    icon: <TerminalRounded sx={{ fontSize: 20 }} />,
  },
];

export function MechanicsBlock() {
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
          sx={{ textAlign: "center", mb: { xs: 5, md: 7 } }}
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
            What you actually get
          </Typography>
          <Typography
            sx={{
              fontSize: { xs: 28, md: 36 },
              fontWeight: 800,
              letterSpacing: -0.5,
              color: tokens.color.text.primary,
            }}
          >
            Named mechanics. Not promises.
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
            Six primitives the orchestrator actually runs on every paid
            execution. Each one has a concrete contract you can read in the
            ledger.
          </Typography>
        </Stack>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
            gap: 2,
          }}
        >
          {MECHANICS.map((m) => (
            <Card
              key={m.name}
              sx={{
                p: 3,
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: `${tokens.radius.md}px`,
                transition: `border-color ${tokens.motion.fast} ease, transform ${tokens.motion.fast} ease`,
                "&:hover": {
                  borderColor: tokens.color.border.strong,
                  transform: "translateY(-2px)",
                },
              }}
            >
              <Stack direction="row" alignItems="center" spacing={1.25}>
                <Box
                  sx={{
                    width: 40,
                    height: 40,
                    borderRadius: `${tokens.radius.sm}px`,
                    bgcolor: `${tokens.color.accent.violet}1f`,
                    border: `1px solid ${tokens.color.border.subtle}`,
                    color: tokens.color.accent.violet,
                    display: "grid",
                    placeItems: "center",
                  }}
                >
                  {m.icon}
                </Box>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 14.5,
                    fontWeight: 800,
                    color: tokens.color.text.primary,
                    letterSpacing: -0.2,
                  }}
                >
                  {m.name}
                </Typography>
              </Stack>
              <Typography
                sx={{
                  mt: 2,
                  fontSize: 13.5,
                  lineHeight: 1.6,
                  color: tokens.color.text.secondary,
                }}
              >
                {m.promise}
              </Typography>
            </Card>
          ))}
        </Box>
      </Box>
    </Box>
  );
}
