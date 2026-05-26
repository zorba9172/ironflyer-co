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

import dynamic from "next/dynamic";
import { Box } from "@mui/material";
import { PageHeader } from "../../../src/components/cockpit";
import { PanelSkeleton } from "../../../src/components/cockpit/health/PanelFrame";
import { placeholderLearning } from "../../../src/components/cockpit/learning/types";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Learning System — Ironflyer",
  description:
    "How Ironflyer gets smarter with every execution: bandit confidence, gate failure rates, blueprint profitability, detected weaknesses, and week-over-week recap.",
};

const LearningPulsePanel = dynamic(
  () =>
    import(
      "../../../src/components/cockpit/learning/LearningPulsePanel"
    ).then((m) => m.LearningPulsePanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const BanditConfidencePanel = dynamic(
  () =>
    import(
      "../../../src/components/cockpit/learning/BanditConfidencePanel"
    ).then((m) => m.BanditConfidencePanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const GateFailureRatePanel = dynamic(
  () =>
    import(
      "../../../src/components/cockpit/learning/GateFailureRatePanel"
    ).then((m) => m.GateFailureRatePanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const BlueprintSuccessPanel = dynamic(
  () =>
    import(
      "../../../src/components/cockpit/learning/BlueprintSuccessPanel"
    ).then((m) => m.BlueprintSuccessPanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const WeaknessesPanel = dynamic(
  () =>
    import(
      "../../../src/components/cockpit/learning/WeaknessesPanel"
    ).then((m) => m.WeaknessesPanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const LearningRecapPanel = dynamic(
  () =>
    import(
      "../../../src/components/cockpit/learning/LearningRecapPanel"
    ).then((m) => m.LearningRecapPanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);

export default function CockpitLearningPage() {
  // Placeholder shape until the GraphQL resolver exposes
  // `learningDashboard` — see file header for the follow-up.
  const data = placeholderLearning();

  return (
    <Box sx={{ px: { xs: 2, md: 4 }, py: { xs: 3, md: 4 }, maxWidth: 1440, mx: "auto" }}>
      <PageHeader
        eyebrow="Feedback Brain"
        title="Learning System"
        description="How Ironflyer gets smarter with every execution. Each panel mirrors a live signal from the learning loop — when a signal isn't wired yet, the panel surfaces what needs to land."
      />

      <Box
        sx={{
          display: "grid",
          gap: { xs: 2, md: 2.5 },
          gridTemplateColumns: {
            xs: "repeat(1, minmax(0, 1fr))",
            md: "repeat(12, minmax(0, 1fr))",
          },
        }}
      >
        {/* Row 1 — pulse (wide) + bandit confidence */}
        <Box sx={{ gridColumn: { md: "span 7" } }}>
          <LearningPulsePanel data={data} />
        </Box>
        <Box sx={{ gridColumn: { md: "span 5" } }}>
          <BanditConfidencePanel data={data} />
        </Box>

        {/* Row 2 — gate failure (left) + blueprint scatter (right) */}
        <Box sx={{ gridColumn: { md: "span 6" } }}>
          <GateFailureRatePanel data={data} />
        </Box>
        <Box sx={{ gridColumn: { md: "span 6" } }}>
          <BlueprintSuccessPanel data={data} />
        </Box>

        {/* Row 3 — weaknesses (wide) + recap */}
        <Box sx={{ gridColumn: { md: "span 7" } }}>
          <WeaknessesPanel data={data} />
        </Box>
        <Box sx={{ gridColumn: { md: "span 5" } }}>
          <LearningRecapPanel data={data} />
        </Box>
      </Box>
    </Box>
  );
}
