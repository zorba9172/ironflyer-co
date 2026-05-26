"use client";

// HomeFAQ — accordion answering the 8 aggressive-buyer questions.
//
// Each answer leads with the mechanic, stays 2-3 sentences, and avoids
// hype. Color comes from tokens.* per the constitutional rule.

import { ExpandMoreRounded } from "@mui/icons-material";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Stack,
  Typography,
} from "@mui/material";
import { tokens } from "../../../../../packages/design-tokens";

interface Item {
  q: string;
  a: string;
}

const ITEMS: Item[] = [
  {
    q: "How is this different from Lovable / Base44 / Bolt?",
    a: "Those products sell prompt-to-app speed. Ironflyer sells the missing production discipline: GateBudget, GatePatchReview, GateBuild, ProfitGuard, append-only ledger, and a real Docker workspace. Same prompt-first surface, but every paid execution clears gates before it touches main.",
  },
  {
    q: "What happens if the AI burns through my wallet?",
    a: "It can't. GateBudget holds funds against your wallet before any expensive call runs, and ProfitGuard refuses reservations that would push the balance negative. If you hit the wall, the API returns 402 Payment Required with a top-up URL — never a surprise bill.",
  },
  {
    q: "Can I review what the AI changed before it ships?",
    a: "Yes. The AI never writes files directly — every change goes through patch.Engine.Propose and lands as a reviewable diff. Accept, reject, or edit each file change in the patch lifecycle before gates approve and apply.",
  },
  {
    q: "Do I get a real Linux box or a browser sandbox?",
    a: "A real per-user Docker workspace with PTY WebSocket access and a File API. The runtime ships Mock and Docker drivers in core/runtime — same surface as your laptop, isolated per user, with owner checks on every store.",
  },
  {
    q: "How do you charge — per token, per app, per month?",
    a: "Prepaid wallet. You top up via Stripe Checkout, every execution reserves a budget hold, and the ledger debits provider cost as it materialises. Unused holds release on commit and platform margin = wallet revenue minus provider cost minus sandbox cost.",
  },
  {
    q: "Can I export my code if I leave?",
    a: "The Docker workspace is yours — pull the full project tree through the File API or git push to your own remote at any time. There is no proprietary build format and no lock-in beyond what your stack itself requires.",
  },
  {
    q: "Does it deploy to production on its own?",
    a: "Only after the gate chain passes. GateBuild verifies the artifact, GateMobileBuild drives real gradlew / xcodebuild / EAS builds, and the deploy artifact lands next to its ledger entry. You approve the production step or hand the orchestrator a deploy contract that lets it ship inside ProfitGuard limits.",
  },
  {
    q: "What languages and frameworks are supported?",
    a: "Web: React, Next.js, Vue, Svelte, Node, Python, Go, Rust against any Postgres / Stripe / Supabase stack. Mobile: Expo, React Native bare, Android-native Kotlin, iOS-native Swift (Pro tier), Flutter. The mobile-coder and mobile-deployer agents own the Expo / EAS / fastlane patches.",
  },
];

export function HomeFAQ() {
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
            Frequently asked
          </Typography>
          <Typography
            sx={{
              fontSize: { xs: 28, md: 36 },
              fontWeight: 800,
              letterSpacing: -0.5,
              color: tokens.color.text.primary,
            }}
          >
            Questions a paying builder asks.
          </Typography>
        </Stack>

        <Box sx={{ maxWidth: 880, mx: "auto" }}>
          {ITEMS.map((it, idx) => (
            <Accordion
              key={it.q}
              disableGutters
              elevation={0}
              square
              sx={{
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: `${tokens.radius.md}px`,
                mb: 1.5,
                overflow: "hidden",
                "&:before": { display: "none" },
                "&.Mui-expanded": {
                  borderColor: tokens.color.border.strong,
                  bgcolor: tokens.color.bg.surfaceRaised,
                },
              }}
              defaultExpanded={idx === 0}
            >
              <AccordionSummary
                expandIcon={
                  <ExpandMoreRounded
                    sx={{ color: tokens.color.accent.violet }}
                  />
                }
                sx={{
                  px: { xs: 2, md: 3 },
                  py: 1,
                  "& .MuiAccordionSummary-content": { my: 1.5 },
                }}
              >
                <Typography
                  sx={{
                    fontSize: { xs: 14.5, md: 15.5 },
                    fontWeight: 700,
                    color: tokens.color.text.primary,
                    letterSpacing: -0.1,
                  }}
                >
                  {it.q}
                </Typography>
              </AccordionSummary>
              <AccordionDetails
                sx={{
                  px: { xs: 2, md: 3 },
                  pb: 2.5,
                  pt: 0,
                  borderTop: `1px solid ${tokens.color.border.subtle}`,
                }}
              >
                <Typography
                  sx={{
                    pt: 2,
                    fontSize: 14,
                    lineHeight: 1.65,
                    color: tokens.color.text.secondary,
                  }}
                >
                  {it.a}
                </Typography>
              </AccordionDetails>
            </Accordion>
          ))}
        </Box>
      </Box>
    </Box>
  );
}
