"use client";

// ProfitTodayTile — the single tile every operator should glance at
// before doing anything else: "are we making money on this tenant
// right now?". Reads the tenantProfitToday GraphQL query which
// resolves to a LedgerRollup for [UTC midnight, now), so the answer
// is real-time without the caller having to plumb a DateTime range.
//
// Visual contract:
//   • Default is COMPACT — 1 row, 1 number, color-coded against margin.
//   • Hover/click expands to the full cost breakdown (provider / sandbox
//     / storage / deployment / premium reasoning).
//   • Green when grossMarginPct ≥ 60, amber 30-60, coral < 30, muted
//     when there's no revenue yet today (so the tile doesn't scream
//     "you're losing money" on an empty day).
//   • Polls every 60s so a runaway execution shows up within a minute
//     of starting; cheaper than a subscription for a dashboard tile.
//
// Drop-in anywhere — no required props, all colors via tokens, no
// raw hex inline. Constitutional design-reference rule: every shade
// is a token reference; if a new shade is needed, add it to
// packages/design-tokens first.

import { useState, type ReactElement } from "react";
import { Box, Stack, Typography, Tooltip, CircularProgress } from "@mui/material";
import { TrendingUpRounded, TrendingDownRounded, TrendingFlatRounded } from "@mui/icons-material";
import { tokens } from "../../theme";
import { useTenantProfitTodayQuery } from "../../lib/gql/__generated__";

export interface ProfitTodayTileProps {
  // When true the tile renders the full cost breakdown without
  // requiring hover/click. Used on the Profit dashboard primary
  // surface; defaults to compact for sidebar-style placement.
  expanded?: boolean;
}

export function ProfitTodayTile({ expanded = false }: ProfitTodayTileProps) {
  const [hovered, setHovered] = useState(false);
  const { data, loading, error } = useTenantProfitTodayQuery({
    fetchPolicy: "cache-and-network",
    pollInterval: 60_000,
  });
  const showDetail = expanded || hovered;
  const roll = data?.tenantProfitToday;
  const margin = roll?.platformMarginUSD ?? 0;
  const marginPct = roll?.grossMarginPct ?? 0;
  const revenue = roll?.revenueUSD ?? 0;

  const accent = marginAccent(revenue, marginPct);

  return (
    <Box
      role="region"
      aria-label="Profit today"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: tokens.radius.md / 8,
        boxShadow: tokens.shadow.sm,
        cursor: expanded ? "default" : "pointer",
        minWidth: 240,
        p: 2,
        transition: `border-color ${tokens.motion.fast} ${tokens.motion.curve}`,
        "&:hover": {
          borderColor: tokens.color.border.strong,
        },
      }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" mb={1}>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            fontWeight: 700,
            letterSpacing: 1,
            textTransform: "uppercase",
          }}
        >
          Profit today
        </Typography>
        {accent.icon}
      </Stack>

      {loading && !roll ? (
        <Stack direction="row" alignItems="center" gap={1.5}>
          <CircularProgress size={14} sx={{ color: tokens.color.text.muted }} />
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
            Reading vault…
          </Typography>
        </Stack>
      ) : error ? (
        <Tooltip title={error.message} arrow>
          <Typography sx={{ color: tokens.color.accent.warning, fontFamily: tokens.font.mono, fontSize: 12 }}>
            unable to load profit snapshot
          </Typography>
        </Tooltip>
      ) : (
        <Stack gap={1}>
          <Stack direction="row" alignItems="baseline" gap={1}>
            <Typography
              sx={{
                color: accent.color,
                fontFamily: tokens.font.mono,
                fontSize: 26,
                fontWeight: 800,
                lineHeight: 1.15,
              }}
            >
              {formatUSD(margin)}
            </Typography>
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 12,
              }}
            >
              {revenue > 0 ? `${marginPct.toFixed(1)}% margin` : "no revenue yet"}
            </Typography>
          </Stack>

          <Typography sx={{ color: tokens.color.text.muted, fontSize: 11 }}>
            Revenue {formatUSD(revenue)} · {showDetail ? "click to collapse" : "hover for breakdown"}
          </Typography>

          {showDetail && roll ? (
            <Stack
              gap={0.5}
              sx={{
                borderTop: `1px solid ${tokens.color.border.subtle}`,
                mt: 1,
                pt: 1,
              }}
            >
              <Row label="Provider"     value={roll.providerCostUSD} />
              <Row label="Sandbox"      value={roll.sandboxCostUSD} />
              <Row label="Storage"      value={roll.storageCostUSD} />
              <Row label="Deployment"   value={roll.deploymentCostUSD} />
              <Row label="Premium AI"   value={roll.premiumReasoningCostUSD} />
              <Row label="Refunds"      value={roll.refundsUSD} muted />
            </Stack>
          ) : null}
        </Stack>
      )}
    </Box>
  );
}

function Row({ label, value, muted }: { label: string; value: number; muted?: boolean }) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="center">
      <Typography
        sx={{
          color: muted ? tokens.color.text.muted : tokens.color.text.secondary,
          fontFamily: tokens.font.mono,
          fontSize: 11,
        }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          color: muted ? tokens.color.text.muted : tokens.color.text.primary,
          fontFamily: tokens.font.mono,
          fontSize: 11,
        }}
      >
        {formatUSD(value)}
      </Typography>
    </Stack>
  );
}

interface Accent {
  color: string;
  icon: ReactElement;
}

function marginAccent(revenue: number, marginPct: number): Accent {
  if (revenue <= 0) {
    return {
      color: tokens.color.text.muted,
      icon: <TrendingFlatRounded sx={{ color: tokens.color.text.muted, fontSize: 16 }} />,
    };
  }
  if (marginPct >= 60) {
    return {
      color: tokens.color.accent.success,
      icon: <TrendingUpRounded sx={{ color: tokens.color.accent.success, fontSize: 16 }} />,
    };
  }
  if (marginPct >= 30) {
    return {
      color: tokens.color.accent.warning,
      icon: <TrendingFlatRounded sx={{ color: tokens.color.accent.warning, fontSize: 16 }} />,
    };
  }
  return {
    color: tokens.color.accent.danger,
    icon: <TrendingDownRounded sx={{ color: tokens.color.accent.danger, fontSize: 16 }} />,
  };
}

function formatUSD(amount: number): string {
  if (!Number.isFinite(amount)) return "—";
  const sign = amount < 0 ? "−" : "";
  const abs = Math.abs(amount);
  if (abs >= 1000) {
    return `${sign}$${abs.toFixed(0)}`;
  }
  if (abs >= 1) {
    return `${sign}$${abs.toFixed(2)}`;
  }
  return `${sign}$${abs.toFixed(4)}`;
}
