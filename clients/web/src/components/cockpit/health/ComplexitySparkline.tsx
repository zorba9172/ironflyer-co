"use client";

// ComplexitySparkline — distribution of cognitive complexity per
// function. Today the HealthDashboard exposes the histogram (bins
// <=5 / 6-10 / 11-15 / 16-20 / 21+). The "over last N deploys" trend
// is a follow-up: the resolver needs to expose per-deploy histograms,
// not just the latest one. Until then we render the current
// histogram as a small multiples bar chart and surface a hint about
// the follow-up.

import dynamic from "next/dynamic";
import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "./PanelFrame";
import type { HealthDashboardShape } from "./types";

const EChart = dynamic(
  () => import("../../charts/EChart").then((m) => m.EChart),
  { ssr: false, loading: () => <Box sx={{ height: 200 }} /> },
);

const BIN_LABELS = ["<=5", "6-10", "11-15", "16-20", "21+"];

export interface ComplexitySparklineProps {
  data: HealthDashboardShape;
}

export function ComplexitySparkline({ data }: ComplexitySparklineProps) {
  const bins = data.complexityHistogram;
  const wired = bins !== null && bins !== undefined;
  const empty = wired && bins!.length === 0;
  const total = wired && bins ? bins.reduce((acc, n) => acc + n, 0) : 0;
  const hot = wired && bins ? (bins[3] ?? 0) + (bins[4] ?? 0) : 0;

  return (
    <PanelFrame
      eyebrow="Complexity"
      title="Cognitive complexity"
      hint={
        wired && !empty
          ? `${hot} function${hot === 1 ? "" : "s"} above the 16-cognitive-complexity threshold — gocognit / sonarjs flags these for refactor.`
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          Complexity histogram not yet wired. Install gocognit (Go) + sonarjs
          (TS) and set{" "}
          <Box
            component="span"
            sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary }}
          >
            IRONFLYER_COMPLEXITY_REPORT_PATH
          </Box>
          {" "}so the Anti-Bloat gate publishes per-function distributions.
        </PanelStubEmpty>
      ) : empty ? (
        <Stack
          sx={{
            py: 4,
            px: 2,
            border: `1px dashed ${tokens.color.border.subtle}`,
            borderRadius: 1,
            bgcolor: tokens.color.bg.inset,
            alignItems: "flex-start",
            gap: 0.5,
          }}
        >
          <Typography sx={{ fontSize: 13, color: tokens.color.brand.mint, fontWeight: 700 }}>
            No complexity findings.
          </Typography>
          <Typography sx={{ fontSize: 12, color: tokens.color.text.muted }}>
            Project compiled clean — every function is under the lowest threshold.
          </Typography>
        </Stack>
      ) : (
        <Box>
          <Stack direction="row" alignItems="baseline" spacing={1} sx={{ mb: 1 }}>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 28,
                fontWeight: 700,
                color: tokens.color.text.primary,
                lineHeight: 1,
              }}
            >
              {total}
            </Typography>
            <Typography sx={{ fontSize: 12, color: tokens.color.text.muted }}>
              functions analyzed
            </Typography>
          </Stack>
          <EChart
            height={200}
            ariaLabel="Cognitive complexity histogram across the codebase"
            option={buildHistogramOption(bins ?? [])}
          />
        </Box>
      )}
    </PanelFrame>
  );
}

function buildHistogramOption(bins: number[]) {
  // Per-bin color: green for the low bins, amber for mid, coral for hot.
  const colors = [
    tokens.color.brand.mint,
    tokens.color.brand.mint,
    tokens.color.brand.amber,
    tokens.color.accent.coral,
    tokens.color.accent.danger,
  ];
  return {
    grid: { left: 8, right: 12, top: 8, bottom: 28, containLabel: true },
    tooltip: {
      trigger: "axis",
      backgroundColor: tokens.color.bg.surface,
      borderColor: tokens.color.border.strong,
      borderWidth: 1,
      textStyle: { color: tokens.color.text.primary, fontSize: 12 },
    },
    xAxis: {
      type: "category",
      data: BIN_LABELS,
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      axisLabel: { color: tokens.color.text.secondary, fontFamily: tokens.font.mono, fontSize: 11 },
    },
    yAxis: {
      type: "value",
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      splitLine: { lineStyle: { color: tokens.color.border.subtle, type: "dashed" } },
      axisLabel: { color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 10 },
    },
    series: [
      {
        type: "bar",
        data: bins.map((value, i) => ({ value, itemStyle: { color: colors[i] ?? colors[0] } })),
        barWidth: 22,
        itemStyle: { borderRadius: [4, 4, 0, 0] },
      },
    ],
  };
}
