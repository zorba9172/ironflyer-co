"use client";

// BlueprintSuccessPanel — scatter of success rate (X) vs avg margin
// (Y) per blueprint. Quadrants name the strategic position:
//   - top-right    → Profitable + reliable (scale these)
//   - top-left     → Profitable but flaky   (stabilize)
//   - bottom-right → Reliable but thin       (price/optimize)
//   - bottom-left  → Drop                    (deprecate)
//
// Each point's symbol size scales with `sampleSize` when available,
// otherwise uses a constant. Sentinel: empty array means no blueprint
// executions yet.

import dynamic from "next/dynamic";
import { Box } from "@mui/material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "../health/PanelFrame";
import type { BlueprintSuccessRate, LearningDashboardShape } from "./types";

const EChart = dynamic(
  () => import("../../charts/EChart").then((m) => m.EChart),
  { ssr: false, loading: () => <Box sx={{ height: 320 }} /> },
);

export interface BlueprintSuccessPanelProps {
  data: LearningDashboardShape;
}

export function BlueprintSuccessPanel({ data }: BlueprintSuccessPanelProps) {
  const rows = data.blueprintSuccessRates ?? [];
  const wired = rows.length > 0;

  return (
    <PanelFrame
      eyebrow="Blueprints"
      title="Profitability vs reliability"
      hint={
        wired
          ? "Top-right quadrant = blueprints to scale. Bottom-left = candidates for deprecation. Bubble size scales with sample size."
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          No blueprint executions recorded yet. Once a blueprint runs
          end-to-end the learning store tracks its success rate and
          margin so this scatter can name what to scale and what to
          drop.
        </PanelStubEmpty>
      ) : (
        <EChart
          height={340}
          ariaLabel="Blueprint success rate versus average margin"
          option={buildOption(rows)}
        />
      )}
    </PanelFrame>
  );
}

function symbolSize(row: BlueprintSuccessRate): number {
  const n = row.sampleSize ?? 0;
  if (n <= 0) return 14;
  // sqrt scale, capped so a runaway sample doesn't dominate.
  return Math.min(38, 8 + Math.sqrt(n) * 2.2);
}

function buildOption(rows: BlueprintSuccessRate[]) {
  const points = rows.map((r) => ({
    name: r.blueprintName,
    value: [r.successRate, r.avgMargin],
    blueprintID: r.blueprintID,
    sampleSize: r.sampleSize ?? 0,
    symbolSize: symbolSize(r),
    itemStyle: {
      color:
        r.successRate >= 0.75 && r.avgMargin >= 0.2
          ? tokens.color.brand.mint
          : r.successRate < 0.4 || r.avgMargin < 0
            ? tokens.color.accent.coral
            : tokens.color.accent.violet,
      borderColor: tokens.color.bg.surface,
      borderWidth: 1,
    },
  }));

  return {
    grid: { left: 8, right: 24, top: 16, bottom: 36, containLabel: true },
    tooltip: {
      trigger: "item",
      backgroundColor: tokens.color.bg.surface,
      borderColor: tokens.color.border.strong,
      borderWidth: 1,
      textStyle: { color: tokens.color.text.primary, fontSize: 12 },
      formatter: (params: unknown) => {
        const p = params as {
          name: string;
          value: [number, number];
          data: { sampleSize: number };
        };
        const success = (p.value[0] * 100).toFixed(1);
        const margin = (p.value[1] * 100).toFixed(1);
        return `<b>${p.name}</b><br/>Success ${success}% · Margin ${margin}%<br/>n=${p.data.sampleSize.toLocaleString()}`;
      },
    },
    xAxis: {
      type: "value",
      min: 0,
      max: 1,
      name: "Success rate",
      nameTextStyle: {
        color: tokens.color.text.muted,
        fontFamily: tokens.font.mono,
        fontSize: 11,
        padding: [8, 0, 0, 0],
      },
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      splitLine: {
        lineStyle: { color: tokens.color.border.subtle, type: "dashed" },
      },
      axisLabel: {
        color: tokens.color.text.muted,
        fontFamily: tokens.font.mono,
        fontSize: 10,
        formatter: (v: number) => `${Math.round(v * 100)}%`,
      },
    },
    yAxis: {
      type: "value",
      name: "Avg margin",
      nameTextStyle: {
        color: tokens.color.text.muted,
        fontFamily: tokens.font.mono,
        fontSize: 11,
        padding: [0, 0, 0, 0],
      },
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      splitLine: {
        lineStyle: { color: tokens.color.border.subtle, type: "dashed" },
      },
      axisLabel: {
        color: tokens.color.text.muted,
        fontFamily: tokens.font.mono,
        fontSize: 10,
        formatter: (v: number) => `${Math.round(v * 100)}%`,
      },
    },
    series: [
      {
        type: "scatter",
        data: points,
        markLine: {
          symbol: "none",
          silent: true,
          lineStyle: {
            color: tokens.color.border.strong,
            type: "dashed",
            width: 1,
          },
          label: {
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10,
          },
          data: [
            { xAxis: 0.5 },
            { yAxis: 0.1 },
          ],
        },
        markArea: {
          silent: true,
          itemStyle: { color: "transparent" },
          label: {
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10,
            position: "insideTopRight",
          },
          data: [
            [
              { name: "Profitable + reliable", coord: [0.5, 0.1] },
              { coord: [1, 1] },
            ],
            [
              {
                name: "Profitable, flaky",
                coord: [0, 0.1],
                label: { position: "insideTopLeft" },
              },
              { coord: [0.5, 1] },
            ],
            [
              {
                name: "Reliable, thin",
                coord: [0.5, -1],
                label: { position: "insideBottomRight" },
              },
              { coord: [1, 0.1] },
            ],
            [
              {
                name: "Drop",
                coord: [0, -1],
                label: { position: "insideBottomLeft" },
              },
              { coord: [0.5, 0.1] },
            ],
          ],
        },
      },
    ],
  };
}
