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
import { useCallback, useEffect, useMemo } from "react";
import { LoadingPanel } from "../../../src/components/cockpit/LoadingPanel";
import { ChatPanel } from "../../../src/components/studio/ChatPanel";
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
import { RequireAuth, useAuth } from "../../../src/lib/auth";
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

  // 2. Chat buffer.
  const messages = useChatStore(selectMessages(executionID));
  const hydrate = useChatStore((s) => s.hydrate);
  const appendIncoming = useChatStore((s) => s.appendIncoming);
  const appendLocal = useChatStore((s) => s.appendLocal);

  useEffect(() => {
    hydrate(executionID);
  }, [executionID, hydrate]);

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

  // 4. refineIdea on chat send.
  const [refineIdea, refineState] = useRefineIdeaMutation();
  const onSend = useCallback(
    async (text: string) => {
      if (!executionID) {
        appendLocal(
          executionID,
          makeErrorMessage(
            "No active execution yet — wait for the orchestrator to admit one.",
          ),
        );
        return;
      }
      appendLocal(executionID, makeUserMessage(text));
      try {
        await refineIdea({
          variables: { executionID, message: text },
        });
        appendLocal(
          executionID,
          makeAssistantThinkingMessage(
            "Refinement queued. Streaming gate verdicts as they land.",
            "Thought for less than a second",
          ),
        );
      } catch (e) {
        appendLocal(executionID, makeErrorMessage(extractErrorMessage(e)));
      }
    },
    [executionID, refineIdea, appendLocal],
  );

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
        codeSlot={<NoExecutionPlaceholder />}
        filesSlot={<NoExecutionPlaceholder />}
        dashboardSlot={<NoExecutionPlaceholder />}
        chatSlot={
          <ChatPanel
            messages={messages}
            status="idle"
            pending={false}
            onSend={onSend}
            userInitials={initials}
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
      codeSlot={
        <CodePane
          projectID={projectID}
          executionID={execution.id}
          executionStatus={execution.status}
        />
      }
      filesSlot={
        <FilesPane
          executionID={execution.id}
          executionStatus={execution.status}
          workspaceID={workspaceID}
        />
      }
      dashboardSlot={<DashboardPane execution={execution} messages={messages} />}
      chatSlot={
        <ChatPanel
          messages={messages}
          status={bucket}
          pending={refineState.loading}
          onSend={onSend}
          userInitials={initials}
        />
      }
    />
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
