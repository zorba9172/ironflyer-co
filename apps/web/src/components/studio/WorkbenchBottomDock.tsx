"use client";

// WorkbenchBottomDock — the collapsible drawer that sits along the
// bottom of the workbench shell. It hosts three tabs:
//
//   • Patches  — the live PatchesPane (proposals + rollbacks).
//   • Logs     — a terminal-style transcript of the chat event stream.
//   • Changes  — the list of files this execution wrote, sourced from
//                executionSupportBundle.changedFiles.
//
// The dock is collapsed by default. A vertical drag handle on the top
// edge lets the operator resize between MIN_DOCK_HEIGHT and
// MAX_DOCK_HEIGHT (clamped inside useWorkbenchLayout).

import {
  CloseRounded,
  DifferenceRounded,
  EditNoteRounded,
  TerminalRounded,
} from "@mui/icons-material";
import { Box, Stack, Tooltip, Typography } from "@mui/material";
import type { SvgIconComponent } from "@mui/icons-material";
import dynamic from "next/dynamic";
import { useCallback, useRef, type ReactNode } from "react";
import { LoadingPanel } from "../cockpit/LoadingPanel";
import { tokens } from "../../theme";
import { useExecutionSupportBundleQuery } from "../../lib/gql/__generated__";
import { relativeTime } from "../../lib/relativeTime";
import type { StudioMessage } from "./types";
import type { WorkbenchDockTab } from "./useWorkbenchLayout";

const PatchesPane = dynamic(
  () => import("./PatchesPane").then((m) => m.PatchesPane),
  {
    ssr: false,
    loading: () => <LoadingPanel label="Loading patches" minHeight="100%" />,
  },
);

export interface WorkbenchBottomDockProps {
  open: boolean;
  tab: WorkbenchDockTab;
  height: number;
  projectID: string;
  executionID: string;
  executionStatus: string;
  messages: StudioMessage[];
  onTabChange: (next: WorkbenchDockTab) => void;
  onClose: () => void;
  onHeightChange: (px: number) => void;
}

interface TabSpec {
  key: WorkbenchDockTab;
  label: string;
  icon: SvgIconComponent;
}

const TABS: TabSpec[] = [
  { key: "patches", label: "Patches", icon: DifferenceRounded },
  { key: "logs", label: "Logs", icon: TerminalRounded },
  { key: "changes", label: "Files changed", icon: EditNoteRounded },
];

export function WorkbenchBottomDock({
  open,
  tab,
  height,
  projectID,
  executionID,
  executionStatus,
  messages,
  onTabChange,
  onClose,
  onHeightChange,
}: WorkbenchBottomDockProps) {
  // Drag handle. The handle is along the top edge of the dock; mouse
  // movement up shrinks the dock, movement down expands it.
  const dragStartRef = useRef<{ y: number; startHeight: number } | null>(null);
  const onDragStart = useCallback(
    (event: React.MouseEvent<HTMLDivElement>) => {
      event.preventDefault();
      dragStartRef.current = { y: event.clientY, startHeight: height };
      const onMove = (ev: MouseEvent) => {
        const start = dragStartRef.current;
        if (!start) return;
        const delta = start.y - ev.clientY; // pulling up = positive
        onHeightChange(start.startHeight + delta);
      };
      const onUp = () => {
        dragStartRef.current = null;
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    [height, onHeightChange],
  );

  if (!open) return null;

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.surface,
        borderTop: `1px solid ${tokens.color.border.subtle}`,
        display: "flex",
        flex: `0 0 ${height}px`,
        flexDirection: "column",
        height,
        minHeight: 0,
        position: "relative",
      }}
      role="region"
      aria-label="Workbench bottom dock"
    >
      {/* Drag handle */}
      <Box
        role="separator"
        aria-orientation="horizontal"
        onMouseDown={onDragStart}
        sx={{
          cursor: "ns-resize",
          height: 4,
          left: 0,
          position: "absolute",
          right: 0,
          top: -2,
          zIndex: 1,
          "&:hover": { bgcolor: `${tokens.color.accent.violet}55` },
        }}
      />

      {/* Tabs row */}
      <Stack
        direction="row"
        sx={{
          alignItems: "center",
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          gap: 0.25,
          height: 38,
          minHeight: 38,
          px: 1,
        }}
      >
        {TABS.map((t) => {
          const active = tab === t.key;
          const Icon = t.icon;
          return (
            <Box
              key={t.key}
              role="tab"
              aria-selected={active}
              onClick={() => onTabChange(t.key)}
              sx={{
                alignItems: "center",
                bgcolor: active
                  ? `${tokens.color.accent.purple}40`
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
                gap: 0.6,
                height: 28,
                letterSpacing: 0.5,
                px: 1,
                textTransform: "uppercase",
                "&:hover": {
                  bgcolor: active
                    ? `${tokens.color.accent.purple}40`
                    : tokens.color.bg.surfaceHover,
                  color: tokens.color.text.primary,
                },
              }}
            >
              <Icon sx={{ fontSize: 14 }} />
              {t.label}
            </Box>
          );
        })}
        <Box sx={{ flex: 1 }} />
        <Tooltip title="Close dock (⌘J)" arrow>
          <Box
            role="button"
            aria-label="Close dock"
            tabIndex={0}
            onClick={onClose}
            onKeyDown={(e: React.KeyboardEvent<HTMLDivElement>) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClose();
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
            <CloseRounded sx={{ fontSize: 16 }} />
          </Box>
        </Tooltip>
      </Stack>

      {/* Tab body */}
      <Box sx={{ flex: 1, minHeight: 0, overflow: "hidden" }}>
        {tab === "patches" && executionID ? (
          <PatchesPane
            projectID={projectID}
            executionStatus={executionStatus}
          />
        ) : null}
        {tab === "logs" ? <LogsTab messages={messages} /> : null}
        {tab === "changes" ? (
          <ChangesTab executionID={executionID} executionStatus={executionStatus} />
        ) : null}
      </Box>
    </Box>
  );
}

// LogsTab — terminal-style transcript. Every chat message gets one
// monospace row; agent/system/error rows are coloured by role so the
// operator can scan the stream at a glance.
function LogsTab({ messages }: { messages: StudioMessage[] }) {
  if (messages.length === 0) {
    return (
      <DockEmpty
        title="No log entries yet"
        body="Execution events stream into this transcript as the orchestrator emits them."
      />
    );
  }
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.inset,
        color: tokens.color.text.secondary,
        fontFamily: tokens.font.mono,
        fontSize: 11.5,
        height: "100%",
        lineHeight: 1.5,
        overflow: "auto",
        p: 1.2,
      }}
    >
      {messages.map((m) => (
        <LogRow key={m.id} m={m} />
      ))}
    </Box>
  );
}

function logTone(role: string): string {
  switch (role) {
    case "error":
      return tokens.color.accent.danger;
    case "agent_result":
      return tokens.color.accent.success;
    case "agent_action":
    case "agent_progress":
      return tokens.color.accent.violet;
    case "costtick":
      return tokens.color.accent.warning;
    case "user":
      return tokens.color.text.primary;
    default:
      return tokens.color.text.secondary;
  }
}

function LogRow({ m }: { m: StudioMessage }) {
  const tone = logTone(m.role);
  return (
    <Box sx={{ display: "flex", gap: 1, whiteSpace: "pre-wrap" }}>
      <Box
        component="span"
        sx={{ color: tokens.color.text.muted, flex: "0 0 64px" }}
      >
        {relativeTime(m.createdAt)}
      </Box>
      <Box
        component="span"
        sx={{ color: tone, flex: "0 0 110px", fontWeight: 700 }}
      >
        {m.role}
      </Box>
      <Box component="span" sx={{ color: tokens.color.text.secondary, flex: 1, minWidth: 0 }}>
        {m.body}
      </Box>
    </Box>
  );
}

// ChangesTab — the bundle.changedFiles array as a sortable list. We
// keep the renderer minimal here because CodePane already owns rich
// file inspection — this tab is just a quick "what did this run
// change" reference.
function ChangesTab({
  executionID,
  executionStatus,
}: {
  executionID: string;
  executionStatus: string;
}) {
  const isTerminal = new Set(["succeeded", "failed", "stopped", "killed", "refunded"]).has(
    executionStatus,
  );
  const q = useExecutionSupportBundleQuery({
    variables: { executionID },
    skip: !executionID,
    pollInterval: isTerminal ? 0 : 6000,
    fetchPolicy: "cache-and-network",
  });
  if (!executionID) {
    return <DockEmpty title="No execution" body="Start a run to see file changes." />;
  }
  if (q.loading && !q.data) {
    return <LoadingPanel label="Loading changed files" minHeight="100%" />;
  }
  const files = q.data?.executionSupportBundle?.changedFiles ?? [];
  if (files.length === 0) {
    return (
      <DockEmpty
        title="No file changes yet"
        body="The finisher will list every file it writes here as the run progresses."
      />
    );
  }
  return (
    <Box
      sx={{
        height: "100%",
        overflow: "auto",
        p: 1.2,
      }}
    >
      <Stack spacing={0.4}>
        {files.map((path: string) => (
          <Box
            key={path}
            sx={{
              alignItems: "center",
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: 0.75,
              bgcolor: tokens.color.bg.inset,
              color: tokens.color.text.secondary,
              display: "flex",
              fontFamily: tokens.font.mono,
              fontSize: 12,
              gap: 1,
              px: 1,
              py: 0.6,
            }}
          >
            <Box
              sx={{
                bgcolor: tokens.color.accent.violet,
                borderRadius: "50%",
                height: 6,
                width: 6,
              }}
            />
            <Box sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {path}
            </Box>
          </Box>
        ))}
      </Stack>
    </Box>
  );
}

function DockEmpty({ title, body }: { title: string; body: string }) {
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={1}
      sx={{
        color: tokens.color.text.muted,
        height: "100%",
        textAlign: "center",
        p: 3,
      }}
    >
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontFamily: tokens.font.mono,
          fontSize: 12,
          fontWeight: 800,
          letterSpacing: 0.5,
          textTransform: "uppercase",
        }}
      >
        {title}
      </Typography>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontSize: 12.5,
          maxWidth: 360,
        }}
      >
        {body}
      </Typography>
    </Stack>
  );
}

// Wrapper helper kept for ReactNode child support when re-using the
// dock in tests — currently unused but exported for future panels.
export type DockChild = ReactNode;
