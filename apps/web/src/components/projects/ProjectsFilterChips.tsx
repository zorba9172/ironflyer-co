"use client";

// ProjectsFilterChips — status filter row for the /projects catalogue.
// Same chip language as the executions FilterChips component (active
// chip uses the violet primary tint), but typed specifically against
// the project filter vocabulary so the projects page does not need to
// reach into the executions module for a shared chip primitive.

import { Chip, Stack } from "@mui/material";
import { tokens } from "../../theme";

export type ProjectFilter = "all" | "active" | "failed" | "archived";

export interface ProjectsFilterChipsOption {
  value: ProjectFilter;
  label: string;
  count?: number;
}

export interface ProjectsFilterChipsProps {
  options: ProjectsFilterChipsOption[];
  value: ProjectFilter;
  onChange: (next: ProjectFilter) => void;
}

export function ProjectsFilterChips({
  options,
  value,
  onChange,
}: ProjectsFilterChipsProps) {
  return (
    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ rowGap: 1 }}>
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <Chip
            key={opt.value}
            label={
              opt.count !== undefined ? `${opt.label} · ${opt.count}` : opt.label
            }
            size="small"
            onClick={() => onChange(opt.value)}
            sx={{
              bgcolor: active
                ? `${tokens.color.accent.violet}22`
                : tokens.color.bg.surface,
              color: active
                ? tokens.color.text.primary
                : tokens.color.text.secondary,
              border: `1px solid ${
                active ? tokens.color.accent.violet : tokens.color.border.subtle
              }`,
              fontFamily: tokens.font.mono,
              fontWeight: 700,
              fontSize: 11.5,
              letterSpacing: 0.6,
              textTransform: "uppercase",
              cursor: "pointer",
              borderRadius: 0.75,
              height: 28,
              minHeight: 28,
              "& .MuiChip-label": { px: 1.25 },
              "&:hover": {
                bgcolor: active
                  ? `${tokens.color.accent.violet}2e`
                  : tokens.color.bg.surfaceHover,
                color: tokens.color.text.primary,
              },
            }}
          />
        );
      })}
    </Stack>
  );
}
