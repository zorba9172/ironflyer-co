"use client";

import {
  AddRounded,
  AppsRounded,
  AutoAwesomeRounded,
  BoltRounded,
  CheckRounded,
  CodeRounded,
  ContentCopyRounded,
  DashboardRounded,
  FolderRounded,
  GitHub,
  HomeRounded,
  HubRounded,
  IntegrationInstructionsRounded,
  KeyboardArrowRightRounded,
  LaptopMacRounded,
  MoreHorizRounded,
  NotificationsNoneRounded,
  OpenInFullRounded,
  PhoneIphoneRounded,
  PlayArrowRounded,
  RefreshRounded,
  RocketLaunchRounded,
  SendRounded,
  SettingsRounded,
  StarRounded,
  TabletMacRounded,
} from "@mui/icons-material";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState, type ChangeEvent } from "react";
import { tokens } from "../../../../packages/design-tokens";
import { extractErrorMessage } from "../../src/lib/errors";
import {
  useDescribeIdeaMutation,
  useProjectsQuery,
} from "../../src/lib/gql/__generated__";
import { useAuth } from "../../src/lib/auth";

type StudioMode = "preview" | "mobile" | "code";

const DEFAULT_PROMPT =
  "Build a client operations portal with projects, invoices, approvals, role-based access and team activity dashboard.";

const CODE_LINES = [
  "import { Card, Stat, Table, Badge } from '@/ui'",
  "import { useProjects } from '@/hooks/useProjects'",
  "",
  "export default function Dashboard() {",
  "  const { data: projects } = useProjects()",
  "",
  "  return (",
  "    <div className=\"p-6 space-y-6\">",
  "      <StatsGrid>",
  "        <Stat label=\"Revenue\" value=\"$18.2k\" />",
  "        <Stat label=\"Open approvals\" value=\"27\" />",
  "        <Stat label=\"Files\" value=\"1.4k\" />",
  "        <Stat label=\"Deploy health\" value=\"99.9%\" />",
  "      </StatsGrid>",
  "      <ProjectsTable data={projects} />",
  "    </div>",
  "  )",
  "}",
];

export default function StudioIndexPage() {
  const router = useRouter();
  const { authenticated } = useAuth();
  const [mode, setMode] = useState<StudioMode>("code");
  const [prompt, setPrompt] = useState(DEFAULT_PROMPT);
  const [assistantText, setAssistantText] = useState(
    "Added project table, connected Stripe billing flow, generated role-based access, and prepared the preview shell.",
  );
  const [error, setError] = useState<string | null>(null);
  const projectsQuery = useProjectsQuery({
    variables: { limit: 6 },
    skip: !authenticated,
    fetchPolicy: "cache-and-network",
  });
  const [describeIdea, { loading: creating }] = useDescribeIdeaMutation();

  const recents = useMemo(
    () =>
      (projectsQuery.data?.projects ?? [])
        .slice()
        .sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : -1))
        .slice(0, 5),
    [projectsQuery.data],
  );

  const simulateGuestBuild = () => {
    setError(null);
    setMode("code");
    setAssistantText(
      "Guest build simulated: plan locked, app shell generated, preview data wired, and deployment checklist prepared. Sign in to run this against a live workspace.",
    );
  };

  const onGenerate = async () => {
    const text = prompt.trim();
    if (!text || creating) return;
    if (!authenticated) {
      simulateGuestBuild();
      return;
    }
    setError(null);
    try {
      const result = await describeIdea({
        variables: { input: { text, startImmediately: true } },
      });
      const project = result.data?.describeIdea.project;
      const execution = result.data?.describeIdea.execution;
      if (!project?.id) throw new Error("Studio did not return a project id.");
      const params = new URLSearchParams({ autorun: "1", tab: "preview" });
      if (execution?.id) params.set("executionID", execution.id);
      void projectsQuery.refetch().catch(() => undefined);
      router.push(`/p/${encodeURIComponent(project.id)}?${params.toString()}`);
    } catch (err) {
      setError(extractErrorMessage(err));
    }
  };

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        background: `radial-gradient(circle at 94% 18%, ${tokens.color.accent.purple}42, transparent 23%), radial-gradient(circle at 9% 90%, ${tokens.color.accent.violet}24, transparent 24%), ${tokens.color.bg.base}`,
        color: tokens.color.text.primary,
        display: "grid",
        gridTemplateColumns: { xs: "1fr", lg: "250px minmax(0, 1fr)" },
        minHeight: "calc(100vh - 58px)",
        height: { xs: "auto", lg: "calc(100vh - 58px)" },
        isolation: "isolate",
        minWidth: 0,
        overflow: { xs: "visible", lg: "hidden" },
        position: "relative",
        "&::before": {
          border: `1px solid ${tokens.color.accent.violet}66`,
          borderRadius: "50%",
          boxShadow: `0 0 48px ${tokens.color.accent.violet}55, inset 0 0 18px ${tokens.color.accent.purple}44`,
          content: '""',
          display: { xs: "none", xl: "block" },
          height: 138,
          pointerEvents: "none",
          position: "absolute",
          right: -92,
          top: 76,
          transform: "rotate(-17deg)",
          width: 330,
          zIndex: 0,
        },
        "&::after": {
          background:
            `radial-gradient(circle, ${tokens.color.accent.violet}e6 0 1px, transparent 1.8px) 12% 18% / 180px 120px, radial-gradient(circle, ${tokens.color.accent.sky}9e 0 1px, transparent 1.8px) 78% 20% / 210px 150px`,
          content: '""',
          display: { xs: "none", lg: "block" },
          inset: 0,
          opacity: 0.42,
          pointerEvents: "none",
          position: "absolute",
          zIndex: 0,
        },
      }}
    >
      <StudioRail recents={recents} authenticated={authenticated} />
      <Box sx={{ display: "flex", flexDirection: "column", minHeight: 0, minWidth: 0, position: "relative", zIndex: 1 }}>
        <TopBar />
        <Box
          sx={{
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
            display: "grid",
            gap: 1.2,
            gridTemplateColumns: {
              xs: "1fr",
              md: "minmax(340px, 480px) minmax(0, 560px)",
            },
            justifyContent: "space-between",
            minWidth: 0,
            px: { xs: 1.2, md: 1.8 },
            py: 1.1,
          }}
        >
          <ModeTabs mode={mode} onChange={setMode} />
          <StatusCards />
        </Box>
        <Box
          sx={{
            display: "grid",
            flex: 1,
            gap: { xs: 1.2, xl: 1.6 },
            gridTemplateColumns: {
              xs: "1fr",
              md: "minmax(250px, 0.62fr) minmax(0, 1.38fr)",
              lg: "320px minmax(0, 1fr) 410px",
              xl: "320px minmax(0, 1fr) 430px",
            },
            gridTemplateRows: { xs: "auto", lg: "minmax(0, 1fr) auto" },
            minHeight: 0,
            minWidth: 0,
            overflow: { xs: "auto", lg: "hidden" },
            p: { xs: 1.2, md: 1.6 },
          }}
        >
          <PromptPanel
            prompt={prompt}
            creating={creating}
            error={error}
            onPromptChange={setPrompt}
            onImprove={(nextPrompt) => {
              setPrompt(nextPrompt);
              setAssistantText("Prompt improved with roles, billing, mobile, deploy gates, and admin coverage.");
            }}
            onGenerate={onGenerate}
            onSuggestion={(text) => {
              setPrompt((current) => `${current.trim()}\n${text}`.trim());
              setAssistantText(`Queued Studio suggestion: ${text}`);
            }}
          />
          <CodeWorkbench mode={mode} />
          <PreviewWorkbench mode={mode} onModeChange={setMode} />
          <AssistantDock
            assistantText={assistantText}
            onSend={(message) => {
              setAssistantText(`Queued refinement: ${message}`);
              setPrompt((current) => `${current.trim()}\nRefine: ${message}`.trim());
            }}
          />
        </Box>
      </Box>
    </Box>
  );
}

function StudioRail({
  recents,
  authenticated,
}: {
  recents: { id: string; name: string }[];
  authenticated: boolean;
}) {
  const nav = [
    { label: "Home", icon: HomeRounded, href: "/" },
    { label: "All apps", icon: AppsRounded, href: "/projects" },
    { label: "Templates", icon: DashboardRounded, href: "/templates" },
    { label: "Integrations", icon: IntegrationInstructionsRounded, href: "/resources" },
    { label: "Studio", icon: HubRounded, href: "/studio", active: true },
  ];
  const visibleRecents =
    authenticated && recents.length > 0
      ? recents.map((p) => p.name)
      : ["MathQuest", "ClientFlow", "Fit booking", "InvoicePro", "TeamHub"];

  return (
    <Box
      sx={{
        borderRight: `1px solid ${tokens.color.border.subtle}`,
        display: { xs: "none", lg: "flex" },
        flexDirection: "column",
        minHeight: 0,
        overflow: "hidden",
        p: 1.4,
        position: "relative",
        zIndex: 1,
      }}
    >
      <Button
        component={Link}
        href="/studio"
        startIcon={<AddRounded />}
        sx={{
          justifyContent: "flex-start",
          border: `1px solid ${tokens.color.border.accent}`,
          bgcolor: `${tokens.color.bg.surfaceRaised}a8`,
          color: tokens.color.text.primary,
          minHeight: 38,
        }}
      >
        New app
      </Button>
      <Stack spacing={0.65} sx={{ mt: 2 }}>
        {nav.map((item) => {
          const Icon = item.icon;
          return (
            <Box
              key={item.label}
              component={Link}
              href={item.href}
              sx={{
                alignItems: "center",
                bgcolor: item.active ? `${tokens.color.accent.purple}70` : "transparent",
                border: item.active ? `1px solid ${tokens.color.border.accent}` : "1px solid transparent",
                borderRadius: `${tokens.radius.sm}px`,
                color: tokens.color.text.primary,
                display: "flex",
                fontSize: 14,
                fontWeight: 800,
                gap: 1,
                minWidth: 0,
                px: 1.1,
                py: 1,
                textDecoration: "none",
              }}
            >
              <Icon sx={{ fontSize: 18 }} />
              <Box sx={{ flex: 1 }}>{item.label}</Box>
              {item.active ? <KeyboardArrowRightRounded sx={{ fontSize: 18 }} /> : null}
            </Box>
          );
        })}
      </Stack>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 11,
          letterSpacing: 0.6,
          mt: 2.2,
          textTransform: "uppercase",
        }}
      >
        Recents
      </Typography>
      <Stack spacing={0.45} sx={{ flex: 1, minHeight: 0, mt: 0.7, overflow: "auto" }}>
        {visibleRecents.map((name) => (
          <Box
            key={name}
            sx={{
              alignItems: "center",
              border: name === "ClientFlow" ? `1px solid ${tokens.color.border.strong}` : "1px solid transparent",
              borderRadius: `${tokens.radius.sm}px`,
              bgcolor: name === "ClientFlow" ? `${tokens.color.accent.purple}24` : "transparent",
              color: name === "ClientFlow" ? tokens.color.text.primary : tokens.color.text.secondary,
              display: "flex",
              fontSize: 13,
              fontWeight: 700,
              gap: 0.8,
              px: 0.9,
              py: 0.65,
            }}
          >
            <StarRounded sx={{ color: tokens.color.accent.violet, fontSize: 15 }} />
            <Box sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {name}
            </Box>
          </Box>
        ))}
      </Stack>
      <Box
        sx={{
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: `${tokens.radius.md}px`,
          bgcolor: `${tokens.color.bg.surfaceRaised}b8`,
          p: 1.4,
        }}
      >
        <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
          <BoltRounded sx={{ color: tokens.color.accent.violet, fontSize: 18 }} />
          <Typography sx={{ fontSize: 13, fontWeight: 900 }}>Upgrade your plan</Typography>
        </Stack>
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 12.5, lineHeight: 1.45, mt: 0.8 }}>
          Get more agent minutes and private deploys.
        </Typography>
        <Button component={Link} href="/pricing" fullWidth size="small" sx={{ mt: 1, border: `1px solid ${tokens.color.border.accent}` }}>
          Upgrade now
        </Button>
      </Box>
      <Stack spacing={0.6} sx={{ mt: 1.3 }}>
        {[
          ["Settings", SettingsRounded],
          ["What's new", NotificationsNoneRounded],
        ].map(([label, Icon]) => {
          const NavIcon = Icon as typeof SettingsRounded;
          return (
            <Stack key={label as string} direction="row" sx={{ alignItems: "center", color: tokens.color.text.secondary, gap: 1, px: 0.8, py: 0.45 }}>
              <NavIcon sx={{ fontSize: 17 }} />
              <Typography sx={{ fontSize: 13, fontWeight: 700 }}>{label as string}</Typography>
            </Stack>
          );
        })}
      </Stack>
      <Box
        sx={{
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: `${tokens.radius.md}px`,
          bgcolor: `${tokens.color.bg.inset}c7`,
          mt: 1,
          p: 1.1,
        }}
      >
        <Typography sx={{ fontSize: 13, fontWeight: 900 }}>Pro plan</Typography>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12, mt: 0.2 }}>Resets in 12 days</Typography>
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 12, mt: 1 }}>Agent minutes</Typography>
        <Typography sx={{ fontSize: 13, fontWeight: 800 }}>2,420 / 5,000</Typography>
        <Box sx={{ bgcolor: `${tokens.color.text.primary}12`, borderRadius: 999, height: 4, mt: 0.8 }}>
          <Box sx={{ bgcolor: tokens.color.accent.purple, borderRadius: 999, height: 4, width: "48%" }} />
        </Box>
      </Box>
    </Box>
  );
}

function TopBar() {
  return (
    <Stack
      direction="row"
      sx={{
        alignItems: "center",
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
        gap: 1,
        minHeight: 64,
        minWidth: 0,
        px: { xs: 1.2, md: 1.8 },
      }}
    >
      <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13, minWidth: 0 }}>
        Studio
      </Typography>
      <KeyboardArrowRightRounded sx={{ color: tokens.color.text.muted, fontSize: 17 }} />
      <Typography sx={{ fontSize: 13, fontWeight: 800, minWidth: 0 }}>ClientFlow</Typography>
      <KeyboardArrowRightRounded sx={{ color: tokens.color.text.muted, fontSize: 17 }} />
      <Box sx={{ flex: 1 }} />
      <Button component={Link} href="/resources" size="small" startIcon={<GitHub />} sx={{ display: { xs: "none", md: "inline-flex" }, border: `1px solid ${tokens.color.border.strong}` }}>
        GitHub
      </Button>
      <Button component="a" href="#studio-preview" size="small" startIcon={<PlayArrowRounded />} sx={{ display: { xs: "none", sm: "inline-flex" }, border: `1px solid ${tokens.color.border.strong}` }}>
        Preview live
      </Button>
      <Button component={Link} href="/deploy" size="small" variant="contained" endIcon={<RocketLaunchRounded />}>
        Publish
      </Button>
    </Stack>
  );
}

function ModeTabs({ mode, onChange }: { mode: StudioMode; onChange: (next: StudioMode) => void }) {
  const tabs = [
    { key: "preview" as const, label: "Preview", icon: LaptopMacRounded },
    { key: "mobile" as const, label: "Mobile", icon: PhoneIphoneRounded },
    { key: "code" as const, label: "Code", icon: CodeRounded },
  ];
  return (
    <Stack direction="row" sx={{ gap: 0.8, minWidth: 0, overflowX: "auto", pb: 0.1 }}>
      {tabs.map((tab) => {
        const Icon = tab.icon;
        const active = mode === tab.key;
        return (
          <Button
            key={tab.key}
            size="small"
            onClick={() => onChange(tab.key)}
            startIcon={<Icon sx={{ fontSize: 16 }} />}
            sx={{
              border: `1px solid ${active ? tokens.color.border.accent : tokens.color.border.subtle}`,
              bgcolor: active ? `${tokens.color.accent.purple}70` : `${tokens.color.bg.surfaceRaised}80`,
              color: tokens.color.text.primary,
              flex: "0 0 auto",
              minWidth: 104,
            }}
          >
            {tab.label}
          </Button>
        );
      })}
    </Stack>
  );
}

function StatusCards() {
  const cards = [
    ["Plan", "Locked"],
    ["Web", "Live"],
    ["Mobile", "Queued"],
    ["Gate", "92/100"],
    ["Deploy", "Preview"],
  ];
  return (
    <Box
      sx={{
        display: "grid",
        gap: 0.8,
        gridTemplateColumns: { xs: "repeat(2, minmax(0, 1fr))", sm: "repeat(5, minmax(92px, 1fr))" },
        minWidth: 0,
      }}
    >
      {cards.map(([label, value]) => (
        <Box
          key={label}
          sx={{
            border: `1px solid ${label === "Gate" ? tokens.color.border.accent : tokens.color.border.subtle}`,
            borderRadius: `${tokens.radius.sm}px`,
            bgcolor: label === "Gate" ? `${tokens.color.accent.purple}55` : `${tokens.color.bg.inset}b0`,
            minWidth: 0,
            px: 1.2,
            py: 0.8,
          }}
        >
          <Typography sx={{ color: tokens.color.text.secondary, fontSize: 11 }}>{label}</Typography>
          <Typography sx={{ fontSize: 14, fontWeight: 900 }}>{value}</Typography>
        </Box>
      ))}
    </Box>
  );
}

function PromptPanel({
  prompt,
  creating,
  error,
  onPromptChange,
  onImprove,
  onGenerate,
  onSuggestion,
}: {
  prompt: string;
  creating: boolean;
  error: string | null;
  onPromptChange: (next: string) => void;
  onImprove: (nextPrompt: string) => void;
  onGenerate: () => void;
  onSuggestion: (text: string) => void;
}) {
  const suggestions = [
    "Add subscription billing flow with stripe",
    "Add team chat and mentions",
    "Add client portal notifications",
    "Add advanced permissions",
  ];
  return (
    <Stack
      spacing={1.15}
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: `${tokens.radius.md}px`,
        bgcolor: `${tokens.color.bg.surfaceRaised}bf`,
        gridRow: { lg: 1 },
        minHeight: { xs: 420, lg: 0 },
        minWidth: 0,
        overflow: { xs: "visible", lg: "hidden" },
        p: 1.2,
      }}
    >
      <Stack direction="row" sx={{ alignItems: "center", gap: 0.8 }}>
        <Typography sx={{ fontSize: 16, fontWeight: 900 }}>AI Prompt</Typography>
        <Box sx={{ flex: 1 }} />
        <Button
          size="small"
          onClick={() => {
            const base = prompt.trim() || DEFAULT_PROMPT;
            onImprove(`${base}\nInclude authenticated roles, billing events, approval states, mobile breakpoints, deploy gates, and an admin dashboard.`);
          }}
          startIcon={<AutoAwesomeRounded sx={{ fontSize: 14 }} />}
          sx={{ border: `1px solid ${tokens.color.border.subtle}`, color: tokens.color.text.secondary, minHeight: 30 }}
        >
          Improve prompt
        </Button>
      </Stack>
      <Box
        component="textarea"
        value={prompt}
        onChange={(event: ChangeEvent<HTMLTextAreaElement>) => onPromptChange(event.target.value)}
        aria-label="New project prompt"
        sx={{
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.strong}`,
          borderRadius: `${tokens.radius.sm}px`,
          color: tokens.color.text.primary,
          flex: "0 0 156px",
          fontFamily: tokens.font.family,
          fontSize: 13,
          lineHeight: 1.55,
          minWidth: 0,
          outline: "none",
          p: 1.2,
          resize: "vertical",
          width: "100%",
        }}
      />
      <Stack direction="row" flexWrap="wrap" useFlexGap sx={{ gap: 0.65 }}>
        {["Add context", "Business goals", "Data models"].map((chip) => (
          <Chip
            key={chip}
            size="small"
            label={chip}
            sx={{
              bgcolor: chip === "Business goals" ? `${tokens.color.accent.warning}18` : `${tokens.color.text.primary}0c`,
              color: chip === "Business goals" ? tokens.color.accent.warning : tokens.color.text.secondary,
              fontSize: 11,
            }}
          />
        ))}
      </Stack>
      <Button fullWidth variant="contained" disabled={!prompt.trim() || creating} onClick={onGenerate} endIcon={creating ? <CircularProgress size={14} sx={{ color: tokens.color.text.primary }} /> : <AutoAwesomeRounded />}>
        Generate
      </Button>
      {error ? <Alert severity="error" variant="outlined" sx={{ color: tokens.color.text.primary }}>{error}</Alert> : null}
      <Typography sx={{ color: tokens.color.text.muted, fontSize: 12, mt: 0.4 }}>Suggestions</Typography>
      <Box sx={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 0.8, minHeight: 0, overflow: "auto" }}>
        {suggestions.map((item) => (
          <Button
            key={item}
            onClick={() => onSuggestion(item)}
            sx={{
              alignItems: "flex-start",
              border: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: `${tokens.color.bg.inset}9c`,
              color: tokens.color.text.primary,
              fontSize: 12,
              justifyContent: "space-between",
              minHeight: 74,
              p: 1.1,
              textAlign: "left",
            }}
            endIcon={<AddRounded />}
          >
            {item}
          </Button>
        ))}
      </Box>
    </Stack>
  );
}

function CodeWorkbench({ mode }: { mode: StudioMode }) {
  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: `${tokens.radius.md}px`,
        bgcolor: `${tokens.color.bg.inset}d6`,
        display: "flex",
        flexDirection: "column",
        gridRow: { lg: 1 },
        minHeight: { xs: 460, lg: 0 },
        minWidth: 0,
        overflow: "hidden",
      }}
    >
      <Stack direction="row" sx={{ alignItems: "center", borderBottom: `1px solid ${tokens.color.border.subtle}`, gap: 1, minHeight: 54, px: 1.6 }}>
        <Typography sx={{ fontSize: 16, fontWeight: 900 }}>Code</Typography>
        <Box sx={{ flex: 1 }} />
        {[ContentCopyRounded, FolderRounded, OpenInFullRounded].map((Icon, index) => (
          <IconButton key={index} size="small" disabled><Icon sx={{ fontSize: 16 }} /></IconButton>
        ))}
      </Stack>
      <Box sx={{ display: "grid", flex: 1, gridTemplateColumns: { xs: "1fr", lg: "136px minmax(0, 1fr)" }, minHeight: 0, minWidth: 0 }}>
        <Box sx={{ borderRight: { lg: `1px solid ${tokens.color.border.subtle}` }, display: { xs: "none", lg: "block" }, p: 1.4 }}>
          <Typography sx={{ color: tokens.color.text.secondary, fontFamily: tokens.font.mono, fontSize: 12 }}>Files</Typography>
          <Stack spacing={0.75} sx={{ mt: 1.2 }}>
            {["src", "components", "pages", "hooks", "lib", "styles", ".env.local", "package.json", "README.md"].map((file, index) => (
              <Stack key={file} direction="row" sx={{ alignItems: "center", gap: 0.7 }}>
                <FolderRounded sx={{ color: index === 0 ? tokens.color.accent.violet : tokens.color.text.muted, fontSize: 16 }} />
                <Typography sx={{ color: tokens.color.text.secondary, fontSize: 12.5 }}>{file}</Typography>
              </Stack>
            ))}
          </Stack>
        </Box>
        <Box sx={{ display: "flex", flexDirection: "column", minHeight: 0, minWidth: 0 }}>
          <Stack direction="row" sx={{ alignItems: "center", borderBottom: `1px solid ${tokens.color.border.subtle}`, gap: 0.8, px: 1.2, py: 0.9 }}>
            <CodeRounded sx={{ color: tokens.color.accent.sky, fontSize: 17 }} />
            <Typography sx={{ fontSize: 13, fontWeight: 900 }}>{mode === "mobile" ? "MobilePreview.tsx" : "Dashboard.tsx"}</Typography>
            <Box sx={{ flex: 1 }} />
            <IconButton size="small" disabled><MoreHorizRounded /></IconButton>
          </Stack>
          <Box component="pre" sx={{ color: tokens.color.text.secondary, flex: 1, fontFamily: tokens.font.mono, fontSize: { xs: 11, md: 12.5 }, lineHeight: 1.7, m: 0, minHeight: 0, overflow: "auto", p: 1.4 }}>
            {CODE_LINES.map((line, index) => (
              <Box component="code" key={`${line}-${index}`} sx={{ display: "block", whiteSpace: { xs: "pre-wrap", md: "pre" }, wordBreak: { xs: "break-word", md: "normal" } }}>
                <Box component="span" sx={{ color: tokens.color.text.muted, display: "inline-block", pr: 1.5, textAlign: "right", width: 28 }}>{index + 1}</Box>
                <Box component="span" sx={{ color: line.includes("import") || line.includes("export") ? tokens.color.accent.violet : line.includes("Stat") ? tokens.color.accent.success : tokens.color.text.secondary }}>{line || " "}</Box>
              </Box>
            ))}
          </Box>
          <Stack direction="row" sx={{ alignItems: "center", borderTop: `1px solid ${tokens.color.border.subtle}`, color: tokens.color.text.secondary, fontFamily: tokens.font.mono, fontSize: 11, gap: 1, px: 1.2, py: 0.8 }}>
            TypeScript
            <Box sx={{ width: 6, height: 6, borderRadius: "50%", bgcolor: tokens.color.accent.success }} />
            No errors
            <Box sx={{ flex: 1 }} />
            Ln 14, Col 22
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}

function PreviewWorkbench({
  mode,
  onModeChange,
}: {
  mode: StudioMode;
  onModeChange: (next: StudioMode) => void;
}) {
  const mobile = mode === "mobile";
  const metrics = [
    {
      label: "Revenue",
      value: "$18.2k",
      accent: tokens.color.accent.violet,
      icon: HubRounded,
      path: "M2 30 C16 30 20 22 29 20 C38 18 42 8 56 12",
    },
    {
      label: "Open approvals",
      value: "27",
      accent: tokens.color.accent.coral,
      icon: DashboardRounded,
      path: "M2 26 C9 20 15 27 22 22 C29 17 34 22 40 16 C47 9 51 8 56 20",
    },
    {
      label: "Files",
      value: "1.4k",
      accent: tokens.color.accent.sky,
      icon: FolderRounded,
      path: "M2 28 C12 24 17 31 25 24 C32 18 37 29 44 20 C50 11 53 9 56 18",
    },
    {
      label: "Deploy health",
      value: "99.9%",
      accent: tokens.color.accent.success,
      icon: RocketLaunchRounded,
      path: "M2 30 C12 28 17 24 24 25 C33 27 37 17 45 14 C51 12 54 19 56 24",
    },
  ];
  return (
    <Box id="studio-preview" sx={{ border: `1px solid ${tokens.color.border.subtle}`, borderRadius: `${tokens.radius.md}px`, bgcolor: `${tokens.color.bg.surfaceRaised}c9`, display: { xs: "block", lg: "flex" }, flexDirection: "column", gridRow: { lg: 1 }, minHeight: { xs: 430, lg: 0 }, minWidth: 0, overflow: "hidden" }}>
      <Stack direction="row" sx={{ alignItems: "center", borderBottom: `1px solid ${tokens.color.border.subtle}`, gap: 1, px: 1.3, py: 1 }}>
        <Typography sx={{ fontSize: 16, fontWeight: 900 }}>Preview</Typography>
        <Box sx={{ flex: 1 }} />
        <Stack direction="row" sx={{ bgcolor: `${tokens.color.bg.inset}b8`, border: `1px solid ${tokens.color.border.subtle}`, borderRadius: `${tokens.radius.sm}px`, gap: 0.4, p: 0.35 }}>
          {[
            { icon: LaptopMacRounded, label: "Desktop preview", next: "preview" as const, active: !mobile },
            { icon: TabletMacRounded, label: "Tablet preview", next: "preview" as const, active: false },
            { icon: PhoneIphoneRounded, label: "Mobile preview", next: "mobile" as const, active: mobile },
          ].map(({ icon: Icon, label, next, active }) => (
            <IconButton
              key={label}
              aria-label={label}
              onClick={() => onModeChange(next)}
              size="small"
              sx={{
                bgcolor: active ? `${tokens.color.accent.purple}55` : "transparent",
                height: 28,
                width: 32,
              }}
            >
              <Icon sx={{ fontSize: 15 }} />
            </IconButton>
          ))}
        </Stack>
        <IconButton size="small" onClick={() => onModeChange("preview")}><RefreshRounded sx={{ fontSize: 17 }} /></IconButton>
        <IconButton size="small" disabled><MoreHorizRounded /></IconButton>
      </Stack>
      <Box sx={{ p: 1.4, minWidth: 0, overflow: "auto" }}>
        <Box sx={{ mx: mobile ? "auto" : 0, maxWidth: mobile ? 300 : "none", border: `1px solid ${tokens.color.border.strong}`, borderRadius: `${tokens.radius.md}px`, bgcolor: tokens.color.bg.inset, boxShadow: `inset 0 1px 0 ${tokens.color.text.primary}10`, overflow: "hidden" }}>
          <Stack direction="row" sx={{ alignItems: "center", gap: 1, px: 1.4, py: 1.2 }}>
            <RocketLaunchRounded sx={{ color: tokens.color.accent.violet, fontSize: 18 }} />
            <Typography sx={{ flex: 1, fontSize: 15, fontWeight: 900 }}>Client operations portal</Typography>
            <Chip size="small" label="Live" sx={{ bgcolor: `${tokens.color.accent.success}22`, color: tokens.color.accent.success }} />
            <IconButton size="small" disabled sx={{ height: 26, width: 26 }}><MoreHorizRounded sx={{ fontSize: 16 }} /></IconButton>
          </Stack>
          <Box sx={{ display: "grid", gap: 1, gridTemplateColumns: mobile ? "1fr" : "repeat(2, minmax(0, 1fr))", px: 1.4 }}>
            {metrics.map(({ label, value, accent, icon: Icon, path }) => (
              <Box
                key={label}
                sx={{
                  border: `1px solid ${tokens.color.border.subtle}`,
                  borderRadius: `${tokens.radius.sm}px`,
                  bgcolor: `${tokens.color.text.primary}08`,
                  p: 1.05,
                  transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}, background-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                  "&:hover": {
                    bgcolor: `${tokens.color.text.primary}0c`,
                    borderColor: `${accent}72`,
                  },
                }}
              >
                <Stack direction="row" sx={{ alignItems: "center", gap: 0.7 }}>
                  <Box sx={{ alignItems: "center", bgcolor: `${accent}1f`, borderRadius: `${tokens.radius.sm}px`, color: accent, display: "flex", height: 24, justifyContent: "center", width: 24 }}>
                    <Icon sx={{ fontSize: 15 }} />
                  </Box>
                  <Typography sx={{ color: tokens.color.text.secondary, fontSize: 11 }}>{label}</Typography>
                </Stack>
                <Stack direction="row" sx={{ alignItems: "end", gap: 1 }}>
                  <Typography sx={{ flex: 1, fontSize: 22, fontWeight: 900 }}>{value}</Typography>
                  <MetricSparkline color={accent} path={path} />
                </Stack>
              </Box>
            ))}
          </Box>
          <Box sx={{ m: 1.4, border: `1px solid ${tokens.color.border.subtle}`, borderRadius: `${tokens.radius.sm}px`, overflow: "hidden" }}>
            <Stack direction="row" sx={{ alignItems: "center", borderBottom: `1px solid ${tokens.color.border.subtle}`, gap: 1, px: 1.1, py: 0.9 }}>
              <Typography sx={{ flex: 1, fontSize: 14, fontWeight: 900 }}>Projects</Typography>
              <Chip size="small" label="Live" sx={{ bgcolor: `${tokens.color.accent.success}22`, color: tokens.color.accent.success, fontWeight: 900 }} />
            </Stack>
            {!mobile ? (
              <Box sx={{ borderBottom: `1px solid ${tokens.color.border.subtle}`, display: "grid", gap: 1, gridTemplateColumns: "minmax(0,1.4fr) 1fr auto 1fr", px: 1.1, py: 0.7 }}>
                {["Project", "Owner", "Status", "Updated"].map((header) => (
                  <Typography key={header} sx={{ color: tokens.color.text.secondary, fontSize: 11 }}>{header}</Typography>
                ))}
              </Box>
            ) : null}
            {[
              ["Website redesign", "Maya P.", "Live", "2m ago"],
              ["Mobile app", "Noah K.", "Preview", "10m ago"],
              ["CRM integration", "Liam T.", "Live", "1h ago"],
              ["Analytics dashboard", "Emma R.", "Queued", "2h ago"],
            ].map((row, index) => (
              <Box key={row[0]} sx={{ borderBottom: index === 3 ? 0 : `1px solid ${tokens.color.border.subtle}`, display: "grid", gap: 1, gridTemplateColumns: mobile ? "1fr auto" : "minmax(0,1.4fr) 1fr auto 1fr", px: 1.1, py: 0.85 }}>
                <Typography sx={{ fontSize: 12.5, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{row[0]}</Typography>
                {!mobile ? <Typography sx={{ color: tokens.color.text.secondary, fontSize: 12 }}>{row[1]}</Typography> : null}
                <Typography sx={{ color: row[2] === "Live" ? tokens.color.accent.success : row[2] === "Preview" ? tokens.color.accent.violet : tokens.color.accent.warning, fontSize: 12, fontWeight: 900 }}>{row[2]}</Typography>
                {!mobile ? <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>{row[3]}</Typography> : null}
              </Box>
            ))}
          </Box>
        </Box>
      </Box>
    </Box>
  );
}

function MetricSparkline({ color, path }: { color: string; path: string }) {
  return (
    <Box
      component="svg"
      viewBox="0 0 58 34"
      aria-hidden="true"
      sx={{
        display: "block",
        flex: "0 0 58px",
        height: 34,
        overflow: "visible",
        width: 58,
      }}
    >
      <Box
        component="path"
        d={path}
        fill="none"
        stroke={color}
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2.2"
        sx={{
          filter: `drop-shadow(0 0 5px ${color}80)`,
        }}
      />
    </Box>
  );
}

function AssistantDock({
  assistantText,
  onSend,
}: {
  assistantText: string;
  onSend: (message: string) => void;
}) {
  const [draft, setDraft] = useState("");
  const done = [
    "Added project table with filters and search",
    "Connected Stripe billing flow",
    "Added role-based access control",
    "Added activity feed and notifications",
  ];
  return (
    <Box sx={{ border: `1px solid ${tokens.color.border.subtle}`, borderRadius: `${tokens.radius.md}px`, bgcolor: `${tokens.color.bg.surfaceRaised}cc`, display: "grid", gap: 1, gridColumn: { xs: "1", md: "1 / -1" }, gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1.1fr) minmax(300px, 0.9fr)" }, minWidth: 0, p: 1.2 }}>
      <Box sx={{ minWidth: 0 }}>
        <Stack direction="row" sx={{ alignItems: "center", gap: 0.8 }}>
          <AutoAwesomeRounded sx={{ color: tokens.color.accent.violet, fontSize: 18 }} />
          <Typography sx={{ fontSize: 16, fontWeight: 900 }}>Studio Assistant</Typography>
        </Stack>
        <Box sx={{ display: "grid", gap: 0.8, gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))", lg: "repeat(4, minmax(0, 1fr))" }, mt: 1 }}>
          {done.map((item) => (
            <Stack key={item} direction="row" sx={{ alignItems: "center", border: `1px solid ${tokens.color.border.subtle}`, borderRadius: `${tokens.radius.sm}px`, gap: 0.8, minWidth: 0, px: 1, py: 0.9 }}>
              <CheckRounded sx={{ color: tokens.color.accent.success, fontSize: 17 }} />
              <Typography sx={{ fontSize: 12, minWidth: 0 }}>{item}</Typography>
            </Stack>
          ))}
        </Box>
      </Box>
      <Box sx={{ border: `1px solid ${tokens.color.border.subtle}`, borderRadius: `${tokens.radius.md}px`, bgcolor: `${tokens.color.bg.inset}c7`, minWidth: 0, p: 1.2 }}>
        <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13 }}>{assistantText}</Typography>
        <Stack
          component="form"
          onSubmit={(event) => {
            event.preventDefault();
            const message = draft.trim();
            if (!message) return;
            onSend(message);
            setDraft("");
          }}
          direction="row"
          sx={{ alignItems: "center", gap: 0.8, mt: 1.2 }}
        >
          <Box
            component="input"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            placeholder="Ask anything..."
            aria-label="Studio assistant message"
            sx={{
              bgcolor: "transparent",
              border: 0,
              color: tokens.color.text.primary,
              flex: 1,
              font: "inherit",
              fontSize: 12,
              minWidth: 0,
              outline: "none",
              "&::placeholder": { color: tokens.color.text.muted },
            }}
          />
          <Box sx={{ flex: 1 }} />
          <IconButton
            type="submit"
            aria-label="Send assistant message"
            size="small"
            sx={{
              bgcolor: `${tokens.color.accent.purple}70`,
              opacity: draft.trim() ? 1 : 0.55,
            }}
          >
            <SendRounded sx={{ fontSize: 18 }} />
          </IconButton>
        </Stack>
      </Box>
    </Box>
  );
}
