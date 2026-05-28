"use client";

// CodePane — the live code reader for the studio.
//
// Layout: file tree on the left (sourced from `project.files`), a
// code reader on the right. Files whose path appears in
// `executionSupportBundle.changedFiles` get a lime dot and a
// "changed" pill so the user instantly sees what this run wrote vs.
// what was carried over.
//
// Per-file panel:
//   • Header — path, language, size, "changed" pill, patch reference
//     (which most recent patch touched this file), copy-path, View
//     raw (downloads the buffer as a Blob URL into a new tab), and a
//     download button.
//   • Body — read-only line-numbered viewer.
//   • Diff banner — the orchestrator does not snapshot a
//     pre-execution baseline today, so we surface the message
//     "Diff coming soon — workspace snapshots not yet baselined."
//     for files known to have changed. Once snapshots ship we'll
//     replace the banner with a real unified diff using diffUtil.
//
// All queries are raw gql tags (project files + patches) so the
// pane can ship without touching the codegen pipeline.

import {
  ArchiveRounded,
  ArticleOutlined,
  ContentCopyRounded,
  DownloadRounded,
  FolderOutlined,
  InfoOutlined,
  MapRounded,
  OpenInNewRounded,
  RefreshRounded,
  WrapTextRounded,
} from "@mui/icons-material";
import {
  Box,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
// JSZip is ~30 KB gzip; importing it eagerly here ships the deflate
// machinery into every cold load of the studio Code tab even though
// it only runs when the operator clicks "Download project ZIP". The
// import is deferred to `downloadProjectZip()` below.
import { useEffect, useMemo, useState } from "react";
import { LoadingPanel } from "../cockpit/LoadingPanel";
import {
  useExecutionSupportBundleQuery,
  usePatchesQuery,
  useProjectFilesQuery,
  type PatchCoreFragment,
  type ProjectFilesQuery,
} from "../../lib/gql/__generated__";
import { tokens } from "../../theme";
import { CodeTextView } from "./CodeTextView";

type ProjectFile = ProjectFilesQuery["projectFiles"][number];
type PatchLite = PatchCoreFragment;

interface FileNode {
  name: string;
  path: string;
  children: FileNode[];
  isFile: boolean;
}

const TERMINAL = new Set(["succeeded", "failed", "stopped", "killed", "refunded"]);

function buildTree(paths: string[]): FileNode {
  const root: FileNode = { name: "", path: "", children: [], isFile: false };
  for (const raw of paths) {
    const parts = raw.split("/").filter(Boolean);
    let cursor = root;
    let acc = "";
    parts.forEach((part, i) => {
      acc = acc ? `${acc}/${part}` : part;
      const isLeaf = i === parts.length - 1;
      let next = cursor.children.find((c) => c.name === part);
      if (!next) {
        next = { name: part, path: acc, children: [], isFile: isLeaf };
        cursor.children.push(next);
      } else if (isLeaf) {
        next.isFile = true;
      }
      cursor = next;
    });
  }
  const sort = (n: FileNode) => {
    n.children.sort((a, b) => {
      if (a.isFile !== b.isFile) return a.isFile ? 1 : -1;
      return a.name.localeCompare(b.name);
    });
    n.children.forEach(sort);
  };
  sort(root);
  return root;
}

function formatBytes(n: number | null): string {
  if (!n) return "—";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

function safeZipName(projectID: string): string {
  const base = projectID.trim() || "project";
  return `${base.replace(/[^a-zA-Z0-9._-]+/g, "-").slice(0, 48)}.zip`;
}

async function downloadProjectZip(projectID: string, files: ProjectFile[]): Promise<void> {
  // Dynamic import — keeps the ~30 KB jszip chunk out of the cold
  // studio bundle. The browser only fetches it the first time the
  // operator clicks the export button.
  const { default: JSZip } = await import("jszip");
  const zip = new JSZip();
  const skipped: string[] = [];

  zip.file(
    ".ironflyer-export.json",
    JSON.stringify(
      {
        projectID,
        exportedAt: new Date().toISOString(),
        fileCount: files.length,
      },
      null,
      2,
    ),
  );

  for (const file of files) {
    if (file.content == null) {
      skipped.push(file.path);
      continue;
    }
    zip.file(file.path, file.content);
  }

  if (skipped.length > 0) {
    zip.file(
      ".ironflyer-skipped-files.txt",
      [
        "These files were present in the project listing but had no text payload in GraphQL.",
        "Use the full IDE/runtime export path for binary payloads.",
        "",
        ...skipped,
      ].join("\n"),
    );
  }

  const blob = await zip.generateAsync({
    compression: "DEFLATE",
    compressionOptions: { level: 6 },
    type: "blob",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = safeZipName(projectID);
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// languageFromPath — quick file-extension → display label. The
// projectFiles resolver already returns a language hint, but only
// when the orchestrator's blueprint planner tagged it; this is the
// fallback for everything else.
function languageFromPath(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() ?? "";
  switch (ext) {
    case "ts":
    case "tsx":
      return "typescript";
    case "js":
    case "jsx":
      return "javascript";
    case "go":
      return "go";
    case "py":
      return "python";
    case "rs":
      return "rust";
    case "java":
      return "java";
    case "rb":
      return "ruby";
    case "md":
      return "markdown";
    case "json":
      return "json";
    case "yml":
    case "yaml":
      return "yaml";
    case "toml":
      return "toml";
    case "sql":
      return "sql";
    case "sh":
    case "bash":
      return "shell";
    case "html":
      return "html";
    case "css":
      return "css";
    case "scss":
      return "scss";
    case "graphql":
    case "gql":
      return "graphql";
    case "":
      return "text";
    default:
      return ext;
  }
}

function NodeRow({
  node,
  depth,
  onPick,
  selected,
  changed,
}: {
  node: FileNode;
  depth: number;
  onPick: (path: string) => void;
  selected: string | null;
  changed: Set<string>;
}) {
  const [open, setOpen] = useState(depth < 2);
  const isSelected = selected === node.path;
  const isChanged = changed.has(node.path);
  return (
    <Box>
      <Stack
        direction="row"
        spacing={0.75}
        sx={{
          alignItems: "center",
          borderRadius: 0.75,
          bgcolor: isSelected ? tokens.color.bg.surfaceHover : "transparent",
          color: isSelected ? tokens.color.accent.violet : tokens.color.text.secondary,
          cursor: "pointer",
          pl: 0.75 + depth * 1.25,
          pr: 1,
          py: 0.45,
          "&:hover": {
            bgcolor: tokens.color.bg.surfaceHover,
            color: tokens.color.text.primary,
          },
        }}
        onClick={() => {
          if (node.isFile) onPick(node.path);
          else setOpen((v) => !v);
        }}
      >
        {node.isFile ? (
          <ArticleOutlined sx={{ fontSize: 14, opacity: 0.8 }} />
        ) : (
          <FolderOutlined sx={{ fontSize: 14, opacity: 0.8 }} />
        )}
        <Typography
          sx={{
            color: "inherit",
            fontFamily: tokens.font.mono,
            fontSize: 12,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            flex: 1,
            fontWeight: isChanged ? 700 : 400,
          }}
        >
          {node.name}
        </Typography>
        {node.isFile && isChanged ? (
          <Box
            sx={{
              bgcolor: tokens.color.accent.violet,
              borderRadius: "50%",
              flex: "0 0 auto",
              height: 6,
              width: 6,
            }}
            aria-label="Changed in this run"
          />
        ) : null}
      </Stack>
      {!node.isFile && open && (
        <Box>
          {node.children.map((c) => (
            <NodeRow
              key={c.path}
              node={c}
              depth={depth + 1}
              onPick={onPick}
              selected={selected}
              changed={changed}
            />
          ))}
        </Box>
      )}
    </Box>
  );
}

export interface CodePaneProps {
  projectID: string;
  executionID: string;
  executionStatus: string;
}

export function CodePane({ projectID, executionID, executionStatus }: CodePaneProps) {
  const isTerminal = TERMINAL.has(executionStatus);

  const filesQuery = useProjectFilesQuery({
    variables: { id: projectID },
    skip: !projectID,
    fetchPolicy: "cache-and-network",
    pollInterval: isTerminal ? 0 : 6000,
  });

  const bundleQuery = useExecutionSupportBundleQuery({
    variables: { executionID },
    skip: !executionID,
    pollInterval: isTerminal ? 0 : 6000,
    fetchPolicy: "cache-and-network",
  });

  const patchesQuery = usePatchesQuery({
    variables: { projectId: projectID },
    skip: !projectID,
    fetchPolicy: "cache-and-network",
    pollInterval: isTerminal ? 0 : 8000,
  });

  const files = filesQuery.data?.projectFiles ?? [];
  const changed = useMemo(
    () => new Set(bundleQuery.data?.executionSupportBundle?.changedFiles ?? []),
    [bundleQuery.data],
  );

  // Build a path → most-recent patch lookup. The patches query returns
  // newest first, so the first hit wins.
  const patchByPath = useMemo(() => {
    const out = new Map<string, PatchLite>();
    for (const p of patchesQuery.data?.patches ?? []) {
      for (const c of p.changes) {
        if (!out.has(c.path)) out.set(c.path, p);
      }
    }
    return out;
  }, [patchesQuery.data]);

  const tree = useMemo(() => buildTree(files.map((f) => f.path)), [files]);

  const [selected, setSelected] = useState<string | null>(null);
  const [wordWrap, setWordWrap] = useState(false);
  const [minimap, setMinimap] = useState(false);
  const [exportingZip, setExportingZip] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  // Auto-pick the first changed file (or first file overall) when the
  // pane opens so the right column is never blank.
  useEffect(() => {
    if (selected) return;
    if (files.length === 0) return;
    const first = files.find((f) => changed.has(f.path)) ?? files[0];
    setSelected(first.path);
  }, [files, changed, selected]);

  const selectedFile = files.find((f) => f.path === selected) ?? null;
  const selectedPatch = selected ? patchByPath.get(selected) ?? null : null;

  const onDownloadZip = async () => {
    if (typeof window === "undefined" || files.length === 0 || exportingZip) return;
    setExportError(null);
    setExportingZip(true);
    try {
      await downloadProjectZip(projectID, files);
    } catch (e) {
      setExportError(e instanceof Error ? e.message : "Could not create ZIP");
    } finally {
      setExportingZip(false);
    }
  };

  if (filesQuery.loading && files.length === 0) {
    return <LoadingPanel label="Loading project files…" minHeight="100%" />;
  }

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        display: "grid",
        gridTemplateColumns: { xs: "1fr", md: "minmax(220px, 320px) 1fr" },
        height: "100%",
        minHeight: 0,
      }}
    >
      {/* Tree */}
      <Box
        sx={{
          borderRight: `1px solid ${tokens.color.border.subtle}`,
          display: "flex",
          flexDirection: "column",
          minHeight: 0,
        }}
      >
        <Stack
          direction="row"
          alignItems="center"
          sx={{
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
            px: 1.5,
            py: 1,
          }}
        >
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.secondary, flex: 1 }}
          >
            Code
          </Typography>
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11,
              mr: 0.5,
            }}
          >
            {files.length}
          </Typography>
          <Tooltip title="Download project ZIP" arrow>
            <span>
              <IconButton
                size="small"
                onClick={() => void onDownloadZip()}
                disabled={files.length === 0 || exportingZip}
                sx={{ color: tokens.color.text.secondary, p: 0.25 }}
                aria-label="Download project ZIP"
              >
                <ArchiveRounded sx={{ fontSize: 14 }} />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title="Refresh" arrow>
            <IconButton
              size="small"
              onClick={() => {
                void filesQuery.refetch();
                void bundleQuery.refetch();
                void patchesQuery.refetch();
              }}
              sx={{ color: tokens.color.text.secondary, p: 0.25 }}
              aria-label="Refresh files"
            >
              <RefreshRounded sx={{ fontSize: 14 }} />
            </IconButton>
          </Tooltip>
        </Stack>
        <Box sx={{ flex: 1, overflowY: "auto", py: 0.5 }}>
          {exportError ? (
            <Typography
              sx={{
                color: tokens.color.accent.warning,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                px: 1.5,
                py: 0.75,
              }}
            >
              ZIP failed: {exportError}
            </Typography>
          ) : null}
          {files.length === 0 ? (
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontSize: 12,
                px: 1.5,
                py: 2,
                textAlign: "center",
              }}
            >
              No files yet — the finisher will write them as soon as the first
              gate passes.
            </Typography>
          ) : (
            tree.children.map((c) => (
              <NodeRow
                key={c.path}
                node={c}
                depth={0}
                onPick={setSelected}
                selected={selected}
                changed={changed}
              />
            ))
          )}
        </Box>
      </Box>

      {/* Content viewer */}
      <Box sx={{ display: "flex", flexDirection: "column", minHeight: 0 }}>
        {selectedFile ? (
          <FileViewer
            projectID={projectID}
            file={selectedFile}
            changed={changed.has(selectedFile.path)}
            patch={selectedPatch}
            wordWrap={wordWrap}
            minimap={minimap}
            onToggleWordWrap={() => setWordWrap((v) => !v)}
            onToggleMinimap={() => setMinimap((v) => !v)}
          />
        ) : (
          <Stack
            alignItems="center"
            justifyContent="center"
            spacing={1}
            sx={{ color: tokens.color.text.muted, height: "100%", textAlign: "center", p: 4 }}
          >
            <Typography variant="overline" sx={{ color: tokens.color.text.secondary }}>
              Select a file to view
            </Typography>
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13, maxWidth: 360 }}>
              Files written by the finisher appear in the tree on the left.
              Click one to read it here.
            </Typography>
          </Stack>
        )}
      </Box>
    </Box>
  );
}

function FileViewer({
  projectID,
  file,
  changed,
  patch,
  wordWrap,
  minimap,
  onToggleWordWrap,
  onToggleMinimap,
}: {
  projectID: string;
  file: ProjectFile;
  changed: boolean;
  patch: PatchLite | null;
  wordWrap: boolean;
  minimap: boolean;
  onToggleWordWrap: () => void;
  onToggleMinimap: () => void;
}) {
  const language = file.language?.trim() || languageFromPath(file.path);

  const openRaw = () => {
    if (typeof window === "undefined" || file.content == null) return;
    const blob = new Blob([file.content], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    window.open(url, "_blank", "noopener,noreferrer");
    // Revoke after a short delay so the new tab has a chance to fetch.
    window.setTimeout(() => URL.revokeObjectURL(url), 30_000);
  };

  const download = () => {
    if (typeof window === "undefined" || file.content == null) return;
    const blob = new Blob([file.content], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = file.path.split("/").pop() ?? "file";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  return (
    <>
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.surface,
          px: 1.5,
          py: 0.75,
        }}
      >
        <Typography
          sx={{
            color: tokens.color.text.primary,
            flex: 1,
            fontFamily: tokens.font.mono,
            fontSize: 12.5,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {file.path}
        </Typography>
        {changed ? (
          <Box
            sx={{
              bgcolor: `${tokens.color.accent.violet}22`,
              border: `1px solid ${tokens.color.accent.violet}55`,
              borderRadius: 0.75,
              color: tokens.color.accent.violet,
              fontFamily: tokens.font.mono,
              fontSize: 10,
              fontWeight: 700,
              letterSpacing: 0.6,
              px: 0.75,
              py: 0.25,
              textTransform: "uppercase",
            }}
          >
            changed
          </Box>
        ) : null}
        {patch ? (
          <Tooltip
            title={`Most recent patch on this file — ${patch.status}`}
            arrow
          >
            <Box
              sx={{
                bgcolor: tokens.color.bg.inset,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: 0.75,
                color: tokens.color.text.secondary,
                fontFamily: tokens.font.mono,
                fontSize: 10,
                fontWeight: 700,
                letterSpacing: 0.6,
                px: 0.75,
                py: 0.25,
                textTransform: "uppercase",
              }}
            >
              patch {patch.id.slice(0, 6)}
            </Box>
          </Tooltip>
        ) : null}
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            letterSpacing: 0.4,
          }}
        >
          {language} · {formatBytes(file.size)}
        </Typography>
        <Tooltip title={wordWrap ? "Disable word wrap" : "Enable word wrap"} arrow>
          <IconButton
            size="small"
            onClick={onToggleWordWrap}
            sx={{
              bgcolor: wordWrap ? `${tokens.color.accent.violet}22` : "transparent",
              color: wordWrap ? tokens.color.accent.violet : tokens.color.text.secondary,
              p: 0.25,
            }}
            aria-label="Toggle word wrap"
          >
            <WrapTextRounded sx={{ fontSize: 14 }} />
          </IconButton>
        </Tooltip>
        <Tooltip title={minimap ? "Hide minimap" : "Show minimap"} arrow>
          <IconButton
            size="small"
            onClick={onToggleMinimap}
            sx={{
              bgcolor: minimap ? `${tokens.color.accent.violet}22` : "transparent",
              color: minimap ? tokens.color.accent.violet : tokens.color.text.secondary,
              p: 0.25,
            }}
            aria-label="Toggle minimap"
          >
            <MapRounded sx={{ fontSize: 14 }} />
          </IconButton>
        </Tooltip>
        <Tooltip title="Copy path" arrow>
          <IconButton
            size="small"
            onClick={() => {
              if (typeof navigator !== "undefined") {
                void navigator.clipboard.writeText(file.path).catch(() => undefined);
              }
            }}
            sx={{ color: tokens.color.text.secondary, p: 0.25 }}
            aria-label="Copy file path"
          >
            <ContentCopyRounded sx={{ fontSize: 14 }} />
          </IconButton>
        </Tooltip>
        <Tooltip title="View raw" arrow>
          <span>
            <IconButton
              size="small"
              onClick={openRaw}
              disabled={file.content == null}
              sx={{ color: tokens.color.text.secondary, p: 0.25 }}
              aria-label="View raw"
            >
              <OpenInNewRounded sx={{ fontSize: 14 }} />
            </IconButton>
          </span>
        </Tooltip>
        <Tooltip title="Download" arrow>
          <span>
            <IconButton
              size="small"
              onClick={download}
              disabled={file.content == null}
              sx={{ color: tokens.color.text.secondary, p: 0.25 }}
              aria-label="Download file"
            >
              <DownloadRounded sx={{ fontSize: 14 }} />
            </IconButton>
          </span>
        </Tooltip>
      </Stack>

      {/* Diff banner — workspace snapshots not yet baselined. */}
      {changed ? <DiffBanner /> : null}

      <Box
        sx={{
          bgcolor: tokens.color.bg.inset,
          flex: 1,
          minHeight: 0,
          overflow: "hidden",
          position: "relative",
        }}
      >
        {file.content == null ? (
          <Stack
            alignItems="center"
            justifyContent="center"
            spacing={1}
            sx={{
              color: tokens.color.text.muted,
              height: "100%",
              p: 4,
              textAlign: "center",
            }}
          >
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 12,
              }}
            >
              Binary or no content stored for this path.
            </Typography>
          </Stack>
        ) : (
          <CodeTextView
            value={file.content}
            language={language}
            path={file.path}
            wordWrap={wordWrap}
          />
        )}
      </Box>
    </>
  );
}

function DiffBanner() {
  return (
    <Stack
      direction="row"
      spacing={0.75}
      sx={{
        alignItems: "center",
        bgcolor: `${tokens.color.accent.sky}10`,
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
        color: tokens.color.accent.sky,
        px: 1.5,
        py: 0.5,
      }}
      role="status"
    >
      <InfoOutlined sx={{ fontSize: 14 }} />
      <Typography
        sx={{
          color: tokens.color.text.secondary,
          fontFamily: tokens.font.mono,
          fontSize: 11,
          letterSpacing: 0.3,
        }}
      >
        Diff coming soon — workspace snapshots not yet baselined.
      </Typography>
    </Stack>
  );
}
