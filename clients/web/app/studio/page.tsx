"use client";

import {
  AddRounded,
  AccountTreeRounded,
  AutoAwesomeRounded,
  BoltRounded,
  CheckRounded,
  ChevronRightRounded,
  CodeRounded,
  DashboardRounded,
  FolderRounded,
  GitHub,
  LaptopMacRounded,
  MoreHorizRounded,
  PhoneAndroidRounded,
  PlayArrowRounded,
  RocketLaunchRounded,
  SearchRounded,
  SendRounded,
  ShieldRounded,
  StopRounded,
  TerminalRounded,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Chip,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useRef, useState, type ReactNode } from "react";
import { BrandLogo } from "../../src/components/BrandLogo";
import { useAuth } from "../../src/lib/auth";
import { extractErrorMessage } from "../../src/lib/errors";
import {
  useDescribeIdeaMutation,
  useProjectsQuery,
} from "../../src/lib/gql/__generated__";
import * as swal from "../../src/lib/swal";
import { tokens } from "../../../../packages/design-tokens";

// /studio is the unauthenticated-friendly entry into the workbench.
// On submit it behaves like the home composer: it calls describeIdea
// to create a real project + execution, then redirects to the canonical
// per-project workspace at /p/{projectID}. The local in-page demo
// (steps, messages, sectionCopy) is kept so visitors who haven't sent
// yet still see a populated, real-looking studio.
const PENDING_PROMPT_KEY = "ironflyer.pendingPrompt.v1";

type StageView = "browser" | "ide" | "android";
type StudioSection =
  | "command"
  | "prompt"
  | "browser"
  | "ide"
  | "android"
  | "graph"
  | "deploy";
type BuildStepState = "done" | "running" | "queued";
type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  body: string;
};

const NAV = [
  { id: "command", label: "Command", icon: DashboardRounded },
  { id: "prompt", label: "Prompt", icon: AutoAwesomeRounded },
  { id: "browser", label: "Browser", icon: LaptopMacRounded },
  { id: "ide", label: "IDE", icon: CodeRounded },
  { id: "android", label: "Android", icon: PhoneAndroidRounded },
  { id: "graph", label: "Graph", icon: AccountTreeRounded },
  { id: "deploy", label: "Deploy", icon: RocketLaunchRounded },
] as const;

const SECTION_COPY: Record<
  StudioSection,
  { title: string; eyebrow: string; detail: string }
> = {
  command: {
    title: "Production workspace",
    eyebrow: "Studio / ClientFlow",
    detail: "One command surface for prompt, code, preview and deploy state.",
  },
  prompt: {
    title: "Prompt control",
    eyebrow: "Studio / Prompt",
    detail: "Shape the request, lock requirements and run the next pass.",
  },
  browser: {
    title: "Browser preview",
    eyebrow: "Studio / Browser",
    detail: "Inspect the generated web app exactly where users will use it.",
  },
  ide: {
    title: "Code workspace",
    eyebrow: "Studio / IDE",
    detail: "Review generated files, diffs, tests and implementation quality.",
  },
  android: {
    title: "Android lab",
    eyebrow: "Studio / Android",
    detail: "Validate the mobile experience inside a native device frame.",
  },
  graph: {
    title: "Build graph",
    eyebrow: "Studio / Graph",
    detail:
      "Track what was created, what is running and what still blocks ship.",
  },
  deploy: {
    title: "Deploy lane",
    eyebrow: "Studio / Deploy",
    detail: "Control gates, environments, release notes and publish readiness.",
  },
};

const FILES = [
  "src/app/dashboard/page.tsx",
  "src/components/StatsGrid.tsx",
  "src/components/ApprovalTable.tsx",
  "src/lib/roles.ts",
  "src/lib/billing.ts",
  "package.json",
];

const CODE_LINES = [
  "import { Badge, Card, Table } from '@/ui'",
  "import { useApprovals, useRevenue } from '@/hooks/client-flow'",
  "",
  "export default function Dashboard() {",
  "  const revenue = useRevenue()",
  "  const approvals = useApprovals()",
  "",
  "  return (",
  '    <main className="space-y-6">',
  "      <StatsGrid revenue={revenue} approvals={approvals} />",
  "      <ApprovalTable rows={approvals.pending} />",
  '      <DeployGate status="preview" score={92} />',
  "    </main>",
  "  )",
  "}",
];

const INITIAL_STEPS: Array<{
  id: string;
  label: string;
  detail: string;
  state: BuildStepState;
}> = [
  {
    id: "model",
    label: "Data model",
    detail: "Clients, projects, invoices and approvals",
    state: "done",
  },
  {
    id: "roles",
    label: "Roles",
    detail: "Owner, manager and reviewer access",
    state: "done",
  },
  {
    id: "ui",
    label: "Responsive UI",
    detail: "Browser shell and Android layout",
    state: "running",
  },
  {
    id: "deploy",
    label: "Deploy lane",
    detail: "Preview checks, gate score and publish plan",
    state: "queued",
  },
];

export default function StudioPage() {
  const [activeSection, setActiveSection] = useState<StudioSection>("command");
  const [stageView, setStageView] = useState<StageView>("browser");
  const [prompt, setPrompt] = useState(
    "Build a client operations portal with projects, invoices, approvals, role-based access and team activity.",
  );
  const [steps, setSteps] = useState(INITIAL_STEPS);
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      id: "m0",
      role: "assistant",
      body: "Ready. Ask for a UI change, a code patch, or an Android pass.",
    },
  ]);
  const [chatPending, setChatPending] = useState(false);
  const router = useRouter();
  const { authenticated } = useAuth();
  const [describeIdea] = useDescribeIdeaMutation();

  const progress = useMemo(() => {
    const done = steps.filter((step) => step.state === "done").length;
    return Math.round((done / steps.length) * 100);
  }, [steps]);

  const runBuildPass = () => {
    setSteps((current) =>
      current.map((step) =>
        step.state === "running"
          ? { ...step, state: "done" }
          : step.state === "queued"
            ? { ...step, state: "running" }
            : step,
      ),
    );
    setMessages((current) => [
      ...current,
      {
        id: `a-${Date.now()}`,
        role: "assistant",
        body: "Build pass completed. Android rendering is now in review and the deploy gate is still visible.",
      },
    ]);
    setActiveSection("android");
    setStageView("android");
  };

  const openSection = (section: StudioSection) => {
    setActiveSection(section);
    if (section === "browser" || section === "ide" || section === "android") {
      setStageView(section);
    }
  };

  const openStageView = (view: StageView) => {
    setStageView(view);
    setActiveSection(view);
  };

  const syncStudioIntent = (message: string) => {
    const lower = message.toLowerCase();
    if (lower.includes("android") || lower.includes("mobile")) {
      setActiveSection("android");
      setStageView("android");
    } else if (
      lower.includes("code") ||
      lower.includes("ide") ||
      lower.includes("file")
    ) {
      setActiveSection("ide");
      setStageView("ide");
    } else if (lower.includes("browser") || lower.includes("preview")) {
      setActiveSection("browser");
      setStageView("browser");
    }

    if (
      lower.includes("deploy") ||
      lower.includes("publish") ||
      lower.includes("gate")
    ) {
      setSteps((current) =>
        current.map((step) =>
          step.id === "deploy" ? { ...step, state: "running" } : step,
        ),
      );
      setActiveSection("deploy");
    }
  };

  const sendChat = async (body: string) => {
    const message = body.trim();
    if (!message || chatPending) return;

    syncStudioIntent(message);
    const userId = `u-${Date.now()}`;
    const assistantId = `a-${Date.now()}`;
    setMessages((current) => [
      ...current,
      { id: userId, role: "user", body: message },
      {
        id: assistantId,
        role: "assistant",
        body: "Spinning up your workspace…",
      },
    ]);
    setChatPending(true);

    const fillAssistant = (nextBody: string) => {
      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId ? { ...item, body: nextBody } : item,
        ),
      );
    };

    // Guest? Stash the prompt and route to signup. The /signup page
    // bounces back to /?welcome=1 on success and the home page restores
    // the prompt + composer state.
    if (!authenticated) {
      try {
        window.sessionStorage.setItem(PENDING_PROMPT_KEY, message);
      } catch {
        // ignore quota / privacy errors — the redirect still works.
      }
      fillAssistant(
        "Create an account to launch the build. Redirecting you to sign up…",
      );
      router.push(`/signup?redirect=${encodeURIComponent("/studio")}`);
      return;
    }

    try {
      const result = await describeIdea({
        variables: {
          input: { text: message, startImmediately: true },
        },
      });
      if (result.errors && result.errors.length > 0) {
        throw new Error(
          result.errors.map((e) => e.message).join("\n") ||
            "Studio rejected the request.",
        );
      }
      const project = result.data?.describeIdea?.project;
      const execution = result.data?.describeIdea?.execution;
      if (!project?.id) {
        const debugDump = JSON.stringify(result.data ?? null).slice(0, 240);
        throw new Error(
          `Studio did not return a project id.\nResponse: ${debugDump}`,
        );
      }
      const params = new URLSearchParams({ tab: "preview", autorun: "1" });
      if (execution?.id) params.set("executionID", execution.id);
      fillAssistant("Workspace is ready. Opening your project…");
      router.push(
        `/p/${encodeURIComponent(project.id)}?${params.toString()}`,
      );
    } catch (err) {
      const errorMessage = extractErrorMessage(err);
      fillAssistant(
        `Could not start the build: ${errorMessage}. Top up your wallet or try a different prompt.`,
      );
      const isFunds =
        /payment.required|insufficient|wallet|top.?up|budget/i.test(
          errorMessage,
        );
      if (isFunds) {
        const res = await swal.fire({
          icon: "warning",
          title: "Top up your wallet to launch",
          text: errorMessage,
          showCancelButton: true,
          confirmButtonText: "Open wallet",
          cancelButtonText: "Close",
        });
        if (res.isConfirmed) router.push("/wallet");
      }
      setChatPending(false);
    }
  };

  const stopChat = () => {
    // describeIdea is a single short-lived mutation; cancelling mid-flight
    // is not exposed. We only reset the local "pending" flag so the user
    // can compose a fresh message if the redirect did not happen.
    setChatPending(false);
  };

  const section = SECTION_COPY[activeSection];

  return (
    <Box
      sx={{
        bgcolor: "#070814",
        color: tokens.color.text.primary,
        display: "grid",
        gridTemplateColumns: { xs: "1fr", lg: "252px minmax(0, 1fr)" },
        height: { xs: "auto", lg: "100dvh" },
        minHeight: "100dvh",
        overflow: { xs: "visible", lg: "hidden" },
      }}
    >
      <StudioSidebar
        activeSection={activeSection}
        onSectionChange={openSection}
      />
      <Box
        sx={{
          display: "grid",
          gridTemplateRows: { xs: "auto auto", lg: "64px minmax(0, 1fr)" },
          minWidth: 0,
        }}
      >
        <StudioTopbar progress={progress} section={section} />
        <Box
          sx={{
            display: "grid",
            gap: 1.2,
            gridTemplateColumns: "1fr",
            minHeight: 0,
            minWidth: 0,
            overflow: { xs: "visible", lg: "auto" },
            p: { xs: 1.2, md: 1.4 },
          }}
        >
          <StudioWorkspace
            activeSection={activeSection}
            chatPending={chatPending}
            messages={messages}
            onPromptChange={setPrompt}
            onRunBuildPass={runBuildPass}
            onSendChat={sendChat}
            onStageViewChange={openStageView}
            onStopChat={stopChat}
            prompt={prompt}
            stageView={stageView}
            steps={steps}
          />
        </Box>
      </Box>
    </Box>
  );
}

type StudioWorkspaceProps = {
  activeSection: StudioSection;
  chatPending: boolean;
  messages: ChatMessage[];
  onPromptChange: (value: string) => void;
  onRunBuildPass: () => void;
  onSendChat: (message: string) => void;
  onStageViewChange: (view: StageView) => void;
  onStopChat: () => void;
  prompt: string;
  stageView: StageView;
  steps: Array<{
    id: string;
    label: string;
    detail: string;
    state: BuildStepState;
  }>;
};

function StudioWorkspace({
  activeSection,
  chatPending,
  messages,
  onPromptChange,
  onRunBuildPass,
  onSendChat,
  onStageViewChange,
  onStopChat,
  prompt,
  stageView,
  steps,
}: StudioWorkspaceProps) {
  const chat = (
    <CommandChat
      messages={messages}
      pending={chatPending}
      onSend={onSendChat}
      onStop={onStopChat}
    />
  );

  if (activeSection === "prompt") {
    return (
      <SectionGrid>
        <Stack spacing={1.2} sx={{ minHeight: 0, minWidth: 0 }}>
          <PromptRunway
            prompt={prompt}
            onPromptChange={onPromptChange}
            onRun={onRunBuildPass}
          />
          <PromptRulesPanel />
        </Stack>
        <Stack spacing={1.2} sx={{ minHeight: 0, minWidth: 0 }}>
          {chat}
          <PromptHistoryPanel />
        </Stack>
      </SectionGrid>
    );
  }

  if (activeSection === "browser") {
    return (
      <SectionGrid wideLeft>
        <StagePanel view="browser" onViewChange={onStageViewChange} />
        <Stack spacing={1.2} sx={{ minHeight: 0, minWidth: 0 }}>
          {chat}
          <BrowserInspectorPanel />
        </Stack>
      </SectionGrid>
    );
  }

  if (activeSection === "ide") {
    return (
      <SectionGrid wideLeft>
        <IdePanel />
        <Stack spacing={1.2} sx={{ minHeight: 0, minWidth: 0 }}>
          {chat}
          <IdeReviewPanel />
        </Stack>
      </SectionGrid>
    );
  }

  if (activeSection === "android") {
    return (
      <SectionGrid wideLeft>
        <StagePanel view="android" onViewChange={onStageViewChange} />
        <Stack spacing={1.2} sx={{ minHeight: 0, minWidth: 0 }}>
          {chat}
          <AndroidControlPanel />
        </Stack>
      </SectionGrid>
    );
  }

  if (activeSection === "graph") {
    return (
      <SectionGrid>
        <GraphMapPanel steps={steps} />
        <Stack spacing={1.2} sx={{ minHeight: 0, minWidth: 0 }}>
          {chat}
          <BuildGraph steps={steps} />
        </Stack>
      </SectionGrid>
    );
  }

  if (activeSection === "deploy") {
    return (
      <SectionGrid>
        <DeployControlPanel steps={steps} />
        <Stack spacing={1.2} sx={{ minHeight: 0, minWidth: 0 }}>
          {chat}
          <BuildGraph steps={steps} />
        </Stack>
      </SectionGrid>
    );
  }

  return (
    <SectionGrid>
      <Box
        sx={{
          display: "grid",
          gap: 1.2,
          gridTemplateRows: {
            xs: "auto",
            lg: "auto auto minmax(0, 1fr)",
          },
          minHeight: 0,
          minWidth: 0,
        }}
      >
        <PromptRunway
          prompt={prompt}
          onPromptChange={onPromptChange}
          onRun={onRunBuildPass}
        />
        {chat}
        <IdePanel />
      </Box>
      <Box
        sx={{
          display: "grid",
          gap: 1.2,
          gridTemplateRows: { xs: "auto", lg: "minmax(0, 1fr) auto" },
          minHeight: 0,
          minWidth: 0,
        }}
      >
        <StagePanel view={stageView} onViewChange={onStageViewChange} />
        <BuildGraph steps={steps} />
      </Box>
    </SectionGrid>
  );
}

function SectionGrid({
  children,
  wideLeft = false,
}: {
  children: ReactNode;
  wideLeft?: boolean;
}) {
  return (
    <Box
      sx={{
        display: "grid",
        gap: 1.2,
        gridTemplateColumns: {
          xs: "1fr",
          lg: wideLeft
            ? "minmax(680px, 1.35fr) minmax(320px, 0.65fr)"
            : "minmax(560px, 1.05fr) minmax(380px, 0.95fr)",
        },
        minHeight: 0,
        minWidth: 0,
      }}
    >
      {children}
    </Box>
  );
}

function PromptRulesPanel() {
  return (
    <Panel title="Requirements" eyebrow="Acceptance criteria">
      <Box sx={{ display: "grid", gap: 1 }}>
        {[
          ["User roles", "Owner, manager and reviewer permissions"],
          ["Core flows", "Projects, invoices, approvals and activity"],
          ["Data", "Seeded dashboard metrics with editable records"],
          ["Quality", "Responsive web plus Android preview checks"],
        ].map(([title, detail]) => (
          <InsightRow key={title} title={title} detail={detail} />
        ))}
      </Box>
    </Panel>
  );
}

function PromptHistoryPanel() {
  return (
    <Panel title="Prompt versions" eyebrow="Run memory">
      <Box sx={{ display: "grid", gap: 1 }}>
        {[
          ["v4", "Added approvals table and deploy gate", "current"],
          ["v3", "Tightened Android spacing", "kept"],
          ["v2", "Added billing model and manager role", "kept"],
        ].map(([version, detail, status]) => (
          <InsightRow
            key={version}
            title={version}
            detail={detail}
            meta={status}
          />
        ))}
      </Box>
    </Panel>
  );
}

function BrowserInspectorPanel() {
  return (
    <Panel title="Browser QA" eyebrow="Viewport checks">
      <Box sx={{ display: "grid", gap: 1 }}>
        {[
          ["1440 desktop", "No overflow, hero and dashboard stable", "pass"],
          ["1024 tablet", "Cards collapse to two columns", "pass"],
          ["390 mobile", "Navigation stacks without route jumps", "review"],
          ["Console", "No runtime errors in preview surface", "pass"],
        ].map(([title, detail, meta]) => (
          <InsightRow key={title} title={title} detail={detail} meta={meta} />
        ))}
      </Box>
    </Panel>
  );
}

function IdeReviewPanel() {
  return (
    <Panel title="Code review" eyebrow="Files and checks">
      <Box sx={{ display: "grid", gap: 1 }}>
        {[
          ["Diff", "4 files changed, 178 additions", "+178"],
          ["Types", "TypeScript clean", "pass"],
          ["Tests", "Client flow smoke ready", "queued"],
          ["Risk", "Deploy gate still requires final pass", "medium"],
        ].map(([title, detail, meta]) => (
          <InsightRow key={title} title={title} detail={detail} meta={meta} />
        ))}
      </Box>
    </Panel>
  );
}

function AndroidControlPanel() {
  return (
    <Panel title="Device lab" eyebrow="Native preview">
      <Box sx={{ display: "grid", gap: 1 }}>
        {[
          ["Pixel 8", "390 x 844, Android 15 baseline", "active"],
          ["Safe areas", "Header and CTA avoid system bars", "pass"],
          ["Touch targets", "Primary controls stay above 44px", "pass"],
          ["Keyboard", "Prompt composer scrolls into view", "review"],
        ].map(([title, detail, meta]) => (
          <InsightRow key={title} title={title} detail={detail} meta={meta} />
        ))}
      </Box>
    </Panel>
  );
}

function GraphMapPanel({
  steps,
}: {
  steps: Array<{
    id: string;
    label: string;
    detail: string;
    state: BuildStepState;
  }>;
}) {
  return (
    <Panel title="System map" eyebrow="Generated architecture" fill>
      <Box
        sx={{
          display: "grid",
          gap: 1,
          gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" },
        }}
      >
        {steps.map((step) => (
          <Box
            key={step.id}
            sx={{
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: 1.2,
              bgcolor: "rgba(255,255,255,0.035)",
              minHeight: 118,
              p: 1.3,
            }}
          >
            <Stack direction="row" alignItems="center" spacing={1}>
              <Box
                sx={{
                  bgcolor: `${stepColor(step.state)}22`,
                  borderRadius: 1,
                  height: 34,
                  width: 34,
                  display: "grid",
                  placeItems: "center",
                }}
              >
                <AccountTreeRounded
                  sx={{ color: stepColor(step.state), fontSize: 18 }}
                />
              </Box>
              <Box sx={{ minWidth: 0 }}>
                <Typography sx={{ fontSize: 14, fontWeight: 950 }}>
                  {step.label}
                </Typography>
                <Typography
                  sx={{ color: tokens.color.text.muted, fontSize: 11 }}
                >
                  {step.state}
                </Typography>
              </Box>
            </Stack>
            <Typography
              sx={{ color: tokens.color.text.secondary, fontSize: 12.5, mt: 1 }}
            >
              {step.detail}
            </Typography>
          </Box>
        ))}
      </Box>
    </Panel>
  );
}

function DeployControlPanel({
  steps,
}: {
  steps: Array<{ state: BuildStepState }>;
}) {
  const deployRunning = steps.some((step) => step.state === "running");
  return (
    <Panel
      title="Release control"
      eyebrow="Production lane"
      action={
        <Button
          variant="contained"
          endIcon={<RocketLaunchRounded sx={{ fontSize: 18 }} />}
        >
          Publish
        </Button>
      }
    >
      <Box sx={{ display: "grid", gap: 1 }}>
        {[
          ["Preview URL", "clientflow.ironflyer.app", "live"],
          [
            "Build gate",
            deployRunning ? "Deploy lane is running" : "Waiting for final pass",
            deployRunning ? "running" : "queued",
          ],
          ["Rollback", "Snapshot retained for instant restore", "ready"],
          ["Notes", "Client portal, Android pass, approval flow", "draft"],
        ].map(([title, detail, meta]) => (
          <InsightRow key={title} title={title} detail={detail} meta={meta} />
        ))}
      </Box>
    </Panel>
  );
}

function InsightRow({
  title,
  detail,
  meta,
}: {
  title: string;
  detail: string;
  meta?: string;
}) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={1}
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1.2,
        bgcolor: "rgba(255,255,255,0.035)",
        px: 1,
        py: 0.9,
      }}
    >
      <CheckRounded sx={{ color: tokens.color.accent.success, fontSize: 17 }} />
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography sx={{ fontSize: 13, fontWeight: 900 }}>{title}</Typography>
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 12 }}>
          {detail}
        </Typography>
      </Box>
      {meta ? (
        <Chip
          label={meta}
          size="small"
          sx={{
            bgcolor: "rgba(143,77,255,0.18)",
            color: tokens.color.accent.violet,
            fontFamily: tokens.font.mono,
            fontSize: 10,
            fontWeight: 900,
            height: 22,
          }}
        />
      ) : null}
    </Stack>
  );
}

function StudioSidebar({
  activeSection,
  onSectionChange,
}: {
  activeSection: StudioSection;
  onSectionChange: (section: StudioSection) => void;
}) {
  return (
    <Box
      sx={{
        bgcolor: "#080918",
        borderBottom: {
          xs: `1px solid ${tokens.color.border.subtle}`,
          lg: 0,
        },
        borderRight: { lg: `1px solid ${tokens.color.border.subtle}` },
        display: { xs: "block", lg: "grid" },
        gridTemplateRows: { lg: "auto auto minmax(0, 1fr) auto" },
        minHeight: { xs: "auto", lg: "100dvh" },
        minWidth: 0,
        p: 1.2,
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ px: 0.4, py: 0.4 }}
      >
        <BrandLogo size={25} inverse href="/studio" />
        <Tooltip title="New project" arrow>
          <IconButton
            component={Link}
            href="/"
            size="small"
            aria-label="New project"
            sx={{
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: 1.2,
              color: tokens.color.text.primary,
            }}
          >
            <AddRounded sx={{ fontSize: 18 }} />
          </IconButton>
        </Tooltip>
      </Stack>

      <Box
        sx={{
          alignItems: "center",
          bgcolor: "#0d0f24",
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1.4,
          display: { xs: "none", lg: "flex" },
          gap: 1,
          mt: 1.3,
          px: 1.2,
          py: 0.95,
        }}
      >
        <SearchRounded sx={{ color: tokens.color.text.muted, fontSize: 18 }} />
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
          Search project
        </Typography>
      </Box>

      <Box
        sx={{ mt: { xs: 1, lg: 1.4 }, minHeight: 0, overflow: "auto", pr: 0.2 }}
      >
        <Typography
          sx={{ ...overlineSx, display: { xs: "none", lg: "block" } }}
        >
          Studio
        </Typography>
        <Stack
          direction={{ xs: "row", lg: "column" }}
          spacing={0.35}
          sx={{
            overflowX: { xs: "auto", lg: "visible" },
            pb: { xs: 0.2, lg: 0 },
          }}
        >
          {NAV.map(({ id, label, icon: Icon }) => {
            const active = activeSection === id;
            return (
              <Button
                key={id}
                fullWidth={false}
                startIcon={<Icon sx={{ fontSize: 18 }} />}
                endIcon={active ? <ChevronRightRounded /> : null}
                onClick={() => onSectionChange(id)}
                sx={{
                  borderRadius: 1.2,
                  color: active
                    ? tokens.color.text.primary
                    : tokens.color.text.secondary,
                  justifyContent: "flex-start",
                  minHeight: 39,
                  minWidth: { xs: 112, lg: "auto" },
                  px: 1.1,
                  textTransform: "none",
                  bgcolor: active
                    ? `${tokens.color.accent.purple}45`
                    : "transparent",
                  border: active
                    ? `1px solid ${tokens.color.border.accent}`
                    : "1px solid transparent",
                  "& .MuiButton-endIcon": { ml: "auto" },
                }}
              >
                {label}
              </Button>
            );
          })}
        </Stack>

        <Box sx={{ display: { xs: "none", lg: "block" } }}>
          <Typography sx={{ ...overlineSx, mt: 2 }}>Projects</Typography>
          <StudioSidebarProjects />
        </Box>
      </Box>

      <Box
        sx={{
          display: { xs: "none", lg: "block" },
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1.4,
          bgcolor: "rgba(255,255,255,0.035)",
          p: 1.2,
        }}
      >
        <Stack direction="row" alignItems="center" spacing={1}>
          <ShieldRounded
            sx={{ color: tokens.color.accent.success, fontSize: 17 }}
          />
          <Typography sx={{ fontSize: 13, fontWeight: 900 }}>
            Preview access
          </Typography>
        </Stack>
        <Typography
          sx={{ color: tokens.color.text.secondary, fontSize: 12, mt: 0.8 }}
        >
          No route jumps. Work stays inside Studio.
        </Typography>
      </Box>
    </Box>
  );
}

function StudioSidebarProjects() {
  const { data, loading } = useProjectsQuery({
    variables: { limit: 8, offset: 0 },
    fetchPolicy: "cache-and-network",
  });
  const projects = useMemo(() => {
    const rows = data?.projects ?? [];
    return [...rows]
      .sort(
        (a, b) =>
          new Date(b.updatedAt || b.createdAt).getTime() -
          new Date(a.updatedAt || a.createdAt).getTime(),
      )
      .slice(0, 8);
  }, [data]);

  if (loading && projects.length === 0) {
    return (
      <Typography sx={{ color: tokens.color.text.muted, fontSize: 12, mt: 1 }}>
        Loading…
      </Typography>
    );
  }

  if (projects.length === 0) {
    return (
      <Typography sx={{ color: tokens.color.text.muted, fontSize: 12, mt: 1 }}>
        No projects yet. Start one from the prompt at home.
      </Typography>
    );
  }

  return (
    <Stack spacing={0.3} sx={{ mt: 0.5 }}>
      {projects.map((p) => (
        <Box
          component={Link}
          key={p.id}
          href={`/p/${encodeURIComponent(p.id)}`}
          sx={{
            alignItems: "center",
            borderRadius: 1.2,
            color: tokens.color.text.secondary,
            display: "flex",
            gap: 1,
            px: 1,
            py: 0.9,
            textDecoration: "none",
            "&:hover": {
              bgcolor: tokens.color.bg.surfaceHover,
              color: tokens.color.text.primary,
            },
          }}
        >
          <FolderRounded
            sx={{ color: tokens.color.accent.violet, fontSize: 17 }}
          />
          <Typography
            sx={{
              fontSize: 13.5,
              fontWeight: 800,
              minWidth: 0,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {p.name || `Project ${p.id.slice(0, 6)}`}
          </Typography>
        </Box>
      ))}
    </Stack>
  );
}

function StudioTopbar({
  progress,
  section,
}: {
  progress: number;
  section: { title: string; eyebrow: string; detail: string };
}) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      sx={{
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
        flexWrap: { xs: "wrap", md: "nowrap" },
        gap: 1.2,
        minWidth: 0,
        px: { xs: 1.4, md: 1.8 },
        py: 1,
      }}
    >
      <Box sx={{ minWidth: 0, width: { xs: "100%", md: "auto" } }}>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
          {section.eyebrow}
        </Typography>
        <Typography sx={{ fontSize: 18, fontWeight: 950 }}>
          {section.title}
        </Typography>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            display: { xs: "none", md: "block" },
            fontSize: 11.5,
            mt: 0.25,
          }}
        >
          {section.detail}
        </Typography>
      </Box>
      <Box sx={{ display: { xs: "none", md: "block" }, flex: 1 }} />
      <StatusPill label="Preview" value="Live" tone="success" />
      <StatusPill label="Build" value={`${progress}%`} tone="violet" />
      <StatusPill label="Gate" value="92" tone="violet" />
      <Button
        variant="outlined"
        startIcon={<GitHub sx={{ fontSize: 18 }} />}
        sx={{ display: { xs: "none", md: "inline-flex" } }}
      >
        GitHub
      </Button>
      <Button
        variant="contained"
        endIcon={<RocketLaunchRounded sx={{ fontSize: 18 }} />}
      >
        Publish
      </Button>
    </Stack>
  );
}

function PromptRunway({
  prompt,
  onPromptChange,
  onRun,
}: {
  prompt: string;
  onPromptChange: (value: string) => void;
  onRun: () => void;
}) {
  return (
    <Panel
      title="Prompt runway"
      eyebrow="Build command"
      action={
        <Button
          variant="contained"
          startIcon={<PlayArrowRounded sx={{ fontSize: 18 }} />}
          onClick={onRun}
        >
          Run pass
        </Button>
      }
    >
      <TextField
        fullWidth
        multiline
        minRows={3}
        value={prompt}
        onChange={(event) => onPromptChange(event.target.value)}
        sx={{
          "& .MuiOutlinedInput-root": {
            bgcolor: "#0a0b1b",
            borderRadius: 1.4,
            color: tokens.color.text.primary,
            fontSize: { xs: 14, md: 15 },
            lineHeight: 1.55,
            "& fieldset": { borderColor: tokens.color.border.subtle },
            "&:hover fieldset": { borderColor: tokens.color.border.strong },
          },
        }}
      />
      <Stack direction="row" flexWrap="wrap" sx={{ gap: 0.8, mt: 1 }}>
        {["Roles", "Billing", "Android", "Deploy gate", "Activity feed"].map(
          (item) => (
            <Chip
              key={item}
              label={item}
              size="small"
              sx={{
                bgcolor: "rgba(255,255,255,0.04)",
                border: `1px solid ${tokens.color.border.subtle}`,
                color: tokens.color.text.secondary,
                fontWeight: 800,
              }}
            />
          ),
        )}
      </Stack>
    </Panel>
  );
}

function IdePanel() {
  return (
    <Panel
      title="IDE"
      eyebrow="Code and files"
      fill
      action={
        <Stack direction="row" spacing={0.6}>
          <IconButton size="small">
            <TerminalRounded sx={{ fontSize: 18 }} />
          </IconButton>
          <IconButton size="small">
            <MoreHorizRounded sx={{ fontSize: 18 }} />
          </IconButton>
        </Stack>
      }
    >
      <Box
        sx={{
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1.4,
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "230px minmax(0, 1fr)" },
          minHeight: { xs: 420, lg: 0 },
          height: "100%",
          overflow: "hidden",
        }}
      >
        <Box
          sx={{
            bgcolor: "#080918",
            borderRight: { md: `1px solid ${tokens.color.border.subtle}` },
            display: { xs: "none", md: "block" },
            p: 1.2,
          }}
        >
          <Typography sx={overlineSx}>Explorer</Typography>
          <Stack spacing={0.45}>
            {FILES.map((file, index) => (
              <Stack
                key={file}
                direction="row"
                spacing={0.8}
                alignItems="center"
                sx={{
                  borderRadius: 0.8,
                  color:
                    index === 0
                      ? tokens.color.text.primary
                      : tokens.color.text.secondary,
                  bgcolor:
                    index === 0 ? "rgba(143,77,255,0.18)" : "transparent",
                  px: 0.8,
                  py: 0.55,
                }}
              >
                <CodeRounded
                  sx={{ color: tokens.color.accent.violet, fontSize: 14 }}
                />
                <Typography
                  sx={{ fontFamily: tokens.font.mono, fontSize: 11.5 }}
                >
                  {file}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </Box>
        <Box sx={{ bgcolor: "#060713", minWidth: 0, overflow: "auto" }}>
          <Stack
            direction="row"
            alignItems="center"
            sx={{
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
              minHeight: 42,
              px: 1.2,
            }}
          >
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 12,
                fontWeight: 900,
              }}
            >
              Dashboard.tsx
            </Typography>
            <Box sx={{ flex: 1 }} />
            <Typography
              sx={{
                color: tokens.color.accent.success,
                fontFamily: tokens.font.mono,
                fontSize: 11,
              }}
            >
              no errors
            </Typography>
          </Stack>
          <Box component="pre" sx={{ m: 0, p: 2, minWidth: 620 }}>
            {CODE_LINES.map((line, index) => (
              <Box
                key={`${line}-${index}`}
                sx={{
                  display: "grid",
                  gridTemplateColumns: "36px minmax(0, 1fr)",
                }}
              >
                <Box
                  sx={{
                    color: tokens.color.text.muted,
                    fontFamily: tokens.font.mono,
                    fontSize: 13,
                    textAlign: "right",
                    pr: 1.4,
                  }}
                >
                  {index + 1}
                </Box>
                <Box
                  sx={{
                    color: codeColor(line),
                    fontFamily: tokens.font.mono,
                    fontSize: 13,
                    lineHeight: 1.75,
                    whiteSpace: "pre",
                  }}
                >
                  {line || " "}
                </Box>
              </Box>
            ))}
          </Box>
        </Box>
      </Box>
    </Panel>
  );
}

function StagePanel({
  view,
  onViewChange,
}: {
  view: StageView;
  onViewChange: (view: StageView) => void;
}) {
  return (
    <Panel
      title="Live stage"
      eyebrow="Preview surface"
      fill
      action={
        <Stack direction="row" spacing={0.5}>
          {(
            [
              ["browser", LaptopMacRounded],
              ["ide", CodeRounded],
              ["android", PhoneAndroidRounded],
            ] as const
          ).map(([key, Icon]) => (
            <IconButton
              key={key}
              size="small"
              onClick={() => onViewChange(key)}
              sx={{
                bgcolor:
                  view === key
                    ? `${tokens.color.accent.purple}55`
                    : "transparent",
                border: `1px solid ${view === key ? tokens.color.border.accent : tokens.color.border.subtle}`,
              }}
            >
              <Icon sx={{ fontSize: 18 }} />
            </IconButton>
          ))}
        </Stack>
      }
    >
      <Box
        sx={{
          alignItems: "center",
          bgcolor: "#050610",
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1.4,
          display: "grid",
          height: "100%",
          minHeight: { xs: 420, lg: 0 },
          overflow: "hidden",
          p: view === "android" ? 1.6 : 1.1,
        }}
      >
        {view === "android" ? (
          <AndroidPreview />
        ) : view === "ide" ? (
          <IdeStagePreview />
        ) : (
          <BrowserPreview compact={false} />
        )}
      </Box>
    </Panel>
  );
}

function BrowserPreview({ compact }: { compact: boolean }) {
  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.strong}`,
        borderRadius: 1.5,
        bgcolor: "#0d1025",
        minHeight: 0,
        overflow: "hidden",
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        spacing={0.7}
        sx={{
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          px: 1,
          py: 0.8,
        }}
      >
        {["#ff6d5f", "#ffbc4a", "#65e38b"].map((color) => (
          <Box
            key={color}
            sx={{ bgcolor: color, borderRadius: "50%", height: 8, width: 8 }}
          />
        ))}
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            ml: 0.5,
          }}
        >
          clientflow.preview
        </Typography>
      </Stack>
      <Box sx={{ p: compact ? 1.2 : 1.6 }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <RocketLaunchRounded sx={{ color: tokens.color.accent.violet }} />
          <Typography sx={{ fontSize: 18, fontWeight: 950 }}>
            Client operations portal
          </Typography>
          <Box sx={{ flex: 1 }} />
          <Chip
            label="Live"
            size="small"
            sx={{
              bgcolor: `${tokens.color.accent.success}24`,
              color: tokens.color.accent.success,
              fontWeight: 900,
            }}
          />
        </Stack>
        <Box
          sx={{
            display: "grid",
            gap: 1,
            gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
            mt: 1.5,
          }}
        >
          {[
            ["Revenue", "$18.2k", tokens.color.accent.violet],
            ["Approvals", "27", tokens.color.accent.coral],
            ["Files", "1.4k", tokens.color.accent.sky],
            ["Deploy", "92", tokens.color.accent.success],
          ].map(([label, value, color]) => (
            <Box
              key={label}
              sx={{
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: 1.2,
                bgcolor: "rgba(255,255,255,0.035)",
                p: 1.2,
              }}
            >
              <Typography
                sx={{ color: tokens.color.text.secondary, fontSize: 12 }}
              >
                {label}
              </Typography>
              <Typography sx={{ color, fontSize: 26, fontWeight: 950 }}>
                {value}
              </Typography>
            </Box>
          ))}
        </Box>
        <Box
          sx={{
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1.2,
            mt: 1.2,
            overflow: "hidden",
          }}
        >
          {[
            "Website redesign",
            "Mobile app",
            "CRM integration",
            "Analytics dashboard",
          ].map((row, index) => (
            <Stack
              key={row}
              direction="row"
              sx={{
                borderTop: index
                  ? `1px solid ${tokens.color.border.subtle}`
                  : 0,
                px: 1.2,
                py: 1,
              }}
            >
              <Typography sx={{ flex: 1, fontSize: 13 }}>{row}</Typography>
              <Typography
                sx={{
                  color:
                    index === 1
                      ? tokens.color.accent.violet
                      : tokens.color.accent.success,
                  fontSize: 12,
                  fontWeight: 900,
                }}
              >
                {index === 1 ? "Preview" : "Live"}
              </Typography>
            </Stack>
          ))}
        </Box>
      </Box>
    </Box>
  );
}

function IdeStagePreview() {
  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.strong}`,
        borderRadius: 1.5,
        bgcolor: "#080918",
        minHeight: 0,
        overflow: "hidden",
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        sx={{
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          gap: 1,
          px: 1,
          py: 0.8,
        }}
      >
        <TerminalRounded
          sx={{ color: tokens.color.accent.violet, fontSize: 17 }}
        />
        <Typography
          sx={{ fontFamily: tokens.font.mono, fontSize: 11.5, fontWeight: 900 }}
        >
          clientflow.workspace
        </Typography>
        <Box sx={{ flex: 1 }} />
        <Typography
          sx={{
            color: tokens.color.accent.success,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            fontWeight: 900,
          }}
        >
          clean
        </Typography>
      </Stack>
      <Box sx={{ display: "grid", gap: 1, p: 1.2 }}>
        {[
          ["edited", "src/app/dashboard/page.tsx", "+38"],
          ["created", "src/components/ApprovalTable.tsx", "+112"],
          ["wired", "src/lib/roles.ts", "+24"],
          ["ready", "tests/clientflow.spec.ts", "92"],
        ].map(([state, file, score]) => (
          <Stack
            key={file}
            direction="row"
            alignItems="center"
            sx={{
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: 1.2,
              bgcolor: "rgba(255,255,255,0.035)",
              gap: 1,
              px: 1,
              py: 0.9,
            }}
          >
            <Chip
              label={state}
              size="small"
              sx={{
                bgcolor: "rgba(143,77,255,0.18)",
                color: tokens.color.accent.violet,
                fontFamily: tokens.font.mono,
                fontSize: 10,
                fontWeight: 900,
                height: 22,
                minWidth: 62,
              }}
            />
            <Typography
              sx={{
                flex: 1,
                fontFamily: tokens.font.mono,
                fontSize: 12,
                minWidth: 0,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {file}
            </Typography>
            <Typography
              sx={{
                color: tokens.color.accent.success,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                fontWeight: 900,
              }}
            >
              {score}
            </Typography>
          </Stack>
        ))}
      </Box>
    </Box>
  );
}

function AndroidPreview() {
  return (
    <Box
      sx={{
        border: "7px solid #111426",
        borderRadius: 4,
        bgcolor: "#070814",
        boxShadow: "0 22px 70px rgba(0,0,0,0.42)",
        height: "100%",
        maxHeight: 590,
        maxWidth: 292,
        minHeight: 470,
        mx: "auto",
        overflow: "hidden",
        width: "100%",
      }}
    >
      <Stack direction="row" alignItems="center" sx={{ px: 1.3, py: 1.1 }}>
        <Typography sx={{ fontSize: 12, fontWeight: 900 }}>9:41</Typography>
        <Box sx={{ flex: 1 }} />
        <BoltRounded
          sx={{ fontSize: 15, color: tokens.color.accent.success }}
        />
      </Stack>
      <Box sx={{ px: 1.4, pb: 1.4 }}>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 11 }}>
          Android Preview
        </Typography>
        <Typography sx={{ fontSize: 20, fontWeight: 950 }}>
          ClientFlow
        </Typography>
        <Box sx={{ display: "grid", gap: 0.8, mt: 1.2 }}>
          {["Approvals ready", "Revenue synced", "Files imported"].map(
            (item) => (
              <Stack
                key={item}
                direction="row"
                alignItems="center"
                sx={{
                  border: `1px solid ${tokens.color.border.subtle}`,
                  borderRadius: 1.2,
                  bgcolor: "rgba(255,255,255,0.04)",
                  px: 1,
                  py: 1,
                }}
              >
                <CheckRounded
                  sx={{ color: tokens.color.accent.success, fontSize: 17 }}
                />
                <Typography sx={{ ml: 0.8, fontSize: 13, fontWeight: 800 }}>
                  {item}
                </Typography>
              </Stack>
            ),
          )}
        </Box>
      </Box>
    </Box>
  );
}

function BuildGraph({
  steps,
}: {
  steps: Array<{
    id: string;
    label: string;
    detail: string;
    state: BuildStepState;
  }>;
}) {
  return (
    <Panel title="Build graph" eyebrow="What changed">
      <Box sx={{ display: "grid", gap: 1 }}>
        {steps.map((step, index) => (
          <Stack
            key={step.id}
            direction="row"
            spacing={1}
            alignItems="flex-start"
          >
            <Box
              sx={{
                alignItems: "center",
                display: "grid",
                justifyItems: "center",
              }}
            >
              <Box
                sx={{
                  alignItems: "center",
                  bgcolor: stepColor(step.state),
                  borderRadius: "50%",
                  color:
                    step.state === "queued" ? tokens.color.text.muted : "#fff",
                  display: "grid",
                  height: 28,
                  placeItems: "center",
                  width: 28,
                }}
              >
                {step.state === "done" ? (
                  <CheckRounded sx={{ fontSize: 17 }} />
                ) : (
                  index + 1
                )}
              </Box>
              {index < steps.length - 1 ? (
                <Box
                  sx={{
                    bgcolor: tokens.color.border.subtle,
                    height: 36,
                    width: 1,
                  }}
                />
              ) : null}
            </Box>
            <Box
              sx={{
                flex: 1,
                minWidth: 0,
                pb: index < steps.length - 1 ? 0.4 : 0,
              }}
            >
              <Stack direction="row" alignItems="center" spacing={1}>
                <Typography sx={{ fontSize: 14, fontWeight: 950 }}>
                  {step.label}
                </Typography>
                <Chip
                  label={step.state}
                  size="small"
                  sx={{
                    bgcolor: `${stepColor(step.state)}22`,
                    color:
                      step.state === "queued"
                        ? tokens.color.text.muted
                        : stepColor(step.state),
                    fontFamily: tokens.font.mono,
                    fontSize: 10,
                    fontWeight: 900,
                    height: 21,
                  }}
                />
              </Stack>
              <Typography
                sx={{
                  color: tokens.color.text.secondary,
                  fontSize: 12.5,
                  mt: 0.25,
                }}
              >
                {step.detail}
              </Typography>
            </Box>
          </Stack>
        ))}
      </Box>
    </Panel>
  );
}

function CommandChat({
  messages,
  pending,
  onSend,
  onStop,
}: {
  messages: ChatMessage[];
  pending: boolean;
  onSend: (message: string) => void;
  onStop: () => void;
}) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const visibleMessages = messages.slice(-4);

  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1.5,
        bgcolor: "rgba(12,13,32,0.9)",
        display: "grid",
        gap: 0.9,
        minWidth: 0,
        p: 1,
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <BoltRounded sx={{ color: tokens.color.accent.violet, fontSize: 16 }} />
        <Typography sx={{ fontSize: 12.5, fontWeight: 950 }}>Chat</Typography>
        <Box sx={{ flex: 1 }} />
        <Typography
          sx={{
            color: pending
              ? tokens.color.accent.violet
              : tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            fontWeight: 900,
            textTransform: "uppercase",
          }}
        >
          {pending ? "Working" : "Ready"}
        </Typography>
      </Stack>
      <Box
        sx={{
          display: "grid",
          gap: 0.65,
          maxHeight: { xs: 132, lg: 122 },
          overflow: "auto",
          pr: 0.3,
        }}
      >
        {visibleMessages.map((message) => (
          <Box
            key={message.id}
            sx={{
              alignSelf: message.role === "user" ? "end" : "start",
              bgcolor:
                message.role === "user"
                  ? "rgba(143,77,255,0.24)"
                  : "rgba(255,255,255,0.045)",
              border: `1px solid ${
                message.role === "user"
                  ? "rgba(188,117,255,0.34)"
                  : tokens.color.border.subtle
              }`,
              borderRadius: 1.2,
              color: tokens.color.text.primary,
              fontSize: 12.6,
              lineHeight: 1.45,
              maxWidth: "86%",
              px: 1,
              py: 0.75,
            }}
          >
            {message.body || "Thinking..."}
          </Box>
        ))}
      </Box>
      <Stack
        component="form"
        direction="row"
        spacing={1}
        onSubmit={(event) => {
          event.preventDefault();
          const nextMessage = inputRef.current?.value ?? "";
          onSend(nextMessage);
          if (inputRef.current) inputRef.current.value = "";
        }}
      >
        <Box
          component="input"
          name="studio-chat"
          ref={inputRef}
          placeholder="Ask for a change..."
          sx={{
            bgcolor: "#080918",
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1.2,
            color: tokens.color.text.primary,
            flex: 1,
            font: "inherit",
            fontSize: 13,
            minWidth: 0,
            outline: 0,
            px: 1.2,
          }}
        />
        <IconButton
          type={pending ? "button" : "submit"}
          aria-label={pending ? "Stop response" : "Send message"}
          onClick={pending ? onStop : undefined}
          sx={{
            bgcolor: `${tokens.color.accent.purple}65`,
            borderRadius: 1.2,
            color: tokens.color.text.primary,
          }}
        >
          {pending ? (
            <StopRounded sx={{ fontSize: 18 }} />
          ) : (
            <SendRounded sx={{ fontSize: 18 }} />
          )}
        </IconButton>
      </Stack>
    </Box>
  );
}

function Panel({
  title,
  eyebrow,
  action,
  children,
  fill = false,
}: {
  title: string;
  eyebrow: string;
  action?: ReactNode;
  children: ReactNode;
  fill?: boolean;
}) {
  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1.5,
        bgcolor: "rgba(12,13,32,0.88)",
        boxShadow: "0 18px 50px rgba(0,0,0,0.20)",
        display: "grid",
        gridTemplateRows: "auto minmax(0, 1fr)",
        minHeight: fill ? 0 : "auto",
        minWidth: 0,
        overflow: "hidden",
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          px: 1.4,
          py: 1.05,
        }}
      >
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={overlineSx}>{eyebrow}</Typography>
          <Typography sx={{ fontSize: 16, fontWeight: 950 }}>
            {title}
          </Typography>
        </Box>
        <Box sx={{ flex: 1 }} />
        {action}
      </Stack>
      <Box sx={{ minHeight: 0, minWidth: 0, overflow: "hidden", p: 1.2 }}>
        {children}
      </Box>
    </Box>
  );
}

function StatusPill({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "success" | "violet";
}) {
  const color =
    tone === "success"
      ? tokens.color.accent.success
      : tokens.color.accent.violet;
  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1.2,
        bgcolor: "rgba(255,255,255,0.035)",
        minWidth: { xs: 68, md: 96 },
        px: 1.1,
        py: 0.7,
      }}
    >
      <Typography sx={{ color: tokens.color.text.muted, fontSize: 10.5 }}>
        {label}
      </Typography>
      <Typography sx={{ color, fontSize: 14, fontWeight: 950 }}>
        {value}
      </Typography>
    </Box>
  );
}

function stepColor(state: BuildStepState) {
  if (state === "done") return tokens.color.accent.success;
  if (state === "running") return tokens.color.accent.violet;
  return "rgba(255,255,255,0.08)";
}

function codeColor(line: string) {
  if (line.startsWith("import")) return tokens.color.accent.violet;
  if (line.trim().startsWith("<")) return tokens.color.accent.success;
  if (line.includes("return") || line.includes("const")) return "#e7d6ff";
  return tokens.color.text.secondary;
}

const overlineSx = {
  color: tokens.color.text.muted,
  fontFamily: tokens.font.mono,
  fontSize: 10.5,
  fontWeight: 900,
  letterSpacing: 0.6,
  textTransform: "uppercase",
} as const;
