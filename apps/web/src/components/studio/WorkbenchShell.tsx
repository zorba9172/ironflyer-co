"use client";

// WorkbenchShell — the single outer card that owns the IDE-grade studio
// layout. One card, one border, internal dividers — no floating
// sub-cards.
//
// Layout:
//
//   ┌─ WorkbenchHeader (status strip below) ─────────────────────────┐
//   │ left rail │ center stage │ right rail (chat)                   │
//   │  (icons)  │  (primary)   │                                     │
//   ├───────────┴──────────────┴─────────────────────────────────────┤
//   │                bottom dock (patches / logs / changes)          │
//   └────────────────────────────────────────────────────────────────┘
//
// Collapse state, primary pane and the bottom dock tab all come from
// useWorkbenchLayout (persisted to localStorage). Keyboard shortcuts
// are wired here once with a global keydown listener:
//
//   ⌘B — toggle left rail
//   ⌘\ — toggle right rail (chat)
//   ⌘J — toggle bottom dock
//   F  — focus mode (hides every rail)
//   1  — primary = preview
//   2  — primary = code
//   3  — primary = files
//   4  — primary = dashboard
//
// Focus mode is a separate boolean that overrides leftOpen/rightOpen/
// dockOpen so the center stage owns the whole surface. Toggling focus
// off restores whatever the user had open before.

import {
  ChatBubbleOutlineRounded,
  CodeRounded,
  DifferenceRounded,
  FolderRounded,
  LayersRounded,
  RocketLaunchRounded,
} from "@mui/icons-material";
import {
  Box,
  Drawer,
  IconButton,
  Tab,
  Tabs,
  Tooltip,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import { useEffect, useState, type ReactNode } from "react";
import { tokens } from "../../theme";
import { WorkbenchBottomDock } from "./WorkbenchBottomDock";
import { WorkbenchHeader } from "./WorkbenchHeader";
import { WorkbenchLeftRail } from "./WorkbenchLeftRail";
import type {
  WorkbenchDockTab,
  WorkbenchPrimary,
} from "./useWorkbenchLayout";
import type { ExecutionCoreFragment } from "../../lib/gql/__generated__";
import type { StudioMessage } from "./types";

const RIGHT_RAIL_WIDTH = 420;
const RIGHT_RAIL_COLLAPSED = 44;
const LEFT_RAIL_EXPANDED = 232;
const LEFT_RAIL_COLLAPSED = 52;

export interface WorkbenchShellProps {
  projectName: string;
  projectID: string;
  execution: ExecutionCoreFragment | null;
  messages: StudioMessage[];

  // Layout state (from useWorkbenchLayout).
  primary: WorkbenchPrimary;
  leftOpen: boolean;
  rightOpen: boolean;
  dockOpen: boolean;
  dockTab: WorkbenchDockTab;
  dockHeight: number;
  focus: boolean;

  setPrimary: (next: WorkbenchPrimary) => void;
  toggleLeft: () => void;
  toggleRight: () => void;
  toggleDock: () => void;
  toggleFocus: () => void;
  setDockTab: (tab: WorkbenchDockTab) => void;
  setDockHeight: (px: number) => void;

  // Slots — the page provides the actual panes.
  previewSlot: ReactNode;
  mobileSlot?: ReactNode;
  codeSlot: ReactNode;
  filesSlot: ReactNode;
  dashboardSlot: ReactNode;
  chatSlot: ReactNode;

  // Optional last-patch headline for the status strip.
  lastPatchSummary?: string | null;
}

export function WorkbenchShell(props: WorkbenchShellProps) {
  const {
    projectName,
    projectID,
    execution,
    messages,
    primary,
    leftOpen,
    rightOpen,
    dockOpen,
    dockTab,
    dockHeight,
    focus,
    setPrimary,
    toggleLeft,
    toggleRight,
    toggleDock,
    toggleFocus,
    setDockTab,
    setDockHeight,
    previewSlot,
    mobileSlot,
    codeSlot,
    filesSlot,
    dashboardSlot,
    chatSlot,
    lastPatchSummary,
  } = props;

  // Keyboard shortcuts. Bound once on mount; cleanup runs on unmount.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const onKey = (event: KeyboardEvent) => {
      // Skip when the operator is typing into a text input / textarea /
      // contentEditable surface — the chat composer and prompt panel
      // must not steal "F" or "1".
      const target = event.target as HTMLElement | null;
      if (target) {
        const tag = target.tagName;
        if (
          tag === "INPUT" ||
          tag === "TEXTAREA" ||
          tag === "SELECT" ||
          target.isContentEditable
        ) {
          return;
        }
      }
      const mod = event.metaKey || event.ctrlKey;
      if (mod && event.key.toLowerCase() === "b") {
        event.preventDefault();
        toggleLeft();
        return;
      }
      if (mod && event.key === "\\") {
        event.preventDefault();
        toggleRight();
        return;
      }
      if (mod && event.key.toLowerCase() === "j") {
        event.preventDefault();
        toggleDock();
        return;
      }
      if (!mod && (event.key === "f" || event.key === "F")) {
        // Plain F. Don't intercept when shift+F or alt+F is pressed —
        // those are common in OS shortcuts.
        if (event.altKey || event.shiftKey) return;
        event.preventDefault();
        toggleFocus();
        return;
      }
      if (!mod && !event.altKey && !event.shiftKey) {
        if (event.key === "1") {
          event.preventDefault();
          setPrimary("preview");
        } else if (event.key === "2") {
          event.preventDefault();
          setPrimary("mobile");
        } else if (event.key === "3") {
          event.preventDefault();
          setPrimary("code");
        } else if (event.key === "4") {
          event.preventDefault();
          setPrimary("files");
        } else if (event.key === "5") {
          event.preventDefault();
          setPrimary("dashboard");
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [toggleLeft, toggleRight, toggleDock, toggleFocus, setPrimary]);

  // Below md (≤ ~900px) we collapse the IDE into a stacked layout that
  // is usable at iPhone 14 width. Rails disappear; the operator swipes
  // between Chat / Code / Preview / Files / Dashboard via a top tab
  // bar; the bottom dock becomes a slide-up sheet.
  const muiTheme = useTheme();
  const isMobile = useMediaQuery(muiTheme.breakpoints.down("md"));

  // Focus mode overrides every rail and the dock.
  const showLeft = !focus && leftOpen;
  const showRight = !focus && rightOpen;
  const showDock = !focus && dockOpen;

  if (isMobile) {
    return (
      <MobileWorkbench
        projectName={projectName}
        projectID={projectID}
        execution={execution}
        messages={messages}
        primary={primary}
        rightOpen={rightOpen}
        dockOpen={dockOpen}
        dockTab={dockTab}
        dockHeight={dockHeight}
        focus={focus}
        setPrimary={setPrimary}
        toggleRight={toggleRight}
        toggleDock={toggleDock}
        toggleFocus={toggleFocus}
        setDockTab={setDockTab}
        setDockHeight={setDockHeight}
        previewSlot={previewSlot}
        mobileSlot={mobileSlot}
        codeSlot={codeSlot}
        filesSlot={filesSlot}
        dashboardSlot={dashboardSlot}
        chatSlot={chatSlot}
        lastPatchSummary={lastPatchSummary}
      />
    );
  }

  // When the left rail is closed we still surface the rail itself in
  // collapsed form so the operator has a way back. Collapsed rail width
  // mirrors WorkbenchLeftRail's internal collapsed width.
  const leftWidth = showLeft
    ? LEFT_RAIL_EXPANDED
    : focus
      ? 0
      : LEFT_RAIL_COLLAPSED;

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        display: "flex",
        flexDirection: "column",
        height: "100%",
        minHeight: 0,
        minWidth: 0,
        p: { xs: 0.6, md: 0.9 },
        width: "100%",
      }}
    >
      <Box
        sx={{
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1.5,
          bgcolor: tokens.color.bg.surface,
          boxShadow: tokens.shadow.md,
          display: "flex",
          flex: 1,
          flexDirection: "column",
          minHeight: 0,
          minWidth: 0,
          overflow: "hidden",
        }}
      >
        <WorkbenchHeader
          projectName={projectName}
          projectID={projectID}
          execution={execution}
          primary={primary}
          onPrimaryChange={setPrimary}
          focus={focus}
          onToggleFocus={toggleFocus}
          lastPatchSummary={lastPatchSummary}
        />

        <Box
          sx={{
            display: "flex",
            flex: 1,
            minHeight: 0,
            minWidth: 0,
          }}
        >
          {!focus ? (
            <WorkbenchLeftRail
              expanded={leftOpen}
              primary={primary}
              rightOpen={rightOpen}
              dockOpen={dockOpen}
              onTogglePrimary={(p: WorkbenchPrimary) => setPrimary(p)}
              onToggleRail={toggleLeft}
              onToggleChat={toggleRight}
              onToggleDock={toggleDock}
            />
          ) : null}

          <Box
            sx={{
              display: "flex",
              flex: 1,
              flexDirection: "column",
              minHeight: 0,
              minWidth: 0,
            }}
          >
            <Box
              sx={{
                display: "flex",
                flex: 1,
                minHeight: 0,
                minWidth: 0,
              }}
            >
              {/* Center stage */}
              <Box
                sx={{
                  display: "flex",
                  flex: 1,
                  flexDirection: "column",
                  minHeight: 0,
                  minWidth: 0,
                  position: "relative",
                }}
                role="region"
                aria-label="Workbench primary pane"
              >
                <PrimaryStage
                  primary={primary}
                  preview={previewSlot}
                  mobile={mobileSlot ?? previewSlot}
                  code={codeSlot}
                  files={filesSlot}
                  dashboard={dashboardSlot}
                />
              </Box>

              {/* Right rail (chat) */}
              {!focus && (
                <Box
                  sx={{
                    bgcolor: tokens.color.bg.surface,
                    borderLeft: `1px solid ${tokens.color.border.subtle}`,
                    display: "flex",
                    flex: `0 0 ${showRight ? RIGHT_RAIL_WIDTH : RIGHT_RAIL_COLLAPSED}px`,
                    flexDirection: "column",
                    minHeight: 0,
                    transition: `flex-basis ${tokens.motion.fast} ${tokens.motion.curve}`,
                    width: showRight ? RIGHT_RAIL_WIDTH : RIGHT_RAIL_COLLAPSED,
                  }}
                  role="complementary"
                  aria-label="Chat panel"
                >
                  {showRight ? (
                    <Box sx={{ display: "flex", flex: 1, minHeight: 0, minWidth: 0 }}>
                      {chatSlot}
                    </Box>
                  ) : (
                    <CollapsedChatStrip
                      onExpand={toggleRight}
                      messageCount={messages.length}
                    />
                  )}
                </Box>
              )}
            </Box>

            {/* Bottom dock */}
            {showDock ? (
              <WorkbenchBottomDock
                open={showDock}
                tab={dockTab}
                height={dockHeight}
                projectID={projectID}
                executionID={execution?.id ?? ""}
                executionStatus={execution?.status ?? "idle"}
                messages={messages}
                onTabChange={setDockTab}
                onClose={toggleDock}
                onHeightChange={setDockHeight}
              />
            ) : null}
          </Box>
        </Box>
      </Box>
    </Box>
  );
}

function PrimaryStage({
  primary,
  preview,
  mobile,
  code,
  files,
  dashboard,
}: {
  primary: WorkbenchPrimary;
  preview: ReactNode;
  mobile: ReactNode;
  code: ReactNode;
  files: ReactNode;
  dashboard: ReactNode;
}) {
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        display: "flex",
        flex: 1,
        flexDirection: "column",
        minHeight: 0,
        minWidth: 0,
      }}
    >
      {primary === "preview" ? preview : null}
      {primary === "mobile" ? mobile : null}
      {primary === "code" ? code : null}
      {primary === "files" ? files : null}
      {primary === "dashboard" ? dashboard : null}
    </Box>
  );
}

function CollapsedChatStrip({
  onExpand,
  messageCount,
}: {
  onExpand: () => void;
  messageCount: number;
}) {
  return (
    <Box
      sx={{
        alignItems: "center",
        display: "flex",
        flex: 1,
        flexDirection: "column",
        gap: 0.8,
        justifyContent: "flex-start",
        minHeight: 0,
        pt: 1.2,
      }}
    >
      <Tooltip title="Open chat (⌘\)" placement="left" arrow>
        <IconButton
          size="small"
          aria-label="Open chat"
          onClick={onExpand}
          sx={{
            border: `1px solid ${tokens.color.border.subtle}`,
            color: tokens.color.text.secondary,
            "&:hover": {
              bgcolor: tokens.color.bg.surfaceHover,
              color: tokens.color.accent.violet,
            },
          }}
        >
          <ChatBubbleOutlineRounded sx={{ fontSize: 17 }} />
        </IconButton>
      </Tooltip>
      <Box
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 10,
          letterSpacing: 0.4,
          textTransform: "uppercase",
          writingMode: "vertical-rl",
        }}
      >
        {messageCount === 0 ? "Chat" : `${messageCount} msgs`}
      </Box>
    </Box>
  );
}

// ──────────────────────────────────────────────────────────────────────
// MobileWorkbench — 390px-friendly stacked layout used below md. The
// rails disappear; the operator swipes between Chat / Preview / Code /
// Files / Dashboard via a top tab bar. The bottom dock turns into a
// slide-up sheet (MUI Drawer anchored to the bottom).
// ──────────────────────────────────────────────────────────────────────

type MobileTab = "chat" | "preview" | "code" | "files" | "dashboard";

interface MobileWorkbenchProps {
  projectName: string;
  projectID: string;
  execution: ExecutionCoreFragment | null;
  messages: StudioMessage[];
  primary: WorkbenchPrimary;
  rightOpen: boolean;
  dockOpen: boolean;
  dockTab: WorkbenchShellProps["dockTab"];
  dockHeight: number;
  focus: boolean;
  setPrimary: (next: WorkbenchPrimary) => void;
  toggleRight: () => void;
  toggleDock: () => void;
  toggleFocus: () => void;
  setDockTab: WorkbenchShellProps["setDockTab"];
  setDockHeight: (px: number) => void;
  previewSlot: ReactNode;
  mobileSlot?: ReactNode;
  codeSlot: ReactNode;
  filesSlot: ReactNode;
  dashboardSlot: ReactNode;
  chatSlot: ReactNode;
  lastPatchSummary?: string | null;
}

function MobileWorkbench(props: MobileWorkbenchProps) {
  const {
    projectName,
    projectID,
    execution,
    messages,
    primary,
    dockOpen,
    dockTab,
    dockHeight,
    focus,
    setPrimary,
    toggleDock,
    toggleFocus,
    setDockTab,
    setDockHeight,
    previewSlot,
    mobileSlot,
    codeSlot,
    filesSlot,
    dashboardSlot,
    chatSlot,
    lastPatchSummary,
  } = props;

  // The mobile tab state mirrors `primary` for preview/code/files/
  // dashboard, plus a dedicated "chat" tab that takes over the stage.
  const [tab, setTab] = useState<MobileTab>("chat");

  // Keep the parent's primary in sync when the operator picks a pane
  // tab so desktop and mobile layouts share the same selection.
  useEffect(() => {
    if (tab === "chat") return;
    if (tab !== primary) setPrimary(tab);
  }, [tab, primary, setPrimary]);

  const tabs: Array<{ key: MobileTab; label: string; icon: typeof CodeRounded }> = [
    { key: "chat", label: "Chat", icon: ChatBubbleOutlineRounded },
    { key: "preview", label: "Preview", icon: RocketLaunchRounded },
    { key: "code", label: "Code", icon: CodeRounded },
    { key: "files", label: "Files", icon: FolderRounded },
    { key: "dashboard", label: "Stats", icon: LayersRounded },
  ];

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        display: "flex",
        flexDirection: "column",
        height: "100%",
        minHeight: 0,
        minWidth: 0,
        width: "100%",
      }}
    >
      <Box
        sx={{
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          bgcolor: tokens.color.bg.surface,
          display: "flex",
          flex: 1,
          flexDirection: "column",
          minHeight: 0,
          minWidth: 0,
          m: 0.5,
          overflow: "hidden",
        }}
      >
        <WorkbenchHeader
          projectName={projectName}
          projectID={projectID}
          execution={execution}
          primary={primary}
          onPrimaryChange={setPrimary}
          focus={focus}
          onToggleFocus={toggleFocus}
          lastPatchSummary={lastPatchSummary}
        />

        <Tabs
          value={tab}
          onChange={(_, next: MobileTab) => setTab(next)}
          variant="scrollable"
          scrollButtons={false}
          sx={{
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
            bgcolor: tokens.color.bg.surface,
            minHeight: 42,
            "& .MuiTab-root": {
              minHeight: 42,
              minWidth: 0,
              px: 1.5,
              fontSize: 12,
              fontWeight: 700,
              textTransform: "none",
              color: tokens.color.text.secondary,
              "&.Mui-selected": { color: tokens.color.text.primary },
            },
            "& .MuiTabs-indicator": {
              backgroundColor: tokens.color.accent.violet,
              height: 2,
            },
          }}
        >
          {tabs.map((t) => {
            const Icon = t.icon;
            return (
              <Tab
                key={t.key}
                value={t.key}
                icon={<Icon sx={{ fontSize: 16 }} />}
                iconPosition="start"
                label={t.label}
              />
            );
          })}
        </Tabs>

        <Box
          sx={{
            bgcolor: tokens.color.bg.base,
            display: "flex",
            flex: 1,
            flexDirection: "column",
            minHeight: 0,
            minWidth: 0,
            paddingBottom: "env(safe-area-inset-bottom)",
          }}
          role="region"
          aria-label="Workbench mobile pane"
        >
          {tab === "chat" ? chatSlot : null}
          {tab === "preview" ? previewSlot : null}
          {tab === "code" ? codeSlot : null}
          {tab === "files" ? filesSlot : null}
          {tab === "dashboard" ? dashboardSlot : null}
        </Box>

        <Box
          sx={{
            borderTop: `1px solid ${tokens.color.border.subtle}`,
            bgcolor: tokens.color.bg.surface,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            px: 1.25,
            py: 0.75,
            gap: 1,
            paddingBottom: `calc(0.75rem + env(safe-area-inset-bottom))`,
          }}
        >
          <Box
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.6,
              textTransform: "uppercase",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              flex: 1,
              minWidth: 0,
            }}
          >
            {messages.length} msgs · {execution?.status ?? "idle"}
          </Box>
          <Tooltip title={dockOpen ? "Hide patches" : "Show patches"} arrow>
            <IconButton
              size="small"
              aria-label="Toggle patches sheet"
              onClick={toggleDock}
              sx={{
                border: `1px solid ${tokens.color.border.subtle}`,
                color: dockOpen ? tokens.color.accent.violet : tokens.color.text.secondary,
                minWidth: 44,
                minHeight: 44,
                "&:hover": {
                  bgcolor: tokens.color.bg.surfaceHover,
                  color: tokens.color.accent.violet,
                },
              }}
            >
              <DifferenceRounded sx={{ fontSize: 18 }} />
            </IconButton>
          </Tooltip>
        </Box>
      </Box>

      <Drawer
        anchor="bottom"
        open={dockOpen}
        onClose={toggleDock}
        slotProps={{
          paper: {
            sx: {
              height: "70vh",
              bgcolor: tokens.color.bg.surface,
              borderTopLeftRadius: 12,
              borderTopRightRadius: 12,
              border: `1px solid ${tokens.color.border.subtle}`,
              overflow: "hidden",
              paddingBottom: "env(safe-area-inset-bottom)",
            },
          },
        }}
      >
        <WorkbenchBottomDock
          open={dockOpen}
          tab={dockTab}
          height={dockHeight}
          projectID={projectID}
          executionID={execution?.id ?? ""}
          executionStatus={execution?.status ?? "idle"}
          messages={messages}
          onTabChange={setDockTab}
          onClose={toggleDock}
          onHeightChange={setDockHeight}
        />
      </Drawer>
    </Box>
  );
}
