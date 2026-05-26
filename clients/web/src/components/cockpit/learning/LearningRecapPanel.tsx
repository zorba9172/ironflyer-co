"use client";

// LearningRecapPanel — the "this week the system improved" stat block.
//
// Four numbers, each with a directional arrow:
//   - Completion-score delta
//   - Margin delta
//   - Reuse rate (last 7d)
//   - Repair-recipe hits (last 7d)
//
// Deltas compare last 7d to prior 7d. When the resolver hasn't yet
// exposed prior-week aggregates the delta arrow renders in a neutral
// state (no green/red claim) so we never lie to the operator.

import { Box, Stack, Typography } from "@mui/material";
import {
  ArrowDownwardRounded,
  ArrowUpwardRounded,
  RemoveRounded,
} from "@mui/icons-material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "../health/PanelFrame";
import type { LearningDashboardShape } from "./types";

export interface LearningRecapPanelProps {
  data: LearningDashboardShape;
}

export function LearningRecapPanel({ data }: LearningRecapPanelProps) {
  // The panel is considered "wired" once at least one of the
  // underlying counters has data. When everything is zero we render
  // the stub state.
  const wired =
    data.averageCompletionScore > 0 ||
    data.averageMarginPctLast7d !== 0 ||
    data.reuseRateLast7d >= 0 ||
    data.repairRecipeHitsLast7d > 0;

  const delta = data.weekDelta ?? {
    completionScoreDelta: null,
    marginDelta: null,
    reuseRateDelta: null,
    repairRecipeHitsDelta: null,
  };

  return (
    <PanelFrame
      eyebrow="Recap"
      title="This week the system improved"
      hint={
        wired
          ? "Last 7 days vs the 7 days before. The deltas are the proof — every successful execution sharpens the next one."
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          Not enough executions in the last 7 days to compute deltas
          yet. As soon as the learning loop has two consecutive weeks
          of data this recap fills in.
        </PanelStubEmpty>
      ) : (
        <Box
          sx={{
            display: "grid",
            gap: 1.5,
            gridTemplateColumns: { xs: "1fr 1fr", sm: "repeat(4, 1fr)" },
          }}
        >
          <RecapStat
            label="Completion score"
            value={formatScore(data.averageCompletionScore)}
            delta={delta.completionScoreDelta}
            higherIsBetter
          />
          <RecapStat
            label="Margin"
            value={formatPct(data.averageMarginPctLast7d)}
            delta={delta.marginDelta}
            higherIsBetter
          />
          <RecapStat
            label="Reuse rate"
            value={
              data.reuseRateLast7d >= 0
                ? formatPct(data.reuseRateLast7d)
                : "—"
            }
            delta={delta.reuseRateDelta}
            higherIsBetter
          />
          <RecapStat
            label="Repair-recipe hits"
            value={data.repairRecipeHitsLast7d.toLocaleString()}
            delta={delta.repairRecipeHitsDelta}
            higherIsBetter
          />
        </Box>
      )}
    </PanelFrame>
  );
}

function formatPct(v: number): string {
  return `${(v * 100).toFixed(1)}%`;
}

function formatScore(v: number): string {
  return v.toFixed(2);
}

interface RecapStatProps {
  label: string;
  value: string;
  delta: number | null;
  higherIsBetter: boolean;
}

function RecapStat({ label, value, delta, higherIsBetter }: RecapStatProps) {
  const hasDelta = delta !== null && delta !== undefined;
  const positive = hasDelta && delta > 0;
  const negative = hasDelta && delta < 0;
  const good = hasDelta && (higherIsBetter ? positive : negative);
  const bad = hasDelta && (higherIsBetter ? negative : positive);

  const color = good
    ? tokens.color.brand.mint
    : bad
      ? tokens.color.accent.coral
      : tokens.color.text.muted;
  const Icon = positive
    ? ArrowUpwardRounded
    : negative
      ? ArrowDownwardRounded
      : RemoveRounded;

  return (
    <Stack
      spacing={0.5}
      sx={{
        p: 1.25,
        borderRadius: 1,
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        minWidth: 0,
      }}
    >
      <Typography
        variant="overline"
        sx={{
          color: tokens.color.text.muted,
          letterSpacing: 1.2,
          fontSize: 10.5,
        }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          fontFamily: tokens.font.mono,
          fontSize: 22,
          fontWeight: 700,
          color: tokens.color.text.primary,
          lineHeight: 1.05,
        }}
      >
        {value}
      </Typography>
      <Stack direction="row" alignItems="center" spacing={0.5}>
        <Icon sx={{ fontSize: 14, color }} />
        <Typography
          sx={{
            fontSize: 11.5,
            fontFamily: tokens.font.mono,
            color,
            fontWeight: 600,
          }}
        >
          {hasDelta ? formatDelta(delta) : "no prior week"}
        </Typography>
      </Stack>
    </Stack>
  );
}

function formatDelta(d: number): string {
  const sign = d > 0 ? "+" : "";
  // Surface as percentage points when the absolute value is small
  // enough to be a rate; otherwise as a plain number.
  if (Math.abs(d) < 5) {
    return `${sign}${(d * 100).toFixed(1)} pp`;
  }
  return `${sign}${d.toLocaleString()}`;
}
