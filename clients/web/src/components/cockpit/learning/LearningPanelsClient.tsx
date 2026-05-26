"use client";

import dynamic from "next/dynamic";
import { Box } from "@mui/material";
import { PanelSkeleton } from "../health/PanelFrame";
import { placeholderLearning } from "./types";

const LearningPulsePanel = dynamic(
  () => import("./LearningPulsePanel").then((m) => m.LearningPulsePanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const BanditConfidencePanel = dynamic(
  () => import("./BanditConfidencePanel").then((m) => m.BanditConfidencePanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const GateFailureRatePanel = dynamic(
  () => import("./GateFailureRatePanel").then((m) => m.GateFailureRatePanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const BlueprintSuccessPanel = dynamic(
  () => import("./BlueprintSuccessPanel").then((m) => m.BlueprintSuccessPanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const WeaknessesPanel = dynamic(
  () => import("./WeaknessesPanel").then((m) => m.WeaknessesPanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);
const LearningRecapPanel = dynamic(
  () => import("./LearningRecapPanel").then((m) => m.LearningRecapPanel),
  { ssr: false, loading: () => <PanelSkeleton /> },
);

export function LearningPanelsClient() {
  const data = placeholderLearning();

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
        <LearningPulsePanel data={data} />
      </Box>
      <Box sx={{ gridColumn: { md: "span 5" } }}>
        <BanditConfidencePanel data={data} />
      </Box>

      <Box sx={{ gridColumn: { md: "span 6" } }}>
        <GateFailureRatePanel data={data} />
      </Box>
      <Box sx={{ gridColumn: { md: "span 6" } }}>
        <BlueprintSuccessPanel data={data} />
      </Box>

      <Box sx={{ gridColumn: { md: "span 7" } }}>
        <WeaknessesPanel data={data} />
      </Box>
      <Box sx={{ gridColumn: { md: "span 5" } }}>
        <LearningRecapPanel data={data} />
      </Box>
    </Box>
  );
}
