'use client';

// Deploy page — MUI 6, alabaster + lime palette.
//
// One-click pipeline: pick a provider, optionally set env vars, hit Deploy.
// Live logs stream over SSE. Past deployments live at the bottom with
// status pills and the resulting public URL.
//
// The page is intentionally a single file for now; the deploy surface is
// small enough that splitting into shells/cards/log-viewers obscures more
// than it clarifies.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useParams } from 'next/navigation';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import {
  Add,
  CloudDownload,
  CloudUpload,
  GitHub,
  OpenInNew,
  Refresh,
  RocketLaunch,
  Train,
} from '@mui/icons-material';

import { api, Project } from '../../../../../lib/api';
import { auth } from '../../../../../lib/auth';
import { tokens } from '../../../../../lib/theme';
import { RequireAuth, useAuth } from '../../../../auth-context';
import { AppShell, PageTitle, Surface } from '../../../workspace-shell';

type Provider = 'fly' | 'railway' | 'github' | 'zip';

type DeployArtifact = {
  path: string;
  source: 'existing' | 'generated';
  stack: string;
  purpose: string;
};

type DeployPlan = {
  stack: string;
  artifacts: DeployArtifact[];
  providers: { fly: boolean; railway: boolean };
};

type DeploymentRecord = {
  id: string;
  projectId: string;
  provider: string;
  region?: string;
  status: 'running' | 'deployed' | 'failed';
  url?: string;
  error?: string;
  createdAt: string;
  finishedAt?: string;
};

type DeployEvent = {
  kind: 'deploy_started' | 'build_started' | 'build_done' | 'push_started' | 'push_done' | 'log' | 'deployed' | 'failed';
  line?: string;
  url?: string;
  error?: string;
  at: string;
};

const FLY_REGIONS = [
  { id: 'iad', label: 'Ashburn (US East)' },
  { id: 'lhr', label: 'London' },
  { id: 'fra', label: 'Frankfurt' },
  { id: 'sin', label: 'Singapore' },
  { id: 'syd', label: 'Sydney' },
  { id: 'gru', label: 'Sao Paulo' },
];

export default function ProjectDeployPage() {
  return (
    <RequireAuth>
      <DeployInner />
    </RequireAuth>
  );
}

function DeployInner() {
  const { user, logout } = useAuth();
  const params = useParams<{ id: string }>();
  const projectId = params?.id ?? '';
  const [project, setProject] = useState<Project | null>(null);
  const [plan, setPlan] = useState<DeployPlan | null>(null);
  const [deployments, setDeployments] = useState<DeploymentRecord[]>([]);
  const [provider, setProvider] = useState<Provider>('fly');
  const [region, setRegion] = useState('iad');
  const [envRows, setEnvRows] = useState<Array<{ key: string; value: string }>>([{ key: '', value: '' }]);
  const [repoName, setRepoName] = useState('');
  const [repoPrivate, setRepoPrivate] = useState(true);
  const [logs, setLogs] = useState<DeployEvent[]>([]);
  const [activeDeploymentId, setActiveDeploymentId] = useState<string | null>(null);
  const [running, setRunning] = useState(false);
  const [banner, setBanner] = useState<{ kind: 'success' | 'error'; text: string } | null>(null);
  const logEndRef = useRef<HTMLDivElement | null>(null);

  const base = '/api/orchestrator';

  // Initial project + plan + history fetch.
  useEffect(() => {
    if (!projectId) return;
    void api.getProject(projectId).then(setProject).catch(() => setProject(null));
    void fetch(`${base}/projects/${projectId}/deploy/plan`, {
      headers: { ...auth.authHeader() },
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((p) => setPlan(p));
    void refreshHistory();
  }, [projectId]);

  // Auto-scroll log panel.
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [logs.length]);

  const refreshHistory = async () => {
    try {
      const res = await fetch(`${base}/projects/${projectId}/deployments`, {
        headers: { ...auth.authHeader() },
      });
      if (res.ok) {
        const list = (await res.json()) as DeploymentRecord[];
        setDeployments(list.reverse());
      }
    } catch {
      // silent — UI keeps last good state
    }
  };

  const startDeploy = async () => {
    setBanner(null);
    setLogs([]);
    setRunning(true);
    const envMap: Record<string, string> = {};
    for (const row of envRows) {
      if (row.key.trim()) envMap[row.key.trim()] = row.value;
    }
    try {
      const res = await fetch(`${base}/projects/${projectId}/deploy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...auth.authHeader() },
        body: JSON.stringify({ provider, region, env: envMap }),
      });
      if (!res.ok) {
        const text = await res.text();
        setBanner({ kind: 'error', text: text || `Deploy failed: ${res.status}` });
        setRunning(false);
        return;
      }
      const json = (await res.json()) as { deploymentId: string; streamURL: string };
      setActiveDeploymentId(json.deploymentId);
      connectSSE(json.deploymentId);
    } catch (err) {
      setBanner({ kind: 'error', text: (err as Error).message });
      setRunning(false);
    }
  };

  const connectSSE = (deploymentId: string) => {
    const url = auth.appendTokenParam(`${base}/deployments/${deploymentId}/stream`);
    const es = new EventSource(url);
    const handler = (ev: MessageEvent) => {
      try {
        const data = JSON.parse(ev.data) as DeployEvent;
        setLogs((prev) => [...prev, data]);
        if (data.kind === 'deployed') {
          setBanner({ kind: 'success', text: `Deployment live: ${data.url ?? ''}` });
          setRunning(false);
          es.close();
          void refreshHistory();
        }
        if (data.kind === 'failed') {
          setBanner({ kind: 'error', text: data.error ?? 'deploy failed' });
          setRunning(false);
          es.close();
          void refreshHistory();
        }
      } catch {
        // ignore malformed payloads
      }
    };
    ['log', 'deploy_started', 'build_started', 'build_done', 'push_started', 'push_done', 'deployed', 'failed'].forEach((k) =>
      es.addEventListener(k, handler as EventListener),
    );
    es.onerror = () => {
      // EventSource auto-retries; if the deploy already terminated we close.
      if (!running) es.close();
    };
  };

  const exportZip = async () => {
    setBanner(null);
    try {
      const res = await fetch(`${base}/projects/${projectId}/export/zip`, {
        method: 'POST',
        headers: { ...auth.authHeader() },
      });
      if (!res.ok) {
        const text = await res.text();
        setBanner({ kind: 'error', text: text || `ZIP export failed: ${res.status}` });
        return;
      }
      const blob = await res.blob();
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = `${projectId}.zip`;
      a.click();
      URL.revokeObjectURL(a.href);
      setBanner({ kind: 'success', text: 'ZIP export downloaded.' });
    } catch (err) {
      setBanner({ kind: 'error', text: (err as Error).message });
    }
  };

  const exportGitHub = async () => {
    setBanner(null);
    try {
      const res = await fetch(`${base}/projects/${projectId}/export/github`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...auth.authHeader() },
        body: JSON.stringify({ repoName: repoName.trim() || projectId, private: repoPrivate }),
      });
      if (!res.ok) {
        const text = await res.text();
        setBanner({ kind: 'error', text: text || `GitHub export failed: ${res.status}` });
        return;
      }
      const json = (await res.json()) as { repoUrl: string };
      setBanner({ kind: 'success', text: `Project pushed to GitHub: ${json.repoUrl}` });
      window.open(json.repoUrl, '_blank');
    } catch (err) {
      setBanner({ kind: 'error', text: (err as Error).message });
    }
  };

  const providerCard = useMemo(() => providerCardSpec(provider), [provider]);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={[]} onLogout={logout}>
      <PageTitle
        eyebrow="Production deploy"
        title={project ? `Deploy · ${project.name}` : 'Deploy'}
        subtitle="Choose a provider, set environment variables, and get a public URL in minutes. You can also export the code to GitHub or ZIP."
        action={
          <Stack direction="row" spacing={1}>
            <Button startIcon={<Refresh />} variant="outlined" onClick={refreshHistory}>
              Refresh history
            </Button>
          </Stack>
        }
      />

      {banner && (
        <Alert severity={banner.kind} sx={{ mb: 2 }} onClose={() => setBanner(null)}>
          {banner.text}
        </Alert>
      )}

      <Stack spacing={2.4}>
        <Surface sx={{ p: 2.4 }}>
          <Typography variant="overline" sx={{ color: '#9fb500' }}>
            Choose provider
          </Typography>
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.4} sx={{ mt: 1.2 }}>
            {(['fly', 'railway', 'github', 'zip'] as Provider[]).map((p) => (
              <ProviderTile
                key={p}
                kind={p}
                selected={provider === p}
                onClick={() => setProvider(p)}
                disabled={
                  (p === 'fly' && plan && !plan.providers.fly) ||
                  (p === 'railway' && plan && !plan.providers.railway) ||
                  false
                }
              />
            ))}
          </Stack>
          {plan && !plan.providers.fly && provider === 'fly' && (
            <Alert severity="warning" sx={{ mt: 1.6 }}>
              FLY_API_TOKEN is not configured on the server. Add it to the orchestrator environment and restart.
            </Alert>
          )}
          {plan && !plan.providers.railway && provider === 'railway' && (
            <Alert severity="warning" sx={{ mt: 1.6 }}>
              RAILWAY_TOKEN is not configured on the server.
            </Alert>
          )}
        </Surface>

        {(provider === 'fly' || provider === 'railway') && (
          <Surface sx={{ p: 2.4 }}>
            <Stack spacing={2}>
              <Typography variant="overline" sx={{ color: '#9fb500' }}>
                Deployment settings
              </Typography>

              {provider === 'fly' && (
                <Box>
                  <Typography variant="body2" sx={{ mb: 0.8 }}>
                    Deployment region
                  </Typography>
                  <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap">
                    {FLY_REGIONS.map((r) => (
                      <Chip
                        key={r.id}
                        label={r.label}
                        onClick={() => setRegion(r.id)}
                        sx={{
                          bgcolor: region === r.id ? tokens.color.accent.lime : 'transparent',
                          color: region === r.id ? '#111' : tokens.color.text.inverse,
                          border: '1px solid rgba(17,17,17,0.14)',
                          cursor: 'pointer',
                        }}
                      />
                    ))}
                  </Stack>
                </Box>
              )}

              <Box>
                <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 0.8 }}>
                  <Typography variant="body2">Environment variables</Typography>
                  <Button
                    size="small"
                    startIcon={<Add />}
                    onClick={() => setEnvRows((prev) => [...prev, { key: '', value: '' }])}
                  >
                    Add variable
                  </Button>
                </Stack>
                <Stack spacing={0.8}>
                  {envRows.map((row, idx) => (
                    <Stack key={idx} direction="row" spacing={1}>
                      <TextField
                        placeholder="KEY"
                        value={row.key}
                        onChange={(e) =>
                          setEnvRows((prev) => prev.map((r, i) => (i === idx ? { ...r, key: e.target.value } : r)))
                        }
                        size="small"
                        sx={{ flex: 1, ...envFieldSx }}
                      />
                      <TextField
                        placeholder="value"
                        value={row.value}
                        onChange={(e) =>
                          setEnvRows((prev) => prev.map((r, i) => (i === idx ? { ...r, value: e.target.value } : r)))
                        }
                        size="small"
                        sx={{ flex: 2, ...envFieldSx }}
                      />
                      <IconButton
                        aria-label="Remove variable"
                        size="small"
                        onClick={() => setEnvRows((prev) => prev.filter((_, i) => i !== idx))}
                        sx={{ color: tokens.color.text.inverse }}
                      >
                        ×
                      </IconButton>
                    </Stack>
                  ))}
                </Stack>
              </Box>

              <Divider sx={{ borderColor: 'rgba(17,17,17,0.12)' }} />

              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="body2" sx={{ color: '#686158' }}>
                  {providerCard.helper}
                </Typography>
                <Button
                  variant="contained"
                  size="large"
                  disabled={running}
                  onClick={startDeploy}
                  startIcon={running ? <CircularProgress size={18} /> : providerCard.icon}
                  sx={{ minWidth: 200 }}
                >
                  {running ? 'Deploying...' : `Deploy to ${providerCard.label}`}
                </Button>
              </Stack>
            </Stack>
          </Surface>
        )}

        {provider === 'github' && (
          <Surface sx={{ p: 2.4 }}>
            <Stack spacing={2}>
              <Typography variant="overline" sx={{ color: '#9fb500' }}>
                Export to GitHub
              </Typography>
              <TextField
                label="Repository name"
                value={repoName}
                onChange={(e) => setRepoName(e.target.value)}
                placeholder={projectId}
                size="small"
                sx={envFieldSx}
              />
              <Stack direction="row" alignItems="center" spacing={1}>
                <Switch checked={repoPrivate} onChange={(e) => setRepoPrivate(e.target.checked)} />
                <Typography variant="body2">Private repository</Typography>
              </Stack>
              <Button
                variant="contained"
                size="large"
                startIcon={<GitHub />}
                onClick={exportGitHub}
                sx={{ alignSelf: 'flex-end', minWidth: 200 }}
              >
                Create repo and push
              </Button>
            </Stack>
          </Surface>
        )}

        {provider === 'zip' && (
          <Surface sx={{ p: 2.4 }}>
            <Stack spacing={2} alignItems="flex-start">
              <Typography variant="overline" sx={{ color: '#9fb500' }}>
                Download as ZIP
              </Typography>
              <Typography variant="body2" sx={{ color: '#686158' }}>
                Package every project file into one archive that is ready for manual deployment on any provider.
              </Typography>
              <Button
                variant="contained"
                size="large"
                startIcon={<CloudDownload />}
                onClick={exportZip}
                sx={{ minWidth: 200 }}
              >
                Download ZIP
              </Button>
            </Stack>
          </Surface>
        )}

        {(logs.length > 0 || running) && (
          <Surface sx={{ p: 2 }}>
            <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1.2 }}>
              <Typography variant="overline" sx={{ color: '#9fb500' }}>
                Deployment log {activeDeploymentId ? `· ${activeDeploymentId.slice(0, 12)}` : ''}
              </Typography>
              {running && <Chip size="small" label="Running" sx={{ bgcolor: tokens.color.accent.lime, color: '#111' }} />}
            </Stack>
            <Box
              sx={{
                bgcolor: '#111',
                color: '#e9f0d0',
                fontFamily: tokens.font.mono ?? 'ui-monospace, monospace',
                fontSize: 12,
                p: 1.5,
                borderRadius: 1,
                maxHeight: 360,
                overflowY: 'auto',
                whiteSpace: 'pre-wrap',
                direction: 'ltr',
                textAlign: 'left',
              }}
            >
              {logs.map((ev, i) => (
                <Box key={i} sx={{ opacity: ev.kind === 'failed' ? 1 : 0.92 }}>
                  <Box component="span" sx={{ color: '#9fb500' }}>
                    [{ev.kind}]
                  </Box>{' '}
                  {ev.line ?? ev.url ?? ev.error}
                </Box>
              ))}
              <div ref={logEndRef} />
            </Box>
          </Surface>
        )}

        <Surface sx={{ p: 2.4 }}>
          <Typography variant="overline" sx={{ color: '#9fb500' }}>
            Previous deployments
          </Typography>
          {deployments.length === 0 ? (
            <Typography variant="body2" sx={{ color: '#686158', mt: 1.2 }}>
              This project has not been deployed yet.
            </Typography>
          ) : (
            <Stack spacing={1.2} sx={{ mt: 1.4 }}>
              {deployments.map((d) => (
                <Box
                  key={d.id}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.2,
                    p: 1.2,
                    borderRadius: 1,
                    border: '1px solid rgba(17,17,17,0.12)',
                    bgcolor: 'rgba(17,17,17,0.02)',
                  }}
                >
                  <StatusPill status={d.status} />
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography variant="body2" sx={{ fontWeight: 500 }}>
                      {d.provider.toUpperCase()} · {d.region || '—'}
                    </Typography>
                    <Typography variant="caption" sx={{ color: '#686158' }}>
                      {new Date(d.createdAt).toLocaleString('en-US')}
                      {d.error ? ` · ${d.error}` : ''}
                    </Typography>
                  </Box>
                  {d.url && (
                    <Button
                      size="small"
                      endIcon={<OpenInNew />}
                      component="a"
                      href={d.url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open
                    </Button>
                  )}
                </Box>
              ))}
            </Stack>
          )}
        </Surface>

        {plan && plan.artifacts.length > 0 && (
          <Surface sx={{ p: 2.4 }}>
            <Typography variant="overline" sx={{ color: '#9fb500' }}>
              Deployment files generated automatically
            </Typography>
            <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap" sx={{ mt: 1.4 }}>
              {plan.artifacts.map((a) => (
                <Chip
                  key={a.path}
                  label={`${a.path}${a.source === 'existing' ? ' ✓' : ''}`}
                  size="small"
                  sx={{
                    bgcolor: a.source === 'existing' ? 'transparent' : tokens.color.accent.lime,
                    color: a.source === 'existing' ? tokens.color.text.inverse : '#111',
                    border: '1px solid rgba(17,17,17,0.14)',
                  }}
                />
              ))}
            </Stack>
            <Typography variant="caption" sx={{ display: 'block', color: '#686158', mt: 1 }}>
              Detected stack: {plan.stack || 'Unknown'}
            </Typography>
          </Surface>
        )}
      </Stack>
    </AppShell>
  );
}

function StatusPill({ status }: { status: DeploymentRecord['status'] }) {
  const map: Record<string, { bg: string; fg: string; label: string }> = {
    running: { bg: '#e9f0d0', fg: '#3a4c00', label: 'Running' },
    deployed: { bg: tokens.color.accent.lime, fg: '#111', label: 'Live' },
    failed: { bg: '#ffd7d7', fg: '#7a0000', label: 'Failed' },
  };
  const s = map[status] ?? map.running;
  return (
    <Chip
      label={s.label}
      size="small"
      sx={{ bgcolor: s.bg, color: s.fg, fontWeight: 500, minWidth: 56, justifyContent: 'center' }}
    />
  );
}

function providerCardSpec(p: Provider) {
  switch (p) {
    case 'fly':
      return {
        label: 'Fly.io',
        icon: <RocketLaunch />,
        helper: 'Deployment runs remotely on Fly. If flyctl is missing on the server, we will show the install message.',
      };
    case 'railway':
      return {
        label: 'Railway',
        icon: <Train />,
        helper: 'Requires the Railway CLI to be installed (npm i -g @railway/cli).',
      };
    case 'github':
      return { label: 'GitHub', icon: <GitHub />, helper: '' };
    case 'zip':
      return { label: 'ZIP', icon: <CloudDownload />, helper: '' };
  }
}

function ProviderTile({
  kind,
  selected,
  disabled,
  onClick,
}: {
  kind: Provider;
  selected: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  const spec = providerCardSpec(kind);
  return (
    <Box
      role="button"
      aria-pressed={selected}
      onClick={() => !disabled && onClick()}
      sx={{
        flex: 1,
        p: 1.6,
        borderRadius: 1.5,
        border: '1px solid',
        borderColor: selected ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)',
        bgcolor: selected ? tokens.color.accent.lime : 'transparent',
        color: selected ? '#111' : tokens.color.text.inverse,
        opacity: disabled ? 0.5 : 1,
        cursor: disabled ? 'not-allowed' : 'pointer',
        transition: 'all 0.15s ease',
        display: 'flex',
        gap: 1.2,
        alignItems: 'center',
      }}
    >
      <Box sx={{ display: 'flex', '& svg': { fontSize: 26 } }}>{spec.icon}</Box>
      <Box>
        <Typography variant="subtitle2" sx={{ fontWeight: 500 }}>
          {spec.label}
        </Typography>
        <Typography variant="caption" sx={{ display: 'block', opacity: 0.75 }}>
          {kind === 'fly' && 'Public URL in about a minute'}
          {kind === 'railway' && 'Deploy through the CLI'}
          {kind === 'github' && 'New repo with first push'}
          {kind === 'zip' && 'Single-file export'}
        </Typography>
      </Box>
      {/* placeholder for upload icon when relevant */}
      {kind === 'fly' && selected ? <CloudUpload sx={{ ml: 'auto' }} /> : null}
    </Box>
  );
}

const envFieldSx = {
  '& .MuiOutlinedInput-root': {
    bgcolor: 'rgba(17,17,17,0.04)',
    color: tokens.color.text.inverse,
    '& fieldset': { borderColor: 'rgba(17,17,17,0.14)' },
    '&:hover fieldset': { borderColor: tokens.color.accent.lime },
    '&.Mui-focused fieldset': { borderColor: tokens.color.accent.lime },
  },
  '& .MuiInputLabel-root': { color: '#686158' },
};
