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
//      IDEFramePane / FilesPane / DashboardPane in the center stage,
//      PatchesPane / logs / changed-files in the bottom dock.

import { Box, Button, Stack, Typography } from "@mui/material";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { LoadingPanel } from "../../../src/components/cockpit/LoadingPanel";
import { ChatPanel } from "../../../src/components/studio/ChatPanel";
import { MobilePreviewFrame } from "../../../src/components/studio/MobilePreviewFrame";
import { PreviewPane } from "../../../src/components/studio/PreviewPane";
import { StudioErrorPanel } from "../../../src/components/studio/StudioErrorPanel";
import { IDEFramePane } from "../../../src/components/studio/IDEFramePane";
import { WorkbenchShell } from "../../../src/components/studio/WorkbenchShell";
import {
  eventToMessage,
  makeAssistantThinkingMessage,
  makeErrorMessage,
  makeUserMessage,
} from "../../../src/components/studio/eventToMessage";
import type { StudioStatusBucket } from "../../../src/components/studio/SuggestionsRow";
import { useWorkbenchLayout } from "../../../src/components/studio/useWorkbenchLayout";
import { getToken, useAuth } from "../../../src/lib/auth";
import {
  streamChat,
  type StreamChatHandle,
} from "../../../src/lib/chat/stream";
import { extractErrorMessage, normalizeError } from "../../../src/lib/errors";
import {
  useCreatePaidExecutionMutation,
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
import { pushToast } from "../../../src/lib/stores/uiStore";
import { tokens } from "../../../src/theme";
import type { StudioAttachment } from "../../../src/components/studio/types";

// Heavy panes lazy-load. They share the same fallback so the shell
// keeps a consistent loading skin while their JS chunk lands.
const paneFallback = <LoadingPanel label="Loading pane" minHeight="100%" />;
const DashboardPane = dynamic(
  () =>
    import("../../../src/components/studio/DashboardPane").then(
      (m) => m.DashboardPane,
    ),
  { ssr: false, loading: () => paneFallback },
);
const FilesPane = dynamic(
  () =>
    import("../../../src/components/studio/FilesPane").then((m) => m.FilesPane),
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
    <Suspense fallback={paneFallback}>
      <ProjectStudioInner />
    </Suspense>
  );
}

function ProjectStudioInner() {
  const params = useParams<{ projectID: string }>();
  const projectID = params?.projectID ?? "";
  const search = useSearchParams();
  const { user } = useAuth();
  const executionIDParam =
    search?.get("executionID") || search?.get("execution") || "";
  const demoPreview = projectID === "demo" && !executionIDParam;

  const layout = useWorkbenchLayout(projectID);

  // 1. Resolve projectID → executionID.
  const projectExecutionsQuery = useProjectExecutionsQuery({
    variables: { projectId: projectID, limit: 5 },
    skip: !projectID || !!executionIDParam || demoPreview,
    fetchPolicy: "cache-and-network",
  });
  const projectQuery = useProjectQuery({
    variables: { id: projectID },
    skip: !projectID || demoPreview,
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
    skip: !executionLookupID || demoPreview,
    fetchPolicy: "cache-and-network",
    pollInterval: shouldPollExecution ? 4000 : 0,
  });

  const execution: ExecutionCoreFragment | null =
    executionQuery.data?.execution ??
    (executionIDParam ? null : resolvedExecution);
  const executionID = execution?.id ?? "";
  // chatStoreKey routes both modes through the same chat store. Before
  // an execution exists, keep the draft chat scoped to the project so
  // local messages never leak across projects.
  const chatStoreKey = executionID || `project:${projectID}:draft`;

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
    skip: !executionID || demoPreview,
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
      const chatID = chatStoreKey;
      appendLocal(chatID, makeUserMessage(text, attachments));

      // No execution yet: keep chat local and require the explicit
      // budget-confirming StartExecutionPanel before any wallet hold.
      if (!executionID && projectID) {
        appendLocal(
          chatID,
          makeAssistantThinkingMessage(
            "Choose a budget in Preview, then start the execution. I will not place a wallet hold from chat alone.",
          ),
        );
        pushToast({
          message:
            "Confirm a budget in Preview before starting a paid execution.",
          severity: "info",
        });
        return;
      }

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
    [
      executionID,
      projectID,
      chatStoreKey,
      refineIdea,
      appendLocal,
      updateLocal,
    ],
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
    demoPreview
      ? "IronFlyer Studio"
      : projectQuery.data?.project?.name ??
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
        previewSlot={
          <StartExecutionPanel
            projectID={projectID}
            seedIdea={projectQuery.data?.project?.idea ?? null}
            onStarted={() => {
              void projectExecutionsQuery.refetch();
            }}
          />
        }
        mobileSlot={
          <StartExecutionPanel
            projectID={projectID}
            seedIdea={projectQuery.data?.project?.idea ?? null}
            onStarted={() => {
              void projectExecutionsQuery.refetch();
            }}
          />
        }
        codeSlot={<IDEFramePane projectID={projectID} />}
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
        <IDEFramePane
          projectID={projectID}
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
      <Stack
        direction="row"
        spacing={0.75}
        sx={{ alignItems: "center", minWidth: 0 }}
      >
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
        <Box component="span">
          {execution ? "files mirrored" : "files pending"}
        </Box>
      </Stack>
    </Box>
  );
}

function NoExecutionPlaceholder() {
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={1}
      sx={{
        color: tokens.color.text.muted,
        flex: 1,
        height: "100%",
        textAlign: "center",
        p: 4,
      }}
    >
      <Typography
        sx={{ color: tokens.color.text.secondary, fontSize: 13, maxWidth: 360 }}
      >
        Start the first execution from the Preview tab to populate this view.
      </Typography>
    </Stack>
  );
}

// StartExecutionPanel — replaces the dead-end "Open Studio" link for
// projects that have no execution yet. The user can describe the build
// (pre-seeded from `project.idea`) and set a wallet hold, and we call
// `createPaidExecution` directly. On success we refetch
// projectExecutions so the page swaps into the full Studio layout
// without a hard reload.
function StartExecutionPanel({
  projectID,
  seedIdea,
  onStarted,
}: {
  projectID: string;
  seedIdea: string | null;
  onStarted: () => void;
}) {
  const [prompt, setPrompt] = useState<string>(seedIdea?.trim() ?? "");
  const [budget, setBudget] = useState<string>("5");
  const [error, setError] = useState<string | null>(null);
  const [createExec, { loading }] = useCreatePaidExecutionMutation();

  const canSubmit = prompt.trim().length > 0 && Number(budget) > 0 && !loading;

  const handleStart = async () => {
    if (!canSubmit) return;
    setError(null);
    try {
      const budgetUSD = Number(budget);
      const res = await createExec({
        variables: {
          input: {
            projectID,
            budgetUSD,
            stopLossUSD: budgetUSD,
            promptSummary: prompt.trim().slice(0, 240),
            metadata: { source: "studio.start_execution" },
          },
        },
        refetchQueries: ["ProjectExecutions"],
      });
      if (!res.data?.createPaidExecution?.id) {
        throw new Error("Orchestrator did not return an execution id.");
      }
      onStarted();
    } catch (e) {
      const n = normalizeError(e);
      if (
        n.code === "INSUFFICIENT_FUNDS" ||
        n.code === "PAYMENT_REQUIRED" ||
        n.status === 402
      ) {
        pushToast({
          message: "Wallet too low — top up to start this execution.",
          severity: "warning",
          href: "/wallet",
          actionLabel: "Top up",
        });
        setError("Wallet has insufficient balance for this hold.");
        return;
      }
      setError(extractErrorMessage(e));
    }
  };

  return (
    <Stack
      spacing={2}
      sx={{
        color: tokens.color.text.primary,
        height: "100%",
        justifyContent: "center",
        maxWidth: 560,
        mx: "auto",
        p: { xs: 3, md: 5 },
        width: "100%",
      }}
    >
      <Box>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.accent.violet, letterSpacing: 1.2 }}
        >
          First execution
        </Typography>
        <Typography
          sx={{ mt: 0.5, fontSize: 22, fontWeight: 800, lineHeight: 1.2 }}
        >
          Start the build for this project
        </Typography>
        <Typography
          sx={{
            mt: 1,
            fontSize: 13.5,
            color: tokens.color.text.secondary,
            lineHeight: 1.55,
          }}
        >
          Describe what you want and set a wallet hold. The finisher plans,
          generates, gates and previews — every dollar is debited from the hold
          as cost materialises; unused funds release on commit.
        </Typography>
      </Box>

      <Box
        component="textarea"
        value={prompt}
        onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
          setPrompt(e.target.value)
        }
        placeholder="Build a client operations portal with projects, invoices, role-based access…"
        rows={5}
        sx={{
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: `${tokens.radius.sm}px`,
          color: tokens.color.text.primary,
          fontFamily: tokens.font.family,
          fontSize: 14,
          lineHeight: 1.55,
          outline: "none",
          p: 1.5,
          resize: "vertical",
          width: "100%",
          "&:focus": { borderColor: tokens.color.accent.violet },
          "&::placeholder": { color: tokens.color.text.muted },
        }}
      />

      <Stack
        direction={{ xs: "column", sm: "row" }}
        spacing={1.5}
        alignItems={{ sm: "flex-end" }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            variant="overline"
            sx={{
              color: tokens.color.text.muted,
              letterSpacing: 1.1,
              fontSize: 10.5,
            }}
          >
            Wallet hold (USD)
          </Typography>
          <Box
            component="input"
            type="number"
            min={1}
            step={1}
            value={budget}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setBudget(e.target.value)
            }
            sx={{
              bgcolor: tokens.color.bg.inset,
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: `${tokens.radius.sm}px`,
              color: tokens.color.text.primary,
              fontFamily: tokens.font.mono,
              fontSize: 16,
              fontWeight: 700,
              mt: 0.5,
              outline: "none",
              px: 1.5,
              py: 1,
              width: "100%",
              "&:focus": { borderColor: tokens.color.accent.violet },
            }}
          />
        </Box>
        <Button
          variant="contained"
          color="primary"
          disabled={!canSubmit}
          onClick={() => void handleStart()}
          sx={{ minWidth: { xs: "100%", sm: 180 }, minHeight: 44 }}
        >
          {loading ? "Starting…" : "Start execution"}
        </Button>
      </Stack>

      {error && (
        <Typography sx={{ color: tokens.color.accent.danger, fontSize: 13 }}>
          {error}
        </Typography>
      )}
    </Stack>
  );
}
