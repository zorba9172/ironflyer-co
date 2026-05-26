"use client";

// GateFailureRatePanel — top 10 gates by failure rate as a horizontal
// echarts bar chart. Per-bar color is semantic:
//   - coral  → high failure (> 20%)   — the gate is shedding work
//   - amber  → mid failure  (5–20%)   — needs attention
//   - mint   → low failure  (< 5%)    — healthy
//
// Sentinel: empty `gateFailureRates` means no gate runs have been
// recorded yet.

import dynamic from "next/dynamic";
import { Box } from "@mui/material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "../health/PanelFrame";
import type { GateFailureRate, LearningDashboardShape } from "./types";

const EChart = dynamic(
  () => import("../../charts/EChart").then((m) => m.EChart),
  { ssr: false, loading: () => <Box sx={{ height: 320 }} /> },
);

export interface GateFailureRatePanelProps {
  data: LearningDashboardShape;
}

export function GateFailureRatePanel({ data }: GateFailureRatePanelProps) {
  const rows = data.gateFailureRates ?? [];
  const wired = rows.length > 0;

  // Top-10 by failure rate, descending. We render with a horizontal
  // bar chart, so reverse for visual top-down order.
  const top = [...rows]
    .sort((a, b) => b.failureRate - a.failureRate)
    .slice(0, 10)
    .reverse();

  return (
    <PanelFrame
      eyebrow="Gates"
      title="Gate failure rate"
      hint={
        wired
          ? "Coral = > 20% failure (gate is shedding work). Mint = < 5% (healthy). The system learns which gates block most often and which thresholds need tuning."
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          No gate runs recorded yet. As soon as an execution traverses
          the finisher pipeline the gate verdicts land in the learning
          store and this ranking turns live.
        </PanelStubEmpty>
      ) : (
        <EChart
          height={Math.max(260, top.length * 28 + 60)}
          ariaLabel="Top 10 gates by failure rate"
          option={buildOption(top)}
        />
      )}
    </PanelFrame>
  );
}

function colorFor(rate: number): string {
  if (rate > 0.2) return tokens.color.accent.coral;
  if (rate < 0.05) return tokens.color.brand.mint;
  return tokens.color.accent.warning;
}

function buildOption(rows: GateFailureRate[]) {
  return {
    grid: { left: 8, right: 36, top: 8, bottom: 28, containLabel: true },
    tooltip: {
      trigger: "item",
      backgroundColor: tokens.color.bg.surface,
      borderColor: tokens.color.border.strong,
      borderWidth: 1,
      textStyle: { color: tokens.color.text.primary, fontSize: 12 },
      formatter: (params: unknown) => {
        const p = params as {
          dataIndex: number;
          value: number;
          name: string;
        };
        const row = rows[p.dataIndex];
        const pct = (p.value * 100).toFixed(1);
        const sample = row ? row.sampleSize.toLocaleString() : "0";
        return `<b>${p.name}</b><br/>${pct}% failure · n=${sample}`;
      },
    },
    xAxis: {
      type: "value",
      max: 1,
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
      type: "category",
      data: rows.map((r) => r.gate),
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      axisTick: { show: false },
      axisLabel: {
        color: tokens.color.text.secondary,
        fontFamily: tokens.font.mono,
        fontSize: 11,
      },
    },
    series: [
      {
        type: "bar",
        data: rows.map((r) => ({
          value: r.failureRate,
          itemStyle: { color: colorFor(r.failureRate) },
        })),
        barWidth: 14,
        itemStyle: { borderRadius: [0, 4, 4, 0] },
        label: {
          show: true,
          position: "right",
          color: tokens.color.text.secondary,
          fontFamily: tokens.font.mono,
          fontSize: 11,
          formatter: (p: { value: number }) =>
            `${(p.value * 100).toFixed(1)}%`,
        },
      },
    ],
  };
}
