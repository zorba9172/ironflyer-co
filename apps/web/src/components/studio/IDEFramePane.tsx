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
import { useProjectFilesQuery } from "../../lib/gql/__generated__";

const LOAD_TIMEOUT_MS = 6000;

export interface IDEFramePaneProps {
  projectID: string;
}

type FrameStatus = "loading" | "ready" | "failed";
type SyncStatus = "idle" | "syncing" | "ready" | "failed";

export function IDEFramePane({ projectID }: IDEFramePaneProps) {
  const muiTheme = useTheme();
  const isMobile = useMediaQuery(muiTheme.breakpoints.down("md"));

  const url = useMemo(() => getOpenvscodeUrl(projectID), [projectID]);
  const folder = useMemo(() => getOpenvscodeFolder(projectID), [projectID]);
  const filesQuery = useProjectFilesQuery({
    variables: { id: projectID },
    skip: !projectID || isMobile,
    fetchPolicy: "cache-and-network",
  });

  // `nonce` doubles as React's key on the iframe so a retry forces a
  // full remount (and a fresh load timer).
  const [nonce, setNonce] = useState(0);
  const [status, setStatus] = useState<FrameStatus>("loading");
  const [syncStatus, setSyncStatus] = useState<SyncStatus>("idle");
  const [syncMeta, setSyncMeta] = useState<{ written: number; skipped: number } | null>(null);
  const [syncError, setSyncError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (isMobile || !projectID || !filesQuery.data) return;
    const projectFiles = filesQuery.data.projectFiles;
    let alive = true;
    const sync = async () => {
      setSyncStatus("syncing");
      setSyncError(null);
      try {
        const res = await fetch("/api/ide/sync", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            projectID,
            files: projectFiles.map((file) => ({
              path: file.path,
              content: file.content,
            })),
          }),
        });
        const payload = (await res.json().catch(() => ({}))) as {
          written?: number;
          skipped?: number;
          error?: string;
        };
        if (!res.ok) {
          throw new Error(payload.error || `sync failed (${res.status})`);
        }
        if (!alive) return;
        setSyncMeta({
          written: payload.written ?? 0,
          skipped: payload.skipped ?? 0,
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
  }, [filesQuery.data, isMobile, projectID, nonce]);

  useEffect(() => {
    if (isMobile) return;
    if (syncStatus !== "ready" && syncStatus !== "failed") return;
    setStatus("loading");
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      // If we still haven't heard from the iframe by the timeout the
      // operator is almost certainly missing the `ide` compose
      // profile. Show the fallback panel with the exact command.
      setStatus((s) => (s === "loading" ? "failed" : s));
    }, LOAD_TIMEOUT_MS);
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [nonce, url, isMobile, syncStatus]);

  const onLoad = useCallback(() => {
    setStatus("ready");
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

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

// IDESkeleton — token-driven shimmer that covers the iframe surface
// until `load` fires (or the timeout flips to `failed`).
function IDESkeleton({ label = "Loading IDE…" }: { label?: string }) {
  return (
    <Box
      role="status"
      aria-busy="true"
      sx={{
        position: "absolute",
        inset: 0,
        bgcolor: tokens.color.bg.inset,
        display: "flex",
        flexDirection: "column",
        gap: 1,
        p: 2,
        zIndex: 1,
      }}
    >
      <Stack direction="row" spacing={1}>
        {Array.from({ length: 6 }).map((_, i) => (
          <Box
            key={i}
            sx={{
              flex: 1,
              height: 18,
              borderRadius: 0.5,
              bgcolor: tokens.color.bg.surface,
              backgroundImage: `linear-gradient(90deg, ${tokens.color.bg.surface} 0%, ${tokens.color.bg.surfaceHover} 50%, ${tokens.color.bg.surface} 100%)`,
              backgroundSize: "200% 100%",
              animation: `ironflyer-ide-shimmer ${tokens.motion.slow} ease-in-out infinite`,
            }}
          />
        ))}
      </Stack>
      <Box
        sx={{
          flex: 1,
          borderRadius: 1,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.surface,
          backgroundImage: `linear-gradient(90deg, ${tokens.color.bg.surface} 0%, ${tokens.color.bg.surfaceHover} 50%, ${tokens.color.bg.surface} 100%)`,
          backgroundSize: "200% 100%",
          animation: `ironflyer-ide-shimmer ${tokens.motion.slow} ease-in-out infinite`,
        }}
      />
      <Typography
        sx={{
          color: tokens.color.text.muted,
          fontFamily: tokens.font.mono,
          fontSize: 11,
          letterSpacing: 0.6,
          textAlign: "center",
          textTransform: "uppercase",
        }}
      >
        {label}
      </Typography>
      <Box
        component="style"
        // Keyframes injected once so the shimmer plays without a
        // global stylesheet contribution.
        dangerouslySetInnerHTML={{
          __html:
            "@keyframes ironflyer-ide-shimmer { 0% { background-position: 200% 0; } 100% { background-position: -200% 0; } }",
        }}
      />
    </Box>
  );
}

function syncLabel(
  status: SyncStatus,
  meta: { written: number; skipped: number } | null,
): string {
  if (status === "ready" && meta) return `synced ${meta.written}`;
  if (status === "failed") return "sync failed";
  if (status === "syncing") return "syncing";
  return "sync queued";
}

function syncTooltip(
  status: SyncStatus,
  meta: { written: number; skipped: number } | null,
  error: string | null,
): string {
  if (status === "ready" && meta) {
    return `${meta.written} files synced${meta.skipped ? `, ${meta.skipped} skipped` : ""}`;
  }
  if (status === "failed") return error || "Project sync failed";
  if (status === "syncing") return "Writing the Monaco project snapshot into the IDE workspace";
  return "Waiting for project files";
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
          docker compose -f infra/compose/docker-compose.dev.yml --profile ide up
          -d
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
