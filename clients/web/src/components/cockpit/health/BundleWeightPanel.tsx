"use client";

// BundleWeightPanel — per-route bundle size (Total / First Load /
// Per Chunk), surfaced as a stacked bar chart.
//
// Source: IRONFLYER_BUNDLE_REPORT_PATH (size-limit + @next/bundle-
// analyzer output). The HealthDashboard struct does not yet carry a
// bundle field; this is a follow-up resolver. Until then the panel
// renders the stub-fallback empty state.

import dynamic from "next/dynamic";
import { Box } from "@mui/material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "./PanelFrame";
import type { BundleRouteRow } from "./types";

const EChart = dynamic(
  () => import("../../charts/EChart").then((m) => m.EChart),
  { ssr: false, loading: () => <Box sx={{ height: 220 }} /> },
);

export interface BundleWeightPanelProps {
  routes?: BundleRouteRow[] | null;
}

export function BundleWeightPanel({ routes }: BundleWeightPanelProps) {
  const wired = !!routes && routes.length > 0;

  return (
    <PanelFrame
      eyebrow="Bundle"
      title="Per-route weight"
      hint={
        wired
          ? "First Load = HTML + critical JS shipped on initial paint. Lower is faster."
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          Bundle report not yet wired. Run size-limit + @next/bundle-analyzer
          on every deploy and set{" "}
          <Box
            component="span"
            sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary }}
          >
            IRONFLYER_BUNDLE_REPORT_PATH
          </Box>
          {" "}so the gate can publish per-route weight.
        </PanelStubEmpty>
      ) : (
        <EChart
          height={240}
          ariaLabel="Bundle weight per route, stacked by First Load and per-chunk JS"
          option={buildBundleOption(routes!)}
        />
      )}
    </PanelFrame>
  );
}

function buildBundleOption(rows: BundleRouteRow[]) {
  return {
    grid: { left: 8, right: 12, top: 28, bottom: 28, containLabel: true },
    legend: {
      top: 0,
      textStyle: { color: tokens.color.text.secondary, fontFamily: tokens.font.mono, fontSize: 10.5 },
      itemWidth: 10,
      itemHeight: 10,
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
      backgroundColor: tokens.color.bg.surface,
      borderColor: tokens.color.border.strong,
      borderWidth: 1,
      textStyle: { color: tokens.color.text.primary, fontSize: 12 },
    },
    xAxis: {
      type: "category",
      data: rows.map((r) => r.route),
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      axisLabel: {
        color: tokens.color.text.secondary,
        fontFamily: tokens.font.mono,
        fontSize: 10.5,
        rotate: 28,
      },
    },
    yAxis: {
      type: "value",
      name: "KB",
      nameTextStyle: { color: tokens.color.text.muted, fontSize: 10 },
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      splitLine: { lineStyle: { color: tokens.color.border.subtle, type: "dashed" } },
      axisLabel: { color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 10 },
    },
    series: [
      {
        name: "First Load",
        type: "bar",
        stack: "weight",
        data: rows.map((r) => r.firstLoadKB),
        itemStyle: { color: tokens.color.accent.violet, borderRadius: [0, 0, 0, 0] },
      },
      {
        name: "Per Chunk",
        type: "bar",
        stack: "weight",
        data: rows.map((r) => r.perChunkKB),
        itemStyle: { color: tokens.color.accent.sky },
      },
    ],
  };
}
