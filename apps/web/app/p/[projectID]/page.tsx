"use client";

// /p/[projectID] — the Studio Workbench.
//
// One unified WorkbenchShell renders the IDE-grade dashboard:
// header + left rail + center stage + right chat rail + collapsible
// bottom dock. Layout state (collapsed rails, focus mode, active pane,
// dock tab, dock height) lives in useWorkbenchLayout and is persisted
// to localStorage keyed by projectID.
//
// What this page owns:
//   1. Resolve the URL projectID → a concrete executionID via the
//      orchestrator's projectExecutions(projectId:) query.
//   2. Subscribe to executionFeed and pipe each event into the
//      zustand chat store via eventToMessage().
//   3. Persist the chat buffer (via the chat store) keyed by
//      executionID.
//   4. Call refineIdea on chat send via the generated mutation hook.
//   5. Compose the shell: ChatPanel on the right, PreviewPane /
//      CodePane / FilesPane / DashboardPane in the center stage,
//      PatchesPane / logs / changed-files in the bottom dock.

import { Box, Stack, Typography } from "@mui/material";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { LoadingPanel } from "../../../src/components/cockpit/LoadingPanel";
import { ChatPanel } from "../../../src/components/studio/ChatPanel";
import { CodeModeSwitcher } from "../../../src/components/studio/CodeModeSwitcher";
import { MobilePreviewFrame } from "../../../src/components/studio/MobilePreviewFrame";
import { PreviewPane } from "../../../src/components/studio/PreviewPane";
import { StudioErrorPanel } from "../../../src/components/studio/StudioErrorPanel";
import { WorkbenchShell } from "../../../src/components/studio/WorkbenchShell";
import {
  eventToMessage,
  makeAssistantThinkingMessage,
  makeErrorMessage,
  makeUserMessage,
} from "../../../src/components/studio/eventToMessage";
import type { StudioStatusBucket } from "../../../src/components/studio/SuggestionsRow";
import { useWorkbenchLayout } from "../../../src/components/studio/useWorkbenchLayout";
import { getToken, RequireAuth, useAuth } from "../../../src/lib/auth";
import { streamChat, type StreamChatHandle } from "../../../src/lib/chat/stream";
import { extractErrorMessage } from "../../../src/lib/errors";
import {
  useExecutionFeedSubscription,
  useExecutionQuery,
  useProjectExecutionsQuery,
  useProjectQuery,
  useRefineIdeaMutation,
  type ExecutionCoreFragment,
} from "../../../src/lib/gql/__generated__";
import {
  selectMessages,
  useChatStore,
} from "../../../src/lib/stores/chatStore";
import { tokens } from "../../../src/theme";
import type { StudioAttachment } from "../../../src/components/studio/types";

// Heavy panes lazy-load. They share the same fallback so the shell
// keeps a consistent loading skin while their JS chunk lands.
const paneFallback = <LoadingPanel label="Loading pane" minHeight="100%" />;
const CodePane = dynamic(
  () => import("../../../src/components/studio/CodePane").then((m) => m.CodePane),
  { ssr: false, loading: () => paneFallback },
);
const DashboardPane = dynamic(
  () =>
    import("../../../src/components/studio/DashboardPane").then(
      (m) => m.DashboardPane,
    ),
  { ssr: false, loading: () => paneFallback },
);
const FilesPane = dynamic(
  () => import("../../../src/components/studio/FilesPane").then((m) => m.FilesPane),
  { ssr: false, loading: () => paneFallback },
);

const TERMINAL_STATUSES = new Set([
  "succeeded",
  "failed",
  "stopped",
  "killed",
  "refunded",
]);

function statusBucket(status: string | undefined | null): StudioStatusBucket {
  switch (status) {
    case "running":
    case "scoring":
    case "admitted":
    case "created":
      return "running";
    case "succeeded":
      return "succeeded";
    case "failed":
    case "killed":
      return "failed";
    default:
      return "idle";
  }
}

function userInitialsFrom(
  name: string | null | undefined,
  email: string | null | undefined,
): string {
  if (name && name.trim()) return name;
  if (email) return email.split("@")[0];
  return "you";
}

export default function ProjectStudioPage() {
  return (
    <RequireAuth>
      <ProjectStudioInner />
    </RequireAuth>
  );
}

function ProjectStudioInner() {
  const params = useParams<{ projectID: string }>();
  const projectID = params?.projectID ?? "";
  const search = useSearchParams();
  const { user } = useAuth();
  const executionIDParam =
    search?.get("executionID") || search?.get("execution") || "";

  const layout = useWorkbenchLayout(projectID);

  // 1. Resolve projectID → executionID.
  const projectExecutionsQuery = useProjectExecutionsQuery({
    variables: { projectId: projectID, limit: 5 },
    skip: !projectID || !!executionIDParam,
    fetchPolicy: "cache-and-network",
  });
  const projectQuery = useProjectQuery({
    variables: { id: projectID },
    skip: !projectID,
    fetchPolicy: "cache-and-network",
  });

  const resolvedExecution: ExecutionCoreFragment | null = useMemo(() => {
    if (!projectID) return null;
    const rows = projectExecutionsQuery.data?.projectExecutions ?? [];
    if (rows.length === 0) return null;
    return rows[0];
  }, [projectExecutionsQuery.data, projectID]);

  const executionLookupID = executionIDParam || resolvedExecution?.id || "";
  const shouldPollExecution =
    !!executionLookupID &&
    (!resolvedExecution || !TERMINAL_STATUSES.has(resolvedExecution.status));
  const executionQuery = useExecutionQuery({
    variables: { id: executionLookupID },
    skip: !executionLookupID,
    fetchPolicy: "cache-and-network",
    pollInterval: shouldPollExecution ? 4000 : 0,
  });

  const execution: ExecutionCoreFragment | null =
    executionQuery.data?.execution ?? (executionIDParam ? null : resolvedExecution);
  const executionID = execution?.id ?? "";
  // chatStoreKey routes both modes through the same chat store: when
  // an execution exists, messages key off its id; otherwise the free-
  // chat sentinel "_" matches the orchestrator's chat_stream path.
  const chatStoreKey = executionID || "_";

  // 2. Chat buffer.
  const messages = useChatStore(selectMessages(chatStoreKey));
  const hydrate = useChatStore((s) => s.hydrate);
  const appendIncoming = useChatStore((s) => s.appendIncoming);
  const appendLocal = useChatStore((s) => s.appendLocal);
  const updateLocal = useChatStore((s) => s.updateLocal);

  // Streaming chat handle — exposed so the user can hit Stop mid-reply
  // (cancels the upstream provider call) and so unmounting cleans up.
  const streamRef = useRef<StreamChatHandle | null>(null);
  const [streaming, setStreaming] = useState(false);
  useEffect(() => {
    return () => {
      streamRef.current?.abort();
      streamRef.current = null;
    };
  }, []);

  useEffect(() => {
    hydrate(chatStoreKey);
  }, [chatStoreKey, hydrate]);

  // 3. Subscribe to executionFeed.
  const sub = useExecutionFeedSubscription({
    variables: { id: executionID },
    skip: !executionID,
  });
  useEffect(() => {
    const ev = sub.data?.executionFeed;
    if (!ev) return;
    const msg = eventToMessage({
      executionID: ev.executionID,
      eventType: ev.eventType,
      payload: ev.payload,
      createdAt: ev.createdAt,
    });
    if (!msg) return;
    appendIncoming(executionID, msg);
  }, [sub.data, executionID, appendIncoming]);

  // 4. Chat send: (a) optimistic user echo, (b) record the refinement
  // on GraphQL so the finisher loop folds it into the next iteration,
  // (c) open the dedicated SSE chat stream for the assistant's reply.
  //    The SSE endpoint owns RAW assistant token deltas; orchestration
  //    events (gate verdicts, cost ticks) continue to flow via the
  //    executionFeed subscription wired above.
  const [refineIdea, refineState] = useRefineIdeaMutation();
  const onSend = useCallback(
    async (text: string, attachments?: StudioAttachment[]) => {
      // Free-chat sentinel: when no execution has been admitted yet,
      // the orchestrator's /executions/_/chat/stream path runs a
      // general copilot reply without execution context. The chat
      // store keys off the same id so messages stay coherent until
      // an execution arrives.
      const chatID = executionID || "_";
      appendLocal(chatID, makeUserMessage(text, attachments));

      // Record the refinement on the running execution. Skipped in
      // free-chat mode — no execution exists to refine yet. Fire-and-
      // forget by design when sent; failure surfaces inline but does
      // not block the assistant reply stream below.
      if (executionID) {
        try {
          await refineIdea({ variables: { executionID, message: text } });
        } catch (e) {
          appendLocal(chatID, makeErrorMessage(extractErrorMessage(e)));
        }
      }

      // Cancel any in-flight stream before opening a new one.
      streamRef.current?.abort();

      const token = getToken();
      if (!token) {
        appendLocal(
          chatID,
          makeErrorMessage("Session expired — please sign in again."),
        );
        return;
      }

      const assistant = makeAssistantThinkingMessage("");
      appendLocal(chatID, assistant);

      let buffer = "";
      setStreaming(true);
      const handle = streamChat(
        { executionID: chatID, message: text, token },
        (ev) => {
          switch (ev.type) {
            case "delta": {
              const chunk =
                typeof ev.data.text === "string" ? ev.data.text : "";
              if (!chunk) return;
              buffer += chunk;
              updateLocal(chatID, assistant.id, { body: buffer });
              break;
            }
            case "finish": {
              updateLocal(chatID, assistant.id, {
                body: buffer || "(no response)",
              });
              break;
            }
            case "error": {
              const msg =
                typeof ev.data.message === "string"
                  ? ev.data.message
                  : "Stream error";
              appendLocal(chatID, makeErrorMessage(msg));
              break;
            }
            // tool_call / tool_result / thinking — surfaced via the
            // dashboard panes for now; the chat buffer keeps only the
            // visible assistant body.
            default:
              break;
          }
        },
      );
      streamRef.current = handle;
      handle.done.finally(() => {
        if (streamRef.current === handle) streamRef.current = null;
        setStreaming(false);
      });
    },
    [executionID, refineIdea, appendLocal, updateLocal],
  );

  const onStop = useCallback(() => {
    streamRef.current?.abort();
    streamRef.current = null;
    setStreaming(false);
  }, []);

  // Derive a one-line last-patch headline for the header status strip.
  const lastPatchSummary = useMemo(() => {
    const patchMsg = messages
      .filter((m) => m.role === "agent_result" || m.role === "system")
      .slice(-1)[0];
    if (!patchMsg) return null;
    return (patchMsg.summary?.trim() || patchMsg.body || "").slice(0, 120);
  }, [messages]);

  if (!projectID) {
    return (
      <Box sx={{ height: "100%", width: "100%", display: "flex", p: 2 }}>
        <StudioErrorPanel
          title="Missing project ID"
          message="The route is malformed — return home and pick a project from the list."
        />
      </Box>
    );
  }

  if (
    (executionIDParam
      ? executionQuery.loading
      : projectExecutionsQuery.loading) &&
    !execution
  ) {
    return (
      <Box sx={{ height: "100%", width: "100%" }}>
        <LoadingPanel label="Loading project…" minHeight="100%" />
      </Box>
    );
  }

  const projectName =
    projectQuery.data?.project?.name ??
    execution?.promptSummary ??
    `Project ${projectID.slice(0, 6)}`;
  const initials = userInitialsFrom(user?.name ?? null, user?.email ?? null);
  const bucket = statusBucket(execution?.status);
  const workspaceID = execution?.workspaceID ?? execution?.id ?? "";

  if (!execution) {
    return (
      <WorkbenchShell
        projectName={projectName}
        projectID={projectID}
        execution={null}
        messages={[]}
        primary={layout.primary}
        leftOpen={layout.leftOpen}
        rightOpen={layout.rightOpen}
        dockOpen={layout.dockOpen}
        dockTab={layout.dockTab}
        dockHeight={layout.dockHeight}
        focus={layout.focus}
        setPrimary={layout.setPrimary}
        toggleLeft={layout.toggleLeft}
        toggleRight={layout.toggleRight}
        toggleDock={layout.toggleDock}
        toggleFocus={layout.toggleFocus}
        setDockTab={layout.setDockTab}
        setDockHeight={layout.setDockHeight}
        previewSlot={<NoExecutionPlaceholder />}
        mobileSlot={<NoExecutionPlaceholder />}
        codeSlot={<NoExecutionPlaceholder />}
        filesSlot={<NoExecutionPlaceholder />}
        dashboardSlot={<NoExecutionPlaceholder />}
        chatSlot={
          <ChatPanel
            messages={messages}
            status="idle"
            pending={streaming || refineState.loading}
            onSend={onSend}
            onStop={onStop}
            userInitials={initials}
            contextBar={
              <PromptContextBar
                projectID={projectID}
                execution={null}
                prompt={projectQuery.data?.project?.idea ?? null}
              />
            }
          />
        }
      />
    );
  }

  return (
    <WorkbenchShell
      projectName={projectName}
      projectID={projectID}
      execution={execution}
      messages={messages}
      primary={layout.primary}
      leftOpen={layout.leftOpen}
      rightOpen={layout.rightOpen}
      dockOpen={layout.dockOpen}
      dockTab={layout.dockTab}
      dockHeight={layout.dockHeight}
      focus={layout.focus}
      setPrimary={layout.setPrimary}
      toggleLeft={layout.toggleLeft}
      toggleRight={layout.toggleRight}
      toggleDock={layout.toggleDock}
      toggleFocus={layout.toggleFocus}
      setDockTab={layout.setDockTab}
      setDockHeight={layout.setDockHeight}
      lastPatchSummary={lastPatchSummary}
      previewSlot={
        <PreviewPane
          executionID={execution.id}
          executionStatus={execution.status}
        />
      }
      mobileSlot={
        <MobilePreviewFrame>
          <PreviewPane
            executionID={execution.id}
            executionStatus={execution.status}
          />
        </MobilePreviewFrame>
      }
      codeSlot={
        <CodeModeSwitcher
          mode={layout.codeMode}
          onModeChange={layout.setCodeMode}
          projectID={projectID}
          monacoSlot={
            <CodePane
              projectID={projectID}
              executionID={execution.id}
              executionStatus={execution.status}
            />
          }
        />
      }
      filesSlot={
        <FilesPane
          executionID={execution.id}
          executionStatus={execution.status}
          workspaceID={workspaceID}
        />
      }
      dashboardSlot={
        <DashboardPane
          projectID={projectID}
          execution={execution}
          messages={messages}
          leftRailOpen={layout.leftOpen}
          chatOpen={layout.rightOpen}
          dockOpen={layout.dockOpen}
          onToggleLeftRail={layout.toggleLeft}
          onToggleChat={layout.toggleRight}
          onToggleDock={layout.toggleDock}
          onRequestAreaClose={async (message: string) => {
            await onSend(`Finisher area close request: ${message}`);
            if (!layout.rightOpen) {
              layout.toggleRight();
            }
          }}
        />
      }
      chatSlot={
        <ChatPanel
          messages={messages}
          status={bucket}
          pending={streaming || refineState.loading}
          onSend={onSend}
          onStop={onStop}
          userInitials={initials}
          contextBar={
            <PromptContextBar
              projectID={projectID}
              execution={execution}
              prompt={
                execution.promptSummary ??
                projectQuery.data?.project?.idea ??
                null
              }
            />
          }
        />
      }
    />
  );
}

function PromptContextBar({
  projectID,
  execution,
  prompt,
}: {
  projectID: string;
  execution: ExecutionCoreFragment | null;
  prompt: string | null;
}) {
  const shortProject = projectID ? projectID.slice(0, 8) : "project";
  const shortExecution = execution?.id ? execution.id.slice(0, 8) : "no-run";
  const workspace = execution?.workspaceID
    ? execution.workspaceID.slice(0, 8)
    : shortExecution;
  const promptText = prompt?.trim() || "No prompt captured yet";

  return (
    <Box
      sx={{
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: tokens.color.bg.inset,
        px: 1.5,
        py: 0.9,
      }}
    >
      <Stack direction="row" spacing={0.75} sx={{ alignItems: "center", minWidth: 0 }}>
        <Box
          sx={{
            bgcolor: `${tokens.color.accent.purple}22`,
            border: `1px solid ${tokens.color.border.accent}`,
            borderRadius: 0.75,
            color: tokens.color.accent.violet,
            flex: "0 0 auto",
            fontFamily: tokens.font.mono,
            fontSize: 10,
            fontWeight: 800,
            letterSpacing: 0.7,
            px: 0.75,
            py: 0.25,
            textTransform: "uppercase",
          }}
        >
          prompt
        </Box>
        <Typography
          sx={{
            color: tokens.color.text.primary,
            flex: 1,
            fontSize: 12.5,
            lineHeight: 1.35,
            minWidth: 0,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
          title={promptText}
        >
          {promptText}
        </Typography>
      </Stack>
      <Stack
        direction="row"
        spacing={1.2}
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 10,
          letterSpacing: 0.45,
          mt: 0.5,
          overflow: "hidden",
          textTransform: "uppercase",
          whiteSpace: "nowrap",
        }}
      >
        <Box component="span">project {shortProject}</Box>
        <Box component="span">workspace {workspace}</Box>
        <Box component="span">{execution ? "files mirrored" : "files pending"}</Box>
      </Stack>
    </Box>
  );
}

function NoExecutionPlaceholder() {
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={1.5}
      sx={{
        color: tokens.color.text.muted,
        flex: 1,
        height: "100%",
        textAlign: "center",
        p: 4,
      }}
    >
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontSize: 16,
          fontWeight: 800,
        }}
      >
        Start a build
      </Typography>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontSize: 13,
          maxWidth: 420,
        }}
      >
        This project has no execution yet. Open Studio and describe what you
        want so Ironflyer can create the first execution.
      </Typography>
      <Box
        component={Link}
        href="/studio"
        sx={{
          color: tokens.color.accent.violet,
          fontWeight: 800,
          textDecoration: "none",
        }}
      >
        Open Studio →
      </Box>
    </Stack>
  );
}
