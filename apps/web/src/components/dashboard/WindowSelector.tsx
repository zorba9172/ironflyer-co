"use client";

// WindowSelector — top-right time window toggle used by the operator
// dashboard. Four fixed windows: 24h, 7d, 30d, 90d. Stays a dumb
// presentational component — the parent owns the URL ↔ state binding.

import { Stack } from "@mui/material";
import { tokens } from "../../theme";

export type DashboardWindow = "24h" | "7d" | "30d" | "90d";

export const WINDOW_OPTIONS: DashboardWindow[] = ["24h", "7d", "30d", "90d"];

export const WINDOW_LABEL: Record<DashboardWindow, string> = {
  "24h": "24h",
  "7d": "7d",
  "30d": "30d",
  "90d": "90d",
};

const WINDOW_HOURS: Record<DashboardWindow, number> = {
  "24h": 24,
  "7d": 24 * 7,
  "30d": 24 * 30,
  "90d": 24 * 90,
};

// windowToRange — caller-facing helper that turns the selected window
// into the {since, until} ISO pair the profit dashboard query expects.
export function windowToRange(window: DashboardWindow): {
  since: string;
  until: string;
} {
  const until = new Date();
  const since = new Date(until.getTime() - WINDOW_HOURS[window] * 60 * 60 * 1000);
  return { since: since.toISOString(), until: until.toISOString() };
}

export interface WindowSelectorProps {
  value: DashboardWindow;
  onChange: (next: DashboardWindow) => void;
}

export function WindowSelector({ value, onChange }: WindowSelectorProps) {
  return (
    <Stack
      direction="row"
      role="tablist"
      aria-label="Dashboard time window"
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        bgcolor: tokens.color.bg.surface,
        p: 0.25,
      }}
    >
      {WINDOW_OPTIONS.map((opt) => {
        const active = opt === value;
        return (
          <button
            key={opt}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(opt)}
            style={{
              appearance: "none",
              border: "none",
              cursor: "pointer",
              padding: "6px 12px",
              minHeight: 28,
              borderRadius: 4,
              backgroundColor: active ? tokens.color.bg.surfaceHover : "transparent",
              color: active ? tokens.color.text.primary : tokens.color.text.secondary,
              fontFamily: tokens.font.mono,
              fontWeight: 700,
              fontSize: 12,
              letterSpacing: 0.6,
              textTransform: "uppercase",
            }}
          >
            {WINDOW_LABEL[opt]}
          </button>
        );
      })}
    </Stack>
  );
}
