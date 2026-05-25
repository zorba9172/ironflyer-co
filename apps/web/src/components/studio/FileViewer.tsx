"use client";

// FileViewer — right pane of the Files tab. Renders the selected
// file's contents in a CodeBlock with a tiny header (path + size +
// "Open in editor" placeholder).
//
// Failure modes we have to handle gracefully:
//   - nothing selected → friendly empty state.
//   - loading → 5 skeleton rows.
//   - read error → ErrorPanel with the runtime message.
//   - binary file → never try to render, show byte count + tip.
//   - truncated → render the slice + a footer note.

import { OpenInNewRounded } from "@mui/icons-material";
import { Box, Skeleton, Stack, Typography } from "@mui/material";
import dynamic from "next/dynamic";
import { ErrorPanel } from "../cockpit/ErrorPanel";
import { tokens } from "../../theme";
import type { FileContent } from "../../lib/runtime";

// Lazy-load the Monaco wrapper — keeps the ~1MB editor bundle out
// of the FilesPane chunk until the user actually opens a file.
const MonacoFileView = dynamic(
  () => import("./MonacoFileView").then((m) => m.MonacoFileView),
  { ssr: false, loading: () => <EditorFallback /> },
);

function EditorFallback() {
  return (
    <Box
      sx={{
        alignItems: "center",
        bgcolor: tokens.color.bg.inset,
        color: tokens.color.text.muted,
        display: "flex",
        fontFamily: tokens.font.mono,
        fontSize: 11,
        height: "100%",
        justifyContent: "center",
        letterSpacing: 1,
        textTransform: "uppercase",
      }}
    >
      Loading editor…
    </Box>
  );
}

export interface FileViewerProps {
  workspaceID: string;
  selectedPath: string | null;
  content: FileContent | null;
  loading?: boolean;
  error?: string | null;
}

// Map common file extensions to the CodeBlock `language` hint. The
// block doesn't currently syntax-highlight, but threading the hint
// through means whenever it grows that capability, every file already
// announces what it is.
const LANG_MAP: Record<string, string> = {
  ts: "typescript",
  tsx: "tsx",
  js: "javascript",
  jsx: "jsx",
  json: "json",
  go: "go",
  py: "python",
  rs: "rust",
  md: "markdown",
  yml: "yaml",
  yaml: "yaml",
  toml: "toml",
  html: "html",
  css: "css",
  scss: "scss",
  sh: "bash",
  sql: "sql",
  graphql: "graphql",
  gql: "graphql",
  dockerfile: "dockerfile",
};

function languageFor(path: string): string | undefined {
  const lower = path.toLowerCase();
  const base = lower.split("/").pop() ?? lower;
  if (base === "dockerfile") return "dockerfile";
  const dot = lower.lastIndexOf(".");
  if (dot < 0) return undefined;
  return LANG_MAP[lower.slice(dot + 1)];
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

export function FileViewer({
  workspaceID,
  selectedPath,
  content,
  loading,
  error,
}: FileViewerProps) {
  if (!selectedPath) {
    return (
      <Stack
        alignItems="center"
        justifyContent="center"
        spacing={1.25}
        sx={{
          color: tokens.color.text.muted,
          height: "100%",
          px: 4,
          textAlign: "center",
        }}
      >
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.secondary, letterSpacing: "0.12em" }}
        >
          Files
        </Typography>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 13, maxWidth: 380 }}>
          Select a file to view its contents.
        </Typography>
      </Stack>
    );
  }

  return (
    <Stack sx={{ height: "100%", minHeight: 0 }}>
      <Header workspaceID={workspaceID} path={selectedPath} content={content} />
      {loading && !content ? (
        <Box sx={{ flex: 1, minHeight: 0, overflow: "auto", p: 2 }}>
          <Stack spacing={0.5}>
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton
                key={i}
                variant="rectangular"
                height={14}
                width={`${60 + ((i * 13) % 35)}%`}
                sx={{
                  bgcolor: tokens.color.bg.surfaceHover,
                  borderRadius: 0.5,
                }}
              />
            ))}
          </Stack>
        </Box>
      ) : error ? (
        <Box sx={{ flex: 1, minHeight: 0, overflow: "auto", p: 2 }}>
          <ErrorPanel title="Could not read file" error={error} />
        </Box>
      ) : content?.isBinary ? (
        <Box sx={{ flex: 1, minHeight: 0, overflow: "auto", p: 2 }}>
          <BinaryNotice size={content.size} />
        </Box>
      ) : content ? (
        <Box
          sx={{
            display: "flex",
            flex: 1,
            flexDirection: "column",
            minHeight: 0,
          }}
        >
          <MonacoFileView
            value={content.content || ""}
            language={languageFor(content.path)}
            path={content.path}
            readOnly
          />
          {content.truncated && (
            <Box
              sx={{
                bgcolor: `${tokens.color.accent.warning}10`,
                borderTop: `1px solid ${tokens.color.border.subtle}`,
                color: tokens.color.accent.warning,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                px: 2,
                py: 0.75,
              }}
            >
              Truncated at 256KB — open in IDE for the full file.
            </Box>
          )}
        </Box>
      ) : null}
    </Stack>
  );
}

interface HeaderProps {
  workspaceID: string;
  path: string;
  content: FileContent | null;
}

function Header({ workspaceID, path, content }: HeaderProps) {
  // "Open in editor" is a placeholder. There is no per-file deep link
  // into the runtime IDE yet, so the click is a no-op visually. When
  // the IDE ships a `/edit?path=...` route the href flips on without
  // touching this component's shape.
  const ideHref = `#open-in-editor:${workspaceID}:${path}`;

  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={1.5}
      sx={{
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: tokens.color.bg.surface,
        flexShrink: 0,
        px: 2,
        py: 1,
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
        title={path}
      >
        {path}
      </Typography>
      {content && (
        <Box
          sx={{
            bgcolor: tokens.color.bg.inset,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 0.75,
            color: tokens.color.text.muted,
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            px: 0.75,
            py: 0.25,
          }}
        >
          {formatBytes(content.size)}
        </Box>
      )}
      <Box
        component="a"
        href={ideHref}
        onClick={(e: React.MouseEvent) => e.preventDefault()}
        sx={{
          alignItems: "center",
          color: tokens.color.text.muted,
          display: "inline-flex",
          fontFamily: tokens.font.mono,
          fontSize: 11,
          gap: 0.5,
          textDecoration: "none",
          "&:hover": { color: tokens.color.accent.violet },
        }}
      >
        Open in editor
        <OpenInNewRounded sx={{ fontSize: 12 }} />
      </Box>
    </Stack>
  );
}

function BinaryNotice({ size }: { size: number }) {
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={1}
      sx={{
        color: tokens.color.text.muted,
        py: 6,
        textAlign: "center",
      }}
    >
      <Typography sx={{ color: tokens.color.text.primary, fontSize: 13, fontWeight: 600 }}>
        Binary file
      </Typography>
      <Typography sx={{ color: tokens.color.text.muted, fontSize: 12 }}>
        {formatBytes(size)}. View in the IDE for raw bytes.
      </Typography>
    </Stack>
  );
}
