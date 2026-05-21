'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  Archive, Code as CodeIcon, Description, Folder, Image as ImageIcon,
  InsertDriveFile,
} from '@mui/icons-material';
import {
  Box, Button, Chip, IconButton, InputAdornment, Stack, TextField,
  Tooltip, Typography,
} from '@mui/material';
import { runtime, Workspace, FileEntry } from '../../../lib/runtime';
import { tokens } from '../../../lib/theme';

interface Props {
  workspace: Workspace | null;
  onWorkspaceChange: (w: Workspace | null) => void;
  projectId: string;
}

type Category = 'images' | 'documents' | 'code' | 'archives' | 'other';

interface CategoryDef {
  key: Category;
  label: string;
  icon: React.ReactNode;
  exts: string[];
}

const CATEGORIES: CategoryDef[] = [
  { key: 'images', label: 'Images', icon: <ImageIcon fontSize="small" />,
    exts: ['.png', '.jpg', '.jpeg', '.gif', '.svg', '.webp', '.bmp', '.ico'] },
  { key: 'documents', label: 'Documents', icon: <Description fontSize="small" />,
    exts: ['.md', '.txt', '.pdf', '.rtf', '.csv', '.tsv', '.doc', '.docx', '.xls', '.xlsx'] },
  { key: 'code', label: 'Code', icon: <CodeIcon fontSize="small" />,
    exts: ['.ts', '.tsx', '.js', '.jsx', '.go', '.py', '.rb', '.java', '.kt', '.swift',
           '.rs', '.c', '.cpp', '.h', '.cs', '.php', '.html', '.css', '.scss', '.sass',
           '.json', '.yaml', '.yml', '.toml', '.xml', '.sh', '.bash', '.zsh',
           '.vue', '.svelte', '.sql', '.graphql', '.dockerfile', '.lua'] },
  { key: 'archives', label: 'Archives', icon: <Archive fontSize="small" />,
    exts: ['.zip', '.tar', '.gz', '.bz2', '.xz', '.7z', '.rar', '.tgz'] },
  { key: 'other', label: 'Other', icon: <InsertDriveFile fontSize="small" />, exts: [] },
];

function categoryOf(path: string): Category {
  const lower = path.toLowerCase();
  const dot = lower.lastIndexOf('.');
  if (dot < 0) {
    // Files like Dockerfile, Makefile, README — treat as code/docs by name.
    const base = lower.slice(lower.lastIndexOf('/') + 1);
    if (base === 'dockerfile' || base === 'makefile') return 'code';
    if (base === 'readme' || base === 'license' || base === 'changelog') return 'documents';
    return 'other';
  }
  const ext = lower.slice(dot);
  for (const cat of CATEGORIES) {
    if (cat.exts.includes(ext)) return cat.key;
  }
  return 'other';
}

// WorkspaceFiles is the Lovable-style file browser: workspace controls on the
// left, a searchable category-grouped file tree in the middle, and a preview
// pane on the right. Quick text edits still work in-place; for full IDE
// power the user switches to the IDE tab next door.
export function WorkspaceFiles({ workspace, onWorkspaceChange, projectId }: Props) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [content, setContent] = useState<string>('');
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [openCategories, setOpenCategories] = useState<Record<Category, boolean>>({
    code: true, documents: true, images: true, archives: false, other: false,
  });

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
    const cat = categoryOf(path);
    if (cat === 'images' || cat === 'archives') {
      // Skip the text read for binary categories — preview pane handles
      // image rendering directly via the file URL.
      setContent('');
      return;
    }
    try {
      const t = await runtime.readFile(workspace.id, path);
      // Skip enormous binary blobs that snuck into the category 'other'.
      setContent(t.length > 200_000 ? t.slice(0, 200_000) + '\n…truncated' : t);
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

  // Group + filter files for the tree.
  const grouped = useMemo(() => {
    const q = query.trim().toLowerCase();
    const buckets: Record<Category, FileEntry[]> = {
      code: [], documents: [], images: [], archives: [], other: [],
    };
    for (const f of files) {
      if (q && !f.path.toLowerCase().includes(q)) continue;
      buckets[categoryOf(f.path)].push(f);
    }
    for (const k of Object.keys(buckets) as Category[]) {
      buckets[k].sort((a, b) => a.path.localeCompare(b.path));
    }
    return buckets;
  }, [files, query]);

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

  const selectedCategory = selected ? categoryOf(selected) : null;
  const fileURL = (path: string) =>
    `/api/runtime/workspaces/${workspace.id}/files/${encodeURI(path)}`;

  return (
    <Box sx={{
      height: '100%',
      display: 'grid',
      gridTemplateColumns: { xs: '1fr', md: '300px minmax(0, 1fr)' },
      gap: 1.5,
    }}>
      {/* LEFT: search + categories */}
      <Box sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
          <Chip label={workspace.driver} size="small" />
          <Tooltip title="Destroy workspace">
            <IconButton size="small" onClick={destroyWorkspace} sx={{ color: 'text.secondary' }}>
              ✕
            </IconButton>
          </Tooltip>
        </Stack>
        <TextField
          size="small" value={query} onChange={(e) => setQuery(e.target.value)}
          placeholder="Search files"
          InputProps={{
            startAdornment: <InputAdornment position="start"><Folder fontSize="small" /></InputAdornment>,
          }}
          sx={{ mb: 1 }}
        />
        <Box sx={{
          flex: 1, overflowY: 'auto', bgcolor: tokens.color.bg.inset, borderRadius: 2, p: 0.8,
          fontFamily: tokens.font.mono, fontSize: 12,
        }}>
          {files.length === 0 && (
            <Box sx={{ color: tokens.color.text.muted, p: 1 }}>(empty)</Box>
          )}
          {CATEGORIES.map((cat) => {
            const items = grouped[cat.key];
            if (items.length === 0) return null;
            const open = openCategories[cat.key];
            return (
              <Box key={cat.key} sx={{ mb: 0.5 }}>
                <Stack
                  direction="row" alignItems="center" spacing={1}
                  onClick={() => setOpenCategories((s) => ({ ...s, [cat.key]: !s[cat.key] }))}
                  sx={{
                    cursor: 'pointer', px: 0.6, py: 0.5, borderRadius: 1,
                    color: tokens.color.text.muted,
                    '&:hover': { color: tokens.color.text.primary },
                  }}>
                  <Box sx={{ width: 14 }}>{open ? '▾' : '▸'}</Box>
                  {cat.icon}
                  <Typography variant="caption" sx={{ flex: 1, fontWeight: 700, fontFamily: tokens.font.mono }}>
                    {cat.label}
                  </Typography>
                  <Typography variant="caption" sx={{ color: tokens.color.text.muted }}>
                    {items.length}
                  </Typography>
                </Stack>
                {open && items.map((f) => (
                  <Box
                    key={f.path}
                    onClick={() => openFile(f.path)}
                    sx={{
                      cursor: 'pointer', py: 0.35, pl: 3.5, pr: 0.6, borderRadius: 1,
                      bgcolor: selected === f.path ? tokens.color.bg.surfaceHover : 'transparent',
                      color: selected === f.path ? tokens.color.accent.lime : tokens.color.text.primary,
                      '&:hover': { bgcolor: tokens.color.bg.surfaceHover },
                      overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    }}
                    title={f.path}>
                    {f.path}
                  </Box>
                ))}
              </Box>
            );
          })}
        </Box>
      </Box>

      {/* RIGHT: preview / editor */}
      <Box sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
          <Typography variant="caption" color="text.secondary" sx={{ fontFamily: tokens.font.mono }}>
            {selected ?? '(no file selected)'}
            {dirty && <Box component="span" sx={{ color: tokens.color.accent.warning, ml: 1 }}>●</Box>}
          </Typography>
          {selectedCategory && (selectedCategory === 'code' || selectedCategory === 'documents') && (
            <Button size="small" variant="outlined" onClick={saveFile} disabled={!selected || !dirty || busy}>
              Save
            </Button>
          )}
        </Stack>

        {selected && selectedCategory === 'images' && (
          <Box sx={{
            flex: 1, display: 'grid', placeItems: 'center', bgcolor: '#0d0e0f',
            borderRadius: 2, overflow: 'hidden', p: 2,
          }}>
            <Box component="img" src={fileURL(selected)} alt={selected}
                 sx={{ maxWidth: '100%', maxHeight: '100%', display: 'block' }} />
          </Box>
        )}

        {selected && (selectedCategory === 'archives' || selectedCategory === 'other') && (
          <Box sx={{
            flex: 1, display: 'grid', placeItems: 'center', bgcolor: tokens.color.bg.inset,
            borderRadius: 2, color: tokens.color.text.muted, textAlign: 'center', p: 3,
          }}>
            <Stack spacing={1} alignItems="center">
              <InsertDriveFile fontSize="large" />
              <Typography variant="body2">Preview is not available for this file type.</Typography>
              <Button size="small" variant="outlined" component="a"
                      href={fileURL(selected)} target="_blank" rel="noopener noreferrer">
                Download
              </Button>
            </Stack>
          </Box>
        )}

        {selected && (selectedCategory === 'code' || selectedCategory === 'documents') && (
          <TextField
            multiline minRows={14} value={content}
            onChange={(e) => { setContent(e.target.value); setDirty(true); }}
            sx={{
              flex: 1,
              '& .MuiInputBase-root': {
                fontFamily: tokens.font.mono, fontSize: 13,
                alignItems: 'flex-start', height: '100%',
              },
              '& .MuiInputBase-input': { height: '100% !important' },
            }}
          />
        )}

        {!selected && (
          <Box sx={{
            flex: 1, display: 'grid', placeItems: 'center', bgcolor: tokens.color.bg.inset,
            borderRadius: 2, color: tokens.color.text.muted,
          }}>
            <Typography variant="body2">Select a file to preview.</Typography>
          </Box>
        )}
      </Box>
    </Box>
  );
}
