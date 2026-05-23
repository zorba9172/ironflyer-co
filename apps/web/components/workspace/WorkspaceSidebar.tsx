'use client';

// WorkspaceSidebar — the 320px left rail. Three collapsible sections:
//   1. Project tree (files from the runtime workspace),
//   2. Finisher gates timeline (status pill per gate),
//   3. Patches list (clickable → emits onSelectPatch).
//
// All sections share the same visual rhythm: tiny overline header, lime
// accent on active items, alabaster surface. Loading and empty states are
// styled, never blank.

import { useState } from 'react';
import {
  Box, Chip, Collapse, IconButton, Skeleton, Stack, Tooltip, Typography,
} from '@mui/material';
import {
  ChevronRight, ExpandMore, Description as FileIcon, Folder, Layers,
  Difference,
} from '@mui/icons-material';
import { GateState, Project } from '../../lib/api';
import { FileEntry, Workspace } from '../../lib/runtime';
import { Patch } from '../../lib/api/patches';
import { tokens } from '../../lib/theme';

interface Props {
  project: Project;
  workspace: Workspace | null;
  files: FileEntry[];
  filesLoading: boolean;
  filesError: string | null;
  selectedFile: string | null;
  onSelectFile: (path: string) => void;
  gateOrder: { key: string; label: string }[];
  patches: Patch[];
  patchesLoading: boolean;
  patchesError: string | null;
  onSelectPatch: (patchId: string) => void;
  onRetryFiles: () => void;
  onRetryPatches: () => void;
}

export function WorkspaceSidebar({
  project, workspace, files, filesLoading, filesError, selectedFile, onSelectFile,
  gateOrder, patches, patchesLoading, patchesError, onSelectPatch,
  onRetryFiles, onRetryPatches,
}: Props) {
  const [openTree, setOpenTree] = useState(true);
  const [openGates, setOpenGates] = useState(true);
  const [openPatches, setOpenPatches] = useState(true);

  return (
    <Stack spacing={0.8} sx={{ height: '100%', minHeight: 0, overflowY: 'auto', pr: 0.3 }}>
      <Section
        title="Files"
        icon={<Folder fontSize="small" />}
        open={openTree}
        onToggle={() => setOpenTree((v) => !v)}
        meta={workspace ? `${files.length}` : 'No runtime'}
      >
        <FileTree
          workspace={workspace}
          files={files}
          loading={filesLoading}
          error={filesError}
          selected={selectedFile}
          onSelect={onSelectFile}
          onRetry={onRetryFiles}
        />
      </Section>

      <Section
        title="Finisher gates"
        icon={<Layers fontSize="small" />}
        open={openGates}
        onToggle={() => setOpenGates((v) => !v)}
        meta={gateMetaLabel(project, gateOrder)}
      >
        <Stack spacing={0.6}>
          {gateOrder.map(({ key, label }) => (
            <GateRow
              key={key}
              label={label}
              state={project.gates[key as keyof typeof project.gates] as GateState | undefined}
            />
          ))}
        </Stack>
      </Section>

      <Section
        title="Patches"
        icon={<Difference fontSize="small" />}
        open={openPatches}
        onToggle={() => setOpenPatches((v) => !v)}
        meta={patchesLoading ? '…' : `${patches.length}`}
      >
        <PatchList
          patches={patches}
          loading={patchesLoading}
          error={patchesError}
          onSelect={onSelectPatch}
          onRetry={onRetryPatches}
        />
      </Section>
    </Stack>
  );
}

function Section({
  title, icon, open, onToggle, meta, children,
}: {
  title: string; icon: React.ReactNode; open: boolean; onToggle: () => void;
  meta?: string; children: React.ReactNode;
}) {
  return (
    <Box sx={{
      borderRadius: `${tokens.radius.sm}px`,
      border: `1px solid ${tokens.color.border.subtle}`,
      bgcolor: tokens.color.bg.surfaceRaised,
      overflow: 'hidden',
      boxShadow: tokens.shadow.sm,
    }}>
      <Stack
        direction="row" alignItems="center" spacing={1}
        onClick={onToggle}
        sx={{
          px: 1, py: 0.75, cursor: 'pointer',
          '&:hover': { bgcolor: tokens.color.bg.surfaceHover },
        }}
      >
        <IconButton size="small" sx={{ p: 0.2, color: tokens.color.text.muted }}>
          {open ? <ExpandMore fontSize="small" /> : <ChevronRight fontSize="small" />}
        </IconButton>
        <Box sx={{ color: tokens.color.accent.sky, display: 'flex' }}>{icon}</Box>
        <Typography variant="overline" sx={{ flex: 1, color: tokens.color.text.secondary, lineHeight: 1.2 }}>
          {title}
        </Typography>
        {meta && (
          <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
            {meta}
          </Typography>
        )}
      </Stack>
      <Collapse in={open}>
        <Box sx={{ px: 0.8, pb: 0.8 }}>{children}</Box>
      </Collapse>
    </Box>
  );
}

function FileTree({
  workspace, files, loading, error, selected, onSelect, onRetry,
}: {
  workspace: Workspace | null;
  files: FileEntry[];
  loading: boolean;
  error: string | null;
  selected: string | null;
  onSelect: (path: string) => void;
  onRetry: () => void;
}) {
  if (!workspace) {
    return (
      <Hint
        title="No runtime attached"
        body="Open the Editor or Terminal tab to initialize a workspace."
      />
    );
  }
  if (loading) {
    return (
      <Stack spacing={0.6} sx={{ py: 0.6 }}>
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} variant="rounded" height={22} />
        ))}
      </Stack>
    );
  }
  if (error) {
    return (
      <ErrorRow message="We could not load the file tree." onRetry={onRetry} />
    );
  }
  if (files.length === 0) {
    return (
      <Hint
        title="The tree is empty"
        body="Run the Finisher so the agent can create the codebase skeleton."
      />
    );
  }
  const sorted = files.filter((f) => !f.isDir).slice().sort((a, b) => a.path.localeCompare(b.path));
  return (
    <Box sx={{ maxHeight: 260, overflowY: 'auto', fontFamily: tokens.font.mono, fontSize: 12 }}>
      {sorted.map((f) => {
        const active = selected === f.path;
        return (
          <Stack
            key={f.path}
            direction="row"
            alignItems="center"
            spacing={0.8}
            onClick={() => onSelect(f.path)}
            sx={{
              px: 0.8, py: 0.42, borderRadius: `${tokens.radius.sm}px`, cursor: 'pointer',
              bgcolor: active ? tokens.color.accent.lime : 'transparent',
              color: active ? tokens.color.text.inverse : tokens.color.text.primary,
              '&:hover': { bgcolor: active ? '#f0ff36' : tokens.color.bg.surfaceHover },
            }}
          >
            <FileIcon sx={{ fontSize: 13, color: active ? tokens.color.text.inverse : tokens.color.text.muted }} />
            <Typography
              variant="caption"
              sx={{
                fontFamily: tokens.font.mono, flex: 1, minWidth: 0,
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }}
              title={f.path}
            >
              {f.path}
            </Typography>
          </Stack>
        );
      })}
    </Box>
  );
}

function GateRow({ state, label }: { state?: GateState; label: string }) {
  const status = state?.status ?? 'pending';
  const colour = ({
    passed: tokens.color.accent.success,
    failed: tokens.color.accent.danger,
    repaired: tokens.color.accent.warning,
    running: tokens.color.accent.lime,
    blocked: tokens.color.accent.coral,
    pending: tokens.color.text.muted,
  } as Record<string, string>)[status] ?? tokens.color.text.muted;
  return (
    <Stack
      direction="row" alignItems="center" spacing={1}
      sx={{
        px: 0.85, py: 0.55,
        borderRadius: `${tokens.radius.sm}px`,
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
      }}
    >
      <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: colour }} />
      <Typography variant="body2" sx={{ flex: 1, color: tokens.color.text.primary }}>{label}</Typography>
      <Tooltip title={state?.updatedAt ? new Date(state.updatedAt).toLocaleString() : 'Pending'}>
        <Chip
          label={status}
          size="small"
          sx={{
            height: 18, fontSize: 10, letterSpacing: '0.04em', fontWeight: 800,
            textTransform: 'uppercase',
            bgcolor: `${colour}22`, color: colour, border: `1px solid ${colour}55`,
            '& .MuiChip-label': { px: 0.9 },
          }}
        />
      </Tooltip>
    </Stack>
  );
}

function PatchList({
  patches, loading, error, onSelect, onRetry,
}: {
  patches: Patch[];
  loading: boolean;
  error: string | null;
  onSelect: (patchId: string) => void;
  onRetry: () => void;
}) {
  if (loading) {
    return (
      <Stack spacing={0.6} sx={{ py: 0.6 }}>
        {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} variant="rounded" height={42} />)}
      </Stack>
    );
  }
  if (error) {
    return <ErrorRow message="We could not load patches." onRetry={onRetry} />;
  }
  if (patches.length === 0) {
    return (
      <Hint
        title="No patches yet"
        body="Every proposed agent change will appear here for review and approval."
      />
    );
  }
  return (
    <Stack spacing={0.5}>
      {patches.slice().reverse().map((p) => (
        <Stack
          key={p.id}
          direction="row"
          spacing={0.8}
          onClick={() => onSelect(p.id)}
          sx={{
            px: 0.85, py: 0.65, borderRadius: `${tokens.radius.sm}px`, cursor: 'pointer',
            bgcolor: tokens.color.bg.inset,
            border: `1px solid ${tokens.color.border.subtle}`,
            '&:hover': { bgcolor: tokens.color.bg.surfaceHover },
          }}
        >
          <Box sx={{
            mt: 0.4, width: 8, height: 8, borderRadius: '50%',
            bgcolor: patchColour(p.status),
            flexShrink: 0,
          }} />
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography variant="body2" sx={{ fontWeight: 700 }} noWrap title={p.title}>
              {p.title || `Patch ${p.id.slice(-6)}`}
            </Typography>
            <Typography variant="caption" color="text.secondary" noWrap title={p.summary}>
              {p.summary || `${p.changes?.length ?? 0} changes`}
            </Typography>
          </Box>
          <Chip
            label={p.status}
            size="small"
            sx={{
              height: 18, fontSize: 10, fontWeight: 800, textTransform: 'uppercase',
              bgcolor: `${patchColour(p.status)}22`, color: patchColour(p.status),
              border: `1px solid ${patchColour(p.status)}55`,
              '& .MuiChip-label': { px: 0.9 },
            }}
          />
        </Stack>
      ))}
    </Stack>
  );
}

function patchColour(status: string): string {
  switch (status) {
    case 'applied':    return tokens.color.accent.success;
    case 'validated':  return tokens.color.accent.lime;
    case 'proposed':   return tokens.color.accent.sky;
    case 'rejected':   return tokens.color.accent.danger;
    case 'rolled-back': return tokens.color.accent.warning;
    default:           return tokens.color.text.muted;
  }
}

function gateMetaLabel(project: Project, order: { key: string }[]): string {
  const passed = order.filter(
    (g) => (project.gates[g.key as keyof typeof project.gates]?.status === 'passed'),
  ).length;
  return `${passed}/${order.length}`;
}

function Hint({ title, body }: { title: string; body: string }) {
  return (
    <Box sx={{
      px: 1, py: 1.1,
      borderRadius: `${tokens.radius.sm}px`,
      bgcolor: tokens.color.bg.inset,
      border: `1px solid ${tokens.color.border.subtle}`,
      textAlign: 'center',
    }}>
      <Typography variant="body2" sx={{ fontWeight: 700 }}>{title}</Typography>
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.3 }}>
        {body}
      </Typography>
    </Box>
  );
}

function ErrorRow({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <Stack
      spacing={0.8}
      sx={{
        px: 1, py: 1,
        borderRadius: `${tokens.radius.sm}px`,
        bgcolor: 'rgba(255,24,24,0.08)',
        border: '1px solid rgba(255,24,24,0.28)',
      }}
    >
      <Typography variant="body2" sx={{ color: tokens.color.accent.danger, fontWeight: 700 }}>
        {message}
      </Typography>
      <Typography
        variant="caption"
        onClick={onRetry}
        sx={{ color: tokens.color.accent.lime, cursor: 'pointer', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em' }}
      >
        Try again
      </Typography>
    </Stack>
  );
}
