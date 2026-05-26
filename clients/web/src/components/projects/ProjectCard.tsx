"use client";

// ProjectCard — one project tile in the /projects grid (and any
// "recent projects" rail). Shows name, optional one-line description
// or "Created {relative time}", latest execution status badge, last-run
// time, total spend chip, and an executions count. The whole tile is
// a clickable link into the per-project studio at /p/{id}.
//
// The 3-dot overflow menu exposes destructive actions wired to the
// real orchestrator mutations — Delete is implemented via
// useDeleteProjectMutation; Rename / Archive are intentionally left as
// TODO(projects-wire) until the orchestrator surface lands.

import {
  ArrowForwardRounded,
  HistoryRounded,
  MoreHorizRounded,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { memo, useState } from "react";
import { extractErrorMessage } from "../../lib/errors";
import { useDeleteProjectMutation } from "../../lib/gql/__generated__";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import { MoneyChip } from "../cockpit/MoneyChip";
import { StatusBadge } from "../cockpit/StatusBadge";

export interface ProjectCardData {
  id: string;
  name: string;
  description?: string | null;
  createdAt: string;
  latestStatus?: string | null;
  // Timestamp of the most recent execution touching this project
  // (endedAt | startedAt | admittedAt | createdAt). Used for the
  // "ran {relative}" metric on the card.
  lastRunAt?: string | null;
  totalSpentUSD: number;
  // Number of executions tied to this project. Surfaced as the third
  // metric on the card so operators see activity volume at a glance.
  executions?: number;
}

export interface ProjectCardProps {
  project: ProjectCardData;
  onDeleted?: (id: string) => void;
}

function ProjectCardImpl({ project, onDeleted }: ProjectCardProps) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [deleteProject, deleteState] = useDeleteProjectMutation();
  const [error, setError] = useState<string | null>(null);

  const onDeleteClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setAnchor(null);
    setError(null);
    setConfirmOpen(true);
  };

  const onConfirm = async () => {
    setError(null);
    try {
      const res = await deleteProject({
        variables: { id: project.id },
        refetchQueries: ["Projects"],
      });
      if (res.data?.deleteProject?.ok === false) {
        setError(res.data.deleteProject.message || "Could not delete project.");
        return;
      }
      setConfirmOpen(false);
      onDeleted?.(project.id);
    } catch (e) {
      setError(extractErrorMessage(e));
    }
  };

  const hasDescription = !!project.description?.trim();
  const lastRunLabel = project.lastRunAt
    ? `Ran ${relativeTime(project.lastRunAt)}`
    : `Created ${relativeTime(project.createdAt)}`;
  const executionsCount = project.executions ?? 0;

  return (
    <Box
      component={Link}
      href={`/p/${project.id}`}
      aria-label={`Open ${project.name || "Untitled project"} in Studio`}
      sx={{
        position: "relative",
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        p: { xs: 2, md: 2.5 },
        display: "flex",
        flexDirection: "column",
        minHeight: { xs: 200, md: 220 },
        textDecoration: "none",
        color: "inherit",
        cursor: "pointer",
        transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}, transform ${tokens.motion.fast} ${tokens.motion.snap}, background-color ${tokens.motion.fast} ${tokens.motion.snap}`,
        "&:hover": {
          borderColor: tokens.color.border.strong,
          bgcolor: tokens.color.bg.surfaceHover,
          transform: "translateY(-1px)",
        },
        "&:focus-visible": {
          outline: `2px solid ${tokens.color.accent.violet}`,
          outlineOffset: 2,
        },
      }}
    >
      <Stack direction="row" spacing={1} alignItems="flex-start" sx={{ mb: 1.25 }}>
        <Typography
          component="h3"
          sx={{
            flex: 1,
            minWidth: 0,
            fontSize: { xs: 16, md: 17 },
            fontWeight: 700,
            color: tokens.color.text.primary,
            letterSpacing: -0.2,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {project.name || "Untitled project"}
        </Typography>
        {project.latestStatus && (
          <StatusBadge status={project.latestStatus} />
        )}
        <IconButton
          size="small"
          aria-label="Project actions"
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            setAnchor(e.currentTarget);
          }}
          sx={{
            color: tokens.color.text.muted,
            ml: 0.25,
            width: 32,
            height: 32,
          }}
        >
          <MoreHorizRounded sx={{ fontSize: 18 }} />
        </IconButton>
        <Menu
          anchorEl={anchor}
          open={!!anchor}
          onClose={() => setAnchor(null)}
          slotProps={{
            paper: {
              sx: {
                minWidth: 200,
                border: `1px solid ${tokens.color.border.subtle}`,
              },
            },
          }}
        >
          <MenuItem
            component={Link}
            href={`/p/${project.id}`}
            onClick={() => setAnchor(null)}
          >
            <ListItemText primary="Open studio" />
          </MenuItem>
          {/* TODO(projects-wire): rename + archive mutations not yet
              exposed by the orchestrator. Hidden until UpdateProject
              supports the new fields. */}
          <MenuItem onClick={onDeleteClick} sx={{ color: tokens.color.accent.danger }}>
            <ListItemText
              primary="Delete project"
              slotProps={{
                primary: { sx: { color: tokens.color.accent.danger, fontWeight: 700 } },
              }}
            />
          </MenuItem>
        </Menu>
      </Stack>

      <Typography
        sx={{
          fontSize: 13.5,
          color: tokens.color.text.secondary,
          flex: 1,
          display: "-webkit-box",
          WebkitLineClamp: hasDescription ? 2 : 1,
          WebkitBoxOrient: "vertical",
          overflow: "hidden",
          mb: 1.75,
        }}
      >
        {hasDescription
          ? project.description
          : `Created ${relativeTime(project.createdAt)}`}
      </Typography>

      <Stack
        direction="row"
        spacing={1}
        alignItems="center"
        useFlexGap
        flexWrap="wrap"
        sx={{ rowGap: 1, mb: 1.75 }}
      >
        <MoneyChip
          amountUSD={project.totalSpentUSD}
          color={project.totalSpentUSD > 0 ? "accent" : "neutral"}
        />
        <Stack
          direction="row"
          spacing={0.5}
          alignItems="center"
          sx={{
            fontFamily: tokens.font.mono,
            fontSize: 11,
            color: tokens.color.text.muted,
            letterSpacing: 0.4,
          }}
        >
          <HistoryRounded sx={{ fontSize: 13 }} />
          <Box component="span">{lastRunLabel}</Box>
        </Stack>
        <Box
          sx={{
            fontFamily: tokens.font.mono,
            fontSize: 11,
            color: tokens.color.text.muted,
            letterSpacing: 0.4,
          }}
        >
          {executionsCount === 1
            ? "1 run"
            : `${executionsCount.toLocaleString()} runs`}
        </Box>
      </Stack>

      <Stack direction="row" alignItems="center" spacing={0.5} sx={{ mt: "auto" }}>
        <Typography
          sx={{
            color: tokens.color.accent.violet,
            fontSize: 13,
            fontWeight: 700,
            letterSpacing: 0.2,
          }}
        >
          Open studio
        </Typography>
        <ArrowForwardRounded
          sx={{ fontSize: 14, color: tokens.color.accent.violet }}
        />
      </Stack>

      <Dialog
        open={confirmOpen}
        onClose={(_, reason) => {
          if (deleteState.loading) return;
          if (reason === "backdropClick" || reason === "escapeKeyDown") {
            setConfirmOpen(false);
          }
        }}
        slotProps={{
          paper: {
            sx: {
              bgcolor: tokens.color.bg.surfaceRaised,
              border: `1px solid ${tokens.color.border.subtle}`,
              minWidth: { xs: 280, sm: 420 },
            },
            // Stop click-through from re-triggering the card link.
            onClick: (e: React.MouseEvent) => e.stopPropagation(),
          },
        }}
      >
        <DialogTitle sx={{ fontWeight: 800 }}>Delete project?</DialogTitle>
        <DialogContent>
          <Typography sx={{ fontSize: 14, color: tokens.color.text.secondary }}>
            “{project.name || "Untitled project"}” will be removed. Linked
            executions stay in the ledger; only the project metadata is
            deleted. This cannot be undone from the UI.
          </Typography>
          {error && (
            <Typography
              sx={{
                mt: 2,
                fontSize: 13,
                color: tokens.color.accent.danger,
              }}
            >
              {error}
            </Typography>
          )}
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              setConfirmOpen(false);
            }}
            disabled={deleteState.loading}
            sx={{ color: tokens.color.text.secondary }}
          >
            Cancel
          </Button>
          <Button
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              void onConfirm();
            }}
            disabled={deleteState.loading}
            variant="outlined"
            sx={{
              borderColor: `${tokens.color.accent.danger}99`,
              color: tokens.color.accent.danger,
              "&:hover": {
                borderColor: tokens.color.accent.danger,
                bgcolor: `${tokens.color.accent.danger}14`,
              },
            }}
          >
            {deleteState.loading ? "Deleting…" : "Delete project"}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}

// Memoised export — projects grid can render 100+ cards; with filter
// / sort changes upstream, each card would re-render unnecessarily.
// Custom comparator only looks at the props we actually consume; the
// onDeleted callback identity may flip across renders but only matters
// when invoked, not at render time.
export const ProjectCard = memo(ProjectCardImpl, (prev, next) => {
  if (prev.onDeleted !== next.onDeleted) return false;
  const a = prev.project;
  const b = next.project;
  return (
    a.id === b.id &&
    a.name === b.name &&
    a.description === b.description &&
    a.createdAt === b.createdAt &&
    a.latestStatus === b.latestStatus &&
    a.lastRunAt === b.lastRunAt &&
    a.totalSpentUSD === b.totalSpentUSD &&
    a.executions === b.executions
  );
});
