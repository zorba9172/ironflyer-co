"use client";

// FilesPane — Files tab of the studio. Two-column layout: tree of the
// live workspace on the left, viewer for the selected file on the
// right. The tree polls every 10s while the execution is still running
// so freshly-scaffolded files appear without a manual refresh; once
// the execution terminates we stop polling because the workspace is
// frozen.
//
// Data source: the runtime workspace HTTP API on port 8090. We bypass
// GraphQL entirely for file traffic — see apps/web/src/lib/runtime.ts.
//
// The orchestrator's Execution type does not (yet) surface a workspace
// id. Until it does, we derive workspaceID from the workspaceID prop
// the parent threads in, which in turn falls back to executionID. The
// runtime's create handler uses executionID as the workspace lease key
// when no explicit workspace id is supplied, so the convention is
// stable.
//
// Compromises:
//   - No diff view. We render whole files, not patch hunks. Diff lives
//     on the V22 roadmap; building it client-side would require either
//     the orchestrator exposing patches over GraphQL or shipping the
//     git history out of the workspace. Neither belongs in this PR.
//   - 256KB cap on reads. Anything larger renders truncated with a
//     footer note pointing the operator to the IDE.
//   - Binary files are detected via null-byte sniff and rendered as a
//     size badge — never as base64 — so the viewer never explodes.

import { Box, Stack, Typography } from "@mui/material";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { FileTree } from "./FileTree";
import { FileViewer } from "./FileViewer";
import { ResizableSplit } from "./ResizableSplit";
import { tokens } from "../../theme";
import {
  listWorkspaceFiles,
  readWorkspaceFile,
  type FileContent,
  type FileEntry,
} from "../../lib/runtime";

const TERMINAL = new Set([
  "succeeded",
  "failed",
  "stopped",
  "killed",
  "refunded",
]);

const POLL_INTERVAL_MS = 10_000;

export interface FilesPaneProps {
  executionID: string;
  executionStatus: string;
  // Optional explicit workspace id. When absent we fall back to
  // executionID — runtime's create handler uses executionID as the
  // workspace lease key when no override is supplied. Once the schema
  // surfaces bundle.workspaceID this prop will flow through with a
  // real value and the fallback becomes dead code.
  workspaceID?: string | null;
}

export function FilesPane({
  executionID,
  executionStatus,
  workspaceID,
}: FilesPaneProps) {
  const wsID = workspaceID && workspaceID.length > 0 ? workspaceID : executionID;
  const isTerminal = TERMINAL.has(executionStatus);

  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [listing, setListing] = useState<boolean>(true);
  const [listError, setListError] = useState<string | null>(null);

  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [content, setContent] = useState<FileContent | null>(null);
  const [reading, setReading] = useState<boolean>(false);
  const [readError, setReadError] = useState<string | null>(null);

  // Race-guard for the read fetch. When the operator clicks through
  // four files in a second, only the latest response should mutate
  // state — older ones are stale by the time they land.
  const readSeqRef = useRef(0);

  // Refresh the listing. Pulled into its own callback so both the
  // initial fetch and the 10s poll share one code path.
  const refreshList = useCallback(async () => {
    if (!wsID) return;
    try {
      const next = await listWorkspaceFiles(wsID);
      setEntries(next);
      setListError(null);
    } catch (e) {
      // First failure surfaces; later polls keep the last successful
      // listing on screen so a transient network blip doesn't blank
      // the tree.
      setListError(e instanceof Error ? e.message : String(e));
    } finally {
      setListing(false);
    }
  }, [wsID]);

  // Initial fetch + reset whenever the workspace target changes.
  useEffect(() => {
    setEntries([]);
    setListing(true);
    setListError(null);
    setSelectedPath(null);
    setContent(null);
    setReadError(null);
    if (!wsID) {
      setListing(false);
      return;
    }
    void refreshList();
  }, [wsID, refreshList]);

  // Poll the listing on a 10s cadence while the execution is running.
  // Once it terminates we stop — the workspace's file set is frozen
  // from the operator's perspective (the runtime may still mutate it,
  // but the studio reflects the bundle moment).
  useEffect(() => {
    if (!wsID || isTerminal) return;
    const id = window.setInterval(() => {
      void refreshList();
    }, POLL_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [wsID, isTerminal, refreshList]);

  // Fetch the selected file's contents whenever the user picks one.
  useEffect(() => {
    if (!wsID || !selectedPath) {
      setContent(null);
      setReadError(null);
      setReading(false);
      return;
    }
    const seq = ++readSeqRef.current;
    setReading(true);
    setReadError(null);
    readWorkspaceFile(wsID, selectedPath)
      .then((c) => {
        if (readSeqRef.current !== seq) return;
        setContent(c);
      })
      .catch((e) => {
        if (readSeqRef.current !== seq) return;
        setContent(null);
        setReadError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (readSeqRef.current !== seq) return;
        setReading(false);
      });
  }, [wsID, selectedPath]);

  const fileCount = useMemo(
    () => entries.filter((e) => e.kind === "file").length,
    [entries],
  );

  // ResizableSplit already gives us a draggable divider with persisted
  // position. Reusing it keeps the Files tab consistent with the rest
  // of the studio's split behaviour (chat ↔ workpad).
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        display: "flex",
        height: "100%",
        minHeight: 0,
        width: "100%",
      }}
    >
      <ResizableSplit
        defaultLeftPct={30}
        left={
          <Box
            sx={{
              borderRight: `1px solid ${tokens.color.border.subtle}`,
              display: "flex",
              flexDirection: "column",
              height: "100%",
              minHeight: 0,
            }}
          >
            <Stack
              direction="row"
              alignItems="center"
              sx={{
                borderBottom: `1px solid ${tokens.color.border.subtle}`,
                flexShrink: 0,
                px: 1.5,
                py: 1,
              }}
            >
              <Typography
                variant="overline"
                sx={{
                  color: tokens.color.text.secondary,
                  flex: 1,
                  letterSpacing: "0.12em",
                }}
              >
                Workspace files
              </Typography>
              <Typography
                sx={{
                  color: tokens.color.text.muted,
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                }}
              >
                {fileCount}
              </Typography>
            </Stack>
            <Box sx={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
              {listError && entries.length === 0 ? (
                <Stack
                  spacing={1}
                  sx={{
                    color: tokens.color.text.muted,
                    px: 1.5,
                    py: 2,
                    textAlign: "center",
                  }}
                >
                  <Typography
                    sx={{ color: tokens.color.accent.danger, fontSize: 12, fontWeight: 600 }}
                  >
                    Could not load files
                  </Typography>
                  <Typography sx={{ color: tokens.color.text.muted, fontSize: 11.5 }}>
                    {listError}
                  </Typography>
                </Stack>
              ) : (
                <FileTree
                  entries={entries}
                  selectedPath={selectedPath}
                  onSelect={setSelectedPath}
                  loading={listing}
                />
              )}
            </Box>
          </Box>
        }
        right={
          <Box sx={{ height: "100%", minHeight: 0 }}>
            <FileViewer
              workspaceID={wsID}
              selectedPath={selectedPath}
              content={content}
              loading={reading}
              error={readError}
            />
          </Box>
        }
      />
    </Box>
  );
}
