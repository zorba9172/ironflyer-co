"use client";

// ReuseRatePanel — two side-by-side gauges:
//   1. Reuse rate (% of recent PreflightDecisions where the coder
//      chose `reuse` or `extend` over `new`).
//   2. LOC per Resolved Capability (net LOC added / capabilities
//      validated — lower is better).
//
// Both come straight from HealthDashboard (ReuseRate, LOCPerCapability,
// AtlasCapabilityCount). Sentinel: ReuseRate = -1 means "no
// PreflightDecisions recorded yet"; LOCPerCapability = 0 with
// AtlasCapabilityCount = 0 means "first run".
//
// Visual rule: violet gauge. No lime — the locked palette uses violet
// for primary metrics and mint for live/success. We use violet for
// the arc and let the surrounding mint chip carry "good" signal.

import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "./PanelFrame";
import type { HealthDashboardShape } from "./types";

export interface ReuseRatePanelProps {
  data: HealthDashboardShape;
}

export function ReuseRatePanel({ data }: ReuseRatePanelProps) {
  const reuseWired = data.reuseRate !== -1;
  const locWired = data.atlasCapabilityCount > 0 || data.locPerCapability > 0;

  return (
    <PanelFrame
      eyebrow="Reuse"
      title="Reuse vs new code"
      hint={
        reuseWired
          ? "Anti-Bloat target: ≥ 50% reuse/extend. Below 30% suggests the Atlas isn't surfacing prior art."
          : undefined
      }
    >
      {!reuseWired && !locWired ? (
        <PanelStubEmpty>
          No PreflightDecisions yet. As soon as the coder agent runs
          against a project, the audit chain records `reuse` / `extend` /
          `new` actions and this panel turns live.
        </PanelStubEmpty>
      ) : (
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            alignItems: "center",
          }}
        >
          <ReuseGauge value={reuseWired ? data.reuseRate : 0} wired={reuseWired} />
          <Stack spacing={0.5}>
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
            >
              LOC per resolved capability
            </Typography>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 32,
                fontWeight: 700,
                color: locWired
                  ? tokens.color.text.primary
                  : tokens.color.text.muted,
                lineHeight: 1.05,
              }}
            >
              {locWired ? Math.round(data.locPerCapability) : "—"}
            </Typography>
            <Typography sx={{ fontSize: 12, color: tokens.color.text.muted }}>
              {data.atlasCapabilityCount} capabilities indexed by the Atlas.
              Lower LOC/capability = tighter coupling between intent and
              shipped code.
            </Typography>
          </Stack>
        </Box>
      )}
    </PanelFrame>
  );
}

// ReuseGauge — SVG semicircle gauge. Hand-rolled rather than echarts
// so the gauge doesn't pull in the full chart bundle for one tile.
// Stroke uses tokens.color.accent.violet per the no-lime rule.
function ReuseGauge({ value, wired }: { value: number; wired: boolean }) {
  // Normalize to 0..1; show empty arc when not wired yet.
  const v = Math.max(0, Math.min(1, value));
  const radius = 64;
  const stroke = 14;
  const circumference = Math.PI * radius;
  const dash = circumference * (wired ? v : 0);

  return (
    <Stack alignItems="center" spacing={1}>
      <Box
        component="svg"
        viewBox="-80 -80 160 96"
        role="img"
        aria-label={`Reuse rate ${(v * 100).toFixed(0)}%`}
        sx={{ width: 180, height: 110 }}
      >
        {/* Background arc */}
        <Box
          component="path"
          d={`M ${-radius} 0 A ${radius} ${radius} 0 0 1 ${radius} 0`}
          fill="none"
          stroke={tokens.color.border.subtle}
          strokeWidth={stroke}
          strokeLinecap="round"
        />
        {/* Foreground arc */}
        <Box
          component="path"
          d={`M ${-radius} 0 A ${radius} ${radius} 0 0 1 ${radius} 0`}
          fill="none"
          stroke={tokens.color.accent.violet}
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={`${dash} ${circumference}`}
          style={{
            transition: `stroke-dasharray ${tokens.motion.base} ${tokens.motion.snap}`,
          }}
        />
        {/* Value label */}
        <Box
          component="text"
          x="0"
          y="-12"
          textAnchor="middle"
          sx={{
            fill: tokens.color.text.primary,
            fontFamily: tokens.font.mono,
            fontSize: 22,
            fontWeight: 700,
          }}
        >
          {wired ? `${Math.round(v * 100)}%` : "—"}
        </Box>
      </Box>
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
      >
        Reuse + extend
      </Typography>
    </Stack>
  );
}
