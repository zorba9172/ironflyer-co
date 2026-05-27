"use client";

// TemplatesPage — blueprint catalogue with category filter, ranking
// strip, and a "use this template" CTA.
//
// V22 contract: "Use this template" goes through describeIdea so the
// wallet hold + execution admit happen atomically (law 1). We pass
// the picked blueprint via blueprintIDOverride so the orchestrator
// pins to it instead of letting the parser re-pick from the
// description text. After the bootstrap returns we route the user
// straight into /p/[projectID]?executionID=<id> so the studio loads
// the running execution without a recents scan.
//
// GraphQL operations consumed:
//   - query Blueprints                  (generated)
//   - query BlueprintRanking            (generated)
//   - mutation DescribeIdea             (generated; with blueprintIDOverride)

import {
  AutoAwesomeRounded,
  RocketLaunchRounded,
  TimerOutlined,
  TollOutlined,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Skeleton,
  Stack,
  Typography,
} from "@mui/material";
import { useRouter } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
import { useAuth } from "../lib/auth";
import { extractErrorMessage } from "../lib/errors";
import * as swal from "../lib/swal";
import { formatMoney, formatNumber } from "../lib/format";
import {
  useBlueprintRankingQuery,
  useBlueprintsQuery,
  useDescribeIdeaMutation,
  type BlueprintsQuery,
} from "../lib/gql/__generated__";
import { tokens } from "../theme";
import { EmptyState, ErrorPanel, LoadingPanel, PageHeader } from "./cockpit";

type Blueprint = BlueprintsQuery["blueprints"][number];

const ALL_CATEGORIES = "All categories";

const skelSx = {
  bgcolor: tokens.color.bg.surfaceHover,
  borderRadius: 1,
};

export function TemplatesPage() {
  const router = useRouter();
  const { authenticated, loading: authLoading } = useAuth();

  const skip = !authenticated;
  const blueprintsQ = useBlueprintsQuery({ skip });
  const rankingQ = useBlueprintRankingQuery({
    skip,
    variables: { byMetric: "executions", limit: 5 },
  });

  const [describeIdea, describeIdeaM] = useDescribeIdeaMutation();

  const [activeCategory, setActiveCategory] = useState<string>(ALL_CATEGORIES);
  const [pendingId, setPendingId] = useState<string | null>(null);

  const blueprints = blueprintsQ.data?.blueprints ?? [];

  const categories = useMemo(() => {
    const set = new Set<string>();
    for (const bp of blueprints) {
      if (bp.category) set.add(bp.category);
    }
    return [ALL_CATEGORIES, ...Array.from(set).sort()];
  }, [blueprints]);

  const visible = useMemo(() => {
    if (activeCategory === ALL_CATEGORIES) return blueprints;
    return blueprints.filter((bp) => bp.category === activeCategory);
  }, [blueprints, activeCategory]);

  const blueprintsById = useMemo(() => {
    const map = new Map<string, Blueprint>();
    for (const bp of blueprints) map.set(bp.id, bp);
    return map;
  }, [blueprints]);

  const handleUse = useCallback(
    async (bp: Blueprint) => {
      setPendingId(bp.id);
      try {
        const res = await describeIdea({
          variables: {
            input: {
              text: bp.description?.trim() || bp.name,
              blueprintIDOverride: bp.id,
              startImmediately: true,
            },
          },
        });
        if (res.errors && res.errors.length > 0) {
          throw new Error(
            res.errors.map((e) => e.message).join("\n") ||
              "Backend rejected the request.",
          );
        }
        const projectID = res.data?.describeIdea.project.id;
        const executionID = res.data?.describeIdea.execution.id;
        if (!projectID) {
          throw new Error("Backend did not return a project id.");
        }
        const qs = new URLSearchParams({ tab: "preview" });
        if (executionID) qs.set("executionID", executionID);
        router.push(`/p/${encodeURIComponent(projectID)}?${qs.toString()}`);
      } catch (err) {
        void swal.error("Template launch failed", extractErrorMessage(err));
        setPendingId(null);
      }
    },
    [describeIdea, router],
  );

  if (authLoading) {
    return (
      <>
        <PageHeader title="Templates" />
        <LoadingPanel label="Loading blueprints" />
      </>
    );
  }

  return (
    <Box>
      <PageHeader
        title="Blueprints"
        eyebrow="ship from a known shape"
        description="Every blueprint declares the gates it runs, a cost prior, and a target time to first preview. Pick one to spin up a new project."
      />

      <Stack spacing={4} sx={{ pb: 6 }}>
        <RankingStrip
          loading={rankingQ.loading}
          rows={rankingQ.data?.blueprintRanking ?? []}
          blueprintsById={blueprintsById}
          onUse={handleUse}
          pendingId={pendingId}
          submitting={describeIdeaM.loading}
        />

        <CategoryFilter
          categories={categories}
          active={activeCategory}
          onPick={setActiveCategory}
        />

        {blueprintsQ.error ? (
          <ErrorPanel
            error={blueprintsQ.error}
            title="Could not load blueprints"
            onRetry={() => blueprintsQ.refetch()}
          />
        ) : blueprintsQ.loading ? (
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
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton
                key={i}
                variant="rectangular"
                height={220}
                sx={skelSx}
              />
            ))}
          </Box>
        ) : visible.length === 0 ? (
          <EmptyState
            icon={<AutoAwesomeRounded sx={{ fontSize: 36 }} />}
            title="No blueprints in this category"
            body="Try another category — new blueprints land as code, not at runtime."
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
            {visible.map((bp) => (
              <BlueprintCard
                key={bp.id}
                bp={bp}
                onUse={() => handleUse(bp)}
                busy={pendingId === bp.id && describeIdeaM.loading}
                disabled={describeIdeaM.loading && pendingId !== bp.id}
              />
            ))}
          </Box>
        )}
      </Stack>
    </Box>
  );
}

function CategoryFilter({
  categories,
  active,
  onPick,
}: {
  categories: string[];
  active: string;
  onPick: (c: string) => void;
}) {
  return (
    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
      {categories.map((c) => {
        const isActive = c === active;
        return (
          <Chip
            key={c}
            label={c}
            onClick={() => onPick(c)}
            sx={{
              bgcolor: isActive
                ? tokens.color.accent.violet
                : tokens.color.bg.surface,
              border: `1px solid ${
                isActive
                  ? tokens.color.accent.violet
                  : tokens.color.border.subtle
              }`,
              color: isActive
                ? tokens.color.text.inverse
                : tokens.color.text.primary,
              fontWeight: 700,
              letterSpacing: 0.2,
              "&:hover": {
                bgcolor: isActive
                  ? tokens.color.accent.violet
                  : tokens.color.bg.surfaceHover,
              },
            }}
          />
        );
      })}
    </Stack>
  );
}

function BlueprintCard({
  bp,
  onUse,
  busy,
  disabled,
}: {
  bp: Blueprint;
  onUse: () => void;
  busy: boolean;
  disabled: boolean;
}) {
  return (
    <Card
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1.5,
        minHeight: 220,
        p: 2.25,
      }}
    >
      <Stack direction="row" alignItems="flex-start" spacing={1}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            sx={{
              color: tokens.color.text.primary,
              fontSize: 17,
              fontWeight: 800,
            }}
          >
            {bp.name}
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.secondary,
              fontSize: 13.5,
              lineHeight: 1.45,
              mt: 0.5,
              display: "-webkit-box",
              WebkitBoxOrient: "vertical",
              WebkitLineClamp: 3,
              overflow: "hidden",
            }}
          >
            {bp.description}
          </Typography>
        </Box>
        <Chip
          size="small"
          label={bp.category || "—"}
          sx={{
            bgcolor: `${tokens.color.accent.violet}1c`,
            color: tokens.color.accent.violet,
            fontWeight: 800,
            letterSpacing: 0.3,
            textTransform: "uppercase",
            fontSize: 10.5,
            height: 22,
          }}
        />
      </Stack>

      <Stack
        direction="row"
        spacing={2}
        sx={{ color: tokens.color.text.secondary, flexWrap: "wrap" }}
        useFlexGap
      >
        <MetaIcon
          icon={<TollOutlined sx={{ fontSize: 14 }} />}
          label={formatMoney(bp.costPriorUSD)}
          title="Cost prior — what runs typically cost"
        />
        <MetaIcon
          icon={<TimerOutlined sx={{ fontSize: 14 }} />}
          label={formatExpectedTime(bp.expectedTimeToPreviewSec)}
          title="Expected time to first preview"
        />
        <MetaIcon
          icon={<RocketLaunchRounded sx={{ fontSize: 14 }} />}
          label={`${formatNumber(bp.fileCount)} files`}
          title="Files the blueprint ships"
        />
      </Stack>

      <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap>
        {bp.supportedGates.slice(0, 6).map((g) => (
          <Chip
            key={g}
            size="small"
            label={g}
            sx={{
              bgcolor: tokens.color.bg.surfaceRaised,
              color: tokens.color.text.secondary,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              fontWeight: 700,
              height: 20,
              letterSpacing: 0.3,
              textTransform: "uppercase",
            }}
          />
        ))}
        {bp.supportedGates.length > 6 && (
          <Chip
            size="small"
            label={`+${bp.supportedGates.length - 6}`}
            sx={{
              bgcolor: tokens.color.bg.surfaceRaised,
              color: tokens.color.text.muted,
              fontSize: 10.5,
              fontWeight: 700,
              height: 20,
            }}
          />
        )}
      </Stack>

      <Box sx={{ flex: 1 }} />

      <Button
        fullWidth
        variant="contained"
        color="primary"
        onClick={onUse}
        disabled={busy || disabled}
        startIcon={
          busy ? (
            <CircularProgress
              size={14}
              sx={{ color: tokens.color.text.inverse }}
            />
          ) : undefined
        }
      >
        {busy ? "Spinning up workspace…" : "Use this template"}
      </Button>
    </Card>
  );
}

function MetaIcon({
  icon,
  label,
  title,
}: {
  icon: React.ReactNode;
  label: string;
  title: string;
}) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={0.5}
      title={title}
      sx={{
        color: tokens.color.text.secondary,
        fontSize: 12.5,
        fontWeight: 700,
      }}
    >
      {icon}
      <span>{label}</span>
    </Stack>
  );
}

function RankingStrip({
  loading,
  rows,
  blueprintsById,
  onUse,
  pendingId,
  submitting,
}: {
  loading: boolean;
  rows: Array<{
    blueprintID: string;
    executions: number;
    avgRevenueUSD: number;
    avgCostUSD: number;
    grossMarginPct: number;
  }>;
  blueprintsById: Map<string, Blueprint>;
  onUse: (bp: Blueprint) => void;
  pendingId: string | null;
  submitting: boolean;
}) {
  return (
    <Box>
      <Stack direction="row" alignItems="baseline" spacing={1} sx={{ mb: 1 }}>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.secondary }}
        >
          Top performers this week
        </Typography>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
          ranked by executions
        </Typography>
      </Stack>
      {loading ? (
        <Box
          sx={{
            display: "grid",
            gap: 1.25,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              md: "repeat(5, 1fr)",
            },
          }}
        >
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} variant="rectangular" height={92} sx={skelSx} />
          ))}
        </Box>
      ) : rows.length === 0 ? (
        <Card sx={{ p: 2, color: tokens.color.text.muted, fontSize: 13 }}>
          No blueprint ranking yet — the first runs will fill this strip.
        </Card>
      ) : (
        <Box
          sx={{
            display: "grid",
            gap: 1.25,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              md: "repeat(5, 1fr)",
            },
          }}
        >
          {rows.map((row, idx) => {
            const bp = blueprintsById.get(row.blueprintID);
            const isPending = pendingId === row.blueprintID && submitting;
            return (
              <Card
                key={row.blueprintID}
                onClick={() => bp && !submitting && onUse(bp)}
                role="button"
                sx={{
                  cursor: bp ? "pointer" : "default",
                  p: 1.5,
                  transition: `border-color ${tokens.motion.fast} ${tokens.motion.curve}`,
                  "&:hover": bp
                    ? { borderColor: tokens.color.accent.violet }
                    : undefined,
                }}
              >
                <Typography
                  sx={{
                    color: tokens.color.accent.violet,
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                    fontWeight: 800,
                    letterSpacing: 0.5,
                  }}
                >
                  #{idx + 1}
                </Typography>
                <Typography
                  sx={{
                    color: tokens.color.text.primary,
                    fontSize: 14,
                    fontWeight: 800,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {bp?.name ?? row.blueprintID}
                </Typography>
                <Typography
                  sx={{
                    color: tokens.color.text.muted,
                    fontFamily: tokens.font.mono,
                    fontSize: 11.5,
                    fontWeight: 700,
                  }}
                >
                  {formatNumber(row.executions)} runs ·{" "}
                  {Number.isFinite(row.grossMarginPct)
                    ? `${row.grossMarginPct.toFixed(0)}% margin`
                    : "—"}
                </Typography>
                {isPending && (
                  <CircularProgress
                    size={12}
                    sx={{ color: tokens.color.accent.violet, mt: 0.5 }}
                  />
                )}
              </Card>
            );
          })}
        </Box>
      )}
    </Box>
  );
}

function formatExpectedTime(seconds: number | null | undefined): string {
  if (!seconds || seconds <= 0) return "—";
  if (seconds < 60) return `≈ ${seconds}s`;
  const m = Math.round(seconds / 60);
  if (m < 60) return `≈ ${m} min`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  return rm === 0 ? `≈ ${h}h` : `≈ ${h}h ${rm}m`;
}

export default TemplatesPage;
