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
import { useMemo, useRef, useState, type ReactNode } from "react";
import { BrandLogo } from "../../src/components/BrandLogo";
import { getToken } from "../../src/lib/apollo";
import { streamChat, type StreamChatHandle } from "../../src/lib/chat/stream";
import { tokens } from "../../../../packages/design-tokens";

type StageView = "browser" | "ide" | "android";
type BuildStepState = "done" | "running" | "queued";
type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  body: string;
};

const NAV = [
  ["Command", DashboardRounded],
  ["Prompt", AutoAwesomeRounded],
  ["Browser", LaptopMacRounded],
  ["IDE", CodeRounded],
  ["Android", PhoneAndroidRounded],
  ["Graph", AccountTreeRounded],
  ["Deploy", RocketLaunchRounded],
] as const;

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
  const streamRef = useRef<StreamChatHandle | null>(null);

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
    setStageView("android");
  };

  const sendChat = (body: string) => {
    const message = body.trim();
    if (!message || chatPending) return;

    streamRef.current?.abort();
    const userId = `u-${Date.now()}`;
    const assistantId = `a-${Date.now()}`;
    setMessages((current) => [
      ...current,
      { id: userId, role: "user", body: message },
      { id: assistantId, role: "assistant", body: "" },
    ]);
    setChatPending(true);

    const fillAssistant = (nextBody: string) => {
      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId ? { ...item, body: nextBody } : item,
        ),
      );
    };

    const token = getToken();
    if (!token) {
      window.setTimeout(() => {
        fillAssistant(previewReply(message));
        setChatPending(false);
      }, 420);
      return;
    }

    let buffer = "";
    const handle = streamChat({ executionID: "_", message, token }, (event) => {
      if (event.type === "delta") {
        const chunk =
          typeof event.data.text === "string" ? event.data.text : "";
        if (!chunk) return;
        buffer += chunk;
        fillAssistant(buffer);
        return;
      }

      if (event.type === "finish") {
        fillAssistant(buffer || previewReply(message));
        return;
      }

      if (event.type === "error") {
        fillAssistant(previewReply(message));
      }
    });

    streamRef.current = handle;
    handle.done.finally(() => {
      if (streamRef.current === handle) streamRef.current = null;
      if (!buffer) fillAssistant(previewReply(message));
      setChatPending(false);
    });
  };

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
      <StudioSidebar />
      <Box
        sx={{
          display: "grid",
          gridTemplateRows: { xs: "auto auto", lg: "64px minmax(0, 1fr)" },
          minWidth: 0,
        }}
      >
        <StudioTopbar progress={progress} />
        <Box
          sx={{
            display: "grid",
            gap: 1.2,
            gridTemplateColumns: {
              xs: "1fr",
              lg: "minmax(560px, 1.18fr) minmax(380px, 0.82fr)",
            },
            minHeight: 0,
            minWidth: 0,
            overflow: { xs: "visible", lg: "auto" },
            p: { xs: 1.2, md: 1.4 },
          }}
        >
          <Box
            sx={{
              display: "grid",
              gap: 1.2,
              gridTemplateRows: {
                xs: "auto auto auto",
                lg: "auto auto minmax(0, 1fr)",
              },
              minHeight: 0,
              minWidth: 0,
            }}
          >
            <PromptRunway
              prompt={prompt}
              onPromptChange={setPrompt}
              onRun={runBuildPass}
            />
            <CommandChat
              messages={messages}
              pending={chatPending}
              onSend={sendChat}
            />
            <IdePanel />
          </Box>
          <Box
            sx={{
              display: "grid",
              gap: 1.2,
              gridTemplateRows: { xs: "auto auto", lg: "minmax(0, 1fr) auto" },
              minHeight: 0,
              minWidth: 0,
            }}
          >
            <StagePanel view={stageView} onViewChange={setStageView} />
            <BuildGraph steps={steps} />
          </Box>
        </Box>
      </Box>
    </Box>
  );
}

function StudioSidebar() {
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
            size="small"
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
          {NAV.map(([label, Icon], index) => (
            <Button
              key={label}
              fullWidth={false}
              startIcon={<Icon sx={{ fontSize: 18 }} />}
              endIcon={index === 0 ? <ChevronRightRounded /> : null}
              sx={{
                borderRadius: 1.2,
                color:
                  index === 0
                    ? tokens.color.text.primary
                    : tokens.color.text.secondary,
                justifyContent: "flex-start",
                minHeight: 39,
                minWidth: { xs: 112, lg: "auto" },
                px: 1.1,
                textTransform: "none",
                bgcolor:
                  index === 0
                    ? `${tokens.color.accent.purple}45`
                    : "transparent",
                border:
                  index === 0
                    ? `1px solid ${tokens.color.border.accent}`
                    : "1px solid transparent",
                "& .MuiButton-endIcon": { ml: "auto" },
              }}
            >
              {label}
            </Button>
          ))}
        </Stack>

        <Box sx={{ display: { xs: "none", lg: "block" } }}>
          <Typography sx={{ ...overlineSx, mt: 2 }}>Projects</Typography>
          {["ClientFlow", "Marketplace", "FieldOps", "InvoicePro"].map(
            (project, index) => (
              <Stack
                key={project}
                direction="row"
                alignItems="center"
                spacing={1}
                sx={{
                  borderRadius: 1.2,
                  color:
                    index === 0
                      ? tokens.color.text.primary
                      : tokens.color.text.secondary,
                  px: 1,
                  py: 0.9,
                  bgcolor:
                    index === 0 ? "rgba(255,255,255,0.04)" : "transparent",
                }}
              >
                <FolderRounded
                  sx={{ color: tokens.color.accent.violet, fontSize: 17 }}
                />
                <Typography sx={{ fontSize: 13.5, fontWeight: 800 }}>
                  {project}
                </Typography>
              </Stack>
            ),
          )}
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

function StudioTopbar({ progress }: { progress: number }) {
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
          Studio / ClientFlow
        </Typography>
        <Typography sx={{ fontSize: 18, fontWeight: 950 }}>
          Production workspace
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
        ) : (
          <BrowserPreview compact={view === "ide"} />
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
}: {
  messages: ChatMessage[];
  pending: boolean;
  onSend: (message: string) => void;
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
          type="submit"
          disabled={pending}
          sx={{ bgcolor: `${tokens.color.accent.purple}65`, borderRadius: 1.2 }}
        >
          <SendRounded sx={{ fontSize: 18 }} />
        </IconButton>
      </Stack>
    </Box>
  );
}

function previewReply(message: string) {
  const lower = message.toLowerCase();
  if (lower.includes("mobile") || lower.includes("android")) {
    return "Android pass queued: I will tighten spacing, keep the preview inside the device frame, and preserve the studio layout.";
  }
  if (lower.includes("code") || lower.includes("ide")) {
    return "IDE pass ready: I will keep the file tree stable, expose only the useful code pane, and avoid extra studio clutter.";
  }
  if (lower.includes("deploy") || lower.includes("publish")) {
    return "Deploy pass ready: I will keep the gate visible, show blockers clearly, and avoid leaving the studio route.";
  }
  return "Got it. I will apply this inside the studio flow and keep the interface focused.";
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
