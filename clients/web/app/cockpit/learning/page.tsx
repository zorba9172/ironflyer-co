// /cockpit/learning — Learning System Dashboard.
//
// Visual surface for the Feedback Brain: the proof that Ironflyer is
// getting smarter with every executed unit. Six panels:
//   1. LearningPulsePanel        — live counters + 24h sparkline
//   2. BanditConfidencePanel     — semicircle gauge (hand-rolled SVG)
//   3. GateFailureRatePanel      — top-10 gates by failure rate
//   4. BlueprintSuccessPanel     — scatter success vs avg margin
//   5. WeaknessesPanel           — top-5 weaknesses, click for evidence
//   6. LearningRecapPanel        — week-over-week delta block
//
// Server component (no `'use client'`): owns metadata + grid layout,
// delegates each panel to a client component imported via next/dynamic
// (ssr:false) so echarts never lands in the cold cockpit bundle.
//
// Backend gap (documented):
//   The GraphQL `learningDashboard { ... }` query is being built in a
//   parallel agent. Until the resolver lands the page renders against
//   `placeholderLearning()` — every panel surfaces its "report not
//   wired" stub copy so the surface is honest about its state.

import { Box } from "@mui/material";
import { PageHeader } from "../../../src/components/cockpit";
import { LearningPanelsClient } from "../../../src/components/cockpit/learning/LearningPanelsClient";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Learning System — Ironflyer",
  description:
    "How Ironflyer gets smarter with every execution: bandit confidence, gate failure rates, blueprint profitability, detected weaknesses, and week-over-week recap.",
};

export default function CockpitLearningPage() {
  return (
    <Box sx={{ px: { xs: 2, md: 4 }, py: { xs: 3, md: 4 }, maxWidth: 1440, mx: "auto" }}>
      <PageHeader
        eyebrow="Feedback Brain"
        title="Learning System"
        description="How Ironflyer gets smarter with every execution. Each panel mirrors a live signal from the learning loop — when a signal isn't wired yet, the panel surfaces what needs to land."
      />

      <LearningPanelsClient />
    </Box>
  );
}
