'use client';

import { useEffect, useState } from 'react';
import {
  Box, Button, Chip, IconButton, Stack, TextField, Typography, Tooltip,
} from '@mui/material';
import { runtime, Workspace, FileEntry } from '../../../lib/runtime';
import { tokens } from '../../../lib/theme';

interface Props {
  workspace: Workspace | null;
  onWorkspaceChange: (w: Workspace | null) => void;
  projectId: string;
}

// Files pane: live file tree from the workspace runtime + simple editor.
export function WorkspaceFiles({ workspace, onWorkspaceChange, projectId }: Props) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [content, setContent] = useState('');
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!workspace) return;
    void refresh();
  }, [workspace?.id]);

  async function refresh() {
    if (!workspace) return;
    try {
      const list = await runtime.listFiles(workspace.id);
      setFiles(list.filter((f) => !f.isDir));
    } catch (e) {
      setError(String(e));
    }
  }

  async function createWorkspace() {
    setBusy(true); setError(null);
    try {
      const w = await runtime.create({ userId: 'demo', projectId });
      onWorkspaceChange(w);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function destroyWorkspace() {
    if (!workspace) return;
    setBusy(true); setError(null);
    try {
      await runtime.destroy(workspace.id);
      onWorkspaceChange(null);
      setFiles([]); setSelected(null); setContent(''); setDirty(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function openFile(path: string) {
    if (!workspace) return;
    setSelected(path);
    setDirty(false);
    try {
      const t = await runtime.readFile(workspace.id, path);
      setContent(t);
    } catch (e) {
      setContent(`error: ${e}`);
    }
  }

  async function saveFile() {
    if (!workspace || !selected) return;
    setBusy(true);
    try {
      await runtime.writeFile(workspace.id, selected, content);
      setDirty(false);
      void refresh();
    } finally {
      setBusy(false);
    }
  }

  if (!workspace) {
    return (
      <Stack spacing={2} sx={{ height: '100%', alignItems: 'flex-start' }}>
        <Typography variant="body2" color="text.secondary">
          No cloud workspace attached to this project yet. Spin one up — Ironflyer will
          provision a sandbox you can also resume on mobile.
        </Typography>
        <Button variant="contained" onClick={createWorkspace} disabled={busy}>
          {busy ? 'Provisioning…' : 'Create workspace'}
        </Button>
        {error && <Typography variant="caption" color="error">{error}</Typography>}
      </Stack>
    );
  }

  return (
    <Box sx={{ height: '100%', display: 'grid', gridTemplateColumns: '220px 1fr', gap: 1.5 }}>
      <Box sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
          <Chip label={workspace.driver} size="small" />
          <Tooltip title="Destroy workspace">
            <IconButton size="small" onClick={destroyWorkspace} sx={{ color: 'text.secondary' }}>
              ✕
            </IconButton>
          </Tooltip>
        </Stack>
        <Box sx={{ flex: 1, overflowY: 'auto',
          bgcolor: tokens.color.bg.inset, borderRadius: 2, p: 1,
          fontFamily: tokens.font.mono, fontSize: 12,
        }}>
          {files.length === 0 && <Box sx={{ color: tokens.color.text.muted }}>(empty)</Box>}
          {files.map((f) => (
            <Box
              key={f.path}
              onClick={() => openFile(f.path)}
              sx={{
                cursor: 'pointer', py: 0.4, px: 0.5, borderRadius: 1,
                bgcolor: selected === f.path ? tokens.color.bg.surfaceHover : 'transparent',
                color: selected === f.path ? tokens.color.accent.lime : tokens.color.text.primary,
                '&:hover': { bgcolor: tokens.color.bg.surfaceHover },
              }}>
              {f.path}
            </Box>
          ))}
        </Box>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
          <Typography variant="caption" color="text.secondary" sx={{ fontFamily: tokens.font.mono }}>
            {selected ?? '(no file selected)'}
            {dirty && <Box component="span" sx={{ color: tokens.color.accent.warning, ml: 1 }}>●</Box>}
          </Typography>
          <Button size="small" variant="outlined" onClick={saveFile} disabled={!selected || !dirty || busy}>
            Save
          </Button>
        </Stack>
        <TextField
          multiline minRows={14} value={content}
          onChange={(e) => { setContent(e.target.value); setDirty(true); }}
          disabled={!selected}
          sx={{
            flex: 1,
            '& .MuiInputBase-root': {
              fontFamily: tokens.font.mono, fontSize: 13,
              alignItems: 'flex-start',
              height: '100%',
            },
            '& .MuiInputBase-input': { height: '100% !important' },
          }}
        />
      </Box>
    </Box>
  );
}
