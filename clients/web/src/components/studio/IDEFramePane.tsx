"use client";

// IDEFramePane — embeds the slim IronFlyer openvscode-server profile
// inside the studio Code pane. The operator gets a real terminal,
// debugger, source control, and extension host without the default
// VS Code chrome taking over the workspace.
//
// Mechanics:
//   • The iframe src is built by `getOpenvscodeUrl` so the URL is
//     centralised and the eventual per-project routing only has to
//     change there.
//   • A 6-second timeout flips a `failed` state if the iframe never
//     fires `load` — almost always because the operator hasn't started
//     the `ide` compose profile. The fallback shows the exact command
//     plus a retry button.
//   • Until `load` (or `failed`), a token-driven shimmer skeleton
//     covers the surface so the operator never sees a blank box.
//
// Below md (mobile) the embedded experience is unusable — VS Code
// inside a 360px viewport collapses every panel. We render a single
// "Open IDE in new tab" CTA instead, gated by the same
// `getOpenvscodeUrl` so the URL stays consistent.

import {
  CloudSyncRounded,
  OpenInNewRounded,
  RefreshRounded,
  SyncAltRounded,
} from "@mui/icons-material";
import {
  Box,
  Button,
  IconButton,
  Stack,
  Tooltip,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { tokens } from "../../theme";
import { getOpenvscodeFolder, getOpenvscodeUrl } from "../../lib/ide";
import { useProjectFilesQuery, useProjectQuery } from "../../lib/gql/__generated__";
import { useAuth } from "../../lib/auth";
import { useIDEWriteBack, type WriteBackStatus } from "./useIDEWriteBack";

const LOAD_TIMEOUT_MS = 6000;
const OPENVSCODE_DEV_COMMAND =
  "docker compose -f infra/compose/docker-compose.dev.yml --profile ide up -d openvscode";

export interface IDEFramePaneProps {
  projectID: string;
}

type FrameStatus = "loading" | "ready" | "failed";
type SyncStatus = "idle" | "syncing" | "ready" | "failed";
interface SyncMeta {
  written: number;
  skipped: number;
  preserved: number;
  removed: number;
  durationMs: number;
}

export function IDEFramePane({ projectID }: IDEFramePaneProps) {
  const muiTheme = useTheme();
  const isMobile = useMediaQuery(muiTheme.breakpoints.down("md"));
  const { authenticated } = useAuth();
  const shouldSyncProjectFiles = authenticated && projectID !== "demo";

  const url = useMemo(() => getOpenvscodeUrl(projectID), [projectID]);
  const folder = useMemo(() => getOpenvscodeFolder(projectID), [projectID]);
  const filesQuery = useProjectFilesQuery({
    variables: { id: projectID },
    skip: !projectID || isMobile || !shouldSyncProjectFiles,
    fetchPolicy: "cache-and-network",
  });
  const projectQuery = useProjectQuery({
    variables: { id: projectID },
    skip: !projectID || isMobile || !shouldSyncProjectFiles,
    fetchPolicy: "cache-first",
  });
  const projectName = projectQuery.data?.project?.name ?? "";

  // `nonce` doubles as React's key on the iframe so a retry forces a
  // full remount (and a fresh load timer).
  const [nonce, setNonce] = useState(0);
  const [status, setStatus] = useState<FrameStatus>("loading");
  const [syncStatus, setSyncStatus] = useState<SyncStatus>("idle");
  const [syncMeta, setSyncMeta] = useState<SyncMeta | null>(null);
  const [syncError, setSyncError] = useState<string | null>(null);
  const [frameLoadMs, setFrameLoadMs] = useState<number | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const frameStartedAt = useRef<number>(nowMs());

  useEffect(() => {
    if (shouldSyncProjectFiles) return;
    setSyncStatus("ready");
    setSyncError(null);
    setSyncMeta(null);
  }, [shouldSyncProjectFiles]);

  useEffect(() => {
    if (!shouldSyncProjectFiles || isMobile || !projectID || !filesQuery.data) return;
    const projectFiles = filesQuery.data.projectFiles;
    let alive = true;
    const sync = async () => {
      const startedAt = nowMs();
      setSyncStatus("syncing");
      setSyncError(null);
      try {
        const res = await fetch("/api/ide/sync", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            projectID,
            projectName,
            files: projectFiles.map((file) => ({
              path: file.path,
              content: file.content,
            })),
          }),
        });
        const payload = (await res.json().catch(() => ({}))) as {
          written?: number;
          skipped?: number;
          preserved?: number;
          removed?: number;
          error?: string;
        };
        if (!res.ok) {
          throw new Error(payload.error || `sync failed (${res.status})`);
        }
        if (!alive) return;
        setSyncMeta({
          written: payload.written ?? 0,
          skipped: payload.skipped ?? 0,
          preserved: payload.preserved ?? 0,
          removed: payload.removed ?? 0,
          durationMs: Math.max(0, Math.round(nowMs() - startedAt)),
        });
        setSyncStatus("ready");
      } catch (e) {
        if (!alive) return;
        setSyncError(e instanceof Error ? e.message : "sync failed");
        setSyncStatus("failed");
      }
    };
    void sync();
    return () => {
      alive = false;
    };
  }, [filesQuery.data, isMobile, projectID, nonce, shouldSyncProjectFiles]);

  useEffect(() => {
    if (isMobile) return;
    if (syncStatus !== "ready" && syncStatus !== "failed") return;
    let alive = true;
    frameStartedAt.current = nowMs();
    setFrameLoadMs(null);
    setStatus("loading");
    fetch(url, { cache: "no-store", mode: "no-cors" }).catch(() => {
      if (alive) setStatus("failed");
    });
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      // If we still haven't heard from the iframe by the timeout the
      // operator is almost certainly missing the `ide` compose
      // profile. Show the fallback panel with the exact command.
      setStatus((s) => (s === "loading" ? "failed" : s));
    }, LOAD_TIMEOUT_MS);
    return () => {
      alive = false;
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [nonce, url, isMobile, syncStatus]);

  const onLoad = useCallback(() => {
    setFrameLoadMs(Math.max(0, Math.round(nowMs() - frameStartedAt.current)));
    setStatus("ready");
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  // Bidirectional sync: once the iframe has loaded and the initial
  // Studio→IDE push finished, start polling the IDE workspace for
  // operator edits and mirror them back into projectFiles.
  const seedFiles = useMemo(
    () =>
      filesQuery.data?.projectFiles.map((f) => ({
        path: f.path,
        content: f.content ?? null,
      })) ?? null,
    [filesQuery.data],
  );
  const writeBack = useIDEWriteBack({
    projectID,
    enabled:
      shouldSyncProjectFiles &&
      !isMobile &&
      status === "ready" &&
      syncStatus === "ready" &&
      !!projectID,
    seedFiles,
  });

  const onRetry = useCallback(() => {
    setNonce((n) => n + 1);
  }, []);

  const openInNewTab = useCallback(() => {
    if (typeof window === "undefined") return;
    window.open(url, "_blank", "noopener,noreferrer");
  }, [url]);

  if (isMobile) {
    return <MobileIDECallout url={url} onOpen={openInNewTab} />;
  }

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        display: "flex",
        flex: 1,
        flexDirection: "column",
        height: "100%",
        minHeight: 0,
        minWidth: 0,
        position: "relative",
      }}
      role="region"
      aria-label="IDE pane"
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{
          alignItems: "center",
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.surface,
          minHeight: 36,
          px: 1.5,
          py: 0.5,
        }}
      >
        <Typography
          sx={{
            color: tokens.color.text.secondary,
            fontFamily: tokens.font.mono,
            fontSize: 11,
            fontWeight: 700,
            letterSpacing: 0.6,
            textTransform: "uppercase",
            flex: 1,
            minWidth: 0,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          openvscode · {folder}
          {frameLoadMs != null ? ` · load ${frameLoadMs}ms` : ""}
        </Typography>
        <Tooltip title={syncTooltip(syncStatus, syncMeta, syncError)} arrow>
          <Stack
            direction="row"
            spacing={0.5}
            sx={{
              alignItems: "center",
              color:
                syncStatus === "failed"
                  ? tokens.color.accent.warning
                  : syncStatus === "ready"
                    ? tokens.color.accent.success
                    : tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.4,
              textTransform: "uppercase",
            }}
          >
            <CloudSyncRounded sx={{ fontSize: 14 }} />
            <Box component="span" sx={{ display: { xs: "none", lg: "inline" } }}>
              {syncLabel(syncStatus, syncMeta)}
            </Box>
          </Stack>
        </Tooltip>
        <Tooltip title={writeBackTooltip(writeBack.status, writeBack.pushedFileCount, writeBack.lastPushedAt, writeBack.lastError)} arrow>
          <Stack
            direction="row"
            spacing={0.5}
            sx={{
              alignItems: "center",
              color: writeBackColor(writeBack.status),
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.4,
              textTransform: "uppercase",
            }}
          >
            <SyncAltRounded sx={{ fontSize: 14 }} />
            <Box component="span" sx={{ display: { xs: "none", lg: "inline" } }}>
              {writeBackLabel(writeBack.status, writeBack.pushedFileCount)}
            </Box>
          </Stack>
        </Tooltip>
        <Tooltip title="Reload IDE" arrow>
          <IconButton
            size="small"
            aria-label="Reload IDE"
            onClick={onRetry}
            sx={{
              color: tokens.color.text.secondary,
              "&:hover": { color: tokens.color.text.primary },
            }}
          >
            <RefreshRounded sx={{ fontSize: 16 }} />
          </IconButton>
        </Tooltip>
        <Tooltip title="Open IDE in new tab" arrow>
          <IconButton
            size="small"
            aria-label="Open IDE in new tab"
            onClick={openInNewTab}
            sx={{
              color: tokens.color.text.secondary,
              "&:hover": { color: tokens.color.text.primary },
            }}
          >
            <OpenInNewRounded sx={{ fontSize: 16 }} />
          </IconButton>
        </Tooltip>
      </Stack>

      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          minWidth: 0,
          position: "relative",
        }}
      >
        {syncStatus === "syncing" || syncStatus === "idle" || filesQuery.loading ? (
          <IDESkeleton label="Syncing project files…" />
        ) : status === "failed" ? (
          <IDEFallback url={url} onRetry={onRetry} onOpen={openInNewTab} />
        ) : (
          <>
            {status === "loading" ? <IDESkeleton label="Loading slim IDE…" /> : null}
            <Box
              key={nonce}
              component="iframe"
              src={url}
              title="Ironflyer IDE"
              onLoad={onLoad}
              // VS Code in the browser needs:
              //   • same-origin so service workers and IndexedDB work
              //   • scripts to run the editor
              //   • forms for the command palette
              //   • downloads so the operator can save files
              //   • popups for the "open file" dialog
              sandbox="allow-same-origin allow-scripts allow-forms allow-downloads allow-popups"
              allow="clipboard-read; clipboard-write; cross-origin-isolated"
              sx={{
                border: 0,
                display: "block",
                height: "100%",
                width: "100%",
                bgcolor: tokens.color.bg.base,
              }}
            />
          </>
        )}
      </Box>
    </Box>
  );
}

// IDESkeleton — token-driven loading panel that covers the iframe
// surface until `load` fires (or the timeout flips to `failed`).
// Branded as an Ironflyer mark + caption rather than a generic
// shimmer — the moment between Studio and IDE should feel like the
// same product, not a third-party tool warming up.
function IDESkeleton({ label = "Loading Ironflyer IDE…" }: { label?: string }) {
  return (
    <Box
      role="status"
      aria-busy="true"
      sx={{
        position: "absolute",
        inset: 0,
        bgcolor: tokens.color.bg.base,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: 2,
        p: 4,
        zIndex: 1,
        backgroundImage: `radial-gradient(circle at 50% 38%, ${tokens.color.accent.purple}33, transparent 55%)`,
      }}
    >
      <Box
        sx={{
          position: "relative",
          width: 56,
          height: 56,
          borderRadius: tokens.radius.sm / 4,
          border: `1px solid ${tokens.color.border.strong}`,
          background: `linear-gradient(135deg, ${tokens.color.accent.purple}, ${tokens.color.brand.magenta} 55%, ${tokens.color.accent.coral})`,
          boxShadow: `0 0 32px ${tokens.color.accent.violet}55, inset 0 0 12px ${tokens.color.accent.coral}44`,
          animation: `ironflyer-ide-pulse ${tokens.motion.slow} ease-in-out infinite`,
        }}
      >
        <Box
          sx={{
            position: "absolute",
            inset: 8,
            borderRadius: 0.5,
            border: `2px solid ${tokens.color.bg.base}`,
            background: tokens.color.bg.base,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: tokens.color.text.primary,
            fontFamily: tokens.font.mono,
            fontSize: 16,
            fontWeight: 800,
            letterSpacing: 1,
          }}
        >
          IF
        </Box>
      </Box>
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontFamily: tokens.font.mono,
          fontSize: 12,
          fontWeight: 700,
          letterSpacing: 1.4,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 10.5,
          letterSpacing: 0.8,
          maxWidth: 360,
          textAlign: "center",
          textTransform: "uppercase",
        }}
      >
        gates · patches · live preview · wallet · workspace
      </Typography>
      <Box
        component="style"
        // Keyframes injected once so the pulse plays without a global
        // stylesheet contribution.
        dangerouslySetInnerHTML={{
          __html:
            "@keyframes ironflyer-ide-pulse { 0%, 100% { transform: scale(1); opacity: 0.92; } 50% { transform: scale(1.06); opacity: 1; } }",
        }}
      />
    </Box>
  );
}

function syncLabel(
  status: SyncStatus,
  meta: SyncMeta | null,
): string {
  if (status === "ready" && meta) return `synced ${meta.written} · ${meta.durationMs}ms`;
  if (status === "failed") return "sync failed";
  if (status === "syncing") return "syncing";
  return "sync queued";
}

function syncTooltip(
  status: SyncStatus,
  meta: SyncMeta | null,
  error: string | null,
): string {
  if (status === "ready" && meta) {
    return [
      `${meta.written} files synced in ${meta.durationMs}ms`,
      meta.preserved ? `${meta.preserved} VS Code edits preserved` : "",
      meta.removed ? `${meta.removed} stale files removed` : "",
      meta.skipped ? `${meta.skipped} skipped` : "",
    ]
      .filter(Boolean)
      .join(", ");
  }
  if (status === "failed") return error || "Project sync failed";
  if (status === "syncing") return "Writing the Studio project snapshot into the IDE workspace";
  return "Waiting for project files";
}

function nowMs(): number {
  return typeof performance !== "undefined" ? performance.now() : Date.now();
}

function writeBackLabel(status: WriteBackStatus, count: number): string {
  if (status === "synced") return `pushed ${count}`;
  if (status === "pushing") return "pushing";
  if (status === "watching") return "watching";
  if (status === "failed") return "push failed";
  return "writeback idle";
}

function writeBackTooltip(
  status: WriteBackStatus,
  count: number,
  lastPushedAt: number | null,
  error: string | null,
): string {
  if (status === "failed") return error || "IDE writeback failed";
  if (status === "pushing") return "Mirroring IDE edits into projectFiles";
  if (status === "synced") {
    const stamp =
      lastPushedAt != null
        ? new Date(lastPushedAt).toLocaleTimeString()
        : "just now";
    return `${count} files pushed at ${stamp}`;
  }
  if (status === "watching") return "Polling IDE for operator edits";
  return "Bidirectional sync inactive";
}

function writeBackColor(status: WriteBackStatus): string {
  if (status === "failed") return tokens.color.accent.warning;
  if (status === "synced") return tokens.color.accent.success;
  if (status === "pushing") return tokens.color.accent.violet;
  return tokens.color.text.muted;
}

// IDEFallback — token-driven panel surfaced when the iframe never
// loads. The copy points the operator at the exact compose command so
// they recover in seconds.
function IDEFallback({
  url,
  onRetry,
  onOpen,
}: {
  url: string;
  onRetry: () => void;
  onOpen: () => void;
}) {
  return (
    <Box
      role="alert"
      sx={{
        position: "absolute",
        inset: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        bgcolor: tokens.color.bg.base,
        p: 3,
      }}
    >
      <Box
        sx={{
          maxWidth: 520,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.surface,
          borderRadius: 1.5,
          boxShadow: tokens.shadow.md,
          p: 3,
        }}
      >
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontSize: 15,
            fontWeight: 800,
          }}
        >
          Open the workspace IDE
        </Typography>
        <Typography
          sx={{
            mt: 1,
            color: tokens.color.text.secondary,
            fontSize: 13,
            lineHeight: 1.55,
          }}
        >
          The openvscode-server container didn&apos;t respond within{" "}
          {Math.round(LOAD_TIMEOUT_MS / 1000)}s at{" "}
          <Box
            component="span"
            sx={{
              color: tokens.color.text.primary,
              fontFamily: tokens.font.mono,
              fontSize: 12,
            }}
          >
            {url}
          </Box>
          . Start it with the dev compose profile, then retry.
        </Typography>
        <Box
          sx={{
            mt: 1.5,
            border: `1px solid ${tokens.color.border.subtle}`,
            bgcolor: tokens.color.bg.inset,
            borderRadius: 1,
            color: tokens.color.text.primary,
            fontFamily: tokens.font.mono,
            fontSize: 12,
            overflowX: "auto",
            px: 1.25,
            py: 1,
            whiteSpace: "pre",
          }}
        >
          {OPENVSCODE_DEV_COMMAND}
        </Box>
        <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
          <Button
            variant="contained"
            color="primary"
            startIcon={<RefreshRounded fontSize="small" />}
            onClick={onRetry}
          >
            Retry connection
          </Button>
          <Button
            variant="outlined"
            startIcon={<OpenInNewRounded fontSize="small" />}
            onClick={onOpen}
            sx={{
              borderColor: tokens.color.border.strong,
              color: tokens.color.text.primary,
              "&:hover": {
                borderColor: tokens.color.accent.violet,
                bgcolor: "transparent",
              },
            }}
          >
            Open in new tab
          </Button>
        </Stack>
      </Box>
    </Box>
  );
}

// MobileIDECallout — VS Code at xs/sm widths collapses to an
// unusable scroll soup. We surface a dedicated CTA that pops the IDE
// into a new tab where the browser's own zoom / address bar gives the
// operator a fighting chance.
function MobileIDECallout({
  url,
  onOpen,
}: {
  url: string;
  onOpen: () => void;
}) {
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={2}
      sx={{
        bgcolor: tokens.color.bg.base,
        flex: 1,
        height: "100%",
        p: 4,
        textAlign: "center",
      }}
    >
      <Typography
        sx={{
          color: tokens.color.text.primary,
          fontSize: 15,
          fontWeight: 800,
        }}
      >
        IDE is best on desktop
      </Typography>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontSize: 13,
          maxWidth: 320,
        }}
      >
        VS Code needs more room than a phone gives it. Pop it open in a new tab
        so you can keep working.
      </Typography>
      <Button
        variant="contained"
        color="primary"
        startIcon={<OpenInNewRounded fontSize="small" />}
        onClick={onOpen}
      >
        Open IDE in new tab
      </Button>
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 10.5,
          letterSpacing: 0.5,
          textTransform: "uppercase",
        }}
      >
        {url.replace(/^https?:\/\//, "")}
      </Typography>
    </Stack>
  );
}
