'use client';

import { useDeferredValue, useEffect, useMemo, useState } from 'react';
import {
  Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  IconButton, ListItemButton, ListItemText, Stack, TextField, Tooltip,
  Typography,
} from '@mui/material';
import { githubApi, GitHubRepo, GitHubStatus } from '../../../lib/github';
import { tokens } from '../../../lib/theme';
import { VirtualList } from '../../../components/performance/VirtualList';

interface Props {
  projectId: string;
  github: { fullName: string; defaultBranch: string; htmlUrl: string } | null;
  workspaceId?: string | null;
  onLinked: () => void;
}

export function GitHubPanel({ projectId, github, workspaceId, onLinked }: Props) {
  const [cloneState, setCloneState] = useState<'idle' | 'cloning' | 'done'>('idle');
  const [status, setStatus] = useState<GitHubStatus | null>(null);
  const [disabled, setDisabled] = useState(false);
  const [picking, setPicking] = useState(false);
  const [repos, setRepos] = useState<GitHubRepo[]>([]);
  const [filter, setFilter] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const s = await githubApi.me();
        if (!cancelled) setStatus(s);
      } catch (e) {
        if ((e as Error).message === 'github-disabled') setDisabled(true);
        else if (!cancelled) setError((e as Error).message);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  async function connectGitHub() {
    setBusy(true); setError(null);
    try {
      await githubApi.startConnect();
    } catch (e) {
      if ((e as Error).message === 'github-disabled') setDisabled(true);
      else setError((e as Error).message);
      setBusy(false);
    }
  }

  async function disconnectGitHub() {
    setBusy(true); setError(null);
    try {
      await githubApi.disconnect();
      const s = await githubApi.me();
      setStatus(s);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function openPicker() {
    setPicking(true);
    setBusy(true); setError(null);
    try {
      const list = await githubApi.repos();
      setRepos(list);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function pickRepo(repo: GitHubRepo) {
    setBusy(true); setError(null);
    try {
      await githubApi.connectRepo(projectId, repo);
      setPicking(false);
      onLinked();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function cloneIntoWorkspace() {
    if (!workspaceId || !github) return;
    setCloneState('cloning'); setError(null);
    try {
      await githubApi.cloneIntoWorkspace(projectId, workspaceId, github.defaultBranch);
      setCloneState('done');
    } catch (e) {
      setError((e as Error).message);
      setCloneState('idle');
    }
  }

  async function unlinkRepo() {
    setBusy(true); setError(null);
    try {
      await githubApi.disconnectRepo(projectId);
      onLinked();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (disabled) {
    return (
      <Box sx={{ p: 2, bgcolor: tokens.color.bg.inset, borderRadius: 2 }}>
        <Typography variant="body2" color="text.secondary">
          GitHub integration not configured. Ask the operator to set{' '}
          <Box component="code" sx={{ fontFamily: tokens.font.mono }}>GITHUB_CLIENT_ID</Box>{' '}
          and{' '}
          <Box component="code" sx={{ fontFamily: tokens.font.mono }}>GITHUB_CLIENT_SECRET</Box>.
        </Typography>
      </Box>
    );
  }

  const deferredFilter = useDeferredValue(filter);
  const visible = useMemo(() => {
    const q = deferredFilter.trim().toLowerCase();
    if (!q) return repos;
    return repos.filter((r) => r.fullName.toLowerCase().includes(q));
  }, [deferredFilter, repos]);

  return (
    <Stack spacing={1.5}>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        <Typography variant="overline" color="text.secondary">GitHub</Typography>
        {status?.connected ? (
          <Chip label={`Connected · ${status.login}`} size="small"
                sx={{ bgcolor: tokens.color.accent.success, color: tokens.color.text.inverse, fontWeight: 700 }} />
        ) : (
          <Chip label="Not connected" size="small"
                sx={{ bgcolor: tokens.color.bg.surfaceHover, color: tokens.color.text.secondary }} />
        )}
      </Stack>

      {!status?.connected && (
        <Button variant="contained" size="small" onClick={connectGitHub} disabled={busy}
                sx={{ alignSelf: 'flex-start' }}>
          {busy ? '…' : 'Connect GitHub'}
        </Button>
      )}

      {status?.connected && !github && (
        <Stack direction="row" spacing={1}>
          <Button variant="contained" size="small" onClick={openPicker} disabled={busy}>
            Bind a repo
          </Button>
          <Tooltip title="Disconnect GitHub account">
            <IconButton size="small" onClick={disconnectGitHub} disabled={busy}
                        sx={{ color: 'text.secondary' }}>✕</IconButton>
          </Tooltip>
        </Stack>
      )}

      {github && (
        <Box sx={{ p: 1.5, bgcolor: tokens.color.bg.inset, borderRadius: 2 }}>
          <Stack direction="row" justifyContent="space-between" alignItems="center">
            <Box>
              <Typography variant="body2" sx={{ fontFamily: tokens.font.mono }}>
                {github.fullName}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                default: <code>{github.defaultBranch || 'main'}</code>
              </Typography>
            </Box>
            <Stack direction="row" spacing={0.5}>
              <Button size="small" variant="outlined" href={github.htmlUrl} target="_blank" rel="noreferrer">
                Open
              </Button>
              {workspaceId && (
                <Button
                  size="small" variant="contained"
                  onClick={cloneIntoWorkspace}
                  disabled={busy || cloneState === 'cloning'}
                >
                  {cloneState === 'cloning' ? 'Cloning…' :
                   cloneState === 'done' ? 'Cloned ✓' : 'Clone to workspace'}
                </Button>
              )}
              <Button size="small" variant="outlined" onClick={unlinkRepo} disabled={busy}>
                Unlink
              </Button>
            </Stack>
          </Stack>
        </Box>
      )}

      {error && <Typography variant="caption" color="error">{error}</Typography>}

      <Dialog open={picking} onClose={() => setPicking(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Pick a repo</DialogTitle>
        <DialogContent>
          <TextField fullWidth size="small" placeholder="filter…" value={filter}
                     onChange={(e) => setFilter(e.target.value)} sx={{ mb: 1 }} />
          <Box sx={{ height: visible.length === 0 ? 'auto' : 360 }}>
            {visible.length === 0 && (
              <Typography variant="body2" color="text.secondary" sx={{ p: 2 }}>
                {busy ? 'Loading…' : 'No repos match.'}
              </Typography>
            )}
            {visible.length > 0 && (
              <VirtualList
                items={visible}
                itemHeight={66}
                height={360}
                keyExtractor={(repo) => repo.id}
                ariaLabel="GitHub repositories"
                renderItem={(repo) => (
                  <RepoRow repo={repo} busy={busy} onPick={() => pickRepo(repo)} />
                )}
              />
            )}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPicking(false)} disabled={busy}>Cancel</Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}

function RepoRow({ repo, busy, onPick }: { repo: GitHubRepo; busy: boolean; onPick: () => void }) {
  return (
    <ListItemButton onClick={onPick} disabled={busy} sx={{ minHeight: 66 }}>
      <ListItemText
        primary={<span style={{ fontFamily: tokens.font.mono }}>{repo.fullName}</span>}
        secondary={
          <>
            {repo.private && <Chip label="private" size="small" sx={{ mr: 1 }} />}
            {repo.description}
          </>
        }
      />
    </ListItemButton>
  );
}
