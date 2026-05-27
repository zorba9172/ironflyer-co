"use client";

// ProjectTabsBar — IDE-style tab strip at the top of the workbench
// that lists every project the operator owns and lets them switch
// between them with one click. The active tab maps to the projectID
// in the route; the trailing "+" tab routes back to the home composer
// so the next prompt can spawn a new project.
//
// Data comes from the codegen useProjectsQuery hook (cache-and-network
// so newly-created projects appear without a hard reload after the
// home composer redirects here). DeleteProject is deliberately not
// wired into the close button — closing a tab is purely a navigation
// concept; project deletion happens from /projects with an explicit
// confirmation. Closing the active tab routes the operator to the
// next tab to its left (or "/" if it was the last one).

import { AddRounded, CircleRounded, CloseRounded } from "@mui/icons-material";
import { Box, IconButton, Skeleton, Stack, Tooltip } from "@mui/material";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo } from "react";
import { tokens } from "../../theme";
import { useProjectsQuery } from "../../lib/gql/__generated__";

const MAX_TABS = 20;
const ACTIVE_STATUSES = new Set([
  "running",
  "admitted",
  "scoring",
  "created",
]);
const FAILED_STATUSES = new Set(["failed", "killed", "stopped", "error"]);

export interface ProjectTabsBarProps {
  projectID: string;
}

export function ProjectTabsBar({ projectID }: ProjectTabsBarProps) {
  const router = useRouter();
  const { data, loading } = useProjectsQuery({
    variables: { limit: MAX_TABS, offset: 0 },
    fetchPolicy: "cache-and-network",
    // Refetch periodically so a project created from another tab /
    // window shows up here without a hard reload.
    pollInterval: 30_000,
  });

  const projects = useMemo(() => {
    const rows = data?.projects ?? [];
    // Most-recently-updated first; the current project always pinned
    // to the front so the operator never loses sight of it.
    const sorted = [...rows].sort((a, b) => {
      const ta = new Date(a.updatedAt || a.createdAt).getTime();
      const tb = new Date(b.updatedAt || b.createdAt).getTime();
      return tb - ta;
    });
    // If the active projectID is not in the first MAX_TABS slice,
    // ensure it is included so the active tab is always visible.
    const slice = sorted.slice(0, MAX_TABS);
    if (projectID && !slice.some((p) => p.id === projectID)) {
      const found = sorted.find((p) => p.id === projectID);
      if (found) slice.unshift(found);
    }
    return slice;
  }, [data, projectID]);

  if (loading && projects.length === 0) {
    return (
      <Box
        sx={{
          alignItems: "center",
          bgcolor: tokens.color.bg.inset,
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          display: "flex",
          gap: 0.5,
          height: 38,
          minWidth: 0,
          px: 0.75,
        }}
      >
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton
            key={i}
            variant="rounded"
            width={140}
            height={28}
            sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
          />
        ))}
      </Box>
    );
  }

  return (
    <Box
      sx={{
        alignItems: "stretch",
        bgcolor: tokens.color.bg.inset,
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
        display: "flex",
        height: 38,
        minWidth: 0,
        overflowX: "auto",
        overflowY: "hidden",
        "&::-webkit-scrollbar": { height: 4 },
        "&::-webkit-scrollbar-thumb": {
          bgcolor: tokens.color.border.subtle,
          borderRadius: 4,
        },
      }}
      role="tablist"
      aria-label="Project tabs"
    >
      {projects.map((p) => {
        const active = p.id === projectID;
        const tone = statusTone(p.status);
        return (
          <ProjectTab
            key={p.id}
            id={p.id}
            name={p.name || `Project ${p.id.slice(0, 6)}`}
            active={active}
            tone={tone}
            onClose={(closedID) => {
              if (closedID !== projectID) return;
              const idx = projects.findIndex((row) => row.id === closedID);
              const next = projects[idx - 1] ?? projects[idx + 1] ?? null;
              router.push(next ? `/p/${encodeURIComponent(next.id)}` : "/");
            }}
          />
        );
      })}
      <Tooltip title="New project" arrow>
        <IconButton
          component={Link}
          href="/"
          size="small"
          aria-label="New project"
          sx={{
            alignSelf: "center",
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 0.75,
            color: tokens.color.text.secondary,
            flex: "0 0 auto",
            height: 26,
            ml: 0.5,
            mr: 0.5,
            width: 26,
            "&:hover": {
              bgcolor: tokens.color.bg.surfaceHover,
              borderColor: tokens.color.accent.violet,
              color: tokens.color.accent.violet,
            },
          }}
        >
          <AddRounded sx={{ fontSize: 16 }} />
        </IconButton>
      </Tooltip>
    </Box>
  );
}

interface ProjectTabProps {
  id: string;
  name: string;
  active: boolean;
  tone: "active" | "failed" | "idle";
  onClose: (id: string) => void;
}

function ProjectTab({ id, name, active, tone, onClose }: ProjectTabProps) {
  const dotColor =
    tone === "active"
      ? tokens.color.accent.success
      : tone === "failed"
        ? tokens.color.accent.danger
        : tokens.color.text.muted;

  return (
    <Box
      role="tab"
      aria-selected={active}
      sx={{
        alignItems: "center",
        bgcolor: active ? tokens.color.bg.surface : "transparent",
        borderRight: `1px solid ${tokens.color.border.subtle}`,
        borderTop: `2px solid ${
          active ? tokens.color.accent.violet : "transparent"
        }`,
        color: active
          ? tokens.color.text.primary
          : tokens.color.text.secondary,
        cursor: "pointer",
        display: "flex",
        flex: "0 0 auto",
        gap: 0.6,
        maxWidth: 200,
        minWidth: 120,
        position: "relative",
        px: 1.25,
        transition: `background ${tokens.motion.fast} ease, color ${tokens.motion.fast} ease`,
        "&:hover": {
          bgcolor: active
            ? tokens.color.bg.surface
            : tokens.color.bg.surfaceHover,
          color: tokens.color.text.primary,
          "& .project-tab-close": { opacity: 1 },
        },
      }}
    >
      <Box
        component={Link}
        href={`/p/${encodeURIComponent(id)}`}
        sx={{
          alignItems: "center",
          color: "inherit",
          display: "flex",
          flex: 1,
          gap: 0.6,
          minWidth: 0,
          textDecoration: "none",
        }}
      >
        <CircleRounded sx={{ color: dotColor, fontSize: 8 }} />
        <Box
          component="span"
          sx={{
            fontFamily: tokens.font.family,
            fontSize: 12.5,
            fontWeight: active ? 700 : 600,
            minWidth: 0,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {name}
        </Box>
      </Box>
      <Box
        component="button"
        type="button"
        className="project-tab-close"
        aria-label={`Close ${name}`}
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          onClose(id);
        }}
        sx={{
          alignItems: "center",
          background: "transparent",
          border: 0,
          borderRadius: 0.5,
          color: tokens.color.text.muted,
          cursor: "pointer",
          display: "flex",
          flex: "0 0 auto",
          height: 18,
          justifyContent: "center",
          opacity: active ? 1 : 0,
          p: 0,
          transition: `opacity ${tokens.motion.fast} ease, background ${tokens.motion.fast} ease`,
          width: 18,
          "&:hover": {
            bgcolor: tokens.color.bg.surfaceRaised,
            color: tokens.color.text.primary,
          },
        }}
      >
        <CloseRounded sx={{ fontSize: 13 }} />
      </Box>
    </Box>
  );
}

function statusTone(status: string | null | undefined): ProjectTabProps["tone"] {
  if (!status) return "idle";
  const s = status.toLowerCase();
  if (FAILED_STATUSES.has(s)) return "failed";
  if (ACTIVE_STATUSES.has(s)) return "active";
  return "idle";
}
