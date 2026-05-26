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

import { Box } from "@mui/material";
import { PageHeader } from "../../../src/components/cockpit";
import { HealthPanelsClient } from "../../../src/components/cockpit/health/HealthPanelsClient";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Code Health — Ironflyer",
  description:
    "Anti-Bloat metrics across the codebase: duplication, dead code, complexity, bundle weight, dependency graph, and reuse rate.",
};

export default function CockpitHealthPage() {
  return (
    <Box sx={{ px: { xs: 2, md: 4 }, py: { xs: 3, md: 4 }, maxWidth: 1440, mx: "auto" }}>
      <PageHeader
        eyebrow="Anti-Bloat Engine"
        title="Code Health"
        description="Anti-Bloat metrics across the codebase. Each panel mirrors a real gate report — when a report isn't wired yet, the panel surfaces the env var to set."
      />

      <HealthPanelsClient />
    </Box>
  );
}
