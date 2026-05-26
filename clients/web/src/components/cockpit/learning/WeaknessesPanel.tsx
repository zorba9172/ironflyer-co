"use client";

// WeaknessesPanel — top 5 weaknesses the learning analyzer has
// detected, sorted by severity. Each row is a compact card with:
//   - dimension chip (colored by severity)
//   - human-readable description
//   - suggested-action button (operator can promote it to a task)
//   - click-expand reveals supporting evidence (paths, sample IDs)
//
// Sentinel: empty weaknesses array means the analyzer hasn't run yet.

import { useState } from "react";
import {
  Box,
  Button,
  Chip,
  Collapse,
  Stack,
  Typography,
} from "@mui/material";
import { ExpandMoreRounded } from "@mui/icons-material";
import { tokens } from "../../../theme";
import { PanelFrame, PanelStubEmpty } from "../health/PanelFrame";
import { severityRank, type LearningDashboardShape, type Weakness } from "./types";

export interface WeaknessesPanelProps {
  data: LearningDashboardShape;
}

export function WeaknessesPanel({ data }: WeaknessesPanelProps) {
  const ranked = [...(data.weaknesses ?? [])].sort(
    (a, b) => severityRank(b.severity) - severityRank(a.severity),
  );
  const top = ranked.slice(0, 5);
  const wired = top.length > 0;

  return (
    <PanelFrame
      eyebrow="Weaknesses"
      title="What the system can&apos;t close yet"
      hint={
        wired
          ? "Ranked by severity. Click a row to see the evidence behind the call-out."
          : undefined
      }
    >
      {!wired ? (
        <PanelStubEmpty>
          The weakness analyzer hasn&apos;t produced any findings yet.
          Once enough outcome events land it surfaces the dimensions
          where the AI under-performs and suggests the next move.
        </PanelStubEmpty>
      ) : (
        <Stack spacing={1}>
          {top.map((w, i) => (
            <WeaknessRow key={`${w.dimension}-${i}`} weakness={w} />
          ))}
        </Stack>
      )}
    </PanelFrame>
  );
}

function severityColor(severity: string): {
  bg: string;
  fg: string;
  border: string;
} {
  switch (severityRank(severity)) {
    case 3:
      return {
        bg: `${tokens.color.accent.coral}22`,
        fg: tokens.color.accent.coral,
        border: `${tokens.color.accent.coral}55`,
      };
    case 2:
      return {
        bg: `${tokens.color.accent.warning}22`,
        fg: tokens.color.accent.warning,
        border: `${tokens.color.accent.warning}55`,
      };
    case 1:
      return {
        bg: `${tokens.color.accent.violet}22`,
        fg: tokens.color.accent.violet,
        border: `${tokens.color.accent.violet}55`,
      };
    default:
      return {
        bg: tokens.color.bg.surfaceRaised,
        fg: tokens.color.text.secondary,
        border: tokens.color.border.subtle,
      };
  }
}

function WeaknessRow({ weakness }: { weakness: Weakness }) {
  const [open, setOpen] = useState(false);
  const sev = severityColor(weakness.severity);
  const evidence = weakness.evidence ?? [];
  const hasEvidence = evidence.length > 0;

  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        bgcolor: tokens.color.bg.inset,
        p: 1.25,
        transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}`,
        "&:hover": { borderColor: tokens.color.border.strong },
      }}
    >
      <Stack direction="row" spacing={1.25} alignItems="flex-start">
        <Chip
          size="small"
          label={weakness.dimension}
          sx={{
            bgcolor: sev.bg,
            color: sev.fg,
            border: `1px solid ${sev.border}`,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            fontWeight: 700,
            letterSpacing: 0.4,
            textTransform: "uppercase",
            height: 22,
            flexShrink: 0,
          }}
        />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            sx={{
              fontSize: 13,
              color: tokens.color.text.primary,
              fontWeight: 600,
              lineHeight: 1.4,
            }}
          >
            {weakness.description}
          </Typography>
          <Stack
            direction="row"
            spacing={1}
            alignItems="center"
            sx={{ mt: 0.75 }}
          >
            <Button
              size="small"
              variant="outlined"
              sx={{
                fontSize: 11.5,
                py: 0.25,
                px: 1,
                minWidth: 0,
                color: tokens.color.text.primary,
                borderColor: tokens.color.border.strong,
                "&:hover": {
                  borderColor: tokens.color.accent.violet,
                  bgcolor: tokens.color.bg.surfaceHover,
                },
              }}
            >
              {weakness.suggestedAction}
            </Button>
            {hasEvidence && (
              <Button
                size="small"
                onClick={() => setOpen((o) => !o)}
                endIcon={
                  <ExpandMoreRounded
                    sx={{
                      fontSize: 16,
                      transform: open ? "rotate(180deg)" : "none",
                      transition: `transform ${tokens.motion.fast} ${tokens.motion.snap}`,
                    }}
                  />
                }
                sx={{
                  fontSize: 11.5,
                  py: 0.25,
                  px: 0.75,
                  minWidth: 0,
                  color: tokens.color.text.secondary,
                }}
              >
                {open ? "Hide evidence" : `Evidence (${evidence.length})`}
              </Button>
            )}
          </Stack>
        </Box>
      </Stack>
      {hasEvidence && (
        <Collapse in={open} unmountOnExit>
          <Stack
            spacing={0.5}
            sx={{
              mt: 1,
              pt: 1,
              borderTop: `1px solid ${tokens.color.border.subtle}`,
            }}
          >
            {evidence.map((e, i) => (
              <Typography
                key={i}
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11.5,
                  color: tokens.color.text.muted,
                  wordBreak: "break-all",
                }}
              >
                {e}
              </Typography>
            ))}
          </Stack>
        </Collapse>
      )}
    </Box>
  );
}
