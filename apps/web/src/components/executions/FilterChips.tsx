"use client";

// FilterChips — small horizontal chip group. Pure presentation: caller
// owns the URL/state mapping. Used by /executions and /deploy.

import { Chip, Stack } from "@mui/material";
import { tokens } from "../../theme";

export interface FilterChipOption<T extends string = string> {
  value: T;
  label: string;
  count?: number;
}

export interface FilterChipsProps<T extends string = string> {
  options: FilterChipOption<T>[];
  value: T;
  onChange: (next: T) => void;
}

export function FilterChips<T extends string = string>({
  options,
  value,
  onChange,
}: FilterChipsProps<T>) {
  return (
    <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ rowGap: 1 }}>
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
              color: active ? tokens.color.text.primary : tokens.color.text.secondary,
              border: `1px solid ${active ? tokens.color.accent.violet : tokens.color.border.subtle}`,
              fontFamily: tokens.font.mono,
              fontWeight: 700,
              fontSize: 11.5,
              letterSpacing: 0.6,
              textTransform: "uppercase",
              cursor: "pointer",
              borderRadius: 0.75,
              height: 26,
              "& .MuiChip-label": { px: 1.25 },
              "&:hover": {
                bgcolor: tokens.color.bg.surfaceHover,
                color: tokens.color.text.primary,
              },
            }}
          />
        );
      })}
    </Stack>
  );
}
