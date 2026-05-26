"use client";

// BanditConfidencePanel — hand-rolled SVG semicircle gauge that mirrors
// the multi-armed bandit's confidence in its current strategy. Style
// matches ReuseRatePanel in /cockpit/health: violet arc on a subtle
// border track, mono numeric label, no chart-lib weight.
//
// Sentinel: banditConfidence === -1 means the bandit hasn't trained
// enough to produce a confidence score yet.

import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "../health/PanelFrame";
import type { LearningDashboardShape } from "./types";

export interface BanditConfidencePanelProps {
  data: LearningDashboardShape;
}

export function BanditConfidencePanel({ data }: BanditConfidencePanelProps) {
  const wired = data.banditConfidence >= 0;
  const v = Math.max(0, Math.min(1, data.banditConfidence));

  return (
    <PanelFrame
      eyebrow="Strategy"
      title="Bandit confidence"
      hint={
        wired
          ? "How confident the AI is in its current strategy across blueprints, providers, and repair recipes."
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          The bandit hasn&apos;t recorded enough trials to produce a
          confidence score. After a handful of executions the
          Thompson-sampling layer publishes a posterior and this gauge
          turns live.
        </PanelStubEmpty>
      ) : (
        <Stack alignItems="center" spacing={1.5}>
          <ConfidenceGauge value={v} />
          <Typography
            sx={{
              fontSize: 12.5,
              color: tokens.color.text.secondary,
              textAlign: "center",
              maxWidth: 280,
              lineHeight: 1.5,
            }}
          >
            How confident the AI is in its current strategy
          </Typography>
        </Stack>
      )}
    </PanelFrame>
  );
}

function ConfidenceGauge({ value }: { value: number }) {
  const radius = 64;
  const stroke = 14;
  const circumference = Math.PI * radius;
  const dash = circumference * value;

  return (
    <Stack alignItems="center" spacing={0.5}>
      <Box
        component="svg"
        viewBox="-80 -80 160 96"
        role="img"
        aria-label={`Bandit confidence ${(value * 100).toFixed(0)}%`}
        sx={{ width: 200, height: 120 }}
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
          y="-10"
          textAnchor="middle"
          sx={{
            fill: tokens.color.text.primary,
            fontFamily: tokens.font.mono,
            fontSize: 26,
            fontWeight: 700,
          }}
        >
          {`${Math.round(value * 100)}%`}
        </Box>
      </Box>
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
      >
        Posterior confidence
      </Typography>
    </Stack>
  );
}
