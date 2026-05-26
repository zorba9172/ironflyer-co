// /cockpit/health — Code Health Dashboard (playbook §8.11).
//
// Visual surface for the Anti-Bloat Engine: dup rate, dead code,
// complexity, bundle weight, dependency graph, reuse rate. One glance
// tells the operator how clean the codebase is.
//
// This page is a server component (no `'use client'`) — it sets the
// page metadata, lays out the 12-column grid, and delegates every
// data-bearing panel to a client component imported via next/dynamic
// (ssr:false) so the heavy chart libs (echarts, @xyflow/react) never
// land in the cold cockpit bundle.
//
// Backend gap (documented):
//   core/orchestrator/internal/business/dashboards/health.go defines the
//   HealthDashboard shape but the GraphQL schema (see
//   core/orchestrator/internal/operations/graph/schema/dashboards.graphql)
//   does not yet expose a `healthDashboard` query field. Each panel
//   therefore renders against the "tool not wired" sentinel state per
//   the health.go contract — the panels are production-ready chrome,
//   the Apollo wiring is the follow-up.

import dynamic from "next/dynamic";
import { Box } from "@mui/material";
import { PageHeader } from "../../../src/components/cockpit";
import { PanelSkeleton } from "../../../src/components/cockpit/health/PanelFrame";
import { placeholderHealth } from "../../../src/components/cockpit/health/types";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Code Health — Ironflyer",
  description:
    "Anti-Bloat metrics across the codebase: duplication, dead code, complexity, bundle weight, dependency graph, and reuse rate.",
};

const DuplicationHeatmap = dynamic(
  () =>
    import("../../../src/components/cockpit/health/DuplicationHeatmap").then(
      (m) => m.DuplicationHeatmap,
    ),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const DeadCodePanel = dynamic(
  () =>
    import("../../../src/components/cockpit/health/DeadCodePanel").then(
      (m) => m.DeadCodePanel,
    ),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const ComplexitySparkline = dynamic(
  () =>
    import("../../../src/components/cockpit/health/ComplexitySparkline").then(
      (m) => m.ComplexitySparkline,
    ),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const BundleWeightPanel = dynamic(
  () =>
    import("../../../src/components/cockpit/health/BundleWeightPanel").then(
      (m) => m.BundleWeightPanel,
    ),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const DependencyGraphPanel = dynamic(
  () =>
    import("../../../src/components/cockpit/health/DependencyGraphPanel").then(
      (m) => m.DependencyGraphPanel,
    ),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const ReuseRatePanel = dynamic(
  () =>
    import("../../../src/components/cockpit/health/ReuseRatePanel").then(
      (m) => m.ReuseRatePanel,
    ),
  { ssr: false, loading: () => <PanelSkeleton /> },
);

export default function CockpitHealthPage() {
  // Placeholder shape until the GraphQL resolver exposes
  // `healthDashboard` — see file header for the follow-up.
  const data = placeholderHealth("workspace");

  return (
    <Box sx={{ px: { xs: 2, md: 4 }, py: { xs: 3, md: 4 }, maxWidth: 1440, mx: "auto" }}>
      <PageHeader
        eyebrow="Anti-Bloat Engine"
        title="Code Health"
        description="Anti-Bloat metrics across the codebase. Each panel mirrors a real gate report — when a report isn't wired yet, the panel surfaces the env var to set."
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
        {/* Row 1 — duplication heatmap (wide) + dead-code headline */}
        <Box sx={{ gridColumn: { md: "span 7" } }}>
          <DuplicationHeatmap data={data} />
        </Box>
        <Box sx={{ gridColumn: { md: "span 5" } }}>
          <DeadCodePanel data={data} />
        </Box>

        {/* Row 2 — complexity + bundle weight */}
        <Box sx={{ gridColumn: { md: "span 6" } }}>
          <ComplexitySparkline data={data} />
        </Box>
        <Box sx={{ gridColumn: { md: "span 6" } }}>
          <BundleWeightPanel />
        </Box>

        {/* Row 3 — dependency graph (wide) + reuse rate */}
        <Box sx={{ gridColumn: { md: "span 7" } }}>
          <DependencyGraphPanel data={data} />
        </Box>
        <Box sx={{ gridColumn: { md: "span 5" } }}>
          <ReuseRatePanel data={data} />
        </Box>
      </Box>
    </Box>
  );
}
