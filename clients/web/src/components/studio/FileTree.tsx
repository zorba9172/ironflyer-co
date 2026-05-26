"use client";

// FileTree — collapsible tree view over the runtime's flat file list.
// We build a nested node graph once with useMemo, then render with a
// recursive component. Folders sort before files, alphabetically; the
// file's extension drives a tiny colored badge so the operator can scan
// a tree without an icon font.
//
// Selection is controlled — the parent owns `selectedPath` so the
// viewer pane stays the single source of truth for "what is open".

import { ChevronRightRounded, ExpandMoreRounded } from "@mui/icons-material";
import { Box, Skeleton, Stack, Typography } from "@mui/material";
import { useMemo, useState } from "react";
import { tokens } from "../../theme";
import { languageBadges } from "../../../../../packages/design-tokens/languages";
import type { FileEntry } from "../../lib/runtime";

export interface FileTreeProps {
  entries: FileEntry[];
  selectedPath: string | null;
  onSelect: (path: string) => void;
  loading?: boolean;
}

interface TreeNode {
  name: string;
  path: string;
  kind: "file" | "dir";
  size?: number;
  children: TreeNode[];
}

// buildTree folds a flat path list into a directory tree. The runtime
// only emits leaf entries (files + explicit dirs), so we synthesize
// intermediate directory nodes on the fly when a file's parent doesn't
// appear in the raw listing. That keeps the tree correct even when the
// driver returns the workspace as "src/app/page.tsx" without listing
// src/ or src/app/ explicitly.
function buildTree(entries: FileEntry[]): TreeNode {
  const root: TreeNode = { name: "", path: "", kind: "dir", children: [] };
  const dirIndex = new Map<string, TreeNode>();
  dirIndex.set("", root);

  // Two-pass: first materialise every directory (real or implied), then
  // attach files. This guarantees we never accidentally double-create a
  // directory because its file appeared before a sibling subdir.
  const ensureDir = (path: string): TreeNode => {
    if (dirIndex.has(path)) return dirIndex.get(path)!;
    const parts = path.split("/");
    const name = parts[parts.length - 1];
    const parentPath = parts.slice(0, -1).join("/");
    const parent = ensureDir(parentPath);
    const node: TreeNode = {
      name,
      path,
      kind: "dir",
      children: [],
    };
    parent.children.push(node);
    dirIndex.set(path, node);
    return node;
  };

  for (const e of entries) {
    const clean = e.path.replace(/^\/+|\/+$/g, "");
    if (!clean) continue;
    const parts = clean.split("/");
    if (e.kind === "dir") {
      ensureDir(clean);
      continue;
    }
    const parentPath = parts.slice(0, -1).join("/");
    const parent = ensureDir(parentPath);
    parent.children.push({
      name: parts[parts.length - 1],
      path: clean,
      kind: "file",
      size: e.size,
      children: [],
    });
  }

  const sort = (n: TreeNode) => {
    n.children.sort((a, b) => {
      if (a.kind !== b.kind) return a.kind === "dir" ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
    n.children.forEach(sort);
  };
  sort(root);
  return root;
}

function badgeFor(name: string): { label: string; color: string } {
  const lower = name.toLowerCase();
  if (lower === "dockerfile" || lower.endsWith(".dockerfile")) {
    return languageBadges.dockerfile;
  }
  const dot = lower.lastIndexOf(".");
  if (dot < 0) return { label: "•", color: tokens.color.text.muted };
  const ext = lower.slice(dot + 1);
  return (
    languageBadges[ext] ?? {
      label: ext.slice(0, 3).toUpperCase(),
      color: tokens.color.text.muted,
    }
  );
}

interface NodeRowProps {
  node: TreeNode;
  depth: number;
  selected: string | null;
  onSelect: (path: string) => void;
}

function NodeRow({ node, depth, selected, onSelect }: NodeRowProps) {
  // Top-level (depth 0) directories open by default — the user's first
  // glance should reveal at least one tier of structure. Deeper folders
  // start closed so a 200-file tree fits on screen.
  const [open, setOpen] = useState(depth === 0 && node.kind === "dir");
  const isSelected = selected === node.path;
  const isDir = node.kind === "dir";

  return (
    <Box>
      <Stack
        direction="row"
        spacing={0.5}
        sx={{
          alignItems: "center",
          borderLeft: `2px solid ${
            isSelected ? tokens.color.accent.violet : "transparent"
          }`,
          color: isSelected
            ? tokens.color.text.primary
            : tokens.color.text.secondary,
          cursor: "pointer",
          pl: 0.5 + depth * 1.25,
          pr: 1,
          py: 0.4,
          bgcolor: isSelected
            ? tokens.color.bg.surfaceHover
            : "transparent",
          "&:hover": {
            bgcolor: tokens.color.bg.surfaceHover,
            color: tokens.color.text.primary,
          },
        }}
        onClick={() => {
          if (isDir) setOpen((v) => !v);
          else onSelect(node.path);
        }}
      >
        {isDir ? (
          open ? (
            <ExpandMoreRounded sx={{ fontSize: 14, opacity: 0.7 }} />
          ) : (
            <ChevronRightRounded sx={{ fontSize: 14, opacity: 0.7 }} />
          )
        ) : (
          <Badge name={node.name} />
        )}
        <Typography
          sx={{
            color: "inherit",
            flex: 1,
            fontFamily: tokens.font.mono,
            fontSize: 12,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {node.name}
        </Typography>
      </Stack>
      {isDir && open && (
        <Box>
          {node.children.map((c) => (
            <NodeRow
              key={c.path}
              node={c}
              depth={depth + 1}
              selected={selected}
              onSelect={onSelect}
            />
          ))}
        </Box>
      )}
    </Box>
  );
}

function Badge({ name }: { name: string }) {
  const b = badgeFor(name);
  return (
    <Box
      sx={{
        alignItems: "center",
        bgcolor: b.color + "22",
        border: `1px solid ${b.color}55`,
        borderRadius: 0.5,
        color: b.color,
        display: "inline-flex",
        flexShrink: 0,
        fontFamily: tokens.font.mono,
        fontSize: 8.5,
        fontWeight: 700,
        height: 14,
        justifyContent: "center",
        letterSpacing: "0.04em",
        minWidth: 26,
        px: 0.5,
      }}
    >
      {b.label}
    </Box>
  );
}

export function FileTree({
  entries,
  selectedPath,
  onSelect,
  loading,
}: FileTreeProps) {
  const tree = useMemo(() => buildTree(entries), [entries]);

  if (loading && entries.length === 0) {
    return (
      <Stack spacing={0.5} sx={{ px: 1.25, py: 1 }}>
        {[0, 1, 2].map((i) => (
          <Skeleton
            key={i}
            variant="rectangular"
            height={16}
            sx={{
              bgcolor: tokens.color.bg.surfaceHover,
              borderRadius: 0.5,
            }}
          />
        ))}
      </Stack>
    );
  }

  if (entries.length === 0) {
    return (
      <Stack
        alignItems="center"
        justifyContent="center"
        spacing={1}
        sx={{
          color: tokens.color.text.muted,
          height: "100%",
          px: 2,
          py: 3,
          textAlign: "center",
        }}
      >
        <Typography sx={{ fontSize: 12.5, lineHeight: 1.6 }}>
          No files yet — the agent is still scaffolding.
        </Typography>
      </Stack>
    );
  }

  return (
    <Box sx={{ py: 0.5 }}>
      {tree.children.map((c) => (
        <NodeRow
          key={c.path}
          node={c}
          depth={0}
          selected={selectedPath}
          onSelect={onSelect}
        />
      ))}
    </Box>
  );
}
