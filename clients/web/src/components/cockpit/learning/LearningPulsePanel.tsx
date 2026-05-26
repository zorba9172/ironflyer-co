"use client";

// LearningPulsePanel — the live heartbeat of the learning system.
//
// Two surfaces in one tile:
//   1. Big counters: outcome events today + all-time (the proof that
//      the system is recording experience).
//   2. A 24h sparkline of events-per-hour, using the locked violet
//      accent — this is the "is it learning right now?" pulse.
//
// Sentinel: outcomeEventsAllTime === 0 means the system has not yet
// recorded a single outcome event. The panel surfaces the stub copy
// in that case rather than rendering a flat line.

import dynamic from "next/dynamic";
import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "../health/PanelFrame";
import type { HourlyOutcomeBucket, LearningDashboardShape } from "./types";

const EChart = dynamic(
  () => import("../../charts/EChart").then((m) => m.EChart),
  { ssr: false, loading: () => <Box sx={{ height: 96 }} /> },
);

export interface LearningPulsePanelProps {
  data: LearningDashboardShape;
}

export function LearningPulsePanel({ data }: LearningPulsePanelProps) {
  const wired = data.outcomeEventsAllTime > 0;
  const buckets = data.hourlyOutcomes ?? [];

  return (
    <PanelFrame
      eyebrow="Pulse"
      title="Outcome events"
      hint={
        wired
          ? "Every patch, gate verdict, repair, and execution writes a measured outcome. The system trains on what actually happened."
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          No outcome events yet. As soon as an execution closes, the
          learning loop records its margin, gate verdicts, and repair
          recipes — this panel turns into a live pulse.
        </PanelStubEmpty>
      ) : (
        <Stack spacing={1.5}>
          <Stack direction="row" spacing={3} alignItems="baseline">
            <CounterStat
              label="Today"
              value={data.outcomeEventsToday}
              accent
            />
            <CounterStat
              label="All-time"
              value={data.outcomeEventsAllTime}
            />
          </Stack>
          <Box>
            <EChart
              height={96}
              ariaLabel="Outcome events per hour over the last 24 hours"
              option={buildSparkOption(buckets)}
            />
            <Typography
              sx={{
                mt: 0.25,
                fontSize: 11,
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                letterSpacing: 0.4,
              }}
            >
              EVENTS / HOUR · LAST 24H
            </Typography>
          </Box>
        </Stack>
      )}
    </PanelFrame>
  );
}

function CounterStat({
  label,
  value,
  accent = false,
}: {
  label: string;
  value: number;
  accent?: boolean;
}) {
  return (
    <Stack spacing={0.25}>
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          fontFamily: tokens.font.mono,
          fontSize: 32,
          fontWeight: 700,
          lineHeight: 1.05,
          color: accent
            ? tokens.color.accent.violet
            : tokens.color.text.primary,
        }}
      >
        {value.toLocaleString()}
      </Typography>
    </Stack>
  );
}

function buildSparkOption(buckets: HourlyOutcomeBucket[]) {
  // Fill 24 zero buckets when the resolver hasn't yet exposed the
  // per-hour series — the line is still drawn (as a flat baseline)
  // so the "wired but quiet" state remains visually honest.
  const series =
    buckets.length === 24
      ? buckets.map((b) => b.count)
      : Array.from({ length: 24 }, (_, i) => buckets[i]?.count ?? 0);

  return {
    grid: { left: 0, right: 0, top: 4, bottom: 4 },
    tooltip: {
      trigger: "axis",
      backgroundColor: tokens.color.bg.surface,
      borderColor: tokens.color.border.strong,
      borderWidth: 1,
      textStyle: { color: tokens.color.text.primary, fontSize: 12 },
      formatter: (params: unknown) => {
        const arr = params as Array<{ dataIndex: number; value: number }>;
        const p = arr[0];
        if (!p) return "";
        const hoursAgo = 23 - p.dataIndex;
        return `${p.value} event${p.value === 1 ? "" : "s"} · ${hoursAgo}h ago`;
      },
    },
    xAxis: { type: "category", show: false, data: series.map((_, i) => i) },
    yAxis: { type: "value", show: false },
    series: [
      {
        type: "line",
        data: series,
        smooth: 0.35,
        showSymbol: false,
        lineStyle: { color: tokens.color.accent.violet, width: 2 },
        areaStyle: {
          color: tokens.color.accent.violet,
          opacity: 0.18,
        },
      },
    ],
  };
}
