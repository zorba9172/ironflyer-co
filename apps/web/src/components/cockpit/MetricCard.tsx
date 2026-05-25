"use client";

// MetricCard — single headline KPI tile used across dashboards.
// Optional trend chip ("+12.4%" / "-3.1pp") with directional colour.

import {
  ArrowDownwardRounded,
  ArrowUpwardRounded,
  RemoveRounded,
} from "@mui/icons-material";
import { Box, Card, Stack, Typography, type SxProps, type Theme } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

export type TrendDirection = "up" | "down" | "flat";

export interface MetricTrend {
  direction: TrendDirection;
  label: string;
  // "good" colours up=green, down=red. "inverse" flips it (e.g. cost).
  polarity?: "good" | "inverse";
}

export interface MetricCardProps {
  label: ReactNode;
  value: ReactNode;
  hint?: ReactNode;
  trend?: MetricTrend;
  icon?: ReactNode;
  accent?: "lime" | "sky" | "coral" | "yellow" | "purple" | "neutral";
  sx?: SxProps<Theme>;
}

function trendColor(direction: TrendDirection, polarity: "good" | "inverse"): string {
  if (direction === "flat") return tokens.color.text.muted;
  const goodUp = polarity === "good";
  const isUp = direction === "up";
  return (isUp ? goodUp : !goodUp) ? tokens.color.accent.success : tokens.color.accent.danger;
}

function trendIcon(direction: TrendDirection): ReactNode {
  if (direction === "up") return <ArrowUpwardRounded sx={{ fontSize: 14 }} />;
  if (direction === "down") return <ArrowDownwardRounded sx={{ fontSize: 14 }} />;
  return <RemoveRounded sx={{ fontSize: 14 }} />;
}

// Accent stops the metric tile uses for its left edge bar. The "lime"
// key is kept for backwards-compat with existing callers but now maps
// to mint per the locked palette ("no lime-first identity").
const ACCENT_BAR: Record<NonNullable<MetricCardProps["accent"]>, string> = {
  lime: tokens.color.accent.success,
  sky: tokens.color.accent.sky,
  coral: tokens.color.accent.coral,
  yellow: tokens.color.accent.yellow,
  purple: tokens.color.accent.purple,
  neutral: tokens.color.border.subtle,
};

export function MetricCard({
  label,
  value,
  hint,
  trend,
  icon,
  accent = "neutral",
  sx,
}: MetricCardProps) {
  return (
    <Card
      sx={{
        position: "relative",
        p: { xs: 2, md: 2.5 },
        overflow: "hidden",
        minWidth: 0,
        "&::before": {
          content: '""',
          position: "absolute",
          inset: 0,
          width: 3,
          backgroundColor: ACCENT_BAR[accent],
          opacity: accent === "neutral" ? 0 : 0.9,
        },
        "&:hover": {
          borderColor: tokens.color.border.strong,
          boxShadow: `0 4px 18px ${tokens.color.brand.graphite}66`,
        },
        ...sx,
      }}
    >
      <Stack direction="row" alignItems="flex-start" spacing={1.5}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.75 }}>
            <Typography
              variant="overline"
              sx={{ color: tokens.color.text.secondary, lineHeight: 1.1 }}
            >
              {label}
            </Typography>
            {trend && (
              <Stack
                direction="row"
                alignItems="center"
                spacing={0.25}
                sx={{
                  color: trendColor(trend.direction, trend.polarity ?? "good"),
                  fontSize: 12,
                  fontWeight: 700,
                }}
              >
                {trendIcon(trend.direction)}
                <Box component="span">{trend.label}</Box>
              </Stack>
            )}
          </Stack>
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontWeight: 700,
              fontSize: { xs: 24, md: 28 },
              lineHeight: 1.05,
              letterSpacing: -0.5,
              color: tokens.color.text.primary,
            }}
          >
            {value}
          </Typography>
          {hint && (
            <Typography
              sx={{
                mt: 0.75,
                color: tokens.color.text.muted,
                fontSize: 12,
              }}
            >
              {hint}
            </Typography>
          )}
        </Box>
        {icon && (
          <Box sx={{ color: tokens.color.text.muted, mt: 0.25 }}>{icon}</Box>
        )}
      </Stack>
    </Card>
  );
}
