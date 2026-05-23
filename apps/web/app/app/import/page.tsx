'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  CloudDownload, GitHub, OpenInNew, PlayArrow, Tune,
} from '@mui/icons-material';
import {
  Box, Button, Chip, CircularProgress, FormControlLabel, LinearProgress, Stack,
  Switch, TextField, Typography,
} from '@mui/material';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';
import { githubApi, GitHubStatus } from '../../../lib/github';
import {
  ImportEvent, ImportResult, startImport,
} from '../../../lib/api/import';
import { VirtualList } from '../../../components/performance/VirtualList';

interface LogLine {
  id: number;
  level: 'info' | 'warn' | 'error' | 'success';
  text: string;
  ts: string;
}

const LOG_LIMIT = 500;

export default function ImportPage() {
  return (
    <RequireAuth>
      <ImportInner />
    </RequireAuth>
  );
}

// Validates a GitHub repo URL or `owner/repo` shorthand. Returns a
// human-readable English error when invalid.
function validateRepoURL(input: string): string | null {
  const s = input.trim();
  if (!s) return 'Enter a valid GitHub repository URL';
  // owner/repo shorthand.
  if (/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(s)) return null;
  try {
    const u = new URL(s);
    if (u.protocol !== 'https:') return 'The URL must use https://';
    if (u.host.toLowerCase() !== 'github.com') return 'Only github.com repositories are supported';
    const path = u.pathname.replace(/^\/+|\/+$/g, '').replace(/\.git$/, '');
    const parts = path.split('/');
    if (parts.length < 2 || !parts[0] || !parts[1]) return 'The URL must include owner/repo';
    return null;
  } catch {
    return 'Enter a valid GitHub repository URL';
  }
}

function ImportInner() {
  const { user, logout } = useAuth();
  const router = useRouter();

  // Form state.
  const [repoUrl, setRepoUrl] = useState('');
  const [repoError, setRepoError] = useState<string | null>(null);
  const [branch, setBranch] = useState('');
  const [subdir, setSubdir] = useState('');
  const [makePublic, setMakePublic] = useState(false);
  const [advanced, setAdvanced] = useState(false);

  // GitHub connection status — used to hint about private repos.
  const [github, setGithub] = useState<GitHubStatus | null>(null);
  const [githubLoaded, setGithubLoaded] = useState(false);

  // Pipeline state.
  const [running, setRunning] = useState(false);
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [currentStage, setCurrentStage] = useState<string>('');
  const [result, setResult] = useState<ImportResult | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const ctrlRef = useRef<AbortController | null>(null);
  const logIdRef = useRef(0);

  useEffect(() => {
    void githubApi.me().then((s) => setGithub(s)).catch(() => setGithub({ connected: false })).finally(() => setGithubLoaded(true));
  }, []);

  const pushLog = useCallback((level: LogLine['level'], text: string) => {
    logIdRef.current += 1;
    setLogs((prev) => {
      const next = [
        ...prev,
        {
        id: logIdRef.current,
        level,
        text,
        ts: new Date().toLocaleTimeString('en-US'),
        },
      ];
      return next.length > LOG_LIMIT ? next.slice(-LOG_LIMIT) : next;
    });
  }, []);

  const stageLabel = useMemo<Record<string, string>>(() => ({
    import_started: 'Starting import...',
    project_created: 'Project record created',
    cloning: 'Cloning repository into the runtime...',
    cloned: 'Repository cloned successfully',
    detecting_stack: 'Detecting stack...',
    stack_detected: 'Stack detected',
    warning: 'Warning',
    ready: 'Import complete',
    failed: 'Import failed',
  }), []);

  function startPipeline() {
    const err = validateRepoURL(repoUrl);
    setRepoError(err);
    if (err) return;
    setLogs([]);
    setResult(null);
    setFailure(null);
    setCurrentStage('import_started');
    setRunning(true);
    pushLog('info', `Sending import request for ${repoUrl.trim()}`);

    const ctrl = startImport(
      {
        repoUrl: repoUrl.trim(),
        branch: branch.trim() || undefined,
        subdir: subdir.trim() || undefined,
        makePublic,
      },
      {
        onEvent: (evt: ImportEvent) => {
          setCurrentStage(evt.type);
          const label = stageLabel[evt.type] ?? evt.type;
          if (evt.type === 'warning' && evt.warning) {
            pushLog('warn', `${label}: ${evt.warning}`);
          } else if (evt.type === 'stack_detected' && evt.stack) {
            pushLog(
              'success',
              `${label}: ${evt.stack.frontend} · ${evt.stack.backend} · ${evt.stack.storage} · ${evt.stack.auth}`,
            );
          } else if (evt.message) {
            pushLog('info', `${label}: ${evt.message}`);
          } else {
            pushLog('info', label);
          }
        },
        onResult: (res: ImportResult) => {
          setResult(res);
          pushLog('success', `Project ready: ${res.projectId}`);
        },
        onError: (msg: string) => {
          setFailure(msg);
          pushLog('error', msg);
        },
        onClose: () => {
          setRunning(false);
          ctrlRef.current = null;
        },
      },
    );
    ctrlRef.current = ctrl;
  }

  function cancelPipeline() {
    ctrlRef.current?.abort();
    ctrlRef.current = null;
    setRunning(false);
    pushLog('warn', 'Import canceled by user');
  }

  function openProject() {
    if (!result) return;
    const initialPrompt = encodeURIComponent(
      `Imported repository ${repoUrl.trim()}. Continue the work and propose the next steps.`,
    );
    router.push(`/app/projects/${result.projectId}?initialPrompt=${initialPrompt}`);
  }

  const progressPct = useMemo(() => {
    const order = [
      '', 'import_started', 'project_created', 'cloning', 'cloned',
      'detecting_stack', 'stack_detected', 'ready',
    ];
    const idx = order.indexOf(currentStage);
    if (idx <= 0) return running ? 6 : 0;
    return Math.min(100, Math.round((idx / (order.length - 1)) * 100));
  }, [currentStage, running]);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} onLogout={logout}>
      <PageTitle
        eyebrow="Import"
        title="Import a GitHub repository"
        subtitle="Bring in your code as-is. Ironflyer will continue finishing it through design, architecture, quality, and deploy gates."
      />

      {githubLoaded && !github?.connected && (
        <Surface sx={{ p: 1.6, mb: 1.6, borderRight: `3px solid ${tokens.color.accent.lime}` }}>
          <Stack direction="row" spacing={1.2} alignItems="center">
            <GitHub />
            <Box sx={{ flex: 1 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 800 }}>
                Connect GitHub to import private repositories
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Public repositories work without a connection, but private repositories require OAuth consent.
              </Typography>
            </Box>
            <Button
              variant="outlined"
              size="small"
              component={Link}
              href="/app/connectors"
              endIcon={<OpenInNew fontSize="small" />}
            >
              Go to connectors
            </Button>
          </Stack>
        </Surface>
      )}

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.05fr 0.95fr' }, gap: 1.6 }}>
        <Surface sx={{ p: 2.2 }}>
          <Stack direction="row" spacing={1.2} alignItems="center" sx={{ mb: 1.4 }}>
            <Box sx={{
              width: 42, height: 42, borderRadius: '8px', display: 'grid', placeItems: 'center',
              bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse,
            }}>
              <CloudDownload />
            </Box>
            <Box>
              <Typography variant="h6" sx={{ fontWeight: 900 }}>Repository details</Typography>
              <Typography variant="body2" color="text.secondary">
                Paste a full URL or use the <code>owner/repo</code> shorthand.
              </Typography>
            </Box>
          </Stack>

          <TextField
            label="GitHub URL"
            placeholder="https://github.com/owner/repo or owner/repo"
            fullWidth
            value={repoUrl}
            onChange={(e) => {
              setRepoUrl(e.target.value);
              if (repoError) setRepoError(null);
            }}
            onBlur={() => setRepoError(validateRepoURL(repoUrl))}
            error={!!repoError}
            helperText={repoError ?? ' '}
            disabled={running}
            sx={{ mb: 1.2 }}
          />

          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: advanced ? 1.2 : 0.4 }}>
            <Button
              size="small"
              variant="text"
              startIcon={<Tune fontSize="small" />}
              onClick={() => setAdvanced((v) => !v)}
            >
              {advanced ? 'Hide advanced options' : 'Advanced options'}
            </Button>
          </Stack>

          {advanced && (
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.2} sx={{ mb: 1.2 }}>
              <TextField
                label="Branch (default: main)"
                placeholder="main"
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                disabled={running}
                fullWidth
              />
              <TextField
                label="Subdirectory (optional)"
                placeholder="apps/web"
                value={subdir}
                onChange={(e) => setSubdir(e.target.value)}
                disabled={running}
                fullWidth
              />
            </Stack>
          )}

          <FormControlLabel
            sx={{ mb: 1.6 }}
            control={
              <Switch
                checked={makePublic}
                onChange={(_, v) => setMakePublic(v)}
                disabled={running}
              />
            }
            label="Make public"
          />

          <Stack direction="row" spacing={1}>
            <Button
              variant="contained"
              startIcon={running ? <CircularProgress size={16} color="inherit" /> : <PlayArrow />}
              onClick={startPipeline}
              disabled={running || !repoUrl.trim()}
            >
              {running ? 'Importing...' : 'Start import'}
            </Button>
            {running && (
              <Button variant="outlined" onClick={cancelPipeline}>
                Cancel
              </Button>
            )}
          </Stack>
        </Surface>

        <Surface sx={{ p: 2.2, display: 'flex', flexDirection: 'column', minHeight: 320 }}>
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
            <Typography variant="h6" sx={{ fontWeight: 900 }}>Import log</Typography>
            <Chip
              size="small"
              label={running ? 'Running' : result ? 'Ready' : failure ? 'Failed' : 'Ready to start'}
              sx={{
                bgcolor: running
                  ? tokens.color.accent.lime
                  : failure
                  ? '#ffd5d5'
                  : result
                  ? tokens.color.accent.lime
                  : '#fffaf1',
                fontWeight: 800,
              }}
            />
          </Stack>
          <LinearProgress
            variant="determinate"
            value={progressPct}
            sx={{
              height: 6, borderRadius: '999px', mb: 1.4,
              bgcolor: 'rgba(17,17,17,0.08)',
              '& .MuiLinearProgress-bar': { bgcolor: tokens.color.accent.lime },
            }}
          />
          <Box
            sx={{
              flex: 1, minHeight: 200,
              bgcolor: '#0d0d0d', color: '#e7e2d4', borderRadius: '8px',
              fontFamily: tokens.font.mono, fontSize: 12.5, lineHeight: 1.6,
              p: 1.4,
            }}
          >
            {logs.length === 0 ? (
              <Typography variant="body2" sx={{ color: '#86807a', fontFamily: tokens.font.mono }}>
                The log will appear here once the import starts.
              </Typography>
            ) : (
              <VirtualList
                items={logs}
                itemHeight={22}
                getItemHeight={(line) => (line.text.length > 120 ? 44 : 22)}
                height={Math.min(360, Math.max(200, logs.length * 22))}
                keyExtractor={(line) => line.id}
                ariaLabel="Import log"
                renderItem={(line) => <LogLineRow line={line} />}
              />
            )}
          </Box>

          {result && (
            <Stack direction="row" spacing={1} sx={{ mt: 1.6 }}>
              <Button variant="contained" onClick={openProject} endIcon={<OpenInNew fontSize="small" />}>
                Open project
              </Button>
              <Chip
                label={`stack: ${result.stack.frontend} · ${result.stack.backend}`}
                sx={{ bgcolor: '#fffaf1', fontWeight: 700 }}
              />
            </Stack>
          )}
          {failure && !result && (
            <Typography variant="body2" sx={{ color: '#9b1010', mt: 1.4 }}>
              {failure}
            </Typography>
          )}
        </Surface>
      </Box>
    </AppShell>
  );
}

function LogLineRow({ line }: { line: LogLine }) {
  return (
    <Box sx={{
      color: line.level === 'error' ? '#ff8a8a'
        : line.level === 'warn' ? '#ffd166'
        : line.level === 'success' ? tokens.color.accent.lime
        : '#e7e2d4',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'pre-wrap',
    }}>
      <span style={{ opacity: 0.55 }}>[{line.ts}]</span> {line.text}
    </Box>
  );
}
