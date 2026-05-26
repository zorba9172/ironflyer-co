"use client";

// RecentsGrid — "pick up where you left" surface for signed-in users.
// Pulls the six most recent executions and renders one card per
// project. We show project-grade context: the prompt summary the
// orchestrator stored, the live execution status, spend so far, and
// when it last moved. Clicking a card routes into the studio (A48).

import {
  ArrowOutwardRounded,
  ScheduleOutlined,
} from "@mui/icons-material";
import { Box, Button, Card, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { useExecutionsQuery } from "../../lib/gql/__generated__";
import { formatMoney } from "../../lib/format";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import { ErrorPanel } from "../cockpit/ErrorPanel";
import { StatusBadge } from "../cockpit/StatusBadge";

export interface RecentsGridProps {
  // When false, render nothing. Page decides based on auth.
  enabled: boolean;
}

export function RecentsGrid({ enabled }: RecentsGridProps) {
  const { data, loading, error } = useExecutionsQuery({
    variables: { limit: 6 },
    skip: !enabled,
    fetchPolicy: "cache-and-network",
  });

  if (!enabled) return null;
  if (loading && !data) {
    return (
      <Stack spacing={2}>
        <SectionHeading title="Pick up where you left" />
        <Stack
          direction="row"
          spacing={2}
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr", md: "repeat(4, 1fr)" },
            gap: 2,
          }}
        >
          {[0, 1, 2, 3].map((i) => (
            <Card
              key={i}
              sx={{
                p: 2,
                height: 132,
                bgcolor: tokens.color.bg.surface,
                opacity: 0.5,
              }}
            />
          ))}
        </Stack>
      </Stack>
    );
  }
  if (error) {
    return (
      <Stack spacing={2}>
        <SectionHeading title="Pick up where you left" />
        <ErrorPanel error={error} title="Could not load recent executions" />
      </Stack>
    );
  }
  const executions = data?.executions ?? [];
  if (executions.length === 0) return null;

  return (
    <Stack spacing={2}>
      <SectionHeading title="Pick up where you left" />
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr", md: "repeat(4, 1fr)" },
          gap: 2,
        }}
      >
        {executions.map((e) => {
          const title =
            (e.promptSummary && e.promptSummary.trim()) ||
            (e.blueprintID ? `Blueprint: ${e.blueprintID}` : "Untitled execution");
          const projectHref = e.projectID
            ? `/p/${e.projectID}`
            : `/execution/${e.id}`;
          const lastTouched = e.endedAt || e.startedAt || e.admittedAt || e.createdAt;
          return (
            <Card
              key={e.id}
              sx={{
                p: 2,
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                display: "flex",
                flexDirection: "column",
                gap: 1.5,
                transition: "border-color 120ms ease, transform 120ms ease",
                "&:hover": {
                  borderColor: tokens.color.accent.violet,
                  transform: "translateY(-1px)",
                },
              }}
            >
              <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={1}>
                <Typography
                  sx={{
                    fontSize: 14,
                    fontWeight: 700,
                    lineHeight: 1.3,
                    color: tokens.color.text.primary,
                    display: "-webkit-box",
                    WebkitLineClamp: 2,
                    WebkitBoxOrient: "vertical",
                    overflow: "hidden",
                  }}
                >
                  {title}
                </Typography>
                <StatusBadge status={e.status} />
              </Stack>
              <Stack direction="row" alignItems="center" spacing={1.5}>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 12,
                    color: tokens.color.accent.violet,
                  }}
                >
                  {formatMoney(e.spentUSD)} spent
                </Typography>
                <Stack direction="row" alignItems="center" spacing={0.5} sx={{ color: tokens.color.text.muted }}>
                  <ScheduleOutlined sx={{ fontSize: 13 }} />
                  <Typography sx={{ fontSize: 11.5, fontFamily: tokens.font.mono }}>
                    {relativeTime(lastTouched)}
                  </Typography>
                </Stack>
              </Stack>
              <Box sx={{ flex: 1 }} />
              <Button
                component={Link}
                href={projectHref}
                size="small"
                variant="outlined"
                endIcon={<ArrowOutwardRounded sx={{ fontSize: 16 }} />}
                sx={{
                  alignSelf: "flex-start",
                  color: tokens.color.text.primary,
                  borderColor: tokens.color.border.strong,
                  "&:hover": {
                    borderColor: tokens.color.accent.violet,
                    bgcolor: "transparent",
                  },
                }}
              >
                Open studio
              </Button>
            </Card>
          );
        })}
      </Box>
    </Stack>
  );
}

function SectionHeading({ title, hint }: { title: string; hint?: string }) {
  return (
    <Stack direction="row" alignItems="baseline" justifyContent="space-between">
      <Typography sx={{ fontSize: 18, fontWeight: 800, letterSpacing: -0.2 }}>
        {title}
      </Typography>
      {hint && (
        <Typography sx={{ fontSize: 12.5, color: tokens.color.text.muted }}>
          {hint}
        </Typography>
      )}
    </Stack>
  );
}
