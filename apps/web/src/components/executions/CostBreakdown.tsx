"use client";

// CostBreakdown — stacked horizontal bar showing provider / sandbox /
// storage / deployment proportions for a single execution. Tooltip
// rows give exact USD per slice. Dependency-free SVG.

import { Box, Stack, Typography } from "@mui/material";
import { formatMoney, formatPercent } from "../../lib/format";
import { tokens } from "../../theme";

export interface CostBreakdownProps {
  providerCostUSD: number;
  sandboxCostUSD: number;
  storageCostUSD: number;
  deploymentCostUSD: number;
}

const SLICE_COLORS = [
  tokens.color.accent.coral,
  tokens.color.accent.sky,
  tokens.color.accent.purple,
  tokens.color.accent.yellow,
];

export function CostBreakdown(props: CostBreakdownProps) {
  const slices = [
    { key: "provider", label: "Provider", value: props.providerCostUSD },
    { key: "sandbox", label: "Sandbox", value: props.sandboxCostUSD },
    { key: "storage", label: "Storage", value: props.storageCostUSD },
    { key: "deployment", label: "Deployment", value: props.deploymentCostUSD },
  ];
  const total = slices.reduce((acc, s) => acc + Math.max(0, s.value), 0);
  return (
    <Stack spacing={1.5}>
      <Box
        sx={{
          display: "flex",
          height: 14,
          borderRadius: 1,
          overflow: "hidden",
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.inset,
        }}
      >
        {total === 0 ? (
          <Box sx={{ flex: 1, bgcolor: tokens.color.bg.surface }} />
        ) : (
          slices.map((s, i) => {
            const pct = (Math.max(0, s.value) / total) * 100;
            if (pct <= 0) return null;
            return (
              <Box
                key={s.key}
                title={`${s.label}: ${formatMoney(s.value)} (${pct.toFixed(1)}%)`}
                sx={{
                  width: `${pct}%`,
                  bgcolor: SLICE_COLORS[i % SLICE_COLORS.length],
                }}
              />
            );
          })
        )}
      </Box>
      <Stack spacing={0.5}>
        {slices.map((s, i) => {
          const pct = total === 0 ? 0 : (Math.max(0, s.value) / total) * 100;
          return (
            <Stack key={s.key} direction="row" alignItems="center" spacing={1}>
              <Box
                sx={{
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  bgcolor: SLICE_COLORS[i % SLICE_COLORS.length],
                  flexShrink: 0,
                }}
              />
              <Typography sx={{ flex: 1, fontSize: 13, color: tokens.color.text.secondary }}>
                {s.label}
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 12.5,
                  color: tokens.color.text.muted,
                }}
              >
                {formatPercent(pct)}
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 12.5,
                  fontWeight: 700,
                  color: tokens.color.text.primary,
                  minWidth: 80,
                  textAlign: "right",
                }}
              >
                {formatMoney(s.value)}
              </Typography>
            </Stack>
          );
        })}
      </Stack>
    </Stack>
  );
}
