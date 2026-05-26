"use client";

// CodeModeSwitcher — wraps the Code pane and lets the operator flip
// between the lightweight Monaco preview (fast, read-only, shows the
// finisher's writes inline) and the slim openvscode-server IDE (real
// terminal/debugger with IronFlyer chrome trimmed down). The choice is persisted
// in the workbench layout state so it survives reloads.
//
// The switcher header is intentionally slim so it doesn't eat the
// Code pane's own toolbar — a single row with the segmented control
// on the left and (when the IDE is selected) a pop-out shortcut on
// the right, dropped into the page through `IDEFramePane`'s own
// header.

import { CodeRounded, LaptopMacRounded } from "@mui/icons-material";
import type { SvgIconComponent } from "@mui/icons-material";
import { Box, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../theme";
import { IDEFramePane } from "./IDEFramePane";
import type { WorkbenchCodeMode } from "./useWorkbenchLayout";

export interface CodeModeSwitcherProps {
  mode: WorkbenchCodeMode;
  onModeChange: (mode: WorkbenchCodeMode) => void;
  projectID: string;
  // The Monaco-mode slot is injected by the page so this wrapper
  // stays decoupled from CodePane's GraphQL surface.
  monacoSlot: ReactNode;
}

interface SegmentDef {
  key: WorkbenchCodeMode;
  label: string;
  icon: SvgIconComponent;
  hint: string;
}

const SEGMENTS: SegmentDef[] = [
  {
    key: "monaco",
    label: "Monaco",
    icon: LaptopMacRounded,
    hint: "Read-only file viewer, fastest paint",
  },
  {
    key: "ide",
    label: "IDE",
    icon: CodeRounded,
    hint: "Slim openvscode-server: terminal, debugger, focused code chrome",
  },
];

export function CodeModeSwitcher({
  mode,
  onModeChange,
  projectID,
  monacoSlot,
}: CodeModeSwitcherProps) {
  return (
    <Box
      sx={{
        display: "flex",
        flex: 1,
        flexDirection: "column",
        height: "100%",
        minHeight: 0,
        minWidth: 0,
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{
          alignItems: "center",
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.surface,
          minHeight: 36,
          px: 1.5,
          py: 0.5,
        }}
        role="tablist"
        aria-label="Code mode"
      >
        <Stack
          direction="row"
          spacing={0.25}
          sx={{
            alignItems: "center",
            bgcolor: tokens.color.bg.inset,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            p: 0.25,
          }}
        >
          {SEGMENTS.map((seg) => {
            const active = mode === seg.key;
            const Icon = seg.icon;
            return (
              <Box
                key={seg.key}
                role="tab"
                aria-selected={active}
                title={seg.hint}
                onClick={() => onModeChange(seg.key)}
                sx={{
                  alignItems: "center",
                  bgcolor: active
                    ? `${tokens.color.accent.purple}55`
                    : "transparent",
                  border: `1px solid ${active ? tokens.color.border.accent : "transparent"}`,
                  borderRadius: 0.75,
                  color: active
                    ? tokens.color.text.primary
                    : tokens.color.text.secondary,
                  cursor: "pointer",
                  display: "flex",
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  fontWeight: 700,
                  gap: 0.5,
                  height: 24,
                  letterSpacing: 0.6,
                  px: 1,
                  textTransform: "uppercase",
                  transition: `background ${tokens.motion.fast} ease, color ${tokens.motion.fast} ease`,
                  "&:hover": { color: tokens.color.text.primary },
                }}
              >
                <Icon sx={{ fontSize: 13 }} />
                {seg.label}
              </Box>
            );
          })}
        </Stack>
        <Box sx={{ flex: 1 }} />
        <Typography
          sx={{
            color: tokens.color.text.muted,
            display: { xs: "none", md: "inline" },
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 0.5,
            textTransform: "uppercase",
          }}
        >
          {mode === "ide"
            ? "Slim VS Code · openvscode-server"
            : "Read-only · Monaco preview"}
        </Typography>
      </Stack>

      <Box
        sx={{
          display: "flex",
          flex: 1,
          flexDirection: "column",
          minHeight: 0,
          minWidth: 0,
        }}
      >
        {mode === "ide" ? (
          <IDEFramePane projectID={projectID} />
        ) : (
          monacoSlot
        )}
      </Box>
    </Box>
  );
}
