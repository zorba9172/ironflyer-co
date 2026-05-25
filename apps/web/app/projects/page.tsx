"use client";

// /projects — the canonical operator surface for "what am I working on
// right now?". Renders inside the CockpitFrame and shows every project
// owned by the calling tenant as a tile with status pill, last-run
// timestamp, total spend, and executions count.
//
// Data sources are typed codegen hooks:
//   - useProjectsQuery     → projects.graphql
//   - useExecutionsQuery   → executions.graphql
//
// Latest execution status, last-run timestamp, total spend, and
// executions count are computed client-side from the executions list
// because the orchestrator does not yet ship a per-project rollup on
// the Project type. We do a single pass over the executions array and
// share the derived maps across every card — there is no per-card
// recomputation.

import { AddRounded, SearchRounded } from "@mui/icons-material";
import {
  Box,
  Button,
  InputAdornment,
  Skeleton,
  Stack,
  TextField,
} from "@mui/material";
import Link from "next/link";
import { useMemo, useState } from "react";
import {
  EmptyState,
  ErrorPanel,
  PageHeader,
} from "../../src/components/cockpit";
import { ProjectsGrid } from "../../src/components/projects/ProjectsGrid";
import type { ProjectCardData } from "../../src/components/projects/ProjectCard";
import {
  ProjectsFilterChips,
  type ProjectFilter,
  type ProjectsFilterChipsOption,
} from "../../src/components/projects/ProjectsFilterChips";
import {
  useExecutionsQuery,
  useProjectsQuery,
} from "../../src/lib/gql/__generated__";
import { RequireAuth } from "../../src/lib/auth";
import { tokens } from "../../src/theme";

// Status vocabulary mirrors the orchestrator's execution + project
// domain constants.
const ACTIVE_STATUSES = new Set([
  "created",
  "admitted",
  "running",
  "scoring",
  "draft",
  "active",
  "succeeded",
  "success",
  "live",
  "completed",
  "approved",
  "promoting",
  "preview_ready",
]);
const FAILED_STATUSES = new Set([
  "failed",
  "stopped",
  "killed",
  "error",
  "refunded",
  "rolled_back",
]);
const ARCHIVED_STATUSES = new Set(["archived", "deleted", "cancelled"]);

function matchesFilter(card: ProjectCardData, filter: ProjectFilter): boolean {
  if (filter === "all") return true;
  const status = (card.latestStatus || "").toLowerCase();
  if (filter === "active") {
    if (ARCHIVED_STATUSES.has(status) || FAILED_STATUSES.has(status)) return false;
    // Treat unknown / blank status as active (newly created projects
    // have not produced executions yet — they belong with the live
    // catalogue, not in failed or archived).
    return status === "" || ACTIVE_STATUSES.has(status) || !FAILED_STATUSES.has(status);
  }
  if (filter === "failed") return FAILED_STATUSES.has(status);
  if (filter === "archived") return ARCHIVED_STATUSES.has(status);
  return true;
}

export default function ProjectsPage() {
  return (
    <RequireAuth>
      <ProjectsView />
    </RequireAuth>
  );
}

function ProjectsView() {
  const [filter, setFilter] = useState<ProjectFilter>("all");
  const [query, setQuery] = useState("");

  const projectsQuery = useProjectsQuery({
    variables: { limit: 100, offset: 0 },
    fetchPolicy: "cache-and-network",
  });
  const executionsQuery = useExecutionsQuery({
    variables: { limit: 200, offset: 0 },
    fetchPolicy: "cache-and-network",
  });

  const projects = projectsQuery.data?.projects;
  const executions = executionsQuery.data?.executions;

  const cards = useMemo<ProjectCardData[]>(() => {
    if (!projects) return [];

    // Single pass over executions — derive latest status timestamp,
    // total spend, and run count per project ID.
    const latest = new Map<string, { status: string; ts: number }>();
    const spent = new Map<string, number>();
    const counts = new Map<string, number>();
    for (const e of executions ?? []) {
      const pid = e.projectID;
      if (!pid) continue;
      const cost =
        (e.providerCostUSD || 0) +
        (e.sandboxCostUSD || 0) +
        (e.storageCostUSD || 0) +
        (e.deploymentCostUSD || 0);
      spent.set(pid, (spent.get(pid) || 0) + cost);
      counts.set(pid, (counts.get(pid) || 0) + 1);
      const ts = new Date(
        e.endedAt || e.startedAt || e.admittedAt || e.createdAt,
      ).getTime();
      const existing = latest.get(pid);
      if (!existing || ts > existing.ts) {
        latest.set(pid, { status: e.status, ts });
      }
    }

    return projects.map<ProjectCardData>((p) => {
      const last = latest.get(p.id);
      return {
        id: p.id,
        name: p.name,
        description: p.description,
        createdAt: p.createdAt,
        latestStatus: last?.status ?? p.status,
        lastRunAt: last ? new Date(last.ts).toISOString() : null,
        totalSpentUSD: spent.get(p.id) || 0,
        executions: counts.get(p.id) || 0,
      };
    });
  }, [projects, executions]);

  // Count per filter — drives the chip counters so the operator sees
  // distribution at a glance.
  const counts = useMemo(() => {
    const c = { all: cards.length, active: 0, failed: 0, archived: 0 };
    for (const card of cards) {
      if (matchesFilter(card, "active")) c.active += 1;
      if (matchesFilter(card, "failed")) c.failed += 1;
      if (matchesFilter(card, "archived")) c.archived += 1;
    }
    return c;
  }, [cards]);

  const visible = useMemo(() => {
    const trimmed = query.trim().toLowerCase();
    return cards
      .filter((c) => matchesFilter(c, filter))
      .filter((c) => {
        if (!trimmed) return true;
        const hay = [c.name, c.description ?? "", c.id].join(" ").toLowerCase();
        return hay.includes(trimmed);
      })
      .sort((a, b) => {
        // Most recently-touched first: prefer lastRunAt, fall back to
        // createdAt so brand-new projects with no executions still
        // land at the top of "All".
        const ta = new Date(a.lastRunAt || a.createdAt).getTime();
        const tb = new Date(b.lastRunAt || b.createdAt).getTime();
        return tb - ta;
      });
  }, [cards, filter, query]);

  const filterOptions: ProjectsFilterChipsOption[] = [
    { value: "all", label: "All", count: counts.all },
    { value: "active", label: "Active", count: counts.active },
    { value: "failed", label: "Failed", count: counts.failed },
    { value: "archived", label: "Archived", count: counts.archived },
  ];

  const loading = projectsQuery.loading && !projects;
  const error = projectsQuery.error;

  return (
    <Box>
      <PageHeader
        title="Projects"
        description="Every paid execution lives inside a project. Pick one to jump back into its studio, or start a new build from the composer."
        actions={
          <Button
            component={Link}
            href="/studio"
            variant="contained"
            color="primary"
            startIcon={<AddRounded sx={{ fontSize: 18 }} />}
          >
            New project
          </Button>
        }
      />

      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.5}
        alignItems={{ md: "center" }}
        justifyContent="space-between"
        sx={{ mb: { xs: 2, md: 2.5 } }}
      >
        <ProjectsFilterChips
          options={filterOptions}
          value={filter}
          onChange={setFilter}
        />
        <TextField
          size="small"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Filter by name or id"
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchRounded
                  sx={{ fontSize: 18, color: tokens.color.text.muted }}
                />
              </InputAdornment>
            ),
          }}
          sx={{
            width: { xs: "100%", md: 300 },
            "& .MuiOutlinedInput-root": {
              bgcolor: tokens.color.bg.surface,
              fontFamily: tokens.font.mono,
              fontSize: 13,
              minHeight: 44,
            },
          }}
        />
      </Stack>

      {loading ? (
        <ProjectsGridSkeleton />
      ) : error ? (
        <ErrorPanel
          error={error}
          title="Could not load projects"
          onRetry={() => void projectsQuery.refetch()}
        />
      ) : cards.length === 0 ? (
        <EmptyState
          title="No projects yet"
          body="Start a new build from Studio and your first project lands here once Ironflyer admits the execution."
          cta={{ label: "Start a project", href: "/studio" }}
        />
      ) : visible.length === 0 ? (
        <EmptyState
          title="No projects match this view"
          body="Clear the filter or search to see every project in the workspace."
          cta={{ label: "Show all", onClick: () => {
            setFilter("all");
            setQuery("");
          } }}
        />
      ) : (
        <ProjectsGrid projects={visible} />
      )}
    </Box>
  );
}

// ProjectsGridSkeleton — placeholders shaped exactly like ProjectCard
// so the page does not jump when the first paint resolves.
function ProjectsGridSkeleton() {
  return (
    <Box
      sx={{
        display: "grid",
        gap: { xs: 1.5, md: 2 },
        gridTemplateColumns: {
          xs: "1fr",
          md: "repeat(2, minmax(0, 1fr))",
          lg: "repeat(3, minmax(0, 1fr))",
          xl: "repeat(4, minmax(0, 1fr))",
        },
      }}
    >
      {Array.from({ length: 8 }).map((_, i) => (
        <Box
          key={i}
          sx={{
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            p: { xs: 2, md: 2.5 },
            minHeight: { xs: 200, md: 220 },
          }}
        >
          <Stack direction="row" spacing={1} sx={{ mb: 1.5 }}>
            <Skeleton
              variant="text"
              width="65%"
              height={22}
              sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
            />
            <Box sx={{ flex: 1 }} />
            <Skeleton
              variant="rounded"
              width={56}
              height={22}
              sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
            />
          </Stack>
          <Skeleton
            variant="text"
            width="92%"
            height={16}
            sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
          />
          <Skeleton
            variant="text"
            width="78%"
            height={16}
            sx={{ bgcolor: tokens.color.bg.surfaceRaised, mb: 2 }}
          />
          <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
            <Skeleton
              variant="rounded"
              width={64}
              height={24}
              sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
            />
            <Skeleton
              variant="rounded"
              width={88}
              height={20}
              sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
            />
          </Stack>
        </Box>
      ))}
    </Box>
  );
}
