"use client";

// DashboardPage — the authenticated cockpit landing surface.
//
// Layout (top → bottom):
//   1. Page header
//   2. Hero strip      Wallet · Recent margin · Active runs
//   3. "Start a new app" card (inline prompt textarea + Build button)
//   4. Bandit "How we route work" mini-card
//   5. Projects grid (bulk select + delete)
//   6. Recent runs table
//   7. 24h cost sparkline
//
// GraphQL operations consumed:
//   - query Wallet               (generated)
//   - query Executions           (generated)
//   - query DashboardBandit      (inline gql — `banditRanking`)
//   - query DashboardProjects    (inline gql — `projects`)
//   - query DashboardVault       (inline gql — `vault`)
//   - query DashboardBudget      (inline gql — `myBudget`)
//   - mutation DashboardCreateProject       (inline gql — `createProject`)
//   - mutation DashboardBulkDeleteProjects  (inline gql)

import { useApolloClient } from "@apollo/client";
// `useApolloClient` is still needed for `apollo.refetchQueries({ include })`
// after bulk delete — there's no generated equivalent for that imperative
// cache surgery.
import {
  AutoAwesomeRounded,
  ChevronRightRounded,
  DeleteOutlineRounded,
  FolderOpenRounded,
  OpenInNewRounded,
  RefreshRounded,
  WorkspacesOutlined,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Checkbox,
  CircularProgress,
  IconButton,
  Skeleton,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import * as swal from "../lib/swal";
import { useRouter } from "next/navigation";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type KeyboardEvent,
} from "react";
import { useAuth } from "../lib/auth";
import { extractErrorMessage } from "../lib/errors";
import { formatMoney, formatNumber } from "../lib/format";
import {
  useBanditRankingQuery,
  useBulkDeleteProjectsMutation,
  useCreateProjectMutation,
  useDashboardProjectsQuery,
  useExecutionsQuery,
  useMyBudgetQuery,
  useVaultQuery,
  useWalletQuery,
  type ExecutionsQuery,
  DashboardProjectsDocument,
} from "../lib/gql/__generated__";
import { relativeTime } from "../lib/relativeTime";
import { tokens } from "../theme";
import {
  EmptyState,
  ErrorPanel,
  LoadingPanel,
  MetricCard,
  PageHeader,
  StatusBadge,
} from "./cockpit";
import dynamic from "next/dynamic";
import { SparklineSVG, type SparklinePoint } from "./SparklineSVG";

const SpendBars = dynamic(
  () => import("./charts/SpendBars").then((m) => m.SpendBars),
  { ssr: false, loading: () => <Box sx={{ height: 160 }} /> },
);

// Inline gql operations were retired in favour of operations/*.graphql +
// codegen. The local DashboardProject alias stays so the rest of the
// file (selection logic, projectsById Map) keeps its narrow row type.
import type { DashboardProjectsQuery } from "../lib/gql/__generated__";
type DashboardProject = DashboardProjectsQuery["projects"][number];

const ACTIVE_RUN_STATUSES = new Set(["created", "admitted", "started", "running"]);

// deriveProjectName — inline so this file doesn't depend on src/lib helpers
// the Foundation agent owns. Same rules as the deleted projectNaming.ts:
// trim, drop filler words, title-case the first 5 tokens, cap at 60 chars.
const NAME_FILLERS = new Set([
  "a",
  "an",
  "the",
  "build",
  "make",
  "create",
  "please",
  "i",
  "want",
  "need",
]);
function deriveProjectName(prompt: string | null | undefined): string {
  if (!prompt) return "Untitled app";
  const cleaned = prompt.replace(/[\r\n\t]+/g, " ").replace(/\s+/g, " ").trim();
  if (!cleaned) return "Untitled app";
  const tokens = cleaned.split(" ").slice(0, 8);
  let i = 0;
  while (i < tokens.length && NAME_FILLERS.has(tokens[i].toLowerCase())) i++;
  const head = tokens
    .slice(i, i + 5)
    .map((w) =>
      w.length <= 4 && w === w.toUpperCase()
        ? w
        : w.charAt(0).toUpperCase() + w.slice(1).toLowerCase(),
    );
  let name = head.join(" ").trim().replace(/[\s.,;:!?-]+$/g, "");
  if (!name) return "Untitled app";
  if (name.length <= 60) return name;
  const cut = name.slice(0, 60);
  const lastSpace = cut.lastIndexOf(" ");
  name = (lastSpace > 30 ? cut.slice(0, lastSpace) : cut).trim();
  return name || "Untitled app";
}

export function DashboardPage() {
  const router = useRouter();
  const { authenticated, loading: authLoading, user } = useAuth();

  // Redirect unauthenticated visitors back to /login with returnTo set.
  useEffect(() => {
    if (authLoading) return;
    if (!authenticated) {
      const to = encodeURIComponent("/dashboard");
      router.replace(`/login?returnTo=${to}`);
    }
  }, [authenticated, authLoading, router]);

  const apollo = useApolloClient();

  const skipUntilAuth = !authenticated;
  const walletQ = useWalletQuery({ skip: skipUntilAuth });
  const projectsQ = useDashboardProjectsQuery({ skip: skipUntilAuth });
  const executionsQ = useExecutionsQuery({
    skip: skipUntilAuth,
    variables: { limit: 20, offset: 0 },
  });
  const vaultQ = useVaultQuery({ skip: skipUntilAuth });
  const budgetQ = useMyBudgetQuery({ skip: skipUntilAuth });
  const banditQ = useBanditRankingQuery({
    skip: skipUntilAuth,
    variables: { lookback: 24 },
  });

  const [createProject, createProjectM] = useCreateProjectMutation();
  const [bulkDelete, bulkDeleteM] = useBulkDeleteProjectsMutation();

  // ----- "Start a new app" prompt state -----
  const [prompt, setPrompt] = useState("");
  const [planMode, setPlanMode] = useState(false);
  const [promptError, setPromptError] = useState<string | null>(null);

  const handleSubmitPrompt = useCallback(async () => {
    setPromptError(null);
    const cleaned = prompt.trim();
    if (!cleaned) return;
    try {
      const name = deriveProjectName(cleaned);
      const idea = planMode ? `[PLAN]\n${cleaned}` : cleaned;
      const res = await createProject({ variables: { input: { name, idea } } });
      if (res.errors && res.errors.length > 0) {
        throw new Error(
          res.errors.map((e) => e.message).join("\n") ||
            "Backend rejected the request.",
        );
      }
      const id = res.data?.createProject.id;
      if (!id) throw new Error("Backend did not return a project id.");
      router.push(`/studio/${encodeURIComponent(id)}?autorun=1`);
    } catch (err) {
      setPromptError(extractErrorMessage(err));
    }
  }, [createProject, planMode, prompt, router]);

  // ----- selection / bulk delete -----
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const projects = useMemo(() => {
    const list = projectsQ.data?.projects ?? [];
    return [...list].sort(
      (a, b) =>
        new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
    );
  }, [projectsQ.data]);

  const projectsById = useMemo(() => {
    const map = new Map<string, DashboardProject>();
    for (const p of projects) map.set(p.id, p);
    return map;
  }, [projects]);

  const toggleSelected = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const clearSelection = () => setSelected(new Set());

  const handleConfirmDelete = useCallback(async () => {
    if (selected.size === 0) return;
    try {
      await bulkDelete({ variables: { ids: Array.from(selected) } });
      void swal.toast(
        `Deleted ${selected.size} project${selected.size === 1 ? "" : "s"}.`,
        "success",
      );
      clearSelection();
      setConfirmingDelete(false);
      await apollo.refetchQueries({ include: [DashboardProjectsDocument] });
    } catch (err) {
      void swal.error("Delete failed", extractErrorMessage(err));
      setConfirmingDelete(false);
    }
  }, [apollo, bulkDelete, selected]);

  // ----- derived metrics -----
  const wallet = walletQ.data?.wallet;
  const vault = vaultQ.data?.vault;
  const executions = executionsQ.data?.executions ?? [];
  const activeRuns = useMemo(
    () => executions.filter((e) => ACTIVE_RUN_STATUSES.has(e.status)).length,
    [executions],
  );

  // 24h hourly spend buckets from myBudget.entries.
  const hourlySpend = useMemo<SparklinePoint[]>(() => {
    const entries = budgetQ.data?.myBudget.entries ?? [];
    const now = Date.now();
    const bucketMs = 60 * 60 * 1000;
    const buckets: SparklinePoint[] = [];
    for (let i = 23; i >= 0; i--) {
      const start = now - i * bucketMs - bucketMs;
      buckets.push({ ts: new Date(start).toISOString(), value: 0 });
    }
    for (const e of entries) {
      const t = new Date(e.ts).getTime();
      if (Number.isNaN(t)) continue;
      const diffH = Math.floor((now - t) / bucketMs);
      if (diffH < 0 || diffH > 23) continue;
      const idx = 23 - diffH;
      buckets[idx].value += Number(e.costUsd) || 0;
    }
    return buckets;
  }, [budgetQ.data]);

  const total24h = useMemo(
    () => hourlySpend.reduce((s, p) => s + p.value, 0),
    [hourlySpend],
  );

  const banditTopRoutes = useMemo(() => {
    const caps = banditQ.data?.banditRanking.capabilities ?? [];
    type Row = {
      provider: string;
      model: string;
      share: number;
      capability: string;
      capabilities: string[];
    };
    const merged = new Map<string, Row>();
    for (const c of caps) {
      const leader = c.winners.find((w) => w.isLeader) ?? c.winners[0];
      if (!leader) continue;
      const key = `${leader.provider}/${leader.model ?? "—"}`;
      const existing = merged.get(key);
      if (!existing) {
        merged.set(key, {
          provider: leader.provider,
          model: leader.model ?? "—",
          share: leader.share,
          capability: c.capability,
          capabilities: [c.capability],
        });
      } else {
        existing.share = Math.max(existing.share, leader.share);
        existing.capabilities.push(c.capability);
      }
    }
    return Array.from(merged.values())
      .sort((a, b) => b.share - a.share)
      .slice(0, 3);
  }, [banditQ.data]);

  if (authLoading || !authenticated) {
    return (
      <>
        <PageHeader title="Dashboard" />
        <LoadingPanel label="Loading your cockpit" />
      </>
    );
  }

  const marginNum = vault ? Number(vault.marginUsd) : null;
  const marginDisplay =
    marginNum === null || !Number.isFinite(marginNum) || marginNum === 0
      ? "—"
      : formatMoney(marginNum);

  return (
    <Box>
      <PageHeader
        title="Dashboard"
        eyebrow="cockpit"
        description={
          user
            ? `Welcome back, ${user.name || user.email}. Below is your wallet, your projects, your recent runs, and how Ironflyer is routing work right now.`
            : undefined
        }
      />

      <Stack spacing={4} sx={{ pb: 6 }}>
        {/* Hero metric strip */}
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(3, 1fr)",
            },
          }}
        >
          {walletQ.loading ? (
            <Skeleton variant="rectangular" height={120} sx={skelSx} />
          ) : (
            <Box
              onClick={() => router.push("/wallet")}
              sx={{ cursor: "pointer", "&:hover": { opacity: 0.92 } }}
              role="button"
              tabIndex={0}
              onKeyDown={(e: KeyboardEvent<HTMLDivElement>) => {
                if (e.key === "Enter") router.push("/wallet");
              }}
            >
              <MetricCard
                label="Wallet available"
                value={formatMoney(wallet?.availableUSD ?? 0)}
                hint="Click to top up"
                accent="lime"
              />
            </Box>
          )}
          {vaultQ.loading ? (
            <Skeleton variant="rectangular" height={120} sx={skelSx} />
          ) : (
            <MetricCard
              label="Recent margin"
              value={marginDisplay}
              hint="revenue − provider cost"
              accent="sky"
            />
          )}
          {executionsQ.loading ? (
            <Skeleton variant="rectangular" height={120} sx={skelSx} />
          ) : (
            <MetricCard
              label="Active runs"
              value={formatNumber(activeRuns)}
              hint="created · admitted · started"
              accent="purple"
            />
          )}
        </Box>

        {/* New app prompt */}
        <NewAppCard
          value={prompt}
          onChange={setPrompt}
          planMode={planMode}
          onPlanModeChange={setPlanMode}
          onSubmit={handleSubmitPrompt}
          submitting={createProjectM.loading}
          errorMessage={promptError}
        />

        {/* Bandit routing card */}
        <BanditRoutingCard
          loading={banditQ.loading}
          rows={banditTopRoutes}
          lookback={banditQ.data?.banditRanking.lookback ?? 24}
        />

        {/* Projects */}
        <ProjectsSection
          loading={projectsQ.loading}
          error={projectsQ.error}
          projects={projects}
          selected={selected}
          onToggleSelected={toggleSelected}
          onClearSelection={clearSelection}
          onAskDelete={() => setConfirmingDelete(true)}
          deleting={bulkDeleteM.loading}
          onOpen={(id) => router.push(`/studio/${encodeURIComponent(id)}`)}
          onRefresh={() => projectsQ.refetch()}
        />

        {/* Recent runs */}
        <RecentRunsTable
          loading={executionsQ.loading}
          error={executionsQ.error}
          rows={executions.slice(0, 10)}
          projectsById={projectsById}
          onOpen={(projectID) =>
            router.push(`/studio/${encodeURIComponent(projectID)}`)
          }
        />

        {/* 24h cost sparkline */}
        <CostStripCard
          loading={budgetQ.loading}
          points={hourlySpend}
          total24h={total24h}
        />
      </Stack>

      <ConfirmDeleteBar
        open={confirmingDelete}
        count={selected.size}
        onCancel={() => setConfirmingDelete(false)}
        onConfirm={handleConfirmDelete}
        busy={bulkDeleteM.loading}
      />

    </Box>
  );
}

const skelSx = {
  bgcolor: tokens.color.bg.surfaceHover,
  borderRadius: 1,
};

// ============================================================
// New app card — inline prompt textarea + Build button
// ============================================================

function NewAppCard({
  value,
  onChange,
  planMode,
  onPlanModeChange,
  onSubmit,
  submitting,
  errorMessage,
}: {
  value: string;
  onChange: (s: string) => void;
  planMode: boolean;
  onPlanModeChange: (b: boolean) => void;
  onSubmit: () => void;
  submitting: boolean;
  errorMessage: string | null;
}) {
  const canSubmit = value.trim().length > 0 && !submitting;
  const handleKey = useCallback(
    (e: KeyboardEvent<HTMLDivElement>) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
        e.preventDefault();
        if (canSubmit) onSubmit();
      }
    },
    [canSubmit, onSubmit],
  );

  return (
    <Card sx={{ p: { xs: 2.5, md: 3 } }}>
      <Stack
        direction={{ xs: "column", sm: "row" }}
        alignItems={{ xs: "flex-start", sm: "baseline" }}
        spacing={1}
        sx={{ mb: 2 }}
      >
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 20,
            fontWeight: 800,
          }}
        >
          Start a new app
        </Typography>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 13.5 }}>
          Describe what you want shipped. ⌘+Enter to send.
        </Typography>
      </Stack>

      <TextField
        fullWidth
        multiline
        minRows={4}
        maxRows={10}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKey}
        placeholder="Describe the product you want shipped. Be specific about users, screens, money flow, and integrations."
        disabled={submitting}
        InputProps={{
          sx: {
            fontSize: { xs: 15, md: 16 },
            fontWeight: 500,
            lineHeight: 1.5,
          },
        }}
      />

      <Stack
        direction={{ xs: "column", sm: "row" }}
        alignItems={{ xs: "stretch", sm: "center" }}
        spacing={1.5}
        sx={{ mt: 2 }}
      >
        <Stack
          direction="row"
          alignItems="center"
          spacing={0.75}
          component="label"
          sx={{ cursor: submitting ? "not-allowed" : "pointer", userSelect: "none" }}
        >
          <Switch
            size="small"
            checked={planMode}
            onChange={(_, c) => onPlanModeChange(c)}
            disabled={submitting}
          />
          <Typography sx={{ color: tokens.color.text.primary, fontSize: 13.5, fontWeight: 700 }}>
            Plan mode
          </Typography>
          <Tooltip title="Runs a planner pass before code.">
            <AutoAwesomeRounded sx={{ color: tokens.color.text.muted, fontSize: 14 }} />
          </Tooltip>
        </Stack>

        <Box sx={{ flex: 1 }} />

        <Button
          variant="contained"
          color="primary"
          disabled={!canSubmit}
          onClick={onSubmit}
          startIcon={
            submitting ? (
              <CircularProgress size={16} sx={{ color: tokens.color.text.inverse }} />
            ) : undefined
          }
          endIcon={!submitting ? <ChevronRightRounded /> : undefined}
        >
          {submitting ? "Creating workspace…" : planMode ? "Plan it" : "Build it"}
        </Button>
      </Stack>

      {errorMessage && (
        <Box sx={{ mt: 2 }}>
          <ErrorPanel error={errorMessage} title="Could not create project" />
        </Box>
      )}
    </Card>
  );
}

// ============================================================
// Bandit "How we route work"
// ============================================================

function BanditRoutingCard({
  loading,
  rows,
  lookback,
}: {
  loading: boolean;
  rows: Array<{
    provider: string;
    model: string;
    share: number;
    capability: string;
    capabilities: string[];
  }>;
  lookback: number;
}) {
  const sentence =
    rows.length === 0
      ? "No routing data yet — your first run will populate this."
      : `Last ${lookback}h, prompts routed to ` +
        rows
          .map(
            (r) =>
              `${prettyModelName(r.provider, r.model)} (${Math.round(r.share * 100)}%)`,
          )
          .join(", ") +
        ".";

  return (
    <Card sx={{ p: 2.5 }}>
      <Stack direction="row" alignItems="baseline" spacing={1} sx={{ mb: 1 }}>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.secondary }}
        >
          How we route work
        </Typography>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
          bandit · live
        </Typography>
      </Stack>
      {loading ? (
        <Skeleton width="80%" sx={skelSx} />
      ) : (
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 15,
            fontWeight: 600,
          }}
        >
          {sentence}
        </Typography>
      )}
    </Card>
  );
}

function prettyModelName(provider: string, model: string): string {
  if (!model || model === "—") return capitalize(provider);
  return `${capitalize(provider)} ${model}`;
}
function capitalize(s: string): string {
  if (!s) return s;
  return s.charAt(0).toUpperCase() + s.slice(1);
}

// ============================================================
// Projects grid
// ============================================================

function ProjectsSection({
  loading,
  error,
  projects,
  selected,
  onToggleSelected,
  onClearSelection,
  onAskDelete,
  deleting,
  onOpen,
  onRefresh,
}: {
  loading: boolean;
  error: unknown;
  projects: DashboardProject[];
  selected: Set<string>;
  onToggleSelected: (id: string) => void;
  onClearSelection: () => void;
  onAskDelete: () => void;
  deleting: boolean;
  onOpen: (id: string) => void;
  onRefresh: () => void;
}) {
  return (
    <Box>
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 1.5 }}>
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 18,
            fontWeight: 800,
          }}
        >
          Projects
        </Typography>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
          {projects.length === 0 ? "" : `${projects.length} total`}
        </Typography>
        <Box sx={{ flex: 1 }} />
        {selected.size > 0 && (
          <>
            <Typography sx={{ color: tokens.color.text.secondary, fontSize: 13 }}>
              {selected.size} selected
            </Typography>
            <Button size="small" onClick={onClearSelection} variant="text">
              Clear
            </Button>
            <Button
              size="small"
              variant="contained"
              color="error"
              startIcon={<DeleteOutlineRounded />}
              onClick={onAskDelete}
              disabled={deleting}
            >
              Delete
            </Button>
          </>
        )}
        <Tooltip title="Refresh">
          <IconButton onClick={onRefresh}>
            <RefreshRounded fontSize="small" />
          </IconButton>
        </Tooltip>
      </Stack>

      {error ? (
        <ErrorPanel error={error} title="Could not load projects" onRetry={onRefresh} />
      ) : loading ? (
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr", lg: "repeat(3, 1fr)" },
          }}
        >
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} variant="rectangular" height={150} sx={skelSx} />
          ))}
        </Box>
      ) : projects.length === 0 ? (
        <EmptyState
          icon={<FolderOpenRounded sx={{ fontSize: 36 }} />}
          title="No projects yet"
          body="Write your first prompt above and Ironflyer will spin up a workspace, route a model, and start shipping."
        />
      ) : (
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "1fr 1fr",
              lg: "repeat(3, 1fr)",
            },
          }}
        >
          {projects.map((p) => (
            <ProjectCard
              key={p.id}
              project={p}
              selected={selected.has(p.id)}
              onToggleSelected={() => onToggleSelected(p.id)}
              onOpen={() => onOpen(p.id)}
            />
          ))}
        </Box>
      )}
    </Box>
  );
}

function ProjectCard({
  project,
  selected,
  onToggleSelected,
  onOpen,
}: {
  project: DashboardProject;
  selected: boolean;
  onToggleSelected: () => void;
  onOpen: () => void;
}) {
  const idea = (project.idea ?? project.description ?? "").trim();
  return (
    <Card
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1.25,
        p: 2,
        borderColor: selected ? tokens.color.accent.violet : undefined,
      }}
    >
      <Stack direction="row" alignItems="flex-start" spacing={1}>
        <Checkbox
          size="small"
          checked={selected}
          onChange={onToggleSelected}
          sx={{ color: tokens.color.text.muted, p: 0.5 }}
        />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 16,
              fontWeight: 800,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {project.name}
          </Typography>
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
            updated {relativeTime(project.updatedAt)}
          </Typography>
        </Box>
        <StatusBadge status={project.status} />
      </Stack>

      <Typography
        sx={{
          color: tokens.color.text.secondary,
          display: "-webkit-box",
          fontSize: 13.5,
          lineHeight: 1.45,
          minHeight: 56,
          overflow: "hidden",
          WebkitBoxOrient: "vertical",
          WebkitLineClamp: 3,
        }}
      >
        {idea || "No prompt captured yet."}
      </Typography>

      <Stack direction="row" alignItems="center" spacing={1}>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontSize: 12,
            fontWeight: 700,
          }}
        >
          {formatNumber(project.files.length)} file
          {project.files.length === 1 ? "" : "s"}
        </Typography>
        <Box sx={{ flex: 1 }} />
        <Button
          size="small"
          onClick={onOpen}
          endIcon={<OpenInNewRounded sx={{ fontSize: 14 }} />}
        >
          Open
        </Button>
      </Stack>
    </Card>
  );
}

// ============================================================
// Recent runs table
// ============================================================

function RecentRunsTable({
  loading,
  error,
  rows,
  projectsById,
  onOpen,
}: {
  loading: boolean;
  error: unknown;
  rows: ExecutionsQuery["executions"];
  projectsById: Map<string, DashboardProject>;
  onOpen: (projectID: string) => void;
}) {
  return (
    <Box>
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontSize: 18,
          fontWeight: 800,
          mb: 1.5,
        }}
      >
        Recent runs
      </Typography>

      {error ? (
        <ErrorPanel error={error} title="Could not load executions" />
      ) : loading ? (
        <Stack spacing={1}>
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} variant="rectangular" height={52} sx={skelSx} />
          ))}
        </Stack>
      ) : rows.length === 0 ? (
        <EmptyState
          icon={<WorkspacesOutlined sx={{ fontSize: 36 }} />}
          title="No runs yet"
          body="Start a project and the first run will land here within seconds."
        />
      ) : (
        <Card sx={{ overflow: "hidden", p: 0 }}>
          {/* Desktop header */}
          <Box
            sx={{
              alignItems: "center",
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
              color: tokens.color.text.muted,
              display: { xs: "none", md: "grid" },
              fontSize: 11.5,
              fontWeight: 800,
              gap: 2,
              gridTemplateColumns:
                "minmax(0,1.4fr) minmax(0,2fr) 90px 90px 110px 70px",
              letterSpacing: 0.4,
              px: 2,
              py: 1.25,
              textTransform: "uppercase",
            }}
          >
            <Box>Project</Box>
            <Box>Prompt</Box>
            <Box>Spent</Box>
            <Box>Score</Box>
            <Box>Status</Box>
            <Box>Age</Box>
          </Box>

          {rows.map((e) => {
            const projectName = e.projectID
              ? projectsById.get(e.projectID)?.name ?? e.projectID
              : "—";
            return (
              <Box
                key={e.id}
                onClick={() => e.projectID && onOpen(e.projectID)}
                role="button"
                tabIndex={0}
                onKeyDown={(ev: KeyboardEvent<HTMLDivElement>) => {
                  if (ev.key === "Enter" && e.projectID) onOpen(e.projectID);
                }}
                sx={{
                  borderBottom: `1px solid ${tokens.color.border.subtle}`,
                  cursor: e.projectID ? "pointer" : "default",
                  display: { xs: "block", md: "grid" },
                  gap: 2,
                  gridTemplateColumns:
                    "minmax(0,1.4fr) minmax(0,2fr) 90px 90px 110px 70px",
                  alignItems: "center",
                  px: 2,
                  py: 1.5,
                  transition: `background-color ${tokens.motion.fast} ${tokens.motion.curve}`,
                  "&:hover": e.projectID
                    ? { bgcolor: tokens.color.bg.surfaceHover }
                    : undefined,
                  "&:last-of-type": { borderBottom: 0 },
                }}
              >
                <Cell label="Project" value={projectName} bold />
                <Cell
                  label="Prompt"
                  value={e.promptSummary ?? "—"}
                  muted
                  truncate
                />
                <Cell label="Spent" value={formatMoney(e.spentUSD)} />
                <Cell label="Score" value={e.completionScore.toFixed(2)} />
                <Cell label="Status" value={<StatusBadge status={e.status} />} />
                <Cell label="Age" value={relativeTime(e.createdAt)} muted />
              </Box>
            );
          })}
        </Card>
      )}
    </Box>
  );
}

function Cell({
  label,
  value,
  bold,
  muted,
  truncate,
}: {
  label: string;
  value: React.ReactNode;
  bold?: boolean;
  muted?: boolean;
  truncate?: boolean;
}) {
  return (
    <Box
      sx={{
        display: { xs: "flex", md: "block" },
        gap: 1.5,
        minWidth: 0,
        py: { xs: 0.25, md: 0 },
      }}
    >
      <Typography
        sx={{
          color: tokens.color.text.muted,
          display: { xs: "block", md: "none" },
          fontSize: 11,
          fontWeight: 700,
          letterSpacing: 0.4,
          minWidth: 70,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
      <Box
        sx={{
          color: muted ? tokens.color.text.secondary : tokens.color.text.primary,
          fontSize: 13.5,
          fontWeight: bold ? 700 : 500,
          minWidth: 0,
          ...(truncate
            ? {
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }
            : {}),
        }}
      >
        {value}
      </Box>
    </Box>
  );
}

// ============================================================
// 24h cost strip
// ============================================================

function CostStripCard({
  loading,
  points,
  total24h,
}: {
  loading: boolean;
  points: SparklinePoint[];
  total24h: number;
}) {
  return (
    <Card sx={{ p: 2.5 }}>
      <Stack
        direction={{ xs: "column", sm: "row" }}
        alignItems={{ xs: "flex-start", sm: "center" }}
        justifyContent="space-between"
        spacing={1.5}
        sx={{ mb: 1.5 }}
      >
        <Box>
          <Typography variant="overline" sx={{ color: tokens.color.text.secondary }}>
            Cost — last 24h
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontFamily: tokens.font.mono,
              fontSize: 24,
              fontWeight: 800,
            }}
          >
            {loading ? "—" : formatMoney(total24h)}
          </Typography>
        </Box>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
          per-hour bucket · ledger
        </Typography>
      </Stack>
      {loading ? (
        <Skeleton variant="rectangular" height={160} sx={skelSx} />
      ) : (
        <SpendBars
          points={points.map((p, i) => ({
            label: p.ts ? hourLabel(p.ts) : `${i}h`,
            value: p.value,
          }))}
          height={160}
          showCumulative={false}
          ariaLabel="Spend per hour over the last 24 hours"
        />
      )}
    </Card>
  );
}

// hourLabel — "09" / "14" etc., dense format so 24 columns stay legible.
function hourLabel(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.getHours().toString().padStart(2, "0");
}

// ============================================================
// Confirm delete bar
// ============================================================

function ConfirmDeleteBar({
  open,
  count,
  onCancel,
  onConfirm,
  busy,
}: {
  open: boolean;
  count: number;
  onCancel: () => void;
  onConfirm: () => void;
  busy: boolean;
}) {
  if (!open) return null;
  return (
    <Box
      role="alertdialog"
      sx={{
        bgcolor: tokens.color.bg.surfaceRaised,
        border: `1px solid ${tokens.color.border.strong}`,
        borderRadius: 1,
        bottom: 24,
        boxShadow: tokens.shadow.lg,
        left: "50%",
        p: 2,
        position: "fixed",
        transform: "translateX(-50%)",
        zIndex: 1300,
      }}
    >
      <Stack direction="row" alignItems="center" spacing={2}>
        <Typography sx={{ color: tokens.color.text.primary, fontWeight: 700 }}>
          Delete {count} project{count === 1 ? "" : "s"}? This cannot be undone.
        </Typography>
        <Button onClick={onCancel} disabled={busy} variant="text">
          Cancel
        </Button>
        <Button variant="contained" color="error" onClick={onConfirm} disabled={busy}>
          {busy ? "Deleting…" : "Delete"}
        </Button>
      </Stack>
    </Box>
  );
}

export default DashboardPage;
