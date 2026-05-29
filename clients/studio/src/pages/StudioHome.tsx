import { useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Button, Card, Chip, CircularProgress, IconButton, InputBase, Stack, Switch, Tooltip, Typography } from '@mui/material';
import { Carousel, toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { formatUSD } from '@ironflyer/core';
import { useStudio } from '../store';
import { mockProject, recentProjects, type Gate, type StudioProject } from '../studioData';
import { STARTERS, matchStarter } from '../lib/starters';
import { useLiveGates } from '../hooks/useLiveGates';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useWallet } from '../hooks/useEconomics';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { text } from '@ironflyer/design-tokens/brand';

interface ApiProject {
  id: string;
  name: string;
  status?: string | null;
  description?: string | null;
  idea?: string | null;
  updatedAt?: string | null;
  project?: StudioProject;
}
interface ApiFile { path: string; content?: string | null }

function toneFor(status?: string | null): string {
  const s = (status ?? '').toLowerCase();
  if (s.includes('ship') || s.includes('done') || s.includes('complete') || s.includes('closed')) return 'success.main';
  if (s.includes('error') || s.includes('block') || s.includes('fail') || s.includes('deny')) return 'error.main';
  if (s.includes('run') || s.includes('preview') || s.includes('live')) return 'primary.main';
  return 'warning.main';
}

function icon(paths: string[]) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round">
      {paths.map((d) => <path key={d} d={d} />)}
    </svg>
  );
}

const icons = {
  import: icon(['M12 3v12', 'M7 10l5 5 5-5', 'M5 21h14']),
  folder: icon(['M3 7h7l2 2h9v9a2 2 0 01-2 2H5a2 2 0 01-2-2z']),
  template: icon(['M4 5h16', 'M4 12h10', 'M4 19h7']),
  start: icon(['M5 12h14', 'M13 6l6 6-6 6']),
};

// Each chip seeds the composer with a concrete finishing intent so the agent
// scaffolds toward that goal instead of a generic build.
const categories: { label: string; prompt: string }[] = [
  { label: 'Import a build', prompt: 'Import my existing repo or Lovable/Bolt link and finish it' },
  { label: 'Finish auth', prompt: 'Add production-grade authentication and session handling' },
  { label: 'Wire payments', prompt: 'Wire Stripe payments with a prepaid wallet and checkout' },
  { label: 'Harden security', prompt: 'Run the security gates and harden this app for production' },
  { label: 'Ship to prod', prompt: 'Take this app through the gates and deploy it to production' },
];

function fallbackRecents(): ApiProject[] {
  return recentProjects.map(({ project }) => ({
    id: project.id,
    name: project.name,
    status: project.deploy.status === 'production' ? 'shipped' : project.gates.some((g) => g.status === 'blocked') ? 'blocked' : 'open',
    description: project.source,
    project,
  }));
}

function projectShell(p: ApiProject): StudioProject {
  return {
    ...mockProject,
    id: p.id,
    name: p.name,
    source: p.description || p.idea || 'saved project',
    deploy: { ...mockProject.deploy, status: p.status?.toLowerCase().includes('ship') ? 'production' : 'none' },
  };
}

function readinessTone(kind: 'wallet' | 'gates' | 'security' | 'deploy' | 'live', danger: boolean, active: boolean): string {
  if (danger) return 'error.main';
  if (active) return kind === 'live' ? 'primary.main' : 'success.main';
  return 'warning.main';
}

function ReadinessTile({ label, value, sub, tone }: { label: string; value: string; sub: string; tone: string }) {
  return (
    <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 2, bgcolor: 'background.paper', px: 1.5, py: 1.25, minWidth: 0 }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 0.5 }}>
        <Box sx={{ width: 7, height: 7, borderRadius: 99, bgcolor: tone, flexShrink: 0 }} />
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>
          {label}
        </Typography>
      </Stack>
      <Typography sx={{ fontWeight: 700, fontSize: text.s105 }} noWrap>{value}</Typography>
      <Typography sx={{ color: 'text.secondary', fontSize: text.s72 }} noWrap>{sub}</Typography>
    </Box>
  );
}

function gateSummary(gates: Gate[]) {
  const closed = gates.filter((g) => g.status === 'closed').length;
  const blocked = gates.filter((g) => g.status === 'blocked').length;
  const active = gates.filter((g) => g.status === 'running' || g.status === 'open').length;
  return { closed, blocked, active, total: gates.length };
}

export function StudioHome() {
  const navigate = useNavigate();
  const request = useRequest();
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
  const { startFromTemplate, openProject, openLiveProject } = useStudio();
  const current = useStudio((s) => s.current);
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const liveProjectId = storeProjectId ?? firstProjectId;
  const { gates, isLive: gatesLive } = useLiveGates();
  const { wallet, isLive: walletLive } = useWallet();
  const { latest, isLive: executionsLive } = useProjectExecutions(liveProjectId);
  const [prompt, setPrompt] = useState('');
  const [planMode, setPlanMode] = useState(false);
  const [openingId, setOpeningId] = useState<string | null>(null);

  const { data: recent, isLive: recentLive } = useGraphQLQuery<ApiProject[], { projects: ApiProject[] }>({
    key: ['projects'],
    operationName: 'Projects', query: operations.PROJECTS,
    fallbackData: [], map: (r) => r.projects ?? [],
  });
  const recents = recent.length ? recent.slice(0, 4) : fallbackRecents();

  const summary = gateSummary(gates);
  const risk = latest?.riskScore ?? current.security.riskScore;
  const deployStatus = current.deploy.status;
  const walletHeadroom = walletLive ? wallet.availableUSD : Math.max(0, current.meters.walletBudget - current.meters.walletUsed);
  const readiness = [
    {
      label: 'Wallet',
      value: formatUSD(walletHeadroom),
      sub: walletLive ? `${formatUSD(wallet.holdUSD)} held` : `${formatUSD(current.meters.walletBudget)} budget`,
      tone: readinessTone('wallet', walletHeadroom <= 0, walletHeadroom > 10),
    },
    {
      label: 'Gates',
      value: `${summary.closed}/${summary.total} closed`,
      sub: summary.blocked ? `${summary.blocked} blocked` : `${summary.active} active`,
      tone: readinessTone('gates', summary.blocked > 0, summary.closed === summary.total),
    },
    {
      label: 'Security',
      value: risk > 0 ? `Risk ${risk}` : 'Clean start',
      sub: current.security.policy.effect === 'deny' ? 'policy deny' : 'policy allow',
      tone: readinessTone('security', current.security.policy.effect === 'deny' || risk >= 70, risk < 40),
    },
    {
      label: 'Deploy',
      value: deployStatus === 'production' ? 'Production' : deployStatus === 'preview' ? 'Preview' : deployStatus === 'failed' ? 'Failed' : 'Not shipped',
      sub: current.deploy.url ?? current.source,
      tone: readinessTone('deploy', deployStatus === 'failed', deployStatus === 'production' || deployStatus === 'preview'),
    },
    {
      label: 'Live',
      value: liveProjectId ? 'Connected' : 'Preview mode',
      sub: gatesLive || walletLive || executionsLive ? 'live data flowing' : 'local sample data',
      tone: readinessTone('live', false, !!liveProjectId && recentLive),
    },
  ];

  const startWith = (value: string) => {
    const v = value.trim() || 'Finish my product';
    startFromTemplate(v, matchStarter(v).files, {
      workMode: planMode ? 'plan' : 'execute',
      preflight: planMode,
    });
    navigate('/build');
  };
  const start = () => startWith(prompt);
  const seedPrompt = (value: string) => {
    setPrompt(value);
    inputRef.current?.focus();
  };
  const startTemplate = (s: (typeof STARTERS)[number]) => {
    startFromTemplate(s.prompt, s.files, {
      workMode: planMode ? 'plan' : 'execute',
      preflight: planMode,
    });
    navigate('/build');
  };
  const openRecent = async (p: ApiProject) => {
    if (p.project || !request) {
      openProject(p.project ?? projectShell(p));
      navigate('/build');
      return;
    }
    setOpeningId(p.id);
    try {
      const d = await request<{ projectFiles: ApiFile[] }>('ProjectFiles', operations.PROJECT_FILES, { id: p.id });
      const files = (d.projectFiles ?? [])
        .filter((f) => typeof f.content === 'string')
        .map((f) => ({ path: f.path, content: f.content as string }));
      openLiveProject(projectShell(p), p.id, files);
      navigate('/build');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not open project.', 'error');
    } finally {
      setOpeningId(null);
    }
  };

  return (
    <Box sx={{ minHeight: '100%', px: { xs: 2, md: 4 }, py: { xs: 3, md: 4 } }}>
      <Box sx={{ maxWidth: 1120, mx: 'auto' }}>
        <Stack direction={{ xs: 'column', md: 'row' }} alignItems={{ xs: 'flex-start', md: 'center' }} justifyContent="space-between" spacing={1.5} sx={{ mb: 2.5 }}>
          <Box sx={{ minWidth: 0 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'text.disabled', mb: 0.75 })}>
              Production finisher
            </Typography>
            <Typography variant="h2" sx={{ fontSize: { xs: text.s180, md: text.s225 }, lineHeight: 1.12 }}>
              Studio cockpit
            </Typography>
          </Box>
          <Chip
            label={liveProjectId ? `live ${liveProjectId}` : current.name}
            variant="outlined"
            sx={{ maxWidth: { xs: '100%', md: 360 }, borderColor: 'divider', color: 'text.secondary', fontFamily: (t) => t.brand.font.mono, fontSize: text.s70 }}
          />
        </Stack>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(5, 1fr)' }, gap: 1, mb: 2.5 }}>
          {readiness.map((item) => <ReadinessTile key={item.label} {...item} />)}
        </Box>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1.4fr) minmax(300px, 0.8fr)' }, gap: 2 }}>
          <Card sx={{ p: { xs: 2, md: 2.5 }, borderRadius: 2 }}>
            <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5, gap: 1 }}>
              <Box>
                <Typography variant="h5" sx={{ fontSize: text.s120 }}>What are we finishing?</Typography>
                <Typography sx={{ color: 'text.secondary', fontSize: text.s82 }}>Describe the work, import a source, or start from a known path.</Typography>
              </Box>
              <Stack direction="row" spacing={0.5}>
                <Tooltip title="Seed an import prompt" arrow>
                  <IconButton size="small" aria-label="Seed import prompt" onClick={() => seedPrompt(categories[0]?.prompt ?? 'Import my existing repo and finish it')} sx={{ border: 1, borderColor: 'divider', borderRadius: 1.5, width: 34, height: 34 }}>
                    {icons.import}
                  </IconButton>
                </Tooltip>
                <Tooltip title="Open projects" arrow>
                  <IconButton size="small" aria-label="Open projects" onClick={() => navigate('/projects')} sx={{ border: 1, borderColor: 'divider', borderRadius: 1.5, width: 34, height: 34 }}>
                    {icons.folder}
                  </IconButton>
                </Tooltip>
                <Tooltip title="Browse templates" arrow>
                  <IconButton size="small" aria-label="Browse templates" onClick={() => navigate('/templates')} sx={{ border: 1, borderColor: 'divider', borderRadius: 1.5, width: 34, height: 34 }}>
                    {icons.template}
                  </IconButton>
                </Tooltip>
              </Stack>
            </Stack>

            <Box
              sx={{
                border: 1,
                borderColor: 'divider',
                borderRadius: 2,
                bgcolor: 'background.default',
                p: 1.5,
                transition: (t) => `border-color ${t.brand.motion.fast}`,
                '&:focus-within': { borderColor: 'primary.main' },
              }}
            >
              <InputBase
                inputRef={inputRef}
                multiline
                minRows={3}
                fullWidth
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder="Paste a repo or Lovable/Bolt link, or describe the product you want to finish..."
                onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); start(); } }}
                autoFocus
                sx={{ fontSize: text.s95, px: 0.5 }}
              />
              <Stack direction={{ xs: 'column', sm: 'row' }} alignItems={{ xs: 'stretch', sm: 'center' }} justifyContent="space-between" spacing={1.5} sx={{ mt: 1.25 }}>
                <Stack direction="row" alignItems="center" spacing={1}>
                  <Typography sx={{ fontSize: text.s82, color: 'text.secondary' }}>Plan first</Typography>
                  <Switch size="small" checked={planMode} onChange={(e) => setPlanMode(e.target.checked)} />
                </Stack>
                <Button variant="contained" onClick={start} endIcon={icons.start} sx={{ alignSelf: { xs: 'stretch', sm: 'auto' } }}>
                  Start
                </Button>
              </Stack>
            </Box>

            <Stack direction="row" spacing={1} sx={{ mt: 2, flexWrap: 'wrap', gap: 1 }}>
              {categories.map((c) => (
                <Chip key={c.label} label={c.label} onClick={() => startWith(c.prompt)} variant="outlined" sx={{ borderColor: 'divider', '&:hover': { bgcolor: 'action.hover' } }} />
              ))}
            </Stack>
          </Card>

          <Card sx={{ p: { xs: 2, md: 2.5 }, borderRadius: 2 }}>
            <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>
                Recent projects
              </Typography>
              {recentLive && <Chip size="small" label="live" sx={{ height: 20, fontSize: text.s68, bgcolor: 'action.hover', color: 'primary.main' }} />}
            </Stack>
            <Stack spacing={1}>
              {recents.map((r) => (
                <Button
                  key={r.id}
                  onClick={() => void openRecent(r)}
                  disabled={openingId === r.id}
                  sx={{ justifyContent: 'flex-start', p: 1.25, border: 1, borderColor: 'divider', borderRadius: 2, textAlign: 'left', '&:hover': { borderColor: 'text.disabled', bgcolor: 'action.hover' } }}
                >
                  <Stack direction="row" alignItems="center" spacing={1.25} sx={{ minWidth: 0, width: '100%' }}>
                    <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: toneFor(r.status), flexShrink: 0 }} />
                    <Box sx={{ minWidth: 0, flex: 1 }}>
                      <Typography sx={{ fontWeight: 650, color: 'text.primary', fontSize: text.s90 }} noWrap>{r.name}</Typography>
                      <Typography sx={{ fontSize: text.s72, color: 'text.disabled' }} noWrap>{r.status ?? r.description ?? 'in progress'}</Typography>
                    </Box>
                    {openingId === r.id && <CircularProgress size={14} />}
                  </Stack>
                </Button>
              ))}
            </Stack>
          </Card>
        </Box>

        <Box sx={{ mt: 2.5 }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled' })}>Start from a template</Typography>
            <Chip size="small" label="runs instantly" sx={(t) => ({ height: 18, fontSize: text.s60, fontFamily: t.brand.font.mono, bgcolor: `${t.palette.success.main}1f`, color: 'success.main' })} />
          </Stack>
          <Carousel slidesPerView="auto" gap={12} pagination={false}>
            {STARTERS.map((tpl) => (
              <Card key={tpl.id} onClick={() => startTemplate(tpl)} sx={{ width: 210, p: 2, cursor: 'pointer', borderRadius: 2, transition: (t) => `border-color ${t.brand.motion.fast}`, '&:hover': { borderColor: 'primary.main' } }}>
                <Typography sx={{ fontWeight: 650, fontSize: text.s95 }}>{tpl.name}</Typography>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s72, color: 'text.disabled', mt: 0.5 })}>{tpl.meta}</Typography>
              </Card>
            ))}
          </Carousel>
        </Box>
      </Box>
    </Box>
  );
}
