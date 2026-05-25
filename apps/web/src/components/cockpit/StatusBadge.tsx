"use client";

// StatusBadge — colour-coded pill for execution / deploy / approval
// statuses. The status vocabulary mirrors the orchestrator's domain
// constants:
//   execution: created | admitted | running | succeeded | failed |
//              stopped | killed | refunded | scoring
//   deploy:    planned | building | preview_ready | awaiting_approval |
//              approved | promoting | live | rolled_back | cancelled |
//              failed
//   approval:  pending | approved | rejected | expired | withdrawn
// Unknown values render as a neutral pill — never throw.

import { Chip, type SxProps, type Theme } from "@mui/material";
import { tokens } from "../../theme";

type Tone = "success" | "danger" | "warning" | "info" | "neutral" | "accent";

const TONE_STYLES: Record<Tone, { bg: string; fg: string; border: string }> = {
  success: {
    bg: `${tokens.color.accent.success}1c`,
    fg: tokens.color.accent.success,
    border: `${tokens.color.accent.success}55`,
  },
  danger: {
    bg: `${tokens.color.accent.danger}1c`,
    fg: tokens.color.accent.danger,
    border: `${tokens.color.accent.danger}55`,
  },
  warning: {
    bg: `${tokens.color.accent.warning}1f`,
    fg: tokens.color.accent.warning,
    border: `${tokens.color.accent.warning}66`,
  },
  info: {
    bg: `${tokens.color.accent.sky}1c`,
    fg: tokens.color.accent.sky,
    border: `${tokens.color.accent.sky}55`,
  },
  accent: {
    // "Accent" tone = live / in-progress states (running, approved,
    // promoting, pass). Per the constitution this routes to mint, not
    // lime — lime is forbidden as identity color.
    bg: `${tokens.color.accent.success}1f`,
    fg: tokens.color.accent.success,
    border: `${tokens.color.accent.success}66`,
  },
  neutral: {
    bg: tokens.color.bg.surfaceRaised,
    fg: tokens.color.text.secondary,
    border: tokens.color.border.subtle,
  },
};

const STATUS_TONE: Record<string, Tone> = {
  // execution
  created: "neutral",
  admitted: "info",
  running: "accent",
  scoring: "info",
  succeeded: "success",
  success: "success",
  failed: "danger",
  stopped: "warning",
  killed: "danger",
  refunded: "warning",
  // deploy
  planned: "neutral",
  building: "info",
  preview_ready: "info",
  awaiting_approval: "warning",
  approved: "accent",
  promoting: "accent",
  live: "success",
  rolled_back: "warning",
  cancelled: "neutral",
  // approval
  pending: "warning",
  rejected: "danger",
  expired: "neutral",
  withdrawn: "neutral",
  // generic
  ok: "success",
  pass: "success",
  warn: "warning",
  warning: "warning",
  error: "danger",
  fail: "danger",
};

function prettyLabel(status: string): string {
  return status.replace(/_/g, " ");
}

export interface StatusBadgeProps {
  status: string | null | undefined;
  // Optional override when the caller knows better than the generic
  // mapping (e.g. "succeeded with refund" should render warning).
  tone?: Tone;
  uppercase?: boolean;
  sx?: SxProps<Theme>;
}

export function StatusBadge({ status, tone, uppercase = true, sx }: StatusBadgeProps) {
  const s = (status || "unknown").toLowerCase();
  const resolved = tone ?? STATUS_TONE[s] ?? "neutral";
  const styles = TONE_STYLES[resolved];
  // Live-state pulse for in-flight executions / deploys. We hide it
  // behind prefers-reduced-motion via the global CssBaseline override.
  const live = s === "running" || s === "promoting" || s === "building" || s === "scoring";
  return (
    <Chip
      size="small"
      label={uppercase ? prettyLabel(s).toUpperCase() : prettyLabel(s)}
      sx={{
        bgcolor: styles.bg,
        color: styles.fg,
        border: `1px solid ${styles.border}`,
        fontFamily: tokens.font.mono,
        fontWeight: 700,
        fontSize: 10.5,
        letterSpacing: 0.8,
        height: 22,
        borderRadius: 0.75,
        position: "relative",
        "& .MuiChip-label": {
          px: 1,
          display: "inline-flex",
          alignItems: "center",
          gap: 0.6,
        },
        ...(live && {
          "& .MuiChip-label::before": {
            content: '""',
            display: "inline-block",
            width: 6,
            height: 6,
            borderRadius: "50%",
            backgroundColor: styles.fg,
            boxShadow: `0 0 0 0 ${styles.fg}`,
            animation: "ironflyerStatusPulse 1.6s ease-in-out infinite",
          },
          "@keyframes ironflyerStatusPulse": {
            "0%, 100%": { opacity: 1, transform: "scale(1)" },
            "50%": { opacity: 0.55, transform: "scale(0.78)" },
          },
        }),
        ...sx,
      }}
    />
  );
}
