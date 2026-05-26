"use client";

// SuggestionsRow — 3-5 seed prompts contextualised on the execution
// status. The chips are wired to the same onSend pipeline the composer
// uses, so a click immediately fires refineIdea (or its fallback).

import { Chip, Stack } from "@mui/material";
import { tokens } from "../../theme";

type StudioStatusBucket = "idle" | "running" | "succeeded" | "failed";

export interface SuggestionsRowProps {
  status: StudioStatusBucket;
  onPick: (text: string) => void;
  disabled?: boolean;
}

// `recommendedFor` is the alias used by PromptPanel (the locked
// VS-Code-style workbench prompt strip). Keep both exports so callers
// can pick whichever name reads better at the site of use.
export const recommendedFor = suggestionsFor;

export function suggestionsFor(status: StudioStatusBucket): string[] {
  switch (status) {
    case "running":
      return [
        "Tell me what's happening right now",
        "Pause this execution",
        "How much have we spent so far?",
      ];
    case "succeeded":
      return [
        "Deploy to preview",
        "Add a feature",
        "Show me the gate report",
        "Tighten the security findings",
      ];
    case "failed":
      return [
        "Show me the error in detail",
        "Try a different blueprint",
        "Refund and start over",
      ];
    case "idle":
    default:
      return [
        "Describe what to build first",
        "Connect a database",
        "Add authentication",
      ];
  }
}

export function SuggestionsRow({ status, onPick, disabled }: SuggestionsRowProps) {
  const items = suggestionsFor(status);
  return (
    <Stack
      direction="row"
      spacing={0.75}
      sx={{
        bgcolor: tokens.color.bg.surface,
        borderTop: `1px solid ${tokens.color.border.subtle}`,
        flexWrap: "wrap",
        gap: 0.75,
        px: 1.5,
        py: 1,
        rowGap: 0.75,
      }}
    >
      {items.map((s) => (
        <Chip
          key={s}
          size="small"
          label={s}
          onClick={() => !disabled && onPick(s)}
          disabled={disabled}
          sx={{
            bgcolor: tokens.color.bg.surfaceRaised,
            color: tokens.color.text.secondary,
            border: `1px solid ${tokens.color.border.subtle}`,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            fontWeight: 600,
            letterSpacing: 0.2,
            height: 26,
            borderRadius: 0.75,
            cursor: disabled ? "not-allowed" : "pointer",
            "&:hover": {
              bgcolor: tokens.color.bg.surfaceHover,
              borderColor: tokens.color.accent.violet + "55",
              color: tokens.color.text.primary,
            },
            "& .MuiChip-label": { px: 1 },
          }}
        />
      ))}
    </Stack>
  );
}

export type { StudioStatusBucket };
