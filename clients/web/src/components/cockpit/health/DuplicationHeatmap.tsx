"use client";

// DuplicationHeatmap — top-10 directories ranked by jscpd duplication
// rate. Bar chart (echarts) rather than a 2D heatmap because the
// signal we care about is one-dimensional ("which directory is the
// worst"). When the dedup report grows per-file pairs, swap this for
// a true 2D heatmap (rows = files, cols = files, intensity = match %).
//
// Backend: HealthDashboard.DedupRate is currently a single project
// scalar — the per-directory split lives in the persisted jscpd
// report (IRONFLYER_DEDUP_REPORT_PATH). Until the GraphQL resolver
// projects a per-directory series, this panel renders the stub-fallback
// when the global DedupRate sentinel is -1, and a uniform placeholder
// dataset otherwise.

import dynamic from "next/dynamic";
import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "./PanelFrame";
import type { HealthDashboardShape, DuplicationDirRow } from "./types";

const EChart = dynamic(
  () => import("../../charts/EChart").then((m) => m.EChart),
  { ssr: false, loading: () => <Box sx={{ height: 220 }} /> },
);

// Placeholder dataset surfaced when the global DedupRate is present
// but per-directory split is not yet projected. Real values come from
// the jscpd report once the resolver wires it.
const PLACEHOLDER_DIRS: DuplicationDirRow[] = [
  { directory: "core/orchestrator", dupPct: 0, loc: 0 },
  { directory: "core/runtime", dupPct: 0, loc: 0 },
  { directory: "clients/web/app", dupPct: 0, loc: 0 },
  { directory: "clients/web/src/components", dupPct: 0, loc: 0 },
  { directory: "clients/vscode-extension", dupPct: 0, loc: 0 },
];

export interface DuplicationHeatmapProps {
  data: HealthDashboardShape;
  perDirectory?: DuplicationDirRow[];
}

export function DuplicationHeatmap({ data, perDirectory }: DuplicationHeatmapProps) {
  const wired = data.dedupRate !== -1;
  const rows = perDirectory ?? PLACEHOLDER_DIRS;

  return (
    <PanelFrame
      eyebrow="Duplication"
      title="Dup rate by directory"
      hint={
        wired
          ? `Project-wide duplication ${(data.dedupRate * 100).toFixed(2)}%. Higher bars = more duplicated lines in that directory.`
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          Dedup report not yet wired. Install jscpd and set{" "}
          <Box component="span" sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary }}>
            IRONFLYER_DEDUP_REPORT_PATH
          </Box>
          {" "}so the Anti-Bloat gate can publish per-directory dup ratios.
        </PanelStubEmpty>
      ) : (
        <Box sx={{ minWidth: 0 }}>
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
              {(data.dedupRate * 100).toFixed(2)}%
            </Typography>
            <Typography sx={{ fontSize: 12, color: tokens.color.text.muted }}>
              project dup rate
            </Typography>
          </Stack>
          <EChart
            height={220}
            ariaLabel="Duplication rate per top-level directory"
            option={buildHeatmapOption(rows)}
          />
        </Box>
      )}
    </PanelFrame>
  );
}

function buildHeatmapOption(rows: DuplicationDirRow[]) {
  return {
    grid: { left: 8, right: 16, top: 8, bottom: 32, containLabel: true },
    tooltip: {
      trigger: "axis",
      backgroundColor: tokens.color.bg.surface,
      borderColor: tokens.color.border.strong,
      borderWidth: 1,
      textStyle: { color: tokens.color.text.primary, fontSize: 12 },
      formatter: (params: unknown) => {
        const arr = params as Array<{ name: string; value: number }>;
        const head = arr[0];
        if (!head) return "";
        return `${head.name}<br/>dup ${head.value.toFixed(2)}%`;
      },
    },
    xAxis: {
      type: "value",
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      axisLabel: { color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 10 },
      splitLine: { lineStyle: { color: tokens.color.border.subtle, type: "dashed" } },
    },
    yAxis: {
      type: "category",
      data: rows.map((r) => r.directory),
      axisLine: { lineStyle: { color: tokens.color.border.subtle } },
      axisLabel: { color: tokens.color.text.secondary, fontFamily: tokens.font.mono, fontSize: 11 },
    },
    series: [
      {
        type: "bar",
        data: rows.map((r) => r.dupPct * 100),
        itemStyle: {
          color: tokens.color.accent.violet,
          borderRadius: [0, 4, 4, 0],
        },
        barWidth: 14,
      },
    ],
  };
}
