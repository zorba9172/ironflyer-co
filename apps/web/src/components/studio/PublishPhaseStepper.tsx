"use client";

// PublishPhaseStepper — horizontal 5-step indicator at the top of the
// PublishDialog. Each step renders as a small pill with a leading
// status glyph and a label; connectors between pills inherit the
// upstream step's tone.
//
// States per step:
//   pending  — gray dot
//   active   — lime spinner (work in flight)
//   success  — lime check
//   failed   — red X
//   skipped  — gray dash ("approval not required")
//
// The component is purely presentational; the parent PublishDialog
// reduces the publish state machine and tells us which step is
// current and how each prior step resolved.

import { CheckRounded, CloseRounded, RemoveRounded } from "@mui/icons-material";
import { Box, CircularProgress, Stack, Typography } from "@mui/material";
import { tokens } from "../../theme";

export type PhaseStepKey = "plan" | "build" | "approve" | "promote" | "live";
export type PhaseStepState = "pending" | "active" | "success" | "failed" | "skipped";

export interface PhaseStep {
  key: PhaseStepKey;
  label: string;
  state: PhaseStepState;
}

export interface PublishPhaseStepperProps {
  steps: PhaseStep[];
}

const TONE: Record<
  PhaseStepState,
  { fg: string; bg: string; border: string; label: string }
> = {
  pending: {
    fg: tokens.color.text.muted,
    bg: tokens.color.bg.surfaceRaised,
    border: tokens.color.border.subtle,
    label: tokens.color.text.muted,
  },
  active: {
    fg: tokens.color.accent.violet,
    bg: `${tokens.color.accent.violet}1f`,
    border: `${tokens.color.accent.violet}66`,
    label: tokens.color.text.primary,
  },
  success: {
    fg: tokens.color.accent.violet,
    bg: `${tokens.color.accent.violet}1f`,
    border: `${tokens.color.accent.violet}66`,
    label: tokens.color.text.primary,
  },
  failed: {
    fg: tokens.color.accent.danger,
    bg: `${tokens.color.accent.danger}1c`,
    border: `${tokens.color.accent.danger}66`,
    label: tokens.color.accent.danger,
  },
  skipped: {
    fg: tokens.color.text.muted,
    bg: tokens.color.bg.surfaceRaised,
    border: tokens.color.border.subtle,
    label: tokens.color.text.muted,
  },
};

function Glyph({ state }: { state: PhaseStepState }) {
  const tone = TONE[state];
  if (state === "active") {
    return (
      <CircularProgress
        size={12}
        thickness={6}
        sx={{ color: tone.fg }}
        aria-label="In progress"
      />
    );
  }
  if (state === "success") {
    return <CheckRounded sx={{ color: tone.fg, fontSize: 14 }} />;
  }
  if (state === "failed") {
    return <CloseRounded sx={{ color: tone.fg, fontSize: 14 }} />;
  }
  if (state === "skipped") {
    return <RemoveRounded sx={{ color: tone.fg, fontSize: 14 }} />;
  }
  // pending — small filled dot
  return (
    <Box
      sx={{
        width: 6,
        height: 6,
        borderRadius: "50%",
        bgcolor: tone.fg,
      }}
    />
  );
}

export function PublishPhaseStepper({ steps }: PublishPhaseStepperProps) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={0}
      sx={{
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        px: 1,
        py: 1,
        overflowX: "auto",
      }}
    >
      {steps.map((step, i) => {
        const tone = TONE[step.state];
        const prev = i > 0 ? steps[i - 1] : null;
        return (
          <Stack key={step.key} direction="row" alignItems="center" sx={{ flex: "0 0 auto" }}>
            {i > 0 && (
              <Box
                aria-hidden
                sx={{
                  width: 14,
                  height: 1,
                  bgcolor:
                    prev && (prev.state === "success" || prev.state === "skipped")
                      ? tokens.color.accent.violet
                      : tokens.color.border.subtle,
                  mx: 0.5,
                  opacity: prev && prev.state === "skipped" ? 0.5 : 1,
                }}
              />
            )}
            <Stack
              direction="row"
              alignItems="center"
              spacing={0.75}
              sx={{
                bgcolor: tone.bg,
                border: `1px solid ${tone.border}`,
                borderRadius: 0.75,
                px: 0.875,
                py: 0.5,
                minHeight: 24,
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  width: 14,
                  height: 14,
                }}
              >
                <Glyph state={step.state} />
              </Box>
              <Typography
                sx={{
                  color: tone.label,
                  fontFamily: tokens.font.mono,
                  fontSize: 10.5,
                  fontWeight: 800,
                  letterSpacing: 0.8,
                  textTransform: "uppercase",
                  whiteSpace: "nowrap",
                }}
              >
                {step.label}
              </Typography>
            </Stack>
          </Stack>
        );
      })}
    </Stack>
  );
}
