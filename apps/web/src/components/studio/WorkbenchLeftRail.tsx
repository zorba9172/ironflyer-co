"use client";

// WorkbenchLeftRail — the IDE-style icon rail on the left side of the
// workbench shell. Each toggle either swaps the active "primary" pane
// in the center stage (preview / code / files / dashboard) or opens
// the right rail (chat) / the bottom dock (patches).
//
// Two render modes:
//   • collapsed — 56px strip, icon only, tooltip on hover.
//   • expanded  — 200px wide, icon + label, the active row gets a
//                 violet glow.
//
// The rail itself is purely a controlled view — every state and every
// action comes from useWorkbenchLayout via props.

import {
  AccountTreeRounded,
  ChatBubbleOutlineRounded,
  ChevronLeftRounded,
  ChevronRightRounded,
  CodeRounded,
  DifferenceRounded,
  FolderRounded,
  KeyboardCommandKeyRounded,
  LayersRounded,
  PhoneIphoneRounded,
  RocketLaunchRounded,
} from "@mui/icons-material";
import { Box, Stack, Tooltip, Typography } from "@mui/material";
import type { SvgIconComponent } from "@mui/icons-material";
import { tokens } from "../../theme";
import type { WorkbenchPrimary } from "./useWorkbenchLayout";

export interface WorkbenchLeftRailProps {
  expanded: boolean;
  primary: WorkbenchPrimary;
  rightOpen: boolean;
  dockOpen: boolean;
  onTogglePrimary: (next: WorkbenchPrimary) => void;
  onToggleRail: () => void;
  onToggleChat: () => void;
  onToggleDock: () => void;
}

interface PrimaryRow {
  key: WorkbenchPrimary;
  label: string;
  icon: SvgIconComponent;
  shortcut: string;
}

const PRIMARY_ROWS: PrimaryRow[] = [
  { key: "preview", label: "Preview", icon: RocketLaunchRounded, shortcut: "1" },
  { key: "mobile", label: "Mobile", icon: PhoneIphoneRounded, shortcut: "2" },
  { key: "code", label: "Code", icon: CodeRounded, shortcut: "3" },
  { key: "files", label: "Files", icon: FolderRounded, shortcut: "4" },
  { key: "dashboard", label: "Dashboard", icon: LayersRounded, shortcut: "5" },
];

export function WorkbenchLeftRail({
  expanded,
  primary,
  rightOpen,
  dockOpen,
  onTogglePrimary,
  onToggleRail,
  onToggleChat,
  onToggleDock,
}: WorkbenchLeftRailProps) {
  const width = expanded ? 200 : 56;
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        borderRight: `1px solid ${tokens.color.border.subtle}`,
        display: "flex",
        flexDirection: "column",
        flex: `0 0 ${width}px`,
        width,
        minHeight: 0,
        transition: `width ${tokens.motion.fast} ${tokens.motion.curve}`,
      }}
      aria-label="Workbench navigation"
    >
      <Stack
        direction="row"
        alignItems="center"
        sx={{
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          color: tokens.color.text.muted,
          height: 44,
          justifyContent: expanded ? "space-between" : "center",
          px: expanded ? 1.4 : 0,
        }}
      >
        {expanded ? (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              fontWeight: 800,
              letterSpacing: 0.6,
              textTransform: "uppercase",
            }}
          >
            Workspace
          </Typography>
        ) : null}
        <Tooltip
          title={expanded ? "Collapse rail (⌘B)" : "Expand rail (⌘B)"}
          placement="right"
          arrow
        >
          <Box
            role="button"
            aria-label="Toggle navigation rail"
            tabIndex={0}
            onClick={onToggleRail}
            onKeyDown={(e: React.KeyboardEvent<HTMLDivElement>) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onToggleRail();
              }
            }}
            sx={{
              alignItems: "center",
              borderRadius: 0.75,
              color: tokens.color.text.secondary,
              cursor: "pointer",
              display: "flex",
              height: 28,
              justifyContent: "center",
              width: 28,
              "&:hover": {
                bgcolor: tokens.color.bg.surfaceHover,
                color: tokens.color.text.primary,
              },
            }}
          >
            {expanded ? (
              <ChevronLeftRounded sx={{ fontSize: 18 }} />
            ) : (
              <ChevronRightRounded sx={{ fontSize: 18 }} />
            )}
          </Box>
        </Tooltip>
      </Stack>

      <Stack
        spacing={0.4}
        sx={{
          flex: 1,
          minHeight: 0,
          overflowY: "auto",
          px: expanded ? 1 : 0.6,
          py: 1,
        }}
      >
        {PRIMARY_ROWS.map((row) => (
          <RailButton
            key={row.key}
            label={row.label}
            icon={row.icon}
            shortcut={row.shortcut}
            active={primary === row.key}
            expanded={expanded}
            onClick={() => onTogglePrimary(row.key)}
          />
        ))}

        <Box
          sx={{
            borderTop: `1px solid ${tokens.color.border.subtle}`,
            my: 1,
          }}
        />

        <RailButton
          label="Chat"
          icon={ChatBubbleOutlineRounded}
          shortcut="\\"
          active={rightOpen}
          expanded={expanded}
          onClick={onToggleChat}
          activeTone="violet"
        />
        <RailButton
          label="Patches"
          icon={DifferenceRounded}
          shortcut="J"
          active={dockOpen}
          expanded={expanded}
          onClick={onToggleDock}
          activeTone="violet"
        />
      </Stack>

      <Box
        sx={{
          borderTop: `1px solid ${tokens.color.border.subtle}`,
          color: tokens.color.text.muted,
          display: expanded ? "block" : "none",
          fontSize: 10.5,
          px: 1.2,
          py: 1,
        }}
      >
        <Stack direction="row" alignItems="center" spacing={0.5}>
          <KeyboardCommandKeyRounded sx={{ fontSize: 12 }} />
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10,
              letterSpacing: 0.4,
              textTransform: "uppercase",
            }}
          >
            Shortcuts
          </Typography>
        </Stack>
        <Box
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10,
            lineHeight: 1.5,
            mt: 0.6,
          }}
        >
          <div>⌘B · rail</div>
          <div>⌘\ · chat</div>
          <div>⌘J · patches</div>
          <div>F · focus</div>
          <div>1–5 · pane</div>
        </Box>
      </Box>

      <Box
        component="div"
        sx={{
          alignItems: "center",
          borderTop: `1px solid ${tokens.color.border.subtle}`,
          color: tokens.color.text.muted,
          display: "flex",
          gap: 0.6,
          height: 32,
          justifyContent: "center",
          px: expanded ? 1 : 0,
        }}
      >
        <AccountTreeRounded sx={{ color: tokens.color.accent.violet, fontSize: 14 }} />
        {expanded ? (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10,
              letterSpacing: 0.4,
              textTransform: "uppercase",
            }}
          >
            Finisher loop
          </Typography>
        ) : null}
      </Box>
    </Box>
  );
}

function RailButton({
  label,
  icon: Icon,
  shortcut,
  active,
  expanded,
  onClick,
  activeTone = "purple",
}: {
  label: string;
  icon: SvgIconComponent;
  shortcut: string;
  active: boolean;
  expanded: boolean;
  onClick: () => void;
  activeTone?: "purple" | "violet";
}) {
  const activeBg =
    activeTone === "violet"
      ? `${tokens.color.accent.violet}33`
      : `${tokens.color.accent.purple}40`;
  const activeBorder =
    activeTone === "violet"
      ? tokens.color.accent.violet
      : tokens.color.border.accent;
  const button = (
    <Box
      role="button"
      tabIndex={0}
      aria-pressed={active}
      aria-label={label}
      onClick={onClick}
      onKeyDown={(e: React.KeyboardEvent<HTMLDivElement>) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick();
        }
      }}
      sx={{
        alignItems: "center",
        bgcolor: active ? activeBg : "transparent",
        border: `1px solid ${active ? activeBorder : "transparent"}`,
        borderRadius: 0.75,
        color: active ? tokens.color.text.primary : tokens.color.text.secondary,
        cursor: "pointer",
        display: "flex",
        fontSize: 13,
        fontWeight: 700,
        gap: 1,
        height: 36,
        justifyContent: expanded ? "flex-start" : "center",
        px: expanded ? 1.2 : 0,
        transition: `background ${tokens.motion.fast} ease, color ${tokens.motion.fast} ease`,
        "&:hover": {
          bgcolor: active ? activeBg : tokens.color.bg.surfaceHover,
          color: tokens.color.text.primary,
        },
      }}
    >
      <Icon sx={{ fontSize: 17 }} />
      {expanded ? (
        <>
          <Box sx={{ flex: 1, minWidth: 0 }}>{label}</Box>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10,
              letterSpacing: 0.4,
            }}
          >
            {shortcut}
          </Typography>
        </>
      ) : null}
    </Box>
  );
  if (expanded) return button;
  return (
    <Tooltip title={`${label} · ${shortcut}`} placement="right" arrow>
      {button}
    </Tooltip>
  );
}
