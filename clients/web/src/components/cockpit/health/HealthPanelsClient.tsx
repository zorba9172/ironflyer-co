"use client";

import dynamic from "next/dynamic";
import { Box } from "@mui/material";
import { PanelSkeleton } from "./PanelFrame";
import { placeholderHealth } from "./types";

const DuplicationHeatmap = dynamic(
  () => import("./DuplicationHeatmap").then((m) => m.DuplicationHeatmap),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const DeadCodePanel = dynamic(
  () => import("./DeadCodePanel").then((m) => m.DeadCodePanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const ComplexitySparkline = dynamic(
  () => import("./ComplexitySparkline").then((m) => m.ComplexitySparkline),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const BundleWeightPanel = dynamic(
  () => import("./BundleWeightPanel").then((m) => m.BundleWeightPanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const DependencyGraphPanel = dynamic(
  () => import("./DependencyGraphPanel").then((m) => m.DependencyGraphPanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const ReuseRatePanel = dynamic(
  () => import("./ReuseRatePanel").then((m) => m.ReuseRatePanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);

export function HealthPanelsClient() {
  const data = placeholderHealth("workspace");

  return (
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
      <Box sx={{ gridColumn: { md: "span 7" } }}>
        <DuplicationHeatmap data={data} />
      </Box>
      <Box sx={{ gridColumn: { md: "span 5" } }}>
        <DeadCodePanel data={data} />
      </Box>

      <Box sx={{ gridColumn: { md: "span 6" } }}>
        <ComplexitySparkline data={data} />
      </Box>
      <Box sx={{ gridColumn: { md: "span 6" } }}>
        <BundleWeightPanel />
      </Box>

      <Box sx={{ gridColumn: { md: "span 7" } }}>
        <DependencyGraphPanel data={data} />
      </Box>
      <Box sx={{ gridColumn: { md: "span 5" } }}>
        <ReuseRatePanel data={data} />
      </Box>
    </Box>
  );
}
