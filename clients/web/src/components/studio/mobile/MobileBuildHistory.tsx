"use client";

// MobileBuildHistory — visual mirror of recent EAS / Linux mobile
// build artifacts. Renders a tight MUI Table with status chips, a
// live progress bar for running builds, and a tail of the build log.
// Every color goes through the theme palette or design tokens.

import { CancelRounded, OpenInNewRounded } from "@mui/icons-material";
import {
  Box,
  Button,
  Chip,
  IconButton,
  LinearProgress,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { tokens } from "../../../theme";
import type {
  MobileBuild,
  MobileBuildStatus,
} from "../../../lib/mobile/useMobileSession";

export interface MobileBuildHistoryProps {
  builds: MobileBuild[];
  onCancel?: (buildId: string) => void;
}

type ChipColor =
  | "default"
  | "primary"
  | "secondary"
  | "info"
  | "success"
  | "warning"
  | "error";

function statusChipProps(status: MobileBuildStatus): {
  color: ChipColor;
  label: string;
} {
  switch (status) {
    case "queued":
      return { color: "info", label: "Queued" };
    case "running":
      return { color: "warning", label: "Running" };
    case "succeeded":
      return { color: "success", label: "Succeeded" };
    case "failed":
      return { color: "error", label: "Failed" };
    case "cancelled":
      return { color: "default", label: "Cancelled" };
    default:
      return { color: "default", label: status };
  }
}

function formatDuration(ms?: number): string {
  if (!ms || ms <= 0) return "—";
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remSec = seconds % 60;
  return `${minutes}m ${remSec}s`;
}

function formatSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return "—";
  const mb = bytes / (1024 * 1024);
  if (mb < 1) return `${(bytes / 1024).toFixed(0)} KB`;
  if (mb < 1024) return `${mb.toFixed(1)} MB`;
  return `${(mb / 1024).toFixed(2)} GB`;
}

function formatStarted(iso: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

export function MobileBuildHistory({
  builds,
  onCancel,
}: MobileBuildHistoryProps) {
  if (builds.length === 0) {
    return (
      <Box
        sx={{
          alignItems: "center",
          bgcolor: tokens.color.bg.surface,
          border: `1px dashed ${tokens.color.border.subtle}`,
          borderRadius: 1,
          color: tokens.color.text.muted,
          display: "flex",
          flexDirection: "column",
          gap: 0.5,
          justifyContent: "center",
          p: 4,
          textAlign: "center",
        }}
      >
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            letterSpacing: 1.2,
            textTransform: "uppercase",
          }}
        >
          No builds yet
        </Typography>
        <Typography
          sx={{ color: tokens.color.text.secondary, fontSize: 13 }}
        >
          Trigger a development, preview, or production build to see
          history here.
        </Typography>
      </Box>
    );
  }

  const running = builds.find((b) => b.status === "running");

  return (
    <Stack spacing={1.5}>
      <Box
        sx={{
          bgcolor: tokens.color.bg.surface,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          overflow: "hidden",
        }}
      >
        <Table size="small" aria-label="Mobile build history">
          <TableHead>
            <TableRow
              sx={{
                "& th": {
                  borderBottomColor: tokens.color.border.subtle,
                  color: tokens.color.text.muted,
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  letterSpacing: 1,
                  textTransform: "uppercase",
                },
              }}
            >
              <TableCell>Platform</TableCell>
              <TableCell>Profile</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Started</TableCell>
              <TableCell>Duration</TableCell>
              <TableCell>Artifact</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {builds.map((b) => {
              const chip = statusChipProps(b.status);
              return (
                <TableRow
                  key={b.id}
                  sx={{
                    "& td": {
                      borderBottomColor: tokens.color.border.subtle,
                      color: tokens.color.text.secondary,
                      fontSize: 12.5,
                    },
                    "&:last-child td": { borderBottom: "none" },
                  }}
                >
                  <TableCell sx={{ color: tokens.color.text.primary }}>
                    {b.platform === "android" ? "Android" : "iOS"}
                  </TableCell>
                  <TableCell>{b.profile}</TableCell>
                  <TableCell>
                    <Chip
                      label={chip.label}
                      color={chip.color}
                      size="small"
                      variant={
                        b.status === "cancelled" ? "outlined" : "filled"
                      }
                    />
                  </TableCell>
                  <TableCell
                    sx={{ fontFamily: tokens.font.mono, fontSize: 11.5 }}
                  >
                    {formatStarted(b.startedAt)}
                  </TableCell>
                  <TableCell
                    sx={{ fontFamily: tokens.font.mono, fontSize: 11.5 }}
                  >
                    {formatDuration(b.durationMs)}
                  </TableCell>
                  <TableCell
                    sx={{ fontFamily: tokens.font.mono, fontSize: 11.5 }}
                  >
                    {b.artifactUrl ? formatSize(b.artifactSizeBytes) : "—"}
                  </TableCell>
                  <TableCell align="right">
                    <Stack
                      direction="row"
                      spacing={0.5}
                      justifyContent="flex-end"
                    >
                      {b.artifactUrl ? (
                        <Tooltip title="Open artifact" arrow>
                          <IconButton
                            component="a"
                            href={b.artifactUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            size="small"
                            aria-label="Open artifact"
                            sx={{ color: tokens.color.text.secondary }}
                          >
                            <OpenInNewRounded sx={{ fontSize: 14 }} />
                          </IconButton>
                        </Tooltip>
                      ) : null}
                      {b.status === "running" || b.status === "queued" ? (
                        <Tooltip title="Cancel build" arrow>
                          <span>
                            <IconButton
                              size="small"
                              aria-label="Cancel build"
                              disabled={!onCancel}
                              onClick={() => onCancel?.(b.id)}
                              sx={{ color: tokens.color.accent.danger }}
                            >
                              <CancelRounded sx={{ fontSize: 14 }} />
                            </IconButton>
                          </span>
                        </Tooltip>
                      ) : null}
                    </Stack>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </Box>

      {running ? (
        <Box
          sx={{
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            p: 1.5,
          }}
        >
          <Stack
            direction="row"
            spacing={1}
            sx={{ alignItems: "center", mb: 1 }}
          >
            <Typography
              sx={{
                color: tokens.color.text.primary,
                fontSize: 13,
                fontWeight: 600,
              }}
            >
              Build {running.id} — {running.platform} / {running.profile}
            </Typography>
            {onCancel ? (
              <Button
                size="small"
                color="error"
                variant="outlined"
                onClick={() => onCancel(running.id)}
                sx={{ ml: "auto" }}
              >
                Cancel
              </Button>
            ) : null}
          </Stack>
          <LinearProgress
            color="warning"
            sx={{
              bgcolor: tokens.color.bg.inset,
              borderRadius: `${tokens.radius.pill}px`,
              height: 6,
              mb: 1.5,
            }}
          />
          <Box
            component="pre"
            sx={{
              bgcolor: tokens.color.bg.inset,
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: 1,
              color: tokens.color.text.secondary,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              lineHeight: 1.5,
              m: 0,
              maxHeight: 220,
              overflow: "auto",
              p: 1.25,
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
            }}
          >
            {running.logTail?.trim() || "Waiting for log output…"}
          </Box>
        </Box>
      ) : null}
    </Stack>
  );
}
